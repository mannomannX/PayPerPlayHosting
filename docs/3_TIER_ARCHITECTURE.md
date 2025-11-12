# PayPerPlay 3-Tier Architecture
**True Pay-Per-Use: Von 70€/Monat Fixkosten zu 7€/Monat Baseline**

**Status:** Planned Architecture
**Current:** Monolith (All-in-One Dedicated Server)
**Target:** 3-Tier Microservices (Control + Proxy + Workload)
**Date:** 2025-11-11

---

## Problem mit der aktuellen Architektur

### Aktueller Zustand: Monolith
```
┌─────────────────────────────────────────────────────────┐
│ HETZNER DEDICATED SERVER (91.98.202.235)                │
│ 70€/month - ALWAYS ON                                   │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌────────────────────────────────────────────────┐     │
│  │  API (Go) + PostgreSQL + Velocity + MC-Server │     │
│  │  Alles in einem Container/Server              │     │
│  └────────────────────────────────────────────────┘     │
│                                                          │
│  PROBLEME:                                              │
│  ❌ Website-Traffic blockiert Spieler-Traffic           │
│  ❌ MC-Server können nicht unabhängig skalieren         │
│  ❌ Velocity restart = gesamtes System offline          │
│  ❌ 70€/month Fixkosten auch bei 0 Spielern             │
│  ❌ Nicht skalierbar über einen Server hinaus           │
│                                                          │
└─────────────────────────────────────────────────────────┘

Kosten bei 0 Last: 70€/month ❌
Kosten bei Peak Last: 70€/month ❌
Skalierbarkeit: KEINE ❌
```

### PayPerPlay Business Model ≠ Always-On Server

**PayPerPlay Prinzip:**
- Spieler zahlen nur für aktive Spielzeit
- Server stoppen bei Inaktivität
- **0 Spieler = 0 Kosten** (für Spieler)

**Aktuelles System:**
- **0 Spieler = 70€ Fixkosten** (für uns!)
- Wir zahlen 24/7 für ungenutzte Kapazität
- Unprofitabel bei niedriger Auslastung

---

## Lösung: 3-Tier Architektur

### Architektur-Übersicht

```
┌─────────────────────────────────────────────────────────┐
│ TIER 1: CONTROL PLANE (Always-On, Minimal)             │
│ Hetzner Cloud CX11: 2GB RAM, 1 vCPU                    │
│ Kosten: 3.50€/month (24/7)                             │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌──────────────────────────────────────────────────┐   │
│  │ API Server (Go Binary)                           │   │
│  │  - REST API für Dashboard/Mobile App            │   │
│  │  - User Management, Authentication               │   │
│  │  - Billing & Payment Processing                  │   │
│  │  - Orchestrierung (Conductor)                    │   │
│  │  RAM: ~50-100 MB                                 │   │
│  └──────────────────────────────────────────────────┘   │
│                                                          │
│  ┌──────────────────────────────────────────────────┐   │
│  │ PostgreSQL (Container)                           │   │
│  │  - User-Daten, Server-Konfiguration             │   │
│  │  - Billing History, Analytics                    │   │
│  │  RAM: ~200-400 MB                                │   │
│  └──────────────────────────────────────────────────┘   │
│                                                          │
│  ┌──────────────────────────────────────────────────┐   │
│  │ Dashboard/Website (Nginx + Static Files)        │   │
│  │  - Frontend (React/Vue)                          │   │
│  │  - Admin Panel                                   │   │
│  │  RAM: ~50 MB                                     │   │
│  └──────────────────────────────────────────────────┘   │
│                                                          │
│  Funktion: Orchestrierung, Verwaltung, Abrechnung      │
│  Traffic: NIEDRIG (nur API calls, keine Spieler)       │
│  Ports: 8000 (API), 80/443 (Website)                   │
│                                                          │
└─────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────┐
│ TIER 2: PROXY LAYER (Always-On, Isolated)              │
│ Hetzner Cloud CX11: 2GB RAM, 1 vCPU                    │
│ Kosten: 3.50€/month (24/7)                             │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌──────────────────────────────────────────────────┐   │
│  │ Velocity Proxy (Minecraft BungeeCord)            │   │
│  │  - Spieler-Routing zu Backend MC-Servern        │   │
│  │  - Single Entry Point für alle Spieler          │   │
│  │  - Hot-Reload von Server-Liste                  │   │
│  │  RAM: ~500-800 MB (je nach Spielerzahl)         │   │
│  └──────────────────────────────────────────────────┘   │
│                                                          │
│  Funktion: Spieler-Routing, Load Balancing             │
│  Traffic: HOCH (alle Minecraft-Verbindungen)           │
│  Port: 25565 (Minecraft Default)                       │
│                                                          │
│  WARUM ISOLIERT?                                        │
│  ✓ Website-Traffic kollidiert NICHT mit Spielern       │
│  ✓ API restart beeinflusst Spieler NICHT               │
│  ✓ Unabhängiges Monitoring/Scaling                     │
│                                                          │
└─────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────┐
│ TIER 3: WORKLOAD LAYER (100% On-Demand)                │
│ Hetzner Cloud VMs: Dynamic Scaling                     │
│ Kosten: 0€ bei 0 Last, X€ bei aktiver Nutzung          │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐        │
│  │ MC-Server  │  │ MC-Server  │  │ MC-Server  │        │
│  │ (CX21)     │  │ (CX31)     │  │ (CX31)     │        │
│  │ 4GB RAM    │  │ 8GB RAM    │  │ 8GB RAM    │        │
│  └────────────┘  └────────────┘  └────────────┘        │
│                                                          │
│  Funktion: Minecraft Server Execution                   │
│  Lifecycle:                                             │
│    1. START: Bei erstem Spieler-Connect                 │
│    2. RUN: Solange Spieler online                       │
│    3. STOP: Nach 5 Min Idle (keine Spieler)             │
│    4. DESTROY: VM wird dekommissioniert                 │
│                                                          │
│  Kommunikation:                                         │
│    - Registrierung bei Velocity (via Control Plane)     │
│    - Health Checks an Control Plane                     │
│    - Metrics/Logs an Monitoring                         │
│                                                          │
│  Kosten: PAY-PER-USE                                    │
│    - 0€ wenn keine Server laufen                        │
│    - 0.01€/h pro CX21 (2 vCPU, 4GB)                     │
│    - 0.02€/h pro CX31 (2 vCPU, 8GB)                     │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

---

## Warum diese Trennung?

### Separation of Concerns

#### 1. Traffic-Isolation

**Problem (Monolith):**
```
Website-Request → Server (überlastet) → Langsame MC-Spieler
MC-Spieler → Server (überlastet) → Langsames Dashboard
```

**Lösung (3-Tier):**
```
Website-Request → Tier 1 (API) ✓ Unabhängig
MC-Spieler → Tier 2 (Velocity) ✓ Isoliert
MC-Server → Tier 3 (VMs) ✓ Skaliert automatisch
```

#### 2. Unabhängige Skalierung

**Tier 1 (Control Plane):**
- Benötigt KEINE Skalierung (1 CX11 reicht für 10.000+ User)
- API ist stateless (Go binary, sehr effizient)
- PostgreSQL-Load minimal (nur Writes bei User-Aktionen)

**Tier 2 (Proxy):**
- Skaliert bei >1000 gleichzeitigen Spielern (erst nach Monaten!)
- Horizontal skalierbar (mehrere Velocity-Instanzen)
- Single Point of Entry (DNS Load Balancing)

**Tier 3 (Workload):**
- Skaliert bei jedem neuen MC-Server (jetzt!)
- 100% automatisch via ScalingEngine
- Unbegrenzt skalierbar (Hetzner Cloud hat 1000+ VMs)

#### 3. Kosten-Optimierung

**Aktuell:**
```
70€/month egal wie viel genutzt wird
```

**Mit 3-Tier:**
```
Tier 1: 3.50€/month (immer)
Tier 2: 3.50€/month (immer)
Tier 3: 0€ bei 0 Last, ~10-50€ bei Peaks

Total: 7-57€/month (je nach Nutzung)
Durchschnitt: ~15-20€/month
Einsparung: 50-55€/month (71-78%)
```

#### 4. Ausfallsicherheit

**Monolith:**
```
API crash → Alles offline ❌
Velocity crash → Alles offline ❌
MC-Server bug → Potenziell alles offline ❌
```

**3-Tier:**
```
API crash → Dashboard offline, aber Spieler spielen weiter ✓
Velocity crash → MC-Server laufen, werden neu registriert ✓
MC-Server bug → Nur dieser Server betroffen, Rest läuft ✓
```

---

## Technische Details

### Tier 1: Control Plane

#### Komponenten

**API Server (Go)**
```go
// Hauptfunktionen:
- User Authentication & Authorization (JWT)
- Server CRUD Operations (Create, Read, Update, Delete)
- Billing & Payment Integration (Stripe/PayPal)
- Conductor Orchestration (VM Management)
- ScalingEngine Control (Auto-Scaling)
- Health Monitoring (alle Tiers)

// Ressourcen:
RAM: 50-100 MB (Go ist sehr effizient)
CPU: <5% bei normalem Traffic
Disk: 500 MB (Binary + Logs)
```

**PostgreSQL**
```sql
-- Schema:
- users (accounts, auth)
- servers (minecraft server configs)
- billing (transactions, usage tracking)
- nodes (fleet management)
- events (audit log, analytics)

-- Ressourcen:
RAM: 200-400 MB (klein, wenige Writes)
CPU: <5% (keine komplexen Queries)
Disk: 5-10 GB (Datenbank + Backups)
```

**Nginx + Frontend**
```nginx
# Static Files:
- React/Vue Dashboard (~10 MB)
- Admin Panel (~5 MB)
- Assets (images, fonts)

# Ressourcen:
RAM: 50 MB (Nginx ist minimal)
CPU: <1%
Disk: 100 MB
```

**Gesamt:**
```
RAM: ~300-550 MB (von 2048 MB = 15-27% genutzt)
CPU: <10% durchschnittlich
Reserve: 1.5 GB RAM frei für Peaks
```

#### Netzwerk-Konfiguration

```yaml
# docker-compose.control-plane.yml
services:
  api:
    image: payperplay/api:latest
    ports:
      - "8000:8000"  # REST API
    environment:
      - DATABASE_URL=postgresql://postgres:5432/payperplay
      - VELOCITY_API_URL=http://velocity-vm:8080
      - HETZNER_CLOUD_TOKEN=${HETZNER_CLOUD_TOKEN}
    networks:
      - control-net

  postgres:
    image: postgres:16-alpine
    volumes:
      - postgres-data:/var/lib/postgresql/data
    networks:
      - control-net

  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./frontend/dist:/usr/share/nginx/html
    networks:
      - control-net

networks:
  control-net:
    driver: bridge
```

---

### Tier 2: Proxy Layer

#### Velocity Proxy

**Konfiguration:**
```yaml
# velocity.toml
[servers]
try = []  # Dynamisch gefüllt via API

[forced-hosts]
"play.payperplay.host" = []  # Alle Server

[advanced]
compression-threshold = 256
compression-level = -1
login-ratelimit = 3000
connection-timeout = 5000
read-timeout = 30000
```

**Remote API für dynamische Server-Registrierung:**
```go
// Velocity Plugin: VelocityRemoteAPI
// Lauscht auf Port 8080 für HTTP Requests

// POST /api/servers
// Body: {"name": "survival-1", "address": "10.0.1.5:25566"}
func RegisterServer(w http.ResponseWriter, r *http.Request) {
    var server ServerRegistration
    json.NewDecoder(r.Body).Decode(&server)

    // Füge Server zur Velocity-Config hinzu
    proxyServer.getServer(server.Name).ifPresent(s ->
        s.setAddress(server.Address)
    )

    // Reload Velocity Config (kein Restart!)
    proxyServer.reloadConfiguration()
}
```

**Control Plane Integration:**
```go
// internal/velocity/remote_client.go
type RemoteVelocityClient struct {
    apiURL string  // http://velocity-vm:8080
}

func (c *RemoteVelocityClient) RegisterServer(name, address string) error {
    payload := map[string]string{
        "name":    name,
        "address": address,
    }

    resp, err := http.Post(
        c.apiURL + "/api/servers",
        "application/json",
        toJSON(payload),
    )

    return handleResponse(resp, err)
}
```

**Ressourcen:**
```
RAM: 500-800 MB (je nach Spielerzahl)
  - 500 MB base
  - +1 MB pro 10 gleichzeitige Spieler
  - Max 2 GB (bei 1500+ Spielern)

CPU: 10-30% (Packet Forwarding)
Netzwerk: HOCH (alle MC-Traffic)
  - Eingehend: ~1-5 Mbit/s pro Spieler
  - Ausgehend: ~1-5 Mbit/s pro Spieler
```

#### Warum isoliert?

1. **Performance:** API-Traffic beeinflusst MC-Latency NICHT
2. **Skalierung:** Velocity kann unabhängig hochskaliert werden
3. **Monitoring:** Separate Metrics für API vs. MC-Traffic
4. **Debugging:** Velocity-Logs separat von API-Logs

---

### Tier 3: Workload Layer

#### On-Demand MC-Server

**Provisioning Flow:**
```
1. User klickt "Server starten" im Dashboard
   └─> POST /api/servers/:id/start

2. Control Plane (Conductor) prüft Kapazität
   └─> NodeRegistry.FindBestNode(ramMB)

3a. FALL 1: Node mit Kapazität vorhanden
    └─> StartContainerRemote(node, server)

3b. FALL 2: Keine Kapazität (>85% ausgelastet)
    └─> ScalingEngine.ProvisionNode()
    └─> Hetzner Cloud API: Create VM (CX21/CX31)
    └─> Wait for ready (~2 minutes)
    └─> StartContainerRemote(newNode, server)

4. Container startet auf Remote-VM
   └─> SSH oder Docker API über Netzwerk

5. Server registriert bei Velocity
   └─> POST http://velocity-vm:8080/api/servers
   └─> Body: {name: "survival-1", address: "10.0.1.5:25566"}

6. User kann connecten via play.payperplay.host
   └─> Velocity routet zu 10.0.1.5:25566
```

**Container Management (Remote):**
```go
// internal/conductor/remote_docker.go
type RemoteDockerClient struct {
    sshClient *ssh.Client
}

func (c *RemoteDockerClient) StartContainer(node *Node, server *Server) error {
    // Option 1: Docker API über SSH Tunnel
    dockerCmd := fmt.Sprintf(
        "docker run -d --name mc-%s -p %d:25565 -m %dM %s",
        server.ID,
        server.Port,
        server.RAMMB,
        server.ImageName,
    )

    // SSH Execute
    session, _ := c.sshClient.NewSession()
    defer session.Close()

    output, err := session.CombinedOutput(dockerCmd)

    // Option 2: Docker Remote API (TCP)
    // client := docker.NewClient("tcp://10.0.1.5:2375")
    // client.ContainerCreate(...)

    return err
}
```

**Auto-Stop bei Idle:**
```go
// internal/monitoring/activity_monitor.go
func (m *ActivityMonitor) CheckIdleServers() {
    for _, server := range m.getRunningServers() {
        // Prüfe Spieler-Count via Velocity API
        playerCount := m.velocityClient.GetPlayerCount(server.Name)

        if playerCount == 0 {
            idleTime := time.Since(server.LastActivity)

            if idleTime > 5*time.Minute {
                // Stop Container
                m.conductor.StopServer(server.ID)

                // Unregister von Velocity
                m.velocityClient.UnregisterServer(server.Name)

                // Nach 1h: VM decommissionen (wenn leer)
                if time.Since(server.StoppedAt) > 1*time.Hour {
                    node := m.nodeRegistry.GetNode(server.NodeID)
                    if node.ContainerCount == 0 {
                        m.scalingEngine.DecommissionNode(node.ID)
                    }
                }
            }
        }
    }
}
```

---

## Kosten-Breakdown

### Baseline (0 Last)

```
Tier 1 (Control Plane):  3.50€/month  (CX11, always-on)
Tier 2 (Proxy Layer):    3.50€/month  (CX11, always-on)
Tier 3 (Workload):       0.00€/month  (keine VMs)
─────────────────────────────────────
TOTAL:                   7.00€/month

Einsparung vs. Monolith: 70€ - 7€ = 63€/month (90% günstiger!)
```

### Normale Auslastung (10 MC-Server, je 4h/Tag)

```
Tier 1: 3.50€/month
Tier 2: 3.50€/month
Tier 3: 10 Server × 0.01€/h × 4h × 30 Tage = 12.00€/month
─────────────────────────────────────
TOTAL: 19.00€/month

Einsparung vs. Monolith: 70€ - 19€ = 51€/month (73% günstiger!)
```

### Peak Auslastung (50 MC-Server, je 8h/Tag)

```
Tier 1: 3.50€/month
Tier 2: 3.50€/month (Velocity kann 100+ Server routen)
Tier 3: 50 Server × 0.01€/h × 8h × 30 Tage = 120.00€/month
─────────────────────────────────────
TOTAL: 127.00€/month

Break-Even Point: Bei ~50 Servern á 8h/Tag
Bei mehr Last: Monolith wäre unpraktikabel (nur 1 Server!)
Mit 3-Tier: Unbegrenzt skalierbar
```

### Kosten-Vergleich Tabelle

| Szenario | Monolith | 3-Tier | Einsparung |
|----------|----------|--------|------------|
| 0 Server | 70€ | 7€ | **90%** |
| 5 Server (2h/Tag) | 70€ | 10€ | **86%** |
| 10 Server (4h/Tag) | 70€ | 19€ | **73%** |
| 20 Server (6h/Tag) | 70€ + mehr Hardware! | 43€ | - |
| 50 Server (8h/Tag) | Nicht möglich | 127€ | ∞ |

**Fazit:** 3-Tier ist bei niedriger bis mittlerer Last **deutlich günstiger** und bei hoher Last **überhaupt erst möglich**.

---

## Migration Roadmap

### Phase 0: Vorbereitung (Jetzt)

**Status:** ✅ Bereits teilweise fertig!

- ✅ Auto-Scaling Code (85% fertig)
- ✅ NodeRegistry (Fleet Management)
- ✅ Hetzner Cloud Integration
- ✅ VM Provisioning
- ⚠️ Velocity läuft noch lokal (auf Monolith)
- ⚠️ MC-Server starten nur lokal

**Aufgaben:**
- [x] Auto-Scaling konfigurieren (Hetzner Token)
- [x] Testing des aktuellen Systems
- [ ] Dokumentation lesen und verstehen

**Zeitaufwand:** 1-2 Tage (Testing)

---

### Phase 1: Velocity auslagern (Tier 2)

**Ziel:** Velocity auf separater VM, Remote-API bauen

**Schritte:**

1. **Velocity-VM erstellen (1 Stunde)**
   ```bash
   hcloud server create \
     --name payperplay-velocity \
     --type cx11 \
     --image ubuntu-22.04 \
     --ssh-key payperplay-main \
     --location nbg1

   # Output: 95.217.xxx.xxx (IP merken!)
   ```

2. **Velocity + Remote API installieren (2 Stunden)**
   ```bash
   ssh root@95.217.xxx.xxx

   # Docker installieren
   curl -fsSL https://get.docker.com | sh

   # Velocity Container starten
   docker run -d \
     --name velocity \
     -p 25565:25577 \
     -p 8080:8080 \
     -v /opt/velocity:/config \
     payperplay/velocity-with-api:latest
   ```

3. **Remote API Plugin entwickeln (1 Tag)**
   ```java
   // VelocityRemoteAPI Plugin
   // POST /api/servers - Server registrieren
   // DELETE /api/servers/:name - Server entfernen
   // GET /api/servers - Alle Server auflisten
   // GET /api/players/:server - Spieler-Count
   ```

4. **Control Plane anpassen (1 Tag)**
   ```go
   // internal/velocity/velocity_service.go

   // Alt: Lokaler Docker-Container
   // func (v *VelocityService) RegisterServer(...)

   // Neu: Remote HTTP API
   type RemoteVelocityClient struct {
       apiURL string
   }

   func NewRemoteVelocityClient(url string) *RemoteVelocityClient {
       return &RemoteVelocityClient{apiURL: url}
   }
   ```

5. **Testing (1 Tag)**
   - MC-Server lokal starten
   - Registrierung bei Remote-Velocity prüfen
   - Spieler-Connect testen
   - Failover-Tests (Velocity Restart)

**Dateien zu ändern:**
- `internal/velocity/velocity_service.go` (Remote-Client)
- `pkg/config/config.go` (VELOCITY_API_URL)
- `cmd/api/main.go` (Client initialisieren)

**Zeitaufwand:** 3-4 Tage

**Risiko:** Niedrig (Velocity ist stabil, nur API-Wrapper)

---

### Phase 2: Remote Container Orchestration (Tier 3)

**Ziel:** MC-Server auf Cloud-VMs starten (nicht nur lokal)

**Schritte:**

1. **Remote Docker Client (2 Tage)**
   ```go
   // internal/docker/remote_client.go

   type RemoteDockerClient struct {
       sshConfig *ssh.ClientConfig
   }

   func (c *RemoteDockerClient) StartContainer(node *Node, server *Server) error {
       // SSH Connection zu Remote-Node
       client, _ := ssh.Dial("tcp", node.IPAddress+":22", c.sshConfig)
       defer client.Close()

       // Docker Command ausführen
       session, _ := client.NewSession()
       cmd := fmt.Sprintf("docker run -d ...")
       output, err := session.CombinedOutput(cmd)

       return err
   }
   ```

2. **Conductor erweitern (1 Tag)**
   ```go
   // internal/conductor/conductor.go

   func (c *Conductor) StartServer(serverID string) error {
       server := c.getServer(serverID)

       // Node finden (lokal ODER remote!)
       node := c.nodeRegistry.FindBestNode(server.RAMMB)

       if node == nil {
           // Keine Kapazität → Auto-Scale!
           node = c.scalingEngine.ProvisionNode("cx21")
       }

       // Container starten (Remote!)
       if node.Type == "cloud" {
           c.remoteDocker.StartContainer(node, server)
       } else {
           c.localDocker.StartContainer(server)
       }

       // Bei Velocity registrieren
       address := fmt.Sprintf("%s:%d", node.IPAddress, server.Port)
       c.velocityClient.RegisterServer(server.Name, address)

       return nil
   }
   ```

3. **Cross-VM Networking (1 Tag)**
   ```bash
   # Hetzner Cloud Private Network
   hcloud network create \
     --name payperplay-net \
     --ip-range 10.0.0.0/16

   # Alle VMs in gleiches Network
   hcloud server attach-to-network \
     --network payperplay-net \
     payperplay-velocity
   ```

4. **Testing (2 Tage)**
   - VM-Provisioning testen
   - Container auf Remote-VM starten
   - Velocity-Registrierung prüfen
   - Spieler-Connect über Velocity → Remote-VM
   - Auto-Scale testen (>85% Kapazität)
   - Auto-Stop testen (Idle nach 5 Min)

**Dateien zu ändern:**
- `internal/docker/remote_client.go` (NEU)
- `internal/conductor/conductor.go` (StartServer erweitern)
- `internal/conductor/scaling_engine.go` (Network-Setup)

**Zeitaufwand:** 5-6 Tage

**Risiko:** Mittel (Networking kann tricky sein)

---

### Phase 3: Tier 1 auf minimal Dedicated (Optional)

**Ziel:** Control Plane auf günstigstes Setup migrieren

**Option A: Kleinere Hetzner Cloud VM (CX11)**
```
Aktuell: Dedicated AX41 (70€/month)
Neu: Cloud CX11 (3.50€/month)
Einsparung: 66.50€/month
```

**Option B: Hetzner Auction Server**
```
Gebrauchte Server ab 15€/month
Mehr RAM als CX11, aber immer "always-on"
```

**Empfehlung:** Später entscheiden (nach Phase 2)

**Zeitaufwand:** 1-2 Tage (Migration + DNS Update)

---

### Phase 4: Monitoring & Observability

**Ziel:** Vollständige Transparenz über alle 3 Tiers

**Komponenten:**

1. **Prometheus (Metrics)**
   ```yaml
   # Metrics sammeln:
   - payperplay_api_requests_total
   - payperplay_server_starts_total
   - payperplay_fleet_capacity_percent
   - payperplay_velocity_players_online
   - payperplay_cloud_cost_eur_hour
   ```

2. **Grafana (Dashboards)**
   ```
   Dashboard 1: Control Plane Health
   - API Response Time
   - Database Connections
   - Memory Usage

   Dashboard 2: Proxy Layer
   - Velocity Players Online
   - Server Count
   - Network Traffic

   Dashboard 3: Workload Layer
   - MC-Server Count
   - VM Count (Cloud)
   - Auto-Scaling Events
   - Cost per Hour
   ```

3. **Alerting**
   ```yaml
   - Alert: API Down (>5 min)
   - Alert: Velocity Down (>2 min)
   - Alert: Scaling Failed
   - Alert: Cost > 100€/day
   ```

**Zeitaufwand:** 2-3 Tage

---

## Gesamt-Timeline

| Phase | Aufgabe | Zeitaufwand | Priorität |
|-------|---------|-------------|-----------|
| Phase 0 | Auto-Scaling testen | 1-2 Tage | ✅ **JETZT** |
| Phase 1 | Velocity auslagern | 3-4 Tage | 🔴 **HOCH** |
| Phase 2 | Remote Orchestration | 5-6 Tage | 🔴 **HOCH** |
| Phase 3 | Control Plane migrieren | 1-2 Tage | 🟡 **MITTEL** |
| Phase 4 | Monitoring | 2-3 Tage | 🟢 **NIEDRIG** |

**Gesamt:** 12-17 Arbeitstage (2.5-3.5 Wochen)

**Empfohlene Reihenfolge:**
1. Phase 0 zuerst (Auto-Scaling funktioniert, aber lokal)
2. Phase 1+2 zusammen (Velocity + Remote = 8-10 Tage)
3. Phase 3+4 später (wenn System stabil läuft)

---

## Risiken & Mitigation

### Risiko 1: Networking-Probleme

**Problem:** VMs können nicht miteinander kommunizieren

**Mitigation:**
- Hetzner Cloud Private Network nutzen (10.0.0.0/16)
- Firewall-Rules testen (SSH, Docker API, Minecraft Ports)
- Fallback: Öffentliche IPs nutzen (weniger sicher)

### Risiko 2: SSH-Overhead

**Problem:** SSH-Verbindungen für Docker-Commands langsam

**Mitigation:**
- Docker Remote API über TCP aktivieren (Port 2375)
- Connection Pooling (SSH-Verbindungen wiederverwenden)
- Alternatve: Kubernetes (später, wenn wirklich nötig)

### Risiko 3: Velocity Single Point of Failure

**Problem:** Wenn Velocity down → Alle Spieler offline

**Mitigation:**
- Health Checks (alle 30s)
- Auto-Restart bei Failure
- Später: Mehrere Velocity-Instanzen + DNS Load Balancing

### Risiko 4: Kosten-Explosion

**Problem:** Viele VMs laufen, kosten steigen unkontrolliert

**Mitigation:**
- SCALING_MAX_CLOUD_NODES=10 (Safety Limit)
- Cost Alerts (>50€/Tag → Warning)
- Auto-Stop nach Idle (5 Min)
- Decommission leerer VMs (nach 1h)

---

## FAQ

### Q: Warum nicht Kubernetes?

**A:** Kubernetes ist Overkill für unseren Use-Case:

**Vorteile von Kubernetes:**
- Automatisches Load Balancing ✓
- Self-Healing ✓
- Declarative Configuration ✓

**Nachteile:**
- **Komplexität:** 5-10x mehr Code/Config
- **Kosten:** Mindestens 3 Nodes für HA (3×7€ = 21€ baseline!)
- **Overhead:** Kubernetes Control Plane braucht ~2 GB RAM
- **Lernkurve:** 2-3 Wochen Einarbeitung

**Unsere Lösung:**
- Einfacher: Docker + SSH + Hetzner Cloud API
- Günstiger: 7€ baseline statt 21€
- Wartbar: Einfacher zu debuggen
- Später migrieren: Wenn wir wirklich >100 VMs haben

### Q: Was passiert bei Velocity-Ausfall?

**A:** Spieler können nicht connecten, aber laufende Server bleiben online.

**Fallback:**
1. Health Check erkennt Ausfall (30s)
2. Auto-Restart von Velocity-Container (10s)
3. Server re-registrieren automatisch (30s)
4. **Gesamt-Downtime: ~1 Minute**

**Später:** Hot-Standby Velocity (zweite Instanz)

### Q: Können wir später auf AWS/GCP migrieren?

**A:** Ja! Die Architektur ist provider-agnostic.

**Was zu ändern:**
- `internal/cloud/hetzner_provider.go` → `aws_provider.go`
- API Calls anpassen (AWS EC2 statt Hetzner Cloud)
- Networking (VPC statt Hetzner Private Network)

**Was GLEICH bleibt:**
- ScalingEngine-Logik
- Conductor
- API
- Velocity-Integration

### Q: Was ist mit Backups?

**A:** Backups bleiben auf Object Storage (S3-kompatibel).

**Strategie:**
- MC-Server Daten → Hetzner Storage Box (5 TB = 3.20€/month)
- PostgreSQL → Daily Backup zu Storage Box
- Bei Server-Stop: Automatisches Backup zu Storage
- Bei Server-Start: Restore von Storage

---

## Nächste Schritte

### Sofort (diese Woche):

1. **Auto-Scaling testen**
   - Hetzner Cloud Token konfigurieren
   - Erste VM provisionieren
   - Logs beobachten

2. **Dokumentation lesen**
   - `docs/SCALING_ARCHITECTURE.md`
   - `docs/AUTO_SCALING_QUICK_START.md`
   - Dieses Dokument (`3_TIER_ARCHITECTURE.md`)

### Nächste 2 Wochen:

3. **Phase 1: Velocity auslagern**
   - VM erstellen
   - Remote API entwickeln
   - Control Plane anpassen
   - Testing

### Nächste 4 Wochen:

4. **Phase 2: Remote Orchestration**
   - Remote Docker Client
   - Conductor erweitern
   - Testing
   - Produktiv-Deployment

---

## Zusammenfassung

### Aktuelle Situation
- ❌ Monolith: Alles auf einem Server (70€/month)
- ❌ Nicht skalierbar
- ❌ Unprofitabel bei niedriger Last

### Ziel-Architektur
- ✅ 3 Tiers: Control + Proxy + Workload
- ✅ Vollständig skalierbar (0 bis ∞ Server)
- ✅ 7€/month bei 0 Last (90% günstiger!)
- ✅ Pay-per-use für MC-Server

### Aufwand
- **Phase 1+2:** 8-10 Arbeitstage (Velocity + Remote)
- **Gesamt:** 12-17 Arbeitstage (mit Monitoring)
- **Empfehlung:** Schritt für Schritt, jede Phase testen

### ROI (Return on Investment)
```
Entwicklungszeit: ~3 Wochen
Kosten-Einsparung: ~50-60€/month
Break-Even: Nach 1 Monat! 🎉

Jahr 1: 600-700€ gespart
Jahr 2: Unbezahlbar (Skalierbarkeit!)
```

---

**Fragen?** Siehe `docs/AUTO_SCALING_QUICK_START.md` für erste Schritte!
