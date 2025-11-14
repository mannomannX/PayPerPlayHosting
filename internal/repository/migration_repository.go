package repository

import (
	"fmt"

	"github.com/payperplay/hosting/internal/models"
	"gorm.io/gorm"
)

// MigrationRepository handles migration database operations
type MigrationRepository struct {
	db *gorm.DB
}

// NewMigrationRepository creates a new migration repository
func NewMigrationRepository(db *gorm.DB) *MigrationRepository {
	return &MigrationRepository{db: db}
}

// Create creates a new migration
func (r *MigrationRepository) Create(migration *models.Migration) error {
	return r.db.Create(migration).Error
}

// FindByID finds a migration by ID
func (r *MigrationRepository) FindByID(id string) (*models.Migration, error) {
	var migration models.Migration
	err := r.db.Where("id = ?", id).First(&migration).Error
	if err != nil {
		return nil, err
	}
	return &migration, nil
}

// FindAll finds all migrations with optional filters
func (r *MigrationRepository) FindAll(filters map[string]interface{}, limit, offset int) ([]models.Migration, error) {
	var migrations []models.Migration
	query := r.db.Model(&models.Migration{})

	// Apply filters
	for key, value := range filters {
		switch key {
		case "status":
			query = query.Where("status = ?", value)
		case "server_id":
			query = query.Where("server_id = ?", value)
		case "reason":
			query = query.Where("reason = ?", value)
		case "triggered_by":
			query = query.Where("triggered_by = ?", value)
		}
	}

	// Apply pagination
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	// Order by created_at DESC
	query = query.Order("created_at DESC")

	err := query.Find(&migrations).Error
	return migrations, err
}

// FindByStatus finds all migrations with a specific status
func (r *MigrationRepository) FindByStatus(status models.MigrationStatus) ([]models.Migration, error) {
	var migrations []models.Migration
	err := r.db.Where("status = ?", status).Order("created_at DESC").Find(&migrations).Error
	return migrations, err
}

// FindByServerID finds all migrations for a specific server
func (r *MigrationRepository) FindByServerID(serverID string) ([]models.Migration, error) {
	var migrations []models.Migration
	err := r.db.Where("server_id = ?", serverID).Order("created_at DESC").Find(&migrations).Error
	return migrations, err
}

// FindActiveMigrationForServer finds the active migration for a server (if any)
func (r *MigrationRepository) FindActiveMigrationForServer(serverID string) (*models.Migration, error) {
	var migration models.Migration
	err := r.db.Where(
		"server_id = ? AND status IN (?)",
		serverID,
		[]models.MigrationStatus{
			models.MigrationStatusPreparing,
			models.MigrationStatusTransferring,
			models.MigrationStatusCompleting,
		},
	).First(&migration).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &migration, nil
}

// FindPendingMigrations finds all migrations that are waiting to be executed
func (r *MigrationRepository) FindPendingMigrations() ([]models.Migration, error) {
	var migrations []models.Migration
	err := r.db.Where("status IN (?)", []models.MigrationStatus{
		models.MigrationStatusScheduled,
		models.MigrationStatusApproved,
	}).Order("created_at ASC").Find(&migrations).Error
	return migrations, err
}

// Update updates a migration
func (r *MigrationRepository) Update(migration *models.Migration) error {
	return r.db.Save(migration).Error
}

// UpdateStatus updates the status of a migration
func (r *MigrationRepository) UpdateStatus(id string, status models.MigrationStatus) error {
	return r.db.Model(&models.Migration{}).Where("id = ?", id).Update("status", status).Error
}

// UpdateProgress updates the progress of a migration
func (r *MigrationRepository) UpdateProgress(id string, progress int) error {
	return r.db.Model(&models.Migration{}).Where("id = ?", id).Update("data_sync_progress", progress).Error
}

// SetError sets an error message for a migration
func (r *MigrationRepository) SetError(id string, errorMessage string) error {
	return r.db.Model(&models.Migration{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":        models.MigrationStatusFailed,
		"error_message": errorMessage,
	}).Error
}

// IncrementRetryCount increments the retry count for a migration
func (r *MigrationRepository) IncrementRetryCount(id string) error {
	return r.db.Model(&models.Migration{}).Where("id = ?", id).UpdateColumn("retry_count", gorm.Expr("retry_count + 1")).Error
}

// Delete deletes a migration (soft delete)
func (r *MigrationRepository) Delete(id string) error {
	return r.db.Delete(&models.Migration{}, "id = ?", id).Error
}

// Count returns the total count of migrations
func (r *MigrationRepository) Count(filters map[string]interface{}) (int64, error) {
	var count int64
	query := r.db.Model(&models.Migration{})

	// Apply filters
	for key, value := range filters {
		switch key {
		case "status":
			query = query.Where("status = ?", value)
		case "server_id":
			query = query.Where("server_id = ?", value)
		case "reason":
			query = query.Where("reason = ?", value)
		}
	}

	err := query.Count(&count).Error
	return count, err
}

// GetStats returns migration statistics
func (r *MigrationRepository) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Count by status
	var statusCounts []struct {
		Status models.MigrationStatus
		Count  int64
	}
	err := r.db.Model(&models.Migration{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&statusCounts).Error
	if err != nil {
		return nil, err
	}

	statusMap := make(map[string]int64)
	for _, sc := range statusCounts {
		statusMap[string(sc.Status)] = sc.Count
	}
	stats["by_status"] = statusMap

	// Total migrations
	var totalCount int64
	r.db.Model(&models.Migration{}).Count(&totalCount)
	stats["total"] = totalCount

	// Success rate
	completed := statusMap[string(models.MigrationStatusCompleted)]
	failed := statusMap[string(models.MigrationStatusFailed)]
	if completed+failed > 0 {
		stats["success_rate"] = float64(completed) / float64(completed+failed) * 100
	} else {
		stats["success_rate"] = 0.0
	}

	// Average duration (for completed migrations)
	var avgDuration float64
	r.db.Model(&models.Migration{}).
		Where("status = ? AND started_at IS NOT NULL AND completed_at IS NOT NULL", models.MigrationStatusCompleted).
		Select("AVG(EXTRACT(EPOCH FROM (completed_at - started_at)))").
		Scan(&avgDuration)
	stats["avg_duration_seconds"] = int(avgDuration)

	// Total savings (completed migrations only)
	var totalSavingsHour float64
	r.db.Model(&models.Migration{}).
		Where("status = ?", models.MigrationStatusCompleted).
		Select("SUM(savings_eur_hour)").
		Scan(&totalSavingsHour)
	stats["total_savings_eur_hour"] = totalSavingsHour
	stats["total_savings_eur_month"] = totalSavingsHour * 730 // ~730 hours/month

	return stats, nil
}

// CleanupOldMigrations deletes migrations older than specified days
func (r *MigrationRepository) CleanupOldMigrations(daysOld int) (int64, error) {
	result := r.db.Unscoped().Where(
		"status IN (?) AND created_at < NOW() - INTERVAL '? days'",
		[]models.MigrationStatus{
			models.MigrationStatusCompleted,
			models.MigrationStatusFailed,
			models.MigrationStatusCancelled,
		},
		daysOld,
	).Delete(&models.Migration{})

	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}

// HasActiveMigration checks if there's an active migration for a server
func (r *MigrationRepository) HasActiveMigration(serverID string) (bool, error) {
	var count int64
	err := r.db.Model(&models.Migration{}).Where(
		"server_id = ? AND status IN (?)",
		serverID,
		[]models.MigrationStatus{
			models.MigrationStatusPreparing,
			models.MigrationStatusTransferring,
			models.MigrationStatusCompleting,
		},
	).Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// FindRecentMigrationForServer finds the most recent migration for a server
func (r *MigrationRepository) FindRecentMigrationForServer(serverID string) (*models.Migration, error) {
	var migration models.Migration
	err := r.db.Where("server_id = ?", serverID).
		Order("created_at DESC").
		First(&migration).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &migration, nil
}

// CanMigrateServer checks if a server can be migrated (no active migration + cooldown)
func (r *MigrationRepository) CanMigrateServer(serverID string, cooldownMinutes int) (bool, error) {
	// Check for active migration
	hasActive, err := r.HasActiveMigration(serverID)
	if err != nil {
		return false, err
	}
	if hasActive {
		return false, fmt.Errorf("server has active migration")
	}

	// Check cooldown period (skip if cooldownMinutes is 0)
	if cooldownMinutes > 0 {
		recent, err := r.FindRecentMigrationForServer(serverID)
		if err != nil {
			return false, err
		}

		if recent != nil && recent.CompletedAt != nil {
			// Check if cooldown period has passed
			var count int64
			query := fmt.Sprintf(
				"server_id = ? AND completed_at > NOW() - INTERVAL '%d minutes'",
				cooldownMinutes,
			)
			err := r.db.Model(&models.Migration{}).Where(
				query,
				serverID,
			).Count(&count).Error

			if err != nil {
				return false, err
			}

			if count > 0 {
				return false, fmt.Errorf("server in cooldown period (%d minutes)", cooldownMinutes)
			}
		}
	}

	return true, nil
}
