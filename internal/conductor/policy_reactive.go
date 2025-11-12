package conductor

import (
	"fmt"
	"sync"
	"time"

	"github.com/payperplay/hosting/internal/cloud"
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

	// Calculate current capacity utilization (based on USABLE RAM, not total)
	// This respects system reserve and only considers RAM available for containers
	if ctx.FleetStats.UsableRAMMB == 0 {
		return false, ScaleRecommendation{Action: ScaleActionNone}
	}

	capacityPercent := (float64(ctx.FleetStats.AllocatedRAMMB) / float64(ctx.FleetStats.UsableRAMMB)) * 100

	logger.Debug("ReactivePolicy: Capacity check", map[string]interface{}{
		"capacity_percent":      capacityPercent,
		"scale_up_threshold":    p.ScaleUpThreshold,
		"allocated_ram_mb":      ctx.FleetStats.AllocatedRAMMB,
		"usable_ram_mb":         ctx.FleetStats.UsableRAMMB,
		"system_reserved_mb":    ctx.FleetStats.SystemReservedRAMMB,
		"total_ram_mb":          ctx.FleetStats.TotalRAMMB,
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

	// Calculate current capacity utilization (based on USABLE RAM, not total)
	if ctx.FleetStats.UsableRAMMB == 0 {
		return false, ScaleRecommendation{Action: ScaleActionNone}
	}

	capacityPercent := (float64(ctx.FleetStats.AllocatedRAMMB) / float64(ctx.FleetStats.UsableRAMMB)) * 100

	logger.Debug("ReactivePolicy: Scale down check", map[string]interface{}{
		"capacity_percent":      capacityPercent,
		"scale_down_threshold":  p.ScaleDownThreshold,
		"usable_ram_mb":         ctx.FleetStats.UsableRAMMB,
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
			{Name: "cpx11", RAMMB: 2048, Cores: 2, HourlyCostEUR: 0.0063},  // NBG1 pricing
			{Name: "cpx22", RAMMB: 4096, Cores: 2, HourlyCostEUR: 0.0096},  // NBG1 pricing (CPX2 - better value than cpx21)
			{Name: "cpx32", RAMMB: 8192, Cores: 4, HourlyCostEUR: 0.0168},  // NBG1 pricing (CPX2 - better value than cpx31)
			{Name: "cpx42", RAMMB: 16384, Cores: 8, HourlyCostEUR: 0.0312}, // NBG1 pricing (CPX2)
		}, nil
	}

	serverTypes, err := p.cloudProvider.GetServerTypes()
	if err != nil {
		logger.Error("Failed to fetch server types from Hetzner", err, nil)
		// Return fallback on error (NBG1 pricing - CPX2 series preferred)
		return []*cloud.ServerType{
			{Name: "cpx11", RAMMB: 2048, Cores: 2, HourlyCostEUR: 0.0063},  // CPX1
			{Name: "cpx22", RAMMB: 4096, Cores: 2, HourlyCostEUR: 0.0096},  // CPX2 - better value
			{Name: "cpx32", RAMMB: 8192, Cores: 4, HourlyCostEUR: 0.0168},  // CPX2 - better value
			{Name: "cpx42", RAMMB: 16384, Cores: 8, HourlyCostEUR: 0.0312}, // CPX2
		}, err
	}

	// Filter to only x86 CPX-series (shared CPU, regular purpose, non-deprecated)
	filtered := make([]*cloud.ServerType, 0)
	for _, st := range serverTypes {
		// Only use CPX-series (shared x86 for regular workloads)
		if len(st.Name) >= 3 && st.Name[:3] == "cpx" {
			filtered = append(filtered, st)
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
// Now queries Hetzner API dynamically instead of hardcoded values
func (p *ReactivePolicy) selectServerType(ctx ScalingContext, capacityPercent float64) string {
	serverTypes, err := p.getAvailableServerTypes()
	if err != nil || len(serverTypes) == 0 {
		logger.Warn("Using fallback server type", map[string]interface{}{"error": err})
		return "cpx21" // Fallback
	}

	// Determine required RAM based on urgency
	var requiredRAMMB int
	if capacityPercent > 95 {
		// Emergency: Need large capacity boost (8GB+)
		requiredRAMMB = 8192
	} else if capacityPercent > 90 {
		// High urgency: Need medium capacity (4GB+)
		requiredRAMMB = 4096
	} else {
		// Normal: Cost-effective option (2GB+)
		requiredRAMMB = 2048
	}

	// Find most cost-effective server type that meets requirements
	var bestType *cloud.ServerType
	for _, st := range serverTypes {
		if st.RAMMB >= requiredRAMMB {
			if bestType == nil || st.HourlyCostEUR < bestType.HourlyCostEUR {
				bestType = st
			}
		}
	}

	if bestType == nil {
		// If no exact match, use smallest available
		bestType = serverTypes[0]
		for _, st := range serverTypes {
			if st.RAMMB < bestType.RAMMB {
				bestType = st
			}
		}
	}

	logger.Info("Selected server type", map[string]interface{}{
		"type":          bestType.Name,
		"ram_mb":        bestType.RAMMB,
		"cores":         bestType.Cores,
		"cost_eur_hr":   bestType.HourlyCostEUR,
		"capacity_pct":  capacityPercent,
		"required_ram":  requiredRAMMB,
	})

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
