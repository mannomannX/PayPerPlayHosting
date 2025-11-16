# Backup System Implementation Plan

## ✅ Completed

### 1. Models Extended
- ✅ `User` model - Added backup plan fields:
  - `BackupPlan` (basic/premium/enterprise)
  - `MaxBackupsPerDay` (default: 3)
  - `MaxRestoresPerMonth` (default: 5, 0 = unlimited)
  - `MaxBackupStorageGB` (default: 10, 0 = unlimited)

- ✅ `BackupRestoreTracking` model - Created for tracking restores:
  - UserID, BackupID, ServerID, RestoredAt
  - Enables monthly restore quota enforcement

- ✅ `Backup` model - Added index to `UserID` field

### 2. Repositories Created
- ✅ `BackupRestoreTrackingRepository` - Track restore operations
  - `GetRestoreCountForMonth(userID)`
  - `GetRestoreCountForDay(userID)`
  - `Create(tracking)`

- ✅ `BackupRepository` - Extended with:
  - `FindByUserID(userID)` - Get all backups for a user

### 3. Services Created
- ✅ `BackupQuotaService` - Quota enforcement:
  - `CanCreateBackup()` - Check daily backup quota
  - `CanRestoreBackup()` - Check monthly restore quota
  - `CanStoreBackup()` - Check storage quota
  - `TrackRestore()` - Record restore operation
  - `GetUserQuotaInfo()` - Get quota stats for user

## 🚧 In Progress

### 4. Integrate Quota Checks into BackupService
Need to modify:
- `CreateBackup()` - Add quota check before creating manual backups
- `RestoreBackup()` - Add restore quota check and tracking

### 5. Initialize Services in main.go
Need to add:
```go
// Initialize BackupRestoreTrackingRepository
restoreTrackingRepo := repository.NewBackupRestoreTrackingRepository(db)

// Initialize BackupQuotaService
backupQuotaService := service.NewBackupQuotaService(backupRepo, restoreTrackingRepo, userRepo)

// Pass to BackupService
backupService := service.NewBackupService(backupRepo, serverRepo, cfg, backupQuotaService)
```

## 📋 Remaining Tasks

### 6. API Endpoints
Create in `internal/api/backup_handlers.go`:
- `GET /api/users/:id/backups` - List user's backups
- `GET /api/users/:id/backups/quota` - Get quota info
- `POST /api/backups/:id/restore` - Restore backup (with quota check)

### 7. Dashboard UI
Create `dashboard/src/pages/BackupsPage.tsx`:
- Backup list table with:
  - Backup name, type, size, created date
  - Server name, Minecraft version
  - Restore button
- Quota info card:
  - Backups today (X/3)
  - Storage used (X.X/10 GB)
  - Restores this month (X/5)
  - Plan upgrade button

### 8. Database Migration
Run on production:
```sql
-- Add backup plan fields to users table
ALTER TABLE users
  ADD COLUMN backup_plan VARCHAR(20) DEFAULT 'basic',
  ADD COLUMN max_backups_per_day INT DEFAULT 3,
  ADD COLUMN max_restores_per_month INT DEFAULT 5,
  ADD COLUMN max_backup_storage_gb INT DEFAULT 10;

-- Add index to backups.user_id
CREATE INDEX IF NOT EXISTS idx_backups_user_id ON backups(user_id);

-- Create backup_restore_tracking table
CREATE TABLE IF NOT EXISTS backup_restore_tracking (
  id SERIAL PRIMARY KEY,
  user_id VARCHAR(36) NOT NULL,
  backup_id VARCHAR(36) NOT NULL,
  server_id VARCHAR(36) NOT NULL,
  server_name VARCHAR(255),
  backup_type VARCHAR(50),
  restored_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_restore_tracking_user_id ON backup_restore_tracking(user_id);
CREATE INDEX idx_restore_tracking_restored_at ON backup_restore_tracking(restored_at);
```

## 📊 Plan Tiers

### Basic Plan (Default)
- ✅ 3 manual backups/day
- ✅ 5 restores/month
- ✅ 10 GB backup storage
- ✅ Scheduled backups (7 days retention)

### Premium Plan (+€1.50/month)
- ✅ **Unlimited** manual backups/day
- ✅ **Unlimited** restores/month
- ✅ 50 GB backup storage
- ✅ Scheduled backups (30 days retention)
- ✅ Hourly backups (24h retention)

### Enterprise Plan (+€5/month)
- ✅ Premium features +
- ✅ **Unlimited** storage
- ✅ Geo-redundant backups
- ✅ Point-in-time restore

## 🔄 Upgrade Logic

```typescript
// Set plan in database
UPDATE users SET
  backup_plan = 'premium',
  max_backups_per_day = 0,      // 0 = unlimited
  max_restores_per_month = 0,   // 0 = unlimited
  max_backup_storage_gb = 50
WHERE id = 'user-id';
```

## 🎯 Next Steps

1. ✅ Complete BackupService integration
2. ✅ Initialize services in main.go
3. ✅ Create API endpoints
4. ✅ Build dashboard UI
5. ✅ Run database migration
6. ✅ Test quota enforcement
7. ✅ Deploy to production
