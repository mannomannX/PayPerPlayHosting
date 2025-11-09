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
	StatusStopped   ServerStatus = "stopped"
	StatusStarting  ServerStatus = "starting"
	StatusRunning   ServerStatus = "running"
	StatusStopping  ServerStatus = "stopping"
	StatusError     ServerStatus = "error"
	StatusSleeping  ServerStatus = "sleeping"  // Phase 2: Container stopped, volume persists
	StatusArchiving ServerStatus = "archiving" // Transitional: Being archived
	StatusArchived  ServerStatus = "archived"  // Phase 3: Compressed and stored remotely
)

// LifecyclePhase represents the server's lifecycle state for billing
type LifecyclePhase string

const (
	PhaseActive   LifecyclePhase = "active"   // Running, full billing
	PhaseSleep    LifecyclePhase = "sleep"    // Stopped < 48h, minimal storage billing
	PhaseArchived LifecyclePhase = "archived" // Stopped > 48h, no billing
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

	// Performance Settings (Phase 2)
	ViewDistance       int `gorm:"default:10"`        // Render distance in chunks (2-32)
	SimulationDistance int `gorm:"default:10"`        // Simulation distance in chunks (3-32, 1.18+ only)

	// World Generation Settings (Phase 2)
	AllowNether        bool   `gorm:"default:true"`     // Enable Nether dimension
	AllowEnd           bool   `gorm:"default:true"`     // Enable End dimension
	GenerateStructures bool   `gorm:"default:true"`     // Generate villages, temples, etc.
	WorldType          string `gorm:"default:default"`  // default, flat, largeBiomes, amplified, buffet, single_biome_surface
	BonusChest         bool   `gorm:"default:false"`    // Spawn with bonus chest
	MaxWorldSize       int    `gorm:"default:29999984"` // World border size in blocks

	// Spawn Settings (Phase 2)
	SpawnProtection int  `gorm:"default:16"`   // Spawn protection radius
	SpawnAnimals    bool `gorm:"default:true"` // Enable animal spawning
	SpawnMonsters   bool `gorm:"default:true"` // Enable monster spawning
	SpawnNPCs       bool `gorm:"default:true"` // Enable villager spawning

	// Network & Performance Settings (Phase 2)
	MaxTickTime                 int `gorm:"default:60000"` // Watchdog timeout in milliseconds
	NetworkCompressionThreshold int `gorm:"default:256"`   // Network compression threshold in bytes

	// Server Description (Phase 4)
	MOTD string `gorm:"size:512;default:'A Minecraft Server'"` // Message of the Day - server description

	// Container Info
	Status      ServerStatus `gorm:"default:stopped"`
	ContainerID string       `gorm:"size:128"`

	// Timestamps
	LastStartedAt *time.Time
	LastStoppedAt *time.Time

	// Lifecycle Management (3-Phase System)
	LifecyclePhase  LifecyclePhase `gorm:"default:active"`      // Current lifecycle phase for billing
	ArchivedAt      *time.Time                                  // When server was archived
	ArchiveLocation string         `gorm:"size:512;default:''"` // Path to archive file (Storage Box)

	// Settings
	IdleTimeoutSeconds   int  `gorm:"default:300"`
	AutoShutdownEnabled  bool `gorm:"default:true"`

	// Velocity Proxy Integration
	VelocityRegistered  bool   `gorm:"default:false"`
	VelocityServerName  string `gorm:"size:128"`

	// RCON Integration for Metrics
	RCONEnabled  bool   `gorm:"default:true"`
	RCONPort     int    `gorm:"default:25575"`
	RCONPassword string `gorm:"size:256;default:'minecraft'"`

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
