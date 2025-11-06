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
	Balance   float64   `gorm:"type:decimal(10,2);default:0" json:"balance"`
	IsActive  bool      `gorm:"default:true" json:"is_active"`
	IsAdmin   bool      `gorm:"default:false" json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// OAuth fields (for future Discord/Microsoft integration)
	DiscordID   string `gorm:"size:50;uniqueIndex" json:"discord_id,omitempty"`
	MicrosoftID string `gorm:"size:255;uniqueIndex" json:"microsoft_id,omitempty"`

	// Relationships
	Servers []MinecraftServer `gorm:"foreignKey:OwnerID" json:"servers,omitempty"`
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

// Custom errors
var (
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrEmailAlreadyExists  = errors.New("email already registered")
)
