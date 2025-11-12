package conductor

import (
	"fmt"
	"time"

	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/logger"
)

// ConsolidationNodeBin represents a node bin for bin-packing algorithm
type ConsolidationNodeBin struct {
	NodeID     string
	TotalRAMMb int
	UsedRAMMb  int
	Containers []ConsolidationContainerInfo
}

// ConsolidationContainerInfo holds container information for consolidation analysis
type ConsolidationContainerInfo struct {
	ServerID    string
	ServerName  string
	RAMMb       int
	Tier        string
	CurrentNode string
	PlayerCount int
	CanMigrate  bool
}

// ConsolidationPolicy implements intelligent container migration & bin-packing for cost optimization (B8)
// This policy focuses on MINIMIZING COSTS by consolidating containers onto fewer nodes
type ConsolidationPolicy struct {
	Enabled               bool          // Enable/disable consolidation
	CooldownPeriod        time.Duration // Wait between consolidation attempts
	ThresholdNodeSavings  int           // Min. number of nodes to save
	MaxCapacityPercent    float64       // Don't consolidate above this capacity (safety)
	AllowMigrationWithPlayers bool      // Allow migration of servers with players (dangerous!)
	lastConsolidation     time.Time

	// Velocity client for player count checks
	velocityClient VelocityClient
}

// VelocityClient interface for player count checks (dependency injection)
type VelocityClient interface {
	GetPlayerCount(serverName string) (int, error)
}

// NewConsolidationPolicy creates a new consolidation policy
func NewConsolidationPolicy(velocityClient VelocityClient) *ConsolidationPolicy {
	return &ConsolidationPolicy{
		Enabled:                   true,
		CooldownPeriod:            30 * time.Minute, // Check every 30 minutes
		ThresholdNodeSavings:      2,                // Only consolidate if saving 2+ nodes
		MaxCapacityPercent:        70.0,             // Don't consolidate if fleet >70% full
		AllowMigrationWithPlayers: false,            // Safety first: only migrate empty servers
		lastConsolidation:         time.Time{},
		velocityClient:            velocityClient,
	}
}

func (p *ConsolidationPolicy) Name() string {
	return "consolidation"
}

func (p *ConsolidationPolicy) Priority() int {
	return 1 // Lowest priority (only runs if no other action needed)
}

// ShouldScaleUp - ConsolidationPolicy does not handle scale-up
func (p *ConsolidationPolicy) ShouldScaleUp(ctx ScalingContext) (bool, ScaleRecommendation) {
	return false, ScaleRecommendation{Action: ScaleActionNone}
}

// ShouldScaleDown - ConsolidationPolicy does not handle scale-down
func (p *ConsolidationPolicy) ShouldScaleDown(ctx ScalingContext) (bool, ScaleRecommendation) {
	return false, ScaleRecommendation{Action: ScaleActionNone}
}

// ShouldConsolidate determines if containers should be migrated to reduce costs
func (p *ConsolidationPolicy) ShouldConsolidate(ctx ScalingContext) (bool, ConsolidationPlan) {
	// Check if enabled
	if !p.Enabled {
		return false, ConsolidationPlan{}
	}

	// Check cooldown period
	if time.Since(p.lastConsolidation) < p.CooldownPeriod {
		logger.Debug("ConsolidationPolicy: Cooldown active", map[string]interface{}{
			"time_since_last": time.Since(p.lastConsolidation).String(),
			"cooldown_period": p.CooldownPeriod.String(),
		})
		return false, ConsolidationPlan{}
	}

	// Need at least 2 cloud nodes to consolidate
	if len(ctx.CloudNodes) < 2 {
		logger.Debug("ConsolidationPolicy: Not enough nodes to consolidate", map[string]interface{}{
			"cloud_nodes": len(ctx.CloudNodes),
		})
		return false, ConsolidationPlan{}
	}

	// Safety check: Don't consolidate if capacity too high (risky!)
	capacityPercent := float64(0)
	if ctx.FleetStats.UsableRAMMB > 0 {
		capacityPercent = (float64(ctx.FleetStats.AllocatedRAMMB) / float64(ctx.FleetStats.UsableRAMMB)) * 100
	}

	if capacityPercent > p.MaxCapacityPercent {
		logger.Debug("ConsolidationPolicy: Fleet too full for safe consolidation", map[string]interface{}{
			"capacity_percent":      capacityPercent,
			"max_capacity_percent":  p.MaxCapacityPercent,
		})
		return false, ConsolidationPlan{}
	}

	// Calculate optimal container layout using bin-packing
	plan := p.calculateOptimalLayout(ctx)

	// Only consolidate if savings are significant
	if plan.NodeSavings < p.ThresholdNodeSavings {
		logger.Debug("ConsolidationPolicy: Savings not significant enough", map[string]interface{}{
			"node_savings":          plan.NodeSavings,
			"threshold":             p.ThresholdNodeSavings,
		})
		return false, ConsolidationPlan{}
	}

	// Update last consolidation time
	p.lastConsolidation = time.Now()

	logger.Info("ConsolidationPolicy: Consolidation recommended", map[string]interface{}{
		"migrations":             len(plan.Migrations),
		"nodes_before":           len(ctx.CloudNodes),
		"nodes_after":            len(plan.NodesToKeep),
		"node_savings":           plan.NodeSavings,
		"estimated_cost_savings": plan.EstimatedCostSavings,
	})

	return true, plan
}

// calculateOptimalLayout implements tier-aware perfect bin-packing
// For standard tiers: O(n) complexity with 100% node utilization
// For custom tiers: First-Fit Decreasing (fallback)
func (p *ConsolidationPolicy) calculateOptimalLayout(ctx ScalingContext) ConsolidationPlan {
	// 1. Collect all containers from all cloud nodes
	containers := []ConsolidationContainerInfo{}
	if ctx.ContainerRegistry == nil {
		logger.Warn("ConsolidationPolicy: ContainerRegistry not available", nil)
		return ConsolidationPlan{NodeSavings: 0}
	}

	for _, node := range ctx.CloudNodes {
		nodeContainers := ctx.ContainerRegistry.GetContainersByNode(node.ID)
		for _, container := range nodeContainers {
			// Get server info to determine tier and migration settings
			server, err := p.getServerInfo(container.ServerID)
			if err != nil {
				logger.Warn("Could not get server info for consolidation", map[string]interface{}{
					"server_id": container.ServerID,
					"error":     err.Error(),
				})
				continue
			}

			playerCount := p.getPlayerCount(container.ServerName)
			canMigrate := p.canMigrateServer(server, playerCount)

			containers = append(containers, ConsolidationContainerInfo{
				ServerID:    container.ServerID,
				ServerName:  container.ServerName,
				RAMMb:       container.RAMMb,
				Tier:        server.RAMTier,
				CurrentNode: node.ID,
				PlayerCount: playerCount,
				CanMigrate:  canMigrate,
			})
		}
	}

	logger.Info("Consolidation analysis started", map[string]interface{}{
		"total_containers": len(containers),
		"cloud_nodes":      len(ctx.CloudNodes),
	})

	// 2. Group containers by tier (for perfect packing of standard tiers)
	tierGroups := make(map[string][]ConsolidationContainerInfo)
	customContainers := []ConsolidationContainerInfo{}

	for _, container := range containers {
		if models.IsStandardTier(container.RAMMb) {
			tierGroups[container.Tier] = append(tierGroups[container.Tier], container)
		} else {
			// Custom tier: use fallback algorithm
			customContainers = append(customContainers, container)
		}
	}

	// 3. Calculate perfect packing for standard tiers
	nodeCapacity := 16384 // cpx41 = 16GB standard worker node
	totalNodesNeeded := 0

	// Count containers by tier (for perfect packing calculation)
	containersByTier := make(map[string]int)
	for tier, containerList := range tierGroups {
		migratable := 0
		for _, c := range containerList {
			if c.CanMigrate {
				migratable++
			}
		}
		containersByTier[tier] = migratable
	}

	// Use models.CalculatePerfectPackingNodes for optimal layout
	totalNodesNeeded = models.CalculatePerfectPackingNodes(containersByTier, nodeCapacity)

	// Add nodes for custom tier containers (fallback to one per container for safety)
	totalNodesNeeded += len(customContainers)

	logger.Info("Perfect packing calculated", map[string]interface{}{
		"standard_tier_containers": len(containers) - len(customContainers),
		"custom_tier_containers":   len(customContainers),
		"current_nodes":            len(ctx.CloudNodes),
		"optimal_nodes":            totalNodesNeeded,
		"node_savings":             len(ctx.CloudNodes) - totalNodesNeeded,
	})

	// 4. Build migration plan
	if totalNodesNeeded >= len(ctx.CloudNodes) {
		// No savings, abort
		logger.Info("No consolidation savings possible", map[string]interface{}{
			"current": len(ctx.CloudNodes),
			"optimal": totalNodesNeeded,
		})
		return ConsolidationPlan{NodeSavings: 0}
	}

	// 5. Assign containers to nodes (simplified for standard tiers)
	bins := p.createOptimalBins(tierGroups, customContainers, totalNodesNeeded, nodeCapacity, ctx.CloudNodes)

	// 6. Determine which bins are actually used
	usedBins := []ConsolidationNodeBin{}
	for _, bin := range bins {
		if len(bin.Containers) > 0 {
			usedBins = append(usedBins, bin)
		}
	}

	// 6. Build migration plan
	migrations := []Migration{}
	nodesToKeep := []string{}
	nodesToRemove := []string{}

	for _, bin := range usedBins {
		nodesToKeep = append(nodesToKeep, bin.NodeID)
		for _, container := range bin.Containers {
			if container.CurrentNode != bin.NodeID {
				migrations = append(migrations, Migration{
					ServerID:    container.ServerID,
					ServerName:  container.ServerName,
					FromNode:    container.CurrentNode,
					ToNode:      bin.NodeID,
					RAMMb:       container.RAMMb,
					PlayerCount: container.PlayerCount,
				})
			}
		}
	}

	// Nodes to remove are all nodes not in nodesToKeep
	nodeMap := make(map[string]bool)
	for _, nodeID := range nodesToKeep {
		nodeMap[nodeID] = true
	}
	for _, node := range ctx.CloudNodes {
		if !nodeMap[node.ID] {
			nodesToRemove = append(nodesToRemove, node.ID)
		}
	}

	nodeSavings := len(ctx.CloudNodes) - len(nodesToKeep)

	// Estimate cost savings (assume CPX22 = €0.0096/h)
	estimatedCostSavings := float64(nodeSavings) * 0.0096

	return ConsolidationPlan{
		Migrations:           migrations,
		NodesToKeep:          nodesToKeep,
		NodesToRemove:        nodesToRemove,
		NodeSavings:          nodeSavings,
		EstimatedCostSavings: estimatedCostSavings,
		Reason: fmt.Sprintf("Consolidate %d servers from %d nodes to %d nodes (save %d nodes, %.4f€/h)",
			len(containers), len(ctx.CloudNodes), len(nodesToKeep), nodeSavings, estimatedCostSavings),
	}
}

// getPlayerCount returns player count for a server (0 if unknown or error)
func (p *ConsolidationPolicy) getPlayerCount(serverName string) int {
	if p.velocityClient == nil {
		return 0 // Assume empty if no velocity client
	}

	count, err := p.velocityClient.GetPlayerCount(serverName)
	if err != nil {
		logger.Debug("ConsolidationPolicy: Could not get player count", map[string]interface{}{
			"server_name": serverName,
			"error":       err.Error(),
		})
		return 0 // Assume empty on error (safer to migrate)
	}

	return count
}

// canMigrateContainer determines if a container can be safely migrated (deprecated)
// Use canMigrateServer instead for tier-aware migration decisions
func (p *ConsolidationPolicy) canMigrateContainer(playerCount int) bool {
	if playerCount == 0 {
		return true // Safe to migrate empty servers
	}

	// If server has players, only migrate if explicitly allowed
	return p.AllowMigrationWithPlayers
}

// getServerInfo retrieves server model for tier and plan information
func (p *ConsolidationPolicy) getServerInfo(serverID string) (*models.MinecraftServer, error) {
	// This requires access to database/repository
	// For now, we'll need to inject this or access it via conductor
	// Placeholder implementation
	db := repository.GetDB()
	if db == nil {
		return nil, fmt.Errorf("database not available")
	}

	var server models.MinecraftServer
	if err := db.Where("id = ?", serverID).First(&server).Error; err != nil {
		return nil, err
	}

	return &server, nil
}

// canMigrateServer determines if a server can be migrated based on tier and plan
func (p *ConsolidationPolicy) canMigrateServer(server *models.MinecraftServer, playerCount int) bool {
	// Check if server allows consolidation (tier + plan based)
	if !server.AllowsConsolidation() {
		return false
	}

	// Tier-specific rules
	switch server.RAMTier {
	case models.TierMicro, models.TierSmall:
		// Micro/Small: Aggressive consolidation
		if server.Plan == models.PlanPayPerPlay {
			// PayPerPlay: allow migration with ≤5 players
			return playerCount <= 5
		}
		// Balanced: only when empty
		return playerCount == 0

	case models.TierMedium:
		// Medium: Only when empty
		return playerCount == 0

	case models.TierLarge, models.TierXLarge:
		// Large/XLarge: Never migrate (too risky)
		return false

	case models.TierCustom:
		// Custom: Never migrate (inefficient)
		return false

	default:
		return false
	}
}

// createOptimalBins assigns containers to bins for perfect packing
func (p *ConsolidationPolicy) createOptimalBins(
	tierGroups map[string][]ConsolidationContainerInfo,
	customContainers []ConsolidationContainerInfo,
	totalNodesNeeded int,
	nodeCapacity int,
	existingNodes []*Node,
) []ConsolidationNodeBin {
	bins := make([]ConsolidationNodeBin, totalNodesNeeded)

	// Initialize bins (reuse existing nodes where possible)
	for i := 0; i < totalNodesNeeded; i++ {
		if i < len(existingNodes) {
			bins[i] = ConsolidationNodeBin{
				NodeID:     existingNodes[i].ID,
				TotalRAMMb: existingNodes[i].UsableRAMMB(),
				UsedRAMMb:  0,
				Containers: []ConsolidationContainerInfo{},
			}
		} else {
			// Will need new node (shouldn't happen often)
			bins[i] = ConsolidationNodeBin{
				NodeID:     fmt.Sprintf("new-node-%d", i),
				TotalRAMMb: nodeCapacity,
				UsedRAMMb:  0,
				Containers: []ConsolidationContainerInfo{},
			}
		}
	}

	// Pack standard tier containers (perfect packing)
	binIndex := 0
	for tier, containers := range tierGroups {
		tierRAM, _ := models.GetTierRAM(tier)
		containersPerNode := nodeCapacity / tierRAM

		for i, container := range containers {
			if !container.CanMigrate {
				continue // Skip non-migratable
			}

			// Determine which bin this container goes to
			containerIndex := i % containersPerNode
			if containerIndex == 0 && i > 0 {
				binIndex++
			}

			if binIndex < len(bins) {
				bins[binIndex].Containers = append(bins[binIndex].Containers, container)
				bins[binIndex].UsedRAMMb += container.RAMMb
			}
		}
		binIndex++ // Move to next bin for next tier
	}

	// Pack custom containers (one per bin for safety)
	for _, container := range customContainers {
		if binIndex < len(bins) {
			bins[binIndex].Containers = append(bins[binIndex].Containers, container)
			bins[binIndex].UsedRAMMb += container.RAMMb
			binIndex++
		}
	}

	return bins
}
