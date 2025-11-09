package models

import "time"

// ServerTemplate defines a pre-configured server template
type ServerTemplate struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Category    string                 `json:"category"` // vanilla, modded, minigame, etc.
	Icon        string                 `json:"icon"`     // emoji or icon class
	Version     string                 `json:"version"`
	ServerType  string                 `json:"serverType"` // vanilla, paper, fabric, forge, spigot
	Memory      int                    `json:"memory"`     // Recommended RAM in MB
	Properties  map[string]interface{} `json:"properties"` // server.properties overrides
	Plugins     []string               `json:"plugins"`    // List of plugin IDs to pre-install
	Mods        []string               `json:"mods"`       // List of mod IDs to pre-install
	WorldPreset string                 `json:"worldPreset,omitempty"` // flat, void, skyblock, etc.
	Tags        []string               `json:"tags"`       // searchable tags
	Popular     bool                   `json:"popular"`    // Featured template
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
}

// TemplateCategory represents a template category
type TemplateCategory struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	Order       int      `json:"order"`
	Templates   []string `json:"templates"` // List of template IDs
}
