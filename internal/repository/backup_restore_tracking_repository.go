package repository

import (
	"time"

	"github.com/payperplay/hosting/internal/models"
	"gorm.io/gorm"
)

// BackupRestoreTrackingRepository handles backup restore tracking operations
type BackupRestoreTrackingRepository struct {
	db *gorm.DB
}

// NewBackupRestoreTrackingRepository creates a new backup restore tracking repository
func NewBackupRestoreTrackingRepository(db *gorm.DB) *BackupRestoreTrackingRepository {
	return &BackupRestoreTrackingRepository{db: db}
}

// Create creates a new restore tracking record
func (r *BackupRestoreTrackingRepository) Create(tracking *models.BackupRestoreTracking) error {
	return r.db.Create(tracking).Error
}

// GetRestoreCountForMonth returns the number of restores for a user in the current month
func (r *BackupRestoreTrackingRepository) GetRestoreCountForMonth(userID string) (int64, error) {
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	var count int64
	err := r.db.Model(&models.BackupRestoreTracking{}).
		Where("user_id = ? AND restored_at >= ?", userID, startOfMonth).
		Count(&count).Error

	return count, err
}

// GetRestoreCountForDay returns the number of restores for a user in the current day
func (r *BackupRestoreTrackingRepository) GetRestoreCountForDay(userID string) (int64, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var count int64
	err := r.db.Model(&models.BackupRestoreTracking{}).
		Where("user_id = ? AND restored_at >= ?", userID, startOfDay).
		Count(&count).Error

	return count, err
}

// GetRestoresForUser returns all restores for a user with optional time filter
func (r *BackupRestoreTrackingRepository) GetRestoresForUser(userID string, since *time.Time, limit int) ([]models.BackupRestoreTracking, error) {
	var restores []models.BackupRestoreTracking

	query := r.db.Where("user_id = ?", userID)
	if since != nil {
		query = query.Where("restored_at >= ?", *since)
	}

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Order("restored_at DESC").Find(&restores).Error
	return restores, err
}

// DeleteOldRecords deletes restore tracking records older than the specified duration
func (r *BackupRestoreTrackingRepository) DeleteOldRecords(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	result := r.db.Where("restored_at < ?", cutoff).Delete(&models.BackupRestoreTracking{})
	return result.RowsAffected, result.Error
}
