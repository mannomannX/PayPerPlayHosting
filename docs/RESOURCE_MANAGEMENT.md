# Resource Management & Edge Cases

## 📋 Inhaltsverzeichnis

1. [Server RAM-Upgrade (Live-Migration)](#1-server-ram-upgrade-live-migration)
2. [Race Conditions bei Ressourcen-Buchung](#2-race-conditions-bei-ressourcen-buchung)
3. [Aktuelle Implementierung](#3-aktuelle-implementierung)
4. [Fehlende Features](#4-fehlende-features)

---

## 1. Server RAM-Upgrade (Live-Migration)

### Problem

**Szenario:** User erhöht RAM im Dashboard von 2GB → 4GB bei laufendem Server mit Spielern.

**Frage:** Wie wird der Server übertragen, ohne Spieler zu kicken?

### Aktueller Stand: ❌ NICHT IMPLEMENTIERT

Docker-Container können RAM **nicht live ändern**. Ein Neustart ist zwingend erforderlich.

### Lösungsansätze

#### Option A: Stop-Update-Start (Einfach, mit Downtime)

```
1. User klickt "Upgrade auf 4GB"
2. API zeigt Warnung: "Server wird neugestartet (Spieler werden gekickt)"
3. User bestätigt
4. Container wird gestoppt
5. RAM-Wert in DB wird aktualisiert
6. Container wird mit neuem RAM-Limit neu erstellt
7. Spieler können wieder joinen

Downtime: ~30 Sekunden
Implementierungsaufwand: 2 Stunden
```

**Code-Änderungen:**

```go
// internal/service/minecraft_service.go

func (s *MinecraftService) UpgradeServerRAM(serverID string, newRAMMB int) error {
    server, err := s.repo.FindByID(serverID)
    if err != nil {
        return err
    }

    wasRunning := server.Status == models.StatusRunning

    // 1. Stop server if running
    if wasRunning {
        if err := s.StopServer(serverID, "RAM upgrade"); err != nil {
            return fmt.Errorf("failed to stop server: %w", err)
        }
    }

    // 2. Update RAM in database
    oldRAM := server.RAMMb
    server.RAMMb = newRAMMB
    if err := s.repo.Update(server); err != nil {
        return fmt.Errorf("failed to update RAM: %w", err)
    }

    // 3. Update Conductor allocation
    if s.conductor != nil {
        s.conductor.UpdateServerAllocation(serverID, oldRAM, newRAMMB)
    }

    // 4. Restart if was running
    if wasRunning {
        if err := s.StartServer(serverID); err != nil {
            return fmt.Errorf("failed to restart server: %w", err)
        }
    }

    return nil
}
```

#### Option B: Zero-Downtime Migration (Komplex, keine Downtime)

```
1. User klickt "Upgrade auf 4GB"
2. Neuer Container wird mit 4GB erstellt (anderer Port)
3. Velocity Proxy wird informiert: "Route mc-server-abc zu neuem Port"
4. Neue Spieler joinen den neuen Container
5. Alter Container wird nach 5 Minuten (wenn leer) gestoppt

Downtime: 0 Sekunden (für neue Spieler)
Bestehende Spieler: Müssen Server verlassen + neu joinen
Implementierungsaufwand: 16 Stunden
```

**Herausforderungen:**

- World-Daten müssen zwischen Containern synchronisiert werden
- Plugins/Mods müssen portiert werden
- Komplexe Orchestrierung erforderlich

### Empfehlung: Option A (für MVP)

- **Pragmatisch:** 99% der Server-Upgrades passieren, wenn keiner spielt
- **Einfach:** User wird vorher gewarnt
- **Schnell:** 2 Stunden Implementierung vs. 16 Stunden
- **Später:** Option B kann als Premium-Feature nachgerüstet werden

---

## 2. Race Conditions bei Ressourcen-Buchung

### Problem

**Szenario:** 5 User starten gleichzeitig Server, aber nur 2GB RAM verfügbar.

**Risiko:** Ohne atomare Reservierung kann es zu Overbooking kommen!

```
T+0ms:  User A startet Server (2GB) → Prüft: 2500 MB frei ✅
T+1ms:  User B startet Server (2GB) → Prüft: 2500 MB frei ✅ (Race!)
T+2ms:  User C startet Server (2GB) → Prüft: 2500 MB frei ✅ (Race!)
T+50ms: Alle 3 Container starten gleichzeitig → 6GB benötigt, 2.5GB verfügbar
T+60ms: OOM Kernel-Killer tötet Container! 💥
```

### Aktueller Stand: ⚠️ TEILWEISE GESCHÜTZT

Der Code hat **Mutexes**, aber keine **transaktionale Reservierung**.

#### Was ist geschützt?

```go
// internal/conductor/conductor.go

func (c *Conductor) CheckCapacity(ramMB int) (bool, string) {
    c.NodeRegistry.mu.RLock()         // ✅ Read-Lock
    defer c.NodeRegistry.mu.RUnlock()

    // Prüfung ist atomic
    if availableRAM < ramMB {
        return false, "insufficient RAM"
    }
    return true, ""
}

func (c *Conductor) AllocateResources(serverID string, ramMB int) error {
    c.NodeRegistry.mu.Lock()          // ✅ Write-Lock
    defer c.NodeRegistry.mu.Unlock()

    // Allokation ist atomic
    node.AllocatedRAMMB += ramMB
    return nil
}
```

#### Was ist NICHT geschützt?

```go
// internal/service/minecraft_service.go

func (s *MinecraftService) StartServer(serverID string) error {
    // RACE CONDITION MÖGLICH HIER! ⚠️

    // Schritt 1: Prüfung (Mutex wird freigegeben nach Check!)
    hasCapacity, reason := s.conductor.CheckCapacity(server.RAMMb)

    // ⚠️ ZWISCHEN Schritt 1 und 2 kann anderer Request reinkommen!

    // Schritt 2: Allokation (neuer Mutex-Lock)
    if hasCapacity {
        s.conductor.AllocateResources(serverID, server.RAMMb)
        // Container starten...
    }
}
```

**Problem:** Check und Allocate sind **nicht atomar**!

### Lösung: Transaktionale Reservierung

#### Implementierung: Optimistic Locking Pattern

```go
// internal/conductor/conductor.go

// TryReserveResources attempts to atomically check AND reserve resources
// Returns: (success bool, reason string, rollback func())
func (c *Conductor) TryReserveResources(serverID string, ramMB int) (bool, string, func()) {
    c.NodeRegistry.mu.Lock()
    defer c.NodeRegistry.mu.Unlock()

    // Atomic Check + Reserve in ONE transaction
    node := c.NodeRegistry.SelectBestNode(ramMB)
    if node == nil {
        return false, "insufficient capacity", nil
    }

    if node.AvailableRAMMB() < ramMB {
        return false, "node capacity exceeded", nil
    }

    // Reserve resources immediately (while still holding lock!)
    node.AllocatedRAMMB += ramMB
    c.ContainerRegistry.Register(serverID, node.ID, ramMB)

    // Return rollback function (in case Docker fails)
    rollback := func() {
        c.NodeRegistry.mu.Lock()
        defer c.NodeRegistry.mu.Unlock()
        node.AllocatedRAMMB -= ramMB
        c.ContainerRegistry.Unregister(serverID)
    }

    return true, "", rollback
}
```

#### Verwendung in MinecraftService:

```go
func (s *MinecraftService) StartServer(serverID string) error {
    server, err := s.repo.FindByID(serverID)
    if err != nil {
        return err
    }

    // Atomic reservation (check + allocate in ONE lock!)
    success, reason, rollback := s.conductor.TryReserveResources(serverID, server.RAMMb)
    if !success {
        // No resources reserved, safe to return
        return fmt.Errorf("cannot start: %s", reason)
    }

    // Resources are NOW reserved, no race condition possible
    // If container creation fails, rollback the reservation
    containerID, err := s.dockerService.CreateContainer(...)
    if err != nil {
        rollback() // Free reserved resources
        return fmt.Errorf("failed to create container: %w", err)
    }

    // Success! Resources stay allocated
    return nil
}
```

#### Vorteile:

✅ **Race-Condition frei** - Check + Allocate sind atomar
✅ **Rollback-sicher** - Bei Docker-Fehlern wird Reservierung freigegeben
✅ **Queue-kompatibel** - Queued Server reservieren KEINE Ressourcen (warten passiv)
✅ **Deadlock-frei** - Nur ein Mutex, keine verschachtelten Locks

---

## 3. Aktuelle Implementierung

### Was funktioniert bereits:

#### ✅ Basis-Schutz durch Mutexes
- NodeRegistry ist thread-safe
- ContainerRegistry ist thread-safe
- StartQueue ist thread-safe

#### ✅ Resource Guards
```go
// CPU Guard: Verhindert Container-Start bei zu hoher CPU-Last
if cpuPercent > 80.0 {
    return false, "CPU GUARD: System under heavy load"
}

// RAM Guard: Verhindert Container-Start bei zu wenig RAM
if availableRAM < ramMB {
    return false, "RAM GUARD: Insufficient memory"
}
```

#### ✅ Queue-System
- Server werden bei Kapazitätsengpass **nicht abgelehnt**, sondern **in Queue gestellt**
- Queue wird alle 30 Sekunden verarbeitet
- Nach Scaling werden queued Server automatisch gestartet

### Was fehlt noch:

#### ❌ Transaktionale Reservierung
- Check + Allocate sind nicht atomar (Race Condition möglich)
- Siehe Lösung oben

#### ❌ RAM-Upgrade Funktion
- User kann RAM nicht im Dashboard ändern
- Braucht API-Endpoint: `PATCH /api/servers/:id/upgrade-ram`

#### ❌ Reservierungs-Timeout
- Reservierte Ressourcen bleiben ewig allokiert, wenn Container-Start fehlschlägt
- Braucht Timeout-Mechanismus (z.B. 5 Minuten)

#### ❌ Preemption (optional)
- Bei kritischer Kapazität: Niedrig-priorisierte Server stoppen, um Platz zu machen
- Für MVP nicht erforderlich

---

## 4. Fehlende Features

### Priorität 1: Transaktionale Reservierung (2 Stunden)

**Aufgabe:** `TryReserveResources()` implementieren (siehe oben)

**Dateien:**
- `internal/conductor/conductor.go` - Neue Methode hinzufügen
- `internal/service/minecraft_service.go` - StartServer() refactoren

**Testing:**
```bash
# Simuliere 10 gleichzeitige Server-Starts
for i in {1..10}; do
  curl -X POST "/api/servers" -d '{"ram_mb": 1024}' &
done
wait

# Erwartung: Maximal so viele Server starten, wie RAM verfügbar
# Keine OOM-Errors, keine Race Conditions
```

### Priorität 2: RAM-Upgrade Endpoint (3 Stunden)

**API-Design:**

```http
PATCH /api/servers/:id/upgrade
Authorization: Bearer <token>
Content-Type: application/json

{
  "ram_mb": 4096,
  "allow_restart": true  // User bestätigt Downtime
}

Response 200 OK:
{
  "message": "Server upgraded successfully",
  "old_ram_mb": 2048,
  "new_ram_mb": 4096,
  "downtime_seconds": 28
}

Response 400 Bad Request:
{
  "error": "Server is running. Set allow_restart=true to confirm downtime."
}

Response 409 Conflict:
{
  "error": "Insufficient capacity for upgrade. Current available: 1024 MB"
}
```

**Implementierung:**

```go
// internal/api/handler.go

func (h *Handler) UpgradeServerRAM(c *gin.Context) {
    serverID := c.Param("id")

    var req struct {
        RAMMB        int  `json:"ram_mb" binding:"required,min=512,max=16384"`
        AllowRestart bool `json:"allow_restart"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "Invalid request"})
        return
    }

    // Check if upgrade is possible
    server, err := h.mcService.GetServer(serverID)
    if err != nil {
        c.JSON(404, gin.H{"error": "Server not found"})
        return
    }

    // Require confirmation if server is running
    if server.Status == models.StatusRunning && !req.AllowRestart {
        c.JSON(400, gin.H{
            "error": "Server is running. Set allow_restart=true to confirm downtime.",
            "current_status": "running",
        })
        return
    }

    // Check capacity BEFORE upgrade
    delta := req.RAMMB - server.RAMMb
    if delta > 0 {
        hasCapacity, reason := h.mcService.CheckCapacityAvailable(delta)
        if !hasCapacity {
            c.JSON(409, gin.H{
                "error": "Insufficient capacity for upgrade",
                "reason": reason,
            })
            return
        }
    }

    // Perform upgrade
    startTime := time.Now()
    if err := h.mcService.UpgradeServerRAM(serverID, req.RAMMB); err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    c.JSON(200, gin.H{
        "message": "Server upgraded successfully",
        "old_ram_mb": server.RAMMb,
        "new_ram_mb": req.RAMMB,
        "downtime_seconds": int(time.Since(startTime).Seconds()),
    })
}
```

### Priorität 3: Reservation Timeout (1 Stunde)

**Problem:** Wenn `TryReserveResources()` erfolgreich ist, aber `CreateContainer()` fehlschlägt UND der rollback() nicht aufgerufen wird (Crash?), bleiben Ressourcen ewig reserviert.

**Lösung:** Timeouts für unbestätigte Reservierungen

```go
// internal/conductor/conductor.go

type Reservation struct {
    ServerID  string
    NodeID    string
    RAMMB     int
    CreatedAt time.Time
    Confirmed bool  // Set to true when container actually starts
}

func (c *Conductor) StartReservationCleaner() {
    ticker := time.NewTicker(1 * time.Minute)
    go func() {
        for range ticker.C {
            c.CleanupStaleReservations(5 * time.Minute)
        }
    }()
}

func (c *Conductor) CleanupStaleReservations(timeout time.Duration) {
    c.NodeRegistry.mu.Lock()
    defer c.NodeRegistry.mu.Unlock()

    now := time.Now()
    for serverID, res := range c.reservations {
        if !res.Confirmed && now.Sub(res.CreatedAt) > timeout {
            // Reservation never confirmed → rollback
            node, exists := c.NodeRegistry.GetNode(res.NodeID)
            if exists {
                node.AllocatedRAMMB -= res.RAMMB
            }
            delete(c.reservations, serverID)

            logger.Warn("Rolled back stale reservation", map[string]interface{}{
                "server_id": serverID,
                "ram_mb": res.RAMMB,
                "age_seconds": int(now.Sub(res.CreatedAt).Seconds()),
            })
        }
    }
}
```

---

## 5. Testing-Checkliste

### RAM-Upgrade Test

```bash
# 1. Create server with 2GB
SERVER_ID=$(curl -X POST "http://localhost:8000/api/servers" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"upgrade-test","ram_mb":2048}' | jq -r '.server.id')

# 2. Start server
curl -X POST "http://localhost:8000/api/servers/$SERVER_ID/start" \
  -H "Authorization: Bearer $TOKEN"

# 3. Try upgrade WITHOUT allow_restart (should fail)
curl -X PATCH "http://localhost:8000/api/servers/$SERVER_ID/upgrade" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"ram_mb":4096,"allow_restart":false}'
# Expected: 400 Bad Request

# 4. Upgrade WITH allow_restart (should succeed)
curl -X PATCH "http://localhost:8000/api/servers/$SERVER_ID/upgrade" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"ram_mb":4096,"allow_restart":true}'
# Expected: 200 OK, server restarts with 4GB

# 5. Verify
curl "http://localhost:8000/api/servers/$SERVER_ID" \
  -H "Authorization: Bearer $TOKEN" | jq '.server.ram_mb'
# Expected: 4096
```

### Race Condition Test

```bash
# Simulate 20 simultaneous server starts (more than capacity!)
TOKEN="your_token"
for i in {1..20}; do
  echo "Starting server $i..."
  curl -X POST "http://localhost:8000/api/servers" \
    -H "Authorization: Bearer $TOKEN" \
    -d "{\"name\":\"race-test-$i\",\"ram_mb\":1024}" &
done
wait

# Check results
curl "http://localhost:8000/api/servers" -H "Authorization: Bearer $TOKEN" | \
  jq '[.servers[] | select(.name | startswith("race-test"))] |
      group_by(.status) |
      map({status: .[0].status, count: length})'

# Expected output:
# [
#   {"status": "running", "count": 3},   # Actual capacity
#   {"status": "queued", "count": 17}    # Waiting for capacity
# ]
# NOT: OOM errors, crashes, or more running than capacity allows!
```

---

## 6. Deployment-Plan

### Phase 1: Transaktionale Reservierung (Woche 1)
- Implementiere `TryReserveResources()`
- Refactore `MinecraftService.StartServer()`
- Testing: Race Condition Tests
- Deploy to Production (low risk, no user-facing changes)

### Phase 2: RAM-Upgrade Endpoint (Woche 2)
- Implementiere `UpgradeServerRAM()` API
- Frontend: "Upgrade RAM" Button im Dashboard
- Testing: Upgrade-Tests (laufend + gestoppt)
- Deploy to Production (Beta-Feature flag)

### Phase 3: Reservation Timeout (Woche 3)
- Implementiere Reservation-Cleaner
- Monitoring: Grafana-Alert für stale reservations
- Testing: Chaos-Engineering (Container-Starts forcieren zu failen)
- Deploy to Production

---

## 7. Offene Fragen

### Billing bei RAM-Upgrade?

**Frage:** Wenn User von 2GB → 4GB upgraded, wie wird abgerechnet?

**Option A:** Sofort höherer Preis (fair, einfach)
```
Server läuft seit 10 Stunden mit 2GB = 0.02€/h × 2GB × 10h = 0.40€
User upgraded auf 4GB
Server läuft weitere 5 Stunden mit 4GB = 0.02€/h × 4GB × 5h = 0.40€
TOTAL: 0.80€
```

**Option B:** Prorate billing (komplex, fairer)
```
Upgrade-Zeitpunkt: 15:30 Uhr (mitten in der Stunde)
Erste Hälfte (15:00-15:30): 2GB × 0.5h = 0.02€
Zweite Hälfte (15:30-16:00): 4GB × 0.5h = 0.04€
TOTAL: 0.06€ für diese Stunde
```

**Empfehlung:** Option A für MVP (einfacher), später Option B.

### Downgrade erlauben?

**Frage:** Kann User RAM auch **senken** (4GB → 2GB)?

**Antwort:** JA, mit gleicher Logik (Stop → Update → Start).

**Risiko:** Wenn World-Daten zu groß für neuen RAM sind → Crash!

**Lösung:** Warnung im UI: "Achtung: Downgrade kann zu Problemen führen, wenn World zu groß ist."

---

## 8. Zusammenfassung

| Feature | Status | Aufwand | Priorität |
|---------|--------|---------|-----------|
| **Transaktionale Reservierung** | ❌ Fehlt | 2h | 🔴 HOCH |
| **RAM-Upgrade API** | ❌ Fehlt | 3h | 🟡 MITTEL |
| **Reservation Timeout** | ❌ Fehlt | 1h | 🟡 MITTEL |
| **Zero-Downtime Migration** | ❌ Fehlt | 16h | 🟢 NIEDRIG |

**Nächster Schritt:** Implementiere transaktionale Reservierung (verhindert Race Conditions).

**Später:** RAM-Upgrade Endpoint (User-Feature).

**Viel später:** Zero-Downtime Migration (Premium-Feature).
