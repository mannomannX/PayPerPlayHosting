# Race Condition & OOM Crash - Root Cause Analysis

**Date**: 2025-11-10
**Status**: ROOT CAUSE IDENTIFIED
**Severity**: CRITICAL

## Executive Summary

The system experiences 200% CPU usage and OOM crashes when starting 3+ servers in parallel. The atomic RAM allocation fix was implemented correctly but didn't solve the problem due to **RAM tracking state loss on restart**.

## Problem Timeline

1. User starts 3 servers in parallel
2. System hits 200% CPU and becomes unresponsive
3. User hard-restarts the server
4. Docker containers auto-recover (2 servers running)
5. Conductor resets to `allocated_ram_mb: 0` (CRITICAL BUG!)
6. System allows more servers to start than capacity permits
7. OOM crash repeats

## Root Cause: State Synchronization Failure

### Current State (2025-11-10 16:15 UTC)

**Docker Reality**:
```bash
mc-098b51ab: 869 MB RAM (running, healthy)
mc-9803548d: 948 MB RAM (running, healthy)
Total Actual Usage: ~1817 MB
```

**Conductor Tracking**:
```json
{
  "allocated_ram_mb": 0,
  "available_ram_mb": 2500,
  "container_count": 0,
  "containers": []
}
```

**THE PROBLEM**: Conductor thinks it has 2500 MB available when it only has ~683 MB (2500 - 1817).

## Technical Analysis

### Why Atomic Allocation Didn't Help

The atomic allocation code in `node_registry.go:AtomicAllocateRAM()` is **correct**:

```go
func (r *NodeRegistry) AtomicAllocateRAM(ramMB int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	node, exists := r.nodes["local-node"]
	if !exists {
		return false
	}

	usableRAM := node.UsableRAMMB()
	availableRAM := usableRAM - node.AllocatedRAMMB // BUG: AllocatedRAMMB is wrong!

	if availableRAM < ramMB {
		return false
	}

	node.AllocatedRAMMB += ramMB
	node.ContainerCount++
	return true
}
```

The problem: **`node.AllocatedRAMMB` doesn't reflect reality after a restart**.

### State Loss Scenarios

1. **Manual Restart** (current case):
   - User runs `docker restart payperplay-api`
   - Docker containers keep running (mc- containers)
   - Conductor state resets to 0
   - No sync on startup

2. **OOM Crash**:
   - API crashes due to OOM
   - Docker restarts the API container
   - mc- containers keep running (separate containers)
   - Conductor state resets to 0

3. **Deployment**:
   - New Docker image deployed
   - mc- containers keep running
   - Conductor state resets to 0

## Why 200% CPU Happens

1. System has 2 vCPUs (200% max)
2. Base system uses ~100% (1 full core)
3. Each Minecraft server uses ~50-100% CPU during startup
4. With 3 servers starting simultaneously:
   - Base: 100%
   - Server 1: 50%
   - Server 2: 50%
   - Server 3: 50%
   - **Total: 250% → System thrashing**

## RAM Capacity Math

```
Total RAM:           3814 MB (physical)
System Reserved:     1000 MB (OS, Docker, API)
Usable for servers:  2500 MB (calculated)
-----------------------------------
Server Slot 1:       1024 MB (allocated)
Server Slot 2:       1024 MB (allocated)
-----------------------------------
Theoretical Max:     2 x 1GB servers (perfectly safe)
```

**But actual container overhead means each 1GB server uses ~900-950 MB**, so 2 servers = ~1.8-1.9 GB, leaving ~600-700 MB free.

## The Fix Strategy

### 1. Container Sync on Startup ✅ (IMPLEMENTING)

On Conductor startup:
```go
func (c *Conductor) SyncRunningContainers() {
	// Query Docker for all mc-* containers
	containers := dockerClient.ListContainers("mc-")

	for _, container := range containers {
		// Extract server ID and RAM from labels
		serverID := container.Labels["server_id"]
		ramMB := container.Labels["ram_mb"]

		// Update Conductor tracking
		c.NodeRegistry.SetAllocatedRAM(ramMB)
		c.ContainerRegistry.Register(containerInfo)
	}
}
```

### 2. Persistent RAM Tracking (Future)

Store allocation state in PostgreSQL:
```sql
CREATE TABLE resource_allocations (
    node_id VARCHAR PRIMARY KEY,
    allocated_ram_mb INTEGER,
    container_count INTEGER,
    last_updated TIMESTAMP
);
```

### 3. Enhanced Debug Logging ✅ (IMPLEMENTING)

```go
logger.Debug("AtomicAllocateRAM", map[string]interface{}{
	"requested_ram":  ramMB,
	"available_ram":  availableRAM,
	"allocated_ram":  node.AllocatedRAMMB,
	"usable_ram":     usableRAM,
	"result":         success,
})
```

### 4. Actual RAM Verification (Future)

Query Docker for actual RAM usage:
```go
func (c *Conductor) VerifyRAMTracking() {
	dockerRAM := GetDockerActualRAM()
	conductorRAM := c.NodeRegistry.AllocatedRAMMB

	if abs(dockerRAM - conductorRAM) > threshold {
		logger.WARN("RAM tracking drift detected", ...)
		c.SyncRunningContainers() // Re-sync
	}
}
```

## Impact Assessment

### Before Fix
- ❌ System crashes with 3+ parallel starts
- ❌ RAM tracking inaccurate after restart
- ❌ No visibility into actual vs tracked state
- ❌ Users experience downtime

### After Fix
- ✅ Container state synced on startup
- ✅ Accurate RAM tracking even after restarts
- ✅ Debug logging for troubleshooting
- ✅ System stable under load

## Testing Plan

1. **Baseline Measurement**:
   - Stop all mc- containers
   - Measure: 0 servers = X MB used
   - Start 1 server = Y MB used
   - Start 2 servers = Z MB used
   - **Verify**: 3rd server is rejected (capacity check works)

2. **Restart Resilience**:
   - Start 2 servers
   - Restart API container
   - **Verify**: Conductor shows 2048 MB allocated
   - Try starting 3rd server
   - **Verify**: Rejected due to insufficient capacity

3. **Parallel Start Protection**:
   - Stop all servers
   - Start 3 servers in parallel
   - **Verify**: First 2 succeed, 3rd is queued

## Files Modified

1. `internal/conductor/conductor.go` - Add SyncRunningContainers()
2. `internal/conductor/node_registry.go` - Add debug logging
3. `internal/service/minecraft_service.go` - Already has atomic allocation
4. `cmd/api/main.go` - Call SyncRunningContainers() on startup

## References

- Initial Implementation: RESOURCE_GUARD_INTEGRATION.md
- Integration Analysis: IMPLEMENTATION_ROADMAP.md
- Atomic Allocation: node_registry.go:AtomicAllocateRAM()

---

**Conclusion**: The race condition fix was correct, but incomplete. State synchronization on startup is the missing piece to prevent OOM crashes.
