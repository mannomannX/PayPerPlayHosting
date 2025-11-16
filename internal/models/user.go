package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// User represents a user account
type User struct {
	ID        string    `gorm:"primaryKey;size:36" json:"id"`
	Email     string    `gorm:"uniqueIndex;size:255;not null" json:"email"`
	Password  string    `gorm:"size:255;not null" json:"-"` // Never expose in JSON
	Username  string    `gorm:"size:100" json:"username"`
	Balance   float64   `json:"balance"` // PostgreSQL uses double precision by default
	IsActive  bool      `gorm:"default:true" json:"is_active"`
	IsAdmin   bool      `gorm:"default:false" json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// OAuth fields (for future Discord/Microsoft integration)
	DiscordID   string `gorm:"size:50;uniqueIndex" json:"discord_id,omitempty"`
	MicrosoftID string `gorm:"size:255;uniqueIndex" json:"microsoft_id,omitempty"`

	// Email verification
	EmailVerified          bool       `gorm:"default:false" json:"email_verified"`
	EmailVerificationToken string     `gorm:"size:255" json:"-"` // Never expose in API
	VerificationExpiresAt  *time.Time `json:"-"`

	// Password reset
	PasswordResetToken string     `gorm:"size:255" json:"-"` // Never expose in API
	ResetExpiresAt     *time.Time `json:"-"`

	// Account security
	FailedLoginAttempts int        `gorm:"default:0" json:"-"`
	LockedUntil         *time.Time `json:"-"`
	LastPasswordChange  *time.Time `json:"-"`

	// Backup Plan & Limits
	BackupPlan         string `gorm:"size:20;default:'basic'" json:"backup_plan"` // basic, premium, enterprise
	MaxBackupsPerDay   int    `gorm:"default:3" json:"max_backups_per_day"`       // Max manual backups/day
	MaxRestoresPerMonth int   `gorm:"default:5" json:"max_restores_per_month"`   // Max restores/month (0 = unlimited)
	MaxBackupStorageGB int    `gorm:"default:10" json:"max_backup_storage_gb"`   // Max backup storage quota in GB (0 = unlimited)

	// Relationships - Temporarily commented out for testing
	// Servers        []MinecraftServer `gorm:"foreignKey:OwnerID" json:"servers,omitempty"`
	// TrustedDevices []TrustedDevice   `gorm:"foreignKey:UserID" json:"-"`
	// SecurityEvents []SecurityEvent   `gorm:"foreignKey:UserID" json:"-"`
}

// BeforeCreate hook to generate UUID
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}

// SetPassword hashes and sets the user password
func (u *User) SetPassword(password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return nil
}

// CheckPassword verifies if the provided password is correct
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

// CanAfford checks if user has enough balance
func (u *User) CanAfford(amount float64) bool {
	return u.Balance >= amount
}

// DeductBalance deducts amount from user balance
func (u *User) DeductBalance(amount float64) error {
	if !u.CanAfford(amount) {
		return ErrInsufficientBalance
	}
	u.Balance -= amount
	return nil
}

// AddBalance adds amount to user balance
func (u *User) AddBalance(amount float64) {
	u.Balance += amount
}

// ========================================
// Email Verification Methods
// ========================================

// GenerateVerificationToken creates a new email verification token (expires in 24h)
func (u *User) GenerateVerificationToken() string {
	token := uuid.New().String()
	expiresAt := time.Now().Add(24 * time.Hour)
	u.EmailVerificationToken = token
	u.VerificationExpiresAt = &expiresAt
	return token
}

// IsVerificationTokenValid checks if the verification token is valid and not expired
func (u *User) IsVerificationTokenValid(token string) bool {
	if u.EmailVerificationToken == "" || token == "" {
		return false
	}
	if u.EmailVerificationToken != token {
		return false
	}
	if u.VerificationExpiresAt == nil || time.Now().After(*u.VerificationExpiresAt) {
		return false
	}
	return true
}

// MarkEmailVerified marks the email as verified and clears the verification token
func (u *User) MarkEmailVerified() {
	u.EmailVerified = true
	u.EmailVerificationToken = ""
	u.VerificationExpiresAt = nil
}

// ========================================
// Password Reset Methods
// ========================================

// GeneratePasswordResetToken creates a new password reset token (expires in 1h)
func (u *User) GeneratePasswordResetToken() string {
	token := uuid.New().String()
	expiresAt := time.Now().Add(1 * time.Hour)
	u.PasswordResetToken = token
	u.ResetExpiresAt = &expiresAt
	return token
}

// IsResetTokenValid checks if the reset token is valid and not expired
func (u *User) IsResetTokenValid(token string) bool {
	if u.PasswordResetToken == "" || token == "" {
		return false
	}
	if u.PasswordResetToken != token {
		return false
	}
	if u.ResetExpiresAt == nil || time.Now().After(*u.ResetExpiresAt) {
		return false
	}
	return true
}

// ClearPasswordResetToken clears the password reset token after successful reset
func (u *User) ClearPasswordResetToken() {
	u.PasswordResetToken = ""
	u.ResetExpiresAt = nil
}

// ========================================
// Account Security Methods
// ========================================

// IsLocked checks if the account is currently locked
func (u *User) IsLocked() bool {
	if u.LockedUntil == nil {
		return false
	}
	return time.Now().Before(*u.LockedUntil)
}

// IncrementFailedLogins increments failed login counter and locks account if necessary
func (u *User) IncrementFailedLogins() time.Duration {
	u.FailedLoginAttempts++

	var lockDuration time.Duration
	switch {
	case u.FailedLoginAttempts >= 15:
		lockDuration = 24 * time.Hour // 24 hours after 15 attempts
	case u.FailedLoginAttempts >= 10:
		lockDuration = 1 * time.Hour // 1 hour after 10 attempts
	case u.FailedLoginAttempts >= 5:
		lockDuration = 15 * time.Minute // 15 minutes after 5 attempts
	default:
		return 0 // No lock yet
	}

	lockUntil := time.Now().Add(lockDuration)
	u.LockedUntil = &lockUntil
	return lockDuration
}

// ResetFailedLogins resets the failed login counter after successful login
func (u *User) ResetFailedLogins() {
	u.FailedLoginAttempts = 0
	u.LockedUntil = nil
}

// UpdatePasswordChanged sets the last password change timestamp
func (u *User) UpdatePasswordChanged() {
	now := time.Now()
	u.LastPasswordChange = &now
}

// Custom errors
var (
	ErrInsufficientBalance      = errors.New("insufficient balance")
	ErrInvalidCredentials       = errors.New("invalid email or password")
	ErrEmailAlreadyExists       = errors.New("email already registered")
	ErrInvalidVerificationToken = errors.New("invalid or expired verification token")
	ErrEmailAlreadyVerified     = errors.New("email already verified")
	ErrInvalidResetToken        = errors.New("invalid or expired password reset token")
	ErrAccountLocked            = errors.New("account is locked due to too many failed login attempts")
	ErrEmailNotVerified         = errors.New("please verify your email before logging in")
)
