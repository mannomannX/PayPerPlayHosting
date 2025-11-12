# Velocity Remote API Plugin

HTTP REST API for dynamic Minecraft server registration in Velocity Proxy.

## Overview

This plugin enables the PayPerPlay Control Plane (Tier 1) to dynamically register/unregister Minecraft servers with the Velocity Proxy (Tier 2) without requiring direct access to Velocity configuration files.

## Architecture

```
┌─────────────────┐         HTTP API          ┌──────────────────┐
│  Control Plane  │ ────────────────────────> │  Velocity Proxy  │
│   (Tier 1)      │  POST /api/servers        │     (Tier 2)     │
│  91.98.202.235  │  DELETE /api/servers/:id  │  91.98.232.193   │
└─────────────────┘                            └──────────────────┘
                                                        │
                                                        │ Routes players
                                                        │
                                                        v
                                              ┌──────────────────┐
                                              │  MC Servers      │
                                              │   (Tier 3)       │
                                              │  Cloud Nodes     │
                                              └──────────────────┘
```

## API Endpoints

### POST /api/servers
Register a new backend Minecraft server with Velocity.

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
  "status": "ok",
  "message": "Server registered successfully",
  "name": "survival-1",
  "address": "91.98.202.235:25566"
}
```

### DELETE /api/servers/:name
Unregister a backend server.

**Response:**
```json
{
  "status": "ok",
  "message": "Server unregistered successfully",
  "name": "survival-1"
}
```

### GET /api/servers
List all registered servers.

**Response:**
```json
{
  "status": "ok",
  "count": 2,
  "servers": [
    {
      "name": "survival-1",
      "address": "91.98.202.235:25566",
      "players": 5
    },
    {
      "name": "creative-1",
      "address": "91.98.202.235:25567",
      "players": 2
    }
  ]
}
```

### GET /api/players/:server
Get player count for a specific server.

**Response:**
```json
{
  "status": "ok",
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
  "servers_count": 2,
  "players_online": 7
}
```

## Building

Requires Maven 3.6+ and Java 17+.

```bash
mvn clean package
```

Output: `target/velocity-remote-api-1.0.0.jar`

## Installation

### Automatic Deployment

Use the provided deployment script:

```bash
cd velocity-plugin
./deploy-velocity-server.sh
```

This will:
1. Build the plugin
2. Install Docker on the Velocity server
3. Create the necessary directory structure
4. Deploy the plugin
5. Start Velocity in a Docker container

### Manual Installation

1. Build the plugin:
   ```bash
   mvn clean package
   ```

2. Copy JAR to Velocity plugins folder:
   ```bash
   scp target/velocity-remote-api-1.0.0.jar root@91.98.232.193:/path/to/velocity/plugins/
   ```

3. Restart Velocity:
   ```bash
   docker restart velocity-proxy
   ```

## Configuration

No plugin configuration required. The API listens on port 8080 by default.

## Testing

### Test Health Endpoint
```bash
curl http://91.98.232.193:8080/health
```

### Test Server Registration
```bash
curl -X POST http://91.98.232.193:8080/api/servers \
  -H "Content-Type: application/json" \
  -d '{"name":"test-server","address":"91.98.202.235:25566"}'
```

### Test Server List
```bash
curl http://91.98.232.193:8080/api/servers
```

### Test Server Unregistration
```bash
curl -X DELETE http://91.98.232.193:8080/api/servers/test-server
```

## Integration with PayPerPlay API

### Environment Configuration

Add to `.env` on the API server (91.98.202.235):

```bash
VELOCITY_API_URL=http://91.98.232.193:8080
```

### Usage in Go Code

```go
import "github.com/payperplay/hosting/internal/velocity"

// Create client
client := velocity.NewRemoteVelocityClient(cfg.VelocityAPIURL)

// Register server when starting
err := client.RegisterServer("survival-1", "91.98.202.235:25566")

// Unregister server when stopping
err := client.UnregisterServer("survival-1")

// Health check
health, err := client.HealthCheck()
```

## Troubleshooting

### Plugin not loading
Check Velocity logs:
```bash
docker logs velocity-proxy
```

### API not accessible
Verify port 8080 is exposed:
```bash
docker port velocity-proxy
```

Check firewall rules:
```bash
sudo ufw allow 8080/tcp
```

### Cannot register servers
Verify Velocity is running:
```bash
docker ps | grep velocity
```

Check plugin status in logs:
```bash
docker logs velocity-proxy 2>&1 | grep "VelocityRemoteAPI"
```

## Dependencies

- Velocity API 3.3.0-SNAPSHOT
- Javalin 5.6.3 (HTTP framework)
- Jackson 2.15.3 (JSON processing)

## License

Part of the PayPerPlay Hosting platform.
