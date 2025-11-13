# Code-Flaws & Potenzielle Probleme - Zusammenfassung

**Stand:** 2025-11-13
**Quelle:** Live-Code-Analyse (nicht aus Docs)

## 🔴 CRITICAL - Sofortige Aufmerksamkeit nötig

### 1. Email Service in MOCK MODE (Production)
**Datei:** [cmd/api/main.go](../cmd/api/main.go:99-102)
**Schweregrad:** CRITICAL
**Impact:** Keine echten Emails in Production

**Code:**
```go
// 🚧 TODO: Replace MockEmailSender with ResendEmailSender when ready for production
mockEmailSender := service.NewMockEmailSender(db)
emailService := service.NewEmailService(mockEmailSender, db)
logger.Info("Email service initialized (🚧 MOCK MODE)", nil)
```

**Betroffene Features:**
- User Registration (Email-Verifizierung)
- Password Reset
- Security Alerts (Neues Device, Account Locked)
- Potenzielle weitere Email-Benachrichtigungen

**Vorschlag:**
- ResendEmailSender implementieren
- Feature-Flag für Mock vs. Real
- Warnung beim Production-Start wenn MOCK aktiv

---

### 2. User-Relations auskommentiert
**Datei:** [internal/models/user.go](../internal/models/user.go:42-45)
**Schweregrad:** CRITICAL
**Impact:** Keine Foreign Key Constraints, Orphaned Records möglich

**Code:**
```go
// Relationships - Temporarily commented out for testing
// Servers        []MinecraftServer `gorm:"foreignKey:OwnerID"`
// TrustedDevices []TrustedDevice   `gorm:"foreignKey:UserID"`
// SecurityEvents []SecurityEvent   `gorm:"foreignKey:UserID"`
```

**Probleme:**
1. Keine DB-Level Foreign Key Constraints
2. Manuelles Relationship-Management nötig
3. N+1 Query Problem bei Server-Listings
4. Risk of Orphaned Records (Server ohne User, TrustedDevices ohne User)
5. Kommentar sagt "Temporarily for testing" - aber in Production?

**Vorschlag:**
- Relations aktivieren
- Migration für Foreign Keys
- Cascade Delete Rules definieren

---

### 3. Hardcoded Default OwnerID = "default"
**Datei:** [internal/models/server.go](../internal/models/server.go:52)
**Schweregrad:** CRITICAL
**Impact:** Multi-Tenancy nicht vollständig implementiert

**Code:**
```go
OwnerID string `gorm:"not null;default:default"` // Future: user system
```

**Probleme:**
- Alle Server ohne expliziten Owner haben "default" als OwnerID
- User "default" existiert möglicherweise nicht in DB
- Multi-Tenancy-Konzept untergraben
- Billing: Wem werden Kosten zugeordnet?

**Vorschlag:**
- OwnerID als REQUIRED bei Server-Erstellung
- Migration: Alle "default" Servers einem Admin-User zuordnen
- Kommentar "Future: user system" entfernen (User-System existiert ja!)

---

### 4. Keine Partial Failure Handling im Bootstrap
**Datei:** [cmd/api/main.go](../cmd/api/main.go)
**Schweregrad:** CRITICAL
**Impact:** App-Crash bei optionalen Service-Fehlern

**Probleme:**
1. Docker-Fehler → Fatal (Zeile 85-87) - OK, ist core
2. InfluxDB-Fehler → Graceful Fallback (Zeile 64-76) - GOOD!
3. Velocity-Fehler → Warning (Zeile 192-195) - GOOD!
4. Aber: Was wenn z.B. Backup Service fehlschlägt (Zeile 127-130)?
   ```go
   backupService, err := service.NewBackupService(serverRepo, cfg)
   if err != nil {
       logger.Fatal("Failed to initialize backup service", err, nil) // 💀 FATAL!
   }
   ```

**Beispiele für Fatal vs. Warning:**
- Fatal: Database, Docker, Core Services
- Warning: InfluxDB, Velocity, Backup (nicht kritisch für Betrieb)

**Vorschlag:**
- Feature Flags für optionale Services
- Graceful Degradation für nicht-kritische Services
- Health-Check zeigt welche Services active sind

---

### 5. Circular Dependencies (MinecraftService ↔ Conductor)
**Datei:** [cmd/api/main.go](../cmd/api/main.go:244-249)
**Schweregrad:** HIGH
**Impact:** Race Conditions möglich, fragile Initialisierung

**Code:**
```go
// Link Conductor to MinecraftService for capacity management
mcService.SetConductor(cond)
logger.Info("Conductor linked to MinecraftService for resource guard", nil)

// Link MinecraftService to Conductor as ServerStarter for queue processing
cond.SetServerStarter(mcService)
logger.Info("MinecraftService linked to Conductor as ServerStarter for queue processing", nil)
```

**Probleme:**
- Bidirectionale Abhängigkeit
- Sequentielle Initialisierung MUSS eingehalten werden
- Fehleranfällig bei Refactoring
- Keine Compiler-Unterstützung für korrekte Reihenfolge

**Current Mitigation:** Funktioniert, aber fragil

**Vorschlag:**
- Interface-basierte Dependency Injection
- Conductor als ersten Service initialisieren
- Services registrieren sich beim Conductor via Interface

---

### 6. Reflection Usage in State Sync (Conductor)
**Datei:** [internal/conductor/conductor.go](../internal/conductor/conductor.go:158-290)
**Schweregrad:** CRITICAL
**Impact:** Runtime errors bei Refactoring, kein Compile-Time Safety

**Code:**
```go
// SyncRunningContainers - Uses REFLECTION to avoid circular dependency!
func (c *Conductor) SyncRunningContainers(dockerSvc interface{}, serverRepo interface{}) {
    // Reflection call to avoid import cycle
    dockerVal := reflect.ValueOf(dockerSvc)
    listMethod := dockerVal.MethodByName("ListRunningMinecraftContainers")

    // Call method via reflection
    results := listMethod.Call([]reflect.Value{})
    // ...
}
```

**Probleme:**
1. **Kein Compile-Time Safety** - Method names hardcoded: "ListRunningMinecraftContainers", "FindByID", "GetRAMMb"
2. **Runtime Panic-Risiko** - Method-Rename → Panic at runtime
3. **Schwer zu refactoren** - IDE kann Dependencies nicht erkennen
4. **Type-Safety verloren** - interface{} Parameter
5. **Debugging schwierig** - Stack traces durch Reflection

**Betroffene Methods:**
- `SyncRunningContainers()` - 4 Reflection-Calls
- `SyncQueuedServers()` - 2 Reflection-Calls

**Vorschlag:**
```go
// Define interfaces
type ContainerLister interface {
    ListRunningMinecraftContainers() ([]docker.ContainerInfo, error)
}

type ServerRepository interface {
    FindByID(id string) (*models.MinecraftServer, error)
}

// Inject via constructor
func NewConductor(containerLister ContainerLister, serverRepo ServerRepository) *Conductor {
    // ...
}
```

**Alternativen:**
- Event-basierte State Sync (MinecraftService publishes events → Conductor subscribes)
- Interface-Inversion (Conductor definiert Interfaces, Services implementieren)

---

### 7. Archive Worker nicht implementiert (LifecycleService)
**Datei:** [internal/service/lifecycle_service.go](../internal/service/lifecycle_service.go:36-37)
**Schweregrad:** CRITICAL
**Impact:** Server werden nie automatisch archiviert, Storage Costs steigen unbegrenzt

**Code:**
```go
func (s *LifecycleService) Start() {
    // Start Sleep Worker (runs every 5 minutes)
    go s.sleepWorker(5 * time.Minute)

    // Future: Archive Worker will run here too
    // go s.archiveWorker(1 * time.Hour)  // 🔴 TODO - NICHT IMPLEMENTIERT!
}
```

**Probleme:**
1. **Phase 3 (Archive) existiert in Models** - Aber kein Worker zum automatischen Transition
2. **48h-Regel aus CLAUDE.md** - "Stopped > 48h → archived" funktioniert NICHT
3. **Storage Costs** - Volumes bleiben auf NVMe, werden nie in Storage Box verschoben
4. **Free Tier Versprechen gebrochen** - "Archived servers are FREE" - aber nichts wird archiviert
5. **Code-Kommentar "Future"** - Seit wann TODO? Warum nicht implementiert?

**Impact:**
- User erwarten Archivierung nach 48h (laut Features-Doku)
- Storage Costs steigen unbegrenzt
- Free Tier gibt es faktisch nicht

**Vorschlag:**
```go
// Implement Archive Worker
func (s *LifecycleService) Start() {
    go s.sleepWorker(5 * time.Minute)
    go s.archiveWorker(1 * time.Hour)  // Check for servers stopped > 48h
}

func (s *LifecycleService) processArchiveTransitions() {
    fortyEightHoursAgo := time.Now().Add(-48 * time.Hour)

    var servers []models.MinecraftServer
    s.db.Where("status = ? AND lifecycle_phase = ? AND last_stopped_at < ?",
        models.StatusSleeping,
        models.PhaseSleep,
        fortyEightHoursAgo,
    ).Find(&servers)

    for _, server := range servers {
        // 1. Compress world to .tar.gz
        // 2. Upload to Hetzner Storage Box
        // 3. Delete container + volume from node
        // 4. Update status to archived, phase to PhaseArchive
        // 5. Publish billing event (phase change to archive = FREE)
    }
}
```

---

### 8. Storage Usage nicht getrackt (BillingService)
**Datei:** [internal/service/billing_service.go](../internal/service/billing_service.go:142)
**Schweregrad:** CRITICAL
**Impact:** Storage Billing fehlt komplett, User zahlen nicht für Storage

**Code:**
```go
func (s *BillingService) recordServerStartedInternal(server *models.MinecraftServer) error {
    event := &models.BillingEvent{
        ServerID:         server.ID,
        RAMMb:            server.RAMMb,
        StorageGB:        0, // 🔴 TODO: Calculate actual storage usage
        HourlyRateEUR:    hourlyRate,
    }

    s.db.Create(event)
}
```

**Probleme:**
1. **StorageGB hardcoded auf 0** - Seit wann? TODO wird nie behoben?
2. **Kein Storage Billing** - User zahlen nur für RAM, nicht für Storage
3. **Business Model unvollständig** - Laut PLAN.md soll Storage berechnet werden
4. **Archive-Phase benötigt Storage Tracking** - Sonst keine Kostenberechnung möglich

**Vorschlag:**
```go
// Get actual storage usage from Docker volume
storageGB := s.getServerStorageUsage(server)

event := &models.BillingEvent{
    StorageGB: storageGB,
    // ...
}
```

**Alternative:**
- Periodic Storage Scan (Background Worker)
- Per-Server Storage Metrics im Monitoring Service

---

### 9. Magic String Status Check (BackupService)
**Datei:** [internal/service/backup_service.go](../internal/service/backup_service.go:95)
**Schweregrad:** CRITICAL
**Impact:** Type-Safety verloren, Typo führt zu Runtime-Bug

**Code:**
```go
func (b *BackupService) RestoreBackup(serverID string, backupPath string) error {
    server := b.repo.FindByID(serverID)

    // Server must be stopped
    if server.Status != "stopped" {  // 🔴 Hardcoded string statt Constant!
        return fmt.Errorf("server must be stopped before restoring backup")
    }
    // ...
}
```

**Probleme:**
1. **Type-Safety verloren** - Sollte `models.StatusStopped` nutzen
2. **Typo-Risk** - "stoped" (1x p) würde kompilieren, aber zur Laufzeit failen
3. **Inconsistent mit Rest des Codes** - Überall sonst werden Constants genutzt
4. **Refactoring-unsafe** - IDE kann Status-Änderungen nicht tracken

**Impact:**
- Bei Typo: Backup Restore funktioniert nie (immer "server must be stopped" Error)
- Schwer zu debuggen

**Vorschlag:**
```go
if server.Status != models.StatusStopped {
    return fmt.Errorf("server must be stopped before restoring backup")
}
```

---

### 10. OAuth: Keine Transaction für User + OAuth Account Creation
**Datei:** [internal/service/oauth_service.go](../internal/service/oauth_service.go:394-427)
**Schweregrad:** CRITICAL
**Impact:** Orphaned Users in Datenbank bei OAuth-Linking-Fehler

**Code:**
```go
func (s *OAuthService) findOrCreateUser(...) (*models.User, bool, bool, error) {
    // Create new user if doesn't exist
    if user == nil {
        user = &models.User{
            Email: userInfo.Email,
            Password: generateRandomPassword(),  // ⚠️ Zufalls-PW für OAuth-only Users
        }

        if err := s.userRepo.Create(user); err != nil {  // ⚠️ Keine Transaction!
            return nil, false, false, err
        }
    }

    // Create OAuth account link
    newOAuthAccount := &models.OAuthAccount{
        UserID:   user.ID,
        Provider: provider,
        // ...
    }

    if err := s.db.Create(newOAuthAccount).Error; err != nil {  // ⚠️ Separate Operation!
        return nil, false, false, err  // ⚠️ User bleibt in DB, aber ohne OAuth-Link!
    }
}
```

**Probleme:**
1. **User wird zuerst erstellt** (Line 405)
2. **OAuth Account wird danach erstellt** (Line 427)
3. **Keine Transaction** - wenn OAuth-Account-Creation fehlschlägt:
   - User existiert in DB
   - ABER: Ohne OAuth-Link
   - User kann sich nicht einloggen (kein Password, nur OAuth)
   - User ist "orphaned"

**Impact:**
- Daten-Inkonsistenz
- Orphaned Users akkumulieren
- Kann zu Account-Problemen führen

**Vorschlag:**
```go
err := s.db.Transaction(func(tx *gorm.DB) error {
    // Create user
    if err := tx.Create(user).Error; err != nil {
        return err
    }

    // Create OAuth link (atomic)
    if err := tx.Create(newOAuthAccount).Error; err != nil {
        return err  // Rollback!
    }

    return nil
})
```

---

### 11. OAuth: Tokens in Plain Text gespeichert (SECURITY RISK!)
**Datei:** [internal/service/oauth_service.go](../internal/service/oauth_service.go:367-374, 421-422)
**Schweregrad:** CRITICAL
**Impact:** AccessToken & RefreshToken unverschlüsselt in Datenbank

**Code:**
```go
func (s *OAuthService) findOrCreateUser(...) {
    // Update existing OAuth account
    oauthAccount.AccessToken = tokenResp.AccessToken      // ⚠️ Plain text!
    oauthAccount.RefreshToken = tokenResp.RefreshToken    // ⚠️ Plain text!
    s.db.Save(&oauthAccount)
}
```

**Probleme:**
1. **AccessToken** wird unverschlüsselt in PostgreSQL gespeichert
2. **RefreshToken** wird unverschlüsselt gespeichert
3. Bei **Database Compromise** (SQL Injection, Backup Leak, etc.):
   - Angreifer hat Zugriff auf Discord/Google/GitHub Tokens
   - Kann OAuth-Provider-APIs im Namen der User nutzen
   - Kann User-Daten bei Providern abrufen
4. **GDPR/Datenschutz-Risiko** - Tokens sind PII

**Impact:**
- CRITICAL Security Vulnerability
- Möglicher Account-Takeover
- Datenleck bei Database Breach

**Vorschlag:**
```go
// Option 1: Encrypt tokens before storage
encryptedAccessToken := encryptToken(tokenResp.AccessToken, s.cfg.EncryptionKey)
encryptedRefreshToken := encryptToken(tokenResp.RefreshToken, s.cfg.EncryptionKey)

oauthAccount.AccessToken = encryptedAccessToken
oauthAccount.RefreshToken = encryptedRefreshToken

// Option 2: Hash tokens (if one-way is sufficient)
oauthAccount.AccessTokenHash = hashToken(tokenResp.AccessToken)

// Option 3: Don't store long-lived tokens, nur refresh on-demand
```

**Empfehlung:**
- AES-256-GCM Encryption
- Key Management via Environment Variable oder Vault
- Token-Rotation bei jedem Login

---

### 12. Remote Node Operations NICHT IMPLEMENTIERT (Recovery + Config)
**Dateien:**
- [internal/service/recovery_service.go:519-527](../internal/service/recovery_service.go:519-527)
- [internal/service/config_service.go:508-510](../internal/service/config_service.go:508-510)
**Schweregrad:** CRITICAL
**Impact:** Crash Recovery und Config Changes funktionieren NUR auf local-node

**Code (RecoveryService):**
```go
func (s *RecoveryService) restartContainer(server *models.MinecraftServer) bool {
    // ...

    if s.isLocalNode(server.NodeID) {
        // Start container locally
        s.dockerService.StartContainer(containerID)
    } else {
        // ⚠️ SAFEGUARD: Recovery not yet supported for remote nodes
        logger.Warn("Container recovery not yet supported for remote nodes")
        server.Status = models.StatusError
        return false  // ⚠️ Recovery FAILED!
    }
}
```

**Code (ConfigService):**
```go
func (s *ConfigService) applyChanges(...) error {
    if requiresRestart && wasRunning {
        // ⚠️ SAFEGUARD: Not yet supported for remote nodes
        if !s.isLocalNode(server.NodeID) {
            return fmt.Errorf("configuration changes requiring restart are not yet supported for remote servers (node: %s)", server.NodeID)
        }
    }
}
```

**Probleme:**
1. **Crash Recovery** funktioniert nur auf local-node (Hetzner Dedicated)
2. **Config Changes** funktionieren nicht auf remote nodes (Hetzner Cloud VMs)
3. **Auto-Scaling** funktioniert - aber Features sind eingeschränkt:
   - Server auf Cloud VMs können crashen → KEIN Auto-Recovery
   - Config ändern → ERROR
4. **Magic String Detection:** `isLocalNode(nodeID string) bool { return nodeID == "" || nodeID == "local-node" }`

**Impact:**
- **Production-kritisch:** Wenn Auto-Scaling aktiv ist (Hetzner Cloud VMs)
- Server auf Cloud-Nodes haben KEINE Crash-Recovery
- User können Config nicht ändern für Remote-Server
- Limitiert die Skalierbarkeit des Systems

**Warum existiert das Safeguard?**
- RecoveryService nutzt lokalen DockerService
- ConfigService nutzt lokalen DockerService
- Remote Docker Client ist separat (RemoteDockerClient Interface)
- Integration fehlt

**Vorschlag:**
```go
// RecoveryService sollte RemoteDockerClient nutzen
if s.isLocalNode(server.NodeID) {
    s.dockerService.StartContainer(containerID)
} else {
    remoteClient := s.dockerService.GetRemoteClient(server.NodeID)
    remoteClient.StartContainer(containerID)
}
```

---

## 🟡 MEDIUM - Sollte behoben werden

### 13. System Node Detection via String-Prefix
**Datei:** [internal/conductor/node_registry.go](../internal/conductor/node_registry.go:40-47)
**Schweregrad:** MEDIUM
**Impact:** Fehl-Klassifizierung von Nodes möglich

**Code:**
```go
func isSystemNodeByID(nodeID string) bool {
    return nodeID == "local-node" ||
           nodeID == "control-plane" ||
           nodeID == "proxy-node" ||
           (len(nodeID) >= 5 && nodeID[:5] == "local") ||
           (len(nodeID) >= 7 && nodeID[:7] == "control") ||
           (len(nodeID) >= 5 && nodeID[:5] == "proxy")
}
```

**Probleme:**
- **Prefix-basierte Erkennung ist fragil** - Worker-Node "proxyman-1" wird als System-Node erkannt
- **Keine explizite Kennzeichnung** - NodeType ist "local", "cloud", "dedicated" - wo ist "system"?
- **Verstreute Logik** - Node.IsSystemNode (bool) vs. isSystemNodeByID(string)
- **Naming Collisions** - User könnte versehentlich "local-worker-1" als Node-ID verwenden

**Impact:**
- System-Node wird als Worker-Node genutzt → API-Server bekommt MC-Container
- Worker-Node wird als System-Node klassifiziert → Keine Container-Allocation

**Vorschlag:**
- **Explicit Flag:** `Node.IsSystemNode` bei Node-Erstellung setzen
- **Enum für Type:** `NodeType = "system" | "worker" | "spare"`
- **Entferne String-Matching**

---

### 11. Hardcoded Scaling Thresholds
**Dateien:**
- [internal/conductor/policy_reactive.go](../internal/conductor/policy_reactive.go:34-40)
- [internal/conductor/scaling_engine.go](../internal/conductor/scaling_engine.go)
**Schweregrad:** MEDIUM
**Impact:** Scaling-Verhalten nicht konfigurierbar ohne Code-Änderung

**Code:**
```go
const (
    ScaleUpThreshold   = 85.0  // >85% capacity → provision VM
    ScaleDownThreshold = 30.0  // <30% capacity for 30min → decommission
    ScaleDownCooldown  = 30 * time.Minute
    MaxCloudNodes      = 10    // Safety limit
)
```

**Probleme:**
- **Nicht Environment-spezifisch** - Dev vs. Prod vs. Test brauchen andere Werte
- **Keine Runtime-Anpassung** - Code-Deploy nötig für Threshold-Änderung
- **Startup Delay hardcoded** - 2 Minuten (warum genau 2?)
- **Queue Check Interval hardcoded** - 30 Sekunden

**Vorschlag:**
```go
// .env
SCALING_SCALE_UP_THRESHOLD=85.0
SCALING_SCALE_DOWN_THRESHOLD=30.0
SCALING_SCALE_DOWN_COOLDOWN_MIN=30
SCALING_MAX_CLOUD_NODES=10
STARTUP_DELAY_MINUTES=2
QUEUE_CHECK_INTERVAL_SEC=30
```

---

### 12. Keine Error Recovery in Workers (Conductor)
**Datei:** [internal/conductor/conductor.go](../internal/conductor/conductor.go:107-151)
**Schweregrad:** MEDIUM
**Impact:** Worker-Crash → kein Auto-Restart

**Code:**
```go
func (c *Conductor) Start() {
    // 4 Background workers ohne Panic Recovery!
    go c.startupDelayWorker()           // ⚠️ No recovery
    go c.periodicQueueWorker()          // ⚠️ No recovery
    go c.reservationTimeoutWorker()     // ⚠️ No recovery
    go c.cpuMetricsWorker()             // ⚠️ No recovery
}
```

**Risiko:**
- Panic in Worker → Gesamte App crasht (Goroutine ist detached)
- Beispiel: Queue-Processor panic → Queue wird nie mehr abgearbeitet
- Startup-Delay-Worker panic → Nodes werden nie healthy

**Vorschlag:**
```go
func (c *Conductor) startWorkerWithRecovery(name string, workerFunc func()) {
    go func() {
        defer func() {
            if r := recover(); r != nil {
                logger.Error(fmt.Sprintf("Conductor worker '%s' panic", name),
                            fmt.Errorf("%v", r), map[string]interface{}{
                                "stack": string(debug.Stack()),
                            })
                // Optional: Auto-restart with exponential backoff
                time.Sleep(10 * time.Second)
                c.startWorkerWithRecovery(name, workerFunc)
            }
        }()
        workerFunc()
    }()
}
```

---

### 13. Queue Processor Race Condition Potential
**Datei:** [internal/conductor/conductor.go](../internal/conductor/conductor.go:107-151)
**Schweregrad:** MEDIUM
**Impact:** Doppelte Queue-Verarbeitung möglich

**Code:**
```go
func (c *Conductor) Start() {
    // Worker 1: One-time nach 2 Minuten
    go c.startupDelayWorker()    // → calls processStartQueue()

    // Worker 2: Periodic jede 30 Sekunden
    go c.periodicQueueWorker()   // → calls processStartQueue()
}
```

**Probleme:**
- **Beide Workers rufen processStartQueue()** - ohne Koordination
- Nach 2 Minuten könnten BEIDE gleichzeitig laufen
- StartQueue ist Thread-Safe (Mutex), aber:
  - Was wenn beide Dequeue() gleichzeitig aufrufen?
  - Könnte ein Server 2x gestartet werden?

**Current Mitigation:**
- StartQueue hat Mutex
- Dequeue() ist atomic
- MinecraftService prüft ob Server schon läuft

**Vorschlag:**
- **One-Shot Worker:** startupDelayWorker stoppt nach 1. Execution
- **ODER:** Mutex um gesamte processStartQueue()-Funktion

---

### 14. Worker-Node Sync REMOVED
**Datei:** [cmd/api/main.go](../cmd/api/main.go:265-266)
**Schweregrad:** MEDIUM
**Impact:** Nodes werden nach Restart nicht wiederhergestellt

**Code:**
```go
// NOTE: Worker-Node sync REMOVED - nodes are registered via ProvisionNode, not recovered
// This prevents node churn during container restarts
```

**Probleme:**
- Nach App-Restart: Nodes müssen manuell re-registriert werden
- Was wenn Node crashed und App crasht gleichzeitig?
- Begründung "prevents node churn" - aber welcher Churn genau?

**Risiko:** Capacity-Loss nach Restarts

**Vorschlag:**
- Node-Discovery-Mechanismus
- Oder: Persistente Node-Registration in DB mit TTL

---

### 15. Orphaned Server Cleanup deaktiviert
**Datei:** [cmd/api/main.go](../cmd/api/main.go:122-123)
**Schweregrad:** MEDIUM
**Impact:** Orphaned Servers bleiben länger existieren

**Code:**
```go
// Note: Orphaned server cleanup is NOT run on startup to avoid race conditions
// during container restarts. The monitoring service handles cleanup periodically.
```

**Probleme:**
- Nach Crash: Orphaned Servers bis zum nächsten Monitoring-Cycle
- Kein Zeitfenster für "periodically" angegeben
- Race Condition: Welche genau?

**Mitigation:** Monitoring Service macht es periodisch (aber wann?)

**Vorschlag:**
- Startup Delay (z.B. 30s warten, dann cleanup)
- Dokumentation WANN Monitoring Service cleanup macht

---

### 16. Dual Source of Truth für Tiers
**Datei:** [internal/models/tier.go](../internal/models/tier.go:27-66)
**Schweregrad:** MEDIUM
**Impact:** Inkonsistenz bei Config-Änderungen

**Code:**
```go
var StandardTiers = map[string]int{
    TierMicro:  2048,
    TierSmall:  4096,
    // ...
}

func ClassifyTier(ramMB int) string {
    // Check hardcoded map
    for tier, ram := range StandardTiers {
        if ramMB == ram { return tier }
    }

    // Check config (different values possible!)
    if ramMB == cfg.StandardTierMicro { return TierMicro }
    // ...
}
```

**Probleme:**
- Zwei Quellen der Wahrheit
- Config kann abweichen von `StandardTiers`
- Welche hat Priorität?

**Vorschlag:**
- NUR Config nutzen
- ODER: StandardTiers aus Config initialisieren beim Start

---

### 17. IP-Range Extraction zu simpel (Security)
**Datei:** [internal/models/security.go](../internal/models/security.go:63-72)
**Schweregrad:** MEDIUM
**Impact:** Device Fingerprinting funktioniert nicht korrekt für IPv6

**Code:**
```go
func extractIPRange(ip string) string {
    // Simple implementation: take first 3 parts of IPv4
    // For production, use proper IP parsing
    for i := len(ip) - 1; i >= 0; i-- {
        if ip[i] == '.' {
            return ip[:i] + ".0"
        }
    }
    return ip
}
```

**Probleme:**
- NUR IPv4
- IPv6 wird falsch behandelt
- Selbst als "For production, use proper IP parsing" markiert
- Aber: Code IST in Production!

**Vorschlag:**
- `net.ParseIP()` nutzen
- IPv4: /24 Subnet (first 3 octets)
- IPv6: /64 Subnet (first 4 groups)

---

### 18. Goroutines ohne Panic Recovery (WebSocket)
**Dateien:**
- [cmd/api/main.go:165](../cmd/api/main.go:165) - `go wsHub.Run()`
- [cmd/api/main.go:350](../cmd/api/main.go:350) - `go dashboardWs.Run()`
**Schweregrad:** MEDIUM
**Impact:** Panic in Goroutine → App-Crash

**Code:**
```go
wsHub := websocket.NewHub()
go wsHub.Run() // ⚠️ No panic recovery!
```

**Risiko:**
- Panic im WebSocket-Hub → gesamte App crasht
- Kein defer-Recovery möglich (Goroutine ist detached)

**Vorschlag:**
```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            logger.Error("WebSocket hub panic", fmt.Errorf("%v", r))
            // Optional: Restart wsHub.Run()
        }
    }()
    wsHub.Run()
}()
```

---

### 19. UsageLog vs. UsageSession Redundanz
**Dateien:**
- [internal/models/server.go:131](../internal/models/server.go:131) - `UsageLogs`
- [internal/models/billing.go:45](../internal/models/billing.go:45) - `UsageSession`
**Schweregrad:** MEDIUM
**Impact:** Verwirrung, potenzielle Datenduplikation

**Code:**
```go
// server.go
type MinecraftServer struct {
    // ...
    UsageLogs []UsageLog `gorm:"foreignKey:ServerID"`
}

type UsageLog struct {
    gorm.Model
    ServerID string
    StartedAt time.Time
    StoppedAt *time.Time
    DurationSeconds int
    CostEUR float64
    // ...
}

// billing.go
type UsageSession struct {
    gorm.Model
    ID string
    ServerID string
    StartedAt time.Time
    StoppedAt *time.Time
    DurationSeconds int
    CostEUR float64
    // ...
}
```

**Probleme:**
- Fast identische Strukturen
- Welches wird genutzt?
- Potenzielle Datenduplikation

**Vorschlag:**
- Eins von beiden deprecaten
- ODER: Klarstellen wofür was genutzt wird

---

### 20. RCON Default Password = "minecraft"
**Datei:** [internal/models/server.go](../internal/models/server.go:128)
**Schweregrad:** LOW-MEDIUM
**Impact:** Schwaches Default-Passwort

**Code:**
```go
RCONPassword string `gorm:"size:256;default:'minecraft'"`
```

**Probleme:**
- Hardcoded, bekanntes Passwort
- Security: Sollte random generiert werden
- Low Impact da RCON nur von localhost?

**Vorschlag:**
- Random Password bei Server-Erstellung
- Oder: UUID-basiert
- Minimum: Dokumentation dass User es ändern soll

---

### 21. Fat Interface - Interface Segregation Violation (Service Layer)
**Datei:** [internal/service/minecraft_service.go](../internal/service/minecraft_service.go:49-127)
**Schweregrad:** MEDIUM
**Impact:** Code schwer wartbar, Tests komplex

**Code:**
```go
// ConductorInterface has 20+ methods!
type ConductorInterface interface {
    CheckCapacity(requiredRAMMB int) (bool, int)
    AtomicAllocateRAM(ramMB int) bool
    ReleaseRAM(ramMB int)
    SelectNodeForContainerAuto(requiredRAMMB int) (string, error)
    AtomicAllocateRAMOnNode(nodeID string, ramMB int) bool
    ReleaseRAMOnNode(nodeID string, ramMB int)
    AtomicReserveStartSlot(...) bool
    ReleaseStartSlot(serverID string)
    RegisterContainer(...)
    GetContainer(...) (...)
    UpdateContainerStatus(...)
    EnqueueServer(...)
    IsServerQueued(...) bool
    RemoveFromQueue(...)
    ProcessStartQueue()
    TriggerScalingCheck()
    GetRemoteNode(...) (...)
    GetRemoteDockerClient() *docker.RemoteDockerClient
    GetNode(...) (...)
    CanStartServer(...) (...)
    // ... 20+ methods total!
}
```

**Probleme:**
1. **Interface Segregation Principle Violation** - Interface zu groß
2. **MinecraftService braucht nicht alle Methods** - Dependency Pollution
3. **Tests müssen alle 20+ Methods mocken** - Selbst wenn nur 3 genutzt werden
4. **Fragil bei Änderungen** - Interface-Änderung betrifft viele Stellen

**Vorschlag:**
```go
// Split into smaller, focused interfaces
type CapacityManager interface {
    SelectNodeForContainerAuto(requiredRAMMB int) (string, error)
    AtomicAllocateRAMOnNode(nodeID string, ramMB int) bool
    ReleaseRAMOnNode(nodeID string, ramMB int)
}

type QueueManager interface {
    EnqueueServer(serverID, serverName string, requiredRAMMB int, userID string)
    IsServerQueued(serverID string) bool
    RemoveFromQueue(serverID string)
}

type ContainerRegistry interface {
    RegisterContainer(...)
    GetContainer(...) (...)
    UpdateContainerStatus(...)
}

type ScalingTrigger interface {
    TriggerScalingCheck()
}

// MinecraftService only needs what it uses
type MinecraftService struct {
    capacityMgr    CapacityManager
    queueMgr       QueueManager
    containerReg   ContainerRegistry
    scalingTrigger ScalingTrigger
}
```

---

### 22. Deprecated Code nicht entfernt (Service Layer)
**Dateien:**
- [internal/service/minecraft_service.go](../internal/service/minecraft_service.go:35-40) - VelocityServiceInterface
- [internal/service/billing_service.go](../internal/service/billing_service.go:122-124) - RecordServerStarted
**Schweregrad:** MEDIUM
**Impact:** Code-Bloat, Verwirrung für Entwickler

**Code:**
```go
// minecraft_service.go
// DEPRECATED - use remoteVelocityClient
velocityService       VelocityServiceInterface
remoteVelocityClient  RemoteVelocityClientInterface // NEW

// billing_service.go
// DEPRECATED: This method is kept for backwards compatibility
// Billing events are now automatically created via Event-Bus subscription
func (s *BillingService) RecordServerStarted(server *models.MinecraftServer) error {
    return s.recordServerStartedInternal(server)
}
```

**Probleme:**
1. **Deprecated Code bleibt im Struct** - velocityService Field noch vorhanden
2. **Public Methods als DEPRECATED** - Wer ruft RecordServerStarted() noch auf?
3. **Keine Migration Timeline** - Wann wird deprecated Code entfernt?
4. **Dead Code** - Wird vermutlich nie aufgeräumt

**Vorschlag:**
- **Grep nach Nutzung:** `grep -r "RecordServerStarted" --exclude-dir=vendor`
- **Wenn nicht genutzt:** Sofort entfernen
- **Wenn noch genutzt:** Deprecation Warning loggen + 1 Release später entfernen

---

### 23. Keine Transaktionen bei Phase Changes (LifecycleService)
**Datei:** [internal/service/lifecycle_service.go](../internal/service/lifecycle_service.go:102-112)
**Schweregrad:** MEDIUM
**Impact:** Inconsistency bei Fehler, Billing könnte out-of-sync sein

**Code:**
```go
func (s *LifecycleService) processSleepTransitions() {
    for _, server := range servers {
        oldPhase := server.LifecyclePhase

        // Update status and lifecycle phase
        updates := map[string]interface{}{
            "status":          models.StatusSleeping,
            "lifecycle_phase": models.PhaseSleep,
        }

        err := s.db.Model(&server).Updates(updates).Error  // ⚠️ NO TRANSACTION!
        if err != nil {
            continue
        }

        // Publish phase change event for billing tracking
        events.PublishBillingPhaseChanged(...)  // ⚠️ Separate operation!
    }
}
```

**Probleme:**
1. **DB Update + Event Publish nicht atomic** - Bei Event-Fehler: Server in Sleep, Billing nicht benachrichtigt
2. **Billing out-of-sync** - User zahlt für falschen Lifecycle-Phase
3. **Keine Rollback-Möglichkeit** - Bei Fehler bleibt inkonsistenter State

**Vorschlag:**
```go
tx := s.db.Begin()

err := tx.Model(&server).Updates(updates).Error
if err != nil {
    tx.Rollback()
    continue
}

// Publish event
err = events.PublishBillingPhaseChanged(...)
if err != nil {
    tx.Rollback()  // Rollback DB update if event fails
    logger.Error("Failed to publish billing event, rolling back", err)
    continue
}

tx.Commit()
```

**Alternative:**
- Event-Publishing als GORM Hook (AfterUpdate) - automatisch innerhalb Transaction

---

### 24. Hardcoded Worker Intervals (LifecycleService)
**Datei:** [internal/service/lifecycle_service.go](../internal/service/lifecycle_service.go:34-37)
**Schweregrad:** MEDIUM
**Impact:** Nicht environment-spezifisch konfigurierbar

**Code:**
```go
func (s *LifecycleService) Start() {
    // Hardcoded intervals
    go s.sleepWorker(5 * time.Minute)  // ⚠️ Why 5 minutes?

    // Future: Archive Worker
    // go s.archiveWorker(1 * time.Hour)  // ⚠️ Why 1 hour?
}
```

**Probleme:**
1. **5 Minuten hardcoded** - Warum genau 5? Empirisch? Arbitrary?
2. **1 Stunde für Archive** - Was wenn man schneller archivieren will?
3. **Nicht Dev/Prod unterscheidbar** - Dev braucht schnellere Intervalle für Testing
4. **Keine Runtime-Anpassung** - Code-Deploy nötig für Interval-Änderung

**Vorschlag:**
```go
// .env
LIFECYCLE_SLEEP_WORKER_INTERVAL_MIN=5
LIFECYCLE_ARCHIVE_WORKER_INTERVAL_MIN=60

// lifecycle_service.go
func NewLifecycleService(cfg *config.Config, ...) *LifecycleService {
    return &LifecycleService{
        sleepInterval:   cfg.LifecycleSleepInterval,
        archiveInterval: cfg.LifecycleArchiveInterval,
    }
}
```

---

### 25. Keine Panic Recovery in Background Workers (MonitoringService)
**Datei:** [internal/service/monitoring_service.go](../internal/service/monitoring_service.go:60-65)
**Schweregrad:** MEDIUM
**Impact:** Worker-Crash → Service inoperabel, keine Auto-Shutdown-Funktionalität

**Code:**
```go
func (m *MonitoringService) Start() {
    go m.scanRunningServers()  // ⚠️ Keine Panic Recovery!
    go m.monitorLoop()          // ⚠️ Keine Panic Recovery!
}
```

**Vorschlag:**
```go
func (m *MonitoringService) Start() {
    go func() {
        defer func() {
            if r := recover(); r != nil {
                logger.Error("scanRunningServers panicked", "error", r)
            }
        }()
        m.scanRunningServers()
    }()
    // Same for monitorLoop
}
```

---

### 26. Hardcoded Monitoring Interval (MonitoringService)
**Datei:** [internal/service/monitoring_service.go](../internal/service/monitoring_service.go:71-74)
**Schweregrad:** MEDIUM
**Impact:** 60-Sekunden-Intervall nicht anpassbar ohne Code-Änderung

**Code:**
```go
func (m *MonitoringService) monitorLoop() {
    ticker := time.NewTicker(60 * time.Second)  // ⚠️ Hardcoded!
    // ...
}
```

**Vorschlag:**
- Config: `MONITORING_INTERVAL=60`
- Constructor Parameter
- Sinnvolle Werte: 30s (responsive) bis 120s (reduced load)

---

### 27. RCON-Fehler werden ignoriert (MonitoringService)
**Datei:** [internal/service/monitoring_service.go](../internal/service/monitoring_service.go:176-181)
**Schweregrad:** MEDIUM
**Impact:** Fehlerhafte Player-Count-Detection führt zu falschen Auto-Shutdown-Entscheidungen

**Code:**
```go
func (m *MonitoringService) getPlayerCount(server *models.MinecraftServer) (int, error) {
    playerCount, err := m.dockerService.GetPlayerCount(server.ID, server.NodeID, server.RCONPort)
    if err != nil {
        return 0, nil  // ⚠️ Fehler wird ignoriert, gibt 0 zurück!
    }
    return playerCount, nil
}
```

**Impact:**
- RCON-Timeouts → Server wird fälschlicherweise als "leer" erkannt → Auto-Shutdown trotz Spielern
- Falsche Idle-Time-Berechnungen

**Vorschlag:**
- Fehler propagieren und im Caller loggen
- Oder: Retry-Logic (3 Versuche mit 5s Delay)
- Oder: Grace Period (nur nach 3 aufeinanderfolgenden RCON-Erfolgen Auto-Shutdown)

---

### 28. Save-Fehler wird ignoriert (BackupService)
**Datei:** [internal/service/backup_service.go](../internal/service/backup_service.go:142-146)
**Schweregrad:** MEDIUM
**Impact:** Inkonsistente Backups wenn Save-Command fehlschlägt

**Code:**
```go
func (b *BackupService) CreateBackup(serverID string) (string, error) {
    // 1. Save world via RCON
    err = b.rconClient.SendCommand(serverID, "save-all")
    if err != nil {
        logger.Warn("Failed to send save-all", "error", err)
        // ⚠️ Fortsetzung trotz Fehler!
    }

    // 2. ZIP world directory
    // ...
}
```

**Impact:**
- Backup enthält möglicherweise unsaved chunks
- Rollback führt zu Datenverlust

**Vorschlag:**
```go
if err != nil {
    return "", fmt.Errorf("failed to save world before backup: %w", err)
}
```

---

### 29. Backup Directory nicht konfigurierbar (BackupService)
**Datei:** [internal/service/backup_service.go](../internal/service/backup_service.go:15-18)
**Schweregrad:** MEDIUM
**Impact:** Hardcoded Pfad, keine Flexibilität für verschiedene Storage-Backends

**Code:**
```go
func NewBackupService(...) *BackupService {
    return &BackupService{
        backupDir: "./backups",  // ⚠️ Hardcoded!
        // ...
    }
}
```

**Vorschlag:**
- Config: `BACKUP_DIR=/mnt/hetzner-storage-box/backups`
- Ermöglicht: Network Storage, S3, etc.

---

### 30. Keine Transaction bei Login (AuthService)
**Datei:** [internal/service/auth_service.go](../internal/service/auth_service.go:85-92)
**Schweregrad:** MEDIUM
**Impact:** Race Condition zwischen FailedLoginCount-Update und SecurityEvent-Log

**Code:**
```go
func (s *AuthService) Login(email, password, userAgent, ipAddress string) (string, *models.User, bool, error) {
    if !user.CheckPassword(password) {
        lockDuration := user.IncrementFailedLogins()
        s.userRepo.Update(user)  // ⚠️ Keine Transaction!

        s.securityService.LogSecurityEvent(models.SecurityEvent{
            UserID:    user.ID,
            EventType: models.SecurityEventFailedLogin,
            // ...
        })  // ⚠️ Separate DB-Operation!
    }
}
```

**Impact:**
- User-Update kann erfolgreich sein, SecurityEvent-Log fehlschlagen → inkonsistente Audit-Logs
- Keine Atomicity zwischen FailedLogins-Counter und Event-Log

**Vorschlag:**
```go
err := s.db.Transaction(func(tx *gorm.DB) error {
    if err := tx.Save(user).Error; err != nil {
        return err
    }
    return s.securityService.LogSecurityEventTx(tx, event)
})
```

---

## 🟢 LOW - Nice to have

### 32. Hardcoded MaxPlayers = 20 (MinecraftService)
**Datei:** [internal/service/minecraft_service.go](../internal/service/minecraft_service.go:196)
**Schweregrad:** LOW
**Impact:** User kann MaxPlayers nicht beim Create anpassen

**Code:**
```go
func (s *MinecraftService) CreateServer(...) (*models.MinecraftServer, error) {
    server := &models.MinecraftServer{
        MaxPlayers: 20,  // ⚠️ Hardcoded!
        // ...
    }
}
```

**Vorschlag:**
- Parameter in CreateServer() API
- Oder: Tier-based Defaults (Micro=10, Small=20, Medium=50, etc.)

---

### 33. Hardcoded Timeouts
**Datei:** [cmd/api/main.go](../cmd/api/main.go)
**Schweregrad:** LOW
**Impact:** Nicht konfigurierbar

**Beispiele:**
- Conductor Health Check: `10*time.Second` (Zeile 228)
- Prometheus Collector: `30*time.Second` (Zeile 224)
- Plugin Sync: "every 6h" (hardcoded im Service)

**Vorschlag:**
- Config-basierte Timeouts
- Environment Variables

---

### 34. Log Level Parsing Case-Sensitive
**Datei:** [cmd/api/main.go](../cmd/api/main.go:386-400)
**Schweregrad:** LOW
**Impact:** User-Verwirrung

**Code:**
```go
func parseLogLevel(level string) logger.LogLevel {
    switch strings.ToUpper(level) { // ⚠️ ToUpper macht es case-insensitive
        case "DEBUG": return logger.DEBUG
        case "INFO": return logger.INFO
        // ...
        default: return logger.INFO
    }
}
```

**Actually:** Code ist OK (`strings.ToUpper`), aber könnte besser dokumentiert sein.

---

### 35. ServerTemplate nicht in DB
**Datei:** [internal/models/template.go](../internal/models/template.go)
**Schweregrad:** LOW
**Impact:** Keine Versionierung, Deployment-Risiko

**Probleme:**
- Templates aus JSON-Datei geladen
- Keine DB-Persistenz
- Keine Versionierung
- Risiko: Deployment ohne Templates-Datei

**Vorschlag:**
- Templates in DB speichern
- Admin-UI für Template-Verwaltung
- Seeding via JSON als Backup

---

### 36. Keine JSONB-Schema-Validierung
**Dateien:** Alle Models mit `datatypes.JSON`
**Schweregrad:** LOW
**Impact:** Invalide JSON in DB möglich

**Beispiele:**
- `PluginVersion.MinecraftVersions`
- `PluginVersion.ServerTypes`
- `PluginVersion.Dependencies`
- `SecurityEvent.Metadata`
- `SystemEvent.Data`

**Vorschlag:**
- JSON-Schema-Validierung vor DB-Insert
- Oder: Type-safe Wrapper-Strukturen mit Marshal/Unmarshal

---

### 37. Keine Prometheus-Metrics für Scaling-Decisions
**Datei:** [internal/conductor/scaling_engine.go](../internal/conductor/scaling_engine.go)
**Schweregrad:** LOW
**Impact:** Scaling-Entscheidungen schwer zu monitoren

**Probleme:**
- Scaling-Decisions werden nur geloggt (nicht als Metrics)
- Keine Prometheus-Counters für:
  - `scaling_decisions_total{action="scale_up|scale_down"}`
  - `scaling_failures_total`
  - `scaling_recommendation_urgency`
  - `policy_evaluation_duration_seconds`
- Dashboard zeigt nur letzte 200 Log-Entries (DebugLogBuffer)

**Vorschlag:**
```go
var (
    scalingDecisions = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "conductor_scaling_decisions_total",
            Help: "Total scaling decisions by action and policy",
        },
        []string{"action", "policy", "urgency"},
    )
)
```

---

### 38. Magic Numbers ohne Dokumentation (Conductor)
**Datei:** [internal/conductor/](../internal/conductor/)
**Schweregrad:** LOW
**Impact:** Code-Verständnis erschwert

**Beispiele:**
- **2 Minuten** - Startup Delay (warum genau 2? Cloud-Init-Zeit? Empirisch ermittelt?)
- **30 Sekunden** - Queue Check Interval (warum 30? Trade-off zwischen Latency und Load?)
- **50MB per 10 containers** - Dynamic System Reserve (Berechnung? Empirie?)
- **15 Minuten** - Minimum Idle Time für Consolidation (warum 15?)
- **30 Minuten** - Minimum Uptime für Consolidation (warum 30?)
- **€0.10/hour** - Minimum Savings für Consolidation (Break-even-Point?)

**Vorschlag:**
- Inline-Kommentare mit Begründungen
- Config-basierte Defaults mit Kommentaren in .env.example
- Design-Doc für Threshold-Werte

---

### 39. TODO Comments in Production Code (Conductor)
**Dateien:**
- [internal/conductor/scaling_engine.go](../internal/conductor/scaling_engine.go:56-57)
**Schweregrad:** LOW
**Impact:** Tech Debt nicht getrackt

**Code:**
```go
// TODO B6: Register SparePoolPolicy when implemented
// TODO B7: Register PredictivePolicy when implemented
```

**Probleme:**
- TODOs im Code statt GitHub Issues
- Keine Priorisierung/Timeline
- "B6", "B7" verweisen auf nicht-existente Dokumentation

**Vorschlag:**
- GitHub Issues erstellen für geplante Features
- Code-Kommentar → Issue-Link
```go
// Planned: SparePoolPolicy (see issue #123)
// Planned: PredictivePolicy (see issue #124)
```

---

## Zusammenfassung

**Gesamt:** 42 Issues
- 🔴 Critical: 12
- 🟡 Medium: 21
- 🟢 Low: 9

**Top 12 CRITICAL Prioritäten:**
1. **Email Service aus MOCK MODE** (#1) - 2h - Production-Blocker
2. **User-Relations aktivieren** (#2) - 4h - Daten-Integrität
3. **OwnerID Default fixen** (#3) - 3h - Multi-Tenancy
4. **Keine Partial Failure Handling** (#4) - 3h - Bootstrap-Robustheit
5. **Circular Dependencies** (#5) - 4h - Architektur
6. **Reflection in State Sync** (#6) - 6h - Code-Stabilität
7. **Archive Worker nicht implementiert** (#7) - 8h - Storage Costs & Free Tier
8. **Storage Usage nicht getrackt** (#8) - 4h - Billing unvollständig
9. **Magic String Status Check** (#9) - 2h - BackupService Type Safety
10. **OAuth: Keine Transaction für User Creation** (#10) - 3h - Data Consistency
11. **OAuth: Tokens in Plain Text** (#11) - 4h - SECURITY RISK! 🔥
12. **Remote Node Operations NICHT IMPLEMENTIERT** (#12) - 12h - Auto-Scaling Limitierung

**Kritische Business & Security Impacts:**
- **#11: SECURITY VULNERABILITY!** OAuth-Tokens unverschlüsselt in DB → Account Takeover möglich
- **#12: Auto-Scaling eingeschränkt!** Crash Recovery + Config Changes funktionieren NUR auf local-node
- **#10: Data Consistency!** Orphaned OAuth Users bei Fehler
- #7 + #8: **Free Tier existiert faktisch nicht!** Archive-Feature versprochen aber nicht implementiert
- #1: Keine Emails in Production (User-Verifizierung kaputt)
- #2 + #3: Multi-Tenancy unvollständig (alle Server haben "default" als Owner)

**Top Medium Priorities (neu aus Service-Layer-Analyse):**
- Panic Recovery in Workers (#25) - Service-Stabilität
- Hardcoded Monitoring Interval (#26) - Konfigurierbarkeit
- RCON-Fehler ignoriert (#27) - Falsche Auto-Shutdown-Entscheidungen
- Save-Fehler ignoriert (#28) - Inkonsistente Backups
- Backup Directory hardcoded (#29) - Storage-Flexibilität
- Keine Transaction bei Login (#30) - Race Conditions
- **NEU: Security-Service-Issues** - Device renewal error, cleanup worker missing
- **NEU: OAuth-Service-Issues** - AuthService inline instantiation, cleanup workers missing
- **NEU: Recovery-Service-Issues** - No panic recovery, magic strings, no retry limit
- **NEU: Config-Service-Issues** - Magic strings, backup failure handling

**Previous Medium Priorities:**
- Fat Interface (#20) - Refactoring nötig
- Deprecated Code (#21) - Code-Bloat
- Phase Change Transactions (#22) - Billing Consistency
- Worker Intervals (#23) - Config-basiert machen

**Letzte Aktualisierung:** 2025-11-13 (Service-Layer-Analyse: 10/27 Services analysiert)
**Quelle:** Live-Code-Analyse (00-SUMMARY, 01-ENTRY_POINTS, 02-DATA_MODELS, 03-DATABASE_LAYER, 04-BUSINESS_LOGIC, 05-CONDUCTOR_CORE)

---

**Nächste Schritte:**
- Service-Layer-Analyse (04-BUSINESS_LOGIC.md) → weitere Issues erwartet
- HTTP API Layer (06-HTTP_API.md)
- Docker Integration (07-DOCKER_INTEGRATION.md)
- Continuous Updates während der Analyse
