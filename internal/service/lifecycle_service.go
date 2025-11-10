package service

import (
	"time"

	"github.com/payperplay/hosting/internal/events"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/logger"
	"gorm.io/gorm"
)

// LifecycleService manages the 3-phase server lifecycle
type LifecycleService struct {
	db         *gorm.DB
	serverRepo *repository.ServerRepository
	stopChan   chan struct{}
}

// NewLifecycleService creates a new lifecycle service
func NewLifecycleService(db *gorm.DB, serverRepo *repository.ServerRepository) *LifecycleService {
	return &LifecycleService{
		db:         db,
		serverRepo: serverRepo,
		stopChan:   make(chan struct{}),
	}
}

// Start begins the lifecycle management workers
func (s *LifecycleService) Start() {
	logger.Info("Starting lifecycle service", nil)

	// Start Sleep Worker (runs every 5 minutes)
	go s.sleepWorker(5 * time.Minute)

	// Future: Archive Worker will run here too
	// go s.archiveWorker(1 * time.Hour)

	logger.Info("Lifecycle service started", nil)
}

// Stop stops all lifecycle workers
func (s *LifecycleService) Stop() {
	logger.Info("Stopping lifecycle service", nil)
	close(s.stopChan)
}

// sleepWorker transitions stopped servers to sleep phase
func (s *LifecycleService) sleepWorker(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run immediately on start
	s.processSleepTransitions()

	for {
		select {
		case <-ticker.C:
			s.processSleepTransitions()
		case <-s.stopChan:
			return
		}
	}
}

// processSleepTransitions finds stopped servers and moves them to sleep phase
func (s *LifecycleService) processSleepTransitions() {
	// Find servers that are:
	// 1. Status = stopped
	// 2. LastStoppedAt > 5 minutes ago
	// 3. LifecyclePhase != sleep (not already sleeping)

	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)

	var servers []models.MinecraftServer
	err := s.db.Where("status = ? AND lifecycle_phase != ? AND last_stopped_at IS NOT NULL AND last_stopped_at < ?",
		models.StatusStopped,
		models.PhaseSleep,
		fiveMinutesAgo,
	).Find(&servers).Error

	if err != nil {
		logger.Error("Failed to find servers for sleep transition", err, nil)
		return
	}

	if len(servers) == 0 {
		logger.Debug("No servers to transition to sleep", nil)
		return
	}

	transitioned := 0
	for _, server := range servers {
		oldPhase := server.LifecyclePhase

		// Update status and lifecycle phase
		updates := map[string]interface{}{
			"status":          models.StatusSleeping,
			"lifecycle_phase": models.PhaseSleep,
		}

		err := s.db.Model(&server).Updates(updates).Error
		if err != nil {
			logger.Error("Failed to transition server to sleep", err, map[string]interface{}{
				"server_id": server.ID,
				"server_name": server.Name,
			})
			continue
		}

		// Publish phase change event for billing tracking
		events.PublishBillingPhaseChanged(server.ID, string(oldPhase), string(models.PhaseSleep))

		transitioned++
		logger.Info("Server transitioned to sleep", map[string]interface{}{
			"server_id":   server.ID,
			"server_name": server.Name,
			"stopped_at":  server.LastStoppedAt,
		})
	}

	if transitioned > 0 {
		logger.Info("Sleep worker completed", map[string]interface{}{
			"transitioned": transitioned,
			"total_found":  len(servers),
		})
	}
}

// WakeFromSleep wakes a sleeping server back to stopped state (ready to start)
func (s *LifecycleService) WakeFromSleep(serverID string) error {
	server, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return err
	}

	if server.LifecyclePhase != models.PhaseSleep {
		logger.Warn("Server is not in sleep phase", map[string]interface{}{
			"server_id": serverID,
			"phase":     server.LifecyclePhase,
		})
		return nil // Not an error, just already awake
	}

	// Transition back to active phase, stopped status
	updates := map[string]interface{}{
		"status":          models.StatusStopped,
		"lifecycle_phase": models.PhaseActive,
	}

	err = s.db.Model(&server).Updates(updates).Error
	if err != nil {
		return err
	}

	logger.Info("Server woken from sleep", map[string]interface{}{
		"server_id":   serverID,
		"server_name": server.Name,
	})

	return nil
}
