# Cloud Orchestration & Scaling Architecture
**Designed for:** B5 (Reactive Scaling), B6 (Hot-Spare Pool), B7 (Predictive Scaling)
**Principle:** ONE unified system, NOT three separate implementations
**Cloud Provider:** Hetzner Cloud (optimized, no abstraction overhead)

---

## 🎯 Design Philosophy

### ❌ FALSCH: 3 separate Systeme
```
B5: reactive_scaler.go
B6: spare_pool_manager.go
B7: predictive_scaler.go
→ Code-Duplikation, inkonsistente Logic, schwer zu warten
```

### ✅ RICHTIG: 1 einheitliches System mit Policies
```
ScalingEngine
├── Policy Interface (reactive, predictive, spare-pool)
├── Cloud Provider Interface (Hetzner, AWS, etc.)
├── Provisioner (VM setup)
└── Event-driven (nutzt Event-Bus)
```

---

## 🏗️ Architektur-Übersicht

```
┌─────────────────────────────────────────────────────────────────┐
│                    CONDUCTOR (Orchestrator)                      │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              SCALING ENGINE (Unified)                     │   │
│  │  ┌────────────────────────────────────────────────────┐  │   │
│  │  │  Policy Manager (Pluggable Strategies)             │  │   │
│  │  │  ├─ ReactivePolicy      (B5) ✅ Jetzt             │  │   │
│  │  │  ├─ SparePoolPolicy     (B6) 🔜 Später            │  │   │
│  │  │  └─ PredictivePolicy    (B7) 🔜 Später            │  │   │
│  │  └────────────────────────────────────────────────────┘  │   │
│  │                                                            │   │
│  │  ┌────────────────────────────────────────────────────┐  │   │
│  │  │  Decision Engine                                    │  │   │
│  │  │  ├─ Should scale up? (combines all policies)       │  │   │
│  │  │  ├─ Should scale down?                              │  │   │
│  │  │  └─ Should provision spare?                         │  │   │
│  │  └────────────────────────────────────────────────────┘  │   │
│  │                                                            │   │
│  │  ┌────────────────────────────────────────────────────┐  │   │
│  │  │  Executor                                           │  │   │
│  │  │  ├─ Scale Up   → CloudProvider.CreateServer()      │  │   │
│  │  │  ├─ Scale Down → CloudProvider.DeleteServer()      │  │   │
│  │  │  └─ Provision  → VMProvisioner.Setup()             │  │   │
│  │  └────────────────────────────────────────────────────┘  │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                                ↓
┌─────────────────────────────────────────────────────────────────┐
│           CLOUD PROVIDER INTERFACE (Abstraction)                 │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  type CloudProvider interface {                           │   │
│  │      CreateServer(spec ServerSpec) (*Server, error)       │   │
│  │      DeleteServer(id string) error                        │   │
│  │      ListServers() ([]Server, error)                      │   │
│  │      GetServerTypes() ([]ServerType, error)               │   │
│  │      WaitForReady(id string) error                        │   │
│  │  }                                                         │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                ↓                                 │
│  ┌────────────────┐  ┌────────────────┐  ┌──────────────────┐  │
│  │ HetznerProvider│  │   AWSProvider  │  │  LocalProvider   │  │
│  │   (B5 jetzt)   │  │   (später)     │  │   (testing)      │  │
│  └────────────────┘  └────────────────┘  └──────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                                ↓
┌─────────────────────────────────────────────────────────────────┐
│                    VM PROVISIONER (Cloud-Init)                   │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  1. Generate Cloud-Init Script                            │   │
│  │     ├─ Install Docker                                     │   │
│  │     ├─ Install PayPerPlay Agent                           │   │
│  │     └─ Configure Firewall                                 │   │
│  │  2. Wait for VM Ready                                     │   │
│  │  3. Register in NodeRegistry                              │   │
│  │  4. Publish event: node.added                             │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                                ↓
┌─────────────────────────────────────────────────────────────────┐
│                    EXISTING SYSTEMS (Nutzen!)                    │
│  ┌────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │ NodeReg    │  │ Prometheus  │  │      Event-Bus          │  │
│  │ (B2 ✅)    │  │ (B1 ✅)     │  │      (B4 ✅)            │  │
│  └────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 📐 Interface Design (Provider-Agnostic)

### 1. Cloud Provider Interface

```go
// internal/cloud/provider.go

package cloud

import "time"

// CloudProvider abstraction - works with ANY cloud (Hetzner, AWS, GCP, etc.)
type CloudProvider interface {
    // Server Management
    CreateServer(spec ServerSpec) (*Server, error)
    DeleteServer(serverID string) error
    ListServers() ([]*Server, error)
    GetServer(serverID string) (*Server, error)

    // Server Types (for capacity planning)
    GetServerTypes() ([]*ServerType, error)

    // Health & Status
    WaitForReady(serverID string, timeout time.Duration) error
    GetServerStatus(serverID string) (ServerStatus, error)

    // Billing Info (für Cost-Tracking)
    GetServerCost(serverID string) (float64, error)
}

// ServerSpec - what we want to create
type ServerSpec struct {
    Name         string
    Type         string            // "cx11", "cx21", etc. (Hetzner) or "t2.micro" (AWS)
    Image        string            // "ubuntu-22.04"
    Location     string            // "nbg1", "hel1", etc.
    CloudInit    string            // Cloud-Init script
    Labels       map[string]string // für Tagging: "managed_by": "payperplay"
    SSHKeys      []string
}

// Server - what we get back
type Server struct {
    ID            string
    Name          string
    Type          string
    IPAddress     string
    PrivateIP     string
    Status        ServerStatus
    CreatedAt     time.Time
    HourlyCostEUR float64
    Labels        map[string]string
}

type ServerStatus string

const (
    ServerStatusInitializing ServerStatus = "initializing"
    ServerStatusRunning      ServerStatus = "running"
    ServerStatusStopped      ServerStatus = "stopped"
    ServerStatusDeleted      ServerStatus = "deleted"
)

// ServerType - available VM sizes
type ServerType struct {
    ID            string
    Name          string
    Cores         int
    RAMMB         int
    DiskGB        int
    HourlyCostEUR float64
    Available     bool
}
```

---

### 2. Scaling Policy Interface

```go
// internal/conductor/scaling_policy.go

package conductor

// ScalingPolicy - pluggable scaling strategies
type ScalingPolicy interface {
    // Name of the policy (for logging)
    Name() string

    // ShouldScaleUp returns true if we need more capacity
    ShouldScaleUp(ctx ScalingContext) (bool, ScaleRecommendation)

    // ShouldScaleDown returns true if we have too much capacity
    ShouldScaleDown(ctx ScalingContext) (bool, ScaleRecommendation)

    // Priority - higher priority policies are checked first
    Priority() int
}

// ScalingContext - all data a policy needs to make decisions
type ScalingContext struct {
    // Fleet Stats (from existing NodeRegistry)
    FleetStats FleetStats

    // Current Nodes
    DedicatedNodes []*Node  // Always-on base capacity
    CloudNodes     []*Node  // Dynamic capacity

    // Historical Data (from InfluxDB via Event-Bus)
    AverageRAMUsageLast1h  float64
    AverageRAMUsageLast24h float64
    PeakRAMUsageLast24h    float64

    // Time Context (for predictive policies)
    CurrentTime  time.Time
    IsWeekend    bool
    IsHoliday    bool

    // Forecast (for B7 - predictive policy)
    ForecastedRAMIn1h  float64
    ForecastedRAMIn2h  float64
}

// ScaleRecommendation - what to do
type ScaleRecommendation struct {
    Action      ScaleAction
    ServerType  string     // Which VM size to use
    Count       int        // How many VMs
    Reason      string     // For logging
    Urgency     Urgency    // How fast we need to act
}

type ScaleAction string

const (
    ScaleActionNone     ScaleAction = "none"
    ScaleActionScaleUp  ScaleAction = "scale_up"
    ScaleActionScaleDown ScaleAction = "scale_down"
    ScaleActionProvisionSpare ScaleAction = "provision_spare"
)

type Urgency string

const (
    UrgencyLow      Urgency = "low"      // Can wait 5-10 minutes
    UrgencyMedium   Urgency = "medium"   // Should act within 2 minutes
    UrgencyHigh     Urgency = "high"     // Act immediately
    UrgencyCritical Urgency = "critical" // Emergency (>95% capacity)
)
```

---

### 3. Reactive Policy (B5) - First Implementation

```go
// internal/conductor/policy_reactive.go

package conductor

import "time"

// ReactivePolicy - scales based on CURRENT capacity
type ReactivePolicy struct {
    ScaleUpThreshold   float64  // 85% capacity
    ScaleDownThreshold float64  // 30% capacity
    CooldownPeriod     time.Duration // 5 minutes
    lastScaleAction    time.Time
}

func NewReactivePolicy() *ReactivePolicy {
    return &ReactivePolicy{
        ScaleUpThreshold:   85.0,
        ScaleDownThreshold: 30.0,
        CooldownPeriod:     5 * time.Minute,
    }
}

func (p *ReactivePolicy) Name() string {
    return "reactive"
}

func (p *ReactivePolicy) Priority() int {
    return 10 // Medium priority (predictive will be 20, spare-pool will be 5)
}

func (p *ReactivePolicy) ShouldScaleUp(ctx ScalingContext) (bool, ScaleRecommendation) {
    // Cooldown check
    if time.Since(p.lastScaleAction) < p.CooldownPeriod {
        return false, ScaleRecommendation{Action: ScaleActionNone}
    }

    // Calculate capacity
    capacityPercent := (ctx.FleetStats.AllocatedRAMMB / ctx.FleetStats.TotalRAMMB) * 100

    if capacityPercent > p.ScaleUpThreshold {
        urgency := p.calculateUrgency(capacityPercent)

        return true, ScaleRecommendation{
            Action:     ScaleActionScaleUp,
            ServerType: p.selectServerType(ctx),
            Count:      1,
            Reason:     fmt.Sprintf("Capacity at %.1f%% (threshold: %.1f%%)", capacityPercent, p.ScaleUpThreshold),
            Urgency:    urgency,
        }
    }

    return false, ScaleRecommendation{Action: ScaleActionNone}
}

func (p *ReactivePolicy) ShouldScaleDown(ctx ScalingContext) (bool, ScaleRecommendation) {
    // Only scale down if we have cloud nodes
    if len(ctx.CloudNodes) == 0 {
        return false, ScaleRecommendation{Action: ScaleActionNone}
    }

    // Cooldown check
    if time.Since(p.lastScaleAction) < p.CooldownPeriod {
        return false, ScaleRecommendation{Action: ScaleActionNone}
    }

    capacityPercent := (ctx.FleetStats.AllocatedRAMMB / ctx.FleetStats.TotalRAMMB) * 100

    // Check if we've been below threshold for 20 minutes (anti-flapping)
    if capacityPercent < p.ScaleDownThreshold {
        // TODO: Check if below threshold for 20 minutes
        return true, ScaleRecommendation{
            Action:     ScaleActionScaleDown,
            Count:      1,
            Reason:     fmt.Sprintf("Capacity at %.1f%% (threshold: %.1f%%)", capacityPercent, p.ScaleDownThreshold),
            Urgency:    UrgencyLow,
        }
    }

    return false, ScaleRecommendation{Action: ScaleActionNone}
}

func (p *ReactivePolicy) calculateUrgency(capacityPercent float64) Urgency {
    if capacityPercent > 95 {
        return UrgencyCritical
    } else if capacityPercent > 90 {
        return UrgencyHigh
    } else if capacityPercent > 85 {
        return UrgencyMedium
    }
    return UrgencyLow
}

func (p *ReactivePolicy) selectServerType(ctx ScalingContext) string {
    // Start with small VMs (Hetzner CX21: 2 vCPU, 4GB RAM, ~5€/month)
    return "cx21"
}
```

---

### 4. Spare Pool Policy (B6) - Prepared for Later

```go
// internal/conductor/policy_spare_pool.go

package conductor

// SparePoolPolicy - keeps warm VMs ready for instant provisioning
type SparePoolPolicy struct {
    MinSpares     int  // Minimum spare VMs to keep
    MaxSpares     int  // Maximum spare VMs
    TimeBasedSize bool // Adjust pool size based on time of day
}

func (p *SparePoolPolicy) Name() string {
    return "spare-pool"
}

func (p *SparePoolPolicy) Priority() int {
    return 5 // Lower priority than reactive/predictive
}

func (p *SparePoolPolicy) ShouldScaleUp(ctx ScalingContext) (bool, ScaleRecommendation) {
    spareCount := p.countSpareNodes(ctx.CloudNodes)

    targetSpares := p.calculateTargetSpares(ctx)

    if spareCount < targetSpares {
        return true, ScaleRecommendation{
            Action:     ScaleActionProvisionSpare,
            ServerType: "cx11", // Smallest VM for spares
            Count:      targetSpares - spareCount,
            Reason:     "Spare pool below target",
            Urgency:    UrgencyLow,
        }
    }

    return false, ScaleRecommendation{Action: ScaleActionNone}
}

func (p *SparePoolPolicy) calculateTargetSpares(ctx ScalingContext) int {
    if !p.TimeBasedSize {
        return p.MinSpares
    }

    // Friday/Saturday: 3 spares
    if ctx.CurrentTime.Weekday() == time.Friday || ctx.CurrentTime.Weekday() == time.Saturday {
        return 3
    }

    // Weekdays: 1 spare
    return 1
}
```

---

### 5. Predictive Policy (B7) - Prepared for Later

```go
// internal/conductor/policy_predictive.go

package conductor

// PredictivePolicy - scales based on FORECASTED demand
type PredictivePolicy struct {
    ForecastHorizon time.Duration // How far ahead to look (2 hours)
    ProvisionTime   time.Duration // How long VM provisioning takes (3 min)
}

func (p *PredictivePolicy) Name() string {
    return "predictive"
}

func (p *PredictivePolicy) Priority() int {
    return 20 // Highest priority - acts BEFORE reactive
}

func (p *PredictivePolicy) ShouldScaleUp(ctx ScalingContext) (bool, ScaleRecommendation) {
    // Only act if we have forecast data
    if ctx.ForecastedRAMIn2h == 0 {
        return false, ScaleRecommendation{Action: ScaleActionNone}
    }

    // Calculate forecasted capacity
    forecastedCapacity := (ctx.ForecastedRAMIn2h / ctx.FleetStats.TotalRAMMB) * 100

    // If forecast shows we'll need more capacity, provision NOW
    if forecastedCapacity > 80 {
        return true, ScaleRecommendation{
            Action:     ScaleActionScaleUp,
            ServerType: "cx21",
            Count:      1,
            Reason:     fmt.Sprintf("Forecasted capacity in 2h: %.1f%%", forecastedCapacity),
            Urgency:    UrgencyMedium,
        }
    }

    return false, ScaleRecommendation{Action: ScaleActionNone}
}
```

---

## 🎯 Scaling Engine (Unified Decision Maker)

```go
// internal/conductor/scaling_engine.go

package conductor

import (
    "time"
    "github.com/payperplay/hosting/pkg/logger"
    "github.com/payperplay/hosting/internal/events"
)

// ScalingEngine - unified scaling orchestrator
type ScalingEngine struct {
    policies       []ScalingPolicy
    cloudProvider  cloud.CloudProvider
    vmProvisioner  *VMProvisioner
    nodeRegistry   *NodeRegistry
    enabled        bool
    checkInterval  time.Duration
    stopChan       chan struct{}
}

func NewScalingEngine(
    cloudProvider cloud.CloudProvider,
    vmProvisioner *VMProvisioner,
    nodeRegistry *NodeRegistry,
) *ScalingEngine {
    engine := &ScalingEngine{
        policies:      []ScalingPolicy{},
        cloudProvider: cloudProvider,
        vmProvisioner: vmProvisioner,
        nodeRegistry:  nodeRegistry,
        enabled:       true,
        checkInterval: 2 * time.Minute,
        stopChan:      make(chan struct{}),
    }

    // Register policies (order matters - sorted by priority)
    engine.RegisterPolicy(NewReactivePolicy())
    // engine.RegisterPolicy(NewSparePoolPolicy())     // B6 later
    // engine.RegisterPolicy(NewPredictivePolicy())    // B7 later

    return engine
}

func (e *ScalingEngine) RegisterPolicy(policy ScalingPolicy) {
    e.policies = append(e.policies, policy)
    // Sort by priority (highest first)
    sort.Slice(e.policies, func(i, j int) bool {
        return e.policies[i].Priority() > e.policies[j].Priority()
    })
}

func (e *ScalingEngine) Start() {
    logger.Info("ScalingEngine started", map[string]interface{}{
        "check_interval": e.checkInterval.String(),
        "policies":       len(e.policies),
    })

    go e.runLoop()
}

func (e *ScalingEngine) Stop() {
    close(e.stopChan)
}

func (e *ScalingEngine) runLoop() {
    ticker := time.NewTicker(e.checkInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            e.evaluateScaling()
        case <-e.stopChan:
            return
        }
    }
}

func (e *ScalingEngine) evaluateScaling() {
    if !e.enabled {
        return
    }

    // Build context from current state
    ctx := e.buildScalingContext()

    // Ask ALL policies for their recommendation (in priority order)
    for _, policy := range e.policies {
        // Check scale UP
        if shouldScale, recommendation := policy.ShouldScaleUp(ctx); shouldScale {
            logger.Info("Scaling decision", map[string]interface{}{
                "policy":    policy.Name(),
                "action":    recommendation.Action,
                "reason":    recommendation.Reason,
                "urgency":   recommendation.Urgency,
            })

            e.executeScaling(recommendation)
            return // Only execute ONE action per cycle
        }

        // Check scale DOWN
        if shouldScale, recommendation := policy.ShouldScaleDown(ctx); shouldScale {
            logger.Info("Scaling decision", map[string]interface{}{
                "policy":    policy.Name(),
                "action":    recommendation.Action,
                "reason":    recommendation.Reason,
            })

            e.executeScaling(recommendation)
            return
        }
    }

    logger.Debug("No scaling action needed", nil)
}

func (e *ScalingEngine) buildScalingContext() ScalingContext {
    stats := e.nodeRegistry.GetFleetStats()
    nodes := e.nodeRegistry.GetAllNodes()

    var dedicatedNodes, cloudNodes []*Node
    for _, node := range nodes {
        if node.Type == "dedicated" {
            dedicatedNodes = append(dedicatedNodes, node)
        } else if node.Type == "cloud" {
            cloudNodes = append(cloudNodes, node)
        }
    }

    return ScalingContext{
        FleetStats:     stats,
        DedicatedNodes: dedicatedNodes,
        CloudNodes:     cloudNodes,
        CurrentTime:    time.Now(),
        // TODO: Add historical data from InfluxDB
        // TODO: Add forecast data (B7)
    }
}

func (e *ScalingEngine) executeScaling(recommendation ScaleRecommendation) error {
    switch recommendation.Action {
    case ScaleActionScaleUp:
        return e.scaleUp(recommendation)

    case ScaleActionScaleDown:
        return e.scaleDown(recommendation)

    case ScaleActionProvisionSpare:
        return e.provisionSpare(recommendation)

    default:
        return nil
    }
}

func (e *ScalingEngine) scaleUp(rec ScaleRecommendation) error {
    logger.Info("Scaling UP", map[string]interface{}{
        "server_type": rec.ServerType,
        "count":       rec.Count,
        "reason":      rec.Reason,
    })

    for i := 0; i < rec.Count; i++ {
        // Provision new VM
        node, err := e.vmProvisioner.ProvisionNode(rec.ServerType)
        if err != nil {
            logger.Error("Failed to provision node", err, nil)

            // Publish scaling event (failed)
            events.PublishScalingEvent("scale_up", "failed", err.Error())
            return err
        }

        logger.Info("Node provisioned successfully", map[string]interface{}{
            "node_id":   node.ID,
            "node_type": rec.ServerType,
        })

        // Publish scaling event (success)
        events.PublishScalingEvent("scale_up", "success", node.ID)
    }

    return nil
}

func (e *ScalingEngine) scaleDown(rec ScaleRecommendation) error {
    // Find idle cloud node to remove
    cloudNodes := e.nodeRegistry.GetNodesByType("cloud")

    if len(cloudNodes) == 0 {
        return nil
    }

    // Find node with lowest utilization
    nodeToRemove := e.findLeastUtilizedNode(cloudNodes)

    // Drain node (move containers to other nodes)
    if err := e.drainNode(nodeToRemove); err != nil {
        return err
    }

    // Delete VM
    if err := e.cloudProvider.DeleteServer(nodeToRemove.ID); err != nil {
        return err
    }

    // Unregister from NodeRegistry
    e.nodeRegistry.UnregisterNode(nodeToRemove.ID)

    logger.Info("Node scaled down", map[string]interface{}{
        "node_id": nodeToRemove.ID,
    })

    events.PublishScalingEvent("scale_down", "success", nodeToRemove.ID)

    return nil
}
```

---

## 🔧 Implementation Order

### Phase 1: Cloud Provider Interface (Tag 1)
```
✅ Create internal/cloud/provider.go (interface)
✅ Create internal/cloud/hetzner_provider.go (implementation)
✅ Test with Hetzner Cloud API (create/delete VM)
```

### Phase 2: VM Provisioner (Tag 1)
```
✅ Create internal/conductor/vm_provisioner.go
✅ Cloud-Init script generation
✅ Docker + Agent installation
✅ Node registration
```

### Phase 3: Scaling Engine (Tag 1-2)
```
✅ Create internal/conductor/scaling_policy.go (interface)
✅ Create internal/conductor/policy_reactive.go (B5)
✅ Create internal/conductor/scaling_engine.go
✅ Integration in Conductor
```

### Phase 4: Monitoring (Tag 0.5)
```
✅ Add Prometheus metrics (fleet_capacity_percent, scaling_events_total)
✅ Event-Bus events (scaling.triggered)
✅ API endpoint: /api/conductor/scaling/status
```

### Phase 5: Testing (Tag 0.5)
```
✅ Test scale-up (create Hetzner VM)
✅ Test scale-down (delete VM)
✅ Test cooldown period
```

---

## 🚀 Future Extensions (B6, B7)

### B6 - Hot-Spare Pool (einfach hinzufügen)
```go
// Just add policy!
engine.RegisterPolicy(NewSparePoolPolicy())
```

### B7 - Predictive Scaling (einfach hinzufügen)
```go
// Just add policy + forecast data!
engine.RegisterPolicy(NewPredictivePolicy())

// Fetch forecast from Python service
ctx.ForecastedRAMIn1h = fetchForecast(1 * time.Hour)
```

**Kein Refactoring nötig!** Alles ist vorbereitet.

---

## ✅ Vorteile dieses Designs

1. **Ein System, nicht drei:** ScalingEngine orchestriert alles
2. **Pluggable Policies:** Neue Strategien = 1 neue Datei
3. **Provider-Agnostic:** Hetzner heute, AWS morgen
4. **Event-Driven:** Nutzt Event-Bus für Observability
5. **Testbar:** Mock CloudProvider für Tests
6. **Smart:** Policies haben Prioritäten, Urgency, Cooldowns

**Bereit zum Starten?** 🚀
