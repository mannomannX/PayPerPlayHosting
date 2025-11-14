package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/payperplay/hosting/internal/conductor"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/logger"
)

// CostOptimizationService analyzes server placements and suggests/performs migrations
type CostOptimizationService struct {
	serverRepo  *repository.ServerRepository
	conductor   *conductor.Conductor
	checkInterval time.Duration
	stopChan    chan struct{}
	wg          sync.WaitGroup

	// Minimum savings to trigger migration (EUR/hour)
	minSavingsThreshold float64

	// Cooldown period after scaling events
	scalingCooldown time.Duration
	lastScalingEvent time.Time
	cooldownMu      sync.RWMutex
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
) *CostOptimizationService {
	return &CostOptimizationService{
		serverRepo:          serverRepo,
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

// logSuggestion logs a cost optimization suggestion
func (s *CostOptimizationService) logSuggestion(suggestion OptimizationSuggestion) {
	logger.Info("💰 Cost Optimization Suggestion", map[string]interface{}{
		"server_id":         suggestion.ServerID,
		"server_name":       suggestion.ServerName,
		"from_node":         suggestion.CurrentNodeID,
		"to_node":           suggestion.TargetNodeID,
		"savings_hour":      fmt.Sprintf("€%.4f", suggestion.SavingsPerHour),
		"savings_month":     fmt.Sprintf("€%.2f", suggestion.SavingsPerMonth),
		"reason":            suggestion.Reason,
	})

	// TODO: Store in database for Dashboard display
}

// performAutoMigration executes an automatic migration
func (s *CostOptimizationService) performAutoMigration(
	suggestion OptimizationSuggestion,
	server models.MinecraftServer,
) {
	logger.Info("🤖 Executing auto-migration", map[string]interface{}{
		"server_id":     suggestion.ServerID,
		"server_name":   suggestion.ServerName,
		"from_node":     suggestion.CurrentNodeID,
		"to_node":       suggestion.TargetNodeID,
		"savings_hour":  fmt.Sprintf("€%.4f", suggestion.SavingsPerHour),
	})

	// TODO: Implement live migration
	// 1. Send warning to players (if any online)
	// 2. Start new container on target node
	// 3. Transfer players (via Velocity)
	// 4. Stop old container
	// 5. Update database

	logger.Warn("Auto-migration not yet implemented - would migrate now", map[string]interface{}{
		"server": suggestion.ServerName,
	})
}

// calculateTotalSavings sums up all savings
func calculateTotalSavings(suggestions []OptimizationSuggestion) float64 {
	total := 0.0
	for _, s := range suggestions {
		total += s.SavingsPerHour
	}
	return total
}
