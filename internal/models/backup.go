package models

import (
	"time"
)

// BackupType represents the type of backup
type BackupType string

const (
	BackupTypeManual         BackupType = "manual"          // User-initiated backup
	BackupTypeScheduled      BackupType = "scheduled"       // Automated scheduled backup (daily/weekly)
	BackupTypePreMigration   BackupType = "pre-migration"   // Backup before container migration
	BackupTypePreDeletion    BackupType = "pre-deletion"    // Backup before server deletion
	BackupTypePreRestore     BackupType = "pre-restore"     // Backup before restoring from another backup
	BackupTypePreUpdate      BackupType = "pre-update"      // Backup before major server update
)

// BackupStatus represents the status of a backup
type BackupStatus string

const (
	BackupStatusPending    BackupStatus = "pending"    // Backup queued
	BackupStatusCreating   BackupStatus = "creating"   // Compression in progress
	BackupStatusUploading  BackupStatus = "uploading"  // Upload to Storage Box in progress
	BackupStatusCompleted  BackupStatus = "completed"  // Backup successful
	BackupStatusFailed     BackupStatus = "failed"     // Backup failed
	BackupStatusDeleted    BackupStatus = "deleted"    // Backup deleted (retention policy)
)

// Backup represents a server backup stored on Hetzner Storage Box
type Backup struct {
	ID        string `gorm:"primaryKey;size:36"`
	CreatedAt time.Time
	UpdatedAt time.Time

	// Server Information
	ServerID   string `gorm:"index;size:36;not null"` // Foreign key to minecraft_servers
	ServerName string `gorm:"size:255"`                // Cached server name for display

	// Backup Metadata
	Type        BackupType   `gorm:"size:50;not null;index"`
	Status      BackupStatus `gorm:"size:50;not null;index"`
	Description string       `gorm:"size:512"` // Optional user description

	// Storage Information
	StoragePath     string `gorm:"size:512;not null"` // Path on Storage Box (e.g., minecraft-backups/{server-id}/manual/2025-11-15.tar.gz)
	CompressedSize  int64  `gorm:"not null"`          // Size of compressed backup in bytes
	OriginalSize    int64  `gorm:"not null"`          // Size before compression in bytes
	CompressionTime int    `gorm:"not null"`          // Time taken to compress (seconds)
	UploadTime      int    `gorm:"not null"`          // Time taken to upload (seconds)

	// Retention Policy
	RetentionDays int        `gorm:"not null;default:7"` // Days to keep backup (0 = keep forever)
	ExpiresAt     *time.Time `gorm:"index"`              // Auto-calculated expiration date

	// Metadata for Restore Operations
	MinecraftVersion string `gorm:"size:50"`  // Minecraft version at backup time
	ServerType       string `gorm:"size:50"`  // Server type (paper, vanilla, etc.)
	RAMMb            int    `gorm:"not null"` // RAM allocation at backup time

	// Error Information (if status = failed)
	ErrorMessage string `gorm:"size:1024"`

	// Audit Trail
	UserID        *string    `gorm:"size:36"` // User who triggered backup (nil for automated)
	CompletedAt   *time.Time                  // When backup was completed
	RestoredAt    *time.Time                  // When backup was last restored
	RestoredCount int        `gorm:"default:0"` // How many times backup was restored
}

// TableName specifies the table name
func (Backup) TableName() string {
	return "backups"
}

// IsExpired checks if the backup has expired according to retention policy
func (b *Backup) IsExpired() bool {
	if b.RetentionDays == 0 {
		return false // Keep forever
	}
	if b.ExpiresAt == nil {
		return false // Not set yet
	}
	return time.Now().After(*b.ExpiresAt)
}

// CalculateExpiresAt calculates and returns the expiration date based on retention days
func (b *Backup) CalculateExpiresAt() time.Time {
	if b.RetentionDays == 0 {
		return time.Time{} // Keep forever (zero time)
	}
	return b.CreatedAt.Add(time.Duration(b.RetentionDays) * 24 * time.Hour)
}

// GetCompressionRatio returns the compression ratio as a percentage
func (b *Backup) GetCompressionRatio() float64 {
	if b.OriginalSize == 0 {
		return 0
	}
	return (1.0 - float64(b.CompressedSize)/float64(b.OriginalSize)) * 100
}
