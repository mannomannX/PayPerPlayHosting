# PayPerPlay Features & API Documentation

## 🚀 Core Features (v2.0)

### 1. **Server Management**
- ✅ Create servers (Paper, Spigot, Forge, Fabric, Vanilla, Purpur)
- ✅ Multi-version support (1.8 - 1.21+)
- ✅ Start/Stop/Delete servers
- ✅ Automatic port assignment (25565-25665)
- ✅ RAM configuration (2GB - 16GB)
- ✅ Docker-isolated containers

### 2. **Auto-Shutdown Monitoring** ⭐ NEW
- ✅ RCON-based player count tracking
- ✅ Automatic shutdown after idle timeout (configurable, default 5min)
- ✅ Real-time player count display
- ✅ Idle timer with visual countdown
- ✅ Background monitoring service (checks every 60s)
- ✅ Auto-start monitoring when server starts

**How it works:**
1. Server starts → Monitoring service begins tracking
2. Every 60 seconds: Check player count via RCON
3. If 0 players: Idle timer increases
4. If players join: Timer resets
5. If idle > timeout: Server auto-stops, billing stops

### 3. **Backup System** ⭐ NEW
- ✅ Create manual backups (ZIP format)
- ✅ List all backups with size and timestamp
- ✅ Restore from backup (replaces current world)
- ✅ Delete old backups
- ✅ Automatic backup before restore (safety)

**API Endpoints:**
```bash
POST   /api/servers/:id/backups          # Create backup
GET    /api/servers/:id/backups          # List backups
POST   /api/servers/:id/backups/restore  # Restore backup
DELETE /api/servers/:id/backups/:filename # Delete backup
```

### 4. **Plugin Management** ⭐ NEW
- ✅ Search popular plugins (EssentialsX, LuckPerms, Vault, WorldEdit)
- ✅ Install plugins from URL (Paper/Spigot/Purpur only)
- ✅ List installed plugins
- ✅ Remove plugins
- ✅ Web-based plugin installer

**Supported for:** Paper, Spigot, Purpur

**Popular Plugins Available:**
- EssentialsX (commands, economy)
- LuckPerms (permissions)
- WorldEdit (world editing)
- Vault (economy API)

### 5. **Usage Tracking & Billing**
- ✅ Sekundengenaue Zeiterfassung
- ✅ Automatische Kostenberechnung
- ✅ Peak player count tracking
- ✅ Shutdown reason logging (idle, manual, crash)
- ✅ Historical usage logs

**Pricing:**
- 2 GB RAM: €0.10/hour
- 4 GB RAM: €0.20/hour
- 8 GB RAM: €0.40/hour
- 16 GB RAM: €0.80/hour

### 6. **Web Dashboard**
- ✅ Create servers via UI
- ✅ Real-time server status
- ✅ Player count display (live)
- ✅ Idle timer countdown
- ✅ Usage history with costs
- ✅ Backup management UI
- ✅ Plugin installer UI
- ✅ Docker logs viewer

### 7. **RCON Integration**
- ✅ RCON enabled on all servers
- ✅ Player count queries
- ✅ Command execution capability
- ✅ Automatic retry on connection failure

### 8. **Docker Optimization**
- ✅ Using `itzg/minecraft-server` image (industry standard)
- ✅ Aikar's JVM flags for optimal performance
- ✅ Memory limits per container
- ✅ Persistent volumes for worlds
- ✅ RCON port binding (localhost only for security)

## 📡 API Endpoints

### Server Management
```
POST   /api/servers                 # Create server
GET    /api/servers                 # List all servers
GET    /api/servers/:id             # Get server details
POST   /api/servers/:id/start       # Start server
POST   /api/servers/:id/stop        # Stop server
DELETE /api/servers/:id             # Delete server
GET    /api/servers/:id/usage       # Get usage logs
GET    /api/servers/:id/logs        # Get Docker logs
```

### Monitoring
```
GET    /api/servers/:id/status                    # Real-time status
POST   /api/servers/:id/auto-shutdown/enable      # Enable auto-shutdown
POST   /api/servers/:id/auto-shutdown/disable     # Disable auto-shutdown
GET    /api/monitoring/status                     # All servers status
```

### Backups
```
POST   /api/servers/:id/backups                   # Create backup
GET    /api/servers/:id/backups                   # List backups
POST   /api/servers/:id/backups/restore           # Restore backup
DELETE /api/servers/:id/backups/:filename         # Delete backup
```

### Plugins
```
POST   /api/servers/:id/plugins                   # Install plugin
GET    /api/servers/:id/plugins                   # List installed
DELETE /api/servers/:id/plugins/:filename         # Remove plugin
GET    /api/plugins/search?q=...                  # Search plugins
```

### Mod Packs (Coming Soon)
```
POST   /api/servers/:id/modpack                   # Install modpack
GET    /api/modpacks/search?q=...                 # Search modpacks
```

## 🔧 Configuration

### Environment Variables (.env)
```env
# Application
APP_NAME=PayPerPlay
DEBUG=true
PORT=8000

# Database
DATABASE_PATH=./payperplay.db

# Minecraft
SERVERS_BASE_PATH=./minecraft/servers
DEFAULT_IDLE_TIMEOUT=300  # 5 minutes

# Billing
RATE_2GB=0.10
RATE_4GB=0.20
RATE_8GB=0.40
RATE_16GB=0.80

# Ports
MC_PORT_START=25565
MC_PORT_END=25665
```

### Server-Specific Config
Each server has:
- `idle_timeout_seconds` (default: 300 = 5 minutes)
- `auto_shutdown_enabled` (default: true)
- `max_players` (default: 20)

## 🎯 Usage Examples

### Create a Paper Server
```bash
curl -X POST http://localhost:8000/api/servers \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Survival Server",
    "server_type": "paper",
    "minecraft_version": "1.20.4",
    "ram_mb": 4096
  }'
```

### Check Server Status (with Player Count)
```bash
curl http://localhost:8000/api/servers/{server_id}/status
```

**Response:**
```json
{
  "server_id": "abc123",
  "is_monitored": true,
  "player_count": 3,
  "idle_seconds": 0,
  "timeout_seconds": 300
}
```

### Create a Backup
```bash
curl -X POST http://localhost:8000/api/servers/{server_id}/backups
```

### Install Plugin
```bash
curl -X POST http://localhost:8000/api/servers/{server_id}/plugins \
  -H "Content-Type: application/json" \
  -d '{
    "plugin_url": "https://github.com/EssentialsX/Essentials/releases/download/2.20.1/EssentialsX-2.20.1.jar",
    "filename": "EssentialsX.jar"
  }'
```

## 📊 Database Schema

### minecraft_servers
- `id` (string, primary key)
- `name`, `owner_id`
- `server_type` (paper, spigot, forge, fabric, vanilla, purpur)
- `minecraft_version`
- `ram_mb`, `max_players`, `port`
- `status` (stopped, starting, running, stopping, error)
- `container_id`
- `idle_timeout_seconds`, `auto_shutdown_enabled`
- Timestamps: `created_at`, `last_started_at`, `last_stopped_at`

### usage_logs
- `id` (auto-increment)
- `server_id` (foreign key)
- `started_at`, `stopped_at`
- `duration_seconds`, `cost_eur`
- `player_count_peak`
- `shutdown_reason` (idle, manual, crash)

## 🚧 Roadmap (v3.0)

### Phase 1: Production Ready
- [ ] WebSocket for real-time updates
- [ ] Velocity Proxy integration (auto-wake on connect)
- [ ] User authentication (JWT)
- [ ] Payment integration (Stripe)

### Phase 2: Advanced Features
- [ ] File manager (edit server files via web)
- [ ] CurseForge API for mod pack installation
- [ ] Scheduled backups (daily, weekly)
- [ ] Multi-server orchestration
- [ ] Resource pack hosting

### Phase 3: Scale
- [ ] PostgreSQL migration
- [ ] Multi-region support
- [ ] Prometheus metrics
- [ ] Grafana dashboards
- [ ] Admin panel

## 🔒 Security Features

1. **RCON Security**: Only bound to localhost (127.0.0.1)
2. **Container Isolation**: Each server in separate Docker container
3. **Resource Limits**: CPU/RAM limits per container
4. **No Root Access**: Containers run as non-root
5. **Backup Encryption**: Optional (coming soon)

## 💡 Performance Optimizations

1. **Aikar's JVM Flags**: Optimized garbage collection
2. **itzg Image**: Industry-proven, optimized base image
3. **Go Backend**: Low memory footprint (~20 MB)
4. **GORM**: Efficient database queries
5. **Monitoring**: Background workers don't block main thread

## 📖 For Developers

### Project Structure
```
/cmd/api                  # Main entry point
/internal
  /api                    # HTTP handlers
  /service                # Business logic
  /repository             # Database access
  /models                 # Data models
  /docker                 # Docker service
  /rcon                   # RCON client
/pkg/config               # Configuration
/web                      # Frontend
/minecraft/servers        # Server data
/minecraft/backups        # Backup storage
```

### Adding a New Feature
1. Add service in `/internal/service`
2. Add handler in `/internal/api`
3. Register routes in `router.go`
4. Update `main.go` to initialize service
5. Update frontend in `web/templates/index.html`

### Running Tests
```bash
go test -v ./...
```

## 🎮 Supported Minecraft Versions

| Version Range | Paper | Spigot | Forge | Fabric | Notes |
|---------------|-------|--------|-------|--------|-------|
| 1.8.x | ✅ | ✅ | ✅ | ✗ | Legacy PvP |
| 1.9 - 1.12 | ✅ | ✅ | ✅ | ✗ | Modded Golden Age |
| 1.13 - 1.16 | ✅ | ✅ | ✅ | ✅ | Nether Update |
| 1.17 - 1.19 | ✅ | ✅ | ✅ | ✅ | Caves & Cliffs |
| 1.20+ | ✅ | ✅ | ✅ | ✅ | Latest |

## 🛠️ Troubleshooting

**Server won't start:**
- Check Docker is running: `docker ps`
- Check logs: `GET /api/servers/:id/logs`
- Verify port not in use: `netstat -an | grep 25565`

**RCON not working:**
- RCON takes ~30s to initialize after server start
- Check container logs for "RCON running"
- Verify password is "minecraft" (default)

**Auto-shutdown not working:**
- Check monitoring service started: Look for "Monitoring service started" in logs
- Verify auto_shutdown_enabled = true
- Check idle_timeout_seconds is set correctly

**Plugin won't install:**
- Only works for Paper/Spigot/Purpur
- Verify download URL is direct link to JAR file
- Check server is stopped (some plugins require restart)

---

**Version**: 2.0
**Last Updated**: 2025-11-06
**Author**: PayPerPlay Team
