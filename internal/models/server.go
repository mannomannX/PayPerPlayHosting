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
	StatusQueued    ServerStatus = "queued"    // Waiting for node assignment and provisioning
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
	RAMMb            int        `gorm:"not null"` // Booked RAM (what customer pays for)
	ActualRAMMB      int        `gorm:"default:0"` // Actual RAM allocated to container (after proportional overhead deduction)
	MaxPlayers       int        `gorm:"default:20"`
	Port             int        `gorm:"unique"`

	// Tier-Based Scaling & Pricing
	RAMTier      string `gorm:"type:varchar(20);default:small"` // micro, small, medium, large, xlarge, custom
	Plan         string `gorm:"type:varchar(20);default:payperplay"` // payperplay, balanced, reserved
	IsCustomTier bool   `gorm:"default:false"` // True if custom RAM size (not standard tier)

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
	Status      ServerStatus `gorm:"default:queued"` // Default to queued - Conductor will assign node
	ContainerID string       `gorm:"size:128"`
	NodeID      string       `gorm:"size:64"` // Multi-Node: Which node hosts this container (assigned by Conductor)

	// Timestamps
	LastStartedAt *time.Time
	LastStoppedAt *time.Time

	// Lifecycle Management (3-Phase System)
	LifecyclePhase  LifecyclePhase `gorm:"default:active"`      // Current lifecycle phase for billing
	ArchivedAt      *time.Time                                  // When server was archived
	ArchiveLocation string         `gorm:"size:512;default:''"` // Path to archive file (Storage Box)
	ArchiveSize     int64          `gorm:"default:0"`           // Size of archive in bytes

	// Pay-Per-Use Settings
	IdleTimeoutSeconds   int  `gorm:"default:300"`  // Seconds of inactivity before auto-shutdown (default: 5 minutes)
	AutoShutdownEnabled  bool `gorm:"default:true"` // Enable auto-shutdown when no players online
	LastPlayerActivity   *time.Time                // Last time a player was online (for idle tracking)
	CurrentPlayerCount   int  `gorm:"default:0"`    // Current number of players online (cached from Velocity)

	// Cost Optimization Settings (B8)
	CostOptimizationLevel int    `gorm:"default:0"`           // 0=Disabled, 1=Suggestions only, 2=Auto-migrate
	AllowMigration        bool   `gorm:"default:true"`        // Allow server to be migrated for cost optimization
	MigrationMode         string `gorm:"default:only_offline"` // Migration modes: "only_offline", "always", "never"

	// Velocity Proxy Integration
	VelocityRegistered  bool   `gorm:"default:false"`
	VelocityServerName  string `gorm:"size:128"`

	// RCON Integration for Metrics
	RCONEnabled  bool   `gorm:"default:true"`
	RCONPort     int    `gorm:"default:25575"`
	RCONPassword string `gorm:"size:256;default:'minecraft'" json:"-"` // FIX CONFIG-3: Never expose RCON password in API responses

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

// GetRAMMb returns the allocated RAM in MB for this server
// Used by Conductor for state synchronization after restarts
func (s *MinecraftServer) GetRAMMb() int {
	return s.RAMMb
}

// CalculateTier automatically determines and sets the tier based on RAM
func (s *MinecraftServer) CalculateTier() {
	s.RAMTier = ClassifyTier(s.RAMMb)
	s.IsCustomTier = (s.RAMTier == TierCustom)
}

// GetHourlyRate returns the hourly cost for this server
func (s *MinecraftServer) GetHourlyRate() float64 {
	return CalculateHourlyRate(s.RAMTier, s.Plan, s.RAMMb)
}

// GetMonthlyRate returns the estimated monthly cost for this server
func (s *MinecraftServer) GetMonthlyRate() float64 {
	return CalculateMonthlyRate(s.RAMTier, s.Plan, s.RAMMb)
}

// AllowsConsolidation returns whether this server allows consolidation based on tier and plan
func (s *MinecraftServer) AllowsConsolidation() bool {
	// Reserved plan: never consolidate
	if s.Plan == PlanReserved {
		return false
	}

	// Custom tier: no consolidation (inefficient)
	if s.IsCustomTier {
		return false
	}

	// Check tier-specific consolidation rules
	if !AllowConsolidation(s.RAMTier) {
		return false
	}

	// Check explicit user opt-out
	if !s.AllowMigration {
		return false
	}

	return true
}

// GetTierDisplayName returns the human-readable tier name
func (s *MinecraftServer) GetTierDisplayName() string {
	return GetTierDisplayName(s.RAMTier)
}

// GetPlanDisplayName returns the human-readable plan name
func (s *MinecraftServer) GetPlanDisplayName() string {
	return GetPlanDisplayName(s.Plan)
}
