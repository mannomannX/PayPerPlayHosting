package service

import (
	"sync"
	"time"

	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/internal/velocity"
	"github.com/payperplay/hosting/pkg/logger"
)

// PlayerCountService tracks player counts via Velocity and triggers auto-shutdown
type PlayerCountService struct {
	velocityClient *velocity.RemoteVelocityClient
	serverRepo     *repository.ServerRepository
	checkInterval  time.Duration
	stopChan       chan struct{}
	wg             sync.WaitGroup
}

// NewPlayerCountService creates a new player count tracking service
func NewPlayerCountService(
	velocityClient *velocity.RemoteVelocityClient,
	serverRepo *repository.ServerRepository,
) *PlayerCountService {
	return &PlayerCountService{
		velocityClient: velocityClient,
		serverRepo:     serverRepo,
		checkInterval:  15 * time.Second, // Check every 15 seconds
		stopChan:       make(chan struct{}),
	}
}

// Start begins player count tracking
func (s *PlayerCountService) Start() {
	s.wg.Add(1)
	go s.trackingLoop()
	logger.Info("Player count tracking service started", map[string]interface{}{
		"check_interval": s.checkInterval.String(),
	})
}

// Stop stops the tracking service
func (s *PlayerCountService) Stop() {
	close(s.stopChan)
	s.wg.Wait()
	logger.Info("Player count tracking service stopped", nil)
}

// trackingLoop runs the periodic player count checks
func (s *PlayerCountService) trackingLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	// Initial check
	s.updatePlayerCounts()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.updatePlayerCounts()
		}
	}
}

// updatePlayerCounts fetches player counts from Velocity and updates database
func (s *PlayerCountService) updatePlayerCounts() {
	// Get all running servers
	runningServers, err := s.serverRepo.FindByStatus(string(models.StatusRunning))
	if err != nil {
		logger.Error("Failed to get running servers for player count update", err, nil)
		return
	}

	if len(runningServers) == 0 {
		return
	}

	// Get all servers from Velocity in one call
	velocityServers, err := s.velocityClient.ListServers()
	if err != nil {
		logger.Warn("Failed to get servers from Velocity for player count", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Build map of server name -> player count
	playerCounts := make(map[string]int)
	for _, vs := range velocityServers {
		playerCounts[vs.Name] = vs.Players
	}

	// Update each server's player count
	now := time.Now()
	updated := 0

	for _, server := range runningServers {
		velocityServerName := "mc-" + server.ID

		playerCount, exists := playerCounts[velocityServerName]
		if !exists {
			// Server not registered with Velocity
			continue
		}

		// Check if player count changed
		if playerCount != server.CurrentPlayerCount {
			oldCount := server.CurrentPlayerCount
			server.CurrentPlayerCount = playerCount

			// If players are online, update LastPlayerActivity
			if playerCount > 0 {
				server.LastPlayerActivity = &now
			}

			// Update database
			if err := s.serverRepo.Update(&server); err != nil {
				logger.Warn("Failed to update player count for server", map[string]interface{}{
					"server_id": server.ID,
					"error":     err.Error(),
				})
				continue
			}

			updated++

			logger.Debug("Player count updated", map[string]interface{}{
				"server_id":   server.ID,
				"server_name": server.Name,
				"old_count":   oldCount,
				"new_count":   playerCount,
			})

			// Log when server becomes empty (potential auto-shutdown trigger)
			if oldCount > 0 && playerCount == 0 {
				logger.Info("Server became empty", map[string]interface{}{
					"server_id":             server.ID,
					"server_name":           server.Name,
					"auto_shutdown_enabled": server.AutoShutdownEnabled,
					"idle_timeout_seconds":  server.IdleTimeoutSeconds,
				})
			}

			// Log when first player joins
			if oldCount == 0 && playerCount > 0 {
				logger.Info("First player joined server", map[string]interface{}{
					"server_id":    server.ID,
					"server_name":  server.Name,
					"player_count": playerCount,
				})
			}
		}
	}

	if updated > 0 {
		logger.Debug("Player count update completed", map[string]interface{}{
			"total_servers": len(runningServers),
			"updated":       updated,
		})
	}
}
