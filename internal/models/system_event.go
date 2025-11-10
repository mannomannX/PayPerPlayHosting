package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// SystemEvent represents a system-wide event for monitoring and analytics
type SystemEvent struct {
	gorm.Model
	EventID   string         `gorm:"uniqueIndex;size:255" json:"event_id"`
	Type      string         `gorm:"index;size:100" json:"type"`
	Timestamp time.Time      `gorm:"index" json:"timestamp"`
	Source    string         `gorm:"size:100" json:"source"`
	ServerID  string         `gorm:"index;size:255" json:"server_id,omitempty"`
	UserID    string         `gorm:"index;size:255" json:"user_id,omitempty"`
	Data      datatypes.JSON `gorm:"type:jsonb" json:"data"`
}

// TableName overrides the table name
func (SystemEvent) TableName() string {
	return "system_events"
}
