package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/logger"
	"gorm.io/gorm"
)

// BackupScheduler handles automated scheduled backups
type BackupScheduler struct {
	db            *gorm.DB
	backupService *BackupService
	backupRepo    *repository.BackupRepository
	serverRepo    *repository.ServerRepository
	ticker        *time.Ticker
	stopChan      chan bool
	mu            sync.Mutex
}

// NewBackupScheduler creates a new backup scheduler
func NewBackupScheduler(db *gorm.DB, backupService *BackupService, backupRepo *repository.BackupRepository, serverRepo *repository.ServerRepository) *BackupScheduler {
	return &BackupScheduler{
		db:            db,
		backupService: backupService,
		backupRepo:    backupRepo,
		serverRepo:    serverRepo,
		stopChan:      make(chan bool),
	}
}

// Start begins the backup scheduler (checks every 5 minutes)
func (s *BackupScheduler) Start() {
	logger.Info("Starting backup scheduler", nil)

	// Check immediately on startup
	go s.processScheduledBackups()

	// Then check every 5 minutes
	s.ticker = time.NewTicker(5 * time.Minute)

	go func() {
		for {
			select {
			case <-s.ticker.C:
				s.processScheduledBackups()
			case <-s.stopChan:
				logger.Info("Stopping backup scheduler", nil)
				return
			}
		}
	}()
}

// Stop stops the backup scheduler
func (s *BackupScheduler) Stop() {
	if s.ticker != nil {
		s.ticker.Stop()
	}
	s.stopChan <- true
}

// processScheduledBackups checks all schedules and creates backups as needed
func (s *BackupScheduler) processScheduledBackups() {
	s.mu.Lock()
	defer s.mu.Unlock()

	var schedules []models.ServerBackupSchedule
	if err := s.db.Where("enabled = ?", true).Find(&schedules).Error; err != nil {
		logger.Error("Failed to fetch backup schedules", err, nil)
		return
	}

	now := time.Now()

	for _, schedule := range schedules {
		// Check if backup is due
		if s.isBackupDue(schedule, now) {
			logger.Info("Creating scheduled backup", map[string]interface{}{
				"server_id":    schedule.ServerID,
				"schedule_id":  schedule.ID,
				"schedule_time": schedule.ScheduleTime,
			})

			// Create backup
			if err := s.createScheduledBackup(schedule); err != nil {
				logger.Error("Failed to create scheduled backup", err, map[string]interface{}{
					"server_id":   schedule.ServerID,
					"schedule_id": schedule.ID,
				})

				// Increment failure count
				s.db.Model(&schedule).Updates(map[string]interface{}{
					"failure_count": schedule.FailureCount + 1,
				})
			} else {
				// Update last backup time and reset failure count
				nextBackup := s.calculateNextBackup(schedule, now)
				s.db.Model(&schedule).Updates(map[string]interface{}{
					"last_backup_at": now,
					"next_backup_at": nextBackup,
					"failure_count":  0,
				})
			}
		}
	}
}

// isBackupDue checks if a backup should be created now
func (s *BackupScheduler) isBackupDue(schedule models.ServerBackupSchedule, now time.Time) bool {
	// If next_backup_at is set and in the future, not due yet
	if schedule.NextBackupAt != nil && schedule.NextBackupAt.After(now) {
		return false
	}

	// If next_backup_at is nil or in the past, check if enough time has passed
	if schedule.LastBackupAt == nil {
		// Never backed up before - create now
		return true
	}

	// Calculate when next backup should be
	nextBackup := s.calculateNextBackup(schedule, *schedule.LastBackupAt)
	return nextBackup.Before(now) || nextBackup.Equal(now)
}

// calculateNextBackup calculates the next backup time based on schedule
func (s *BackupScheduler) calculateNextBackup(schedule models.ServerBackupSchedule, from time.Time) time.Time {
	// Parse schedule time (HH:MM)
	scheduleHour := 3
	scheduleMinute := 0
	fmt.Sscanf(schedule.ScheduleTime, "%d:%d", &scheduleHour, &scheduleMinute)

	// Get next occurrence of schedule time
	nextBackup := time.Date(
		from.Year(), from.Month(), from.Day(),
		scheduleHour, scheduleMinute, 0, 0,
		from.Location(),
	)

	// If the time has already passed today, schedule for tomorrow
	if nextBackup.Before(from) || nextBackup.Equal(from) {
		nextBackup = nextBackup.Add(24 * time.Hour)
	}

	// Handle weekly frequency
	if schedule.Frequency == "weekly" {
		// If more than a day has passed, add days until next week
		for nextBackup.Before(from) {
			nextBackup = nextBackup.Add(7 * 24 * time.Hour)
		}
	}

	return nextBackup
}

// createScheduledBackup creates a backup for a scheduled server
func (s *BackupScheduler) createScheduledBackup(schedule models.ServerBackupSchedule) error {
	// Get server
	server, err := s.serverRepo.FindByID(schedule.ServerID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}
	if server == nil {
		return fmt.Errorf("server not found: %s", schedule.ServerID)
	}

	// Create backup with new signature
	backup, err := s.backupService.CreateBackup(
		schedule.ServerID,
		models.BackupTypeScheduled,
		fmt.Sprintf("Scheduled backup for %s", server.Name),
		nil, // No user ID for automated backups
		0,   // Use default retention (7 days for scheduled)
	)
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	logger.Info("Scheduled backup created", map[string]interface{}{
		"server_id":   schedule.ServerID,
		"server_name": server.Name,
		"backup_id":   backup.ID,
	})

	// Clean up old backups if max_backups is exceeded
	if schedule.MaxBackups > 0 {
		if err := s.cleanupOldBackups(schedule.ServerID, schedule.MaxBackups); err != nil {
			logger.Warn("Failed to cleanup old backups", map[string]interface{}{
				"server_id": schedule.ServerID,
				"error":     err.Error(),
			})
		}
	}

	return nil
}

// cleanupOldBackups removes old backups exceeding the max limit
func (s *BackupScheduler) cleanupOldBackups(serverID string, maxBackups int) error {
	backups, err := s.backupRepo.FindByServerIDAndType(serverID, models.BackupTypeScheduled)
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	// If we have more backups than allowed, delete oldest ones
	if len(backups) > maxBackups {
		// Backups are already sorted by created_at DESC, reverse for oldest first
		// Backups are returned newest first, so reverse
		toDelete := len(backups) - maxBackups
		for i := len(backups) - 1; i >= len(backups)-toDelete; i-- {
			logger.Info("Deleting old backup", map[string]interface{}{
				"server_id": serverID,
				"backup_id": backups[i].ID,
			})

			if err := s.backupService.DeleteBackup(backups[i].ID); err != nil {
				logger.Warn("Failed to delete old backup", map[string]interface{}{
					"server_id": serverID,
					"backup_id": backups[i].ID,
					"error":     err.Error(),
				})
			}
		}
	}

	return nil
}

// GetSchedule returns a backup schedule for a server
func (s *BackupScheduler) GetSchedule(serverID string) (*models.ServerBackupSchedule, error) {
	var schedule models.ServerBackupSchedule
	if err := s.db.Where("server_id = ?", serverID).First(&schedule).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No schedule configured
		}
		return nil, err
	}
	return &schedule, nil
}

// CreateSchedule creates a new backup schedule
func (s *BackupScheduler) CreateSchedule(serverID string, enabled bool, frequency string, scheduleTime string, maxBackups int) (*models.ServerBackupSchedule, error) {
	// Check if schedule already exists
	existing, err := s.GetSchedule(serverID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("backup schedule already exists for this server")
	}

	// Calculate next backup time
	now := time.Now()
	nextBackup := s.calculateNextBackup(models.ServerBackupSchedule{
		Frequency:    frequency,
		ScheduleTime: scheduleTime,
	}, now)

	schedule := &models.ServerBackupSchedule{
		ServerID:     serverID,
		Enabled:      enabled,
		Frequency:    frequency,
		ScheduleTime: scheduleTime,
		MaxBackups:   maxBackups,
		NextBackupAt: &nextBackup,
	}

	if err := s.db.Create(schedule).Error; err != nil {
		return nil, err
	}

	return schedule, nil
}

// UpdateSchedule updates an existing backup schedule
func (s *BackupScheduler) UpdateSchedule(serverID string, updates map[string]interface{}) (*models.ServerBackupSchedule, error) {
	schedule, err := s.GetSchedule(serverID)
	if err != nil {
		return nil, err
	}
	if schedule == nil {
		return nil, fmt.Errorf("backup schedule not found for server %s", serverID)
	}

	if err := s.db.Model(schedule).Updates(updates).Error; err != nil {
		return nil, err
	}

	// Recalculate next backup if schedule time or frequency changed
	if _, hasFreq := updates["frequency"]; hasFreq {
		now := time.Now()
		nextBackup := s.calculateNextBackup(*schedule, now)
		s.db.Model(schedule).Update("next_backup_at", nextBackup)
	}
	if _, hasTime := updates["schedule_time"]; hasTime {
		now := time.Now()
		nextBackup := s.calculateNextBackup(*schedule, now)
		s.db.Model(schedule).Update("next_backup_at", nextBackup)
	}

	return schedule, nil
}

// DeleteSchedule deletes a backup schedule
func (s *BackupScheduler) DeleteSchedule(serverID string) error {
	return s.db.Where("server_id = ?", serverID).Delete(&models.ServerBackupSchedule{}).Error
}
