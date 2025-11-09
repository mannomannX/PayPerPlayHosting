package models

import (
	"time"
)

// ServerBackupSchedule represents automated backup configuration for a server
type ServerBackupSchedule struct {
	ID        uint             `gorm:"primaryKey" json:"id"`
	ServerID  string           `gorm:"size:64;not null;uniqueIndex" json:"server_id"`
	Server    *MinecraftServer `gorm:"foreignKey:ServerID" json:"-"`
	Enabled   bool             `gorm:"default:false;not null" json:"enabled"`

	// Schedule settings
	Frequency      string    `gorm:"size:20;default:'daily';not null" json:"frequency"` // daily, weekly, custom
	ScheduleTime   string    `gorm:"size:5;default:'03:00';not null" json:"schedule_time"` // HH:MM format
	MaxBackups     int       `gorm:"default:7;not null" json:"max_backups"` // Auto-delete old backups

	// Execution tracking
	LastBackupAt   *time.Time `json:"last_backup_at"`
	NextBackupAt   *time.Time `json:"next_backup_at"`
	LastBackupSize string     `json:"last_backup_size,omitempty"`
	FailureCount   int        `gorm:"default:0;not null" json:"failure_count"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
