package conductor

import "time"

// ScalingPolicy defines an interface for different scaling strategies
// Implementations: ReactivePolicy (B5), SparePoolPolicy (B6), PredictivePolicy (B7)
type ScalingPolicy interface {
	// Name returns the policy name for logging
	Name() string

	// Priority returns the policy priority (higher = checked first)
	// Predictive (20) > Reactive (10) > SparePool (5)
	Priority() int

	// ShouldScaleUp returns true if more capacity is needed
	ShouldScaleUp(ctx ScalingContext) (bool, ScaleRecommendation)

	// ShouldScaleDown returns true if we have excess capacity
	ShouldScaleDown(ctx ScalingContext) (bool, ScaleRecommendation)
}

// ScalingContext provides all data needed for scaling decisions
type ScalingContext struct {
	// Fleet Statistics (from NodeRegistry)
	FleetStats FleetStats

	// Current Nodes
	DedicatedNodes []*Node // Always-on base capacity
	CloudNodes     []*Node // Dynamic capacity

	// Historical Data (from InfluxDB - for advanced policies)
	AverageRAMUsageLast1h  float64
	AverageRAMUsageLast24h float64
	PeakRAMUsageLast24h    float64

	// Time Context (for predictive policies)
	CurrentTime time.Time
	IsWeekend   bool
	IsHoliday   bool

	// Forecast Data (for B7 - Predictive Policy)
	ForecastedRAMIn1h float64
	ForecastedRAMIn2h float64
}

// ScaleRecommendation describes what action to take
type ScaleRecommendation struct {
	Action     ScaleAction
	ServerType string  // Which VM size: "cx11", "cx21", etc.
	Count      int     // How many VMs
	Reason     string  // Human-readable reason for logging
	Urgency    Urgency // How fast to act
}

// ScaleAction defines the type of scaling operation
type ScaleAction string

const (
	ScaleActionNone           ScaleAction = "none"
	ScaleActionScaleUp        ScaleAction = "scale_up"
	ScaleActionScaleDown      ScaleAction = "scale_down"
	ScaleActionProvisionSpare ScaleAction = "provision_spare" // For B6
)

// Urgency defines how quickly we need to act
type Urgency string

const (
	UrgencyLow      Urgency = "low"      // Can wait 5-10 minutes
	UrgencyMedium   Urgency = "medium"   // Should act within 2 minutes
	UrgencyHigh     Urgency = "high"     // Act immediately
	UrgencyCritical Urgency = "critical" // Emergency (>95% capacity)
)
