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
**Fix**: Extract to temp-dir, atomic swap only on success
**Status**: 📋 PLANNED - Modify UnarchiveServer to use temp directory

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
**Status**: ⏳ TODO

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
**Status**: ⏳ TODO

### 10. Queue Timeout
**Problem**: Hetzner API timeout → queue stuck forever
**Impact**: Server stays in queue indefinitely
**Fix**: Queue entry timeout (e.g., 10min) with error notification
**Status**: ⏳ TODO

## Implementation Plan
1. Fix #1 (Volume Loss Fallback) - CRITICAL
2. Fix #4 (Multi-Start Dedup) - HIGH
3. Fix #7 (Minecraft Health) - HIGH
4. Fix #2 (Backup Safety) - MEDIUM
5. Fix #3 (Restore Atomicity) - MEDIUM
6. Fix #6 (Migration Rollback) - MEDIUM
7. Fix #5,#8,#9,#10 - LOW
