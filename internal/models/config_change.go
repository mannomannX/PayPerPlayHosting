package models

import (
	"time"

	"gorm.io/gorm"
)

// ConfigChangeType represents the type of configuration change
type ConfigChangeType string

const (
	ConfigChangeRAM              ConfigChangeType = "ram"
	ConfigChangeVersion          ConfigChangeType = "minecraft_version"
	ConfigChangeServerType       ConfigChangeType = "server_type"
	ConfigChangeServerProperties ConfigChangeType = "server_properties"
	ConfigChangeMaxPlayers       ConfigChangeType = "max_players"
	// Phase 1 Gameplay Settings
	ConfigChangeGamemode          ConfigChangeType = "gamemode"
	ConfigChangeDifficulty        ConfigChangeType = "difficulty"
	ConfigChangePVP               ConfigChangeType = "pvp"
	ConfigChangeCommandBlock      ConfigChangeType = "enable_command_block"
	ConfigChangeLevelSeed         ConfigChangeType = "level_seed"
)

// ConfigChangeStatus represents the status of a configuration change
type ConfigChangeStatus string

const (
	ConfigChangeStatusPending   ConfigChangeStatus = "pending"
	ConfigChangeStatusApplying  ConfigChangeStatus = "applying"
	ConfigChangeStatusCompleted ConfigChangeStatus = "completed"
	ConfigChangeStatusFailed    ConfigChangeStatus = "failed"
	ConfigChangeStatusRolledBack ConfigChangeStatus = "rolled_back"
)

// ConfigChange represents a configuration change with audit trail
type ConfigChange struct {
	gorm.Model
	ID string `gorm:"primaryKey;size:64"`

	// Reference
	ServerID string `gorm:"not null;index"`
	UserID   string `gorm:"not null;index"` // Who made the change

	// Change Details
	ChangeType ConfigChangeType   `gorm:"not null"`
	Status     ConfigChangeStatus `gorm:"not null;default:pending"`

	// Values
	OldValue string `gorm:"type:text"` // JSON or simple value
	NewValue string `gorm:"type:text"` // JSON or simple value

	// Metadata
	RequiresRestart bool   `gorm:"default:false"` // Does this change need container restart?
	ErrorMessage    string `gorm:"type:text"`     // If failed, why?

	// Timestamps
	AppliedAt   *time.Time
	CompletedAt *time.Time
}

// TableName specifies the table name for ConfigChange
func (ConfigChange) TableName() string {
	return "config_changes"
}
