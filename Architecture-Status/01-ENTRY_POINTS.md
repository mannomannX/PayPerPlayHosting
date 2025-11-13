# Application Entry Points - Code-Analyse

**Datei:** [cmd/api/main.go](../cmd/api/main.go)
**Zeilen:** 402
**Zweck:** Zentrale Application Bootstrap & Dependency Initialization

## Übersicht

Die `main.go` ist der **einzige Einstiegspunkt** der Anwendung und orchestriert die komplette Initialisierung aller Komponenten in einer fest definierten Reihenfolge.

## Initialisierungssequenz (26 Phasen)

### Phase 1-3: Grundlagen
```go
1. Config Load           (Zeile 29)  - config.Load()
2. Logger Init           (Zeile 32)  - logger.NewLogger()
3. Database Init         (Zeile 43)  - repository.InitDB()
```

### Phase 4-5: Event Infrastructure
```go
4. Event Storage Setup   (Zeile 48-81)
   - PostgreSQL (primär, immer aktiv)
   - InfluxDB (optional, fallback zu PostgreSQL-only)
   - Multi-Storage Pattern für Dual-Write

5. Event Bus Global Set  (Zeile 81)  - events.SetEventStorage()
```

**Besonderheit:** Dual-Storage Pattern mit graceful degradation zu DB-only falls InfluxDB fehlt.

### Phase 6-7: Core Infrastructure
```go
6. Docker Service        (Zeile 84)  - docker.NewDockerService()
7. Repositories          (Zeile 92-96)
   - ServerRepository
   - UserRepository
   - ConfigChangeRepository
   - FileRepository
   - PluginRepository
```

### Phase 8-12: Security & Auth Layer
```go
 8. Email Service        (Zeile 98-102) - 🚧 MOCK MODE
 9. Security Service     (Zeile 105)    - Device trust, security events
10. Auth Service         (Zeile 109)    - JWT, Login, Registration
11. OAuth Service        (Zeile 110)    - OAuth2 providers
12. Middleware Link      (Zeile 126)    - middleware.SetAuthService()
```

**⚠️ CRITICAL ISSUE:** Email Service im MOCK MODE (Zeile 99-102)
```go
// 🚧 TODO: Replace MockEmailSender with ResendEmailSender when ready for production
mockEmailSender := service.NewMockEmailSender(db)
emailService := service.NewEmailService(mockEmailSender, db)
logger.Info("Email service initialized (🚧 MOCK MODE)", nil)
```

### Phase 13-15: Core Business Services
```go
13. Minecraft Service     (Zeile 113) - Haupt-Business-Logic
14. Monitoring Service    (Zeile 114) - Auto-Shutdown, Health Checks
15. Recovery Service      (Zeile 117) - Crash Detection & Auto-Restart
    - recoveryService.Start()
    - defer recoveryService.Stop()
```

**Wichtig:** Recovery Service läuft als Background-Worker (Start/Stop-Lifecycle).

### Phase 16-18: Data Management Services
```go
16. Backup Service        (Zeile 127) - Backup/Restore Operations
17. Backup Scheduler      (Zeile 133) - Automated Backup Worker
    - backupScheduler.Start()
    - defer backupScheduler.Stop()
18. Lifecycle Service     (Zeile 139) - 3-Phase Lifecycle (Active/Sleep/Archive)
    - lifecycleService.Start()
    - defer lifecycleService.Stop()
```

**Pattern:** Worker-Services mit Start/Stop-Lifecycle und defer-Cleanup.

### Phase 19: Billing & Analytics
```go
19. Billing Service       (Zeile 145) - Cost Analytics & Usage Tracking
    - billingService.Start()  // Event-Bus Subscriber!
    - defer billingService.Stop()
```

**Event-Driven:** Subscribed zu ServerStarted/ServerStopped Events (Event-Bus Pattern).

### Phase 20-21: Plugin Ecosystem
```go
20. Plugin Sync Service   (Zeile 151) - Auto-Sync from Modrinth (every 6h)
    - pluginSyncService.Start()
    - defer pluginSyncService.Stop()
21. Plugin Manager        (Zeile 156) - Plugin Installation/Management
22. Plugin Service        (Zeile 159) - Plugin API
```

### Phase 22-23: Real-time Communication
```go
22. WebSocket Hub         (Zeile 164) - Real-time Updates
    - go wsHub.Run()  // Goroutine!
23. Service Linking       (Zeile 169-176)
    - mcService.SetWebSocketHub(wsHub)
    - recoveryService.SetWebSocketHub(wsHub)
    - monitoringService.SetRecoveryService(recoveryService)
```

**Pattern:** Bidirectional dependency injection nach Initialisierung.

### Phase 24: Velocity Proxy Integration
```go
24. Velocity Service      (Zeile 179-201)
    - Local Velocity (Docker Container auf Port 25565)
    - velocityService.Start()
    - defer velocityService.Stop()

25. Remote Velocity       (Zeile 203-215) - NEW 3-Tier Architecture
    - Optional, wenn VELOCITY_API_URL konfiguriert
    - remoteVelocityClient = velocity.NewRemoteVelocityClient()
    - mcService.SetRemoteVelocityClient(remoteVelocityClient)
```

**⚠️ Architecture Note:** Dual-Mode: Lokaler Velocity Container + Remote Velocity API.

### Phase 26-28: Monitoring & Metrics
```go
26. Monitoring Start      (Zeile 218) - monitoringService.Start()
27. Prometheus Exporter   (Zeile 223) - Metrics Collection (every 30s)
```

### Phase 29-32: **CONDUCTOR CORE** (Zentral!)
```go
29. Conductor Init        (Zeile 228)
    cond := conductor.NewConductor(10*time.Second, cfg.SSHPrivateKeyPath)

30. Scaling Engine        (Zeile 231-241)
    IF cfg.HetznerCloudToken != "":
      - hetznerProvider := cloud.NewHetznerProvider()
      - cond.InitializeScaling(hetznerProvider, ...)
    ELSE:
      - Scaling disabled (Warning geloggt)

31. Conductor Linking     (Zeile 244-249)
    - mcService.SetConductor(cond)        // MinecraftService -> Conductor
    - cond.SetServerStarter(mcService)    // Conductor -> MinecraftService

32. Conductor Start       (Zeile 251)
    - cond.Start()
    - defer cond.Stop()
```

**🔥 CRITICAL PATTERN:** Circular Dependency Resolution:
- MinecraftService needs Conductor für Capacity Management
- Conductor needs MinecraftService als ServerStarter für Queue Processing
- Gelöst durch Post-Init-Linking (Zeile 244-249)

### Phase 33-35: **STATE RECOVERY** (Critical!)
```go
33. Container Sync        (Zeile 255-258)
    cond.SyncRunningContainers(dockerService, serverRepo)
    // Verhindert OOM nach Restarts!

34. Queue Sync            (Zeile 261-263)
    cond.SyncQueuedServers(serverRepo, false)
    // Verhindert Queue-Verlust nach Restarts!

35. Initial Scaling       (Zeile 269-271)
    cond.TriggerScalingCheck()
```

**⚠️ REMOVED:** Worker-Node Sync (Zeile 265-266)
```go
// NOTE: Worker-Node sync REMOVED - nodes are registered via ProvisionNode, not recovered
// This prevents node churn during container restarts
```

**Warum kritisch?** Ohne diese Syncs würden nach einem Neustart:
- Laufende Container nicht im RAM-Tracking sein → OOM-Risiko
- Queued Servers verloren gehen → SLA-Verletzung

### Phase 36-38: API Layer
```go
36. Handler Init          (Zeile 274-346) - 20+ Handler-Instanzen
37. Dashboard WebSocket   (Zeile 349-352)
    - go dashboardWs.Run()  // Goroutine!
    - defer dashboardWs.Shutdown()
38. Router Setup          (Zeile 358) - api.SetupRouter(...)
```

### Phase 39-40: Server Lifecycle
```go
39. Graceful Shutdown     (Zeile 361-370)
    - Signal Handler (SIGINT, SIGTERM)
    - Servers bleiben LAUFEN (auto-shutdown handled es)
    - os.Exit(0)

40. HTTP Server Start     (Zeile 373-382)
    - router.Run(addr)
```

**⚠️ Design Decision:** Bei Shutdown werden Server NICHT gestoppt.
Begründung (Zeile 367-368):
```go
// Leave servers running - they will be managed by auto-shutdown
// This allows maintenance without disrupting active servers
```

## Dependency Graph (Vereinfacht)

```
Config
  ├─> Logger
  ├─> Database
  │     ├─> Event-Bus (PostgreSQL + InfluxDB)
  │     ├─> Repositories (5x)
  │     │     ├─> Email Service (MOCK) ⚠️
  │     │     │     ├─> Security Service
  │     │     │     │     ├─> Auth/OAuth
  │     │     │     │     └─> Services (23x)
  │     │     │           ├─> WebSocket Hub
  │     │     │           ├─> Velocity (Local + Remote)
  │     │     │           ├─> Prometheus
  │     │     │           └─> CONDUCTOR ⭐
  │     │     │                 ├─> Scaling Engine (Hetzner)
  │     │     │                 ├─> State Sync (Container + Queue)
  │     │     │                 └─> API Handlers (20+)
  │     │     │                       └─> HTTP Server
  ├─> Docker Service
```

## Code-Flaws & Potenzielle Probleme

### 🔴 CRITICAL

1. **Email Service in MOCK MODE** (Zeile 99-102)
   - **Impact:** Keine echten Emails in Production
   - **Betroffene Features:** User Registration, Password Reset, Security Alerts
   - **Location:** `mockEmailSender := service.NewMockEmailSender(db)`

2. **Keine Partial Failure Handling**
   - **Problem:** Wenn ein Service fehlschlägt → komplette App down
   - **Beispiel:** InfluxDB-Fehler ist graceful (Zeile 64-76), aber Docker-Fehler ist fatal (Zeile 85-87)
   - **Vorschlag:** Feature Flags für optionale Services

3. **Komplexe Circular Dependencies**
   - **Beispiel:** MinecraftService ↔ Conductor (Zeile 244-249)
   - **Risiko:** Race Conditions bei paralleler Nutzung vor vollständiger Initialisierung
   - **Current Mitigation:** Sequentielle Initialisierung (funktioniert, aber fragil)

### 🟡 MEDIUM

4. **Orphaned Server Cleanup deaktiviert** (Zeile 122-123)
   ```go
   // Note: Orphaned server cleanup is NOT run on startup to avoid race conditions
   // during container restarts. The monitoring service handles cleanup periodically.
   ```
   - **Risiko:** Nach Crash könnten Orphaned Servers länger existieren
   - **Mitigation:** Monitoring Service macht es periodisch

5. **Worker-Node Sync REMOVED** (Zeile 265-266)
   - **Problem:** Nodes werden nach Restart nicht wiederhergestellt
   - **Begründung:** "prevents node churn" - aber was wenn Node crashed?
   - **Risiko:** Manuelle Re-Registration nötig

6. **Goroutines ohne Panic Recovery**
   - **Locations:**
     - `go wsHub.Run()` (Zeile 165)
     - `go dashboardWs.Run()` (Zeile 350)
   - **Risiko:** Panic in Goroutine → App-Crash (kein defer-Recovery)

### 🟢 LOW

7. **Hardcoded Timeouts**
   - Conductor Health Check: `10*time.Second` (Zeile 228)
   - Prometheus Collector: `30*time.Second` (Zeile 224)
   - Plugin Sync: "every 6h" (hardcoded im Service)
   - **Vorschlag:** Config-basiert

8. **Log Level Parsing Case-Sensitive** (Zeile 386-400)
   - Nur uppercase funktioniert: "DEBUG", nicht "debug"
   - Fallback zu INFO ist OK, aber könnte verwirrender sein

## Abhängigkeiten zu anderen Modulen

### Importierte Packages
```go
internal/api              - HTTP Handlers & Router
internal/cloud            - Hetzner Provider
internal/conductor        - Fleet Orchestration (KERN!)
internal/docker           - Container Management
internal/events           - Event-Bus
internal/middleware       - Auth, Logging, Rate Limiting
internal/monitoring       - Prometheus, Metrics
internal/repository       - Database Access
internal/service          - Business Logic (23 Services)
internal/storage          - InfluxDB Client
internal/velocity         - Proxy Integration
internal/websocket        - Real-time Updates
pkg/config                - Configuration
pkg/logger                - Structured Logging
```

### Externe Dependencies (aus Imports)
Keine direkt sichtbar - alle über interne Packages abstrahiert.

## Lifecycle-Pattern

**Worker-Services mit Start/Stop:**
1. Recovery Service (Zeile 118-119)
2. Backup Scheduler (Zeile 134-135)
3. Lifecycle Service (Zeile 140-141)
4. Billing Service (Zeile 146-147)
5. Plugin Sync Service (Zeile 152-153)
6. Monitoring Service (Zeile 218-219)
7. Velocity Service (Zeile 192-201)
8. Conductor (Zeile 251-252)
9. WebSocket Hub (Zeile 165 - nur Run, kein explicit Stop)
10. Dashboard WebSocket (Zeile 350-351)

**Defer-Cleanup Pattern:**
Alle Worker-Services nutzen `defer service.Stop()` für graceful cleanup.

## Performance-Überlegungen

1. **Sequentielle Initialisierung** - könnte parallelisiert werden für schnelleren Start
2. **Viele Goroutines** - mindestens 12+ aktive Goroutines nach Start
3. **Keine Warmup-Phase** - Service sofort verfügbar nach Start (potenzielle Race Conditions)

## Änderungshistorie (aus Kommentaren)

- **Removed:** Orphaned server cleanup on startup (Zeile 122)
- **Removed:** Worker-Node sync (Zeile 265)
- **Added:** Remote Velocity Client (3-tier architecture) (Zeile 203)
- **Added:** Dashboard WebSocket (Zeile 349)
- **Changed:** Billing to Event-Bus subscription (Zeile 172)

## Nächste Schritte

Siehe [02-DATA_MODELS.md](02-DATA_MODELS.md) für Datenmodell-Analyse.
