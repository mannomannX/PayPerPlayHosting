# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**PayPerPlay** is a pay-per-use Minecraft server hosting platform built with Go that solves the fundamental challenge of pay-per-use hosting: bridging the gap between what customers use (minutes) and what providers pay for (monthly Hetzner server costs). The system uses intelligent orchestration to make this business model profitable while keeping costs lower for users.

## Build & Development Commands

### Basic Commands
```bash
# Build the binary
go build -o payperplay ./cmd/api

# Build and run (Windows)
go build -o payperplay.exe ./cmd/api
./payperplay.exe

# Build and run (Linux/Mac)
make build
make run

# Run with auto-reload (requires: go install github.com/cosmtrek/air@latest)
air

# Run tests
go test -v ./...

# Clean build artifacts
rm -f payperplay payperplay.db
rm -rf minecraft/servers/*
```

### First-Time Setup
```bash
# Install dependencies
go mod download && go mod tidy

# Copy environment template
cp .env.example .env

# Pull required Docker image
docker pull itzg/minecraft-server:latest

# For production: Use docker-compose
docker compose -f docker-compose.prod.yml up -d
```

### Production Deployment
```bash
# Deploy to production server (root@91.98.202.235)
./deploy-production.sh

# Build and copy binary to production
go build -o payperplay-NEW ./cmd/api
scp payperplay-NEW root@91.98.202.235:/root/PayPerPlayHosting/

# Restart production services
ssh root@91.98.202.235 "cd /root/PayPerPlayHosting && docker compose -f docker-compose.prod.yml restart payperplay"
```

## Architecture Overview

### The Conductor Pattern

The entire system revolves around the **Conductor** (`internal/conductor/conductor.go`), which acts as the central orchestrator coordinating:

1. **Node Registry** - Tracks all worker nodes (Hetzner Dedicated + Cloud VMs)
2. **Container Registry** - Tracks all running Minecraft containers across nodes
3. **Scaling Engine** - Auto-scales by provisioning/decommissioning Hetzner Cloud VMs
4. **Health Checker** - Monitors node and container health
5. **Start Queue** - Queues server starts when capacity is insufficient
6. **VM Provisioner** - Provisions Hetzner Cloud VMs via API
7. **Consolidation Policy** - Migrates containers to reduce node count and save costs

The Conductor is initialized in `cmd/api/main.go` and manages the entire fleet lifecycle.

### 3-Phase Lifecycle System

Servers transition through three phases to minimize costs while maximizing performance:

**Phase 1: Active (Running)**
- Container running, players can join
- Full per-minute billing
- Status: `running`

**Phase 2: Sleep (Stopped < 48h)**
- Container stopped (`docker stop`), volume persists on NVMe
- Zero CPU/RAM usage, minimal storage cost
- <1 second restart time (instant-on)
- Status: `stopped` or `sleeping`

**Phase 3: Archived (Stopped > 48h)**
- Container/volume deleted, world compressed to `.tar.gz`
- Stored in Hetzner Storage Box (~€3/TB)
- FREE for users
- ~30 second wake-up time
- Status: `archived`

### Multi-Tier Architecture

**Control Plane (Conductor)** - Central orchestrator on primary server
- Fleet management
- Node health monitoring
- Auto-scaling decisions
- Start queue management

**Proxy Layer (Velocity)** - Minecraft proxy at port 25565
- Auto-wake on player connect
- Dynamic server routing
- Java-based plugin in `velocity-plugin/`

**Worker Nodes** - Hetzner Dedicated + Cloud VMs
- Hetzner Dedicated (AX101): Base capacity, always running
- Hetzner Cloud (cpx22/32/42/62): Elastic burst capacity, on-demand
- Each node runs Docker containers for Minecraft servers

### Auto-Scaling Strategy

The Scaling Engine (`internal/conductor/scaling_engine.go`) implements two scaling modes:

**Reactive Scaling (Implemented):**
- Checks every 2 minutes
- Scale UP: >85% capacity → provision Hetzner Cloud VM
- Scale DOWN: <30% capacity for >30 min → decommission VM
- Safety limit: max 10 cloud nodes

**Predictive Scaling (Planned):**
- Time-series analysis of usage patterns
- Proactive VM provisioning before peak hours
- Integration with InfluxDB for metrics

### Consolidation Policy

The system can migrate containers between nodes to reduce costs (`internal/conductor/policy_consolidation.go`):

- Only runs when system stable (2-hour cooldown after scaling)
- Cost-aware: only if savings >€0.10/hour
- Tier-aware: only small/medium servers (2-8GB)
- Plan-aware: never migrate reserved plans (24/7)
- Safety: minimum 30min uptime, 15min idle time
- Warm-swap strategy (Blue/Green deployment for containers)

## Key Directories

```
internal/
├── api/                    # HTTP handlers & routing (29 files)
│   ├── router.go          # Route definitions
│   ├── handlers.go        # Core server CRUD
│   ├── conductor_handler.go  # Fleet management API
│   └── scaling_handler.go    # Auto-scaling API
├── conductor/              # Fleet orchestration (13 files) ⭐ CORE SYSTEM
│   ├── conductor.go       # Central coordinator
│   ├── node_registry.go   # Node tracking
│   ├── scaling_engine.go  # Auto-scaling logic
│   ├── vm_provisioner.go  # Hetzner Cloud API
│   └── policy_consolidation.go  # Cost optimization
├── service/                # Business logic (26 files)
│   ├── minecraft_service.go  # Server lifecycle
│   ├── billing_service.go    # Usage tracking
│   ├── lifecycle_service.go  # 3-phase lifecycle
│   └── monitoring_service.go # Auto-shutdown
├── repository/             # Database access (GORM)
├── models/                 # Data models
│   ├── server.go          # MinecraftServer model
│   ├── node.go            # Node model
│   └── tier.go            # Tier-based pricing
├── docker/                 # Docker Engine API abstraction
└── cloud/                  # Cloud provider integrations
    └── hetzner_provider.go  # Hetzner Cloud API client
```

## Important Code Patterns

### Conductor Initialization

The Conductor is initialized in `cmd/api/main.go` after all services. This order is critical:

1. Database connection
2. Repositories
3. Services (minecraft, billing, lifecycle, etc.)
4. Conductor
5. Scaling Engine (if Hetzner token configured)

**Important:** Always perform container state sync and queue sync on startup to prevent data loss after restarts.

### Resource Guard Pattern

The system uses a **proportional overhead model** to prevent OOM:

- Dedicated nodes: Fixed 1GB system overhead
- Cloud nodes: 15% proportional overhead (e.g., 32GB → 27.2GB usable)

Formula: `UsableRAM = TotalRAM - (TotalRAM * 0.15)`

This is implemented in `internal/conductor/node_registry.go` and `internal/conductor/conductor.go`.

### Node Selection Algorithm

When placing containers, the Conductor (`internal/conductor/node_selector.go`) considers:

1. **RAM Availability** - Node has sufficient free RAM
2. **Status** - Node must be healthy (not draining/unhealthy)
3. **Node Type** - Prefer dedicated over cloud nodes
4. **Load Balancing** - Select least loaded node

Never hardcode node selection. Always use `conductor.SelectNodeForContainer()`.

### Event-Driven Billing

The system uses an event bus (`internal/events/`) to track server lifecycle events:

- Server start → `ServerStarted` event → Billing entry created
- Server stop → `ServerStopped` event → Billing entry closed
- Events stored in PostgreSQL + InfluxDB (optional)

This ensures accurate per-minute billing without polling.

### Health Checking

The Health Checker (`internal/conductor/health_checker.go`) runs every 60 seconds:

- Checks node SSH connectivity
- Checks Docker daemon health
- Marks nodes as unhealthy after 3 failed checks
- Marks nodes as healthy only after cloud-init completes
- Re-registers nodes after health status changes

## Configuration

Edit `.env` for configuration. Key settings:

### Auto-Scaling Configuration
```env
# Required for auto-scaling
HETZNER_CLOUD_TOKEN=your-token-here
HETZNER_SSH_KEY_NAME=payperplay-main
SCALING_ENABLED=true

# Scaling thresholds
SCALING_SCALE_UP_THRESHOLD=85.0   # Provision VM when >85% capacity
SCALING_SCALE_DOWN_THRESHOLD=30.0  # Decommission when <30% for 30min
SCALING_MAX_CLOUD_NODES=10         # Safety limit
```

### System Resource Reservation
```env
SYSTEM_RESERVED_RAM_MB=1000        # Base overhead for system services
SYSTEM_RESERVED_RAM_PERCENT=15.0   # Additional % for cloud nodes
```

### Minecraft Settings
```env
SERVERS_BASE_PATH=./minecraft/servers
DEFAULT_IDLE_TIMEOUT=300           # 5 minutes
MC_PORT_START=25565
MC_PORT_END=25665
```

## API Endpoints

The API is organized into logical groups:

**Server Management** (`/api/servers`)
- `POST /api/servers` - Create server (queues if no capacity)
- `GET /api/servers` - List servers
- `POST /api/servers/:id/start` - Start server
- `POST /api/servers/:id/stop` - Stop server
- `DELETE /api/servers/:id` - Delete server

**Fleet Management** (`/api/conductor`)
- `GET /api/conductor/status` - Fleet status (nodes, capacity, queue)
- `POST /api/conductor/nodes/register` - Register node
- `GET /api/conductor/nodes` - List nodes

**Auto-Scaling** (`/api/scaling`)
- `GET /api/scaling/status` - Scaling status
- `POST /api/scaling/trigger` - Force scaling check

**Monitoring**
- `GET /health` - Health check
- `GET /metrics` - Prometheus metrics
- `GET /ws` - WebSocket for real-time updates

## Testing & Debugging

### Manual Testing on Production
```bash
# Create a test server
TOKEN="your-jwt-token"
curl -X POST "http://localhost:8000/api/servers" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"TestServer","minecraft_version":"1.21","server_type":"paper","ram_mb":4096,"port":0}'

# Check fleet status
curl http://localhost:8000/conductor/status | python3 -c "import sys, json; print(json.dumps(json.load(sys.stdin), indent=2))"

# Monitor logs (production)
ssh root@91.98.202.235 "docker compose -f /root/PayPerPlayHosting/docker-compose.prod.yml logs payperplay --tail=100"

# Check database
ssh root@91.98.202.235 'docker exec payperplay-postgres psql -U payperplay -d payperplay -c "SELECT id, name, status, ram_mb, node_id FROM minecraft_servers;"'
```

### Common Issues

**Queue not processing:**
- Check if any nodes are registered and healthy
- Check logs for "Evaluating scaling" and "REACTIVE" keywords
- Ensure Hetzner token is configured if no nodes available

**Node shows as unhealthy:**
- Check SSH connectivity: `ssh root@<node-ip> "docker ps"`
- Check if cloud-init completed: Look for "Cloud-Init completed" in logs
- Health checker waits for cloud-init before marking nodes healthy

**Containers not starting:**
- Check resource guard: Ensure node has sufficient RAM
- Check Docker logs: `docker logs <container-id>`
- Verify port allocation: Ports 25565-25665 must be available

## Important Conventions

### Database Migrations

The system uses GORM AutoMigrate. Add new fields to models in `internal/models/` and run the application to auto-migrate.

For production schema changes, SSH into the production server and run SQL directly:
```bash
ssh root@91.98.202.235 'docker exec payperplay-postgres psql -U payperplay -d payperplay -c "ALTER TABLE..."'
```

### Logging Standards

Use structured logging with the `pkg/logger` package:

```go
logger.Info("Server started", "server_id", serverID, "node_id", nodeID)
logger.Warn("Node unhealthy", "node_id", nodeID, "reason", reason)
logger.Error("Failed to provision VM", "error", err)
logger.Debug("Scaling check", "capacity_percent", capacityPercent)
```

Log levels:
- `DEBUG` - Detailed diagnostics (scaling checks, state transitions)
- `INFO` - Important state changes (server started, node registered)
- `WARN` - Recoverable issues (node unhealthy, retry attempts)
- `ERROR` - Critical failures (provisioning failed, DB errors)

### Error Handling

Always return structured errors from API handlers:

```go
c.JSON(http.StatusBadRequest, gin.H{
    "error": "Insufficient capacity",
    "details": "No healthy nodes available",
})
```

Use the `errors` package for wrapping errors with context:

```go
return fmt.Errorf("failed to provision VM: %w", err)
```

### Docker Container Management

All Docker operations go through `internal/docker/service.go`. Never call the Docker API directly. This ensures consistent error handling and logging.

### Node Registration

Nodes can be registered in two ways:

1. **Manual Registration** - Via API or database insert (for dedicated servers)
2. **Auto-Registration** - VM provisioner registers placeholder node BEFORE Hetzner API call to prevent race conditions

The placeholder node pattern prevents the queue processor from triggering duplicate provisioning requests.

## Deployment Notes

### Production Stack

The production environment runs on `root@91.98.202.235` with:

- **PayPerPlay API** - Go binary in Docker container
- **PostgreSQL 16** - Database
- **Nginx** - Reverse proxy
- **Velocity Proxy** - Minecraft proxy at port 25565

Deploy with: `./deploy-production.sh`

### Velocity Plugin

The Velocity plugin is separate from the main Go application:

```bash
cd velocity-plugin
mvn clean package
./deploy-velocity.sh  # Copies JAR to production Velocity server
```

The plugin handles auto-wake functionality (player connects → API call to start server → dynamic routing).

### Monitoring

- **Prometheus metrics** - `/metrics` endpoint
- **WebSocket dashboard** - `/ws` for real-time fleet status
- **InfluxDB** (optional) - Time-series metrics for predictive scaling

## Resources

- **Main Docs**: `README.md`, `QUICKSTART.md`, `FEATURES.md`
- **Architecture**: `PLAN.md` (872 lines of business/technical design)
- **Deployment**: `DEPLOYMENT.md`
- **Troubleshooting**: `TROUBLESHOOTING.md`, `LOGS.md`
- **Design Docs**: `CONSOLIDATION_POLICY_DESIGN.md`, `VELOCITY_DESIGN.md`

---

Built with Go 1.23+, Docker, Gin, GORM, and PostgreSQL. Deployed on Hetzner infrastructure.
