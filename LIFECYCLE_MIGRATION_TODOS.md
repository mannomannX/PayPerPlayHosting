# Node Lifecycle Migration TODOs

This document tracks the remaining work to fully integrate the new Node Lifecycle system.

## ✅ COMPLETED

1. **Node Lifecycle States** (`internal/conductor/node_lifecycle.go`)
   - Defined lifecycle states: provisioning, initializing, ready, active, idle, draining, decommissioned, unhealthy
   - Implemented `CanBeDecommissioned()` safety check
   - Implemented state transition validation
   - Added NodeLifecycleMetrics tracking

2. **Audit Logging** (`internal/audit/audit_log.go`)
   - Created AuditLogger for tracking all destructive actions
   - Logs decommission attempts, provisions, migrations
   - Records state snapshots and decisions
   - Supports querying by node, action type

3. **Node Struct Extended** (`internal/conductor/node.go`)
   - Added `LifecycleState` field
   - Added `HealthStatus` field (separate from old Status)
   - Added `Metrics` field for lifecycle tracking
   - Deprecated old `Status` field (kept for compatibility)

4. **VMProvisioner Safety Checks** (`internal/conductor/vm_provisioner.go`)
   - Updated `DecommissionNode()` to accept `decisionBy` parameter
   - Added comprehensive safety checks before decommission
   - Integrated audit logging (success, rejection, failure)
   - State transitions (draining → decommissioned)
   - Detailed state snapshots in logs

## ⚠️ COMPILATION FIXES NEEDED

These changes will cause compilation errors that MUST be fixed:

### 1. Conductor Struct (`internal/conductor/conductor.go`)
Add AuditLog field:
```go
type Conductor struct {
    // ... existing fields
    AuditLog *audit.AuditLogger  // ADD THIS
}
```

### 2. Conductor Initialization (`cmd/api/main.go`)
Initialize AuditLogger:
```go
auditLog := audit.NewAuditLogger(1000)  // Keep 1000 entries
cond := conductor.NewConductor(
    nodeRegistry,
    containerRegistry,
    // ... other params
)
cond.AuditLog = auditLog  // ADD THIS
```

### 3. VMProvisioner Needs Conductor Reference
Current: VMProvisioner has nodeRegistry, needs full conductor for AuditLog
```go
type VMProvisioner struct {
    // ... existing fields
    conductor *Conductor  // ADD THIS for audit log access
}
```

### 4. All DecommissionNode() Calls Need Update
Search for: `DecommissionNode(nodeID)`
Replace with: `DecommissionNode(nodeID, "policy_name")`

Examples:
- Reactive Policy: `DecommissionNode(nodeID, "reactive_policy")`
- Consolidation: `DecommissionNode(nodeID, "consolidation_policy")`
- Manual: `DecommissionNode(nodeID, "manual")`

Files likely affected:
- `internal/conductor/scaling_engine.go`
- `internal/conductor/policy_reactive.go`
- `internal/conductor/policy_consolidation.go`
- Any API handlers that trigger decommission

### 5. Node Registration Needs Lifecycle State Initialization

When creating new nodes, set initial lifecycle state:

**For NEW cloud nodes (ProvisionNode):**
```go
node := &Node{
    ID: serverID,
    // ... other fields
    LifecycleState: NodeStateProvisioning,  // ADD THIS
    HealthStatus: HealthStatusUnknown,       // ADD THIS
    Metrics: NodeLifecycleMetrics{         // ADD THIS
        ProvisionedAt: time.Now(),
        CurrentContainers: 0,
        TotalContainersEver: 0,
    },
}
```

**For SYSTEM nodes (local, proxy):**
```go
node := &Node{
    ID: "local-node",
    // ... other fields
    LifecycleState: NodeStateActive,  // System nodes start as active
    HealthStatus: HealthStatusHealthy,
    Metrics: NodeLifecycleMetrics{
        ProvisionedAt: time.Now(),
        CurrentContainers: 0,
        TotalContainersEver: 0,
    },
}
```

**For RECOVERED nodes (SyncExistingWorkerNodes):**
```go
node := &Node{
    ID: serverID,
    // ... other fields
    LifecycleState: NodeStateReady,  // Unknown history, start as ready
    HealthStatus: HealthStatusUnknown,  // Will be checked by health checker
    Metrics: NodeLifecycleMetrics{
        ProvisionedAt: time.Now(),  // Or use server creation time from Hetzner
        CurrentContainers: 0,
        TotalContainersEver: 0,
    },
}
```

### 6. Health Checker State Transitions

When health check completes (`internal/conductor/health_checker.go`):

**After Cloud-Init completes:**
```go
if node.LifecycleState == NodeStateInitializing {
    node.TransitionLifecycleState(NodeStateReady, "cloud_init_completed")
}
```

**When first container added:**
```go
if node.ShouldTransitionToActive() {
    node.TransitionLifecycleState(NodeStateActive, "first_container_added")
}
```

**When last container removed:**
```go
if node.ShouldTransitionToIdle() {
    node.TransitionLifecycleState(NodeStateIdle, "last_container_removed")
}
```

**When container added to idle node:**
```go
if node.ShouldTransitionFromIdle() {
    node.TransitionLifecycleState(NodeStateActive, "container_added")
}
```

### 7. Container Add/Remove Tracking

Update `ContainerRegistry.RegisterContainer()`:
```go
func (r *ContainerRegistry) RegisterContainer(container *Container) {
    // ... existing code

    // Update node metrics
    if node, exists := r.nodeRegistry.GetNode(container.NodeID); exists {
        node.Metrics.CurrentContainers = node.ContainerCount
        node.Metrics.TotalContainersEver++

        if node.Metrics.FirstContainerAt == nil {
            now := time.Now()
            node.Metrics.FirstContainerAt = &now
        }
    }
}
```

Update `ContainerRegistry.UnregisterContainer()`:
```go
func (r *ContainerRegistry) UnregisterContainer(containerID string) {
    // ... existing code

    // Update node metrics
    if node, exists := r.nodeRegistry.GetNode(container.NodeID); exists {
        node.Metrics.CurrentContainers = node.ContainerCount

        if node.ContainerCount == 0 {
            now := time.Now()
            node.Metrics.LastContainerAt = &now
        }
    }
}
```

### 8. Import Statement for audit package

Add to files that use AuditLogger:
```go
import (
    "github.com/payperplay/hosting/internal/audit"
)
```

## 🔧 OPTIONAL ENHANCEMENTS (Can be done later)

1. **Planning-Aware Projections**
   - Create `FleetProjection` struct
   - Track nodes in provisioning/initializing state
   - Consider "future capacity" in scaling decisions

2. **Migration Planning**
   - Create `MigrationPlan` struct
   - Consolidation with safety checks
   - Cost-benefit analysis

3. **Dashboard Integration**
   - Expose audit log via API
   - Show lifecycle state in node list
   - Metrics tracking

4. **Database Persistence**
   - Store audit log in PostgreSQL
   - Query historical decisions
   - Long-term analytics

## 🚀 DEPLOYMENT CHECKLIST

Before deploying:
1. ✅ All compilation errors fixed
2. ✅ All DecommissionNode() calls updated
3. ✅ Node initialization sets lifecycle state
4. ✅ Audit log initialized in main.go
5. ✅ Test locally with `go build`
6. ✅ Commit and push to git
7. ✅ Deploy via docker-compose
8. ✅ Test with real server creation/deletion
9. ✅ Check audit logs for rejected decommissions
10. ✅ Verify nodes don't get deleted prematurely

## 📝 TESTING SCENARIOS

After deployment, test these scenarios:

**Scenario 1: New Node → Active → Idle → Decommission**
1. Create server → Node provisioned (state: provisioning)
2. Cloud-Init completes → Node ready (state: ready)
3. Container starts → Node active (state: active)
4. Container stops → Node idle (state: idle)
5. Wait 30min (or configured grace period)
6. Scaling engine should decommission idle node

**Scenario 2: Safety Rejection - Node Too Young**
1. Create server → Node provisioned
2. Cloud-Init completes → Node ready
3. Immediately try to decommission
4. Should be REJECTED (reason: "Ready node too young")
5. Audit log should show rejection

**Scenario 3: Safety Rejection - Has Containers**
1. Node is active with containers
2. Try to decommission
3. Should be REJECTED (reason: "Node has X containers")
4. Audit log should show rejection

**Scenario 4: System Node Protection**
1. Try to decommission local-node or proxy-node
2. Should be REJECTED (reason: "cannot decommission dedicated node")
3. Audit log should show rejection

## 🐛 KNOWN ISSUES

None yet - this is a fresh implementation.

## 📚 DOCUMENTATION

Update these files after deployment:
- `README.md` - Mention lifecycle states
- `ARCHITECTURE.md` - Document state machine
- `API.md` - Document audit log endpoints (if added)
