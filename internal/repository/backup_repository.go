package repository

import (
	"time"

	"github.com/payperplay/hosting/internal/models"
	"gorm.io/gorm"
)

// BackupRepository handles database operations for backups
type BackupRepository struct {
	db *gorm.DB
}

// NewBackupRepository creates a new backup repository
func NewBackupRepository(db *gorm.DB) *BackupRepository {
	return &BackupRepository{db: db}
}

// Create creates a new backup record
func (r *BackupRepository) Create(backup *models.Backup) error {
	return r.db.Create(backup).Error
}

// Update updates a backup record
func (r *BackupRepository) Update(backup *models.Backup) error {
	return r.db.Save(backup).Error
}

// FindByID finds a backup by ID
func (r *BackupRepository) FindByID(id string) (*models.Backup, error) {
	var backup models.Backup
	err := r.db.Where("id = ?", id).First(&backup).Error
	if err != nil {
		return nil, err
	}
	return &backup, nil
}

// FindByServerID finds all backups for a server
func (r *BackupRepository) FindByServerID(serverID string) ([]models.Backup, error) {
	var backups []models.Backup
	err := r.db.Where("server_id = ?", serverID).
		Order("created_at DESC").
		Find(&backups).Error
	return backups, err
}

// FindByServerIDAndType finds backups by server and type
func (r *BackupRepository) FindByServerIDAndType(serverID string, backupType models.BackupType) ([]models.Backup, error) {
	var backups []models.Backup
	err := r.db.Where("server_id = ? AND type = ?", serverID, backupType).
		Order("created_at DESC").
		Find(&backups).Error
	return backups, err
}

// FindByStatus finds all backups with a specific status
func (r *BackupRepository) FindByStatus(status models.BackupStatus) ([]models.Backup, error) {
	var backups []models.Backup
	err := r.db.Where("status = ?", status).
		Order("created_at DESC").
		Find(&backups).Error
	return backups, err
}

// FindExpired finds all expired backups that should be deleted
func (r *BackupRepository) FindExpired() ([]models.Backup, error) {
	var backups []models.Backup
	now := time.Now()
	err := r.db.Where("expires_at IS NOT NULL AND expires_at < ? AND status = ?",
		now, models.BackupStatusCompleted).
		Find(&backups).Error
	return backups, err
}

// Delete deletes a backup record
func (r *BackupRepository) Delete(id string) error {
	return r.db.Delete(&models.Backup{}, "id = ?", id).Error
}

// FindByUserID finds all backups for a user
func (r *BackupRepository) FindByUserID(userID string) ([]models.Backup, error) {
	var backups []models.Backup
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&backups).Error
	return backups, err
}

// CountByServerID counts backups for a server
func (r *BackupRepository) CountByServerID(serverID string) (int64, error) {
	var count int64
	err := r.db.Model(&models.Backup{}).
		Where("server_id = ? AND status = ?", serverID, models.BackupStatusCompleted).
		Count(&count).Error
	return count, err
}

// GetTotalBackupSize calculates total storage used by completed backups
func (r *BackupRepository) GetTotalBackupSize() (int64, error) {
	var totalSize int64
	err := r.db.Model(&models.Backup{}).
		Where("status = ?", models.BackupStatusCompleted).
		Select("COALESCE(SUM(compressed_size), 0)").
		Scan(&totalSize).Error
	return totalSize, err
}

// GetServerBackupSize calculates total storage used by a server's backups
func (r *BackupRepository) GetServerBackupSize(serverID string) (int64, error) {
	var totalSize int64
	err := r.db.Model(&models.Backup{}).
		Where("server_id = ? AND status = ?", serverID, models.BackupStatusCompleted).
		Select("COALESCE(SUM(compressed_size), 0)").
		Scan(&totalSize).Error
	return totalSize, err
}

// FindOldestBackupForServer finds the oldest completed backup for a server
func (r *BackupRepository) FindOldestBackupForServer(serverID string) (*models.Backup, error) {
	var backup models.Backup
	err := r.db.Where("server_id = ? AND status = ?", serverID, models.BackupStatusCompleted).
		Order("created_at ASC").
		First(&backup).Error
	if err != nil {
		return nil, err
	}
	return &backup, nil
}

// FindLatestBackupForServer finds the most recent completed backup for a server
func (r *BackupRepository) FindLatestBackupForServer(serverID string) (*models.Backup, error) {
	var backup models.Backup
	err := r.db.Where("server_id = ? AND status = ?", serverID, models.BackupStatusCompleted).
		Order("created_at DESC").
		First(&backup).Error
	if err != nil {
		return nil, err
	}
	return &backup, nil
}

// MarkAsDeleted marks a backup as deleted (soft delete in Storage Box)
func (r *BackupRepository) MarkAsDeleted(id string) error {
	return r.db.Model(&models.Backup{}).
		Where("id = ?", id).
		Update("status", models.BackupStatusDeleted).Error
}
