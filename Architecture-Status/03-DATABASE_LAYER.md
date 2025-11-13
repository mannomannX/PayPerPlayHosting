# Database Layer - Repository Pattern Analyse

**Verzeichnis:** [internal/repository/](../internal/repository/)
**Dateien:** 7 Repository-Dateien
**Pattern:** Repository Pattern mit Interface-Abstraktion
**ORM:** GORM
**Unterstützte Datenbanken:** PostgreSQL (primär), SQLite (legacy)

## Übersicht

Die Datenbankschicht nutzt das **Repository Pattern** zur Abstraktion des Datenbankzugriffs. Alle Repositories nutzen GORM als ORM und implementieren CRUD-Operationen für ihre jeweiligen Models.

## Architektur-Pattern

### Provider Interface ([database_interface.go](../internal/repository/database_interface.go))

**Abstraktions-Layer für Multi-Database-Support:**

```go
type DatabaseProvider interface {
    GetDB() *gorm.DB
    Migrate(models ...interface{}) error
    Close() error
    Ping() error
}
```

**Implementierungen:**
1. `SQLiteProvider` - Legacy (Zeile 18-45)
2. `PostgreSQLProvider` - Production (Zeile 47-75)

**⚠️ Kommentar-Inkonsistenz:**
```go
// PostgreSQLProvider implements DatabaseProvider for PostgreSQL
// Placeholder for future implementation  // ⚠️ VERALTET!
type PostgreSQLProvider struct {
    db *gorm.DB
}
```

**Problem:** Kommentar sagt "Placeholder for future", aber PostgreSQL IST implementiert und wird in Production genutzt (siehe [database.go:32-43](../internal/repository/database.go:32-43)).

### Repository Interface Pattern

**Clean Architecture Interface:**
```go
type ServerRepositoryInterface interface {
    Create(server *models.MinecraftServer) error
    FindByID(id string) (*models.MinecraftServer, error)
    FindAll() ([]models.MinecraftServer, error)
    FindByOwner(ownerID string) ([]models.MinecraftServer, error)
    Update(server *models.MinecraftServer) error
    Delete(id string) error
    GetUsedPorts() ([]int, error)

    // Usage logs
    CreateUsageLog(log *models.UsageLog) error
    GetActiveUsageLog(serverID string) (*models.UsageLog, error)
    UpdateUsageLog(log *models.UsageLog) error
    GetServerUsageLogs(serverID string) ([]models.UsageLog, error)
}

// Compile-time interface check
var _ ServerRepositoryInterface = (*ServerRepository)(nil)
```

**🟢 GOOD:** Compile-time Interface-Validierung (Zeile 96).

**🟡 INCOMPLETE:** Nur `ServerRepository` hat ein Interface, andere Repositories nicht.

## Database Initialization ([database.go](../internal/repository/database.go))

### Initialisierungssequenz

```go
func InitDB(cfg *config.Config) error {
    // 1. GORM Config mit Logger
    gormConfig := &gorm.Config{
        Logger: logger.Default.LogMode(logger.Silent),
    }
    if cfg.Debug {
        gormConfig.Logger = logger.Default.LogMode(logger.Info)
    }

    // 2. Database Type Switch
    switch cfg.DatabaseType {
    case "postgres", "postgresql":
        DB, err = gorm.Open(postgres.Open(cfg.DatabaseURL), gormConfig)
        dbProvider = &PostgreSQLProvider{db: DB}
    default:
        return fmt.Errorf("unsupported database type: %s", cfg.DatabaseType)
    }

    // 3. Auto-Migrate (17 Models!)
    err = dbProvider.Migrate(
        &models.User{},
        &models.MinecraftServer{},
        &models.UsageLog{},
        // ... 14 weitere Models
    )
}
```

**Auto-Migrated Models (17):**
1. User
2. MinecraftServer
3. UsageLog
4. ConfigChange
5. ServerFile
6. ServerWebhook
7. ServerBackupSchedule
8. BillingEvent
9. UsageSession
10. TrustedDevice
11. SecurityEvent
12. OAuthAccount
13. OAuthState
14. SystemEvent
15. Plugin
16. PluginVersion
17. InstalledPlugin

**🔴 ISSUE: SQLite-Support entfernt, aber Code noch vorhanden**

```go
switch cfg.DatabaseType {
case "postgres", "postgresql":
    // PostgreSQL code
default:
    return fmt.Errorf("unsupported database type: %s (only 'postgres' is supported)", cfg.DatabaseType)
}
```

**Problem:** SQLiteProvider existiert noch, wird aber nicht mehr initialisiert. Dead Code.

### Password Masking (Security)

```go
func maskPassword(url string) string {
    // postgres://user:PASSWORD@host:port/db -> postgres://user:****@host:port/db
    // Find password between : and @
    // ...
    return url[:start] + "****" + url[end:]
}
```

**🟡 ISSUE: Zu simple Implementierung** (Zeile 89-113)

**Probleme:**
- Magic Number `i > 10` (Zeile 99) - warum 10?
- Funktioniert nicht für URLs mit Ports ohne Auth: `postgres://host:5432/db`
- Funktioniert nicht für URLs mit @ im Passwort

**Vorschlag:** URL-Parser nutzen (`url.Parse()`)

## Repository Implementations

### 1. ServerRepository ([server_repository.go](../internal/repository/server_repository.go))

**Besonderheit: Extensive Unscoped() Usage**

```go
func (r *ServerRepository) FindByID(id string) (*models.MinecraftServer, error) {
    var server models.MinecraftServer
    // Use Unscoped() to find soft-deleted servers (needed for cleanup)
    err := r.db.Unscoped().Where("id = ?", id).First(&server).Error
    return &server, err
}

func (r *ServerRepository) FindAll() ([]models.MinecraftServer, error) {
    var servers []models.MinecraftServer
    // Use Unscoped() to include soft-deleted servers (for cleanup)
    err := r.db.Unscoped().Find(&servers).Error
    return servers, err
}

func (r *ServerRepository) GetUsedPorts() ([]int, error) {
    var ports []int
    // Use Unscoped() to include ports from soft-deleted servers (they still block the port)
    err := r.db.Unscoped().Model(&models.MinecraftServer{}).
        Where("port IS NOT NULL").
        Pluck("port", &ports).Error
    return ports, err
}

func (r *ServerRepository) Delete(id string) error {
    // Use Unscoped() to perform a hard delete (not soft delete)
    return r.db.Unscoped().Where("id = ?", id).Delete(&models.MinecraftServer{}).Error
}
```

**Rationale (aus Kommentaren):**
- FindByID/FindAll: "needed for cleanup" → Soft-deleted Servers müssen gesehen werden
- GetUsedPorts: "they still block the port" → Auch gelöschte Server blockieren Ports
- Delete: "hard delete" → Ports wirklich freigeben

**🟡 DESIGN CONCERN:**

**Problem:** ALLE FindByID/FindAll nutzen Unscoped() → User sehen auch gelöschte Server!

**Besserer Ansatz:**
```go
func (r *ServerRepository) FindByID(id string) (*models.MinecraftServer, error) {
    // Normal query WITHOUT Unscoped
    err := r.db.Where("id = ?", id).First(&server).Error
}

func (r *ServerRepository) FindByIDIncludingDeleted(id string) (*models.MinecraftServer, error) {
    // Separate method WITH Unscoped for cleanup tasks
    err := r.db.Unscoped().Where("id = ?", id).First(&server).Error
}
```

**Current Mitigation:** Services müssen manuell auf `DeletedAt` prüfen.

### 2. UserRepository ([user_repository.go](../internal/repository/user_repository.go))

**Standard CRUD + OAuth + Auth:**

```go
type UserRepository struct {
    db *gorm.DB
}

// Standard CRUD
func (r *UserRepository) Create(user *models.User) error
func (r *UserRepository) FindByID(id string) (*models.User, error)
func (r *UserRepository) FindByEmail(email string) (*models.User, error)
func (r *UserRepository) Update(user *models.User) error
func (r *UserRepository) Delete(id string) error
func (r *UserRepository) FindAll() ([]models.User, error)

// OAuth Lookups
func (r *UserRepository) FindByDiscordID(discordID string) (*models.User, error)
func (r *UserRepository) FindByMicrosoftID(microsoftID string) (*models.User, error)

// Balance Operations (Atomic!)
func (r *UserRepository) UpdateBalance(userID string, newBalance float64) error
func (r *UserRepository) IncrementBalance(userID string, amount float64) error
func (r *UserRepository) DecrementBalance(userID string, amount float64) error

// Auth Token Lookups
func (r *UserRepository) FindByVerificationToken(token string) (*models.User, error)
func (r *UserRepository) FindByResetToken(token string) (*models.User, error)
```

**🟢 GOOD: Atomic Balance Operations**

```go
func (r *UserRepository) IncrementBalance(userID string, amount float64) error {
    return r.db.Model(&models.User{}).Where("id = ?", userID).
        Update("balance", gorm.Expr("balance + ?", amount)).Error
}
```

**Warum gut?**
- Nutzt SQL-Expression für atomare Updates
- Kein Race Condition-Risiko (Read → Modify → Write)
- Single Query, keine Transaction nötig

### 3. PluginRepository ([plugin_repository.go](../internal/repository/plugin_repository.go))

**Komplexestes Repository mit 3 Entity-Typen:**

```go
// Plugin CRUD
func (r *PluginRepository) CreatePlugin(plugin *models.Plugin) error
func (r *PluginRepository) FindPluginByID(id string) (*models.Plugin, error)
func (r *PluginRepository) FindPluginBySlug(slug string) (*models.Plugin, error)
func (r *PluginRepository) FindPluginByExternalID(source, externalID) (*models.Plugin, error)
func (r *PluginRepository) UpdatePlugin(plugin *models.Plugin) error
func (r *PluginRepository) DeletePlugin(id string) error
func (r *PluginRepository) ListPlugins(category, source, limit) ([]models.Plugin, error)
func (r *PluginRepository) SearchPlugins(searchTerm, limit) ([]models.Plugin, error)

// PluginVersion CRUD
func (r *PluginRepository) CreatePluginVersion(version *models.PluginVersion) error
func (r *PluginRepository) FindVersionByID(id string) (*models.PluginVersion, error)
func (r *PluginRepository) FindVersionsByPluginID(pluginID string) ([]models.PluginVersion, error)
func (r *PluginRepository) FindLatestStableVersion(pluginID string) (*models.PluginVersion, error)
func (r *PluginRepository) FindCompatibleVersions(pluginID, mcVersion, serverType) ([]models.PluginVersion, error)
func (r *PluginRepository) UpdatePluginVersion(version *models.PluginVersion) error
func (r *PluginRepository) DeletePluginVersion(id string) error

// InstalledPlugin CRUD
func (r *PluginRepository) InstallPlugin(installed *models.InstalledPlugin) error
func (r *PluginRepository) FindInstalledPlugin(serverID, pluginID) (*models.InstalledPlugin, error)
func (r *PluginRepository) ListInstalledPlugins(serverID string) ([]models.InstalledPlugin, error)
func (r *PluginRepository) UpdateInstalledPlugin(installed *models.InstalledPlugin) error
func (r *PluginRepository) UninstallPlugin(serverID, pluginID) error
func (r *PluginRepository) FindPluginsWithAutoUpdate(pluginID) ([]models.InstalledPlugin, error)

// Batch Operations (Sync)
func (r *PluginRepository) UpsertPlugin(plugin *models.Plugin) error
func (r *PluginRepository) UpsertPluginVersion(version *models.PluginVersion) error
func (r *PluginRepository) GetPluginStats() (map[string]interface{}, error)
```

**Eager Loading mit Preload:**
```go
func (r *PluginRepository) FindPluginByID(id string) (*models.Plugin, error) {
    var plugin models.Plugin
    err := r.db.Preload("Versions").First(&plugin, "id = ?", id).Error
    return &plugin, err
}

func (r *PluginRepository) ListInstalledPlugins(serverID string) ([]models.InstalledPlugin, error) {
    var installed []models.InstalledPlugin
    err := r.db.Preload("Plugin").Preload("Version"). // Double Preload!
        Where("server_id = ?", serverID).
        Order("installed_at DESC").
        Find(&installed).Error
    return installed, err
}
```

**🟢 GOOD:** Nutzt Preload für N+1-Query-Vermeidung.

**PostgreSQL-spezifische Features:**

```go
func (r *PluginRepository) SearchPlugins(searchTerm string, limit int) ([]models.Plugin, error) {
    query := r.db.Model(&models.Plugin{}).
        Where("name ILIKE ? OR description ILIKE ?", "%"+searchTerm+"%", "%"+searchTerm+"%")
    // ILIKE = Case-insensitive LIKE (PostgreSQL-spezifisch!)
}
```

**🟡 TODO in Code:**

```go
func (r *PluginRepository) FindCompatibleVersions(pluginID, mcVersion, serverType) ([]models.PluginVersion, error) {
    // Query versions and filter in-memory for JSON array contains
    // PostgreSQL JSON operators could be used for better performance  // ⚠️ TODO
    err := r.db.Where("plugin_id = ?", pluginID).
        Order("release_date DESC").
        Find(&versions).Error

    // Filter compatible versions (would be more efficient with JSON operators)
    // For now, we return all and filter in service layer  // ⚠️ Ineffizient!
    return versions, nil
}
```

**Problem:** ALLE Versionen werden geladen, Filterung in Service-Layer.

**Bessere Lösung (PostgreSQL JSONB):**
```go
// PostgreSQL JSONB contains operator
query := r.db.Where("plugin_id = ?", pluginID).
    Where("minecraft_versions @> ?", fmt.Sprintf(`["%s"]`, mcVersion)).
    Where("server_types @> ?", fmt.Sprintf(`["%s"]`, serverType))
```

### 4. FileRepository ([file_repository.go](../internal/repository/file_repository.go))

**File-spezifische Operationen:**

```go
// Standard CRUD
func (r *FileRepository) Create(file *models.ServerFile) error
func (r *FileRepository) FindByID(id string) (*models.ServerFile, error)
func (r *FileRepository) FindByServerID(serverID string) ([]models.ServerFile, error)
func (r *FileRepository) Update(file *models.ServerFile) error
func (r *FileRepository) Delete(id string) error

// File-Type-spezifisch
func (r *FileRepository) FindByServerIDAndType(serverID, fileType) ([]models.ServerFile, error)
func (r *FileRepository) FindActiveByServerIDAndType(serverID, fileType) (*models.ServerFile, error)

// Activation Management (Only one active per type!)
func (r *FileRepository) DeactivateAllOfType(serverID, fileType) error

// Status Management
func (r *FileRepository) UpdateStatus(id, status, errorMessage) error
```

**Active File Pattern:**

```go
func (r *FileRepository) FindActiveByServerIDAndType(serverID, fileType) (*models.ServerFile, error) {
    var file models.ServerFile
    err := r.db.Where("server_id = ? AND file_type = ? AND is_active = ?", serverID, fileType, true).
        First(&file).Error
    if err != nil {
        if err == gorm.ErrRecordNotFound {
            return nil, nil // No active file is not an error  // 🟢 GOOD!
        }
        return nil, err
    }
    return &file, nil
}

func (r *FileRepository) DeactivateAllOfType(serverID, fileType) error {
    return r.db.Model(&models.ServerFile{}).
        Where("server_id = ? AND file_type = ?", serverID, fileType).
        Update("is_active", false).Error
}
```

**Workflow:**
1. Upload neues Resource Pack → Create (is_active = false)
2. User aktiviert → DeactivateAllOfType (alte deaktivieren) → Update (neue aktivieren)

**🟢 GOOD:** Atomic-Safe Pattern.

### 5. ConfigChangeRepository ([config_change_repository.go](../internal/repository/config_change_repository.go))

**Simplest Repository:**

```go
type ConfigChangeRepository struct {
    db *gorm.DB
}

func (r *ConfigChangeRepository) Create(change *models.ConfigChange) error
func (r *ConfigChangeRepository) Update(change *models.ConfigChange) error
func (r *ConfigChangeRepository) FindByID(id string) (*models.ConfigChange, error)
func (r *ConfigChangeRepository) FindByServerID(serverID string) ([]models.ConfigChange, error)
func (r *ConfigChangeRepository) FindByUserID(userID string) ([]models.ConfigChange, error)
```

**Audit-Trail Usage:** Alle Queries nutzen `Order("created_at DESC")` für chronologische Reihenfolge.

## Common Patterns

### 1. Constructor Pattern

**Alle Repositories:**
```go
type XyzRepository struct {
    db *gorm.DB
}

func NewXyzRepository(db *gorm.DB) *XyzRepository {
    return &XyzRepository{db: db}
}
```

**Usage in main.go:**
```go
serverRepo := repository.NewServerRepository(db)
userRepo := repository.NewUserRepository(db)
// ...
```

### 2. Error Handling

**Direkte GORM Error Propagation:**
```go
func (r *ServerRepository) FindByID(id string) (*models.MinecraftServer, error) {
    var server models.MinecraftServer
    err := r.db.Where("id = ?", id).First(&server).Error
    if err != nil {
        return nil, err // Direct propagation
    }
    return &server, nil
}
```

**No Custom Error Wrapping!** Services müssen GORM-Errors interpretieren.

**🟡 INCONSISTENCY:**

Einige Repositories checken `gorm.ErrRecordNotFound`, andere nicht:

```go
// FileRepository (Zeile 54-60) - Checkt!
if err == gorm.ErrRecordNotFound {
    return nil, nil // No active file is not an error
}

// ServerRepository - Checkt NICHT!
err := r.db.Where("id = ?", id).First(&server).Error
if err != nil {
    return nil, err // Could be ErrRecordNotFound!
}
```

**Impact:** Caller muss inconsistent Error Handling machen.

### 3. Soft Delete Handling

**Two Strategies:**

**A) Default (Most Repositories):**
```go
func (r *UserRepository) FindByID(id string) (*models.User, error) {
    var user models.User
    err := r.db.First(&user, "id = ?", id).Error // Excludes soft-deleted
    return &user, nil
}
```

**B) Unscoped (ServerRepository):**
```go
func (r *ServerRepository) FindByID(id string) (*models.MinecraftServer, error) {
    var server models.MinecraftServer
    err := r.db.Unscoped().Where("id = ?", id).First(&server).Error // Includes soft-deleted!
    return &server, nil
}
```

**🔴 PROBLEM:** Siehe "DESIGN CONCERN" bei ServerRepository oben.

### 4. Batch Operations / Upserts

**PluginRepository (Zeile 204-238):**
```go
func (r *PluginRepository) UpsertPlugin(plugin *models.Plugin) error {
    existing, err := r.FindPluginByExternalID(plugin.Source, plugin.ExternalID)
    if err != nil {
        if err == gorm.ErrRecordNotFound {
            return r.CreatePlugin(plugin) // Create
        }
        return err
    }
    plugin.ID = existing.ID
    return r.UpdatePlugin(plugin) // Update
}
```

**🟢 GOOD:** Idempotent Sync-Operationen für Modrinth-Integration.

## Global Database Access

**Singleton Pattern (Anti-Pattern!):**

```go
var DB *gorm.DB

func InitDB(cfg *config.Config) error {
    // ...
    DB, err = gorm.Open(...)
}

func GetDB() *gorm.DB {
    return DB
}
```

**🟡 CONCERN:**

**Problem:**
- Globale Variable `DB` (Zeile 14)
- Kein Thread-Safety (nicht nötig bei GORM, aber schlechter Stil)
- Schwer testbar (Mocking schwierig)

**Better Practice:**
- Dependency Injection (bereits genutzt via `NewXyzRepository(db)`)
- `DB` Variable als private machen
- Nur `GetDB()` exportieren

## Code-Flaws & Potenzielle Probleme

### 🔴 CRITICAL

1. **ServerRepository.FindByID includes soft-deleted** (server_repository.go:23)
   - Alle Queries nutzen `Unscoped()`
   - User sehen gelöschte Server
   - Sollte separate Methoden geben: `FindByID` vs. `FindByIDIncludingDeleted`

### 🟡 MEDIUM

2. **PostgreSQL Comment veraltet** (database_interface.go:48)
   - Sagt "Placeholder for future", aber IS implemented
   - Verwirrend für neue Entwickler

3. **SQLite-Support Dead Code** (database_interface.go:18-45)
   - SQLiteProvider existiert
   - Wird nicht initialisiert (database.go:32-48)
   - Sollte entfernt werden oder dokumentiert als "deprecated"

4. **Password Masking zu simpel** (database.go:89-113)
   - Magic Number `i > 10`
   - Funktioniert nicht für alle URL-Formate
   - Sollte `url.Parse()` nutzen

5. **FindCompatibleVersions ineffizient** (plugin_repository.go:130-146)
   - Lädt ALLE Versionen
   - Filterung in Service-Layer
   - Sollte PostgreSQL JSONB-Operators nutzen

6. **Inconsistent ErrRecordNotFound Handling**
   - FileRepository checkt (gut!)
   - Andere Repositories nicht (schlecht!)
   - Services müssen GORM-Errors kennen

### 🟢 LOW

7. **Keine Repository Interfaces für alle**
   - Nur ServerRepository hat Interface
   - Andere Repositories nicht
   - Erschwert Testing/Mocking

8. **Default Limits hardcoded**
   - ListPlugins: Default 100 (plugin_repository.go:73)
   - SearchPlugins: Default 50 (plugin_repository.go:89)
   - Sollte config-basiert sein

## Best Practices (Positiv)

✅ **Gute Patterns:**
1. Repository Pattern mit Constructor
2. Atomic Balance Operations (gorm.Expr)
3. Eager Loading mit Preload
4. Upsert-Pattern für Idempotenz
5. PostgreSQL-spezifische Features (ILIKE)
6. Hard Delete vs. Soft Delete bewusst genutzt (GetUsedPorts)

## Abhängigkeiten

**Interne:**
- `github.com/payperplay/hosting/internal/models` - Data Models
- `github.com/payperplay/hosting/pkg/config` - Configuration

**Externe:**
- `gorm.io/gorm` - ORM
- `gorm.io/driver/postgres` - PostgreSQL Driver

## Nächste Schritte

Siehe [04-BUSINESS_LOGIC.md](04-BUSINESS_LOGIC.md) für Service-Layer-Analyse.
