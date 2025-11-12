package conductor

import (
	"fmt"
	"sync"
	"time"

	"github.com/payperplay/hosting/internal/cloud"
	"github.com/payperplay/hosting/pkg/config"
	"github.com/payperplay/hosting/pkg/logger"
)

// ReactivePolicy scales based on CURRENT capacity utilization (B5)
// This is the foundation - it reacts to what's happening RIGHT NOW
type ReactivePolicy struct {
	ScaleUpThreshold   float64       // Scale up when capacity > 85%
	ScaleDownThreshold float64       // Scale down when capacity < 30%
	CooldownPeriod     time.Duration // Wait 5 minutes between actions
	MinCloudNodes      int           // Never scale below this (0 = can scale to zero)
	MaxCloudNodes      int           // Never scale above this
	lastScaleAction    time.Time
	lastScaleType      ScaleAction

	// Dynamic server type selection (queries Hetzner API)
	cloudProvider   cloud.CloudProvider
	serverTypeCache []*cloud.ServerType
	cacheExpiry     time.Time
	cacheMutex      sync.RWMutex
}

// NewReactivePolicy creates a new reactive scaling policy
func NewReactivePolicy(cloudProvider cloud.CloudProvider) *ReactivePolicy {
	return &ReactivePolicy{
		ScaleUpThreshold:   85.0,              // Scale up at 85% capacity
		ScaleDownThreshold: 30.0,              // Scale down below 30% capacity
		CooldownPeriod:     5 * time.Minute,   // 5 minute cooldown
		MinCloudNodes:      0,                  // Can scale to zero
		MaxCloudNodes:      10,                 // Max 10 cloud nodes
		lastScaleAction:    time.Time{},
		lastScaleType:      ScaleActionNone,
		cloudProvider:      cloudProvider,
		serverTypeCache:    nil,
		cacheExpiry:        time.Time{},
	}
}

func (p *ReactivePolicy) Name() string {
	return "reactive"
}

func (p *ReactivePolicy) Priority() int {
	return 10 // Medium priority (Predictive will be 20, SparePool will be 5, Consolidation will be 1)
}

// ShouldConsolidate - ReactivePolicy does not handle consolidation (delegated to ConsolidationPolicy)
func (p *ReactivePolicy) ShouldConsolidate(ctx ScalingContext) (bool, ConsolidationPlan) {
	return false, ConsolidationPlan{} // ReactivePolicy focuses on capacity, not cost optimization
}

// ShouldScaleUp checks if we need more capacity
func (p *ReactivePolicy) ShouldScaleUp(ctx ScalingContext) (bool, ScaleRecommendation) {
	// CRITICAL: If servers are queued but NO worker nodes exist, provision immediately
	// This handles the case where MC containers need worker nodes but none are available
	// FIX: Only provision if ZERO worker nodes (including unhealthy ones being provisioned)
	if ctx.QueuedServerCount > 0 && len(ctx.WorkerNodes) == 0 {
		logger.Info("ReactivePolicy: Queued servers need worker node - provisioning immediately", map[string]interface{}{
			"queued_servers":  ctx.QueuedServerCount,
			"worker_nodes":    len(ctx.WorkerNodes),
		})

		serverType := p.selectServerType(ctx, 0)
		p.lastScaleAction = time.Now()
		p.lastScaleType = ScaleActionScaleUp

		return true, ScaleRecommendation{
			Action:     ScaleActionScaleUp,
			ServerType: serverType,
			Count:      1,
			Reason:     fmt.Sprintf("Queued servers (%d) require worker node - no worker nodes available", ctx.QueuedServerCount),
			Urgency:    UrgencyHigh, // High urgency - users are waiting
		}
	}

	// Check cooldown period
	if time.Since(p.lastScaleAction) < p.CooldownPeriod {
		logger.Debug("ReactivePolicy: Cooldown active", map[string]interface{}{
			"time_since_last": time.Since(p.lastScaleAction).String(),
			"cooldown_period": p.CooldownPeriod.String(),
		})
		return false, ScaleRecommendation{Action: ScaleActionNone}
	}

	// Check if we've hit max cloud nodes
	if len(ctx.CloudNodes) >= p.MaxCloudNodes {
		logger.Warn("ReactivePolicy: Max cloud nodes reached", map[string]interface{}{
			"current_nodes": len(ctx.CloudNodes),
			"max_nodes":     p.MaxCloudNodes,
		})
		return false, ScaleRecommendation{Action: ScaleActionNone}
	}

	// PROPORTIONAL OVERHEAD SYSTEM: Calculate based on TOTAL RAM (not UsableRAM!)
	// With proportional overhead, we allocate based on BOOKED RAM (e.g. 8GB server = 8192MB allocation)
	// The actual container gets less (ActualRAM), but capacity planning uses TOTAL RAM
	// This is the CORRECT way - we no longer pre-reserve system overhead!
	if ctx.FleetStats.TotalRAMMB == 0 {
		return false, ScaleRecommendation{Action: ScaleActionNone}
	}

	capacityPercent := (float64(ctx.FleetStats.AllocatedRAMMB) / float64(ctx.FleetStats.TotalRAMMB)) * 100

	logger.Debug("ReactivePolicy: Capacity check", map[string]interface{}{
		"capacity_percent":      capacityPercent,
		"scale_up_threshold":    p.ScaleUpThreshold,
		"allocated_ram_mb":      ctx.FleetStats.AllocatedRAMMB,
		"total_ram_mb":          ctx.FleetStats.TotalRAMMB,
		"system_reserved_mb":    ctx.FleetStats.SystemReservedRAMMB,
		"queued_servers":        ctx.QueuedServerCount,
		"worker_nodes":          len(ctx.WorkerNodes),
	})

	// Check if we need to scale up
	if capacityPercent > p.ScaleUpThreshold {
		urgency := p.calculateUrgency(capacityPercent)
		serverType := p.selectServerType(ctx, capacityPercent)

		p.lastScaleAction = time.Now()
		p.lastScaleType = ScaleActionScaleUp

		return true, ScaleRecommendation{
			Action:     ScaleActionScaleUp,
			ServerType: serverType,
			Count:      1, // Scale one at a time
			Reason: fmt.Sprintf("Capacity at %.1f%% (threshold: %.1f%%)",
				capacityPercent, p.ScaleUpThreshold),
			Urgency: urgency,
		}
	}

	return false, ScaleRecommendation{Action: ScaleActionNone}
}

// ShouldScaleDown checks if we can remove capacity
func (p *ReactivePolicy) ShouldScaleDown(ctx ScalingContext) (bool, ScaleRecommendation) {
	// Don't scale down if we have no cloud nodes
	if len(ctx.CloudNodes) <= p.MinCloudNodes {
		return false, ScaleRecommendation{Action: ScaleActionNone}
	}

	// Check cooldown period
	if time.Since(p.lastScaleAction) < p.CooldownPeriod {
		return false, ScaleRecommendation{Action: ScaleActionNone}
	}

	// Don't scale down immediately after scaling up (prevent flapping)
	if p.lastScaleType == ScaleActionScaleUp && time.Since(p.lastScaleAction) < 20*time.Minute {
		logger.Debug("ReactivePolicy: Recently scaled up, waiting", map[string]interface{}{
			"time_since_scale_up": time.Since(p.lastScaleAction).String(),
		})
		return false, ScaleRecommendation{Action: ScaleActionNone}
	}

	// PROPORTIONAL OVERHEAD SYSTEM: Calculate based on TOTAL RAM (not UsableRAM!)
	if ctx.FleetStats.TotalRAMMB == 0 {
		return false, ScaleRecommendation{Action: ScaleActionNone}
	}

	capacityPercent := (float64(ctx.FleetStats.AllocatedRAMMB) / float64(ctx.FleetStats.TotalRAMMB)) * 100

	logger.Debug("ReactivePolicy: Scale down check", map[string]interface{}{
		"capacity_percent":      capacityPercent,
		"scale_down_threshold":  p.ScaleDownThreshold,
		"total_ram_mb":          ctx.FleetStats.TotalRAMMB,
		"allocated_ram_mb":      ctx.FleetStats.AllocatedRAMMB,
		"cloud_nodes":           len(ctx.CloudNodes),
	})

	// Check if we can scale down
	if capacityPercent < p.ScaleDownThreshold {
		// Additional safety: check if we've been below threshold for a while
		// This prevents flapping during minor fluctuations
		// TODO: Implement time-based tracking (need historical data)

		p.lastScaleAction = time.Now()
		p.lastScaleType = ScaleActionScaleDown

		return true, ScaleRecommendation{
			Action:     ScaleActionScaleDown,
			Count:      1, // Remove one at a time
			Reason: fmt.Sprintf("Capacity at %.1f%% (threshold: %.1f%%)",
				capacityPercent, p.ScaleDownThreshold),
			Urgency: UrgencyLow,
		}
	}

	return false, ScaleRecommendation{Action: ScaleActionNone}
}

// calculateUrgency determines how quickly we need to act
func (p *ReactivePolicy) calculateUrgency(capacityPercent float64) Urgency {
	if capacityPercent > 95 {
		return UrgencyCritical // Emergency!
	} else if capacityPercent > 92 {
		return UrgencyHigh // Act immediately
	} else if capacityPercent > 88 {
		return UrgencyMedium // Act within 2 minutes
	}
	return UrgencyLow // Can wait a bit
}

// getAvailableServerTypes fetches and caches server types from Hetzner API
// Cache expires after 1 hour to get fresh pricing and availability
func (p *ReactivePolicy) getAvailableServerTypes() ([]*cloud.ServerType, error) {
	p.cacheMutex.RLock()
	if p.serverTypeCache != nil && time.Now().Before(p.cacheExpiry) {
		defer p.cacheMutex.RUnlock()
		return p.serverTypeCache, nil
	}
	p.cacheMutex.RUnlock()

	// Cache expired or not populated, fetch from API
	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()

	// Double-check after acquiring write lock
	if p.serverTypeCache != nil && time.Now().Before(p.cacheExpiry) {
		return p.serverTypeCache, nil
	}

	if p.cloudProvider == nil {
		// Fallback if cloudProvider not available (shouldn't happen)
		logger.Warn("CloudProvider not available, using fallback server types", nil)
		return []*cloud.ServerType{
			{Name: "cpx22", RAMMB: 4096, Cores: 2, HourlyCostEUR: 0.0096},   // NBG1 pricing (CPX2 series)
			{Name: "cpx32", RAMMB: 8192, Cores: 4, HourlyCostEUR: 0.0168},   // NBG1 pricing (CPX2 series)
			{Name: "cpx42", RAMMB: 16384, Cores: 8, HourlyCostEUR: 0.0312},  // NBG1 pricing (CPX2 series)
			{Name: "cpx52", RAMMB: 24576, Cores: 16, HourlyCostEUR: 0.0624}, // NBG1 pricing (CPX2 series)
		}, nil
	}

	serverTypes, err := p.cloudProvider.GetServerTypes()
	if err != nil {
		logger.Error("Failed to fetch server types from Hetzner", err, nil)
		// Return fallback on error (NBG1 pricing - CPX2 series)
		return []*cloud.ServerType{
			{Name: "cpx22", RAMMB: 4096, Cores: 2, HourlyCostEUR: 0.0096},   // CPX2 series
			{Name: "cpx32", RAMMB: 8192, Cores: 4, HourlyCostEUR: 0.0168},   // CPX2 series
			{Name: "cpx42", RAMMB: 16384, Cores: 8, HourlyCostEUR: 0.0312},  // CPX2 series
			{Name: "cpx52", RAMMB: 24576, Cores: 16, HourlyCostEUR: 0.0624}, // CPX2 series
		}, err
	}

	// Filter to only CPX2-series (shared CPU, latest generation)
	// CPX2 series: cpx12, cpx22, cpx32, cpx42, cpx52, cpx62 (ends with '2')
	// Exclude old CPX1 series: cpx11, cpx21, cpx31, cpx41, cpx51 (ends with '1')
	filtered := make([]*cloud.ServerType, 0)
	for _, st := range serverTypes {
		// Only use CPX2-series (newer generation with better performance)
		if len(st.Name) >= 4 && st.Name[:3] == "cpx" {
			// Check if name ends with '2' (CPX2 generation)
			if st.Name[len(st.Name)-1] == '2' {
				filtered = append(filtered, st)
			}
		}
	}

	p.serverTypeCache = filtered
	p.cacheExpiry = time.Now().Add(1 * time.Hour) // Cache for 1 hour

	logger.Info("Server types cached", map[string]interface{}{
		"count": len(filtered),
		"types": func() []string {
			names := make([]string, len(filtered))
			for i, st := range filtered {
				names[i] = st.Name
			}
			return names
		}(),
	})

	return filtered, nil
}

// selectServerType chooses the appropriate VM size based on needs
// Tier-aware implementation with queue analysis and perfect packing
func (p *ReactivePolicy) selectServerType(ctx ScalingContext, capacityPercent float64) string {
	serverTypes, err := p.getAvailableServerTypes()
	if err != nil || len(serverTypes) == 0 {
		logger.Warn("Using fallback server type", map[string]interface{}{"error": err})
		return "cpx22" // Fallback to CPX2 series (4GB)
	}

	// Filter by configured min/max RAM
	cfg := config.AppConfig
	filtered := p.filterByRAMConstraints(serverTypes, cfg.WorkerNodeMinRAMMB, cfg.WorkerNodeMaxRAMMB)
	if len(filtered) == 0 {
		logger.Warn("No server types match RAM constraints, using unfiltered", map[string]interface{}{
			"min_ram": cfg.WorkerNodeMinRAMMB,
			"max_ram": cfg.WorkerNodeMaxRAMMB,
		})
		filtered = serverTypes
	}

	// Strategy selection based on config
	var selectedType string
	switch cfg.WorkerNodeStrategy {
	case "queue-based":
		selectedType = p.selectByQueue(ctx, filtered)
	case "capacity-based":
		selectedType = p.selectByCapacity(ctx, capacityPercent, filtered)
	default: // "tier-aware" (default)
		// Queue-based has priority if queue exists
		if ctx.QueuedServerCount > 0 {
			selectedType = p.selectByQueue(ctx, filtered)
		} else {
			selectedType = p.selectByCapacity(ctx, capacityPercent, filtered)
		}
	}

	logger.Info("Selected server type (tier-aware)", map[string]interface{}{
		"type":         selectedType,
		"strategy":     cfg.WorkerNodeStrategy,
		"capacity_pct": capacityPercent,
		"queue_count":  ctx.QueuedServerCount,
	})

	return selectedType
}

// filterByRAMConstraints filters server types by min/max RAM limits
func (p *ReactivePolicy) filterByRAMConstraints(serverTypes []*cloud.ServerType, minRAMMB, maxRAMMB int) []*cloud.ServerType {
	filtered := make([]*cloud.ServerType, 0)
	for _, st := range serverTypes {
		if st.RAMMB >= minRAMMB && st.RAMMB <= maxRAMMB {
			filtered = append(filtered, st)
		}
	}
	return filtered
}

// selectByQueue selects node type based on queued servers (multi-tenant packing)
func (p *ReactivePolicy) selectByQueue(ctx ScalingContext, serverTypes []*cloud.ServerType) string {
	cfg := config.AppConfig

	// TODO: Calculate total RAM needed from queue when StartQueue is available
	// For now, estimate based on QueuedServerCount
	// Assume average 4GB per queued server
	totalQueueRAM := ctx.QueuedServerCount * 4096

	// Add buffer for growth
	bufferMultiplier := 1.0 + (cfg.WorkerNodeBufferPercent / 100.0)
	targetRAM := int(float64(totalQueueRAM) * bufferMultiplier)

	logger.Info("Queue-based node selection", map[string]interface{}{
		"total_queue_ram": totalQueueRAM,
		"buffer_percent":  cfg.WorkerNodeBufferPercent,
		"target_ram":      targetRAM,
	})

	// Find smallest node that fits target RAM + buffer
	var bestType *cloud.ServerType
	for _, st := range serverTypes {
		if st.RAMMB >= targetRAM {
			if bestType == nil || st.RAMMB < bestType.RAMMB {
				bestType = st
			}
		}
	}

	// If target is too large, use largest available
	if bestType == nil && len(serverTypes) > 0 {
		bestType = serverTypes[0]
		for _, st := range serverTypes {
			if st.RAMMB > bestType.RAMMB {
				bestType = st
			}
		}
	}

	if bestType != nil {
		return bestType.Name
	}

	return "cpx42" // Fallback to CPX2 series (16GB standard worker node)
}

// selectByCapacity selects node type based on current capacity pressure
func (p *ReactivePolicy) selectByCapacity(ctx ScalingContext, capacityPercent float64, serverTypes []*cloud.ServerType) string {
	cfg := config.AppConfig

	// Determine target RAM based on urgency
	var targetRAM int
	if capacityPercent > 95 {
		// Emergency: Use maximum allowed
		targetRAM = cfg.WorkerNodeMaxRAMMB
	} else if capacityPercent > 90 {
		// High urgency: Use 8GB
		targetRAM = 8192
	} else {
		// Normal: Use minimum allowed (cost-effective)
		targetRAM = cfg.WorkerNodeMinRAMMB
	}

	logger.Info("Capacity-based node selection", map[string]interface{}{
		"capacity_percent": capacityPercent,
		"target_ram":       targetRAM,
	})

	// Find closest match to target RAM
	return p.findClosestServerType(serverTypes, targetRAM)
}

// findClosestServerType finds the server type closest to target RAM
func (p *ReactivePolicy) findClosestServerType(serverTypes []*cloud.ServerType, targetRAM int) string {
	if len(serverTypes) == 0 {
		return "cpx42" // Fallback to CPX2 series (16GB)
	}

	var bestType *cloud.ServerType
	minDiff := int(^uint(0) >> 1) // Max int

	for _, st := range serverTypes {
		// Prefer types >= targetRAM, but allow smaller if no match
		diff := st.RAMMB - targetRAM
		if diff >= 0 {
			// Type is >= target, prefer smallest that fits
			if diff < minDiff {
				minDiff = diff
				bestType = st
			}
		} else {
			// Type is < target, only use if no >= match found
			if bestType == nil {
				bestType = st
			}
		}
	}

	if bestType == nil {
		bestType = serverTypes[0]
	}

	return bestType.Name
}

// SetCooldownPeriod allows adjusting the cooldown period (for testing)
func (p *ReactivePolicy) SetCooldownPeriod(duration time.Duration) {
	p.CooldownPeriod = duration
}

// SetThresholds allows adjusting thresholds (for testing/tuning)
func (p *ReactivePolicy) SetThresholds(scaleUp, scaleDown float64) {
	p.ScaleUpThreshold = scaleUp
	p.ScaleDownThreshold = scaleDown
}

// SetNodeLimits allows adjusting min/max cloud nodes
func (p *ReactivePolicy) SetNodeLimits(min, max int) {
	p.MinCloudNodes = min
	p.MaxCloudNodes = max
}
