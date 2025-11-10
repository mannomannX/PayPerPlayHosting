# Plugin Marketplace System

## Overview

The Plugin Marketplace is an **auto-syncing**, **zero-maintenance** system that aggregates plugins from external sources (Modrinth, Hangar, SpigotMC) and provides:

- **Automatic Discovery**: Plugins sync automatically from Modrinth API every 6 hours
- **Version Management**: Semantic versioning with compatibility checking
- **Auto-Updates**: Optional automatic updates to compatible versions
- **Dependency Resolution**: Automatic detection and resolution of plugin dependencies
- **Compatibility Checking**: Filters plugins by Minecraft version and server type

## Architecture

```
External APIs (Modrinth)
    ↓ (Auto-Sync every 6h)
Plugin Database (PostgreSQL)
    ↓
PluginManager Service
    ↓
Server /plugins/ directory
```

### Key Components

1. **Modrinth API Client** (`internal/external/modrinth_client.go`)
   - Fetches plugins and versions from Modrinth
   - Handles rate limiting and pagination
   - Supports search, filtering, and compatibility checking

2. **PluginSync Service** (`internal/service/plugin_sync_service.go`)
   - Background worker running every 6 hours
   - Syncs top 500 plugins from Modrinth
   - Updates plugin metadata and versions automatically
   - Zero manual maintenance required

3. **PluginManager Service** (`internal/service/plugin_manager_service.go`)
   - Handles installation, updates, and removal
   - Compatibility checking (MC version + server type)
   - Auto-update system for installed plugins
   - Dependency resolution (future feature)

4. **Database Models** (`internal/models/plugin.go`)
   - `Plugin`: Base plugin information
   - `PluginVersion`: Version-specific data
   - `InstalledPlugin`: Tracks which plugins are installed where

## Data Model

### Plugin
```go
type Plugin struct {
    ID          string         // UUID
    Name        string         // "WorldEdit"
    Slug        string         // "worldedit" (unique)
    Description string
    Author      string
    Category    PluginCategory // "world-management", "admin-tools", etc.
    IconURL     string

    // External Source (for auto-sync)
    Source     PluginSource   // "modrinth", "hangar", "spigot", "manual"
    ExternalID string         // Project ID at source

    // Stats
    DownloadCount int
    Rating        float64
    LastSynced    time.Time
}
```

### PluginVersion
```go
type PluginVersion struct {
    ID       string
    PluginID string
    Version  string // "7.2.15" (Semantic Versioning)

    // Compatibility (stored as JSON arrays)
    MinecraftVersions []string // ["1.20", "1.20.1", "1.20.2"]
    ServerTypes       []string // ["paper", "spigot", "purpur"]
    Dependencies      []Dependency

    // Download
    DownloadURL string
    FileHash    string // SHA256/SHA512 for integrity
    FileSize    int64

    // Metadata
    Changelog   string
    ReleaseDate time.Time
    IsStable    bool // vs. beta/alpha
}
```

### InstalledPlugin
```go
type InstalledPlugin struct {
    ServerID   string
    PluginID   string
    VersionID  string

    Enabled    bool
    AutoUpdate bool // Auto-update to new compatible versions

    InstalledAt time.Time
    LastUpdated time.Time
}
```

## API Endpoints

### Marketplace Browsing

```http
# List marketplace plugins
GET /api/marketplace/plugins?category=admin-tools&limit=50

# Search marketplace
GET /api/marketplace/search?q=worldedit&limit=20

# Get plugin details
GET /api/marketplace/plugins/:slug
```

### Server Plugin Management

```http
# List installed plugins
GET /api/servers/:id/marketplace/plugins

# Install plugin
POST /api/servers/:id/marketplace/plugins
{
  "plugin_slug": "worldedit",
  "version_id": "optional-specific-version",
  "auto_update": false
}

# Uninstall plugin
DELETE /api/servers/:id/marketplace/plugins/:plugin_id

# Check for updates
GET /api/servers/:id/marketplace/updates

# Update plugin to specific version
PUT /api/servers/:id/marketplace/plugins/:plugin_id
{
  "version_id": "new-version-id"
}

# Auto-update all plugins with AutoUpdate=true
POST /api/servers/:id/marketplace/auto-update

# Toggle plugin enabled/disabled
POST /api/servers/:id/marketplace/plugins/:plugin_id/toggle
{
  "enabled": true
}

# Toggle auto-update setting
POST /api/servers/:id/marketplace/plugins/:plugin_id/auto-update
{
  "auto_update": true
}
```

### Admin Functions

```http
# Manually trigger full marketplace sync
POST /api/admin/marketplace/sync

# Manually sync specific plugin
POST /api/admin/marketplace/plugins/:slug/sync
```

## Installation Flow

When a user installs a plugin:

1. **Compatibility Check**
   - System checks server's Minecraft version (e.g., "1.20.4")
   - System checks server type (e.g., "paper")
   - Filters plugin versions to find compatible ones

2. **Version Selection**
   - If `version_id` specified: Use that version
   - Otherwise: Select latest **stable** compatible version
   - Stable versions are preferred over beta/alpha

3. **Download & Verify**
   - Download `.jar` file from DownloadURL (cached from Modrinth)
   - Verify file integrity using SHA512 hash
   - Save to `/minecraft/servers/{server_id}/plugins/{plugin_slug}.jar`

4. **Record Installation**
   - Create `InstalledPlugin` record in database
   - Set `AutoUpdate` flag based on user preference
   - Record installation timestamp

5. **Server Restart** (Optional)
   - User can manually restart server to load plugin
   - Future: Hot-reload support via PlugMan

## Auto-Update System

### Background Worker

The `PluginSync` service runs every 6 hours and:

1. Fetches latest plugin data from Modrinth
2. Updates plugin metadata (description, downloads, etc.)
3. Discovers new plugin versions
4. Triggers auto-updates for eligible plugins

### Auto-Update Logic

For each server with `InstalledPlugin.AutoUpdate = true`:

1. **Check for Updates**
   - Find latest compatible version for server's MC version + type
   - Compare with currently installed version

2. **Compatibility Validation**
   - New version must support server's Minecraft version
   - New version must support server's server type (Paper/Spigot/etc.)

3. **Update Execution**
   - Backup old `.jar` file as `.jar.backup`
   - Download new version
   - Verify hash
   - Replace old file
   - Update `InstalledPlugin` record
   - Delete backup on success

4. **Rollback on Failure**
   - If download fails, restore from backup
   - Log error for user review

## Compatibility Checking

### Minecraft Version Matching

Plugin versions specify supported MC versions:
```json
{
  "game_versions": ["1.20", "1.20.1", "1.20.2", "1.20.4"]
}
```

System checks if server's version is in the list.

### Server Type Matching

Plugin versions specify supported loaders:
```json
{
  "loaders": ["paper", "spigot", "bukkit"]
}
```

System checks if server's type matches. Paper servers can use:
- `paper` plugins
- `spigot` plugins (backward compatible)
- `bukkit` plugins (backward compatible)

## Dependency Resolution

**Status**: Partially implemented

Plugin versions can declare dependencies:
```json
{
  "dependencies": [
    {
      "project_id": "worldedit-project-id",
      "dependency_type": "required"
    }
  ]
}
```

**Current Behavior**:
- Dependencies are stored in database
- NOT automatically installed yet

**Future Enhancement**:
- Automatically suggest required dependencies
- One-click install of dependency chain
- Conflict detection (incompatible plugins)

## Categories

Plugins are categorized for easier discovery:

- `world-management`: WorldEdit, VoxelSniper, etc.
- `admin-tools`: EssentialsX, LuckPerms, etc.
- `economy`: Vault, EconomyAPI, etc.
- `mechanics`: Custom game mechanics plugins
- `protection`: WorldGuard, GriefPrevention, etc.
- `social`: Chat, discord integration plugins
- `utility`: General utility plugins
- `optimization`: Performance optimization plugins

## External Sources

### Modrinth (Primary)

**API**: https://api.modrinth.com/v2

**Features**:
- Well-documented REST API
- Semantic versioning
- Game version + loader compatibility
- SHA512 hashes for integrity
- Rate limiting: 300 requests/minute

**Sync Strategy**:
- Fetch top 500 plugins by downloads
- Sync every 6 hours
- Batch requests with pagination (100 plugins/request)
- 200ms delay between requests (rate limiting)

### Hangar (Future)

**Status**: Not implemented yet
**API**: https://hangar.papermc.io/api

Paper-specific plugins, similar to Modrinth.

### SpigotMC (Future)

**Status**: Not implemented yet
**Challenges**:
- No official API
- Requires web scraping
- Less reliable than Modrinth

### Manual Upload (Future)

**Status**: Not implemented yet

Allow users to upload custom/private plugins:
- Set `Source = "manual"`
- No auto-sync
- Manual version management

## Performance Considerations

### Caching

- Plugin metadata cached in PostgreSQL
- Download URLs cached (no repeated API calls during install)
- Auto-sync runs off-peak (configurable interval)

### Rate Limiting

- Modrinth API: 300 req/min
- Our sync: 200ms delay between requests (~300 plugins/hour)
- Sync limited to top 500 plugins (reduces API usage)

### Database Queries

- Indexes on: `slug`, `external_id`, `source`, `category`
- Version queries optimized for compatibility checking
- Installed plugins preloaded with Plugin + Version relations

## Monitoring & Observability

### Logs

```go
logger.Info("Plugin sync service started", map[string]interface{}{
    "sync_interval": "6h",
})

logger.Info("Plugin marketplace sync completed", map[string]interface{}{
    "synced_plugins": 487,
    "duration_ms": 12450,
})
```

### Metrics (Future)

Prometheus metrics to add:
- `marketplace_sync_duration_seconds`
- `marketplace_plugins_total`
- `marketplace_versions_total`
- `marketplace_installs_total`
- `marketplace_updates_total`

## Configuration

### Environment Variables

```bash
# No configuration needed for MVP
# Modrinth API is public and requires no auth
```

### Service Configuration

```go
// Sync interval (default: 6 hours)
syncInterval := 6 * time.Hour

// Plugin limit (top N plugins)
pluginLimit := 500

// Rate limiting delay
rateDelay := 200 * time.Millisecond
```

## Future Enhancements

### Phase 2 Features

1. **Dependency Auto-Install**
   - Automatically install required dependencies
   - Warn about optional dependencies
   - Detect circular dependencies

2. **Version Pinning**
   - User can pin to specific major version (e.g., "7.x")
   - Auto-update within pinned version range
   - Prevent breaking changes

3. **Rollback Support**
   - Keep last 3 versions as backups
   - One-click rollback to previous version
   - Automatic rollback on server crash after update

4. **Plugin Recommendations**
   - "Users who installed X also installed Y"
   - "Essential plugins for {server_type}"
   - Category-based recommendations

5. **Multi-Source Support**
   - Hangar (Paper plugins)
   - SpigotMC (web scraping)
   - Manual upload (custom plugins)

6. **Hot Reload**
   - Integration with PlugMan for hot reload
   - No server restart required for compatible plugins

7. **Conflict Detection**
   - Detect incompatible plugin combinations
   - Warn before installing conflicting plugins
   - Suggest alternatives

## Troubleshooting

### Plugin Not Found

**Symptom**: Search returns no results for known plugin

**Cause**: Plugin not synced yet or not on Modrinth

**Solution**: Trigger manual sync:
```http
POST /api/admin/marketplace/plugins/worldedit/sync
```

### Version Incompatible

**Symptom**: "No compatible version found" error

**Cause**: Plugin doesn't support server's MC version or type

**Solution**:
- Check plugin's supported versions on Modrinth
- Upgrade/downgrade Minecraft version
- Switch server type (Paper <-> Spigot)

### Auto-Update Not Working

**Symptom**: Plugins not updating automatically

**Cause**: `AutoUpdate = false` or no compatible newer version

**Solution**:
```http
POST /api/servers/:id/marketplace/plugins/:plugin_id/auto-update
{ "auto_update": true }
```

### Download Fails

**Symptom**: Plugin installation fails during download

**Cause**: Network issue or Modrinth CDN down

**Solution**: Retry installation after a few minutes

## Security Considerations

### File Integrity

- All downloads verified with SHA512 hash
- Prevents man-in-the-middle attacks
- Detects corrupted downloads

### Source Trust

- Modrinth is a reputable, moderated platform
- Plugins reviewed before listing
- User reports for malicious plugins

### Permissions

- Only authenticated users can install plugins
- Server ownership verified before installation
- Admin-only manual sync endpoints

## Examples

### Example 1: Install WorldEdit

```bash
# Search for WorldEdit
curl -X GET "http://localhost:8000/api/marketplace/search?q=worldedit" \
  -H "Authorization: Bearer {token}"

# Install latest compatible version
curl -X POST "http://localhost:8000/api/servers/{server_id}/marketplace/plugins" \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{
    "plugin_slug": "worldedit",
    "auto_update": true
  }'
```

### Example 2: Check for Updates

```bash
curl -X GET "http://localhost:8000/api/servers/{server_id}/marketplace/updates" \
  -H "Authorization: Bearer {token}"

# Response:
{
  "updates": [
    {
      "plugin_id": "...",
      "plugin_name": "WorldEdit",
      "current_version": "7.2.15",
      "latest_version": "7.2.16",
      "latest_version_id": "...",
      "auto_update": true
    }
  ],
  "count": 1
}
```

### Example 3: Enable Auto-Update

```bash
curl -X POST "http://localhost:8000/api/servers/{server_id}/marketplace/plugins/{plugin_id}/auto-update" \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{
    "auto_update": true
  }'
```

## Conclusion

The Plugin Marketplace provides a **fully automated**, **zero-maintenance** system for managing Minecraft server plugins. By aggregating from Modrinth and auto-syncing every 6 hours, users get:

✅ **No manual plugin management**
✅ **Always up-to-date marketplace**
✅ **Automatic compatibility checking**
✅ **Optional auto-updates**
✅ **Semantic versioning**
✅ **File integrity verification**

All with **ZERO manual upkeep** for the platform operator.
