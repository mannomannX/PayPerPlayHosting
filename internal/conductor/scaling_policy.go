package conductor

import "time"

// ScalingPolicy defines an interface for different scaling strategies
// Implementations: ReactivePolicy (B5), SparePoolPolicy (B6), PredictivePolicy (B7), ConsolidationPolicy (B8)
type ScalingPolicy interface {
	// Name returns the policy name for logging
	Name() string

	// Priority returns the policy priority (higher = checked first)
	// Predictive (20) > Reactive (10) > SparePool (5) > Consolidation (1)
	Priority() int

	// ShouldScaleUp returns true if more capacity is needed
	ShouldScaleUp(ctx ScalingContext) (bool, ScaleRecommendation)

	// ShouldScaleDown returns true if we have excess capacity
	ShouldScaleDown(ctx ScalingContext) (bool, ScaleRecommendation)

	// ShouldConsolidate returns true if containers should be migrated for cost optimization (B8)
	ShouldConsolidate(ctx ScalingContext) (bool, ConsolidationPlan)
}

// ScalingContext provides all data needed for scaling decisions
type ScalingContext struct {
	// Fleet Statistics (from NodeRegistry)
	FleetStats FleetStats

	// Current Nodes
	DedicatedNodes []*Node // Always-on base capacity
	CloudNodes     []*Node // Dynamic capacity

	// Container Registry (for B8 - Consolidation Policy)
	ContainerRegistry *ContainerRegistry

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
	ScaleActionConsolidate    ScaleAction = "consolidate"     // For B8
)

// Urgency defines how quickly we need to act
type Urgency string

const (
	UrgencyLow      Urgency = "low"      // Can wait 5-10 minutes
	UrgencyMedium   Urgency = "medium"   // Should act within 2 minutes
	UrgencyHigh     Urgency = "high"     // Act immediately
	UrgencyCritical Urgency = "critical" // Emergency (>95% capacity)
)

// ConsolidationPlan describes container migrations for cost optimization (B8)
type ConsolidationPlan struct {
	Migrations            []Migration // List of servers to migrate
	NodesToKeep           []string    // Node IDs to keep running
	NodesToRemove         []string    // Node IDs to decommission after migration
	NodeSavings           int         // Number of nodes saved
	EstimatedCostSavings  float64     // EUR per hour saved
	Reason                string      // Human-readable reason
}

// Migration describes a single server migration
type Migration struct {
	ServerID    string // Server to migrate
	ServerName  string // Server name for logging
	FromNode    string // Source node ID
	ToNode      string // Target node ID
	RAMMb       int    // Server RAM size
	PlayerCount int    // Current player count (0 = safe to migrate)
}
