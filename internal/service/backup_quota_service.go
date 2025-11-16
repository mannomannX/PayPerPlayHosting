package service

import (
	"fmt"
	"time"

	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/logger"
)

// BackupQuotaService handles backup quota and limit enforcement
type BackupQuotaService struct {
	backupRepo         *repository.BackupRepository
	restoreTrackingRepo *repository.BackupRestoreTrackingRepository
	userRepo           *repository.UserRepository
}

// NewBackupQuotaService creates a new backup quota service
func NewBackupQuotaService(
	backupRepo *repository.BackupRepository,
	restoreTrackingRepo *repository.BackupRestoreTrackingRepository,
	userRepo *repository.UserRepository,
) *BackupQuotaService {
	return &BackupQuotaService{
		backupRepo:         backupRepo,
		restoreTrackingRepo: restoreTrackingRepo,
		userRepo:           userRepo,
	}
}

// CanCreateBackup checks if a user can create a manual backup based on daily quota
func (s *BackupQuotaService) CanCreateBackup(userID string, backupType models.BackupType) (bool, string, error) {
	// Only enforce limits for manual backups
	if backupType != models.BackupTypeManual {
		return true, "", nil
	}

	// Get user
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return false, "", fmt.Errorf("failed to find user: %w", err)
	}

	// Check daily backup limit
	today := time.Now()
	startOfDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())

	var count int64
	// Count manual backups created today
	backups, err := s.backupRepo.FindByUserID(userID)
	if err != nil {
		return false, "", fmt.Errorf("failed to count backups: %w", err)
	}

	// Count manual backups created today
	for _, backup := range backups {
		if backup.Type == models.BackupTypeManual && backup.CreatedAt.After(startOfDay) {
			count++
		}
	}

	if int(count) >= user.MaxBackupsPerDay {
		return false, fmt.Sprintf("Daily backup limit reached (%d/%d). Upgrade to Premium for more backups.", count, user.MaxBackupsPerDay), nil
	}

	// Check storage quota
	canStore, storageMsg, err := s.CanStoreBackup(userID)
	if err != nil {
		return false, "", err
	}
	if !canStore {
		return false, storageMsg, nil
	}

	return true, "", nil
}

// CanStoreBackup checks if user has enough storage quota for another backup
func (s *BackupQuotaService) CanStoreBackup(userID string) (bool, string, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return false, "", fmt.Errorf("failed to find user: %w", err)
	}

	// 0 means unlimited
	if user.MaxBackupStorageGB == 0 {
		return true, "", nil
	}

	// Calculate current storage usage
	backups, err := s.backupRepo.FindByUserID(userID)
	if err != nil {
		return false, "", fmt.Errorf("failed to get backups: %w", err)
	}

	var totalSizeBytes int64
	for _, backup := range backups {
		if backup.Status == models.BackupStatusCompleted {
			totalSizeBytes += backup.CompressedSize
		}
	}

	totalSizeGB := float64(totalSizeBytes) / 1024 / 1024 / 1024

	if totalSizeGB >= float64(user.MaxBackupStorageGB) {
		return false, fmt.Sprintf("Storage quota exceeded (%.2f/% dGB). Please delete old backups or upgrade your plan.", totalSizeGB, user.MaxBackupStorageGB), nil
	}

	return true, "", nil
}

// CanRestoreBackup checks if user can restore a backup based on monthly quota
func (s *BackupQuotaService) CanRestoreBackup(userID string) (bool, string, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return false, "", fmt.Errorf("failed to find user: %w", err)
	}

	// 0 means unlimited (Premium/Enterprise plans)
	if user.MaxRestoresPerMonth == 0 {
		return true, "", nil
	}

	// Count restores this month
	restoreCount, err := s.restoreTrackingRepo.GetRestoreCountForMonth(userID)
	if err != nil {
		return false, "", fmt.Errorf("failed to count restores: %w", err)
	}

	if int(restoreCount) >= user.MaxRestoresPerMonth {
		return false, fmt.Sprintf("Monthly restore limit reached (%d/%d). Upgrade to Premium for unlimited restores.", restoreCount, user.MaxRestoresPerMonth), nil
	}

	return true, "", nil
}

// TrackRestore records a restore operation for quota tracking
func (s *BackupQuotaService) TrackRestore(userID, backupID, serverID, serverName string, backupType models.BackupType) error {
	tracking := &models.BackupRestoreTracking{
		UserID:     userID,
		BackupID:   backupID,
		ServerID:   serverID,
		ServerName: serverName,
		BackupType: string(backupType),
		RestoredAt: time.Now(),
	}

	if err := s.restoreTrackingRepo.Create(tracking); err != nil {
		logger.Error("BACKUP-QUOTA: Failed to track restore", err, map[string]interface{}{
			"user_id":   userID,
			"backup_id": backupID,
		})
		return fmt.Errorf("failed to track restore: %w", err)
	}

	logger.Info("BACKUP-QUOTA: Restore tracked", map[string]interface{}{
		"user_id":     userID,
		"backup_id":   backupID,
		"server_name": serverName,
	})

	return nil
}

// GetUserQuotaInfo returns quota information for a user
func (s *BackupQuotaService) GetUserQuotaInfo(userID string) (map[string]interface{}, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	// Get backup count today
	today := time.Now()
	startOfDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())

	backups, err := s.backupRepo.FindByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get backups: %w", err)
	}

	var backupsToday int64
	var totalSizeBytes int64
	var totalBackups int

	for _, backup := range backups {
		if backup.Status == models.BackupStatusCompleted {
			totalBackups++
			totalSizeBytes += backup.CompressedSize
			if backup.Type == models.BackupTypeManual && backup.CreatedAt.After(startOfDay) {
				backupsToday++
			}
		}
	}

	// Get restore count this month
	restoresThisMonth, err := s.restoreTrackingRepo.GetRestoreCountForMonth(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to count restores: %w", err)
	}

	totalSizeGB := float64(totalSizeBytes) / 1024 / 1024 / 1024

	info := map[string]interface{}{
		"plan": user.BackupPlan,

		// Backup limits
		"backups_today":     backupsToday,
		"max_backups_day":   user.MaxBackupsPerDay,
		"backups_remaining": user.MaxBackupsPerDay - int(backupsToday),

		// Storage limits
		"storage_used_gb":     totalSizeGB,
		"storage_quota_gb":    user.MaxBackupStorageGB,
		"storage_unlimited":   user.MaxBackupStorageGB == 0,
		"total_backups":       totalBackups,

		// Restore limits
		"restores_this_month": restoresThisMonth,
		"max_restores_month":  user.MaxRestoresPerMonth,
		"restores_unlimited":  user.MaxRestoresPerMonth == 0,
		"restores_remaining":  0,
	}

	if user.MaxRestoresPerMonth > 0 {
		info["restores_remaining"] = user.MaxRestoresPerMonth - int(restoresThisMonth)
	} else {
		info["restores_remaining"] = -1 // -1 indicates unlimited
	}

	return info, nil
}
