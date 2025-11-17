# User Flow Issues & UX Problems

**Comprehensive analysis of all user flows, edge cases, and potential issues**

Date: 2025-01-17
Status: Analysis Complete - 53 Issues Identified

---

## 🔴 CRITICAL - Security & Data Loss

### AUTH-1: Email Verification Bypass
**Flow**: User Registration
**Problem**: User receives JWT token immediately after registration, even though `EmailVerified=false`
**Impact**: Can create servers and use API without verifying email
**Current Code**: `auth_handlers.go:57-73` - Token generated before email verification
**Fix Required**: Block token generation until email verified, or restrict API access for unverified users

### AUTH-2: OAuth Callback Frontend Gap
**Flow**: OAuth Login (Discord/Google/GitHub)
**Problem**: OAuth callback returns JSON but no frontend redirect handling
**Impact**: User sees raw JSON response instead of being redirected to dashboard
**Current Code**: `oauth_handlers.go:76-88` - Returns JSON, no redirect
**Fix Required**: Implement frontend redirect with token in URL or cookie

### SERVER-1: Balance Check Missing
**Flow**: Server Creation
**Problem**: No pre-authorization balance check before server creation
**Impact**: User can create servers with negative balance
**Current Code**: `handlers.go:30-81` - No balance validation
**Fix Required**: Check `user.CanAfford(estimatedCost)` before creating server

### SERVER-2: Start After Delete Race
**Flow**: Server Deletion
**Problem**: If server is queued/starting and user clicks delete, container may start AFTER deletion
**Impact**: Orphaned container, continued billing, data loss
**Current Code**: `minecraft_service.go:1310-1349` - No status check before deletion
**Fix Required**: Block deletion if status is `starting` or `queued`, force stop first

### BACKUP-1: Restore Without Validation
**Flow**: Backup Restore
**Problem**: Restore deletes current world before validating backup integrity (FIX #3 only validates extract, not content)
**Impact**: Data loss if backup is corrupted
**Current Code**: `archive_service.go:203-252` - Validates tar extraction but not world data
**Fix Required**: Validate critical files exist (level.dat, region files) before atomic swap

### BACKUP-2: Quota Deadlock
**Flow**: Backup Quota Exceeded → Server Deletion
**Problem**: FIX #8 blocks deletion if backup quota exceeded, but user can't delete old backups to free quota
**Impact**: User cannot delete server, quota permanently stuck
**Current Code**: `minecraft_service.go:1310-1349` - Blocks deletion, no backup management UI
**Fix Required**: Implement backup list/delete API, or allow deletion if quota exceeded

### FILE-1: No Upload Validation
**Flow**: File Upload to Server
**Problem**: No file type validation, size limits, or virus scanning
**Impact**: User can upload malicious files, fill disk, crash server
**Current Code**: File upload handlers exist but no validation
**Fix Required**: Whitelist file extensions (.jar, .yml, .json, .png), max file size, virus scan

### BILLING-1: Negative Balance Allowed
**Flow**: Server Running → Balance Depleted
**Problem**: Billing continues even when balance hits 0, goes negative
**Impact**: User owes money, no payment enforcement
**Current Code**: `billing_service.go` - No balance check on billing events
**Fix Required**: Auto-stop servers when balance < 0, require pre-paid balance or payment method

### BILLING-2: Crash Billing Continue
**Flow**: Minecraft Crashes, Container Runs
**Problem**: FIX #7 detects crashes but doesn't stop billing
**Impact**: User pays for non-functional server
**Current Code**: `health_checker.go:447-457` - Logs warning but no action
**Fix Required**: Auto-stop container and refund time if Minecraft unresponsive >5 minutes

---

## 🟡 MEDIUM - UX & Business Logic

### AUTH-3: Account Deletion No Checks
**Flow**: Account Deletion
**Problem**: User can delete account with active servers, positive balance, pending billing
**Impact**: Orphaned servers, billing data loss, support issues
**Current Code**: `auth_service.go:416-435` - Just deletes user
**Fix Required**: Check for active servers (block or auto-delete), check balance (refund or require withdrawal)

### AUTH-4: Password Reset Timing Attack
**Flow**: Password Reset Request
**Problem**: Always returns success but timing reveals if email exists
**Impact**: Email enumeration via timing side-channel
**Current Code**: `auth_service.go:300-318` - Database lookup reveals timing
**Fix Required**: Add constant-time delay or dummy email send for non-existent users

### AUTH-5: Device Trust No Management
**Flow**: Login from New Device
**Problem**: Device trust system exists, alerts sent, but no UI to view/revoke devices
**Impact**: User can't manage trusted devices, no way to revoke compromised device
**Current Code**: Security events logged, no API for device management
**Fix Required**: Implement `/api/auth/devices` to list, trust, revoke devices

### SERVER-3: Minecraft Version Not Validated
**Flow**: Server Creation
**Problem**: User can specify any string as `minecraft_version`, no validation against supported versions
**Impact**: Server creation fails silently or pulls wrong Docker image
**Current Code**: `handlers.go:25` - String field, no validation
**Fix Required**: Validate version format (e.g., `1.20.4`), check against supported versions list

### SERVER-4: Port Pool Exhaustion
**Flow**: Server Creation
**Problem**: Port range 25565-25665 (100 ports), no handling when exhausted
**Impact**: Server creation fails, queue grows, no capacity expansion
**Current Code**: Port allocation exists but no exhaustion handling
**Fix Required**: Dynamic port range expansion, or queue servers until port available

### SERVER-5: Name Validation Missing
**Flow**: Server Creation
**Problem**: Server name allows duplicates, special characters, unicode, SQL injection chars
**Impact**: Confusing UX, potential injection attacks, Velocity routing issues
**Current Code**: `handlers.go:23` - String field, no validation
**Fix Required**: Regex validation `^[a-zA-Z0-9_-]{3,32}$`, check uniqueness per user

### SERVER-6: Status Transition Not Atomic
**Flow**: Server Start (StatusStarting → StatusRunning)
**Problem**: Status update and billing start are separate transactions
**Impact**: Server shows "running" but billing never started, or vice versa
**Current Code**: Status updated before container fully started
**Fix Required**: Atomic transaction or event-based status update after container healthy

### SERVER-7: Start Failure Rollback
**Flow**: Server Start → Container Fails
**Problem**: If container fails after billing started, billing continues
**Impact**: User charged for failed server
**Current Code**: Billing starts on `StatusStarting`, no rollback on failure
**Fix Required**: Close billing session if start fails, refund partial time

### SERVER-8: No Graceful Shutdown Notice
**Flow**: Server Stop (Manual or Idle)
**Problem**: Players in-game get kicked without warning
**Impact**: Poor UX, data loss (unsaved progress), player complaints
**Current Code**: `docker_service.go` sends SIGTERM immediately
**Fix Required**: Send RCON command "say Server shutting down in 10s", wait, then stop

### CONFIG-1: Apply Without Restart Warning
**Flow**: Config Change (MOTD, Difficulty, etc.)
**Problem**: Config changes accepted but require restart, no UI warning
**Impact**: User expects immediate effect, confusion when nothing changes
**Current Code**: Config updates database but doesn't notify restart required
**Fix Required**: Return `{"restart_required": true}` in API response, UI shows warning

### CONFIG-2: Invalid Values Allowed
**Flow**: Config Change
**Problem**: No validation of config values (e.g., `MaxPlayers=-1`, `ViewDistance=999`)
**Impact**: Server crashes on start, poor performance
**Current Code**: Database accepts any integer value
**Fix Required**: Validate ranges (MaxPlayers: 1-1000, ViewDistance: 2-32, etc.)

### CONFIG-3: RCON Password Exposed
**Flow**: Get Server Details
**Problem**: RCON password returned in API response
**Current Code**: `handlers.go:111` - Returns full server object including `RCONPassword`
**Impact**: Security risk if API response logged or leaked
**Fix Required**: Exclude `RCONPassword` from JSON serialization, use `-` tag

### CONFIG-4: No Audit Trail
**Flow**: Config Change
**Problem**: No record of who changed what, when
**Impact**: Support issues, no accountability, can't revert changes
**Current Code**: ConfigChange model exists but not populated
**Fix Required**: Log all config changes to `config_changes` table with user, timestamp, old/new values

### CONFIG-5: Runtime Config Ignored
**Flow**: Config Change While Server Running
**Problem**: Database updated but container not notified
**Impact**: Changes lost on next restart
**Current Code**: No RCON command or container restart trigger
**Fix Required**: Send RCON `reload` command or auto-restart container (with warning)

### BILLING-3: Reserved Plan No Monthly Job
**Flow**: Reserved Plan Server
**Problem**: `Plan=reserved` exists but no scheduled job for monthly billing
**Impact**: Reserved customers never billed
**Current Code**: `billing_service.go` only tracks usage sessions, no monthly billing
**Fix Required**: Cron job to bill reserved servers on 1st of month (prorated if mid-month)

### BILLING-4: No Invoice Generation
**Flow**: Billing Events Recorded
**Problem**: `BillingEvent` table exists but no invoice/receipt generation
**Impact**: User can't download invoices, no payment history
**Current Code**: Events logged, no aggregation or PDF export
**Fix Required**: Implement `/api/billing/invoices` with monthly summaries, PDF export

### BILLING-5: No Cost Estimate
**Flow**: Server Creation
**Problem**: User doesn't see cost estimate before creating server
**Impact**: Billing shock, user creates expensive server accidentally
**Current Code**: Cost calculated internally but not shown to user
**Fix Required**: Return `estimated_hourly_cost` in create API response, show in UI

### BILLING-6: Currency Hardcoded EUR
**Flow**: All Billing
**Problem**: Prices hardcoded in EUR, no multi-currency support
**Impact**: Non-EU users confused by pricing
**Current Code**: `PricingConfig` uses EUR floats
**Fix Required**: Add `Currency` field to User model, convert on display (or store in cents)

### BACKUP-3: No Backup Management UI
**Flow**: View Backups
**Problem**: Backups created but no API to list, download, or delete backups
**Impact**: User can't manage backups, can't free quota
**Current Code**: Backup model exists, no list/delete endpoints
**Fix Required**: Implement `/api/servers/:id/backups` GET/DELETE endpoints

### BACKUP-4: Scheduled Backups Not Implemented
**Flow**: Automatic Backups
**Problem**: `ServerBackupSchedule` model exists but no cron job
**Impact**: Only manual backups work, advertised feature missing
**Current Code**: Model exists, no worker implementation
**Fix Required**: Implement backup scheduler worker (check `backup_schedules` table every hour)

### BACKUP-5: No Backup Encryption
**Flow**: Backup Creation
**Problem**: Backups stored as plain tar.gz, no encryption
**Impact**: Storage Box compromise leaks all user world data
**Current Code**: `backup_service.go:218-250` - Plain tar.gz
**Fix Required**: Encrypt archives with user-specific key (AES-256), store key in database

### BACKUP-6: Deleted Server Backup No Restore
**Flow**: Server Deletion → Restore
**Problem**: FIX #8 creates pre-deletion backup but no UI to restore deleted servers
**Impact**: Backup useless, user thinks data lost
**Current Code**: Backup created, no restore endpoint for deleted servers
**Fix Required**: Allow restore of deleted servers (create new server from backup)

### FILE-2: Upload Disk Quota Missing
**Flow**: File Upload
**Problem**: No per-user or per-server disk quota enforcement
**Impact**: User can fill entire disk, crash system
**Current Code**: File upload exists but no quota check
**Fix Required**: Set quota per tier (e.g., Small=5GB, Medium=10GB), check before upload

### FILE-3: Upload to Running Server
**Flow**: File Upload While Server Running
**Problem**: File uploaded to disk but container doesn't see it (volume mount)
**Impact**: User expects immediate effect, confusion
**Current Code**: File written to host, container volume may be cached
**Fix Required**: Notify container (RCON `reload` or restart), or block uploads while running

### PLUGIN-1: No Compatibility Check
**Flow**: Plugin Installation
**Problem**: User can install plugin for wrong Minecraft/server version
**Impact**: Server crashes on start, plugin conflicts
**Current Code**: Plugin install exists but no version validation
**Fix Required**: Check `plugin.compatible_versions` against `server.minecraft_version`

### PLUGIN-2: Corrupted Plugin No Auto-Uninstall
**Flow**: Plugin Corruption
**Problem**: Corrupted plugin crashes server, no auto-detection or removal
**Impact**: Server stuck in crash loop
**Current Code**: FIX #7 detects crashes but doesn't identify plugin cause
**Fix Required**: Parse crash logs for plugin exceptions, auto-disable suspect plugin

### PLUGIN-3: No Dependency Resolution
**Flow**: Plugin Installation
**Problem**: Plugin requires dependencies (e.g., ProtocolLib) but not auto-installed
**Impact**: Plugin doesn't work, user confused
**Current Code**: No dependency tracking
**Fix Required**: Add `dependencies` field to Plugin model, auto-install deps on plugin install

### PLUGIN-4: Silent Update Breaks Server
**Flow**: Plugin Auto-Update
**Problem**: Plugin auto-updates to version incompatible with Minecraft version
**Impact**: Server crashes, user doesn't know why
**Current Code**: Plugin updates implemented but no safety checks
**Fix Required**: Disable auto-update by default, or create backup before update

---

## 🟢 MINOR - Edge Cases & Polish

### AUTH-6: No Email Change Flow
**Flow**: Change Email Address
**Problem**: User can update username but not email
**Impact**: User stuck with typo email
**Current Code**: No email change endpoint
**Fix Required**: Implement email change with verification (send code to new email)

### AUTH-7: Token Refresh Window Small
**Flow**: Token Expiration
**Problem**: Token expires after 24h, no refresh window overlap
**Impact**: User logged out mid-session if client doesn't refresh in time
**Current Code**: `auth_service.go:163` - 24h expiry, no overlap
**Fix Required**: Implement refresh token rotation or longer expiry (7 days) with sliding window

### SERVER-9: Archived Server Delete Without Unarchive
**Flow**: Delete Archived Server
**Problem**: User can delete archived server without unarchiving first
**Impact**: Archive file left on Storage Box, wasted space
**Current Code**: Deletion allowed regardless of lifecycle phase
**Fix Required**: Auto-delete archive file when deleting archived server

### SERVER-10: Multi-User Access Race
**Flow**: Multiple Users Control Same Server (Future)
**Problem**: If team access added, start/stop race conditions possible
**Impact**: FIX #4 only checks same user, not concurrent users
**Current Code**: No team access yet, but race possible in future
**Fix Required**: Use distributed lock (Redis) for start/stop operations

### SERVER-11: Container Crash No Auto-Recovery
**Flow**: Minecraft Crash → Auto-Restart
**Problem**: FIX #7 detects crashes but doesn't restart container
**Impact**: User manually restarts, poor UX
**Current Code**: `health_checker.go:456` - TODO comment, no implementation
**Fix Required**: Auto-restart container (max 3 retries), notify user if repeated crashes

### SERVER-12: Queue Position Not Shown
**Flow**: Server Queued (No Capacity)
**Problem**: User sees "queued" but no position in queue or ETA
**Impact**: Poor UX, user doesn't know how long to wait
**Current Code**: StartQueue exists, no position tracking
**Fix Required**: Return `queue_position` and `estimated_wait_minutes` in API

### CONFIG-6: Gamemode Change No Player Notification
**Flow**: Change Gamemode While Server Running
**Problem**: Gamemode changed but players not notified, expect creative but still survival
**Impact**: Confusion, user thinks change failed
**Current Code**: Config change doesn't trigger in-game notification
**Fix Required**: Send RCON command `defaultgamemode <mode>` + notify all players

### BILLING-7: No Refund for Outages
**Flow**: Node Failure → Downtime
**Problem**: User charged for time when server was down
**Impact**: Poor customer satisfaction
**Current Code**: Billing continues regardless of uptime
**Fix Required**: Track downtime (node unhealthy events), auto-refund to balance

### BILLING-8: Pay-Per-Use Prepaid Not Implemented
**Flow**: Prepaid Balance for Pay-Per-Play
**Problem**: User mentioned "Pre-Paid für das Pay-Per-Use" but no implementation
**Impact**: User must manually add balance, no auto-topup
**Current Code**: Balance exists but no payment integration
**Fix Required**: Implement payment gateway (Stripe), auto-topup when balance < threshold

### BACKUP-7: No Backup Size Warning
**Flow**: Backup Creation
**Problem**: Large world creates huge backup, no warning before quota exceeded
**Impact**: Backup fails, user confused
**Current Code**: Quota check exists but no pre-check
**Fix Required**: Calculate world size before backup, warn if >80% quota

### BACKUP-8: Restore No Progress Indicator
**Flow**: Backup Restore (30+ seconds)
**Problem**: User clicks restore, sees nothing for 30s, thinks it failed
**Impact**: User clicks again (double restore), confusion
**Current Code**: Restore is synchronous, no progress feedback
**Fix Required**: Make restore async, WebSocket progress updates, or return `status=restoring`

### FILE-4: No File Preview
**Flow**: View Server Files
**Problem**: User can list files but not preview (configs, logs)
**Impact**: Must download to view, poor UX
**Current Code**: File list exists, no content preview
**Fix Required**: Implement `/api/servers/:id/files/:path/content` GET endpoint (text files only)

### FILE-5: No File Editing
**Flow**: Edit server.properties
**Problem**: User must download, edit locally, re-upload
**Impact**: Cumbersome workflow
**Current Code**: File upload exists, no edit
**Fix Required**: Implement file edit API (PUT with content body)

### EXTERNAL-1: Hetzner API Timeout Retry
**Flow**: VM Provisioning Fails
**Problem**: FIX #10 adds queue timeout but no retry logic
**Impact**: Single API timeout fails permanently
**Current Code**: `vm_provisioner.go` - Single attempt
**Fix Required**: Retry Hetzner API calls (max 3 attempts with exponential backoff)

### EXTERNAL-2: Node Loss No Failover
**Flow**: Dedicated Node Crashes
**Problem**: Servers on crashed node stuck, no automatic migration
**Impact**: Manual intervention required
**Current Code**: Health checker marks node unhealthy, no action
**Fix Required**: Auto-migrate servers from unhealthy node to healthy node (after 5min threshold)

### EXTERNAL-3: Storage Box Unavailable
**Flow**: Unarchive Server
**Problem**: If Storage Box down, unarchive fails, no fallback
**Impact**: Server stuck archived, user can't play
**Current Code**: `archive_service.go` - Single SFTP attempt
**Fix Required**: Retry SFTP connection, or cache recent archives locally (last 24h)

### EXTERNAL-4: Database Connection Loss
**Flow**: Database Unavailable
**Problem**: API returns 500 errors, no graceful degradation
**Impact**: Entire platform down
**Current Code**: No connection pooling or retry logic
**Fix Required**: Implement DB connection pooling, retry logic, circuit breaker

### EXTERNAL-5: Network Partition Split-Brain
**Flow**: Conductor ↔ Node Network Failure
**Problem**: Conductor thinks node dead, node thinks conductor dead, both take action
**Impact**: Duplicate servers, resource conflicts
**Current Code**: No distributed consensus
**Fix Required**: Implement consensus protocol (Raft) or external coordinator (etcd)

### MIGRATION-1: No Downtime Notification
**Flow**: Container Migration (Consolidation)
**Problem**: FIX #6 implements migration but no player notification
**Impact**: Players kicked without warning
**Current Code**: `migration_service.go` - Warm-swap but no RCON notify
**Fix Required**: Send RCON `say Migrating server, reconnect in 30s` before migration

### MIGRATION-2: Player Join Cancels Migration
**Flow**: Migration During Player Join
**Problem**: If player joins during migration, what happens?
**Impact**: Undefined behavior, potential data loss
**Current Code**: No player check before migration
**Fix Required**: Check player count, abort migration if players online

### MIGRATION-3: No Migration History
**Flow**: View Migration Logs
**Problem**: Migrations occur but no audit trail
**Impact**: Can't debug migration issues, no accountability
**Current Code**: Migration model exists but not persisted
**Fix Required**: Create `migrations` table, log all migration events

### MIGRATION-4: Consolidation No User Control
**Flow**: Server Auto-Migrated
**Problem**: Consolidation policy exists but user can't opt-out
**Impact**: User's reserved server migrated against their will
**Current Code**: `AllowMigration=true` by default, no UI to change
**Fix Required**: Add "Allow migration" checkbox in server settings, respect user preference

---

## Summary

**Total Issues**: 53
**Critical**: 9 (Security, Data Loss, Billing)
**Medium**: 29 (UX, Business Logic)
**Minor**: 15 (Edge Cases, Polish)

**Top Priority Fixes**:
1. AUTH-1: Block API access for unverified emails
2. SERVER-1: Balance pre-authorization before server creation
3. BILLING-1: Auto-stop servers when balance depleted
4. BACKUP-1: Validate backup integrity before restore
5. BACKUP-2: Implement backup management API to fix quota deadlock
6. BILLING-2: Auto-stop and refund if Minecraft crashes
7. SERVER-2: Block deletion during start/queue states
8. FILE-1: File upload validation (type, size, malware)
9. EXTERNAL-2: Auto-failover for crashed nodes

**Quick Wins** (Low effort, high impact):
- CONFIG-3: Hide RCON password in API responses
- SERVER-5: Server name validation regex
- BILLING-5: Show cost estimate on server creation
- CONFIG-1: Return `restart_required` flag
- SERVER-12: Show queue position and ETA

**Future Enhancements** (Feature additions):
- BILLING-8: Payment gateway integration (Stripe)
- AUTH-6: Email change flow
- FILE-5: In-browser file editing
- BACKUP-4: Scheduled backup cron worker
- MIGRATION-3: Migration audit log
