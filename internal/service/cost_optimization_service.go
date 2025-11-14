package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/payperplay/hosting/internal/conductor"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/logger"
)

// CostOptimizationService analyzes server placements and suggests/performs migrations
type CostOptimizationService struct {
	serverRepo    *repository.ServerRepository
	migrationRepo *repository.MigrationRepository
	conductor     *conductor.Conductor
	checkInterval time.Duration
	stopChan      chan struct{}
	wg            sync.WaitGroup

	// Minimum savings to trigger migration (EUR/hour)
	minSavingsThreshold float64

	// Cooldown period after scaling events
	scalingCooldown  time.Duration
	lastScalingEvent time.Time
	cooldownMu       sync.RWMutex

	// Current suggestions
	currentSuggestions []OptimizationSuggestion
	suggestionsMu      sync.RWMutex
	lastAnalysis       time.Time
}

// OptimizationSuggestion represents a cost-saving opportunity
type OptimizationSuggestion struct {
	ServerID         string    `json:"server_id"`
	ServerName       string    `json:"server_name"`
	CurrentNodeID    string    `json:"current_node_id"`
	CurrentCost      float64   `json:"current_cost_eur_hour"`
	TargetNodeID     string    `json:"target_node_id"`
	TargetCost       float64   `json:"target_cost_eur_hour"`
	SavingsPerHour   float64   `json:"savings_eur_hour"`
	SavingsPerMonth  float64   `json:"savings_eur_month"`
	Reason           string    `json:"reason"`
	CreatedAt        time.Time `json:"created_at"`
	Applied          bool      `json:"applied"`
}

// NewCostOptimizationService creates a new cost optimization service
func NewCostOptimizationService(
	serverRepo *repository.ServerRepository,
	migrationRepo *repository.MigrationRepository,
) *CostOptimizationService {
	return &CostOptimizationService{
		serverRepo:          serverRepo,
		migrationRepo:       migrationRepo,
		checkInterval:       2 * time.Hour, // Check every 2 hours
		minSavingsThreshold: 0.10,          // Minimum €0.10/hour savings
		scalingCooldown:     2 * time.Hour, // Wait 2h after scaling events
		stopChan:            make(chan struct{}),
	}
}

// SetConductor sets the conductor instance
func (s *CostOptimizationService) SetConductor(cond *conductor.Conductor) {
	s.conductor = cond
}

// NotifyScalingEvent notifies the service of a scaling event (to trigger cooldown)
func (s *CostOptimizationService) NotifyScalingEvent() {
	s.cooldownMu.Lock()
	defer s.cooldownMu.Unlock()
	s.lastScalingEvent = time.Now()
	logger.Info("Cost optimization cooldown started after scaling event", map[string]interface{}{
		"cooldown_duration": s.scalingCooldown.String(),
	})
}

// isInCooldown checks if we're in cooldown period
func (s *CostOptimizationService) isInCooldown() bool {
	s.cooldownMu.RLock()
	defer s.cooldownMu.RUnlock()

	if s.lastScalingEvent.IsZero() {
		return false
	}

	return time.Since(s.lastScalingEvent) < s.scalingCooldown
}

// Start begins the optimization analysis loop
func (s *CostOptimizationService) Start() {
	s.wg.Add(1)
	go s.analysisLoop()
	logger.Info("Cost optimization service started", map[string]interface{}{
		"check_interval":    s.checkInterval.String(),
		"min_savings":       s.minSavingsThreshold,
		"scaling_cooldown":  s.scalingCooldown.String(),
	})
}

// Stop stops the optimization service
func (s *CostOptimizationService) Stop() {
	close(s.stopChan)
	s.wg.Wait()
	logger.Info("Cost optimization service stopped", nil)
}

// analysisLoop runs periodic cost analysis
func (s *CostOptimizationService) analysisLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	// Initial analysis after 5 minutes (let system stabilize)
	time.Sleep(5 * time.Minute)
	s.analyzeAndOptimize()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.analyzeAndOptimize()
		}
	}
}

// analyzeAndOptimize performs cost analysis and takes action based on server settings
func (s *CostOptimizationService) analyzeAndOptimize() {
	if s.conductor == nil {
		logger.Warn("Cannot perform cost analysis: Conductor not set", nil)
		return
	}

	// Skip if in cooldown
	if s.isInCooldown() {
		logger.Debug("Cost optimization skipped: in cooldown period", map[string]interface{}{
			"time_remaining": (s.scalingCooldown - time.Since(s.lastScalingEvent)).Round(time.Minute).String(),
		})
		return
	}

	// Skip if system is not stable
	if !s.conductor.IsScalingSystemStable() {
		logger.Debug("Cost optimization skipped: scaling in progress", nil)
		return
	}

	logger.Info("Starting cost optimization analysis...", nil)

	// Get all running servers
	servers, err := s.serverRepo.FindByStatus(string(models.StatusRunning))
	if err != nil {
		logger.Error("Failed to get running servers for cost analysis", err, nil)
		return
	}

	// Get all nodes
	nodes := s.conductor.GetAllNodesForCostAnalysis()
	nodeMap := make(map[string]conductor.CostNodeInfo)
	for _, node := range nodes {
		nodeMap[node.ID] = node
	}

	suggestions := s.analyzeCostOpportunities(servers, nodeMap)

	// Store suggestions for API access
	s.suggestionsMu.Lock()
	s.currentSuggestions = suggestions
	s.lastAnalysis = time.Now()
	s.suggestionsMu.Unlock()

	if len(suggestions) == 0 {
		logger.Info("Cost optimization analysis complete: no opportunities found", map[string]interface{}{
			"servers_analyzed": len(servers),
		})
		return
	}

	logger.Info("Cost optimization opportunities found", map[string]interface{}{
		"opportunities":     len(suggestions),
		"total_savings_h":   calculateTotalSavings(suggestions),
		"total_savings_mo":  calculateTotalSavings(suggestions) * 730, // ~730 hours/month
	})

	// Process suggestions based on server settings
	s.processSuggestions(suggestions, servers)
}

// analyzeCostOpportunities finds servers that could be moved to cheaper nodes
func (s *CostOptimizationService) analyzeCostOpportunities(
	servers []models.MinecraftServer,
	nodeMap map[string]conductor.CostNodeInfo,
) []OptimizationSuggestion {
	suggestions := []OptimizationSuggestion{}

	for _, server := range servers {
		// Skip if cost optimization disabled
		if server.CostOptimizationLevel == 0 {
			continue
		}

		// Skip reserved plans (24/7 guaranteed placement)
		if server.Plan == "reserved" {
			continue
		}

		// Only optimize small/medium servers (2-8GB)
		if server.RAMMb < 2048 || server.RAMMb > 8192 {
			continue
		}

		// Get current node info
		currentNode, exists := nodeMap[server.NodeID]
		if !exists || !currentNode.IsHealthy {
			continue
		}

		// Find cheaper alternatives
		currentCost := currentNode.CostPerHour

		for _, targetNode := range nodeMap {
			// Skip same node
			if targetNode.ID == server.NodeID {
				continue
			}

			// Skip unhealthy nodes
			if !targetNode.IsHealthy {
				continue
			}

			// Skip if can't fit
			if !s.conductor.CanFitServerOnNode(targetNode.ID, server.RAMMb) {
				continue
			}

			targetCost := targetNode.CostPerHour
			savings := currentCost - targetCost

			// Check if savings meet threshold
			if savings < s.minSavingsThreshold {
				continue
			}

			// Found a cost-saving opportunity
			suggestion := OptimizationSuggestion{
				ServerID:        server.ID,
				ServerName:      server.Name,
				CurrentNodeID:   server.NodeID,
				CurrentCost:     currentCost,
				TargetNodeID:    targetNode.ID,
				TargetCost:      targetCost,
				SavingsPerHour:  savings,
				SavingsPerMonth: savings * 730,
				Reason:          fmt.Sprintf("Move from %s (€%.4f/h) to %s (€%.4f/h)", currentNode.Type, currentCost, targetNode.Type, targetCost),
				CreatedAt:       time.Now(),
				Applied:         false,
			}

			suggestions = append(suggestions, suggestion)
			break // Take first cheaper option
		}
	}

	return suggestions
}

// processSuggestions handles suggestions based on optimization level
func (s *CostOptimizationService) processSuggestions(
	suggestions []OptimizationSuggestion,
	servers []models.MinecraftServer,
) {
	serverMap := make(map[string]models.MinecraftServer)
	for _, srv := range servers {
		serverMap[srv.ID] = srv
	}

	for _, suggestion := range suggestions {
		server, exists := serverMap[suggestion.ServerID]
		if !exists {
			continue
		}

		switch server.CostOptimizationLevel {
		case 1:
			// Level 1: Suggestions only - log for admin
			s.logSuggestion(suggestion)

		case 2:
			// Level 2: Auto-migrate (only if allowed by settings)
			if server.AllowMigration && s.canAutoMigrate(server) {
				s.performAutoMigration(suggestion, server)
			} else {
				s.logSuggestion(suggestion)
			}
		}
	}
}

// canAutoMigrate checks if server can be auto-migrated
func (s *CostOptimizationService) canAutoMigrate(server models.MinecraftServer) bool {
	// Check migration mode
	if server.MigrationMode == "never" {
		return false
	}

	// If mode is "only_offline", check player count
	if server.MigrationMode == "only_offline" {
		if server.CurrentPlayerCount > 0 {
			return false
		}
	}

	// Check minimum uptime (30 minutes)
	if server.LastStartedAt != nil {
		uptime := time.Since(*server.LastStartedAt)
		if uptime < 30*time.Minute {
			return false
		}
	}

	// Check minimum idle time (15 minutes since last player)
	if server.LastPlayerActivity != nil {
		idleTime := time.Since(*server.LastPlayerActivity)
		if idleTime < 15*time.Minute {
			return false
		}
	}

	return true
}

// logSuggestion creates a migration record with status "suggested"
func (s *CostOptimizationService) logSuggestion(suggestion OptimizationSuggestion) {
	// Check if a suggestion already exists for this server
	recent, err := s.migrationRepo.FindRecentMigrationForServer(suggestion.ServerID)
	if err != nil {
		logger.Error("Failed to check for recent migration", err, map[string]interface{}{
			"server_id": suggestion.ServerID,
		})
		return
	}

	// Don't create duplicate suggestions
	if recent != nil && recent.Status == models.MigrationStatusSuggested {
		logger.Debug("Skipping duplicate suggestion", map[string]interface{}{
			"server_id": suggestion.ServerID,
			"migration_id": recent.ID,
		})
		return
	}

	// Get node names from conductor
	fromNodeName := "Unknown"
	toNodeName := "Unknown"
	if fromNode, exists := s.conductor.NodeRegistry.GetNode(suggestion.CurrentNodeID); exists {
		fromNodeName = fromNode.Hostname
	}
	if toNode, exists := s.conductor.NodeRegistry.GetNode(suggestion.TargetNodeID); exists {
		toNodeName = toNode.Hostname
	}

	// Create migration record
	migration := &models.Migration{
		ID:              uuid.New().String(),
		ServerID:        suggestion.ServerID,
		FromNodeID:      suggestion.CurrentNodeID,
		FromNodeName:    fromNodeName,
		ToNodeID:        suggestion.TargetNodeID,
		ToNodeName:      toNodeName,
		Status:          models.MigrationStatusSuggested,
		Reason:          models.MigrationReasonCostOptimization,
		SavingsEURHour:  suggestion.SavingsPerHour,
		SavingsEURMonth: suggestion.SavingsPerMonth,
		CreatedAt:       time.Now(),
		TriggeredBy:     "system",
		Notes:           suggestion.Reason,
	}

	if err := s.migrationRepo.Create(migration); err != nil {
		logger.Error("Failed to create migration record", err, map[string]interface{}{
			"server_id": suggestion.ServerID,
		})
		return
	}

	logger.Info("💰 Cost Optimization Suggestion Created", map[string]interface{}{
		"migration_id":      migration.ID,
		"server_id":         suggestion.ServerID,
		"server_name":       suggestion.ServerName,
		"from_node":         fromNodeName,
		"to_node":           toNodeName,
		"savings_hour":      fmt.Sprintf("€%.4f", suggestion.SavingsPerHour),
		"savings_month":     fmt.Sprintf("€%.2f", suggestion.SavingsPerMonth),
	})
}

// performAutoMigration creates a migration record with status "scheduled" for immediate execution
func (s *CostOptimizationService) performAutoMigration(
	suggestion OptimizationSuggestion,
	server models.MinecraftServer,
) {
	// Check if migration is allowed (no active migration + cooldown check)
	canMigrate, err := s.migrationRepo.CanMigrateServer(suggestion.ServerID, 30) // 30 minute cooldown
	if err != nil || !canMigrate {
		logger.Warn("Cannot migrate server - cooldown or active migration", map[string]interface{}{
			"server_id": suggestion.ServerID,
			"error":     err,
		})
		return
	}

	// Get node names from conductor
	fromNodeName := "Unknown"
	toNodeName := "Unknown"
	if fromNode, exists := s.conductor.NodeRegistry.GetNode(suggestion.CurrentNodeID); exists {
		fromNodeName = fromNode.Hostname
	}
	if toNode, exists := s.conductor.NodeRegistry.GetNode(suggestion.TargetNodeID); exists {
		toNodeName = toNode.Hostname
	}

	// Create migration record with status "scheduled"
	now := time.Now()
	migration := &models.Migration{
		ID:              uuid.New().String(),
		ServerID:        suggestion.ServerID,
		FromNodeID:      suggestion.CurrentNodeID,
		FromNodeName:    fromNodeName,
		ToNodeID:        suggestion.TargetNodeID,
		ToNodeName:      toNodeName,
		Status:          models.MigrationStatusScheduled,
		Reason:          models.MigrationReasonCostOptimization,
		SavingsEURHour:  suggestion.SavingsPerHour,
		SavingsEURMonth: suggestion.SavingsPerMonth,
		CreatedAt:       now,
		ScheduledAt:     &now,
		PlayerCountAtStart: server.CurrentPlayerCount,
		TriggeredBy:     "system",
		Notes:           fmt.Sprintf("Auto-migration (Level 2): %s", suggestion.Reason),
	}

	if err := s.migrationRepo.Create(migration); err != nil {
		logger.Error("Failed to create migration record", err, map[string]interface{}{
			"server_id": suggestion.ServerID,
		})
		return
	}

	logger.Info("🤖 Auto-Migration Scheduled", map[string]interface{}{
		"migration_id":  migration.ID,
		"server_id":     suggestion.ServerID,
		"server_name":   suggestion.ServerName,
		"from_node":     fromNodeName,
		"to_node":       toNodeName,
		"savings_hour":  fmt.Sprintf("€%.4f", suggestion.SavingsPerHour),
		"player_count":  server.CurrentPlayerCount,
	})

	// TODO: Migration Service will pick this up and execute
	// For now, just create the record - execution will be implemented in Migration Service
}

// calculateTotalSavings sums up all savings
func calculateTotalSavings(suggestions []OptimizationSuggestion) float64 {
	total := 0.0
	for _, s := range suggestions {
		total += s.SavingsPerHour
	}
	return total
}

// GetCurrentSuggestions returns the current optimization suggestions
func (s *CostOptimizationService) GetCurrentSuggestions() []OptimizationSuggestion {
	s.suggestionsMu.RLock()
	defer s.suggestionsMu.RUnlock()

	// Return a copy to avoid race conditions
	suggestions := make([]OptimizationSuggestion, len(s.currentSuggestions))
	copy(suggestions, s.currentSuggestions)

	return suggestions
}

// ServiceStatus represents the status of the cost optimization service
type ServiceStatus struct {
	IsRunning            bool      `json:"is_running"`
	LastAnalysis         time.Time `json:"last_analysis"`
	NextAnalysis         time.Time `json:"next_analysis"`
	InCooldown           bool      `json:"in_cooldown"`
	CooldownRemaining    string    `json:"cooldown_remaining,omitempty"`
	CurrentSuggestions   int       `json:"current_suggestions"`
	TotalSavingsPerHour  float64   `json:"total_savings_eur_hour"`
	TotalSavingsPerMonth float64   `json:"total_savings_eur_month"`
	CheckInterval        string    `json:"check_interval"`
	MinSavingsThreshold  float64   `json:"min_savings_threshold_eur_hour"`
}

// GetStatus returns the current status of the service
func (s *CostOptimizationService) GetStatus() ServiceStatus {
	s.suggestionsMu.RLock()
	suggestions := s.currentSuggestions
	lastAnalysis := s.lastAnalysis
	s.suggestionsMu.RUnlock()

	s.cooldownMu.RLock()
	inCooldown := s.isInCooldown()
	lastScaling := s.lastScalingEvent
	s.cooldownMu.RUnlock()

	totalSavingsHour := calculateTotalSavings(suggestions)

	status := ServiceStatus{
		IsRunning:            true,
		LastAnalysis:         lastAnalysis,
		NextAnalysis:         lastAnalysis.Add(s.checkInterval),
		InCooldown:           inCooldown,
		CurrentSuggestions:   len(suggestions),
		TotalSavingsPerHour:  totalSavingsHour,
		TotalSavingsPerMonth: totalSavingsHour * 730, // ~730 hours/month
		CheckInterval:        s.checkInterval.String(),
		MinSavingsThreshold:  s.minSavingsThreshold,
	}

	if inCooldown && !lastScaling.IsZero() {
		remaining := s.scalingCooldown - time.Since(lastScaling)
		status.CooldownRemaining = remaining.Round(time.Minute).String()
	}

	return status
}

// TriggerImmediateAnalysis triggers an immediate cost optimization analysis
func (s *CostOptimizationService) TriggerImmediateAnalysis() {
	logger.Info("Manual cost optimization analysis triggered", nil)
	s.analyzeAndOptimize()
}
