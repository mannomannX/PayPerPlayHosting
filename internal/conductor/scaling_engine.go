package conductor

import (
	"fmt"
	"sort"
	"time"

	"github.com/payperplay/hosting/internal/cloud"
	"github.com/payperplay/hosting/internal/events"
	"github.com/payperplay/hosting/pkg/logger"
)

// ScalingEngine orchestrates all scaling operations
// It combines multiple policies (reactive, spare-pool, predictive) into unified decisions
type ScalingEngine struct {
	policies       []ScalingPolicy
	cloudProvider  cloud.CloudProvider
	vmProvisioner  *VMProvisioner
	nodeRegistry   *NodeRegistry
	startQueue     *StartQueue // Queue for servers waiting for capacity
	conductor      *Conductor  // Back-reference for migrations (B8)
	velocityClient interface{} // For Velocity-aware migrations (can be nil or *velocity.RemoteVelocityClient)
	debugLogBuffer *DebugLogBuffer
	enabled        bool
	checkInterval  time.Duration
	stopChan       chan struct{}
}

// NewScalingEngine creates a new scaling engine
func NewScalingEngine(
	cloudProvider cloud.CloudProvider,
	vmProvisioner *VMProvisioner,
	nodeRegistry *NodeRegistry,
	startQueue *StartQueue,
	debugLogBuffer *DebugLogBuffer,
	enabled bool,
	velocityClient VelocityClient,
) *ScalingEngine {
	engine := &ScalingEngine{
		policies:       []ScalingPolicy{},
		cloudProvider:  cloudProvider,
		vmProvisioner:  vmProvisioner,
		nodeRegistry:   nodeRegistry,
		startQueue:     startQueue,
		conductor:      nil, // Set later via SetConductor()
		velocityClient: velocityClient,
		debugLogBuffer: debugLogBuffer,
		enabled:        enabled,
		checkInterval:  2 * time.Minute, // Check every 2 minutes
		stopChan:       make(chan struct{}),
	}

	// Register default policies
	engine.RegisterPolicy(NewReactivePolicy(cloudProvider, debugLogBuffer))
	// TODO B6: engine.RegisterPolicy(NewSparePoolPolicy())
	// TODO B7: engine.RegisterPolicy(NewPredictivePolicy())

	// B8 Container Migration & Cost Optimization
	if velocityClient != nil {
		// ConsolidationPolicy only needs VelocityClient interface (GetPlayerCount method)
		// velocityClient should implement both VelocityClient and VelocityRemoteClient
		if vc, ok := velocityClient.(VelocityClient); ok {
			engine.RegisterPolicy(NewConsolidationPolicy(vc))
		}
	}

	return engine
}

// RegisterPolicy adds a new scaling policy
// Policies are automatically sorted by priority (highest first)
func (e *ScalingEngine) RegisterPolicy(policy ScalingPolicy) {
	e.policies = append(e.policies, policy)

	// Sort policies by priority (highest first)
	sort.Slice(e.policies, func(i, j int) bool {
		return e.policies[i].Priority() > e.policies[j].Priority()
	})

	logger.Info("Scaling policy registered", map[string]interface{}{
		"policy":   policy.Name(),
		"priority": policy.Priority(),
	})
}

// SetConductor sets the conductor reference (called after initialization to avoid circular dependency)
func (e *ScalingEngine) SetConductor(conductor *Conductor) {
	e.conductor = conductor
}

// Start begins the scaling engine evaluation loop
func (e *ScalingEngine) Start() {
	logger.Info("ScalingEngine started", map[string]interface{}{
		"check_interval": e.checkInterval.String(),
		"policies_count": len(e.policies),
		"policies": func() []string {
			names := make([]string, len(e.policies))
			for i, p := range e.policies {
				names[i] = p.Name()
			}
			return names
		}(),
	})

	go e.runLoop()
}

// Stop stops the scaling engine
func (e *ScalingEngine) Stop() {
	logger.Info("Stopping ScalingEngine", nil)
	close(e.stopChan)
}

// Enable enables scaling operations
func (e *ScalingEngine) Enable() {
	e.enabled = true
	logger.Info("ScalingEngine enabled", nil)
}

// Disable disables scaling operations (for maintenance)
func (e *ScalingEngine) Disable() {
	e.enabled = false
	logger.Info("ScalingEngine disabled", nil)
}

// IsEnabled returns whether scaling is enabled
func (e *ScalingEngine) IsEnabled() bool {
	return e.enabled
}

// TriggerImmediateCheck triggers an immediate scaling evaluation
// This is called when a new server is created or capacity changes to avoid waiting for the next interval
func (e *ScalingEngine) TriggerImmediateCheck() {
	if !e.enabled {
		logger.Debug("Immediate scaling check skipped (disabled)", nil)
		return
	}

	logger.Info("Triggering immediate scaling check", map[string]interface{}{
		"reason": "server_created_or_modified",
	})

	// Run evaluation in background to avoid blocking the caller
	go e.evaluateScaling()
}

// runLoop is the main evaluation loop
func (e *ScalingEngine) runLoop() {
	ticker := time.NewTicker(e.checkInterval)
	defer ticker.Stop()

	// NOTE: Do NOT run immediately on start to prevent race conditions
	// Give the system time to sync container state, queue state, and node state
	// before making scaling decisions that could decommission nodes

	for {
		select {
		case <-ticker.C:
			e.evaluateScaling()
		case <-e.stopChan:
			logger.Info("ScalingEngine loop stopped", nil)
			return
		}
	}
}

// evaluateScaling checks all policies and executes scaling if needed
func (e *ScalingEngine) evaluateScaling() {
	if !e.enabled {
		logger.Debug("Scaling evaluation skipped (disabled)", nil)
		return
	}

	// Build current context
	ctx := e.buildScalingContext()

	logger.Debug("Evaluating scaling", map[string]interface{}{
		"total_ram_mb":     ctx.FleetStats.TotalRAMMB,
		"allocated_ram_mb": ctx.FleetStats.AllocatedRAMMB,
		"capacity_percent": func() float64 {
			if ctx.FleetStats.TotalRAMMB == 0 {
				return 0
			}
			return (float64(ctx.FleetStats.AllocatedRAMMB) / float64(ctx.FleetStats.TotalRAMMB)) * 100
		}(),
		"dedicated_nodes": len(ctx.DedicatedNodes),
		"cloud_nodes":     len(ctx.CloudNodes),
	})

	// Ask all policies (in priority order) if we should scale UP
	for _, policy := range e.policies {
		if shouldScale, recommendation := policy.ShouldScaleUp(ctx); shouldScale {
			fields := map[string]interface{}{
				"policy":      policy.Name(),
				"action":      recommendation.Action,
				"server_type": recommendation.ServerType,
				"count":       recommendation.Count,
				"reason":      recommendation.Reason,
				"urgency":     recommendation.Urgency,
			}
			logger.Info("Scale UP decision", fields)

			// Add to debug log buffer for dashboard
			if e.conductor != nil && e.conductor.DebugLogBuffer != nil {
				e.conductor.DebugLogBuffer.Add("INFO", fmt.Sprintf("Scale UP: %s (%s)", recommendation.Reason, recommendation.ServerType), fields)
			}

			// Publish scaling decision event
			capacityPercent := 0.0
			if ctx.FleetStats.TotalRAMMB > 0 {
				capacityPercent = (float64(ctx.FleetStats.AllocatedRAMMB) / float64(ctx.FleetStats.TotalRAMMB)) * 100
			}
			events.PublishScalingDecision(policy.Name(), string(recommendation.Action), recommendation.ServerType, recommendation.Reason, string(recommendation.Urgency), recommendation.Count, capacityPercent, nil)

			if err := e.executeScaling(recommendation); err != nil {
				logger.Error("Failed to execute scaling", err, map[string]interface{}{
					"policy":     policy.Name(),
					"action":     recommendation.Action,
					"recommendation": recommendation,
				})
			}

			return // Only execute ONE action per cycle
		}
	}

	// Ask all policies if we should scale DOWN
	for _, policy := range e.policies {
		if shouldScale, recommendation := policy.ShouldScaleDown(ctx); shouldScale {
			fields := map[string]interface{}{
				"policy": policy.Name(),
				"action": recommendation.Action,
				"count":  recommendation.Count,
				"reason": recommendation.Reason,
			}
			logger.Info("Scale DOWN decision", fields)

			// Add to debug log buffer for dashboard
			if e.conductor != nil && e.conductor.DebugLogBuffer != nil {
				e.conductor.DebugLogBuffer.Add("INFO", fmt.Sprintf("Scale DOWN: %s (count: %d)", recommendation.Reason, recommendation.Count), fields)
			}

			// Publish scaling decision event
			capacityPercent := 0.0
			if ctx.FleetStats.TotalRAMMB > 0 {
				capacityPercent = (float64(ctx.FleetStats.AllocatedRAMMB) / float64(ctx.FleetStats.TotalRAMMB)) * 100
			}
			events.PublishScalingDecision(policy.Name(), string(recommendation.Action), recommendation.ServerType, recommendation.Reason, string(recommendation.Urgency), recommendation.Count, capacityPercent, nil)

			if err := e.executeScaling(recommendation); err != nil {
				logger.Error("Failed to execute scaling", err, map[string]interface{}{
					"policy": policy.Name(),
					"action": recommendation.Action,
				})
			}

			return // Only execute ONE action per cycle
		}
	}

	// Ask all policies if we should CONSOLIDATE (B8 - lowest priority, only if no other action)
	for _, policy := range e.policies {
		if shouldConsolidate, plan := policy.ShouldConsolidate(ctx); shouldConsolidate {
			logger.Info("CONSOLIDATION decision", map[string]interface{}{
				"policy":                 policy.Name(),
				"migrations":             len(plan.Migrations),
				"nodes_before":           len(ctx.CloudNodes),
				"nodes_after":            len(plan.NodesToKeep),
				"node_savings":           plan.NodeSavings,
				"estimated_cost_savings": plan.EstimatedCostSavings,
				"reason":                 plan.Reason,
			})

			// Publish consolidation started event
			events.PublishConsolidationStarted(len(plan.Migrations), len(ctx.CloudNodes), len(plan.NodesToKeep), plan.NodeSavings, plan.EstimatedCostSavings, plan.Reason, plan.NodesToRemove)

			if err := e.executeConsolidation(plan); err != nil {
				logger.Error("Failed to execute consolidation", err, map[string]interface{}{
					"policy": policy.Name(),
				})
			}

			return // Only execute ONE action per cycle
		}
	}

	logger.Debug("No scaling action needed", nil)
}

// buildScalingContext gathers all data needed for scaling decisions
func (e *ScalingEngine) buildScalingContext() ScalingContext {
	stats := e.nodeRegistry.GetFleetStats()
	nodes := e.nodeRegistry.GetAllNodes()

	var dedicatedNodes, cloudNodes, workerNodes []*Node
	for _, node := range nodes {
		// CRITICAL FIX: Only count NON-SYSTEM dedicated nodes for capacity planning
		// System nodes (local-node, proxy-node) don't host Minecraft containers
		if node.Type == "dedicated" && !node.IsSystemNode {
			dedicatedNodes = append(dedicatedNodes, node)
		} else if node.Type == "cloud" {
			cloudNodes = append(cloudNodes, node)
		}

		// Worker nodes are all non-system nodes (can run MC containers)
		if !node.IsSystemNode {
			workerNodes = append(workerNodes, node)
		}
	}

	now := time.Now()

	// Get ContainerRegistry from conductor (for B8 Consolidation)
	var containerRegistry *ContainerRegistry
	if e.conductor != nil {
		containerRegistry = e.conductor.ContainerRegistry
	}

	// Get queue size
	queueSize := 0
	if e.startQueue != nil {
		queueSize = e.startQueue.Size()
	}

	return ScalingContext{
		FleetStats:        stats,
		DedicatedNodes:    dedicatedNodes,
		CloudNodes:        cloudNodes,
		WorkerNodes:       workerNodes,
		QueuedServerCount: queueSize,
		ContainerRegistry: containerRegistry,
		CurrentTime:       now,
		IsWeekend:         now.Weekday() == time.Saturday || now.Weekday() == time.Sunday,
		IsHoliday:         false, // TODO: Holiday calendar

		// TODO: Add historical data from InfluxDB
		AverageRAMUsageLast1h:  0,
		AverageRAMUsageLast24h: 0,
		PeakRAMUsageLast24h:    0,

		// TODO B7: Add forecast data from ML service
		ForecastedRAMIn1h: 0,
		ForecastedRAMIn2h: 0,
	}
}

// executeScaling performs the actual scaling operation
func (e *ScalingEngine) executeScaling(rec ScaleRecommendation) error {
	switch rec.Action {
	case ScaleActionScaleUp:
		return e.scaleUp(rec)

	case ScaleActionScaleDown:
		return e.scaleDown(rec)

	case ScaleActionProvisionSpare:
		return e.provisionSpare(rec)

	default:
		return nil
	}
}

// scaleUp provisions new cloud nodes
func (e *ScalingEngine) scaleUp(rec ScaleRecommendation) error {
	logger.Info("Scaling UP", map[string]interface{}{
		"server_type": rec.ServerType,
		"count":       rec.Count,
		"reason":      rec.Reason,
		"urgency":     rec.Urgency,
	})

	for i := 0; i < rec.Count; i++ {
		// Provision new VM
		node, err := e.vmProvisioner.ProvisionNode(rec.ServerType)
		if err != nil {
			logger.Error("Failed to provision node", err, map[string]interface{}{
				"server_type": rec.ServerType,
				"attempt":     i + 1,
			})

			// Publish scaling event (failed)
			events.PublishScalingEvent("scale_up", "failed", err.Error())

			return fmt.Errorf("failed to provision node: %w", err)
		}

		logger.Info("Node provisioned successfully", map[string]interface{}{
			"node_id":     node.ID,
			"node_type":   rec.ServerType,
			"ip":          node.IPAddress,
			"ram_mb":      node.TotalRAMMB,
			"cost_eur_hr": node.HourlyCostEUR,
		})

		// Publish scaling event (success)
		events.PublishScalingEvent("scale_up", "success", node.ID)
	}

	// After successful scale-up, process the start queue
	// This will attempt to start any queued servers now that we have capacity
	e.processStartQueueAfterScaleUp()

	return nil
}

// scaleDown removes idle cloud nodes
func (e *ScalingEngine) scaleDown(rec ScaleRecommendation) error {
	logger.Info("Scaling DOWN", map[string]interface{}{
		"count":  rec.Count,
		"reason": rec.Reason,
	})

	// Get all cloud nodes
	cloudNodes := e.nodeRegistry.GetNodesByType("cloud")

	if len(cloudNodes) == 0 {
		logger.Warn("No cloud nodes to scale down", nil)
		return nil
	}

	// Find the least utilized node
	nodeToRemove := e.findLeastUtilizedNode(cloudNodes)

	if nodeToRemove == nil {
		logger.Warn("No suitable node found for scale down", nil)
		return nil
	}

	// Check if node has containers
	if nodeToRemove.ContainerCount > 0 {
		logger.Warn("Node has active containers, cannot scale down yet", map[string]interface{}{
			"node_id":         nodeToRemove.ID,
			"container_count": nodeToRemove.ContainerCount,
		})
		// TODO: Implement container draining/migration
		return fmt.Errorf("node has active containers: %s", nodeToRemove.ID)
	}

	// Decommission the node
	if err := e.vmProvisioner.DecommissionNode(nodeToRemove.ID, "reactive_policy"); err != nil {
		logger.Error("Failed to decommission node", err, map[string]interface{}{
			"node_id": nodeToRemove.ID,
		})

		events.PublishScalingEvent("scale_down", "failed", err.Error())
		return fmt.Errorf("failed to decommission node: %w", err)
	}

	logger.Info("Node scaled down successfully", map[string]interface{}{
		"node_id": nodeToRemove.ID,
	})

	events.PublishScalingEvent("scale_down", "success", nodeToRemove.ID)

	return nil
}

// provisionSpare provisions a spare node for hot-spare pool (B6)
func (e *ScalingEngine) provisionSpare(rec ScaleRecommendation) error {
	logger.Info("Provisioning spare node", map[string]interface{}{
		"server_type": rec.ServerType,
		"reason":      rec.Reason,
	})

	// Use smallest VM type for spares
	node, err := e.vmProvisioner.ProvisionSpareNode()
	if err != nil {
		logger.Error("Failed to provision spare node", err, nil)
		return fmt.Errorf("failed to provision spare: %w", err)
	}

	logger.Info("Spare node provisioned", map[string]interface{}{
		"node_id": node.ID,
	})

	events.PublishScalingEvent("provision_spare", "success", node.ID)

	return nil
}

// findLeastUtilizedNode finds the cloud node with lowest utilization
func (e *ScalingEngine) findLeastUtilizedNode(nodes []*Node) *Node {
	if len(nodes) == 0 {
		return nil
	}

	// Sort by container count (least utilized first)
	var leastUtilized *Node
	minContainers := int(^uint(0) >> 1) // Max int

	for _, node := range nodes {
		if node.ContainerCount < minContainers {
			minContainers = node.ContainerCount
			leastUtilized = node
		}
	}

	return leastUtilized
}

// executeConsolidation performs container migration and node decommissioning (B8)
func (e *ScalingEngine) executeConsolidation(plan ConsolidationPlan) error {
	logger.Info("Executing CONSOLIDATION", map[string]interface{}{
		"migrations":    len(plan.Migrations),
		"nodes_before":  len(plan.NodesToKeep) + len(plan.NodesToRemove),
		"nodes_after":   len(plan.NodesToKeep),
		"node_savings":  plan.NodeSavings,
		"cost_savings":  plan.EstimatedCostSavings,
	})

	// 1. Execute migrations
	successfulMigrations := 0
	failedMigrations := 0

	for _, migration := range plan.Migrations {
		logger.Info("Migrating server", map[string]interface{}{
			"server_id":   migration.ServerID,
			"server_name": migration.ServerName,
			"from_node":   migration.FromNode,
			"to_node":     migration.ToNode,
			"ram_mb":      migration.RAMMb,
			"players":     migration.PlayerCount,
		})

		if err := e.migrateServer(migration); err != nil {
			logger.Error("Migration failed", err, map[string]interface{}{
				"server_id":   migration.ServerID,
				"server_name": migration.ServerName,
			})
			failedMigrations++

			// Publish event
			events.PublishScalingEvent("consolidation_migration_failed", migration.ServerID, err.Error())

			continue // Try other migrations
		}

		successfulMigrations++

		// Publish event
		events.PublishScalingEvent("consolidation_migration_success", migration.ServerID, "")
	}

	logger.Info("Migrations completed", map[string]interface{}{
		"successful": successfulMigrations,
		"failed":     failedMigrations,
	})

	// If any migrations failed, abort decommissioning (safer)
	if failedMigrations > 0 {
		logger.Warn("Aborting node decommissioning due to failed migrations", map[string]interface{}{
			"failed_count": failedMigrations,
		})
		return fmt.Errorf("consolidation partially failed: %d migrations failed", failedMigrations)
	}

	// 2. Decommission empty nodes
	for _, nodeID := range plan.NodesToRemove {
		logger.Info("Decommissioning node", map[string]interface{}{
			"node_id": nodeID,
		})

		if err := e.vmProvisioner.DecommissionNode(nodeID, "consolidation_policy"); err != nil {
			logger.Error("Failed to decommission node", err, map[string]interface{}{
				"node_id": nodeID,
			})

			// Publish event
			events.PublishScalingEvent("consolidation_decommission_failed", nodeID, err.Error())

			// Don't fail entire consolidation if one decommission fails
			continue
		}

		logger.Info("Node decommissioned successfully", map[string]interface{}{
			"node_id": nodeID,
		})

		// Publish event
		events.PublishScalingEvent("consolidation_decommission_success", nodeID, "")
	}

	// Publish overall consolidation success
	events.PublishScalingEvent("consolidation_complete", fmt.Sprintf("saved_%d_nodes", plan.NodeSavings), "")
	events.PublishConsolidationCompleted(successfulMigrations, failedMigrations)

	logger.Info("Consolidation completed successfully", map[string]interface{}{
		"nodes_decommissioned": len(plan.NodesToRemove),
		"cost_savings_eur_hr":  plan.EstimatedCostSavings,
		"cost_savings_eur_mo":  plan.EstimatedCostSavings * 730,
	})

	return nil
}

// migrateServer migrates a single server from one node to another
// This is a helper method that delegates to Conductor.MigrateServer()
func (e *ScalingEngine) migrateServer(migration Migration) error {
	if e.conductor == nil {
		return fmt.Errorf("conductor not set, cannot perform migration")
	}

	// Type-assert velocityClient to VelocityRemoteClient
	var velocityRemoteClient VelocityRemoteClient
	if e.velocityClient != nil {
		var ok bool
		velocityRemoteClient, ok = e.velocityClient.(VelocityRemoteClient)
		if !ok {
			return fmt.Errorf("velocityClient does not implement VelocityRemoteClient interface")
		}
	}

	return e.conductor.MigrateServer(
		migration.ServerID,
		migration.FromNode,
		migration.ToNode,
		velocityRemoteClient,
	)
}

// GetStatus returns the current scaling engine status
func (e *ScalingEngine) GetStatus() ScalingEngineStatus {
	ctx := e.buildScalingContext()

	capacityPercent := 0.0
	if ctx.FleetStats.TotalRAMMB > 0 {
		capacityPercent = (float64(ctx.FleetStats.AllocatedRAMMB) / float64(ctx.FleetStats.TotalRAMMB)) * 100
	}

	policyNames := make([]string, len(e.policies))
	for i, p := range e.policies {
		policyNames[i] = p.Name()
	}

	return ScalingEngineStatus{
		Enabled:         e.enabled,
		Policies:        policyNames,
		TotalRAMMB:      ctx.FleetStats.TotalRAMMB,
		AllocatedRAMMB:  ctx.FleetStats.AllocatedRAMMB,
		CapacityPercent: capacityPercent,
		DedicatedNodes:  len(ctx.DedicatedNodes),
		CloudNodes:      len(ctx.CloudNodes),
		TotalNodes:      len(ctx.DedicatedNodes) + len(ctx.CloudNodes),
	}
}

// processStartQueueAfterScaleUp checks the start queue and attempts to start queued servers
// This is called after a successful scale-up to immediately utilize new capacity
func (e *ScalingEngine) processStartQueueAfterScaleUp() {
	if e.startQueue.Size() == 0 {
		logger.Info("No servers in queue after scale-up", nil)
		return
	}

	logger.Info("Processing start queue after scale-up", map[string]interface{}{
		"queue_size":     e.startQueue.Size(),
		"total_required": e.startQueue.GetTotalRequiredRAM(),
	})

	// Get current fleet stats
	fleetStats := e.nodeRegistry.GetFleetStats()

	logger.Info("New capacity available after scale-up", map[string]interface{}{
		"available_ram_mb": fleetStats.AvailableRAMMB,
		"usable_ram_mb":    fleetStats.UsableRAMMB,
		"allocated_ram_mb": fleetStats.AllocatedRAMMB,
		"queue_size":       e.startQueue.Size(),
	})

	// Note: The actual server starts will be handled by the Conductor.ProcessStartQueue()
	// which is called periodically and has access to MinecraftService.
	// This method just logs that capacity is available for debugging.
}

// ScalingEngineStatus represents the current state of the scaling engine
type ScalingEngineStatus struct {
	Enabled         bool     `json:"enabled"`
	Policies        []string `json:"policies"`
	TotalRAMMB      int      `json:"total_ram_mb"`
	AllocatedRAMMB  int      `json:"allocated_ram_mb"`
	CapacityPercent float64  `json:"capacity_percent"`
	DedicatedNodes  int      `json:"dedicated_nodes"`
	CloudNodes      int      `json:"cloud_nodes"`
	TotalNodes      int      `json:"total_nodes"`
}
