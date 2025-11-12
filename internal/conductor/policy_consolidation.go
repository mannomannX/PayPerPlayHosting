package conductor

import (
	"fmt"
	"sort"
	"time"

	"github.com/payperplay/hosting/pkg/logger"
)

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

// calculateOptimalLayout implements First-Fit Decreasing bin-packing algorithm
func (p *ConsolidationPolicy) calculateOptimalLayout(ctx ScalingContext) ConsolidationPlan {
	// 1. Collect all containers from all cloud nodes
	type ContainerInfo struct {
		ServerID    string
		ServerName  string
		RAMMb       int
		CurrentNode string
		PlayerCount int
		CanMigrate  bool
	}

	containers := []ContainerInfo{}
	if ctx.ContainerRegistry == nil {
		logger.Warn("ConsolidationPolicy: ContainerRegistry not available", nil)
		return ConsolidationPlan{NodeSavings: 0}
	}

	for _, node := range ctx.CloudNodes {
		nodeContainers := ctx.ContainerRegistry.GetContainersByNode(node.ID)
		for _, container := range nodeContainers {
			playerCount := p.getPlayerCount(container.ServerName)
			canMigrate := p.canMigrateContainer(playerCount)

			containers = append(containers, ContainerInfo{
				ServerID:    container.ServerID,
				ServerName:  container.ServerName,
				RAMMb:       container.RAMMb,
				CurrentNode: node.ID,
				PlayerCount: playerCount,
				CanMigrate:  canMigrate,
			})
		}
	}

	// 2. Sort containers by RAM size (descending - largest first for First-Fit Decreasing)
	sort.Slice(containers, func(i, j int) bool {
		return containers[i].RAMMb > containers[j].RAMMb
	})

	// 3. Bin-packing: Try to fit containers into minimal number of nodes
	type NodeBin struct {
		NodeID      string
		TotalRAMMb  int
		UsedRAMMb   int
		Containers  []ContainerInfo
	}

	// Start with existing nodes as bins (sorted by most full first - Best-Fit)
	bins := []NodeBin{}
	for _, node := range ctx.CloudNodes {
		bins = append(bins, NodeBin{
			NodeID:     node.ID,
			TotalRAMMb: node.UsableRAMMB(),
			UsedRAMMb:  0,
			Containers: []ContainerInfo{},
		})
	}

	// Sort bins by most full first (prefer filling existing nodes)
	sort.Slice(bins, func(i, j int) bool {
		return bins[i].UsedRAMMb > bins[j].UsedRAMMb
	})

	// 4. Place each container into first bin that fits
	for _, container := range containers {
		// Skip containers that cannot be migrated
		if !container.CanMigrate {
			// Keep container on current node
			for i := range bins {
				if bins[i].NodeID == container.CurrentNode {
					bins[i].Containers = append(bins[i].Containers, container)
					bins[i].UsedRAMMb += container.RAMMb
					break
				}
			}
			continue
		}

		// Find first bin with enough space
		placed := false
		for i := range bins {
			available := bins[i].TotalRAMMb - bins[i].UsedRAMMb
			if available >= container.RAMMb {
				bins[i].Containers = append(bins[i].Containers, container)
				bins[i].UsedRAMMb += container.RAMMb
				placed = true
				break
			}
		}

		if !placed {
			// This should not happen if capacity < 70%
			logger.Warn("ConsolidationPolicy: Could not place container", map[string]interface{}{
				"server_id":   container.ServerID,
				"server_name": container.ServerName,
				"ram_mb":      container.RAMMb,
			})
			return ConsolidationPlan{NodeSavings: 0} // Abort consolidation
		}
	}

	// 5. Determine which bins are actually used
	usedBins := []NodeBin{}
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

// canMigrateContainer determines if a container can be safely migrated
func (p *ConsolidationPolicy) canMigrateContainer(playerCount int) bool {
	if playerCount == 0 {
		return true // Safe to migrate empty servers
	}

	// If server has players, only migrate if explicitly allowed
	return p.AllowMigrationWithPlayers
}
