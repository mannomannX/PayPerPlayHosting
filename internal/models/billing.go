package models

import (
	"time"

	"gorm.io/gorm"
)

// BillingEventType represents the type of billing event
type BillingEventType string

const (
	EventServerStarted BillingEventType = "server_started"
	EventServerStopped BillingEventType = "server_stopped"
	EventPhaseChanged  BillingEventType = "phase_changed"
)

// BillingEvent tracks every billable event for accurate cost calculation
type BillingEvent struct {
	gorm.Model
	ID string `gorm:"primaryKey;size:64"`

	// Server reference
	ServerID   string `gorm:"index;not null"`
	ServerName string `gorm:"size:256"`
	OwnerID    string `gorm:"index;not null"`

	// Event details
	EventType BillingEventType `gorm:"not null"`
	Timestamp time.Time        `gorm:"index;not null"`

	// Resource configuration at time of event
	RAMMb            int            `gorm:"not null"`
	StorageGB        float64        // Disk usage in GB
	LifecyclePhase   LifecyclePhase `gorm:"not null"`
	PreviousPhase    LifecyclePhase
	MinecraftVersion string `gorm:"size:64"`

	// Cost metadata
	HourlyRateEUR float64 // Rate at time of event (for historical accuracy)
	DailyRateEUR  float64 // For storage billing (sleep phase)
}

// UsageSession represents a continuous period of server activity
type UsageSession struct {
	gorm.Model
	ID string `gorm:"primaryKey;size:64"`

	// Server reference
	ServerID   string `gorm:"index;not null"`
	ServerName string `gorm:"size:256"`
	OwnerID    string `gorm:"index;not null"`

	// Session timing
	StartedAt time.Time  `gorm:"not null;index"`
	StoppedAt *time.Time `gorm:"index"`

	// Resource configuration
	RAMMb            int     `gorm:"not null"`
	StorageGB        float64 // Average storage during session
	MinecraftVersion string  `gorm:"size:64"`

	// Calculated costs
	DurationSeconds int     // Total session duration
	CostEUR         float64 // Total cost for this session
	HourlyRateEUR   float64 // Rate used for calculation
}

// CostSummary provides aggregated cost information for a server
type CostSummary struct {
	ServerID   string  `json:"server_id"`
	ServerName string  `json:"server_name"`
	OwnerID    string  `json:"owner_id"`
	RAMMb      int     `json:"ram_mb"`
	StorageGB  float64 `json:"storage_gb"`

	// Current month costs
	ActiveCostEUR  float64 `json:"active_cost_eur"`  // Phase 1: Running time
	SleepCostEUR   float64 `json:"sleep_cost_eur"`   // Phase 2: Storage while stopped
	ArchiveCostEUR float64 `json:"archive_cost_eur"` // Phase 3: Always 0 (free)
	TotalCostEUR   float64 `json:"total_cost_eur"`

	// Time breakdown
	ActiveSeconds  int `json:"active_seconds"`
	SleepSeconds   int `json:"sleep_seconds"`
	ArchiveSeconds int `json:"archive_seconds"`

	// Live session (if running)
	CurrentSessionStartedAt *time.Time `json:"current_session_started_at,omitempty"`
	CurrentSessionCostEUR   float64    `json:"current_session_cost_eur"`

	// Forecast
	ForecastNextMonthEUR float64 `json:"forecast_next_month_eur"`
}

// PricingConfig holds the current pricing rates
type PricingConfig struct {
	// Phase 1: Active (Running)
	ActiveRateEURPerGBHour float64 `json:"active_rate_eur_per_gb_hour"` // Default: 0.02

	// Phase 2: Sleep (Stopped < 48h)
	SleepRateEURPerGBDay float64 `json:"sleep_rate_eur_per_gb_day"` // Default: 0.00333 (~0.10/month)

	// Phase 3: Archive (Stopped > 48h)
	ArchiveRateEURPerGBDay float64 `json:"archive_rate_eur_per_gb_day"` // Default: 0.00 (free)
}

// DefaultPricingConfig returns the default pricing configuration
func DefaultPricingConfig() PricingConfig {
	return PricingConfig{
		ActiveRateEURPerGBHour: 0.02,    // 2 cents per GB-hour
		SleepRateEURPerGBDay:   0.00333, // ~3.3 millicents per GB-day (~0.10/month)
		ArchiveRateEURPerGBDay: 0.00,    // Free
	}
}

// CalculateActiveMinuteRate calculates the per-minute rate for active servers
func (p PricingConfig) CalculateActiveMinuteRate(ramGB float64) float64 {
	return (p.ActiveRateEURPerGBHour * ramGB) / 60.0
}
