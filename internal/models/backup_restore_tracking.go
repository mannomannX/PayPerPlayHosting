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
