# Business Logic Layer - Service Architecture

**Verzeichnis:** [internal/service/](../internal/service/)
**Dateien:** 27 Service-Dateien
**Zweck:** Business Logic, Orchestrierung, Feature-Implementation
**Pattern:** Service Layer mit Event-Driven Architecture

## Übersicht

Die Service-Layer implementiert die **gesamte Business Logic** der PayPerPlay-Plattform. Services koordinieren zwischen Repositories (Datenzugriff), Docker (Container), Conductor (Orchestrierung) und externen APIs.

**Architektur-Position:**
```
API Handler → MinecraftService → Conductor → Docker/Cloud
                      ↓
              BillingService (Event-Bus)
              LifecycleService (Background Worker)
              MonitoringService (Auto-Shutdown)
              BackupService (Scheduled)
```

## Service-Kategorien

### 1. **Core Services** (System-kritisch)
- `minecraft_service.go` - Server Lifecycle (CRUD, Start, Stop)
- `billing_service.go` - Usage Tracking & Cost Calculation
- `lifecycle_service.go` - 3-Phase Lifecycle (Active/Sleep/Archive)
- `monitoring_service.go` - Auto-Shutdown & Health Monitoring
- `backup_service.go` - Automated Backups

### 2. **Authentication & Security**
- `auth_service.go` - JWT Token Management
- `security_service.go` - Device Fingerprinting, 2FA, Account Lockout
- `oauth_service.go` - GitHub/Google OAuth Integration
- `email_service.go` - Email Notifications (MOCK!)

### 3. **Plugin System**
- `plugin_service.go` - Plugin CRUD
- `plugin_manager_service.go` - Plugin Installation/Management
- `plugin_sync_service.go` - Modrinth Sync (6-hour intervals)

### 4. **File Management**
- `file_service.go` - File Upload/Download
- `filemanager_service.go` - File Browser
- `file_integration_service.go` - File Operations
- `file_validator.go` - Security Validation (path traversal, size limits)
- `file_metrics.go` - Storage Metrics

### 5. **Minecraft Features**
- `console_service.go` - RCON Command Execution
- `player_list_service.go` - Player Management
- `motd_service.go` - Message of the Day
- `resource_pack_service.go` - Resource Pack Management
- `world_service.go` - World Management
- `template_service.go` - Server Templates
- `config_service.go` - Server Configuration
- `recovery_service.go` - Server Recovery/Restore

### 6. **Integrations**
- `webhook_service.go` - Webhook Notifications
- `backup_scheduler.go` - Backup Scheduling

---

## Detaillierte Service-Analyse

### 1. MinecraftService - Kern des Systems

**Datei:** [internal/service/minecraft_service.go](../internal/service/minecraft_service.go)
**Zeilen:** ~800 (geschätzt basierend auf 200 Zeilen Preview)
**Verantwortung:** Server CRUD, Lifecycle Management, Orchestrierung

#### Struktur

```go
type MinecraftService struct {
    repo                  *repository.ServerRepository
    dockerService         *docker.DockerService
    cfg                   *config.Config
    velocityService       VelocityServiceInterface    // DEPRECATED
    remoteVelocityClient  RemoteVelocityClientInterface // NEW: HTTP API
    wsHub                 WebSocketHubInterface       // Real-time updates
    conductor             ConductorInterface          // Capacity management
}
```

#### Interface-basierte Dependencies

**ConductorInterface (Zeile 49-127):** 20+ Methods!

```go
type ConductorInterface interface {
    // Capacity Checks (DEPRECATED methods)
    CheckCapacity(requiredRAMMB int) (bool, int)
    AtomicAllocateRAM(ramMB int) bool      // DEPRECATED: single-node
    ReleaseRAM(ramMB int)                   // DEPRECATED: single-node

    // Multi-Node RAM Management (CURRENT)
    SelectNodeForContainerAuto(requiredRAMMB int) (string, error)
    AtomicAllocateRAMOnNode(nodeID string, ramMB int) bool
    ReleaseRAMOnNode(nodeID string, ramMB int)

    // CPU Guard (Prevents simultaneous container starts)
    AtomicReserveStartSlot(serverID, serverName string, ramMB int) bool
    ReleaseStartSlot(serverID string)

    // Container Registry
    RegisterContainer(serverID, serverName, containerID, nodeID string, ramMB, dockerPort, minecraftPort int, status string)
    GetContainer(serverID string) (containerInfo interface{}, exists bool)
    UpdateContainerStatus(serverID, status string)

    // Start Queue Management
    EnqueueServer(serverID, serverName string, requiredRAMMB int, userID string)
    IsServerQueued(serverID string) bool
    RemoveFromQueue(serverID string)
    ProcessStartQueue()

    // Scaling Trigger
    TriggerScalingCheck() // Forces immediate scaling evaluation

    // Remote Node Operations
    GetRemoteNode(nodeID string) (*docker.RemoteNode, error)
    GetRemoteDockerClient() *docker.RemoteDockerClient
    GetNode(nodeID string) (interface{}, bool)

    // Startup Guard
    CanStartServer(ramMB int) (bool, string)
}
```

**🔥 CRITICAL Interface Design Issue:**
- **20+ Methods** in Interface - Violation of Interface Segregation Principle (ISP)
- Besserer Ansatz: Split in mehrere kleinere Interfaces
  - `CapacityManager` - RAM/Node Selection
  - `QueueManager` - Start Queue
  - `ContainerRegistry` - Container Tracking
  - `ScalingTrigger` - Auto-Scaling

#### Key Methods

**CreateServer() - Zeile 162-200:**

```go
func (s *MinecraftService) CreateServer(name, serverType, minecraftVersion, ramMB, ownerID) (*models.MinecraftServer, error) {
    // 1. Generate server ID
    serverID := uuid.New().String()[:8]  // 8-char UUID

    // 2. Find available port
    usedPorts := s.repo.GetUsedPorts()
    port := s.dockerService.FindAvailablePort(usedPorts)

    // 3. Create server record
    server := &models.MinecraftServer{
        ID:                   serverID,
        Name:                 name,
        OwnerID:              ownerID,
        ServerType:           serverType,
        MinecraftVersion:     minecraftVersion,
        RAMMb:                ramMB,
        Port:                 port,
        Status:               models.StatusQueued,  // ⚠️ Always starts in queue!
        IdleTimeoutSeconds:   s.cfg.DefaultIdleTimeout,
        AutoShutdownEnabled:  true,
        MaxPlayers:           20,
    }

    // 4. Save to DB
    err := s.repo.Create(server)
    // ...
}
```

**🟡 MEDIUM Issue: Hardcoded Defaults**
- `MaxPlayers = 20` - Should be configurable or tier-based
- `AutoShutdownEnabled = true` - Should be user's choice
- `IdleTimeoutSeconds = cfg.DefaultIdleTimeout` - OK, aber sollte pro-Tier unterschiedlich sein

#### Velocity Integration

**OLD (DEPRECATED):**
```go
type VelocityServiceInterface interface {
    RegisterServer(server *models.MinecraftServer) error
    UnregisterServer(serverID string) error
    IsRunning() bool
}
```

**NEW (HTTP API):**
```go
type RemoteVelocityClientInterface interface {
    RegisterServer(name, address string) error
    UnregisterServer(name string) error
}
```

**🟡 MEDIUM Issue: Deprecated Code nicht entfernt**
- `velocityService` Field bleibt im Struct
- `SetVelocityService()` Method als DEPRECATED markiert
- Sollte komplett entfernt werden nach Migration

---

### 2. BillingService - Event-Driven Billing

**Datei:** [internal/service/billing_service.go](../internal/service/billing_service.go)
**Zeilen:** ~400 (geschätzt)
**Verantwortung:** Usage Tracking, Cost Calculation, Billing Events

#### Event-Driven Architecture

**Start() - Event-Bus Subscription (Zeile 32-41):**

```go
func (s *BillingService) Start() {
    bus := events.GetEventBus()

    // Subscribe to server lifecycle events
    bus.Subscribe(events.EventServerStarted, s.handleServerStarted)
    bus.Subscribe(events.EventServerStopped, s.handleServerStopped)
    bus.Subscribe(events.EventBillingPhaseChanged, s.handlePhaseChanged)

    logger.Info("BillingService subscribed to Event-Bus", nil)
}
```

**✅ GOOD Pattern:**
- Decoupled from MinecraftService
- Automatic billing tracking via events
- No direct coupling to Docker/Conductor

#### Event Handlers

**handleServerStarted() - Zeile 49-66:**

```go
func (s *BillingService) handleServerStarted(event events.Event) {
    // 1. Fetch server details
    server := s.serverRepo.FindByID(event.ServerID)

    // 2. Record billing event and create usage session
    s.recordServerStartedInternal(server)
}
```

**handleServerStopped() - Zeile 68-85:**

```go
func (s *BillingService) handleServerStopped(event events.Event) {
    // 1. Fetch server details
    server := s.serverRepo.FindByID(event.ServerID)

    // 2. Record billing event and close usage session
    s.recordServerStoppedInternal(server)
}
```

**handlePhaseChanged() - Zeile 87-117:**

```go
func (s *BillingService) handlePhaseChanged(event events.Event) {
    oldPhase := event.Data["old_phase"].(string)
    newPhase := event.Data["new_phase"].(string)

    s.RecordPhaseChange(server, oldPhase, newPhase)
}
```

#### Billing Records

**recordServerStartedInternal() - Zeile 126-150:**

```go
func (s *BillingService) recordServerStartedInternal(server *models.MinecraftServer) error {
    now := time.Now()

    // Calculate tier-based hourly rate
    hourlyRate := s.getHourlyRateForServer(server)

    // Create billing event
    event := &models.BillingEvent{
        ID:               uuid.New().String(),
        ServerID:         server.ID,
        ServerName:       server.Name,
        OwnerID:          server.OwnerID,
        EventType:        models.EventServerStarted,
        Timestamp:        now,
        RAMMb:            server.RAMMb,
        StorageGB:        0, // 🔴 TODO: Calculate actual storage usage
        LifecyclePhase:   models.PhaseActive,
        HourlyRateEUR:    hourlyRate,
    }

    s.db.Create(event)
    // ...
}
```

**🔴 CRITICAL Issue: StorageGB = 0**
- Storage usage wird nicht getrackt
- Billing ist unvollständig
- TODO seit wann im Code?

**🟡 MEDIUM Issue: Deprecated Method**
```go
// DEPRECATED: This method is kept for backwards compatibility
// Billing events are now automatically created via Event-Bus subscription
func (s *BillingService) RecordServerStarted(server *models.MinecraftServer) error {
    return s.recordServerStartedInternal(server)
}
```
- Public Method als DEPRECATED markiert
- Wer ruft es noch auf? Entfernen oder behalten?

---

### 3. LifecycleService - 3-Phase Lifecycle Management

**Datei:** [internal/service/lifecycle_service.go](../internal/service/lifecycle_service.go)
**Zeilen:** ~200 (geschätzt)
**Verantwortung:** Automatic Phase Transitions (Active → Sleep → Archive)

#### Background Worker Pattern

**Start() - Zeile 29-40:**

```go
func (s *LifecycleService) Start() {
    logger.Info("Starting lifecycle service", nil)

    // Start Sleep Worker (runs every 5 minutes)
    go s.sleepWorker(5 * time.Minute)

    // Future: Archive Worker will run here too
    // go s.archiveWorker(1 * time.Hour)  // 🔴 TODO

    logger.Info("Lifecycle service started", nil)
}
```

**🔴 CRITICAL Issue: Archive Worker nicht implementiert**
- Code-Kommentar sagt "Future"
- Aber: Archive-Phase ist in Models definiert!
- Phase 3 (Archive) funktioniert nicht automatisch

#### Sleep Worker

**sleepWorker() - Zeile 48-64:**

```go
func (s *LifecycleService) sleepWorker(interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    // Run immediately on start
    s.processSleepTransitions()

    for {
        select {
        case <-ticker.C:
            s.processSleepTransitions()
        case <-s.stopChan:
            return
        }
    }
}
```

**✅ GOOD Pattern:**
- Graceful shutdown via stopChan
- Immediate execution on start
- Clean ticker cleanup with defer

**🟡 MEDIUM Issue: Hardcoded Interval**
- 5 Minuten hardcoded
- Sollte konfigurierbar sein (Environment Variable)

#### Sleep Transition Logic

**processSleepTransitions() - Zeile 66-128:**

```go
func (s *LifecycleService) processSleepTransitions() {
    fiveMinutesAgo := time.Now().Add(-5 * time.Minute)

    // Find servers that are:
    // 1. Status = stopped
    // 2. LastStoppedAt > 5 minutes ago
    // 3. LifecyclePhase != sleep (not already sleeping)

    var servers []models.MinecraftServer
    s.db.Where("status = ? AND lifecycle_phase != ? AND last_stopped_at IS NOT NULL AND last_stopped_at < ?",
        models.StatusStopped,
        models.PhaseSleep,
        fiveMinutesAgo,
    ).Find(&servers)

    for _, server := range servers {
        oldPhase := server.LifecyclePhase

        // Update status and lifecycle phase
        updates := map[string]interface{}{
            "status":          models.StatusSleeping,
            "lifecycle_phase": models.PhaseSleep,
        }

        s.db.Model(&server).Updates(updates)

        // Publish phase change event for billing tracking
        events.PublishBillingPhaseChanged(server.ID, string(oldPhase), string(models.PhaseSleep))

        logger.Info("Server transitioned to sleep", ...)
    }
}
```

**✅ GOOD:**
- Event-driven billing notification
- Clear transition logic
- Detailed logging

**🟡 MEDIUM Issue: Keine Transaction**
- Updates + Event Publish sollten atomic sein
- Bei Fehler im Event-Publish: Server ist in Sleep, aber Billing nicht benachrichtigt

---

### 4. MonitoringService - Auto-Shutdown & Crash Detection

**Datei:** [internal/service/monitoring_service.go](../internal/service/monitoring_service.go)
**Zeilen:** ~400 (geschätzt)
**Verantwortung:** Player Count Monitoring, Auto-Shutdown, Crash Detection

#### Struktur

```go
type MonitoringService struct {
    mcService       *MinecraftService
    repo            *repository.ServerRepository
    cfg             *config.Config
    recoveryService *RecoveryService

    // Track idle timers per server
    idleTimers map[string]*IdleTimer
    mu         sync.RWMutex

    // Graceful shutdown
    ctx    context.Context
    cancel context.CancelFunc
}

type IdleTimer struct {
    ServerID       string
    IdleSince      time.Time
    LastPlayerCount int
    CheckInterval  time.Duration
    TimeoutSeconds int  // From server.IdleTimeoutSeconds
}
```

#### Auto-Shutdown Workflow

**Start() - Zeile 58-66:**

```go
func (m *MonitoringService) Start() {
    log.Println("Starting monitoring service...")

    // Initial scan for running servers
    go m.scanRunningServers()  // ⚠️ No panic recovery!

    // Start monitoring loop
    go m.monitorLoop()          // ⚠️ No panic recovery!
}
```

**🟡 MEDIUM Issue: Keine Panic Recovery**
- 2 Goroutines ohne Panic Recovery
- Crash → Monitoring stoppt komplett → Server laufen unbegrenzt weiter

**monitorLoop() - Zeile 80-100:**

```go
func (m *MonitoringService) monitorLoop() {
    ticker := time.NewTicker(60 * time.Second) // ⚠️ Hardcoded!
    defer ticker.Stop()

    for {
        select {
        case <-m.ctx.Done():
            return  // ✅ GOOD: Graceful shutdown
        case <-ticker.C:
            m.checkAllServers()

            // Also check for crashed servers if recovery service is available
            if m.recoveryService != nil {
                m.recoveryService.CheckAndRecoverCrashedServers()
            }
        }
    }
}
```

**✅ GOOD:**
- Graceful shutdown via Context
- Recovery Service Integration
- Separation of concerns (Idle vs. Crash detection)

**🟡 MEDIUM Issue: Hardcoded 60-Second Interval**
- Sollte konfigurierbar sein
- Dev vs. Prod brauchen unterschiedliche Intervalle (Dev: 10s, Prod: 60s)

#### Player Count Detection

**checkServer() - Zeile 168-200:**

```go
func (m *MonitoringService) checkServer(serverID string) {
    server := m.repo.FindByID(serverID)

    // Skip checks
    if server.Status != models.StatusRunning { return }
    if !server.AutoShutdownEnabled { return }

    // Get player count via RCON
    playerCount, err := m.getPlayerCount(server)

    timer := m.idleTimers[serverID]

    if err != nil {
        // ⚠️ Warning logged, but timer NOT updated
        // Server might be stopped even if RCON fails!
        log.Printf("Warning: Could not get player count: %v", err)
        // ... continues with old playerCount
    }

    // Logic: If playerCount > 0 → reset IdleSince
    //        If playerCount == 0 → check if idle time > TimeoutSeconds
}
```

**🟡 MEDIUM Issue: RCON Fehler werden ignoriert**
- Wenn RCON fehlschlägt, wird `playerCount = 0` angenommen
- Server könnte gestoppt werden obwohl Spieler online sind
- Besserer Ansatz: Bei RCON-Fehler → skip this check

#### Integration mit RecoveryService

**Zeile 92-96:**
```go
// Also check for crashed servers if recovery service is available
if m.recoveryService != nil {
    m.recoveryService.CheckAndRecoverCrashedServers()
}
```

**✅ GOOD:**
- Optional dependency (if nil → skip)
- Crash detection alle 60 Sekunden
- Separation of Concerns

---

### 5. BackupService - Backup & Restore

**Datei:** [internal/service/backup_service.go](../internal/service/backup_service.go)
**Zeilen:** ~300 (geschätzt)
**Verantwortung:** ZIP-basierte Backups, Restore mit Rollback-Sicherheit

#### Backup Workflow

**CreateBackup() - Zeile 40-85:**

```go
func (b *BackupService) CreateBackup(serverID string) (string, error) {
    server := b.repo.FindByID(serverID)

    // If server is running, save world data before backup
    if server.Status == models.StatusRunning {
        if err := b.saveRunningServer(server); err != nil {
            log.Printf("Warning: failed to save running server: %v", err)
            // Continue with backup anyway ⚠️ Data loss möglich!
        }
    }

    serverDir := filepath.Join(b.cfg.ServersBasePath, serverID)

    // Create ZIP file with timestamp
    timestamp := time.Now().Format("20060102-150405")
    backupFilename := fmt.Sprintf("%s-%s.zip", serverID, timestamp)
    backupPath := filepath.Join(b.backupDir, backupFilename)

    if err := b.zipDirectory(serverDir, backupPath); err != nil {
        return "", err
    }

    // Publish event for billing/analytics
    events.PublishBackupCreated(serverID, backupFilename, stat.Size())

    return backupPath, nil
}
```

**✅ GOOD:**
- RCON "save-all" vor Backup (für running servers)
- Event Publishing (BackupCreated)
- Timestamp in Filename

**🟡 MEDIUM Issue: Save-Fehler wird ignoriert**
```go
if err := b.saveRunningServer(server); err != nil {
    log.Printf("Warning: failed to save running server: %v", err)
    // Continue with backup anyway ⚠️
}
```
- Bei RCON-Fehler wird Backup trotzdem erstellt
- Potenzielle Data Loss (unsaved chunks)
- Besserer Ansatz: Return error, user entscheidet

#### Restore mit Rollback-Sicherheit

**RestoreBackup() - Zeile 87-126:**

```go
func (b *BackupService) RestoreBackup(serverID string, backupPath string) error {
    server := b.repo.FindByID(serverID)

    // Server must be stopped
    if server.Status != "stopped" {
        return fmt.Errorf("server must be stopped before restoring backup")
    }

    serverDir := filepath.Join(b.cfg.ServersBasePath, serverID)

    // Backup current data (just in case)
    tempBackup := serverDir + ".pre-restore"
    if err := os.Rename(serverDir, tempBackup); err != nil {
        return err
    }

    // Unzip backup
    if err := b.unzipArchive(backupPath, serverDir); err != nil {
        // Restore old data on error ✅ ROLLBACK!
        os.RemoveAll(serverDir)
        os.Rename(tempBackup, serverDir)
        return err
    }

    // Remove temporary backup
    os.RemoveAll(tempBackup)

    events.PublishBackupRestored(serverID, filepath.Base(backupPath))

    return nil
}
```

**✅ EXCELLENT:**
- Pre-Restore Backup als Rollback-Sicherheit
- Atomic Restore (Rename statt Copy)
- Event Publishing

**🔴 CRITICAL Issue: Status-Check mit String statt Constant**
```go
if server.Status != "stopped" {  // ⚠️ Hardcoded string!
    return fmt.Errorf("server must be stopped before restoring backup")
}
```
- Sollte sein: `server.Status != models.StatusStopped`
- Magic string → anfällig für Typos

#### Backup Directory

**NewBackupService() - Zeile 25-36:**
```go
func NewBackupService(repo, cfg) (*BackupService, error) {
    backupDir := filepath.Join(filepath.Dir(cfg.ServersBasePath), "backups")

    if err := os.MkdirAll(backupDir, 0755); err != nil {
        return nil, err
    }

    return &BackupService{
        backupDir: backupDir,
    }
}
```

**🟡 MEDIUM Issue: Backup Dir nicht konfigurierbar**
- Hardcoded als `../backups` relativ zu ServersBasePath
- Was wenn man Backups auf separatem Volume will? (Best Practice!)
- Sollte sein: `cfg.BackupBasePath`

---

### 6. AuthService - JWT Authentication

**Datei:** [internal/service/auth_service.go](../internal/service/auth_service.go)
**Zeilen:** ~350 (geschätzt)
**Verantwortung:** JWT Token Management, Email Verification, Account Lockout

#### Registration Flow

**Register() - Zeile 42-83:**

```go
func (s *AuthService) Register(email, password, username string) (*models.User, error) {
    // Check if email already exists
    _, err := s.userRepo.FindByEmail(email)
    if err == nil {
        return nil, models.ErrEmailAlreadyExists
    }

    // Create user
    user := &models.User{
        Email:         email,
        Username:      username,
        Balance:       0.0,
        IsActive:      true,
        IsAdmin:       false,
        EmailVerified: false, // ⚠️ Not verified yet
    }

    // Hash password
    user.SetPassword(password)

    // Generate email verification token
    verificationToken := user.GenerateVerificationToken()

    // Save to database
    s.userRepo.Create(user)

    // Send verification email
    if err := s.emailService.SendVerificationEmail(user.Email, user.Username, verificationToken); err != nil {
        // Log error but don't fail registration ⚠️
        // User can request a new verification email later
        return user, nil
    }

    return user, nil
}
```

**✅ GOOD:**
- Email Verification Flow implementiert
- Error bei Email-Send failet Registration nicht
- Token-Generierung in User Model (Encapsulation)

**🔴 CRITICAL Issue: Email Service in MOCK MODE!**
- SendVerificationEmail() ruft MockEmailSender auf (#1 in BUGS.md)
- User bekommt keine Verification Email
- Account bleibt unverifiziert → kann nicht einloggen

#### Login mit Security Features

**Login() - Zeile 85-150:**

```go
func (s *AuthService) Login(email, password, userAgent, ipAddress string) (string, *models.User, bool, error) {
    user := s.userRepo.FindByEmail(email)

    // Checks:
    if !user.IsActive { return ErrAccountDeactivated }
    if !user.EmailVerified { return ErrEmailNotVerified }
    if user.IsLocked() { return ErrAccountLocked }

    // Verify password
    if !user.CheckPassword(password) {
        // Increment failed login attempts
        lockDuration := user.IncrementFailedLogins()
        s.userRepo.Update(user)

        // Log failed login
        s.securityService.LogSecurityEvent(user.ID, EventLoginFailure, ipAddress, userAgent, false, "Invalid password")

        // If account just got locked, send alert
        if lockDuration > 0 {
            s.securityService.LogSecurityEvent(user.ID, EventAccountLocked, ...)
            s.securityService.SendAccountLockedAlert(user, lockDuration)
        }

        return ErrInvalidCredentials
    }

    // Check if this is a trusted device
    _, isTrusted := s.securityService.CheckTrustedDevice(user.ID, userAgent, ipAddress)

    // Successful login - reset failed login attempts
    user.ResetFailedLogins()
    s.userRepo.Update(user)

    // Log successful login
    if isTrusted {
        s.securityService.LogSecurityEvent(user.ID, EventLoginSuccess, ...)
    } else {
        // New device - log and send alert
        s.securityService.LogSecurityEvent(user.ID, EventLoginNewDevice, ...)
        s.securityService.SendNewDeviceAlert(user, deviceName, ipAddress)
    }

    // Generate JWT token
    token := s.GenerateToken(user)

    return token, user, !isTrusted, nil  // Returns requiresTwoFA flag
}
```

**✅ EXCELLENT Security Features:**
- Account Lockout nach Failed Logins (3-Tier: 5/10/15 Attempts)
- Email Verification required
- Trusted Device Detection
- New Device Alerts
- Security Event Logging
- 2FA Support (requiresTwoFA flag)

**🟡 MEDIUM Issue: Keine Transaction**
- `user.IncrementFailedLogins()` + `s.userRepo.Update()` + `s.securityService.LogSecurityEvent()`
- Bei Fehler: Inconsistent State
- Sollte in Transaction sein

**🟢 LOW Issue: JWT Expiry vermutlich hardcoded**
- `GenerateToken()` nicht gezeigt, aber vermutlich hardcoded Expiry
- Sollte konfigurierbar sein (Dev: 1h, Prod: 7d)

---

## Patterns & Architecture

### ✅ GOOD Patterns

1. **Event-Driven Architecture (BillingService)**
   - Decoupled via Event-Bus
   - Automatic tracking ohne direkte Abhängigkeiten

2. **Interface-basierte Dependencies**
   - MinecraftService nutzt Interfaces für Conductor, Velocity, WebSocket
   - Testbar und austauschbar

3. **Background Workers mit Graceful Shutdown**
   - LifecycleService nutzt stopChan
   - Clean Ticker cleanup mit defer

4. **Service Layer Separation**
   - Klare Trennung zwischen Services
   - Jeder Service hat eine klare Verantwortung

### ⚠️ CONCERNING Patterns

1. **Fat Interfaces (ConductorInterface)**
   - 20+ Methods in einem Interface
   - Violation of Interface Segregation Principle
   - Sollte in kleinere Interfaces aufgeteilt werden

2. **Hardcoded Defaults**
   - `MaxPlayers = 20`
   - `AutoShutdownEnabled = true`
   - Worker Intervals (5 Minuten, 1 Stunde)

3. **Deprecated Code nicht entfernt**
   - `VelocityServiceInterface` als DEPRECATED markiert
   - `RecordServerStarted()` DEPRECATED aber public
   - Sollte nach Migration entfernt werden

4. **Missing Implementations**
   - Archive Worker (TODO Comment)
   - Storage Usage Tracking (StorageGB = 0)

---

## Service 7: SecurityService - Device Trust & Security Events

**Datei:** [internal/service/security_service.go](../internal/service/security_service.go) (164 Zeilen)
**Verantwortung:** Trusted Device Management, Security Event Logging
**Dependencies:** EmailService

### Architektur

```go
type SecurityService struct {
    db           *gorm.DB
    emailService *EmailService
}
```

**Key Features:**
- **Device Fingerprinting:** `GenerateDeviceID(userAgent, ipAddress)` → SHA-256
- **30-Day Trust Period:** Auto-renewal on each login
- **Security Event Logging:** Failed logins, account locks, device changes
- **Email Alerts:** New device, account locked, password changed

### Important Methods

**CheckTrustedDevice() - Zeile 26-42:**
```go
func (s *SecurityService) CheckTrustedDevice(userID, userAgent, ipAddress string) (*models.TrustedDevice, bool) {
    deviceID := models.GenerateDeviceID(userAgent, ipAddress)

    var device models.TrustedDevice
    err := s.db.Where("user_id = ? AND device_id = ? AND is_active = ? AND expires_at > ?",
        userID, deviceID, true, time.Now()).First(&device).Error

    if err != nil {
        return nil, false
    }

    // Renew trust for another 30 days
    device.Renew()
    s.db.Save(&device)  // ⚠️ Error not checked!

    return &device, true
}
```

**Issues:**
- Line 39: `s.db.Save(&device)` error is ignored
- Hardcoded 30-day trust period (Line 37, 55)

**TrustNewDevice() - Zeile 45-70:**
- Creates new TrustedDevice entry
- 30-day expiration with auto-renewal
- Logs device addition

**CleanupExpiredDevices() - Zeile 148-163:**
- Removes expired/inactive devices
- ⚠️ **NOT CALLED ANYWHERE** - No worker started!

### Bewertung

✅ **Good:**
- Clean device trust mechanism
- Comprehensive security event logging
- Integration with email alerts

⚠️ **Issues:**
- No error handling for device renewal save
- Hardcoded 30-day trust period
- Cleanup function exists but is never invoked
- No background worker to periodically clean expired devices

---

## Service 8: OAuthService - Social Login Integration

**Datei:** [internal/service/oauth_service.go](../internal/service/oauth_service.go) (484 Zeilen)
**Verantwortung:** OAuth 2.0 Authentication (Discord, Google, GitHub)
**Dependencies:** UserRepository, SecurityService, EmailService

### Architektur

```go
type OAuthService struct {
    db              *gorm.DB
    userRepo        *repository.UserRepository
    cfg             *config.Config
    securityService *SecurityService
    emailService    *EmailService
}
```

**Supported Providers:**
- **Discord:** identify + email scopes
- **Google:** openid + email + profile scopes
- **GitHub:** user:email scope

### Key Features

**1. CSRF Protection via State Tokens**
```go
// GenerateAuthURL() - Zeile 95-141
state, err := generateRandomState()  // 32 bytes base64
oauthState := &models.OAuthState{
    State:     state,
    Provider:  provider,
    ExpiresAt: time.Now().Add(10 * time.Minute),  // ⚠️ Hardcoded!
}
s.db.Create(oauthState)
```

**2. Token Exchange & User Info Fetching**
```go
// HandleCallback() - Zeile 144-219
tokenResp := s.exchangeCodeForToken(provider, code)
userInfo := s.getUserInfo(provider, tokenResp.AccessToken)
user, isNewUser, isNewDevice, err := s.findOrCreateUser(provider, userInfo, tokenResp, userAgent, ipAddress)
```

**3. Find-or-Create User Logic**
```go
// findOrCreateUser() - Zeile 358-438
// Try find existing OAuth account
oauthAccount := db.Where("provider = ? AND provider_id = ?", provider, userInfo.ID)

if not found {
    // Try find user by email
    user := userRepo.FindByEmail(userInfo.Email)

    if user == nil {
        // Create new user
        user = &models.User{
            Email: userInfo.Email,
            Password: generateRandomPassword(),  // ⚠️ Error ignored!
            EmailVerified: userInfo.Verified,
            Balance: 0.0,
        }
        userRepo.Create(user)  // ⚠️ No transaction!
    }

    // Link OAuth account
    db.Create(&models.OAuthAccount{...})  // ⚠️ Separate operation!
}
```

### Critical Issues

**1. AuthService instantiated inline (Line 182-188):**
```go
func (s *OAuthService) HandleCallback(...) {
    authService := &AuthService{  // ⚠️ Should be injected!
        userRepo:        s.userRepo,
        cfg:             s.cfg,
        emailService:    s.emailService,
        securityService: s.securityService,
    }
    token, err := authService.GenerateToken(user)
}
```

**2. No transaction for user creation + OAuth link (Line 394-427):**
- User is created first (line 405)
- OAuth account is created after (line 427)
- If OAuth account creation fails → orphaned user in DB
- Should use `db.Transaction()`

**3. Hardcoded timeouts:**
- State expiration: 10 minutes (Line 111)
- HTTP client timeout: 10 seconds (Line 254, 302)

**4. Error ignored in generateRandomPassword() (Line 452):**
```go
func generateRandomPassword() string {
    b := make([]byte, 32)
    rand.Read(b)  // ⚠️ Error not checked!
    return base64.URLEncoding.EncodeToString(b)
}
```

**5. No cleanup worker for expired OAuth states:**
- States accumulate in database
- No worker to delete expired states

**6. Tokens stored in plain text (Line 367, 421-422):**
```go
oauthAccount.AccessToken = tokenResp.AccessToken    // ⚠️ Plain text!
oauthAccount.RefreshToken = tokenResp.RefreshToken  // ⚠️ Plain text!
s.db.Save(&oauthAccount)
```
- Security risk if database is compromised
- Should encrypt or hash tokens

### Bewertung

✅ **Good:**
- Comprehensive OAuth 2.0 implementation
- CSRF protection with state tokens
- Support for 3 major providers
- Find-or-create user logic
- New device detection and alerts

🔴 **Critical:**
- No transaction for user + OAuth account creation
- Tokens stored in plain text

⚠️ **Medium:**
- AuthService instantiated inline
- No cleanup worker for expired states
- Error ignored in password generation
- Hardcoded timeouts

---

## Service 9: RecoveryService - Automatic Crash Detection & Recovery

**Datei:** [internal/service/recovery_service.go](../internal/service/recovery_service.go) (618 Zeilen)
**Verantwortung:** Crash Detection, Log Analysis, Auto-Recovery
**Dependencies:** ServerRepository, DockerService

### Architektur

```go
type RecoveryService struct {
    serverRepo    *repository.ServerRepository
    dockerService *docker.DockerService
    cfg           *config.Config
    wsHub         WebSocketHubInterface
    recoveryQueue chan *models.MinecraftServer  // Buffer: 10
    stopChan      chan struct{}
}
```

**Recovery Flow:**
```
Container Crash → Log Analysis → Determine Cause → Apply Strategy → Restart/Error
```

### Crash Detection Strategies

**1. System OOM (FATAL) - Zeile 183-189:**
```go
if strings.Contains(logsLower, "insufficient memory") ||
   strings.Contains(logsLower, "cannot allocate memory") {
    return "system_oom"
}
```
→ **NO RESTART** - Host has insufficient RAM, restart will loop forever

**2. Java OOM (Recoverable) - Zeile 192-195:**
```go
if strings.Contains(logsLower, "java.lang.outofmemoryerror") {
    return "oom"
}
```
→ **Restart** - Might help if temporary spike

**3. Config Corruption - Zeile 174-180:**
```go
if strings.Contains(logsLower, "numberformatexception") ||
   strings.Contains(logsLower, "for input string: \"default\"") {
    return "config_corruption"
}
```
→ **Fix + Restart** - Run `fixPaperConfig()` script

**4. Version Mismatch - Zeile 169-172:**
```go
if strings.Contains(logsLower, "chunk saved with newer version") {
    return "version_mismatch"
}
```
→ **No Auto-Recovery** - User must update version manually

**5. Port Conflict - Zeile 198-201:**
```go
if strings.Contains(logsLower, "address already in use") {
    return "port_conflict"
}
```
→ **Cleanup + Restart**

### Config Repair Logic

**fixPaperConfig() - Zeile 247-327:**

Fixes Paper config corruption caused by previous bug:
```go
// Step 1: Fix max-leash-distance (ONLY field that expects float)
fixedContent = strings.ReplaceAll(fixedContent, "max-leash-distance: default", "max-leash-distance: 10.0")

// Step 2: Fix ALL other fields that were incorrectly set to 10.0
invalidFloats := []string{
    "auto-save-interval: 10.0",
    "delay-chunk-unloads-by: 10.0",
    // ... 8 more fields
}
for _, invalidField := range invalidFloats {
    correctField := strings.Replace(invalidField, ": 10.0", ": default", 1)
    fixedContent = strings.ReplaceAll(fixedContent, invalidField, correctField)
}

// Create backup before writing
backupFile := fmt.Sprintf("%s.backup.%d", configFile, time.Now().Unix())
os.WriteFile(backupFile, []byte(originalContent), 0644)
os.WriteFile(configFile, []byte(fixedContent), 0644)
```

### Recovery Queue

**processRecoveryQueue() - Zeile 77-86:**
```go
func (s *RecoveryService) processRecoveryQueue() {
    for {
        select {
        case <-s.stopChan:
            return
        case server := <-s.recoveryQueue:
            s.attemptRecovery(server)  // ⚠️ No panic recovery!
        }
    }
}
```

### Critical Limitations

**Remote Node Recovery NOT SUPPORTED (Line 519-527):**
```go
if !s.isLocalNode(server.NodeID) {
    logger.Warn("Container recovery not yet supported for remote nodes")
    server.Status = models.StatusError
    return false
}
```

**Magic String Detection (Line 615-617):**
```go
func (s *RecoveryService) isLocalNode(nodeID string) bool {
    return nodeID == "" || nodeID == "local-node"  // ⚠️ Magic string!
}
```

### Bewertung

✅ **Excellent:**
- Sophisticated crash detection with log analysis
- Multiple recovery strategies based on crash cause
- Config repair script for Paper corruption
- System OOM vs Java OOM distinction (prevents infinite loops)
- Recovery queue with buffering
- WebSocket integration for real-time notifications
- Backup creation before config repair

🔴 **Critical:**
- Remote node recovery NOT IMPLEMENTED
- Tokens stored in plain text in OAuth

⚠️ **Medium:**
- No panic recovery in processRecoveryQueue goroutine
- Magic string "local-node" for node detection
- No retry limit (can loop indefinitely)
- Hardcoded queue size (10)
- Hardcoded timeout values (30s stop, 90s ready)
- Direct Docker client usage instead of dockerService methods

---

## Service 10: ConfigService - Configuration Management with Audit Trail

**Datei:** [internal/service/config_service.go](../internal/service/config_service.go) (664 Zeilen)
**Verantwortung:** Server Configuration Changes, Audit Trail, Validation
**Dependencies:** ServerRepository, ConfigChangeRepository, DockerService, BackupService

### Architektur

```go
type ConfigService struct {
    serverRepo       *repository.ServerRepository
    configChangeRepo *repository.ConfigChangeRepository
    dockerService    *docker.DockerService
    backupService    *BackupService
    motdService      *MOTDService
}
```

**Config Change Flow:**
```
API Request → Validate → Create Audit Record → Backup → Apply → Container Recreate → Complete
```

### Supported Configuration Changes

**Phase 1: Core Settings**
- `ram_mb` → Validated (2048, 4096, 8192, 16384) → Requires Restart
- `minecraft_version` → Requires Restart
- `max_players` → Requires Restart
- `server_type` → Validated (paper/spigot/bukkit) + Compatibility Check → Requires Restart

**Phase 1: Gameplay**
- `gamemode` → Validated (survival/creative/adventure/spectator) → Requires Restart
- `difficulty` → Validated (peaceful/easy/normal/hard) → Requires Restart
- `pvp`, `enable_command_block`, `level_seed` → Requires Restart

**Phase 2: Performance**
- `view_distance` → Validated (2-32 chunks) → Requires Restart
- `simulation_distance` → Validated (3-32 chunks) → Requires Restart

**Phase 2: World Generation**
- `allow_nether`, `allow_end`, `generate_structures`, `world_type`, `bonus_chest`, `max_world_size` → Requires Restart

**Phase 2: Spawn & Network**
- `spawn_protection`, `spawn_animals`, `spawn_monsters`, `spawn_npcs` → Requires Restart
- `max_tick_time`, `network_compression_threshold` → Requires Restart

**Phase 4: Server Description**
- `motd` → Does NOT require restart (writes to server.properties)

### Audit Trail

**ApplyConfigChanges() - Zeile 49-391:**

```go
func (s *ConfigService) ApplyConfigChanges(req ConfigChangeRequest) (*models.ConfigChange, error) {
    // 1. Validate server exists
    server, err := s.serverRepo.FindByID(req.ServerID)

    // 2. Create audit record
    change := &models.ConfigChange{
        ID:       uuid.New().String()[:8],
        ServerID: req.ServerID,
        UserID:   req.UserID,
        Status:   models.ConfigChangeStatusPending,
        OldValue: ...,
        NewValue: ...,
    }

    // 3. Validate all changes
    for key, newValue := range req.Changes {
        // Extensive validation per field type
    }

    // 4. Create backup before changes (if server running)
    if server.Status == models.StatusRunning && requiresRestart {
        s.backupService.CreateBackup(req.ServerID)  // ⚠️ Error logged, not returned
    }

    // 5. Apply changes
    change.Status = models.ConfigChangeStatusApplying
    err = s.applyChanges(server, req.Changes, requiresRestart)

    // 6. Mark as completed
    change.Status = models.ConfigChangeStatusCompleted
    s.configChangeRepo.Update(change)
}
```

### Container Recreation

**applyChanges() - Zeile 394-630:**

```go
func (s *ConfigService) applyChanges(server *models.MinecraftServer, changes map[string]interface{}, requiresRestart bool) error {
    wasRunning := server.Status == models.StatusRunning

    // Update server model (switch-case for all fields)
    server.RAMMb = int(changes["ram_mb"].(float64))
    // ... 30+ more fields

    // Save to database
    s.serverRepo.Update(server)

    // If requires restart and server was running, recreate container
    if requiresRestart && wasRunning {
        // ⚠️ SAFEGUARD: Not yet supported for remote nodes
        if !s.isLocalNode(server.NodeID) {
            return fmt.Errorf("configuration changes requiring restart are not yet supported for remote servers")
        }

        // Stop + Remove old container
        s.dockerService.StopContainer(server.ContainerID, 30)
        s.dockerService.RemoveContainer(server.ContainerID, true)

        // Create new container with updated config
        containerID, err := s.dockerService.CreateContainer(
            server.ID,
            // ... 30+ parameters
        )

        // Start new container
        s.dockerService.StartContainer(containerID)
        s.dockerService.WaitForServerReady(containerID, 90)

        server.Status = models.StatusRunning
        s.serverRepo.Update(server)
    }
}
```

### Validation Examples

**RAM Validation (Line 79-81):**
```go
if !s.isValidRAM(int(ramMb)) {
    return fmt.Errorf("invalid RAM value: %d (must be 2048, 4096, 8192, or 16384)", int(ramMb))
}
```

**Server Type Compatibility (Line 108-110):**
```go
if !s.isCompatibleServerType(string(server.ServerType), newType) {
    return fmt.Errorf("server type change from %s to %s is not supported (only paper/spigot/bukkit are compatible)", ...)
}
```

**View Distance Validation (Line 165-171):**
```go
viewDist := int(newValue.(float64))
if viewDist < 2 || viewDist > 32 {
    return fmt.Errorf("invalid view distance: %d (must be between 2 and 32)", viewDist)
}
```

**MOTD Length Validation (Line 316-319):**
```go
motd := fmt.Sprintf("%v", newValue)
if len(motd) > 512 {
    return fmt.Errorf("MOTD too long: %d characters (max 512)", len(motd))
}
```

### Bewertung

✅ **Excellent:**
- Comprehensive configuration management
- Full audit trail for all changes
- Extensive validation for all config fields
- Automatic backup before changes
- Graceful container recreation
- Support for 30+ config parameters
- MOTD changes without restart

🔴 **Critical:**
- Remote node config changes NOT IMPLEMENTED (Line 508-510)

⚠️ **Medium:**
- Magic string "local-node" for node detection
- Hardcoded timeout values (30s stop, 90s ready)
- Backup failure doesn't block config changes (Line 345)

🟢 **Low:**
- Hardcoded validation values (RAM tiers, view distance ranges, etc.)

---

## Code-Flaws & Issues (Service Layer)

### 🔴 CRITICAL

1. **Archive Worker nicht implementiert** (lifecycle_service.go:36-37)
   - Archive-Phase existiert in Models, aber kein Worker
   - Server werden nie automatisch archiviert
   - Impact: Storage Costs steigen unbegrenzt

2. **Storage Usage nicht getrackt** (billing_service.go:142)
   - `StorageGB = 0` hardcoded
   - Billing unvollständig
   - Impact: User zahlen nicht für Storage

3. **Fat Interface - Interface Segregation Violation** (minecraft_service.go:49-127)
   - ConductorInterface hat 20+ Methods
   - MinecraftService braucht nicht alle
   - Impact: Code schwer wartbar, Tests komplex

### 🟡 MEDIUM

4. **Hardcoded MaxPlayers = 20** (minecraft_service.go:196)
   - Sollte tier-basiert oder user-konfigurierbar sein

5. **Deprecated Code nicht entfernt**
   - VelocityServiceInterface (minecraft_service.go:35-40)
   - RecordServerStarted() (billing_service.go:122-124)
   - Impact: Code-Bloat, Verwirrung

6. **Keine Transaktionen bei Phase Changes** (lifecycle_service.go:102-112)
   - DB Update + Event Publish sollten atomic sein
   - Impact: Inconsistency bei Fehler

7. **Hardcoded Worker Intervals**
   - 5 Minuten Sleep Worker (lifecycle_service.go:34)
   - 1 Stunde Archive Worker (geplant aber nicht implementiert)
   - Impact: Nicht environment-spezifisch konfigurierbar

### 🟢 LOW

8. **AutoShutdownEnabled = true hardcoded** (minecraft_service.go:195)
   - Sollte User's Choice sein
   - Impact: User kann Auto-Shutdown nicht deaktivieren beim Create

---

## Nächste Schritte

- **06-HTTP_API.md** - HTTP Handler Layer analysieren
- **07-DOCKER_INTEGRATION.md** - Container Management analysieren

---

**Status:** Partial Analysis (10/27 Services dokumentiert)
**Analysiert:**
1. MinecraftService (1358 Zeilen) - Server Lifecycle Core
2. BillingService (421 Zeilen) - Event-Driven Usage Tracking
3. LifecycleService (200 Zeilen) - 3-Phase Lifecycle (Archive Worker TODO!)
4. MonitoringService (352 Zeilen) - Auto-Shutdown & Health Checks
5. BackupService (345 Zeilen) - ZIP-based Backups with Rollback
6. AuthService (470 Zeilen) - JWT Authentication & Account Lockout
7. SecurityService (164 Zeilen) - Device Trust & Security Events
8. OAuthService (483 Zeilen) - OAuth 2.0 (Discord/Google/GitHub)
9. RecoveryService (617 Zeilen) - Crash Detection & Auto-Recovery
10. ConfigService (664 Zeilen) - Config Management with Audit Trail

**Verbleibend (17):**
- email_service.go (601 Zeilen) - MockEmailSender (CRITICAL #1 in BUGS.md)
- plugin_service.go, plugin_manager_service.go, plugin_sync_service.go
- file_service.go, filemanager_service.go, file_integration_service.go, file_validator.go, file_metrics.go
- console_service.go, player_list_service.go, motd_service.go
- resource_pack_service.go, world_service.go, template_service.go
- webhook_service.go, backup_scheduler.go, recovery_service.go

**Nächster Schritt:** Weitere Services analysieren oder zu 06-HTTP_API.md weitergehen
**Analysedatum:** 2025-11-13
