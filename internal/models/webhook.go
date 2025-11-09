package models

import (
	"time"
)

// ServerWebhook represents a Discord webhook configuration for a server
type ServerWebhook struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ServerID  uint      `gorm:"not null;index" json:"server_id"`
	Server    *Server   `gorm:"foreignKey:ServerID" json:"-"`
	WebhookURL string   `gorm:"type:text;not null" json:"webhook_url"`
	Enabled   bool      `gorm:"default:true;not null" json:"enabled"`

	// Event filters (which events to send)
	OnServerStart   bool `gorm:"default:true;not null" json:"on_server_start"`
	OnServerStop    bool `gorm:"default:true;not null" json:"on_server_stop"`
	OnServerCrash   bool `gorm:"default:true;not null" json:"on_server_crash"`
	OnPlayerJoin    bool `gorm:"default:true;not null" json:"on_player_join"`
	OnPlayerLeave   bool `gorm:"default:true;not null" json:"on_player_leave"`
	OnBackupCreated bool `gorm:"default:false;not null" json:"on_backup_created"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WebhookEvent represents the type of event being sent to Discord
type WebhookEvent string

const (
	WebhookEventServerStart   WebhookEvent = "server_start"
	WebhookEventServerStop    WebhookEvent = "server_stop"
	WebhookEventServerCrash   WebhookEvent = "server_crash"
	WebhookEventPlayerJoin    WebhookEvent = "player_join"
	WebhookEventPlayerLeave   WebhookEvent = "player_leave"
	WebhookEventBackupCreated WebhookEvent = "backup_created"
)

// DiscordWebhookPayload represents a Discord webhook message
type DiscordWebhookPayload struct {
	Content   string         `json:"content,omitempty"`
	Username  string         `json:"username,omitempty"`
	AvatarURL string         `json:"avatar_url,omitempty"`
	Embeds    []DiscordEmbed `json:"embeds,omitempty"`
}

// DiscordEmbed represents a Discord embed
type DiscordEmbed struct {
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	Color       int                 `json:"color,omitempty"`
	Fields      []DiscordEmbedField `json:"fields,omitempty"`
	Footer      *DiscordEmbedFooter `json:"footer,omitempty"`
	Timestamp   string              `json:"timestamp,omitempty"`
}

// DiscordEmbedField represents a field in a Discord embed
type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// DiscordEmbedFooter represents a footer in a Discord embed
type DiscordEmbedFooter struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url,omitempty"`
}

// WebhookEventData contains event-specific data for webhooks
type WebhookEventData struct {
	ServerID   uint
	ServerName string
	EventType  WebhookEvent
	PlayerName string // for player events
	Message    string // additional context
	Timestamp  time.Time
}
