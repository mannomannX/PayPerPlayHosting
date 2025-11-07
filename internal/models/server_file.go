package models

import (
	"time"

	"gorm.io/gorm"
)

// FileType represents the type of server file
type FileType string

const (
	FileTypeResourcePack FileType = "resource_pack"
	FileTypeDataPack     FileType = "data_pack"
	FileTypeServerIcon   FileType = "server_icon"
	FileTypeWorldGen     FileType = "world_gen"
)

// FileStatus represents the status of a file
type FileStatus string

const (
	FileStatusUploading FileStatus = "uploading"
	FileStatusProcessing FileStatus = "processing"
	FileStatusActive    FileStatus = "active"
	FileStatusInactive  FileStatus = "inactive"
	FileStatusFailed    FileStatus = "failed"
)

// ServerFile represents an uploaded file for a Minecraft server
type ServerFile struct {
	gorm.Model
	ID       string `gorm:"primaryKey;size:64"`
	ServerID string `gorm:"not null;index"`

	// File info
	FileType FileType   `gorm:"not null;index"`
	FileName string     `gorm:"not null"`
	FilePath string     `gorm:"not null"` // Relative path from server directory
	Status   FileStatus `gorm:"not null;default:uploading"`

	// Validation
	SHA1Hash string  `gorm:"size:40"` // SHA1 hash for verification
	SizeMB   float64 `gorm:"not null"`

	// Versioning
	Version  int  `gorm:"default:1"`
	IsActive bool `gorm:"default:false"` // Only one file per type can be active

	// Metadata (JSON)
	// For resource packs: {"require_pack": true, "pack_format": 15}
	// For data packs: {"pack_format": 10, "description": "Custom loot"}
	// For world gen: {"dimensions": ["custom_nether"], "biomes": [...]}
	Metadata string `gorm:"type:text"`

	// Audit
	UploadedBy string    `gorm:"not null;index"`
	UploadedAt time.Time `gorm:"not null"`

	// Error tracking
	ErrorMessage string `gorm:"type:text"`

	// Relations
	Server MinecraftServer `gorm:"foreignKey:ServerID;references:ID"`
}

// TableName specifies the table name for ServerFile
func (ServerFile) TableName() string {
	return "server_files"
}

// FileMetadata represents type-specific metadata
type FileMetadata struct {
	// Resource Pack metadata
	RequirePack bool   `json:"require_pack,omitempty"`
	PackFormat  int    `json:"pack_format,omitempty"`
	Description string `json:"description,omitempty"`

	// Data Pack metadata
	DataPackFormat int      `json:"data_pack_format,omitempty"`
	Namespaces     []string `json:"namespaces,omitempty"`

	// World Gen metadata
	Dimensions []string `json:"dimensions,omitempty"`
	Biomes     []string `json:"biomes,omitempty"`

	// Icon metadata
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
}
