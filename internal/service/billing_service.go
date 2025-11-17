package service

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/payperplay/hosting/internal/events"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/logger"
	"gorm.io/gorm"
)

// BillingService manages cost calculation and billing events
type BillingService struct {
	db         *gorm.DB
	serverRepo *repository.ServerRepository
	pricing    models.PricingConfig
}

// NewBillingService creates a new billing service
func NewBillingService(db *gorm.DB, serverRepo *repository.ServerRepository) *BillingService {
	return &BillingService{
		db:         db,
		serverRepo: serverRepo,
		pricing:    models.DefaultPricingConfig(),
	}
}

// Start subscribes to Event-Bus for automatic billing tracking
func (s *BillingService) Start() {
	bus := events.GetEventBus()

	// Subscribe to server lifecycle events
	bus.Subscribe(events.EventServerStarted, s.handleServerStarted)
	bus.Subscribe(events.EventServerStopped, s.handleServerStopped)
	bus.Subscribe(events.EventBillingPhaseChanged, s.handlePhaseChanged)

	logger.Info("BillingService subscribed to Event-Bus", nil)
}

// Stop unsubscribes from Event-Bus (cleanup)
func (s *BillingService) Stop() {
	// Event-Bus doesn't support unsubscribe yet, but we log it
	logger.Info("BillingService stopped", nil)
}

// handleServerStarted handles server.started events from Event-Bus
func (s *BillingService) handleServerStarted(event events.Event) {
	// Fetch server details
	server, err := s.serverRepo.FindByID(event.ServerID)
	if err != nil {
		logger.Error("Failed to fetch server for billing", err, map[string]interface{}{
			"server_id": event.ServerID,
		})
		return
	}

	// Record billing event and create usage session
	if err := s.recordServerStartedInternal(server); err != nil {
		logger.Error("Failed to record server start for billing", err, map[string]interface{}{
			"server_id": server.ID,
		})
	}
}

// handleServerStopped handles server.stopped events from Event-Bus
func (s *BillingService) handleServerStopped(event events.Event) {
	// Fetch server details
	server, err := s.serverRepo.FindByID(event.ServerID)
	if err != nil {
		logger.Error("Failed to fetch server for billing", err, map[string]interface{}{
			"server_id": event.ServerID,
		})
		return
	}

	// Record billing event and close usage session
	if err := s.recordServerStoppedInternal(server); err != nil {
		logger.Error("Failed to record server stop for billing", err, map[string]interface{}{
			"server_id": server.ID,
		})
	}
}

// handlePhaseChanged handles billing.phase_changed events from Event-Bus
func (s *BillingService) handlePhaseChanged(event events.Event) {
	// Fetch server details
	server, err := s.serverRepo.FindByID(event.ServerID)
	if err != nil {
		logger.Error("Failed to fetch server for billing", err, map[string]interface{}{
			"server_id": event.ServerID,
		})
		return
	}

	// Extract phase change data
	oldPhaseStr, ok1 := event.Data["old_phase"].(string)
	newPhaseStr, ok2 := event.Data["new_phase"].(string)

	if !ok1 || !ok2 {
		logger.Warn("Invalid phase change event data", map[string]interface{}{
			"event": event,
		})
		return
	}

	oldPhase := models.LifecyclePhase(oldPhaseStr)
	newPhase := models.LifecyclePhase(newPhaseStr)

	if err := s.RecordPhaseChange(server, oldPhase, newPhase); err != nil {
		logger.Error("Failed to record phase change", err, map[string]interface{}{
			"server_id": server.ID,
		})
	}
}

// RecordServerStarted records a server start event and begins a new usage session
// DEPRECATED: This method is kept for backwards compatibility
// Billing events are now automatically created via Event-Bus subscription
func (s *BillingService) RecordServerStarted(server *models.MinecraftServer) error {
	return s.recordServerStartedInternal(server)
}

// recordServerStartedInternal is the internal implementation called by Event-Bus
func (s *BillingService) recordServerStartedInternal(server *models.MinecraftServer) error {
	now := time.Now()

	// Calculate tier-based hourly rate
	hourlyRate := s.getHourlyRateForServer(server)

	// Create billing event
	event := &models.BillingEvent{
		ID:               uuid.New().String(),
		ServerID:         server.ID,
		ServerName:       server.Name,
		OwnerID:          server.OwnerID,
		EventType:        models.EventServerStarted,
		Timestamp:        now,
		RAMMb:            server.RAMMb,
		StorageGB:        0, // TODO: Calculate actual storage usage
		LifecyclePhase:   models.PhaseActive,
		PreviousPhase:    server.LifecyclePhase,
		MinecraftVersion: server.MinecraftVersion,
		HourlyRateEUR:    hourlyRate,
	}

	if err := s.db.Create(event).Error; err != nil {
		return fmt.Errorf("failed to create billing event: %w", err)
	}

	// Create new usage session
	session := &models.UsageSession{
		ID:               uuid.New().String(),
		ServerID:         server.ID,
		ServerName:       server.Name,
		OwnerID:          server.OwnerID,
		StartedAt:        now,
		RAMMb:            server.RAMMb,
		StorageGB:        0,
		MinecraftVersion: server.MinecraftVersion,
		HourlyRateEUR:    hourlyRate,
	}

	if err := s.db.Create(session).Error; err != nil {
		return fmt.Errorf("failed to create usage session: %w", err)
	}

	logger.Debug("Billing: Server started", map[string]interface{}{
		"server_id":   server.ID,
		"server_name": server.Name,
		"ram_mb":      server.RAMMb,
		"tier":        server.RAMTier,
		"plan":        server.Plan,
		"hourly_rate": hourlyRate,
	})

	return nil
}

// RecordServerStopped records a server stop event and closes the usage session
// DEPRECATED: This method is kept for backwards compatibility
// Billing events are now automatically created via Event-Bus subscription
func (s *BillingService) RecordServerStopped(server *models.MinecraftServer) error {
	return s.recordServerStoppedInternal(server)
}

// recordServerStoppedInternal is the internal implementation called by Event-Bus
func (s *BillingService) recordServerStoppedInternal(server *models.MinecraftServer) error {
	now := time.Now()

	// Create billing event
	event := &models.BillingEvent{
		ID:               uuid.New().String(),
		ServerID:         server.ID,
		ServerName:       server.Name,
		OwnerID:          server.OwnerID,
		EventType:        models.EventServerStopped,
		Timestamp:        now,
		RAMMb:            server.RAMMb,
		StorageGB:        0,
		LifecyclePhase:   models.PhaseSleep, // Transitions to sleep
		PreviousPhase:    models.PhaseActive,
		MinecraftVersion: server.MinecraftVersion,
		HourlyRateEUR:    s.pricing.ActiveRateEURPerGBHour,
	}

	if err := s.db.Create(event).Error; err != nil {
		return fmt.Errorf("failed to create billing event: %w", err)
	}

	// Find and close the open usage session
	var session models.UsageSession
	err := s.db.Where("server_id = ? AND stopped_at IS NULL", server.ID).
		Order("started_at DESC").
		First(&session).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Warn("No open session found for server stop", map[string]interface{}{
				"server_id": server.ID,
			})
			return nil
		}
		return fmt.Errorf("failed to find open session: %w", err)
	}

	// Calculate session duration and cost
	session.StoppedAt = &now
	durationSeconds := int(now.Sub(session.StartedAt).Seconds())
	session.DurationSeconds = durationSeconds

	// Cost = (RAM in GB) * (hours) * (hourly rate)
	ramGB := float64(session.RAMMb) / 1024.0
	hours := float64(durationSeconds) / 3600.0
	session.CostEUR = ramGB * hours * session.HourlyRateEUR

	if err := s.db.Save(&session).Error; err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	logger.Info("Billing: Server stopped", map[string]interface{}{
		"server_id":        server.ID,
		"server_name":      server.Name,
		"duration_seconds": durationSeconds,
		"cost_eur":         session.CostEUR,
	})

	return nil
}

// RecordPhaseChange records a lifecycle phase transition
func (s *BillingService) RecordPhaseChange(server *models.MinecraftServer, oldPhase, newPhase models.LifecyclePhase) error {
	event := &models.BillingEvent{
		ID:               uuid.New().String(),
		ServerID:         server.ID,
		ServerName:       server.Name,
		OwnerID:          server.OwnerID,
		EventType:        models.EventPhaseChanged,
		Timestamp:        time.Now(),
		RAMMb:            server.RAMMb,
		StorageGB:        0,
		LifecyclePhase:   newPhase,
		PreviousPhase:    oldPhase,
		MinecraftVersion: server.MinecraftVersion,
		HourlyRateEUR:    s.pricing.ActiveRateEURPerGBHour,
		DailyRateEUR:     s.pricing.SleepRateEURPerGBDay,
	}

	if err := s.db.Create(event).Error; err != nil {
		return fmt.Errorf("failed to create phase change event: %w", err)
	}

	logger.Info("Billing: Phase changed", map[string]interface{}{
		"server_id": server.ID,
		"old_phase": oldPhase,
		"new_phase": newPhase,
	})

	return nil
}

// GetServerCosts calculates the cost summary for a server for the current month
func (s *BillingService) GetServerCosts(serverID string) (*models.CostSummary, error) {
	server, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return nil, fmt.Errorf("server not found: %w", err)
	}

	// Get start of current month
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	summary := &models.CostSummary{
		ServerID:   server.ID,
		ServerName: server.Name,
		OwnerID:    server.OwnerID,
		RAMMb:      server.RAMMb,
		StorageGB:  0, // TODO: Calculate actual storage
	}

	// Calculate active phase costs (completed sessions this month)
	var sessions []models.UsageSession
	err = s.db.Where("server_id = ? AND started_at >= ? AND stopped_at IS NOT NULL", serverID, monthStart).
		Find(&sessions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sessions: %w", err)
	}

	for _, session := range sessions {
		summary.ActiveCostEUR += session.CostEUR
		summary.ActiveSeconds += session.DurationSeconds
	}

	// Add current running session cost (if server is running)
	if server.Status == models.StatusRunning && server.LastStartedAt != nil {
		if server.LastStartedAt.After(monthStart) {
			durationSeconds := int(now.Sub(*server.LastStartedAt).Seconds())
			ramGB := float64(server.RAMMb) / 1024.0
			hours := float64(durationSeconds) / 3600.0
			currentCost := ramGB * hours * s.pricing.ActiveRateEURPerGBHour

			summary.CurrentSessionStartedAt = server.LastStartedAt
			summary.CurrentSessionCostEUR = currentCost
			summary.ActiveCostEUR += currentCost
			summary.ActiveSeconds += durationSeconds
		}
	}

	// Calculate sleep phase costs (stopped but not archived)
	// Cost = Storage (GB) × Days × Daily Rate
	if server.LifecyclePhase == models.PhaseSleep && server.LastStoppedAt != nil {
		sleepStart := server.LastStoppedAt
		if sleepStart.Before(monthStart) {
			sleepStart = &monthStart
		}

		sleepDays := now.Sub(*sleepStart).Hours() / 24.0
		summary.SleepCostEUR = summary.StorageGB * sleepDays * s.pricing.SleepRateEURPerGBDay
		summary.SleepSeconds = int(now.Sub(*sleepStart).Seconds())
	}

	// Archive phase is always free
	summary.ArchiveCostEUR = 0.00

	// Total cost
	summary.TotalCostEUR = summary.ActiveCostEUR + summary.SleepCostEUR + summary.ArchiveCostEUR

	// Forecast next month (simple: current month * 30/days_elapsed)
	daysElapsed := now.Sub(monthStart).Hours() / 24.0
	if daysElapsed > 0 {
		summary.ForecastNextMonthEUR = (summary.TotalCostEUR / daysElapsed) * 30.0
	}

	return summary, nil
}

// GetOwnerCosts calculates total costs for all servers of an owner
func (s *BillingService) GetOwnerCosts(ownerID string) (float64, error) {
	var totalCost float64
	var serverIDs []string

	err := s.db.Model(&models.MinecraftServer{}).Where("owner_id = ?", ownerID).Pluck("id", &serverIDs).Error
	if err != nil {
		return 0, fmt.Errorf("failed to fetch servers: %w", err)
	}

	for _, serverID := range serverIDs {
		summary, err := s.GetServerCosts(serverID)
		if err != nil {
			logger.Error("Failed to get server costs", err, map[string]interface{}{
				"server_id": serverID,
			})
			continue
		}
		totalCost += summary.TotalCostEUR
	}

	return totalCost, nil
}

// GetBillingEvents returns all billing events for a server
func (s *BillingService) GetBillingEvents(serverID string) ([]models.BillingEvent, error) {
	var events []models.BillingEvent
	err := s.db.Where("server_id = ?", serverID).
		Order("timestamp DESC").
		Find(&events).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch billing events: %w", err)
	}

	return events, nil
}

// GetUsageSessions returns all usage sessions for a server
func (s *BillingService) GetUsageSessions(serverID string) ([]models.UsageSession, error) {
	var sessions []models.UsageSession
	err := s.db.Where("server_id = ?", serverID).
		Order("started_at DESC").
		Find(&sessions).Error

	if err != nil {
		return nil, fmt.Errorf("failed to fetch usage sessions: %w", err)
	}

	return sessions, nil
}

// getHourlyRateForServer returns the tier-based hourly rate for a server
// This replaces the legacy flat-rate pricing with tier+plan based pricing
func (s *BillingService) getHourlyRateForServer(server *models.MinecraftServer) float64 {
	// Auto-calculate tier if not set
	if server.RAMTier == "" {
		server.CalculateTier()
	}

	// Use server's tier+plan based rate
	return server.GetHourlyRate()
}

// ===================================
// GAP-3: Billing Zombie Session Cleanup
// ===================================

// CleanupZombieSessions finds and closes billing sessions that are still open
// but the server is no longer running. This fixes GAP-3 (Billing Session Zombies)
// which can occur when:
// - Container crashes and Docker event is lost
// - Node dies and Event Bus is unreachable
// - Code bug or DB transaction failure during session close
func (s *BillingService) CleanupZombieSessions() (int, error) {
	logger.Info("BILLING-CLEANUP: Starting zombie session cleanup", nil)

	// Find all open sessions where the server is not running
	var zombieSessions []models.UsageSession
	err := s.db.Raw(`
		SELECT usage_sessions.*
		FROM usage_sessions
		LEFT JOIN minecraft_servers ON usage_sessions.server_id = minecraft_servers.id
		WHERE usage_sessions.stopped_at IS NULL
		  AND (minecraft_servers.status IS NULL OR minecraft_servers.status != ?)
	`, models.StatusRunning).Scan(&zombieSessions).Error

	if err != nil {
		return 0, fmt.Errorf("failed to find zombie sessions: %w", err)
	}

	if len(zombieSessions) == 0 {
		logger.Debug("BILLING-CLEANUP: No zombie sessions found", nil)
		return 0, nil
	}

	logger.Warn("BILLING-CLEANUP: Found zombie sessions", map[string]interface{}{
		"count": len(zombieSessions),
	})

	closedCount := 0
	now := time.Now()

	for _, session := range zombieSessions {
		// Calculate session duration and cost
		session.StoppedAt = &now
		durationSeconds := int(now.Sub(session.StartedAt).Seconds())
		session.DurationSeconds = durationSeconds

		// Cost = (RAM in GB) * (hours) * (hourly rate)
		ramGB := float64(session.RAMMb) / 1024.0
		hours := float64(durationSeconds) / 3600.0
		session.CostEUR = ramGB * hours * session.HourlyRateEUR

		// Grace period: Max 24h session duration for zombie cleanup
		maxDuration := 24 * time.Hour
		if time.Since(session.StartedAt) > maxDuration {
			logger.Warn("BILLING-CLEANUP: Zombie session exceeded 24h, capping duration", map[string]interface{}{
				"server_id":      session.ServerID,
				"started_at":     session.StartedAt,
				"actual_hours":   hours,
				"capped_hours":   24.0,
			})
			session.DurationSeconds = int(maxDuration.Seconds())
			session.CostEUR = ramGB * 24.0 * session.HourlyRateEUR
		}

		// Update session
		if err := s.db.Save(&session).Error; err != nil {
			logger.Error("BILLING-CLEANUP: Failed to close zombie session", err, map[string]interface{}{
				"session_id": session.ID,
				"server_id":  session.ServerID,
			})
			continue
		}

		closedCount++
		logger.Info("BILLING-CLEANUP: Closed zombie session", map[string]interface{}{
			"session_id":       session.ID,
			"server_id":        session.ServerID,
			"server_name":      session.ServerName,
			"duration_seconds": session.DurationSeconds,
			"cost_eur":         session.CostEUR,
		})
	}

	logger.Info("BILLING-CLEANUP: Zombie session cleanup completed", map[string]interface{}{
		"total_zombies": len(zombieSessions),
		"closed":        closedCount,
		"failed":        len(zombieSessions) - closedCount,
	})

	return closedCount, nil
}

// StartZombieCleanupWorker starts a background worker that periodically cleans up zombie sessions
// Runs every 10 minutes by default
func (s *BillingService) StartZombieCleanupWorker(interval time.Duration) {
	if interval == 0 {
		interval = 10 * time.Minute // Default: 10 minutes
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		logger.Info("BILLING-CLEANUP: Zombie cleanup worker started", map[string]interface{}{
			"interval": interval.String(),
		})

		// Run immediately on startup
		if _, err := s.CleanupZombieSessions(); err != nil {
			logger.Error("BILLING-CLEANUP: Initial zombie cleanup failed", err, nil)
		}

		// Then run on schedule
		for range ticker.C {
			if _, err := s.CleanupZombieSessions(); err != nil {
				logger.Error("BILLING-CLEANUP: Scheduled zombie cleanup failed", err, nil)
			}
		}
	}()
}
