package repository

import (
	"github.com/payperplay/hosting/internal/models"
	"gorm.io/gorm"
)

// UserRepository handles database operations for users
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create creates a new user
func (r *UserRepository) Create(user *models.User) error {
	return r.db.Create(user).Error
}

// FindByID finds a user by ID
func (r *UserRepository) FindByID(id string) (*models.User, error) {
	var user models.User
	err := r.db.First(&user, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByEmail finds a user by email
func (r *UserRepository) FindByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.db.First(&user, "email = ?", email).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByDiscordID finds a user by Discord ID
func (r *UserRepository) FindByDiscordID(discordID string) (*models.User, error) {
	var user models.User
	err := r.db.First(&user, "discord_id = ?", discordID).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByMicrosoftID finds a user by Microsoft ID
func (r *UserRepository) FindByMicrosoftID(microsoftID string) (*models.User, error) {
	var user models.User
	err := r.db.First(&user, "microsoft_id = ?", microsoftID).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Update updates a user
func (r *UserRepository) Update(user *models.User) error {
	return r.db.Save(user).Error
}

// Delete deletes a user
func (r *UserRepository) Delete(id string) error {
	return r.db.Delete(&models.User{}, "id = ?", id).Error
}

// FindAll returns all users
func (r *UserRepository) FindAll() ([]models.User, error) {
	var users []models.User
	err := r.db.Find(&users).Error
	return users, err
}

// UpdateBalance updates user balance
func (r *UserRepository) UpdateBalance(userID string, newBalance float64) error {
	return r.db.Model(&models.User{}).Where("id = ?", userID).Update("balance", newBalance).Error
}

// IncrementBalance adds to user balance
func (r *UserRepository) IncrementBalance(userID string, amount float64) error {
	return r.db.Model(&models.User{}).Where("id = ?", userID).
		Update("balance", gorm.Expr("balance + ?", amount)).Error
}

// DecrementBalance subtracts from user balance
func (r *UserRepository) DecrementBalance(userID string, amount float64) error {
	return r.db.Model(&models.User{}).Where("id = ?", userID).
		Update("balance", gorm.Expr("balance - ?", amount)).Error
}
