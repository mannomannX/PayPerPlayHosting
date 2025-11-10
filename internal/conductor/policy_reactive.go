package conductor

import (
	"fmt"
	"time"

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
}

// NewReactivePolicy creates a new reactive scaling policy
func NewReactivePolicy() *ReactivePolicy {
	return &ReactivePolicy{
		ScaleUpThreshold:   85.0,              // Scale up at 85% capacity
		ScaleDownThreshold: 30.0,              // Scale down below 30% capacity
		CooldownPeriod:     5 * time.Minute,   // 5 minute cooldown
		MinCloudNodes:      0,                  // Can scale to zero
		MaxCloudNodes:      10,                 // Max 10 cloud nodes
		lastScaleAction:    time.Time{},
		lastScaleType:      ScaleActionNone,
	}
}

func (p *ReactivePolicy) Name() string {
	return "reactive"
}

func (p *ReactivePolicy) Priority() int {
	return 10 // Medium priority (Predictive will be 20, SparePool will be 5)
}

// ShouldScaleUp checks if we need more capacity
func (p *ReactivePolicy) ShouldScaleUp(ctx ScalingContext) (bool, ScaleRecommendation) {
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

// selectServerType chooses the appropriate VM size based on needs
func (p *ReactivePolicy) selectServerType(ctx ScalingContext, capacityPercent float64) string {
	// For emergency situations, use larger VMs
	if capacityPercent > 95 {
		return "cx31" // 2 vCPU, 8GB RAM - larger capacity boost
	}

	// For high capacity, use medium VMs
	if capacityPercent > 90 {
		return "cx21" // 2 vCPU, 4GB RAM - standard cloud node
	}

	// For normal scaling, use small VMs (most cost-effective)
	return "cx21" // 2 vCPU, 4GB RAM - default choice
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
