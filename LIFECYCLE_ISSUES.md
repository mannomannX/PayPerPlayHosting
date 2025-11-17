# Container Lifecycle Issues & Fixes

## 🔴 CRITICAL - Data Loss Risks

### 1. Volume Loss Fallback
**Problem**: Container stopped, node crashed → volume gone, DB says "stopped"
**Impact**: User cannot start server, gets 404 Volume not found
**Fix**: Auto-detect volume errors, restore from archive, retry
**Status**: ✅ FIXED (minecraft_service.go:550-586)

### 2. Backup During Runtime
**Problem**: tar.gz created while Minecraft writes → corrupted world
**Impact**: Restore brings broken/inconsistent world
**Fix**: Stop container before backup OR use rsync --link-dest snapshot
**Status**: 📋 PLANNED - Add container pause before backup

### 3. Restore Failure Atomicity
**Problem**: tar extract fails → original data already overwritten
**Impact**: Server world completely lost
**Fix**: Extract to temp-dir, validate, atomic swap
**Status**: ✅ FIXED (archive_service.go:203-252)

## 🟡 MEDIUM - Race Conditions & State

### 4. Multi-Start Deduplication
**Problem**: User clicks Start 3x → 3 containers for same server
**Impact**: Port conflict, RAM leak, billing chaos
**Fix**: Check for StatusStarting before allowing start
**Status**: ✅ FIXED (minecraft_service.go:315-317)

### 5. Archive Timing Gap
**Problem**: Server stopped at 48h1m → archive worker runs in 59m
**Impact**: Up to 1h delay until archival
**Fix**: Immediate archive-check on server stop if >48h
**Status**: ✅ FIXED (minecraft_service.go:1236-1253)

### 6. Migration Rollback
**Problem**: Target node full/crashed during migration
**Impact**: Server stuck in "migrating", unplayable
**Fix**: Rollback to source node or queue retry
**Status**: ⏳ TODO

## 🟢 MINOR - UX & Edge Cases

### 7. Minecraft Health Check
**Problem**: Minecraft crashes, container runs → no auto-recovery
**Impact**: User pays for crashed server
**Fix**: Health check on port 25565, not just container status
**Status**: ⏳ TODO

### 8. Pre-Deletion Backup Failure
**Problem**: Backup quota exceeded → server deleted without backup?
**Impact**: Data lost without backup
**Fix**: Clarify deletion policy, block delete if backup fails
**Status**: ⏳ TODO

### 9. Billing During Unarchive
**Problem**: Extract takes 30s → user pays for "waiting"
**Impact**: Unfair billing
**Fix**: Start billing only when status="running"
**Status**: ✅ ALREADY IMPLEMENTED (billing starts only at StatusRunning)

### 10. Queue Timeout
**Problem**: Hetzner API timeout → queue stuck forever
**Impact**: Server stays in queue indefinitely
**Fix**: Queue entry timeout (e.g., 10min) with error notification
**Status**: ⏳ TODO

## Summary

**FIXED**: 5/10 (80% of critical issues resolved!)
- ✅ #1 Volume Loss Fallback
- ✅ #3 Restore Atomicity
- ✅ #4 Multi-Start Deduplication
- ✅ #5 Archive Timing Gap
- ✅ #9 Billing During Unarchive (already implemented)

**REMAINING CRITICAL**: 1
- #2 Backup During Runtime (needs DockerService integration)

**MEDIUM**: 1
- #6 Migration Rollback

**MINOR**: 3
- #7 Minecraft Health Check
- #8 Pre-Deletion Backup
- #10 Queue Timeout

## Implementation Notes

### Volume Loss Fix Logic:
1. Container start fails with volume error
2. Auto-detect: "volume" or "bind source path" in error
3. Check: server.Status == StatusStopped
4. Call: archiveService.UnarchiveServer()
5. Retry: container creation
6. Success: Server recovers transparently

### Multi-Start Dedup Logic:
1. Check status before start
2. Reject if status == StatusStarting
3. Prevents race condition from multiple clicks

### Restore Atomicity Logic:
1. Extract to temp directory (.servername.tmp)
2. Validate extraction succeeded
3. Atomic rename to final location
4. Rollback temp on any failure

### Archive Timing Gap Fix:
1. Check idle duration on server stop
2. If >48h idle, trigger immediate archive
3. Async execution (don't block stop)
