package models

import (
	"time"

	"gorm.io/gorm"
)

// OAuthProviderType represents different OAuth providers
type OAuthProviderType string

const (
	OAuthProviderDiscord  OAuthProviderType = "discord"
	OAuthProviderGoogle   OAuthProviderType = "google"
	OAuthProviderGitHub   OAuthProviderType = "github"
	OAuthProviderMicrosoft OAuthProviderType = "microsoft"
)

// OAuthAccount represents a linked OAuth account
type OAuthAccount struct {
	gorm.Model
	ID           string            `gorm:"primaryKey;size:64"`
	UserID       string            `gorm:"index;not null;size:36"` // Foreign key to User
	Provider     OAuthProviderType `gorm:"not null;index;size:20"`
	ProviderID   string            `gorm:"not null;size:255"`       // OAuth provider's user ID
	Email        string            `gorm:"size:255"`                // Email from OAuth provider
	Username     string            `gorm:"size:255"`                // Username from OAuth provider
	AvatarURL    string            `gorm:"size:500"`                // Profile picture URL
	AccessToken  string            `gorm:"size:500" json:"-"`       // Never expose
	RefreshToken string            `gorm:"size:500" json:"-"`       // Never expose
	ExpiresAt    *time.Time        `json:"-"`                       // Token expiration
	Scopes       string            `gorm:"size:500"`                // Granted OAuth scopes
	LastUsedAt   time.Time         `gorm:"not null"`

	// Relationship
	User User `gorm:"foreignKey:UserID"`
}

// IsExpired checks if the OAuth token is expired
func (oa *OAuthAccount) IsExpired() bool {
	if oa.ExpiresAt == nil {
		return false // No expiration
	}
	return time.Now().After(*oa.ExpiresAt)
}

// OAuthState represents temporary OAuth state for CSRF protection
type OAuthState struct {
	gorm.Model
	State     string    `gorm:"primaryKey;size:64"`
	Provider  OAuthProviderType `gorm:"not null;size:20"`
	ExpiresAt time.Time `gorm:"not null;index"`
	UserAgent string    `gorm:"size:500"`
	IPAddress string    `gorm:"size:45"`
}

// IsExpired checks if the OAuth state is expired
func (os *OAuthState) IsExpired() bool {
	return time.Now().After(os.ExpiresAt)
}
