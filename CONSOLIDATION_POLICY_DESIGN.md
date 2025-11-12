# Intelligent Consolidation Policy - Design Document

## Problem Statement
Current consolidation policy is too aggressive and causes node churn:
- Deletes nodes immediately after provisioning
- Ignores containers being deployed
- Runs too frequently (30min cooldown)
- Has no stability checks

## Design Goals
1. **Stability First** - Never delete nodes that are actively being used
2. **Cost Optimization** - Only consolidate when savings are significant (>€0.10/h)
3. **Container-Safe** - Never disrupt running or deploying containers
4. **Tier-Aware** - Respect different RAM tiers and server types
5. **Smart Timing** - Only run when system is stable

## Core Principles

### 1. MINIMUM NODE UPTIME
- **Rule:** Node must be alive for at least 30 minutes before being considered for consolidation
- **Reason:** Prevents deleting freshly provisioned nodes
- **Implementation:** Track `node.CreatedAt`, only consolidate if `time.Since(createdAt) > 30min`

### 2. MINIMUM IDLE TIME
- **Rule:** Node must be empty for at least 15 minutes before deletion
- **Reason:** Don't delete nodes that just became empty (container might be redeploying)
- **Implementation:** Track `node.LastContainerRemovedAt`, only delete if idle >15min

### 3. DEPLOYMENT-AWARE
- **Rule:** Never consolidate while containers are being deployed
- **Reason:** ProcessStartQueue might be deploying containers right now
- **Implementation:** Check `ctx.QueuedServerCount > 0` OR any container status = 'starting'

### 4. MINIMUM NODE COUNT
- **Rule:** Always keep at least 1 Worker-Node if any containers exist
- **Reason:** Need capacity for running containers
- **Implementation:** `if totalContainers > 0 && nodeSavings >= len(cloudNodes) { return false }`

### 5. COST-AWARE THRESHOLD
- **Rule:** Only consolidate if savings >= €0.10/hour (significant)
- **Reason:** Don't churn nodes for tiny savings
- **Implementation:** Calculate real costs from node types (cpx22/32/42/62)

## Consolidation Algorithm

### Phase 1: Pre-Flight Checks
```
1. Enabled? → No: return
2. Cooldown active? (2 hours) → Yes: return
3. System stable? (no scaling operations) → No: return
4. Queue empty? → No: return (containers being deployed)
5. Enough nodes? (≥2) → No: return
```

### Phase 2: Node Analysis
```
For each Worker-Node:
  1. How long has it been alive? (node.CreatedAt)
  2. How many containers? (current count)
  3. How long has it been idle? (lastContainerRemoved)
  4. Node cost/hour? (from node type: cpx22=0.0096, cpx32=0.014, etc.)
  5. Can it be deleted? (alive>30min AND idle>15min AND containers=0)
```

### Phase 3: Perfect Packing
```
1. Collect all containers from all nodes
2. Group by RAM tier (2GB, 4GB, 8GB, 16GB, custom)
3. Calculate optimal node count using CalculatePerfectPackingNodes()
4. Ensure at least 1 node remains if containers exist
5. Calculate actual cost savings (sum of node costs to remove)
```

### Phase 4: Migration Planning
```
1. If optimal_nodes >= current_nodes → No savings, return
2. If cost_savings < €0.10/h → Not significant, return
3. Select nodes to keep (prefer cheaper nodes: cpx22 > cpx32 > cpx42 > cpx62)
4. Select nodes to remove (expensive + idle + old)
5. Build migration plan (move containers to kept nodes)
```

### Phase 5: Validation
```
1. Will migrations fit on remaining nodes? → No: abort
2. Are all migrations safe? (servers allow consolidation) → No: abort
3. Is final capacity >30% free? → No: abort (too tight)
```

## Safety Checks

### Before ANY Node Deletion:
1. ✅ Node has 0 containers
2. ✅ Node is NOT System Node (is_system_node = false)
3. ✅ Node has been alive >30 minutes
4. ✅ Node has been idle >15 minutes
5. ✅ At least 1 other Worker-Node will remain (if containers exist)
6. ✅ Queue is empty (no pending deployments)
7. ✅ No scaling operations in progress

### Cooldown Settings:
- **Consolidation Cooldown:** 2 hours (not 30 min!)
- **Reason:** Prevent thrashing, let system stabilize

### Capacity Thresholds:
- **Max Capacity for Consolidation:** 70% (keep 30% free buffer)
- **Minimum Cost Savings:** €0.10/hour (significant threshold)
- **Minimum Node Savings:** 1 node (but only if cost threshold met)

## Node Selection Strategy

### Nodes to KEEP (Priority Order):
1. Nodes with containers (always keep)
2. Cheapest nodes first (cpx22 > cpx32 > cpx42 > cpx62)
3. Newest nodes (recently provisioned = better hardware)
4. Nodes in optimal locations

### Nodes to REMOVE (Priority Order):
1. Most expensive nodes (cpx62 > cpx42 > cpx32 > cpx22)
2. Oldest idle nodes
3. Nodes with longest idle time
4. Nodes that are empty AND old (>30min) AND idle (>15min)

## Implementation Checklist

- [ ] Add `node.LastContainerRemovedAt` tracking
- [ ] Add `node.IdleDuration()` method
- [ ] Add real cost calculation from node types
- [ ] Change cooldown from 30min → 2 hours
- [ ] Add minimum uptime check (30min)
- [ ] Add minimum idle check (15min)
- [ ] Add "at least 1 node remains" rule
- [ ] Add cost threshold check (€0.10/h)
- [ ] Add deployment-aware check (queue + starting containers)
- [ ] Add node selection strategy (prefer cheap/new)
- [ ] Add comprehensive logging for debugging

## Testing Scenarios

### Scenario 1: Fresh Node Provisioning
```
Given: 1 Worker-Node just provisioned (age: 5min)
And: 0 containers
Then: Consolidation should SKIP (node too young)
```

### Scenario 2: Container Just Removed
```
Given: 1 Worker-Node (age: 1 hour, idle: 2min)
Then: Consolidation should SKIP (idle time too short)
```

### Scenario 3: Queue Has Servers
```
Given: 2 Worker-Nodes (both idle >15min)
And: 1 server in queue
Then: Consolidation should SKIP (deployment pending)
```

### Scenario 4: Valid Consolidation
```
Given: 3 Worker-Nodes (cpx32, cpx42, cpx62)
And: All empty for >15min, all alive >30min
And: 0 servers in queue
And: Can pack all containers into 1 node (cpx32)
And: Cost savings = €0.045/h (cpx42 + cpx62 removed)
Then: Consolidation should PROCEED
      Remove: cpx42, cpx62 (most expensive)
      Keep: cpx32 (cheapest)
```

### Scenario 5: Always Keep 1 Node
```
Given: 2 Worker-Nodes (both empty)
And: 0 containers, 0 queue
Then: Consolidation should KEEP AT LEAST 1 NODE
      (For fast scaling when new servers arrive)
```

## Cost Calculation

### Node Types & Costs (CPX2 Series):
```
cpx12: 2GB RAM  = €0.0048/h  (not used for workers)
cpx22: 4GB RAM  = €0.0096/h  ← Preferred for small loads
cpx32: 8GB RAM  = €0.0168/h  ← Good balance
cpx42: 16GB RAM = €0.0312/h  ← For heavier loads
cpx52: 24GB RAM = €0.0624/h  ← For very heavy loads
cpx62: 32GB RAM = €0.1056/h  ← Most expensive
```

### Savings Calculation:
```go
var totalSavings float64
for _, nodeID := range nodesToRemove {
    node := getNode(nodeID)
    totalSavings += node.HourlyCostEUR
}

if totalSavings < 0.10 {
    return false // Not worth the churn
}
```

## Migration Safety

### Servers That CAN Be Migrated:
- Tier: Micro/Small/Medium (2GB/4GB/8GB)
- Plan: PayPerPlay (elastic)
- Players: 0 (empty servers only)
- Status: running (not starting/stopping)

### Servers That CANNOT Be Migrated:
- Tier: Large/XLarge (16GB+) → Too risky
- Tier: Custom → Inefficient packing
- Plan: Balanced with players → Disrupts gameplay
- Status: starting/stopping → Unstable

## Monitoring & Logging

### Log Every Consolidation Decision:
```json
{
  "event": "consolidation.evaluated",
  "nodes_before": 3,
  "nodes_after": 1,
  "nodes_removed": ["cpx42-123", "cpx62-456"],
  "nodes_kept": ["cpx32-789"],
  "cost_savings_per_hour": 0.1368,
  "cost_savings_per_month": 99.86,
  "containers_migrated": 0,
  "reason": "2 nodes idle >15min, savings significant"
}
```

### Metrics to Track:
- Consolidations per day
- Average cost savings per consolidation
- Failed consolidations (and why)
- Node churn rate (provisions vs decommissions)

## Conclusion

This new design ensures:
✅ Nodes stay stable (30min minimum uptime)
✅ No premature deletions (15min idle time)
✅ Container-safe (never during deployment)
✅ Cost-effective (only if savings >€0.10/h)
✅ Smart selection (keep cheap, remove expensive)
