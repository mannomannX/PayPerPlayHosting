package models

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SecurityEventType represents different types of security events
type SecurityEventType string

const (
	EventLoginSuccess        SecurityEventType = "login_success"
	EventLoginFailure        SecurityEventType = "login_failure"
	EventLoginNewDevice      SecurityEventType = "login_new_device"
	EventPasswordChanged     SecurityEventType = "password_changed"
	EventEmailChanged        SecurityEventType = "email_changed"
	EventAccountLocked       SecurityEventType = "account_locked"
	EventAccountUnlocked     SecurityEventType = "account_unlocked"
	EventAccountDeleted      SecurityEventType = "account_deleted"
	EventEmailVerified       SecurityEventType = "email_verified"
	EventPasswordResetRequest SecurityEventType = "password_reset_request"
	EventPasswordResetSuccess SecurityEventType = "password_reset_success"
)

// TrustedDevice represents a device that the user trusts for 30 days
type TrustedDevice struct {
	gorm.Model
	ID        string    `gorm:"primaryKey;size:64"`
	UserID    string    `gorm:"index;not null;size:36"`
	DeviceID  string    `gorm:"index;not null;size:64"` // SHA256(UserAgent + IP-Range)
	Name      string    `gorm:"size:255"`                // e.g., "Chrome on Windows"
	UserAgent string    `gorm:"size:500"`
	IPAddress string    `gorm:"size:45"` // IPv6 compatible
	LastUsed  time.Time `gorm:"not null"`
	ExpiresAt time.Time `gorm:"not null;index"`
	IsActive  bool      `gorm:"default:true;index"`
}

// BeforeCreate hook to generate UUID
func (td *TrustedDevice) BeforeCreate(tx *gorm.DB) error {
	if td.ID == "" {
		td.ID = uuid.New().String()
	}
	return nil
}

// GenerateDeviceID creates a device fingerprint
func GenerateDeviceID(userAgent, ipAddress string) string {
	// Extract IP range (first 3 octets for IPv4, first 4 groups for IPv6)
	// This allows roaming within same network
	ipRange := extractIPRange(ipAddress)

	data := fmt.Sprintf("%s:%s", userAgent, ipRange)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// extractIPRange extracts network range from IP
func extractIPRange(ip string) string {
	// Simple implementation: take first 3 parts of IPv4
	// For production, use proper IP parsing
	for i := len(ip) - 1; i >= 0; i-- {
		if ip[i] == '.' {
			return ip[:i] + ".0"
		}
	}
	return ip
}

// IsExpired checks if the device trust has expired
func (td *TrustedDevice) IsExpired() bool {
	return time.Now().After(td.ExpiresAt)
}

// Renew extends the trust for another 30 days
func (td *TrustedDevice) Renew() {
	td.LastUsed = time.Now()
	td.ExpiresAt = time.Now().Add(30 * 24 * time.Hour)
}

// SecurityEvent logs security-relevant events
type SecurityEvent struct {
	gorm.Model
	ID        string            `gorm:"primaryKey;size:64"`
	UserID    string            `gorm:"index;not null;size:36"`
	EventType SecurityEventType `gorm:"not null;index"`
	IPAddress string            `gorm:"size:45"`
	UserAgent string            `gorm:"size:500"`
	DeviceID  string            `gorm:"size:64"`
	Location  string            `gorm:"size:255"` // Optional: City, Country
	Success   bool              `gorm:"default:true"`
	Reason    string            `gorm:"size:500"` // Error reason if failed
	Metadata  string            `gorm:"type:json"` // Additional context
	Timestamp time.Time         `gorm:"not null;index"`
}

// BeforeCreate hook to generate UUID and set timestamp
func (se *SecurityEvent) BeforeCreate(tx *gorm.DB) error {
	if se.ID == "" {
		se.ID = uuid.New().String()
	}
	if se.Timestamp.IsZero() {
		se.Timestamp = time.Now()
	}
	// Ensure Metadata is valid JSON if empty
	if se.Metadata == "" {
		se.Metadata = "{}"
	}
	return nil
}
