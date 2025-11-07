package repository

import (
	"github.com/payperplay/hosting/internal/models"
	"gorm.io/gorm"
)

// FileRepository handles database operations for server files
type FileRepository struct {
	db *gorm.DB
}

// NewFileRepository creates a new file repository
func NewFileRepository(db *gorm.DB) *FileRepository {
	return &FileRepository{db: db}
}

// Create creates a new server file
func (r *FileRepository) Create(file *models.ServerFile) error {
	return r.db.Create(file).Error
}

// FindByID finds a server file by ID
func (r *FileRepository) FindByID(id string) (*models.ServerFile, error) {
	var file models.ServerFile
	err := r.db.Where("id = ?", id).First(&file).Error
	if err != nil {
		return nil, err
	}
	return &file, nil
}

// FindByServerID finds all files for a server
func (r *FileRepository) FindByServerID(serverID string) ([]models.ServerFile, error) {
	var files []models.ServerFile
	err := r.db.Where("server_id = ?", serverID).Order("uploaded_at DESC").Find(&files).Error
	return files, err
}

// FindByServerIDAndType finds files for a server by type
func (r *FileRepository) FindByServerIDAndType(serverID string, fileType models.FileType) ([]models.ServerFile, error) {
	var files []models.ServerFile
	err := r.db.Where("server_id = ? AND file_type = ?", serverID, fileType).
		Order("uploaded_at DESC").
		Find(&files).Error
	return files, err
}

// FindActiveByServerIDAndType finds the active file for a server by type
func (r *FileRepository) FindActiveByServerIDAndType(serverID string, fileType models.FileType) (*models.ServerFile, error) {
	var file models.ServerFile
	err := r.db.Where("server_id = ? AND file_type = ? AND is_active = ?", serverID, fileType, true).
		First(&file).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No active file is not an error
		}
		return nil, err
	}
	return &file, nil
}

// Update updates a server file
func (r *FileRepository) Update(file *models.ServerFile) error {
	return r.db.Save(file).Error
}

// Delete deletes a server file
func (r *FileRepository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&models.ServerFile{}).Error
}

// DeactivateAllOfType deactivates all files of a specific type for a server
// Used when activating a new file (only one can be active at a time)
func (r *FileRepository) DeactivateAllOfType(serverID string, fileType models.FileType) error {
	return r.db.Model(&models.ServerFile{}).
		Where("server_id = ? AND file_type = ?", serverID, fileType).
		Update("is_active", false).Error
}

// UpdateStatus updates the status of a file
func (r *FileRepository) UpdateStatus(id string, status models.FileStatus, errorMessage string) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if errorMessage != "" {
		updates["error_message"] = errorMessage
	}
	return r.db.Model(&models.ServerFile{}).Where("id = ?", id).Updates(updates).Error
}
