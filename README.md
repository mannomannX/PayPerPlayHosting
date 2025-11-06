# PayPerPlay Minecraft Hosting

Pay-per-play Minecraft-Server-Hosting mit automatischem Start/Stop und sekundengenauer Abrechnung.

Built with **Go**, **Docker**, **Gin**, and **GORM**.

## Features

- ✅ Auto-Start bei Player-Verbindung (via Velocity Proxy - coming soon)
- ✅ Auto-Stop bei Inaktivität
- ✅ Multi-Version-Support (1.8 - 1.21+)
- ✅ Multi-Type-Support (Paper, Spigot, Forge, Fabric, Vanilla, Purpur)
- ✅ Sekundengenaue Billing-Daten
- ✅ REST API für Server-Management
- ✅ Web-Dashboard (Alpine.js + Tailwind)
- ✅ Docker-basierte Isolation
- ✅ Production-ready Go-Code

## Tech-Stack

| Component | Technology |
|-----------|-----------|
| **Backend** | Go 1.21+ |
| **Web Framework** | Gin |
| **ORM** | GORM |
| **Database** | SQLite (dev) / PostgreSQL (prod) |
| **Container** | Docker |
| **Frontend** | Alpine.js + Tailwind CSS |
| **MC Server Images** | itzg/minecraft-server |

## Quick-Start

### Prerequisites

- **Go 1.21+** - [Download here](https://go.dev/dl/)
- **Docker Desktop** - [Download here](https://www.docker.com/products/docker-desktop/)
- **Git** (optional)

### Installation

```bash
# 1. Stelle sicher dass Go installiert ist
go version  # sollte >= 1.21 sein

# 2. Wechsel ins Projekt-Verzeichnis
cd C:\Users\Robin\Desktop\PayPerPlayHosting

# 3. Installiere Dependencies
go mod download
go mod tidy

# 4. Kopiere .env-Template
copy .env.example .env

# 5. Pull Docker-Image (dauert beim ersten Mal ~5 Minuten)
docker pull itzg/minecraft-server:latest

# 6. Build & Run
go build -o payperplay.exe ./cmd/api
./payperplay.exe

# 7. Öffne Browser
start http://localhost:8000
```

### Using the API

```bash
# Create a server
curl -X POST http://localhost:8000/api/servers \
  -H "Content-Type: application/json" \
  -d '{
    "name": "TestServer",
    "server_type": "paper",
    "minecraft_version": "1.20.4",
    "ram_mb": 2048
  }'

# List servers
curl http://localhost:8000/api/servers

# Start a server
curl -X POST http://localhost:8000/api/servers/{server_id}/start

# Stop a server
curl -X POST http://localhost:8000/api/servers/{server_id}/stop

# Get usage logs
curl http://localhost:8000/api/servers/{server_id}/usage

# Delete server
curl -X DELETE http://localhost:8000/api/servers/{server_id}
```

## Project Structure

```
PayPerPlayHosting/
├── cmd/
│   └── api/              # Main application entry point
├── internal/
│   ├── api/              # HTTP handlers & routing
│   ├── service/          # Business logic
│   ├── repository/       # Database access
│   ├── models/           # Data models
│   └── docker/           # Docker service
├── pkg/
│   └── config/           # Configuration management
├── web/
│   ├── templates/        # HTML templates
│   └── static/           # Static assets (CSS, JS)
├── minecraft/
│   ├── servers/          # Server data (persistent volumes)
│   └── plugins/          # Custom plugins (future)
├── PayPerPlay-Docs/      # Comprehensive documentation
├── go.mod                # Go dependencies
├── Makefile              # Build commands (Linux/Mac)
└── README.md
```

## Configuration

Edit `.env` file:

```env
# Application
APP_NAME=PayPerPlay
DEBUG=true
PORT=8000

# Minecraft
SERVERS_BASE_PATH=./minecraft/servers
DEFAULT_IDLE_TIMEOUT=300  # 5 minutes

# Billing (EUR per hour)
RATE_2GB=0.10
RATE_4GB=0.20
RATE_8GB=0.40
RATE_16GB=0.80

# Port Range
MC_PORT_START=25565
MC_PORT_END=25665
```

## Supported Server Types

| Type | Description | Best For |
|------|-------------|----------|
| **paper** | High-performance Paper MC | Plugins, Best performance (default) |
| **spigot** | Spigot server | Plugins, Legacy compatibility |
| **forge** | Forge mod loader | Tech mods (FTB, ATM, etc.) |
| **fabric** | Fabric mod loader | Modern mods, Performance mods |
| **vanilla** | Official Mojang server | Pure vanilla experience |
| **purpur** | Purpur (Paper fork) | Experimental features |

## Supported Minecraft Versions

- **1.8** - 1.8.9 (PvP, Legacy)
- **1.12** - 1.12.2 (Modded golden age)
- **1.16** - 1.16.5 (Nether Update)
- **1.18** - 1.18.2 (Caves & Cliffs)
- **1.19** - 1.19.4 (Wild Update)
- **1.20** - 1.20.4+ (Latest)
- **1.21+** (Future versions)

## Development

```bash
# Build
go build -o payperplay.exe ./cmd/api

# Run
./payperplay.exe

# Run with auto-reload (install air first: go install github.com/cosmtrek/air@latest)
air

# Test
go test -v ./...

# Clean
del payperplay.exe payperplay.db
```

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| POST | `/api/servers` | Create server |
| GET | `/api/servers` | List servers |
| GET | `/api/servers/:id` | Get server details |
| POST | `/api/servers/:id/start` | Start server |
| POST | `/api/servers/:id/stop` | Stop server |
| DELETE | `/api/servers/:id` | Delete server |
| GET | `/api/servers/:id/usage` | Get usage logs |
| GET | `/api/servers/:id/logs?tail=100` | Get Docker logs |

## Roadmap

### Phase 1: MVP (Current) ✅
- [x] Core server management (Create, Start, Stop, Delete)
- [x] Docker integration
- [x] Basic web UI
- [x] Usage tracking & billing
- [x] Multi-version support
- [x] Multi-type support (Paper, Forge, Fabric)

### Phase 2: Auto-Scaling (Next)
- [ ] Velocity proxy integration (auto-wake on connect)
- [ ] Auto-shutdown monitoring (check player count every 60s)
- [ ] Server status websocket (real-time updates)

### Phase 3: Production Features
- [ ] User authentication (JWT)
- [ ] Payment integration (Stripe)
- [ ] Backup system (Restic + Hetzner Storage Box)
- [ ] File manager (SFTP / Web-based)
- [ ] Plugin marketplace (1-click install)
- [ ] Mod pack installer (CurseForge API)

### Phase 4: Scale
- [ ] Multi-server support (horizontal scaling)
- [ ] PostgreSQL migration
- [ ] Prometheus metrics
- [ ] Grafana dashboards
- [ ] Admin panel

## Performance

**System Requirements** (per dedicated server):
- CPU: 6+ cores (AMD Ryzen or Intel Xeon)
- RAM: 32-64 GB
- Storage: 500 GB+ NVMe SSD
- Network: 1 Gbps

**Capacity** (Hetzner AX41 example):
- ~20-30 concurrent 4GB servers
- ~50-80 servers total (with overprovisioning)
- Startup time: <30s (Paper), <60s (Forge)

## Why Go?

- **Performance**: 10-20x faster than Node.js/Python for this workload
- **Memory**: ~20 MB footprint vs. 50-100 MB (Python) or 30-50 MB (Node)
- **Concurrency**: Goroutines perfect for managing thousands of servers
- **Deployment**: Single binary, no dependencies
- **Docker SDK**: Native, well-supported
- **Production-Ready**: Type-safe, compiled, robust error handling

## Troubleshooting

**"go: command not found"**
- Install Go from https://go.dev/dl/
- Nach Installation: Terminal neu starten

**Docker errors**
- Stelle sicher dass Docker Desktop läuft
- Teste mit: `docker ps`

**Port already in use**
- Ändere PORT in `.env` (z.B. auf 8001)

## Contributing

See [PayPerPlay-Docs/](PayPerPlay-Docs/) for detailed documentation on:
- Architecture
- Business model
- Minecraft technical details
- Backup strategy
- Implementation roadmap

## License

MIT

## Support

- Documentation: [PayPerPlay-Docs/](PayPerPlay-Docs/)
- Issues: GitHub Issues
- Discord: (coming soon)

---

Built with ❤️ in Go 🚀
