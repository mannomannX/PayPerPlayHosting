package models

import (
	"time"

	"gorm.io/gorm"
)

// ServerType represents the type of Minecraft server
type ServerType string

const (
	ServerTypePaper   ServerType = "paper"
	ServerTypeSpigot  ServerType = "spigot"
	ServerTypeForge   ServerType = "forge"
	ServerTypeFabric  ServerType = "fabric"
	ServerTypeVanilla ServerType = "vanilla"
	ServerTypePurpur  ServerType = "purpur"
)

// ServerStatus represents the current status of a server
type ServerStatus string

const (
	StatusStopped  ServerStatus = "stopped"
	StatusStarting ServerStatus = "starting"
	StatusRunning  ServerStatus = "running"
	StatusStopping ServerStatus = "stopping"
	StatusError    ServerStatus = "error"
)

// MinecraftServer represents a Minecraft server instance
type MinecraftServer struct {
	gorm.Model
	ID string `gorm:"primaryKey;size:64"`

	// Basic Info
	Name    string `gorm:"not null"`
	OwnerID string `gorm:"not null;default:default"` // Future: user system

	// Server Configuration
	ServerType       ServerType `gorm:"not null"`
	MinecraftVersion string     `gorm:"not null"`
	RAMMb            int        `gorm:"not null"`
	MaxPlayers       int        `gorm:"default:20"`
	Port             int        `gorm:"unique"`

	// Gameplay Settings (Phase 1)
	Gamemode           string `gorm:"default:survival"`       // survival, creative, adventure, spectator
	Difficulty         string `gorm:"default:normal"`         // peaceful, easy, normal, hard
	PVP                bool   `gorm:"default:true"`           // Enable PvP
	EnableCommandBlock bool   `gorm:"default:false"`          // Enable command blocks
	LevelSeed          string `gorm:"size:256;default:''"`    // World seed (empty = random)

	// Container Info
	Status      ServerStatus `gorm:"default:stopped"`
	ContainerID string       `gorm:"size:128"`

	// Timestamps
	LastStartedAt *time.Time
	LastStoppedAt *time.Time

	// Settings
	IdleTimeoutSeconds   int  `gorm:"default:300"`
	AutoShutdownEnabled  bool `gorm:"default:true"`

	// Velocity Proxy Integration
	VelocityRegistered  bool   `gorm:"default:false"`
	VelocityServerName  string `gorm:"size:128"`

	// Relations
	UsageLogs []UsageLog `gorm:"foreignKey:ServerID;constraint:OnDelete:CASCADE"`
}

// UsageLog tracks server usage for billing
type UsageLog struct {
	gorm.Model
	ServerID string `gorm:"not null;index"`

	// Timestamps
	StartedAt time.Time
	StoppedAt *time.Time

	// Usage metrics
	DurationSeconds  int     // Calculated on stop
	CostEUR          float64 // Calculated on stop
	PlayerCountPeak  int     `gorm:"default:0"`
	ShutdownReason   string  // "idle", "manual", "crash"

	// Relation
	Server MinecraftServer `gorm:"foreignKey:ServerID;references:ID"`
}

// TableName overrides the table name
func (MinecraftServer) TableName() string {
	return "minecraft_servers"
}

func (UsageLog) TableName() string {
	return "usage_logs"
}
