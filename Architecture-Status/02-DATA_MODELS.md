# Data Models - Vollständige Code-Analyse

**Verzeichnis:** [internal/models/](../internal/models/)
**Dateien:** 11 Model-Dateien
**ORM:** GORM (gorm.io)
**Datenbank:** PostgreSQL 16

## Übersicht

PayPerPlay nutzt ein **umfassendes Datenmodell** mit 13+ Haupt-Entities. Das System basiert auf GORM mit PostgreSQL als primäre Datenbank.

## Core Models

### 1. MinecraftServer ([server.go](../internal/models/server.go))

**Haupt-Entity** des gesamten Systems.

```go
type MinecraftServer struct {
    gorm.Model
    ID string `gorm:"primaryKey;size:64"`

    // Basic Info
    Name    string
    OwnerID string `gorm:"default:default"` // ⚠️ Hardcoded default

    // Server Configuration
    ServerType       ServerType // paper, spigot, forge, fabric, vanilla, purpur
    MinecraftVersion string
    RAMMb            int     // Booked RAM (customer pays for)
    ActualRAMMB      int     // Actual allocated (after overhead)
    MaxPlayers       int `gorm:"default:20"`
    Port             int `gorm:"unique"`

    // Tier & Plan
    RAMTier      string `gorm:"default:small"` // micro, small, medium, large, xlarge, custom
    Plan         string `gorm:"default:payperplay"` // payperplay, balanced, reserved
    IsCustomTier bool

    // Container Info
    Status      ServerStatus `gorm:"default:queued"`
    ContainerID string
    NodeID      string // Multi-Node: Which node hosts this

    // Lifecycle (3-Phase System)
    LifecyclePhase  LifecyclePhase `gorm:"default:active"` // active, sleep, archived
    ArchivedAt      *time.Time
    ArchiveLocation string

    // Migration Settings
    AllowMigration bool   `gorm:"default:true"`
    MigrationMode  string `gorm:"default:only_offline"` // only_offline, always, never

    // Velocity Integration
    VelocityRegistered  bool
    VelocityServerName  string

    // RCON
    RCONEnabled  bool   `gorm:"default:true"`
    RCONPort     int    `gorm:"default:25575"`
    RCONPassword string `gorm:"default:'minecraft'"`

    // Relations
    UsageLogs []UsageLog `gorm:"foreignKey:ServerID;constraint:OnDelete:CASCADE"`
}
```

**Konstanten:**
- **ServerType:** paper, spigot, forge, fabric, vanilla, purpur (6 Typen)
- **ServerStatus:** queued, stopped, starting, running, stopping, error, sleeping, archiving, archived (9 States)
- **LifecyclePhase:** active, sleep, archived (3 Phasen)

**Methoden:**
- `CalculateTier()` - Auto-Tier-Klassifizierung basierend auf RAM
- `GetHourlyRate()`, `GetMonthlyRate()` - Preisberechnung
- `AllowsConsolidation()` - Prüft ob Migration erlaubt (basierend auf Tier, Plan, User-Präferenz)
- `GetTierDisplayName()`, `GetPlanDisplayName()` - UI-Namen

**⚠️ CRITICAL ISSUES:**

1. **Default OwnerID = "default"** (Zeile 52)
   - Problem: Alle Server ohne User-Zuordnung haben "default" als OwnerID
   - Impact: Multi-Tenancy nicht vollständig implementiert
   - Location: `OwnerID string gorm:"not null;default:default"`

2. **Auskommentierte Gameplay-Settings** (Zeile 67-95)
   - ~30 Gameplay-Settings (Gamemode, Difficulty, PVP, WorldGen, etc.) sind vorhanden
   - Gut durchdacht, aber könnten überladen werden
   - Risiko: Zu viele Konfigurationsoptionen verwirren User

### 2. User ([user.go](../internal/models/user.go))

Umfassendes User-Model mit Auth, Security, OAuth.

```go
type User struct {
    ID        string `gorm:"primaryKey;size:36"` // UUID
    Email     string `gorm:"uniqueIndex"`
    Password  string `json:"-"` // bcrypt hashed
    Username  string
    Balance   float64
    IsActive  bool `gorm:"default:true"`
    IsAdmin   bool `gorm:"default:false"`

    // OAuth
    DiscordID   string `gorm:"uniqueIndex"`
    MicrosoftID string `gorm:"uniqueIndex"`

    // Email Verification
    EmailVerified          bool
    EmailVerificationToken string `json:"-"`
    VerificationExpiresAt  *time.Time

    // Password Reset
    PasswordResetToken string `json:"-"`
    ResetExpiresAt     *time.Time

    // Security
    FailedLoginAttempts int
    LockedUntil         *time.Time
    LastPasswordChange  *time.Time

    // ⚠️ Relationships COMMENTED OUT
    // Servers        []MinecraftServer
    // TrustedDevices []TrustedDevice
    // SecurityEvents []SecurityEvent
}
```

**Methoden:**
- `SetPassword(password)` - bcrypt hashing
- `CheckPassword(password)` - bcrypt verification
- `CanAfford(amount)`, `DeductBalance(amount)`, `AddBalance(amount)` - Billing
- `GenerateVerificationToken()` - 24h Email-Verifizierungs-Token
- `GeneratePasswordResetToken()` - 1h Password-Reset-Token
- `IncrementFailedLogins()` - Account-Lockout (5/10/15 attempts → 15min/1h/24h)
- `ResetFailedLogins()` - Nach erfolgreicher Login

**Account Lockout Policy:**
- 5 Attempts → 15 Minuten Lock
- 10 Attempts → 1 Stunde Lock
- 15 Attempts → 24 Stunden Lock

**🔴 CRITICAL ISSUE:**

**Auskommentierte Relations** (Zeile 42-45)
```go
// Relationships - Temporarily commented out for testing
// Servers        []MinecraftServer `gorm:"foreignKey:OwnerID"`
// TrustedDevices []TrustedDevice   `gorm:"foreignKey:UserID"`
// SecurityEvents []SecurityEvent   `gorm:"foreignKey:UserID"`
```

**Impact:**
- Keine DB-Level Foreign Key Constraints
- Manuelles Query-Management nötig
- Risiko von Orphaned Records
- Performance: N+1 Query Problem

### 3. Tier & Pricing ([tier.go](../internal/models/tier.go))

Tier-basiertes Pricing-System.

```go
const (
    TierMicro   = "micro"   // 2GB
    TierSmall   = "small"   // 4GB
    TierMedium  = "medium"  // 8GB
    TierLarge   = "large"   // 16GB
    TierXLarge  = "xlarge"  // 32GB
    TierCustom  = "custom"  // Non-standard
)

const (
    PlanPayPerPlay = "payperplay" // Aggressive optimization, cheapest
    PlanBalanced   = "balanced"   // Moderate optimization
    PlanReserved   = "reserved"   // No optimization, dedicated
)

var StandardTiers = map[string]int{
    TierMicro:  2048,
    TierSmall:  4096,
    TierMedium: 8192,
    TierLarge:  16384,
    TierXLarge: 32768,
}
```

**Funktionen:**
- `ClassifyTier(ramMB)` - Bestimmt Tier basierend auf RAM
- `GetNearestStandardTier(ramMB)` - Rundet auf nächsten Standard-Tier hoch
- `CalculateHourlyRate(tier, plan, ramMB)` - Stundensatz
- `CalculateMonthlyRate(tier, plan, ramMB)` - Monatssatz (730h)
- `AllowConsolidation(tier)` - Tier-spezifische Consolidation-Regeln
- `GetContainersPerNode(tier, nodeRAMMB)` - Bin-Packing-Berechnung
- `CalculatePerfectPackingNodes(containersByTier, nodeRAMMB)` - Optimale Node-Anzahl

**Player Range Estimates:**
- Micro: 5-10 players
- Small: 10-20 players
- Medium: 20-40 players
- Large: 40-80 players
- XLarge: 80-150 players

**🟡 POTENTIAL ISSUE:**

**Hardcoded vs. Config-Based Tiers** (Zeile 42-66)
```go
// Check if it matches a standard tier
for tier, ram := range StandardTiers {
    if ramMB == ram { return tier }
}

// Check if it matches configured standard tiers (in case they're customized)
if ramMB == cfg.StandardTierMicro { return TierMicro }
// ...
```

**Problem:**
- Zwei Quellen der Wahrheit: `StandardTiers` map UND `config.AppConfig`
- Wenn Config angepasst wird, aber `StandardTiers` nicht → Inkonsistenz
- **Lösung:** Entweder nur Config ODER nur Konstanten nutzen

### 4. Billing Models ([billing.go](../internal/models/billing.go))

Event-basiertes Billing-System.

```go
type BillingEvent struct {
    gorm.Model
    ID string

    ServerID   string
    ServerName string
    OwnerID    string

    EventType BillingEventType // server_started, server_stopped, phase_changed
    Timestamp time.Time

    // Resource config at time of event
    RAMMb            int
    StorageGB        float64
    LifecyclePhase   LifecyclePhase
    PreviousPhase    LifecyclePhase
    MinecraftVersion string

    HourlyRateEUR float64 // Historical rate
    DailyRateEUR  float64
}

type UsageSession struct {
    gorm.Model
    ID string

    ServerID string
    StartedAt time.Time
    StoppedAt *time.Time

    RAMMb            int
    StorageGB        float64
    DurationSeconds  int
    CostEUR          float64
    HourlyRateEUR    float64
}

type PricingConfig struct {
    ActiveRateEURPerGBHour float64 // Default: 0.02 (2 cent/GB-hour)
    SleepRateEURPerGBDay   float64 // Default: 0.00333 (~0.10€/month)
    ArchiveRateEURPerGBDay float64 // Default: 0.00 (free)
}
```

**Billing Flow:**
1. Server startet → `BillingEvent` (server_started)
2. Laufzeit tracken → `UsageSession` (open)
3. Server stoppt → `BillingEvent` (server_stopped), `UsageSession` (closed)
4. Cost berechnet: `DurationSeconds * HourlyRate / 3600`

**3-Phase Pricing:**
- **Phase 1 (Active):** 0.02 EUR/GB-hour
- **Phase 2 (Sleep):** 0.00333 EUR/GB-day (~0.10€/month)
- **Phase 3 (Archive):** FREE

### 5. Plugin Ecosystem ([plugin.go](../internal/models/plugin.go))

Marketplace für Minecraft-Plugins.

```go
type Plugin struct {
    ID          string
    Name        string
    Slug        string `gorm:"uniqueIndex"` // "worldedit"
    Description string
    Author      string
    Category    PluginCategory
    IconURL     string

    Source     PluginSource // modrinth, hangar, spigot, manual
    ExternalID string       // ID at external source

    DownloadCount int
    Rating        float64
    LastSynced    time.Time

    Versions []PluginVersion
}

type PluginVersion struct {
    ID       string
    PluginID string
    Version  string // "7.2.15" (Semantic Versioning)

    MinecraftVersions datatypes.JSON // ["1.20", "1.20.1"] (PostgreSQL JSONB!)
    ServerTypes       datatypes.JSON // ["paper", "spigot"]
    Dependencies      datatypes.JSON // Array of Dependency objects

    DownloadURL string
    FileHash    string // SHA512 (128 hex characters)
    FileSize    int64

    Changelog   string
    ReleaseDate time.Time
    IsStable    bool
}

type InstalledPlugin struct {
    ID        string
    ServerID  string
    PluginID  string
    VersionID string

    Enabled    bool
    AutoUpdate bool // Auto-update to new compatible versions

    InstalledAt time.Time
    LastUpdated time.Time
}
```

**Plugin Sources:**
- Modrinth (primary)
- Hangar (Paper plugins)
- Spigot (legacy)
- Manual (user uploads)

**Kategorien:** world-management, admin-tools, economy, mechanics, protection, social, utility, optimization

**🟢 GOOD DESIGN:**
- JSONB für flexible Compatibility-Arrays
- SHA512 für Integrity-Checks
- Auto-Update Opt-In pro Server
- Versioning mit Semantic Versioning

### 6. Server Files ([server_file.go](../internal/models/server_file.go))

File-Upload-System für Resource Packs, Data Packs, etc.

```go
type ServerFile struct {
    gorm.Model
    ID       string
    ServerID string

    FileType FileType   // resource_pack, data_pack, server_icon, world_gen
    FileName string
    FilePath string     // Relative path from server directory
    Status   FileStatus // uploading, processing, active, inactive, failed

    SHA1Hash string  // Verification
    SizeMB   float64

    Version  int
    IsActive bool // Only one file per type can be active

    Metadata string `gorm:"type:text"` // JSON metadata

    UploadedBy string
    UploadedAt time.Time

    ErrorMessage string
}
```

**File Types:**
- Resource Pack (Client-side textures)
- Data Pack (Server-side logic)
- Server Icon (64x64 PNG)
- World Gen (Custom world generation)

**Versioning:**
- Multiple versions pro File-Type
- Nur eine aktive Version gleichzeitig
- SHA1-Hash für Integritätsprüfung

### 7. Config Changes ([config_change.go](../internal/models/config_change.go))

Audit-Trail für Server-Konfigurationsänderungen.

```go
type ConfigChange struct {
    gorm.Model
    ID string

    ServerID string
    UserID   string // Who made the change

    ChangeType ConfigChangeType // ram, minecraft_version, server_type, etc.
    Status     ConfigChangeStatus // pending, applying, completed, failed, rolled_back

    OldValue string // JSON or simple value
    NewValue string

    RequiresRestart bool
    ErrorMessage    string

    AppliedAt   *time.Time
    CompletedAt *time.Time
}
```

**34 Config Change Types:**
- Core: RAM, Version, Server Type, Max Players
- Gameplay (Phase 1): Gamemode, Difficulty, PVP, Command Blocks, Seed
- Performance (Phase 2): View Distance, Simulation Distance
- World Gen (Phase 2): Nether, End, Structures, World Type, Bonus Chest
- Spawn (Phase 2): Protection, Animals, Monsters, NPCs
- Network (Phase 2): Max Tick Time, Compression Threshold
- MOTD (Phase 4)

**Workflow:**
1. User ändert Config → `ConfigChange` (pending)
2. System wendet an → Status: applying
3. Erfolgreich → Status: completed
4. Fehler → Status: failed (mit ErrorMessage)
5. Rollback möglich → Status: rolled_back

**🟢 GOOD DESIGN:**
- Vollständiger Audit-Trail
- Rollback-Support
- Restart-Tracking (`RequiresRestart`)

### 8. OAuth & Security ([oauth.go](../internal/models/oauth.go), [security.go](../internal/models/security.go))

**OAuth:**
```go
type OAuthAccount struct {
    gorm.Model
    ID           string
    UserID       string
    Provider     OAuthProviderType // discord, google, github, microsoft
    ProviderID   string
    Email        string
    Username     string
    AvatarURL    string
    AccessToken  string `json:"-"` // Never expose
    RefreshToken string `json:"-"`
    ExpiresAt    *time.Time
    Scopes       string
    LastUsedAt   time.Time
}

type OAuthState struct {
    gorm.Model
    State     string // CSRF protection token
    Provider  OAuthProviderType
    ExpiresAt time.Time
    UserAgent string
    IPAddress string
}
```

**Security:**
```go
type TrustedDevice struct {
    gorm.Model
    ID        string
    UserID    string
    DeviceID  string // SHA256(UserAgent + IP-Range)
    Name      string // "Chrome on Windows"
    UserAgent string
    IPAddress string
    LastUsed  time.Time
    ExpiresAt time.Time // 30 days
    IsActive  bool
}

type SecurityEvent struct {
    gorm.Model
    ID        string
    UserID    string
    EventType SecurityEventType // login_success, login_failure, etc.
    IPAddress string
    UserAgent string
    DeviceID  string
    Location  string // City, Country
    Success   bool
    Reason    string
    Metadata  string `gorm:"type:json"`
    Timestamp time.Time
}
```

**Security Event Types (10):**
- login_success, login_failure, login_new_device
- password_changed, email_changed
- account_locked, account_unlocked, account_deleted
- email_verified
- password_reset_request, password_reset_success

**Device Fingerprinting:**
```go
func GenerateDeviceID(userAgent, ipAddress string) string {
    ipRange := extractIPRange(ipAddress) // First 3 octets
    data := fmt.Sprintf("%s:%s", userAgent, ipRange)
    hash := sha256.Sum256([]byte(data))
    return fmt.Sprintf("%x", hash)
}
```

**🟡 ISSUE: Zu simple IP-Range Extraction** (Zeile 63-72)
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
- Funktioniert nur für IPv4
- IPv6 wird nicht korrekt gehandled
- Kein Parsing von CIDR-Notationen
- Kommentar sagt selbst "For production, use proper IP parsing"

### 9. Webhooks ([webhook.go](../internal/models/webhook.go))

Discord Webhook Integration.

```go
type ServerWebhook struct {
    ID         uint
    ServerID   string
    WebhookURL string
    Enabled    bool

    // Event filters
    OnServerStart   bool
    OnServerStop    bool
    OnServerCrash   bool
    OnPlayerJoin    bool
    OnPlayerLeave   bool
    OnBackupCreated bool
}
```

**Discord Payload:**
```go
type DiscordWebhookPayload struct {
    Content   string
    Username  string
    AvatarURL string
    Embeds    []DiscordEmbed
}

type DiscordEmbed struct {
    Title       string
    Description string
    Color       int
    Fields      []DiscordEmbedField
    Footer      *DiscordEmbedFooter
    Timestamp   string
}
```

### 10. Backup Schedule ([backup_schedule.go](../internal/models/backup_schedule.go))

Automatisierte Backups.

```go
type ServerBackupSchedule struct {
    ID        uint
    ServerID  string `gorm:"uniqueIndex"` // One schedule per server
    Enabled   bool

    Frequency      string // daily, weekly, custom
    ScheduleTime   string // "03:00" (HH:MM)
    MaxBackups     int    // Auto-delete old backups

    LastBackupAt   *time.Time
    NextBackupAt   *time.Time
    LastBackupSize string
    FailureCount   int
}
```

### 11. Server Template ([template.go](../internal/models/template.go))

Vorkonfigurierte Server-Templates (keine GORM-Persistenz!).

```go
type ServerTemplate struct {
    ID          string
    Name        string
    Description string
    Category    string // vanilla, modded, minigame
    Icon        string
    Version     string
    ServerType  string
    Memory      int // Recommended RAM
    Properties  map[string]interface{} // server.properties overrides
    Plugins     []string // Plugin IDs to pre-install
    Mods        []string
    WorldPreset string // flat, void, skyblock
    Tags        []string
    Popular     bool
}
```

**🟡 DESIGN NOTE:** Kein GORM-Model, wird aus JSON geladen ([templates/server-templates.json](../templates/server-templates.json)).

### 12. System Events ([system_event.go](../internal/models/system_event.go))

Event-Bus Storage.

```go
type SystemEvent struct {
    gorm.Model
    EventID   string `gorm:"uniqueIndex"`
    Type      string
    Timestamp time.Time
    Source    string
    ServerID  string
    UserID    string
    Data      datatypes.JSON `gorm:"type:jsonb"`
}
```

**Event-Bus Integration:** Siehe [09-EVENT_SYSTEM.md](09-EVENT_SYSTEM.md).

## Datenbank-Schema-Übersicht

**Tabellen (13+):**
1. `minecraft_servers` - Haupt-Servers
2. `usage_logs` - Server-Usage (veraltet? Siehe UsageSession in billing)
3. `users` - User-Accounts
4. `billing_events` - Event-basiertes Billing
5. `usage_sessions` - Billing-Sessions
6. `plugins` - Plugin-Marketplace
7. `plugin_versions` - Plugin-Versionen
8. `installed_plugins` - Server-spezifische Installations
9. `server_files` - Uploaded Files
10. `config_changes` - Config-Audit-Trail
11. `oauth_accounts` - OAuth-Verknüpfungen
12. `oauth_states` - CSRF-Token für OAuth
13. `trusted_devices` - Device Trust (30 Tage)
14. `security_events` - Security-Log
15. `server_webhooks` - Discord Webhooks
16. `server_backup_schedules` - Backup-Config
17. `system_events` - Event-Bus-Storage

**JSONB-Felder (PostgreSQL-spezifisch):**
- `PluginVersion.MinecraftVersions` (Array von Strings)
- `PluginVersion.ServerTypes` (Array von Strings)
- `PluginVersion.Dependencies` (Array von Dependency-Objekten)
- `SecurityEvent.Metadata` (Flexible Event-Daten)
- `SystemEvent.Data` (Event-Bus-Payload)

## GORM-Patterns

### BeforeCreate Hooks

**UUID-Generierung:**
```go
func (u *User) BeforeCreate(tx *gorm.DB) error {
    if u.ID == "" {
        u.ID = uuid.New().String()
    }
    return nil
}
```

**Verwendet von:** User, Plugin, PluginVersion, InstalledPlugin, TrustedDevice, SecurityEvent

### Custom Table Names

```go
func (MinecraftServer) TableName() string {
    return "minecraft_servers"
}
```

**Alle Models mit Custom Table Name:** MinecraftServer, UsageLog, Plugin, PluginVersion, InstalledPlugin, ServerFile, ConfigChange, SystemEvent

### Soft Deletes

Genutzt von allen Models via `gorm.Model` (enthält `DeletedAt gorm.DeletedAt`).

## Code-Flaws & Potenzielle Probleme

### 🔴 CRITICAL

1. **User-Relations auskommentiert** (user.go:42-45)
   - Keine Foreign Key Constraints auf DB-Level
   - Manuelles Relationship-Management nötig
   - Risk: Orphaned Records

2. **Hardcoded Default OwnerID = "default"** (server.go:52)
   - Multi-Tenancy nicht vollständig implementiert
   - Alle User-losen Server haben gleichen Owner

3. **Dual Source of Truth für Tiers** (tier.go:27-66)
   - `StandardTiers` map UND `config.AppConfig` Tiers
   - Potenzielle Inkonsistenz bei Config-Änderungen

### 🟡 MEDIUM

4. **IP-Range Extraction zu simpel** (security.go:63-72)
   - Nur IPv4
   - Kein IPv6-Support
   - Selbst als "For production, use proper IP parsing" markiert

5. **Keine JSONB-Validierung**
   - JSONB-Felder (PluginVersion.Dependencies, SecurityEvent.Metadata, etc.)
   - Keine Schema-Validierung
   - Risiko: Invalide JSON in DB

6. **UsageLog vs. UsageSession Redundanz**
   - `MinecraftServer.UsageLogs` (server.go:131)
   - `UsageSession` (billing.go:45)
   - Scheinen gleichen Zweck zu erfüllen - welches ist aktuell?

### 🟢 LOW

7. **RCON Default Password = "minecraft"** (server.go:128)
   - Hardcoded default
   - Security: Sollte random generiert werden

8. **Email-Token-Länge**
   - Verification: 24h (gut)
   - Reset: 1h (gut)
   - Aber: UUID als Token (könnte predictable sein? → UUID v4 ist random, also OK)

9. **ServerTemplate nicht in DB**
   - Wird aus JSON-Datei geladen
   - Keine Versionierung
   - Risiko: Deployment ohne Templates möglich

## Best Practices (Positiv)

✅ **Gute Patterns:**
1. JSON-Tags mit `-` für sensitive Felder (Password, Tokens)
2. BeforeCreate Hooks für UUID-Generierung
3. Method Receivers für Business-Logic (User.SetPassword, Server.AllowsConsolidation)
4. Ausführliche Konstanten (34 ConfigChangeTypes!)
5. Audit-Trails (ConfigChange, SecurityEvent)
6. JSONB für flexible Schema (Plugin-Kompatibilität)
7. Soft Deletes standardmäßig aktiv

## Abhängigkeiten

**Externe Packages:**
- `gorm.io/gorm` - ORM
- `gorm.io/datatypes` - JSONB-Support
- `github.com/google/uuid` - UUID-Generierung
- `golang.org/x/crypto/bcrypt` - Password-Hashing

**Interne Packages:**
- `github.com/payperplay/hosting/pkg/config` - Config für Tier-Pricing

## Nächste Schritte

Siehe [03-DATABASE_LAYER.md](03-DATABASE_LAYER.md) für Repository-Pattern-Analyse.
