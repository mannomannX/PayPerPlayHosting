# Velocity Plugin Deployment Guide

This guide explains how to deploy the Velocity Remote API plugin to the production Velocity server.

## Architecture Overview

```
┌──────────────────────────────────────────────────────────────┐
│                     3-Tier Architecture                       │
├──────────────────────────────────────────────────────────────┤
│                                                               │
│  Tier 1: Control Plane (91.98.202.235)                      │
│  ├─ PayPerPlay API (Go)                                      │
│  ├─ PostgreSQL Database                                      │
│  └─ Communicates with Velocity via HTTP API                  │
│                                                               │
│  Tier 2: Velocity Proxy (91.98.232.193)                     │
│  ├─ Velocity Proxy (Minecraft)  Port: 25577                 │
│  ├─ Remote API Plugin (Java)     Port: 8080                 │
│  └─ Routes players to backend servers                        │
│                                                               │
│  Tier 3: Minecraft Servers (Cloud Nodes)                    │
│  ├─ Paper/Spigot/etc servers                                │
│  └─ Dynamically registered with Velocity                     │
│                                                               │
└──────────────────────────────────────────────────────────────┘
```

## Quick Start

### 1. Deploy using the automated script:

```bash
cd velocity-plugin
chmod +x deploy-velocity.sh
./deploy-velocity.sh
```

That's it! The script handles everything:
- Git pull (if in a repo)
- Maven build using Docker
- Copy JAR to Velocity server
- Restart Velocity container
- Health check verification

## Manual Deployment Steps

If you need to deploy manually or want to understand what the script does:

### Step 1: Build the Plugin

```bash
cd velocity-plugin

# Using Docker (recommended - no local Java/Maven needed):
docker run --rm \
  -v "$(pwd):/workspace" \
  -w /workspace \
  maven:3.8-openjdk-17 \
  mvn clean package

# OR using local Maven (if installed):
mvn clean package
```

This creates: `target/velocity-remote-api-1.0.0.jar`

### Step 2: Copy to Velocity Server

```bash
# Copy the built JAR to Velocity server
scp target/velocity-remote-api-1.0.0.jar root@91.98.232.193:/root/velocity/plugins/
```

### Step 3: Restart Velocity

```bash
# SSH to Velocity server and restart container
ssh root@91.98.232.193 "cd /root/velocity && docker compose restart velocity"
```

### Step 4: Verify Deployment

```bash
# Wait 10 seconds for startup, then test API
sleep 10
curl http://91.98.232.193:8080/health

# Should return:
# {"status":"ok","version":"1.0.0","servers_count":0,"players_online":0}
```

## Git-Based Workflow

### Initial Setup (One-time)

1. **Create a Git repository** (if not already done):

```bash
cd C:\Users\Robin\Desktop\PayPerPlayHosting
git add velocity-plugin/
git commit -m "Add Velocity Remote API plugin"
git push origin main
```

2. **Clone on Velocity server** (optional - for remote builds):

```bash
ssh root@91.98.232.193
cd /root
git clone https://github.com/yourusername/PayPerPlayHosting.git
```

### Regular Deployment Workflow

1. **Make code changes locally**:

```bash
# Edit files in velocity-plugin/src/main/java/com/payperplay/velocity/
# Example: vim RemoteAPI.java
```

2. **Commit and push**:

```bash
git add velocity-plugin/
git commit -m "Update Velocity plugin: Add new feature"
git push origin main
```

3. **Deploy to production**:

```bash
cd velocity-plugin
./deploy-velocity.sh
```

The script will:
- Pull latest code from git (if you run it in a repo)
- Build fresh JAR from current code
- Deploy to Velocity server
- Restart and verify

## File Structure

```
velocity-plugin/
├── src/
│   └── main/
│       ├── java/
│       │   └── com/payperplay/velocity/
│       │       └── RemoteAPI.java        # Main plugin code
│       └── resources/
│           └── velocity-plugin.json       # Plugin metadata
├── pom.xml                                # Maven configuration
├── deploy-velocity.sh                     # Automated deployment script
├── DEPLOYMENT.md                          # This file
└── target/                                # Build output (generated)
    └── velocity-remote-api-1.0.0.jar     # Compiled plugin
```

## Configuration Files on Velocity Server

Located at: `/root/velocity/` on `91.98.232.193`

```
/root/velocity/
├── docker-compose.yml           # Velocity container configuration
├── config/
│   ├── velocity.toml           # Velocity proxy configuration
│   └── forwarding.secret       # Player info forwarding secret
└── plugins/
    └── velocity-remote-api-1.0.0.jar  # Our plugin
```

### docker-compose.yml

```yaml
version: '3.8'

services:
  velocity:
    image: itzg/bungeecord:java17
    container_name: payperplay-velocity
    environment:
      TYPE: VELOCITY
      MEMORY: 512M
    ports:
      - "25577:25577"   # Minecraft proxy
      - "8080:8080"      # HTTP API
    volumes:
      - ./config:/config
      - ./plugins:/plugins
    restart: unless-stopped
    networks:
      - velocity-network

networks:
  velocity-network:
    driver: bridge
```

### velocity.toml (Key Settings)

```toml
bind = "0.0.0.0:25577"
motd = "<#09add3>PayPerPlay Velocity Proxy"
show-max-players = 500
online-mode = true
force-key-authentication = true
player-info-forwarding-mode = "modern"
forwarding-secret-file = "forwarding.secret"
```

## API Endpoints

Once deployed, the following HTTP endpoints are available at `http://91.98.232.193:8080`:

### POST /api/servers
Register a new backend server with Velocity.

**Request:**
```json
{
  "name": "survival-1",
  "address": "91.98.202.235:25566"
}
```

**Response:**
```json
{
  "message": "Server registered successfully",
  "name": "survival-1",
  "address": "91.98.202.235:25566"
}
```

### DELETE /api/servers/{name}
Unregister a backend server.

**Example:**
```bash
curl -X DELETE http://91.98.232.193:8080/api/servers/survival-1
```

### GET /api/servers
List all registered servers.

**Response:**
```json
{
  "servers": [
    {
      "name": "survival-1",
      "address": "91.98.202.235:25566",
      "players": 5
    }
  ],
  "total": 1
}
```

### GET /api/players/{server}
Get player count for a specific server.

**Example:**
```bash
curl http://91.98.232.193:8080/api/players/survival-1
```

**Response:**
```json
{
  "server": "survival-1",
  "players": 5
}
```

### GET /health
Health check endpoint.

**Response:**
```json
{
  "status": "ok",
  "version": "1.0.0",
  "servers_count": 1,
  "players_online": 5
}
```

## Troubleshooting

### Build fails with "Cannot find symbol"

**Cause**: Old Java files with missing dependencies.

**Fix**: Make sure only `RemoteAPI.java` exists in `src/main/java/com/payperplay/velocity/`:

```bash
cd velocity-plugin
ls src/main/java/com/payperplay/velocity/

# Should only show: RemoteAPI.java

# If you see old files (ServerWakeupListener.java, PayPerPlayPlugin.java, PluginConfig.java):
rm src/main/java/com/payperplay/velocity/ServerWakeupListener.java
rm src/main/java/com/payperplay/velocity/PayPerPlayPlugin.java
rm src/main/java/com/payperplay/velocity/PluginConfig.java
```

### Health check fails after deployment

**Cause**: Velocity container not fully started.

**Fix**: Wait 15-20 seconds and try again:

```bash
ssh root@91.98.232.193 'docker logs payperplay-velocity'

# Look for:
# [INFO] Listening on /0.0.0.0:25577
# [INFO] VelocityRemoteAPI plugin loaded
# [INFO] Remote API started on port 8080
```

### Plugin not loading

**Cause**: JAR not in correct location or Velocity needs restart.

**Fix**:

```bash
# Verify JAR exists
ssh root@91.98.232.193 'ls -lh /root/velocity/plugins/'

# Should show: velocity-remote-api-1.0.0.jar

# Restart Velocity
ssh root@91.98.232.193 'cd /root/velocity && docker compose restart velocity'

# Check logs
ssh root@91.98.232.193 'docker logs -f payperplay-velocity'
```

### Can't connect to Velocity from Control Plane

**Cause**: Network connectivity or wrong URL.

**Fix**: Verify from Control Plane server:

```bash
# SSH to Control Plane
ssh root@91.98.202.235

# Test connectivity
curl http://91.98.232.193:8080/health

# Check environment variable
docker exec payperplay-api-1 env | grep VELOCITY_API_URL

# Should show: VELOCITY_API_URL=http://91.98.232.193:8080
```

## Useful Commands

### View Velocity logs (live):

```bash
ssh root@91.98.232.193 'docker logs -f payperplay-velocity'
```

### Check container status:

```bash
ssh root@91.98.232.193 'docker ps'
```

### Restart Velocity container:

```bash
ssh root@91.98.232.193 'cd /root/velocity && docker compose restart velocity'
```

### Test API endpoints:

```bash
# Health check
curl http://91.98.232.193:8080/health

# List servers
curl http://91.98.232.193:8080/api/servers

# Register a test server
curl -X POST http://91.98.232.193:8080/api/servers \
  -H "Content-Type: application/json" \
  -d '{"name":"test-server","address":"91.98.202.235:25566"}'
```

### Check Velocity configuration:

```bash
ssh root@91.98.232.193 'cat /root/velocity/config/velocity.toml'
```

### Clean rebuild:

```bash
cd velocity-plugin
rm -rf target/
./deploy-velocity.sh
```

## Next Steps

After deployment is working:

1. **Integrate with server lifecycle** (Task 1.7):
   - Modify `internal/service/minecraft_service.go` to call Velocity API when servers start/stop
   - Automatic server registration on startup
   - Automatic unregistration on shutdown

2. **End-to-end testing** (Task 1.8):
   - Create a real Minecraft server
   - Register it with Velocity via API
   - Connect as a player through Velocity proxy
   - Verify player routing works

3. **Eliminate local-node** (Phase 2):
   - Move all Minecraft servers to cloud nodes
   - Keep only Control Plane and Velocity on fixed IPs

## Support

- Velocity docs: https://docs.papermc.io/velocity
- Javalin docs: https://javalin.io/documentation
- Project issues: [GitHub Issues]
