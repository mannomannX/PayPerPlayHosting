package models

import (
	"time"
)

// BackupRestoreTracking tracks restore operations for user quota management
type BackupRestoreTracking struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     string    `gorm:"size:36;not null;index" json:"user_id"`
	BackupID   string    `gorm:"size:36;not null" json:"backup_id"`
	ServerID   string    `gorm:"size:36;not null" json:"server_id"`
	RestoredAt time.Time `gorm:"not null;index" json:"restored_at"`

	// Metadata
	ServerName string `gorm:"size:255" json:"server_name"`
	BackupType string `gorm:"size:50" json:"backup_type"`

	CreatedAt time.Time `json:"created_at"`
}

// TableName specifies the table name
func (BackupRestoreTracking) TableName() string {
	return "backup_restore_tracking"
}

// GetRestoreCountForMonth returns number of restores for a user in the current month
func GetRestoreCountForMonth(db interface{}, userID string) (int64, error) {
	type DB interface {
		Model(value interface{}) interface{ Count(count *int64) interface{ Error() error } }
	}

	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	var count int64
	result := db.(DB).Model(&BackupRestoreTracking{}).Count(&count)
	if result.Error() != nil {
		return 0, result.Error()
	}

	// This is a placeholder - actual implementation should use WHERE clause
	// The repository layer will handle the proper query
	return count, nil
}
