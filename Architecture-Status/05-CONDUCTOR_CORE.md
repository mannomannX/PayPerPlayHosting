# Conductor Core - Fleet Orchestration Engine

**Verzeichnis:** [internal/conductor/](../internal/conductor/)
**Dateien:** 13 Core-Dateien
**Zweck:** Zentrale Fleet-Orchestrierung, Auto-Scaling, Resource Management
**Pattern:** Central Orchestrator + Policy-Based Scaling

## Übersicht

Der **Conductor** ist das **Herzstück** von PayPerPlay. Er orchestriert die gesamte Server-Fleet, managed Kapazität, führt Auto-Scaling durch und koordiniert Container-Placement über mehrere Nodes hinweg.

**Architektur-Position:**
```
User Request → API → MinecraftService → CONDUCTOR → Docker/Cloud
                                            ↓
                                   Node Registry (Fleet State)
                                   Container Registry (Container Tracking)
                                   Start Queue (Capacity Waiting)
                                   Scaling Engine (Auto-Scaling)
                                   Health Checker (Node Monitoring)
```

## Core-Komponenten (13 Dateien)

### 1. conductor.go - Central Orchestrator

**Haupt-Struktur:**
```go
type Conductor struct {
    NodeRegistry      *NodeRegistry
    ContainerRegistry *ContainerRegistry
    HealthChecker     *HealthChecker
    NodeSelector      *NodeSelector
    ScalingEngine     *ScalingEngine
    RemoteClient      *docker.RemoteDockerClient
    CloudProvider     cloud.CloudProvider
    StartQueue        *StartQueue
    DebugLogBuffer    *DebugLogBuffer
    StartedAt         time.Time
    serverStarter     ServerStarter  // Interface injection!
    stopChan          chan struct{}
}
```

**Lifecycle:**
1. **NewConductor()** - Initialisierung (Zeile 42-83)
2. **InitializeScaling()** - Scaling Engine Setup (Zeile 85-105)
3. **Start()** - Bootstrap & Worker-Start (Zeile 107-151)
4. **Stop()** - Graceful Shutdown (Zeile 392-408)

**Bootstrap-Sequenz (Start-Methode):**
```go
func (c *Conductor) Start() {
    1. c.HealthChecker.Start()                 // Health monitoring
    2. c.bootstrapLocalNode()                  // Register localhost
    3. c.bootstrapProxyNode()                  // Register proxy (optional)
    4. c.ScalingEngine.Start()                 // Auto-scaling
    5. go c.startupDelayWorker()               // 2-min startup delay
    6. go c.periodicQueueWorker()              // Queue processor (30s)
    7. go c.reservationTimeoutWorker()         // Reservation cleanup (5min)
    8. go c.cpuMetricsWorker()                 // CPU monitoring (60s)
}
```

**🔥 CRITICAL: State Recovery**

**SyncRunningContainers() - Zeile 153-290:**
```go
// CRITICAL: This prevents OOM crashes after restarts by detecting existing containers
// Called on startup to recover state after crashes/restarts/deployments
func (c *Conductor) SyncRunningContainers(dockerSvc, serverRepo) {
    // Uses REFLECTION to avoid circular dependency!
    dockerVal := reflect.ValueOf(dockerSvc)
    listMethod := dockerVal.MethodByName("ListRunningMinecraftContainers")

    // For each running container:
    1. Get server from DB
    2. Extract RAM allocation
    3. Force-allocate RAM in NodeRegistry
    4. Register in ContainerRegistry  // ⚠️ CRITICAL for HealthChecker!
}
```

**⚠️ REFLECTION USAGE:**
- **Warum?** Avoid circular dependency (Conductor → MinecraftService → Conductor)
- **Risiko:** Runtime errors wenn Method-Namen ändern
- **Fragil:** Kein Compile-Time Safety

**SyncQueuedServers() - Zeile 292-390:**
```go
// CRITICAL: Prevents queue loss after container restart
// Ensures Worker-Nodes aren't decommissioned prematurely
func (c *Conductor) SyncQueuedServers(serverRepo, triggerScaling bool) {
    // Query DB for status="queued" servers
    // Re-enqueue all into StartQueue
    // Optionally trigger scaling check
}
```

**Background Workers (4):**

1. **startupDelayWorker** - 2-Minuten-Delay
   ```go
   // Wait 2 minutes before processing queue (Cloud-Init time!)
   time.Sleep(2 * time.Minute)
   logger.Info("Startup delay expired, triggering queue check")
   c.processStartQueue()
   ```

2. **periodicQueueWorker** - 30-Sekunden-Failsafe
   ```go
   // Periodic queue check (every 30s) as failsafe
   ticker := time.NewTicker(30 * time.Second)
   for { c.processStartQueue() }
   ```

3. **reservationTimeoutWorker** - 5-Minuten-Cleanup
   ```go
   // Clean up stale reservations (timeout: 30 minutes)
   ticker := time.NewTicker(5 * time.Minute)
   ```

4. **cpuMetricsWorker** - 60-Sekunden-CPU-Tracking
   ```go
   // Update CPU metrics for all nodes
   ticker := time.NewTicker(60 * time.Second)
   ```

### 2. node_registry.go - Fleet State Management

**Thread-Safe Node Registry:**
```go
type NodeRegistry struct {
    nodes map[string]*Node
    mu    sync.RWMutex
}
```

**Key Operations:**
```go
// Registration
func (r *NodeRegistry) RegisterNode(node *Node)

// Queries
func (r *NodeRegistry) GetNode(nodeID string) (*Node, bool)
func (r *NodeRegistry) GetAllNodes() []*Node
func (r *NodeRegistry) GetHealthyNodes() []*Node
func (r *NodeRegistry) GetNodesByType(nodeType string) []*Node

// Resource Management
func (r *NodeRegistry) AtomicAllocateRAMOnNode(nodeID, ramMB) bool
func (r *NodeRegistry) FreeRAMOnNode(nodeID, ramMB)

// Status Updates
func (r *NodeRegistry) UpdateNodeStatus(nodeID, status)
func (r *NodeRegistry) UpdateNodeCPU(nodeID, cpuUsagePercent)
```

**🔥 CRITICAL: AtomicAllocateRAMOnNode (Zeile 154-215)**

```go
func (r *NodeRegistry) AtomicAllocateRAMOnNode(nodeID string, ramMB int) bool {
    r.mu.Lock()
    defer r.mu.Unlock()

    node := r.nodes[nodeID]
    usableRAM := node.UsableRAMMB()  // Total - SystemReserve
    availableRAM := usableRAM - node.AllocatedRAMMB

    if availableRAM < ramMB {
        logger.Info("REJECTED - Insufficient capacity")
        return false
    }

    // Atomically allocate
    node.AllocatedRAMMB += ramMB
    node.ContainerCount++

    logger.Info("SUCCESS")
    return true
}
```

**Warum Atomic?**
- **Race Condition Prevention:** Ohne Lock könnten 2 Server gleichzeitig gleichen RAM bekommen
- **OOM Protection:** Verhindert Over-Allocation
- **Critical Section:** Mutex schützt Read-Modify-Write

**System Node Detection (Zeile 38-47):**
```go
func isSystemNodeByID(nodeID string) bool {
    return nodeID == "local-node" ||
           nodeID == "control-plane" ||
           nodeID == "proxy-node" ||
           (len(nodeID) >= 5 && nodeID[:5] == "local") ||
           (len(nodeID) >= 7 && nodeID[:7] == "control") ||
           (len(nodeID) >= 5 && nodeID[:5] == "proxy")
}
```

**🟡 ISSUE: String-Prefix-Matching ist fragil**
- Was wenn Worker-Node "proxyman-1" heißt? → Wird als System-Node erkannt!
- Besserer Ansatz: Explicit IsSystemNode Flag bei Node-Erstellung

### 3. node.go - Node Model

**Node Struct (40 Felder!):**
```go
type Node struct {
    ID                  string
    Hostname            string
    IPAddress           string
    Type                string  // dedicated, cloud, local, spare
    IsSystemNode        bool    // API/Proxy nodes - cannot run MC

    TotalRAMMB          int
    TotalCPUCores       int
    CPUUsagePercent     float64
    Status              NodeStatus  // healthy, unhealthy, unknown
    LastHealthCheck     time.Time

    ContainerCount      int
    AllocatedRAMMB      int
    SystemReservedRAMMB int

    DockerSocketPath    string
    SSHUser             string
    SSHPort             int
    SSHKeyPath          string

    CreatedAt             time.Time
    LastContainerAdded    time.Time
    LastContainerRemoved  time.Time

    Labels                map[string]string
    HourlyCostEUR         float64
    CloudProviderID       string  // Hetzner server ID
}
```

**Resource Calculation Methods:**

```go
// Usable RAM = Total - System Reserve
func (n *Node) UsableRAMMB() int {
    return n.TotalRAMMB - n.SystemReservedRAMMB
}

// Available RAM = Usable - Allocated
func (n *Node) AvailableRAMMB() int {
    usable := n.UsableRAMMB()
    available := usable - n.AllocatedRAMMB
    return max(0, available)
}

// Utilization based on USABLE (not total!)
func (n *Node) RAMUtilizationPercent() float64 {
    return (float64(n.AllocatedRAMMB) / float64(n.UsableRAMMB())) * 100
}
```

**🔥 3-Tier System Reserve Strategy (Zeile 116-163):**

```go
func (n *Node) CalculateSystemReserve(baseReserveMB, reservePercent) int {
    const smallNodeThreshold = 8192  // 8GB

    if n.TotalRAMMB < 8GB {
        // Small/Dedicated: Fixed base + dynamic
        reserve = baseReserveMB
        reserve += (n.ContainerCount / 10) * 50MB  // +50MB per 10 containers
    } else {
        // Large/Cloud: Percentage-based
        reserve = max(baseReserveMB, n.TotalRAMMB * (reservePercent / 100))
    }

    // Safety: Reserve cannot exceed 50% of total
    return min(reserve, n.TotalRAMMB / 2)
}
```

**Reserve-Strategie:**
- **Dedicated (< 8GB):** 1000MB base + 50MB per 10 containers
- **Cloud (>= 8GB):** 15% of total RAM (minimum 1000MB)
- **Safety Cap:** Max 50% of total RAM

**Consolidation Eligibility:**
```go
func (n *Node) CanBeConsolidated(minUptime, minIdleTime) bool {
    if n.IsSystemNode { return false }
    if !n.IsEmpty() { return false }
    if n.UptimeDuration() < minUptime { return false }  // min 30min alive
    if n.IdleDuration() < minIdleTime { return false }  // min 15min idle
    return true
}
```

### 4. start_queue.go - Capacity Wait Queue

**Simple FIFO Queue:**
```go
type StartQueue struct {
    queue []*QueuedServer
    mu    sync.RWMutex
}

type QueuedServer struct {
    ServerID      string
    ServerName    string
    RequiredRAMMB int
    QueuedAt      time.Time
    UserID        string
}
```

**Operations:**
```go
func (q *StartQueue) Enqueue(server *QueuedServer)
func (q *StartQueue) Dequeue() *QueuedServer  // FIFO
func (q *StartQueue) Peek() *QueuedServer
func (q *StartQueue) Remove(serverID string) bool
func (q *StartQueue) GetPosition(serverID string) int  // 1-based
func (q *StartQueue) Size() int
```

**Duplicate Prevention (Zeile 38-46):**
```go
// Check if server is already queued
for _, s := range q.queue {
    if s.ServerID == server.ServerID {
        logger.Warn("Server already in queue, skipping")
        return
    }
}
```

**Event Publishing:**
- `PublishServerQueued` - Server added to queue
- `PublishServerDequeued` - Server removed from queue
- `PublishQueueUpdated` - Queue state changed

### 5. scaling_engine.go - Auto-Scaling Orchestrator

**Policy-Based Architecture:**
```go
type ScalingEngine struct {
    policies       []ScalingPolicy  // Sorted by priority!
    cloudProvider  cloud.CloudProvider
    vmProvisioner  *VMProvisioner
    nodeRegistry   *NodeRegistry
    startQueue     *StartQueue
    conductor      *Conductor  // Back-reference
    velocityClient interface{}
    enabled        bool
    checkInterval  time.Duration  // 2 minutes
    stopChan       chan struct{}
}
```

**Registered Policies:**
1. **ReactivePolicy** - Capacity-based (IMPLEMENTED)
2. **SparePoolPolicy** - Buffer maintenance (TODO B6)
3. **PredictivePolicy** - Time-series forecast (TODO B7)
4. **ConsolidationPolicy** - Cost optimization (IMPLEMENTED B8)

**Policy Registration (Zeile 50-62):**
```go
engine.RegisterPolicy(NewReactivePolicy(cloudProvider))
// TODO B6: engine.RegisterPolicy(NewSparePoolPolicy())
// TODO B7: engine.RegisterPolicy(NewPredictivePolicy())

if velocityClient != nil {
    engine.RegisterPolicy(NewConsolidationPolicy(velocityClient))
}

// Sort by priority (highest first)
sort.Slice(e.policies, func(i, j int) bool {
    return e.policies[i].Priority() > e.policies[j].Priority()
})
```

**Evaluation Loop (Zeile 144-161):**
```go
func (e *ScalingEngine) runLoop() {
    ticker := time.NewTicker(2 * time.Minute)

    e.evaluateScaling()  // Run immediately

    for {
        select {
        case <-ticker.C:
            e.evaluateScaling()
        case <-e.stopChan:
            return
        }
    }
}
```

**Scaling Decision Process (Zeile 164-252):**
```go
func (e *ScalingEngine) evaluateScaling() {
    ctx := e.buildScalingContext()  // Current fleet state

    // Ask all policies (priority order): Should we scale UP?
    for _, policy := range e.policies {
        if shouldScale, recommendation := policy.ShouldScaleUp(ctx); shouldScale {
            logger.Info("Scale UP decision", recommendation)
            e.executeScaleUp(recommendation)
            return  // One action per cycle
        }
    }

    // Ask all policies: Should we scale DOWN?
    for _, policy := range e.policies {
        if shouldScale, recommendation := policy.ShouldScaleDown(ctx); shouldScale {
            logger.Info("Scale DOWN decision", recommendation)
            e.executeScaleDown(recommendation)
            return
        }
    }
}
```

**🟢 GOOD: Policy Pattern**
- Pluggable scaling strategies
- Priority-based execution
- One action per cycle (prevent thrashing)

### 6. policy_reactive.go - Capacity-Based Scaling

**Reactive Thresholds:**
```go
const (
    ScaleUpThreshold   = 85.0  // >85% capacity → provision VM
    ScaleDownThreshold = 30.0  // <30% capacity for 30min → decommission
    ScaleDownCooldown  = 30 * time.Minute
    MaxCloudNodes      = 10    // Safety limit
)
```

**Scale UP Logic (Zeile 50-120):**
```go
func (p *ReactivePolicy) ShouldScaleUp(ctx *ScalingContext) (bool, *ScalingRecommendation) {
    // Calculate QUEUE-aware capacity
    totalRAM := ctx.FleetStats.TotalRAMMB
    allocatedRAM := ctx.FleetStats.AllocatedRAMMB
    queueRAM := ctx.QueueStats.TotalRequiredRAM

    effectiveAllocated := allocatedRAM + queueRAM
    capacityPercent := (effectiveAllocated / totalRAM) * 100

    if capacityPercent > ScaleUpThreshold {
        // Determine VM size based on queue size
        vmSize := determineVMSize(queueRAM)

        return true, &ScalingRecommendation{
            Action: "scale_up",
            ServerType: vmSize,  // cpx22, cpx32, cpx42, cpx62
            Count: 1,
            Reason: fmt.Sprintf("Capacity %.1f%% exceeds threshold %.1f%%",
                                capacityPercent, ScaleUpThreshold),
            Urgency: calculateUrgency(capacityPercent),
        }
    }

    return false, nil
}
```

**🔥 QUEUE-AWARE CAPACITY:**
- Considers queued servers as "virtual allocation"
- Prevents scaling delay (don't wait for actual allocation)
- Formula: `EffectiveCapacity = Allocated + QueuedRAM`

**Scale DOWN Logic (Zeile 122-200):**
```go
func (p *ReactivePolicy) ShouldScaleDown(ctx *ScalingContext) (bool, *ScalingRecommendation) {
    cloudNodes := ctx.CloudNodes
    if len(cloudNodes) == 0 {
        return false, nil  // No cloud nodes to remove
    }

    capacityPercent := (ctx.FleetStats.AllocatedRAMMB / ctx.FleetStats.TotalRAMMB) * 100

    if capacityPercent < ScaleDownThreshold {
        // Check cooldown (must be <30% for 30 minutes)
        if time.Since(p.lastScaleDown) < ScaleDownCooldown {
            return false, nil
        }

        // Find empty cloud node
        for _, node := range cloudNodes {
            if node.IsEmpty() && node.IdleDuration() > 15*time.Minute {
                return true, &ScalingRecommendation{
                    Action: "scale_down",
                    NodeID: node.ID,
                    Reason: fmt.Sprintf("Capacity %.1f%% below threshold %.1f%% for 30min",
                                        capacityPercent, ScaleDownThreshold),
                }
            }
        }
    }

    return false, nil
}
```

**Safety Mechanisms:**
- 30-minute cooldown after scale-down
- Only remove EMPTY nodes
- Minimum 15-minute idle time
- Max 10 cloud nodes (hard limit)

### 7. policy_consolidation.go - Cost Optimization (B8)

**Container Migration for Cost Reduction:**

**Consolidation Conditions (Zeile 60-150):**
```go
func (p *ConsolidationPolicy) ShouldScaleDown(ctx *ScalingContext) (bool, *ScalingRecommendation) {
    // Safety: System must be stable
    if time.Since(ctx.LastScalingAction) < 2*time.Hour {
        return false, nil  // Too soon after last action
    }

    // Find empty cloud nodes that can be decommissioned
    candidateNodes := findEmptyCloudNodes(ctx.CloudNodes)

    // Calculate potential savings
    for _, node := range candidateNodes {
        if node.HourlyCostEUR > 0.10 && node.CanBeConsolidated(30*time.Minute, 15*time.Minute) {
            return true, &ScalingRecommendation{
                Action: "consolidate",
                NodeID: node.ID,
                Reason: fmt.Sprintf("Node empty, potential savings €%.2f/hour", node.HourlyCostEUR),
            }
        }
    }

    return false, nil
}
```

**Migration Requirements:**
- System stable (2h cooldown after scaling)
- Cost savings >€0.10/hour
- Tier-aware (only small/medium servers, 2-8GB)
- Plan-aware (never migrate reserved plans)
- Minimum 30min uptime
- Minimum 15min idle time

**Velocity Integration:**
- Checks player count before migration
- Only migrates offline servers
- Uses Velocity API for server status

### 8. vm_provisioner.go - Hetzner Cloud API

**VM Provisioning Workflow:**

```go
type VMProvisioner struct {
    cloudProvider  cloud.CloudProvider
    nodeRegistry   *NodeRegistry
    debugLogBuffer *DebugLogBuffer
    sshKeyName     string
}

func (p *VMProvisioner) ProvisionNode(serverType string) (*Node, error) {
    // 1. Create placeholder node (BEFORE Hetzner API!)
    placeholderNode := &Node{
        ID: fmt.Sprintf("worker-%d", time.Now().Unix()),
        Status: NodeStatusUnknown,
        Type: "cloud",
    }
    p.nodeRegistry.RegisterNode(placeholderNode)

    // 2. Call Hetzner API to create server
    hetznerServer, err := p.cloudProvider.CreateServer(serverType, p.sshKeyName)
    if err != nil {
        p.nodeRegistry.RemoveNode(placeholderNode.ID)
        return nil, err
    }

    // 3. Update placeholder with real data
    placeholderNode.IPAddress = hetznerServer.PublicIP
    placeholderNode.CloudProviderID = hetznerServer.ID
    placeholderNode.HourlyCostEUR = hetznerServer.HourlyCostEUR

    // 4. Wait for Cloud-Init to complete
    // (HealthChecker will mark as healthy when SSH succeeds)

    return placeholderNode, nil
}
```

**🔥 CRITICAL: Placeholder Pattern (Zeile 80-120)**

**Warum Placeholder BEFORE API Call?**
- **Race Condition Prevention:** Queue Processor könnte sonst doppeltes Provisioning triggern
- **State Tracking:** Node ist sofort im Registry, auch wenn Hetzner API langsam ist
- **Cleanup:** Bei Fehler wird Placeholder entfernt

**Cloud-Init Integration:**
- Server wird mit Status "unknown" registriert
- HealthChecker versucht SSH-Verbindung
- Nach Cloud-Init-Completion: Status → "healthy"
- Queue-Processor startet dann Server

### 9. health_checker.go - Node Monitoring

**Health Check Workflow:**

```go
type HealthChecker struct {
    nodeRegistry      *NodeRegistry
    containerRegistry *ContainerRegistry
    remoteClient      *docker.RemoteDockerClient
    debugLogBuffer    *DebugLogBuffer
    interval          time.Duration
    stopChan          chan struct{}
}

func (hc *HealthChecker) Start() {
    go hc.runLoop()
}

func (hc *HealthChecker) runLoop() {
    ticker := time.NewTicker(hc.interval)  // Default: 60s

    for {
        select {
        case <-ticker.C:
            hc.checkAllNodes()
        case <-hc.stopChan:
            return
        }
    }
}
```

**Node Health Check (3 Attempts):**
```go
func (hc *HealthChecker) checkNode(node *Node) {
    for attempt := 1; attempt <= 3; attempt++ {
        if hc.pingNode(node) {
            // SUCCESS
            if node.Status != NodeStatusHealthy {
                logger.Info("Node recovered", node.ID)
                hc.nodeRegistry.UpdateNodeStatus(node.ID, NodeStatusHealthy)

                // Re-register after health change! (Fix race condition)
                hc.nodeRegistry.RegisterNode(node)
            }
            return
        }

        time.Sleep(5 * time.Second)  // Wait between attempts
    }

    // FAILED after 3 attempts
    if node.Status == NodeStatusHealthy {
        logger.Warn("Node became unhealthy", node.ID)
        hc.nodeRegistry.UpdateNodeStatus(node.ID, NodeStatusUnhealthy)
    }
}
```

**SSH-Based Ping:**
```go
func (hc *HealthChecker) pingNode(node *Node) bool {
    if node.ID == "local-node" {
        return true  // Localhost always healthy
    }

    // SSH + Docker ping
    cmd := "docker ps"
    _, err := hc.remoteClient.ExecuteCommand(node.IPAddress, cmd)
    return err == nil
}
```

### 10. container_registry.go - Container Tracking

**Container State Management:**
```go
type ContainerRegistry struct {
    containers   map[string]*ContainerInfo  // containerID -> info
    nodeRegistry *NodeRegistry
    mu           sync.RWMutex
}

type ContainerInfo struct {
    ContainerID string
    ServerID    string
    NodeID      string
    RAMMb       int
    Status      string
    StartedAt   time.Time
}
```

**Lifecycle Tracking:**
```go
func (r *ContainerRegistry) RegisterContainer(info *ContainerInfo)
func (r *ContainerRegistry) UnregisterContainer(containerID string)
func (r *ContainerRegistry) GetContainerByServerID(serverID) *ContainerInfo
func (r *ContainerRegistry) GetNodeAllocation(nodeID) (containerCount, allocatedRAM)
```

**Integration mit NodeRegistry:**
- Bei Register → NodeRegistry.AllocatedRAMMB++
- Bei Unregister → NodeRegistry.AllocatedRAMMB--
- HealthChecker nutzt GetNodeAllocation() für Sanity Checks

### 11. node_selector.go - Intelligent Placement

**Node Selection Algorithm:**
```go
func (ns *NodeSelector) SelectNodeForContainer(ramMB int) (*Node, error) {
    healthyNodes := ns.nodeRegistry.GetHealthyNodes()

    // Filter: Only nodes with sufficient capacity
    candidates := []
    for _, node := range healthyNodes {
        if !node.IsSystemNode && node.AvailableRAMMB() >= ramMB {
            candidates = append(candidates, node)
        }
    }

    if len(candidates) == 0 {
        return nil, ErrNoCapacity
    }

    // Sort by:
    // 1. Node type (dedicated > cloud)
    // 2. RAM utilization (lowest first - load balancing)
    sort.Slice(candidates, func(i, j int) bool {
        if candidates[i].Type != candidates[j].Type {
            return candidates[i].Type == "dedicated"  // Prefer dedicated
        }
        return candidates[i].RAMUtilizationPercent() < candidates[j].RAMUtilizationPercent()
    })

    return candidates[0], nil
}
```

**Selection Criteria:**
1. **Health:** Node must be healthy
2. **Not System Node:** Cannot place on API/Proxy nodes
3. **Capacity:** Available RAM >= requested RAM
4. **Prefer Dedicated:** Use dedicated nodes before cloud
5. **Load Balancing:** Select least loaded node

### 12. debug_log_buffer.go - Dashboard Logging

**Ring Buffer für Debug-Logs:**
```go
type DebugLogBuffer struct {
    entries []*DebugLogEntry
    maxSize int
    mu      sync.RWMutex
}

type DebugLogEntry struct {
    Timestamp time.Time
    Level     string
    Message   string
    Data      map[string]interface{}
}

func (b *DebugLogBuffer) Add(level, message, data) {
    // Ring buffer: Oldest entries are overwritten
    if len(b.entries) >= b.maxSize {
        b.entries = b.entries[1:]  // Remove oldest
    }
    b.entries = append(b.entries, entry)
}
```

**Usage:** Dashboard WebSocket zeigt letzte 200 Events

### 13. scaling_policy.go - Policy Interface

**Policy Contract:**
```go
type ScalingPolicy interface {
    Name() string
    Priority() int  // Higher = executes first
    ShouldScaleUp(ctx *ScalingContext) (bool, *ScalingRecommendation)
    ShouldScaleDown(ctx *ScalingContext) (bool, *ScalingRecommendation)
}

type ScalingContext struct {
    FleetStats         *FleetStats
    QueueStats         *QueueStats
    DedicatedNodes     []*Node
    CloudNodes         []*Node
    LastScalingAction  time.Time
}

type ScalingRecommendation struct {
    Action     string  // scale_up, scale_down, consolidate
    ServerType string  // cpx22, cpx32, etc.
    NodeID     string  // For scale_down
    Count      int
    Reason     string
    Urgency    int  // 1-10
}
```

## Architektur-Patterns

### 1. **Central Orchestrator Pattern**
- Conductor als Single Point of Coordination
- Alle Komponenten kommunizieren über Conductor
- State in Registries (Node, Container)

### 2. **Policy-Based Scaling**
- Pluggable scaling strategies
- Priority-based execution
- One action per cycle

### 3. **Reflection for Decoupling**
- SyncRunningContainers nutzt Reflection
- Vermeidet circular dependency
- Fragil, aber funktional

### 4. **Placeholder Pattern**
- Register node BEFORE cloud API call
- Prevents race conditions in provisioning
- Cleanup bei Fehler

### 5. **Ring Buffer Logging**
- DebugLogBuffer für Dashboard
- Fixed size (200 entries)
- Oldest overwritten

## Code-Flaws & Potenzielle Probleme

### 🔴 CRITICAL

1. **Reflection Usage in State Sync** (conductor.go:158-290)
   - Runtime errors möglich
   - Kein Compile-Time Safety
   - Method names hardcoded: "ListRunningMinecraftContainers", "FindByID", "GetRAMMb"
   - **Risiko:** Refactoring bricht Sync

2. **System Node Detection via String-Prefix** (node_registry.go:40-47)
   ```go
   nodeID == "local-node" ||
   (len(nodeID) >= 5 && nodeID[:5] == "local")  // ⚠️ Fragil!
   ```
   - Worker-Node "proxyman-1" wird als System-Node erkannt
   - Besserer Ansatz: Explicit Flag

### 🟡 MEDIUM

3. **Hardcoded Constants**
   - ScaleUpThreshold = 85.0
   - ScaleDownThreshold = 30.0
   - MaxCloudNodes = 10
   - Startup Delay = 2 minutes
   - Sollte config-basiert sein

4. **Keine Error Recovery in Workers**
   - 4 Background-Goroutines ohne Panic-Recovery
   - Wenn einer crasht → keine automatische Restart

5. **Queue Processor Race Condition Potential**
   - startupDelayWorker + periodicQueueWorker laufen parallel
   - Beide rufen processStartQueue()
   - Potenzielle doppelte Verarbeitung?

6. **No Metrics Export**
   - Scaling decisions nicht als Prometheus-Metrics
   - Schwer zu monitoren ohne Dashboard

### 🟢 LOW

7. **Magic Numbers**
   - 2-minute startup delay (warum 2?)
   - 30-second queue check (warum 30?)
   - 50MB per 10 containers (warum 50?)

8. **TODO Comments in Production Code**
   - "TODO B6: SparePoolPolicy"
   - "TODO B7: PredictivePolicy"
   - Sollten als GitHub Issues getrackt werden

## Performance-Überlegungen

**Positive:**
- ✅ Atomic RAM allocation (Mutex)
- ✅ Read-Write-Locks für Registries
- ✅ One scaling action per cycle (no thrashing)

**Concerns:**
- ⚠️ Reflection overhead in State Sync
- ⚠️ No connection pooling für SSH-Health-Checks
- ⚠️ Linear search in Node Selection (OK für <100 nodes)

## Integration Points

**Eingehende Abhängigkeiten:**
- `cmd/api/main.go` - Initialisierung & State Sync
- `internal/service/minecraft_service.go` - Server Start Requests
- `internal/api/conductor_handler.go` - Fleet Status API

**Ausgehende Abhängigkeiten:**
- `internal/docker` - Container Management
- `internal/cloud` - Hetzner Cloud API
- `internal/velocity` - Player Count Check
- `internal/events` - Event Publishing

## Nächste Schritte

Siehe [04-BUSINESS_LOGIC.md](04-BUSINESS_LOGIC.md) für Service-Layer-Analyse.
