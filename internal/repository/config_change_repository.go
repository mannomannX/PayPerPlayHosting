package repository

import (
	"github.com/payperplay/hosting/internal/models"
	"gorm.io/gorm"
)

type ConfigChangeRepository struct {
	db *gorm.DB
}

func NewConfigChangeRepository(db *gorm.DB) *ConfigChangeRepository {
	return &ConfigChangeRepository{db: db}
}

// Create creates a new config change record
func (r *ConfigChangeRepository) Create(change *models.ConfigChange) error {
	return r.db.Create(change).Error
}

// Update updates a config change record
func (r *ConfigChangeRepository) Update(change *models.ConfigChange) error {
	return r.db.Save(change).Error
}

// FindByID finds a config change by ID
func (r *ConfigChangeRepository) FindByID(id string) (*models.ConfigChange, error) {
	var change models.ConfigChange
	err := r.db.Where("id = ?", id).First(&change).Error
	return &change, err
}

// FindByServerID finds all config changes for a server
func (r *ConfigChangeRepository) FindByServerID(serverID string) ([]models.ConfigChange, error) {
	var changes []models.ConfigChange
	err := r.db.Where("server_id = ?", serverID).Order("created_at DESC").Find(&changes).Error
	return changes, err
}

// FindByUserID finds all config changes by a user
func (r *ConfigChangeRepository) FindByUserID(userID string) ([]models.ConfigChange, error) {
	var changes []models.ConfigChange
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&changes).Error
	return changes, err
}
