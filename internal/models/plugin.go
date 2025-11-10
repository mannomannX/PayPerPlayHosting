package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// PluginSource represents the external source of a plugin
type PluginSource string

const (
	SourceModrinth PluginSource = "modrinth"
	SourceHangar   PluginSource = "hangar"
	SourceSpigot   PluginSource = "spigot"
	SourceManual   PluginSource = "manual"
)

// PluginCategory represents plugin categories
type PluginCategory string

const (
	CategoryWorldManagement PluginCategory = "world-management"
	CategoryAdminTools      PluginCategory = "admin-tools"
	CategoryEconomy         PluginCategory = "economy"
	CategoryMechanics       PluginCategory = "mechanics"
	CategoryProtection      PluginCategory = "protection"
	CategorySocial          PluginCategory = "social"
	CategoryUtility         PluginCategory = "utility"
	CategoryOptimization    PluginCategory = "optimization"
)

// Plugin represents a plugin available in the marketplace
type Plugin struct {
	ID          string         `gorm:"primaryKey;size:64"`
	Name        string         `gorm:"not null;size:255;index"`
	Slug        string         `gorm:"not null;uniqueIndex;size:255"` // "worldedit"
	Description string         `gorm:"type:text"`
	Author      string         `gorm:"size:255"`
	Category    PluginCategory `gorm:"size:100;index"`
	IconURL     string         `gorm:"size:500"`

	// External Source Tracking (for auto-updates)
	Source     PluginSource `gorm:"not null;size:50;index"`
	ExternalID string       `gorm:"not null;size:255;index"` // ID at external source

	// Auto-populated stats
	DownloadCount int     `gorm:"default:0"`
	Rating        float64 `gorm:"default:0"`
	LastSynced    time.Time

	// Timestamps
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// Relations
	Versions []PluginVersion `gorm:"foreignKey:PluginID"`
}

// BeforeCreate hook to generate UUID
func (p *Plugin) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

// PluginVersion represents a specific version of a plugin
type PluginVersion struct {
	ID       string  `gorm:"primaryKey;size:64"`
	PluginID string  `gorm:"not null;index;size:64"`
	Plugin   *Plugin `gorm:"constraint:OnDelete:CASCADE"`

	// Version info
	Version string `gorm:"not null;size:50"` // "7.2.15" (Semantic Versioning)

	// Auto-detected Compatibility (stored as JSON arrays)
	MinecraftVersions datatypes.JSON `gorm:"type:json"` // ["1.20", "1.20.1", "1.20.2"]
	ServerTypes       datatypes.JSON `gorm:"type:json"` // ["paper", "spigot", "purpur"]

	// Dependencies (stored as JSON)
	Dependencies datatypes.JSON `gorm:"type:json"` // Array of Dependency objects

	// Download info (cached from external source)
	DownloadURL string `gorm:"size:500"`
	FileHash    string `gorm:"size:64"` // SHA256 for integrity
	FileSize    int64

	// Metadata
	Changelog   string `gorm:"type:text"`
	ReleaseDate time.Time
	IsStable    bool `gorm:"default:true"` // vs. beta/alpha

	// Timestamps
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// BeforeCreate hook to generate UUID
func (pv *PluginVersion) BeforeCreate(tx *gorm.DB) error {
	if pv.ID == "" {
		pv.ID = uuid.New().String()
	}
	return nil
}

// Dependency represents a plugin dependency
type Dependency struct {
	PluginSlug string `json:"plugin_slug"`
	Required   bool   `json:"required"` // required vs. optional
	MinVersion string `json:"min_version,omitempty"`
}

// InstalledPlugin represents a plugin installed on a specific server
type InstalledPlugin struct {
	ID        string `gorm:"primaryKey;size:64"`
	ServerID  string `gorm:"not null;index;size:64"`
	PluginID  string `gorm:"not null;index;size:64"`
	VersionID string `gorm:"not null;index;size:64"`

	Plugin  *Plugin        `gorm:"constraint:OnDelete:CASCADE"`
	Version *PluginVersion `gorm:"constraint:OnDelete:CASCADE"`

	// State
	Enabled    bool `gorm:"default:true"`
	AutoUpdate bool `gorm:"default:false"` // Auto-update to new compatible versions

	// Timestamps
	InstalledAt time.Time      `gorm:"not null"`
	LastUpdated time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

// BeforeCreate hook to generate UUID and set timestamp
func (ip *InstalledPlugin) BeforeCreate(tx *gorm.DB) error {
	if ip.ID == "" {
		ip.ID = uuid.New().String()
	}
	if ip.InstalledAt.IsZero() {
		ip.InstalledAt = time.Now()
	}
	return nil
}

// TableName specifies the table name
func (Plugin) TableName() string {
	return "plugins"
}

func (PluginVersion) TableName() string {
	return "plugin_versions"
}

func (InstalledPlugin) TableName() string {
	return "installed_plugins"
}
