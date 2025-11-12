# 3-Tier Architecture Migration Guide

**Ziel:** Von Hybrid-Monolith zu echter 3-Tier Architektur
**Einsparung:** 70€/month → 7€/month Baseline (90% günstiger!)
**Zeitaufwand:** 8-12 Tage (Phase 1+2)

---

## Aktueller Zustand vs. Ziel

### ❌ IST (Hybrid-Monolith):
```
┌────────────────────────────────────────────────┐
│ Main Server (91.98.202.235) - 70€/month       │
│ = API + PostgreSQL + Velocity + MC-Server     │
└────────────────────────────────────────────────┘
         ↓ (Auto-Scaling)
┌────────────────────────────────────────────────┐
│ Cloud Nodes (Hetzner CX21/CX31)                │
│ = NUR Minecraft Server                         │
└────────────────────────────────────────────────┘
```

**Probleme:**
- ❌ Velocity + API auf demselben Server
- ❌ "local-node" erlaubt MC-Server auf API-Server
- ❌ 70€/month Fixkosten

---

### ✅ SOLL (Echtes 3-Tier):
```
┌────────────────────────────────┐
│ TIER 1: Control Plane          │
│ Hetzner CX11 - 3.50€/month     │
│ = API + PostgreSQL + Dashboard │
└────────────────────────────────┘
         ↓ (orchestriert)
┌────────────────────────────────┐
│ TIER 2: Proxy Layer            │
│ Hetzner CX11 - 3.50€/month     │
│ = Velocity Proxy ONLY          │
└────────────────────────────────┘
         ↓ (routet zu)
┌────────────────────────────────┐
│ TIER 3: Workload Layer         │
│ Dynamic (0€ bei 0 Last)        │
│ = NUR Minecraft Server         │
└────────────────────────────────┘
```

**Total: 7€/month Baseline**

---

## Phase 1: Velocity Isolation (3-4 Tage)

### Schritt 1.1: Velocity-VM erstellen

**Hetzner Cloud CX11 (2GB RAM, 1 vCPU, 3.50€/month)**

```bash
# 1. SSH Key erstellen (falls noch nicht vorhanden)
ssh-keygen -t ed25519 -C "payperplay-velocity" -f ~/.ssh/payperplay-velocity

# 2. Public Key zu Hetzner hinzufügen
cat ~/.ssh/payperplay-velocity.pub
# → Kopieren und in Hetzner Cloud Console als "payperplay-main" SSH Key hinzufügen

# 3. VM erstellen via Hetzner Cloud Console ODER hcloud CLI:
hcloud server create \
  --name payperplay-velocity \
  --type cx11 \
  --image ubuntu-22.04 \
  --ssh-key payperplay-main \
  --location nbg1

# Output:
# Server 'payperplay-velocity' created
# IPv4: 95.217.xxx.xxx
```

**Speichere die IP-Adresse!** (z.B. `95.217.xxx.xxx`)

---

### Schritt 1.2: Docker & Velocity installieren

```bash
# SSH zu Velocity-VM
ssh root@95.217.xxx.xxx

# Docker installieren
curl -fsSL https://get.docker.com | sh
systemctl enable docker
systemctl start docker

# Verzeichnisse erstellen
mkdir -p /opt/velocity/{config,plugins}
cd /opt/velocity
```

**Velocity Docker Container (mit Remote API Plugin):**

```bash
# velocity-docker-compose.yml erstellen
cat > /opt/velocity/docker-compose.yml <<'EOF'
version: '3.8'

services:
  velocity:
    image: itzg/bungeecord:latest
    container_name: velocity-proxy
    restart: unless-stopped
    ports:
      - "25565:25577"  # Minecraft Port (außen:innen)
      - "8080:8080"    # Remote API Port
    environment:
      - TYPE=VELOCITY
      - VERSION=LATEST
      - MEMORY=512M
    volumes:
      - ./config:/config
      - ./plugins:/plugins
    networks:
      - velocity-net

networks:
  velocity-net:
    driver: bridge
EOF

# Container starten
docker compose up -d

# Logs prüfen
docker logs -f velocity-proxy
```

**Expected Output:**
```
[INFO] Velocity is listening on 0.0.0.0:25577
```

**Test:**
```bash
# Von deinem lokalen PC:
nc -zv 95.217.xxx.xxx 25565
# Should output: Connection to 95.217.xxx.xxx 25565 port [tcp/minecraft] succeeded!
```

---

### Schritt 1.3: Remote API Plugin entwickeln (Java)

**Problem:** Velocity braucht ein Plugin für dynamische Server-Registrierung via HTTP.

**Lösung:** VelocityRemoteAPI Plugin

**Datei:** `velocity-remote-api/src/main/java/com/payperplay/velocity/RemoteAPI.java`

```java
package com.payperplay.velocity;

import com.google.inject.Inject;
import com.velocitypowered.api.event.Subscribe;
import com.velocitypowered.api.event.proxy.ProxyInitializeEvent;
import com.velocitypowered.api.plugin.Plugin;
import com.velocitypowered.api.proxy.ProxyServer;
import com.velocitypowered.api.proxy.server.ServerInfo;
import io.javalin.Javalin;
import io.javalin.http.Context;
import org.slf4j.Logger;

import java.net.InetSocketAddress;
import java.util.Map;

@Plugin(
    id = "velocity-remote-api",
    name = "VelocityRemoteAPI",
    version = "1.0.0",
    authors = {"PayPerPlay"}
)
public class RemoteAPI {
    private final ProxyServer server;
    private final Logger logger;
    private Javalin app;

    @Inject
    public RemoteAPI(ProxyServer server, Logger logger) {
        this.server = server;
        this.logger = logger;
    }

    @Subscribe
    public void onProxyInitialization(ProxyInitializeEvent event) {
        // Start HTTP server on port 8080
        app = Javalin.create(config -> {
            config.showJavalinBanner = false;
        }).start(8080);

        // POST /api/servers - Register new server
        app.post("/api/servers", this::registerServer);

        // DELETE /api/servers/:name - Unregister server
        app.delete("/api/servers/{name}", this::unregisterServer);

        // GET /api/servers - List all servers
        app.get("/api/servers", this::listServers);

        // GET /api/players/:server - Get player count
        app.get("/api/players/{server}", this::getPlayerCount);

        logger.info("VelocityRemoteAPI started on port 8080");
    }

    private void registerServer(Context ctx) {
        try {
            Map<String, String> body = ctx.bodyAsClass(Map.class);
            String name = body.get("name");
            String address = body.get("address");

            if (name == null || address.isEmpty()) {
                ctx.status(400).result("Missing 'name' or 'address'");
                return;
            }

            // Parse address (e.g., "10.0.1.5:25566")
            String[] parts = address.split(":");
            String host = parts[0];
            int port = Integer.parseInt(parts[1]);

            // Register server to Velocity
            ServerInfo serverInfo = new ServerInfo(name, new InetSocketAddress(host, port));
            server.registerServer(serverInfo);

            logger.info("Registered server: {} -> {}", name, address);
            ctx.status(200).result("Server registered");

        } catch (Exception e) {
            logger.error("Failed to register server", e);
            ctx.status(500).result("Internal server error: " + e.getMessage());
        }
    }

    private void unregisterServer(Context ctx) {
        try {
            String name = ctx.pathParam("name");

            server.getServer(name).ifPresent(registeredServer -> {
                server.unregisterServer(registeredServer.getServerInfo());
                logger.info("Unregistered server: {}", name);
            });

            ctx.status(200).result("Server unregistered");

        } catch (Exception e) {
            logger.error("Failed to unregister server", e);
            ctx.status(500).result("Internal server error: " + e.getMessage());
        }
    }

    private void listServers(Context ctx) {
        Map<String, String> servers = new java.util.HashMap<>();
        server.getAllServers().forEach(registeredServer -> {
            ServerInfo info = registeredServer.getServerInfo();
            servers.put(info.getName(), info.getAddress().toString());
        });

        ctx.json(servers);
    }

    private void getPlayerCount(Context ctx) {
        String serverName = ctx.pathParam("server");

        server.getServer(serverName).ifPresentOrElse(
            registeredServer -> {
                int playerCount = registeredServer.getPlayersConnected().size();
                ctx.json(Map.of("server", serverName, "players", playerCount));
            },
            () -> ctx.status(404).result("Server not found")
        );
    }
}
```

**pom.xml:**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>

    <groupId>com.payperplay</groupId>
    <artifactId>velocity-remote-api</artifactId>
    <version>1.0.0</version>
    <packaging>jar</packaging>

    <properties>
        <java.version>17</java.version>
        <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
    </properties>

    <repositories>
        <repository>
            <id>papermc</id>
            <url>https://repo.papermc.io/repository/maven-public/</url>
        </repository>
    </repositories>

    <dependencies>
        <!-- Velocity API -->
        <dependency>
            <groupId>com.velocitypowered</groupId>
            <artifactId>velocity-api</artifactId>
            <version>3.3.0-SNAPSHOT</version>
            <scope>provided</scope>
        </dependency>

        <!-- Javalin (lightweight HTTP server) -->
        <dependency>
            <groupId>io.javalin</groupId>
            <artifactId>javalin</artifactId>
            <version>5.6.3</version>
        </dependency>

        <!-- Gson (JSON parsing) -->
        <dependency>
            <groupId>com.google.code.gson</groupId>
            <artifactId>gson</artifactId>
            <version>2.10.1</version>
        </dependency>
    </dependencies>

    <build>
        <plugins>
            <plugin>
                <groupId>org.apache.maven.plugins</groupId>
                <artifactId>maven-compiler-plugin</artifactId>
                <version>3.11.0</version>
                <configuration>
                    <source>17</source>
                    <target>17</target>
                </configuration>
            </plugin>

            <plugin>
                <groupId>org.apache.maven.plugins</groupId>
                <artifactId>maven-shade-plugin</artifactId>
                <version>3.5.0</version>
                <executions>
                    <execution>
                        <phase>package</phase>
                        <goals>
                            <goal>shade</goal>
                        </goals>
                        <configuration>
                            <createDependencyReducedPom>false</createDependencyReducedPom>
                        </configuration>
                    </execution>
                </executions>
            </plugin>
        </plugins>
    </build>
</project>
```

**Build & Deploy:**

```bash
# Build plugin
mvn clean package

# Kopiere JAR zu Velocity-VM
scp target/velocity-remote-api-1.0.0.jar root@95.217.xxx.xxx:/opt/velocity/plugins/

# Restart Velocity
ssh root@95.217.xxx.xxx "docker restart velocity-proxy"

# Test API
curl -X POST http://95.217.xxx.xxx:8080/api/servers \
  -H "Content-Type: application/json" \
  -d '{"name":"test-server","address":"10.0.1.5:25566"}'

# Expected: "Server registered"
```

---

### Schritt 1.4: Control Plane anpassen (Go)

**Neue Datei:** `internal/velocity/remote_client.go`

```go
package velocity

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

// RemoteVelocityClient communicates with Velocity via HTTP API
type RemoteVelocityClient struct {
    apiURL     string
    httpClient *http.Client
}

// ServerRegistration represents a server to register
type ServerRegistration struct {
    Name    string `json:"name"`
    Address string `json:"address"`
}

// NewRemoteVelocityClient creates a new client
func NewRemoteVelocityClient(apiURL string) *RemoteVelocityClient {
    return &RemoteVelocityClient{
        apiURL: apiURL,
        httpClient: &http.Client{
            Timeout: 10 * time.Second,
        },
    }
}

// RegisterServer registers a Minecraft server with Velocity
func (c *RemoteVelocityClient) RegisterServer(name, address string) error {
    payload := ServerRegistration{
        Name:    name,
        Address: address,
    }

    jsonData, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("failed to marshal payload: %w", err)
    }

    resp, err := c.httpClient.Post(
        c.apiURL+"/api/servers",
        "application/json",
        bytes.NewBuffer(jsonData),
    )
    if err != nil {
        return fmt.Errorf("failed to send request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }

    return nil
}

// UnregisterServer removes a server from Velocity
func (c *RemoteVelocityClient) UnregisterServer(name string) error {
    req, err := http.NewRequest(
        "DELETE",
        fmt.Sprintf("%s/api/servers/%s", c.apiURL, name),
        nil,
    )
    if err != nil {
        return fmt.Errorf("failed to create request: %w", err)
    }

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("failed to send request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }

    return nil
}

// GetPlayerCount returns the number of players on a server
func (c *RemoteVelocityClient) GetPlayerCount(serverName string) (int, error) {
    resp, err := c.httpClient.Get(
        fmt.Sprintf("%s/api/players/%s", c.apiURL, serverName),
    )
    if err != nil {
        return 0, fmt.Errorf("failed to send request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusNotFound {
        return 0, fmt.Errorf("server not found")
    }

    if resp.StatusCode != http.StatusOK {
        return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }

    var result struct {
        Server  string `json:"server"`
        Players int    `json:"players"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return 0, fmt.Errorf("failed to decode response: %w", err)
    }

    return result.Players, nil
}
```

**Config anpassen:** `pkg/config/config.go`

```go
type Config struct {
    // ... existing fields ...

    // Velocity Proxy (Tier 2)
    VelocityAPIURL string // e.g., "http://95.217.xxx.xxx:8080"
}

func Load() *Config {
    // ... existing code ...

    config := &Config{
        // ... existing fields ...
        VelocityAPIURL: getEnv("VELOCITY_API_URL", "http://localhost:8080"),
    }

    return config
}
```

**Main anpassen:** `cmd/api/main.go`

```go
// Initialize Velocity service (REMOTE)
velocityClient := velocity.NewRemoteVelocityClient(cfg.VelocityAPIURL)
logger.Info("Velocity Remote Client initialized", map[string]interface{}{
    "api_url": cfg.VelocityAPIURL,
})

// Link to MinecraftService
mcService.SetVelocityClient(velocityClient)
```

**MinecraftService anpassen:** `internal/service/minecraft_service.go`

```go
type MinecraftService struct {
    // ... existing fields ...
    velocityClient *velocity.RemoteVelocityClient
}

func (s *MinecraftService) SetVelocityClient(client *velocity.RemoteVelocityClient) {
    s.velocityClient = client
}

// When starting a server, register with Velocity:
func (s *MinecraftService) StartServer(serverID string) error {
    // ... existing code to start container ...

    // Get node IP and port
    nodeIP := remoteNode.IPAddress // e.g., "10.0.1.5"
    serverPort := server.Port       // e.g., 25566

    // Register with Velocity
    if s.velocityClient != nil {
        address := fmt.Sprintf("%s:%d", nodeIP, serverPort)
        err := s.velocityClient.RegisterServer(server.Name, address)
        if err != nil {
            logger.Error("Failed to register server with Velocity", err, map[string]interface{}{
                "server": server.Name,
                "address": address,
            })
        } else {
            logger.Info("Server registered with Velocity", map[string]interface{}{
                "server": server.Name,
                "address": address,
            })
        }
    }

    return nil
}

// When stopping a server, unregister from Velocity:
func (s *MinecraftService) StopServer(serverID string, reason string) error {
    // ... existing code to stop container ...

    // Unregister from Velocity
    if s.velocityClient != nil {
        err := s.velocityClient.UnregisterServer(server.Name)
        if err != nil {
            logger.Warn("Failed to unregister server from Velocity", map[string]interface{}{
                "server": server.Name,
                "error": err.Error(),
            })
        }
    }

    return nil
}
```

**Environment Variable setzen:**

```bash
# In docker-compose.prod.yml oder .env:
VELOCITY_API_URL=http://95.217.xxx.xxx:8080
```

---

### Schritt 1.5: Testing - Velocity Isolation

```bash
# 1. Server erstellen
curl -X POST http://91.98.202.235:8000/api/servers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "velocity-test-1",
    "minecraft_version": "1.21",
    "server_type": "paper",
    "ram_mb": 1024,
    "port": 0
  }'

# 2. Server starten
curl -X POST http://91.98.202.235:8000/api/servers/{SERVER_ID}/start \
  -H "Authorization: Bearer $TOKEN"

# 3. Prüfe Velocity-Logs
ssh root@95.217.xxx.xxx "docker logs velocity-proxy | grep velocity-test-1"

# Expected: "Registered server: velocity-test-1 -> 10.0.1.5:25566"

# 4. Prüfe ob Server in Velocity registriert ist
curl http://95.217.xxx.xxx:8080/api/servers

# Expected: {"velocity-test-1":"10.0.1.5:25566"}

# 5. Test Minecraft Connection
# Von deinem lokalen PC:
# Minecraft → Server: play.payperplay.host:25565
# Velocity routet automatisch zu velocity-test-1
```

**Bei Erfolg:**
- ✅ Velocity läuft auf separater VM (Tier 2)
- ✅ API kann Server dynamisch bei Velocity registrieren
- ✅ Spieler können via Velocity verbinden

---

## Phase 2: "local-node" Elimination (1-2 Tage)

### Problem
Aktuell erlaubt `isLocalNode()` Minecraft-Server auf dem API-Server:

```go
if s.isLocalNode(selectedNodeID) {
    s.dockerService.CreateContainer(...) // ❌ Läuft auf API-Server!
}
```

### Lösung
**Kein "local-node" mehr! Nur Cloud Nodes!**

---

### Schritt 2.1: Conductor anpassen

**Datei:** `internal/conductor/conductor.go`

```go
func NewConductor(checkInterval time.Duration, sshKeyPath string) *Conductor {
    cond := &Conductor{
        nodeRegistry: NewNodeRegistry(),
        // ... rest of initialization ...
    }

    // ❌ REMOVED: Register local-node
    // cond.nodeRegistry.RegisterNode("local-node", "localhost", 16384, 8)

    // ✅ Only cloud nodes will be registered via Auto-Scaling!

    return cond
}
```

**Test:**
```bash
# Check conductor status - should show NO nodes initially
curl -H "Authorization: Bearer $TOKEN" \
  http://91.98.202.235:8000/api/conductor/nodes

# Expected: []
```

---

### Schritt 2.2: MinecraftService anpassen

**Datei:** `internal/service/minecraft_service.go`

```go
// ❌ REMOVED: isLocalNode() helper - no longer needed

// StartServer now REQUIRES a cloud node
func (s *MinecraftService) StartServer(serverID string) error {
    // ... existing code ...

    // MULTI-NODE: Select cloud node
    nodeID, err := s.conductor.SelectNodeForContainerAuto(server.RAMMb)
    if err != nil {
        // No nodes available - trigger Auto-Scaling!
        log.Printf("NODE_SELECTION: No nodes available, triggering Auto-Scaling...")

        // Server goes to queue, will auto-start when node is ready
        s.conductor.EnqueueServer(server.ID, server.Name, server.RAMMb, server.OwnerID)

        return fmt.Errorf("no nodes available - provisioning cloud node (server queued for auto-start)")
    }

    selectedNodeID = nodeID

    // ✅ ALWAYS use Remote Docker Client (no local fallback!)
    remoteNode, err := s.conductor.GetRemoteNode(selectedNodeID)
    if err != nil {
        return fmt.Errorf("failed to get remote node: %w", err)
    }

    // Build container config
    containerName := fmt.Sprintf("mc-%s", server.ID)
    imageName := docker.GetDockerImageName(string(server.ServerType))
    env := docker.BuildContainerEnv(server)
    portBindings := docker.BuildPortBindings(server.Port)
    binds := docker.BuildVolumeBinds(server.ID, "/minecraft/servers")

    // Create and start container on remote node
    ctx := context.Background()
    containerID, err := s.conductor.GetRemoteDockerClient().StartContainer(
        ctx,
        remoteNode,
        containerName,
        imageName,
        env,
        portBindings,
        binds,
        server.RAMMb,
    )

    // ... rest of code ...
}
```

**Same for `StartServerFromQueue()`** - remove `isLocalNode()` check.

---

### Schritt 2.3: Testing - Cloud-Only

```bash
# 1. PREREQUISITE: Auto-Scaling MUSS aktiviert sein!
# In docker-compose.prod.yml:
HETZNER_CLOUD_TOKEN=<your_token>
SCALING_ENABLED=true

# 2. API neu starten
ssh root@91.98.202.235
cd /root/PayPerPlayHosting
docker compose -f docker-compose.prod.yml up -d payperplay

# 3. Prüfe Conductor Status
curl -H "Authorization: Bearer $TOKEN" \
  http://91.98.202.235:8000/api/conductor/nodes

# Expected: [] (no nodes initially)

# 4. Server erstellen & starten
curl -X POST http://91.98.202.235:8000/api/servers/{SERVER_ID}/start \
  -H "Authorization: Bearer $TOKEN"

# Expected Response:
# {
#   "error": "no nodes available - provisioning cloud node (server queued for auto-start)"
# }

# 5. Warte 2 Minuten für Cloud Node Provisioning
sleep 120

# 6. Prüfe Nodes
curl -H "Authorization: Bearer $TOKEN" \
  http://91.98.202.235:8000/api/conductor/nodes

# Expected: [{"node_id":"cloud-abc123","ip_address":"95.217.yyy.yyy",...}]

# 7. Server sollte jetzt automatisch starten (Queue Processing)
curl -H "Authorization: Bearer $TOKEN" \
  http://91.98.202.235:8000/api/servers/{SERVER_ID}

# Expected: {"status":"running","node_id":"cloud-abc123"}

# 8. SSH zu Cloud Node und prüfe Container
ssh root@95.217.yyy.yyy "docker ps"

# Expected: mc-{SERVER_ID} container running
```

---

## Phase 3: Control Plane Migration (Optional, 1-2 Tage)

**Aktuell:** API läuft auf Dedicated Server (70€/month)
**Ziel:** API auf Hetzner Cloud CX11 (3.50€/month)

### Schritte

```bash
# 1. CX11 erstellen
hcloud server create \
  --name payperplay-control \
  --type cx11 \
  --image ubuntu-22.04 \
  --ssh-key payperplay-main \
  --location nbg1

# 2. Docker installieren
ssh root@<control-plane-ip>
curl -fsSL https://get.docker.com | sh

# 3. Code deployen
git clone https://github.com/your-repo/PayPerPlayHosting.git
cd PayPerPlayHosting
cp .env.example .env

# 4. Anpassen: .env
VELOCITY_API_URL=http://95.217.xxx.xxx:8080
HETZNER_CLOUD_TOKEN=<token>
SCALING_ENABLED=true

# 5. Build & Start
docker compose -f docker-compose.prod.yml up -d

# 6. DNS Update
# play.payperplay.host → <control-plane-ip>

# 7. Test
curl http://<control-plane-ip>:8000/health

# 8. Dedicated Server kündigen (nach erfolgreichem Test)
```

---

## Zusammenfassung

### Kosten-Vergleich

| Setup | Kosten |
|-------|--------|
| **Aktuell (Hybrid-Monolith)** | 70€/month (Dedicated) |
| **Nach Phase 1+2 (3-Tier auf Dedicated)** | 70€ + 3.50€ = 73.50€/month |
| **Nach Phase 3 (Echtes 3-Tier)** | 3.50€ + 3.50€ = **7€/month** |

**Einsparung nach Phase 3:** 63€/month (90%)

### Timeline

| Phase | Zeitaufwand | Priorität |
|-------|-------------|-----------|
| Phase 1: Velocity Isolation | 3-4 Tage | 🔴 HOCH |
| Phase 2: local-node Elimination | 1-2 Tage | 🔴 HOCH |
| Phase 3: Control Plane Migration | 1-2 Tage | 🟡 MITTEL |

**Total: 5-8 Tage** für echtes 3-Tier

---

## Troubleshooting

### Problem: Velocity Registrierung schlägt fehl

```bash
# Check Velocity API erreichbar
curl http://95.217.xxx.xxx:8080/api/servers

# Check Firewall
ssh root@95.217.xxx.xxx "ufw status"
# Falls aktiv: ufw allow 8080

# Check Plugin geladen
ssh root@95.217.xxx.xxx "docker logs velocity-proxy | grep RemoteAPI"
# Expected: "VelocityRemoteAPI started on port 8080"
```

### Problem: Keine Cloud Nodes werden erstellt

```bash
# Check Auto-Scaling aktiviert
docker logs payperplay-api | grep "Scaling engine initialized"

# Check Hetzner Token
echo $HETZNER_CLOUD_TOKEN

# Manual Test
hcloud server list
```

---

## Nächste Schritte

**JETZT:**
1. ✅ Lese diesen Guide vollständig
2. ✅ Erstelle Velocity-VM (Schritt 1.1)
3. ✅ Installiere Docker & Velocity (Schritt 1.2)
4. ✅ Entwickle Remote API Plugin (Schritt 1.3)

**Fragen? Siehe [3_TIER_ARCHITECTURE.md](3_TIER_ARCHITECTURE.md) für weitere Details!**
