package models

import (
	"fmt"

	"github.com/payperplay/hosting/pkg/config"
)

// Tier names (standard)
const (
	TierMicro   = "micro"   // 2GB
	TierSmall   = "small"   // 4GB
	TierMedium  = "medium"  // 8GB
	TierLarge   = "large"   // 16GB
	TierXLarge  = "xlarge"  // 32GB
	TierCustom  = "custom"  // Non-standard RAM size
)

// Plan names
const (
	PlanPayPerPlay = "payperplay" // Aggressive optimization, cheapest
	PlanBalanced   = "balanced"   // Moderate optimization
	PlanReserved   = "reserved"   // No optimization, dedicated resources
)

// StandardTiers maps tier name to RAM in MB
var StandardTiers = map[string]int{
	TierMicro:  2048,  // 2GB
	TierSmall:  4096,  // 4GB
	TierMedium: 8192,  // 8GB
	TierLarge:  16384, // 16GB
	TierXLarge: 32768, // 32GB
}

// TierNames returns all standard tier names in order
func TierNames() []string {
	return []string{TierMicro, TierSmall, TierMedium, TierLarge, TierXLarge}
}

// ClassifyTier determines the tier for a given RAM size
func ClassifyTier(ramMB int) string {
	cfg := config.AppConfig

	// Check if it matches a standard tier
	for tier, ram := range StandardTiers {
		if ramMB == ram {
			return tier
		}
	}

	// Check if it matches configured standard tiers (in case they're customized)
	if ramMB == cfg.StandardTierMicro {
		return TierMicro
	}
	if ramMB == cfg.StandardTierSmall {
		return TierSmall
	}
	if ramMB == cfg.StandardTierMedium {
		return TierMedium
	}
	if ramMB == cfg.StandardTierLarge {
		return TierLarge
	}
	if ramMB == cfg.StandardTierXLarge {
		return TierXLarge
	}

	// Non-standard size = custom tier
	return TierCustom
}

// IsStandardTier checks if a RAM size is a standard tier
func IsStandardTier(ramMB int) bool {
	return ClassifyTier(ramMB) != TierCustom
}

// GetTierRAM returns RAM in MB for a standard tier
func GetTierRAM(tier string) (int, error) {
	if ram, ok := StandardTiers[tier]; ok {
		return ram, nil
	}
	return 0, fmt.Errorf("unknown tier: %s", tier)
}

// GetNearestStandardTier returns the nearest standard tier for a given RAM size
// Returns the tier that is >= ramMB (rounds up to prevent under-provisioning)
func GetNearestStandardTier(ramMB int) string {
	tiers := []struct {
		name string
		ram  int
	}{
		{TierMicro, 2048},
		{TierSmall, 4096},
		{TierMedium, 8192},
		{TierLarge, 16384},
		{TierXLarge, 32768},
	}

	for _, tier := range tiers {
		if ramMB <= tier.ram {
			return tier.name
		}
	}

	// If larger than XLarge, return XLarge (will be custom)
	return TierXLarge
}

// CalculateHourlyRate calculates the hourly rate based on tier, plan, and RAM
func CalculateHourlyRate(tier string, plan string, ramMB int) float64 {
	cfg := config.AppConfig
	ramGB := float64(ramMB) / 1024.0

	var rate float64
	switch plan {
	case PlanPayPerPlay:
		rate = cfg.PricingPayPerPlay
	case PlanBalanced:
		rate = cfg.PricingBalanced
	case PlanReserved:
		rate = cfg.PricingReserved
	default:
		rate = cfg.PricingPayPerPlay // Default
	}

	// Custom tier gets premium pricing
	if tier == TierCustom {
		rate = cfg.PricingCustom
	}

	return rate * ramGB
}

// CalculateMonthlyRate calculates the monthly rate (assuming 730 hours/month)
func CalculateMonthlyRate(tier string, plan string, ramMB int) float64 {
	return CalculateHourlyRate(tier, plan, ramMB) * 730.0
}

// AllowConsolidation checks if consolidation is allowed for a tier
func AllowConsolidation(tier string) bool {
	cfg := config.AppConfig

	switch tier {
	case TierMicro:
		return cfg.AllowConsolidationMicro
	case TierSmall:
		return cfg.AllowConsolidationSmall
	case TierMedium:
		return cfg.AllowConsolidationMedium
	case TierLarge:
		return cfg.AllowConsolidationLarge
	case TierXLarge:
		return cfg.AllowConsolidationXLarge
	case TierCustom:
		return cfg.AllowConsolidationCustom
	default:
		return false // Safe default
	}
}

// GetTierDisplayName returns a human-readable tier name
func GetTierDisplayName(tier string) string {
	switch tier {
	case TierMicro:
		return "Micro (2GB)"
	case TierSmall:
		return "Small (4GB)"
	case TierMedium:
		return "Medium (8GB)"
	case TierLarge:
		return "Large (16GB)"
	case TierXLarge:
		return "XLarge (32GB)"
	case TierCustom:
		return "Custom"
	default:
		return "Unknown"
	}
}

// GetPlanDisplayName returns a human-readable plan name
func GetPlanDisplayName(plan string) string {
	switch plan {
	case PlanPayPerPlay:
		return "PayPerPlay"
	case PlanBalanced:
		return "Balanced"
	case PlanReserved:
		return "Reserved"
	default:
		return "Unknown"
	}
}

// ValidatePlan checks if a plan is valid
func ValidatePlan(plan string) bool {
	return plan == PlanPayPerPlay || plan == PlanBalanced || plan == PlanReserved
}

// ValidateTier checks if a tier is valid
func ValidateTier(tier string) bool {
	_, ok := StandardTiers[tier]
	return ok || tier == TierCustom
}

// GetRecommendedPlan returns the recommended plan for a tier
func GetRecommendedPlan(tier string) string {
	switch tier {
	case TierMicro, TierSmall:
		return PlanPayPerPlay // Small servers benefit most from optimization
	case TierMedium:
		return PlanBalanced // Medium servers balance cost and stability
	case TierLarge, TierXLarge:
		return PlanReserved // Large servers should be stable
	default:
		return PlanPayPerPlay
	}
}

// GetTierPlayerRange returns the estimated player range for a tier
func GetTierPlayerRange(tier string) string {
	switch tier {
	case TierMicro:
		return "5-10 players"
	case TierSmall:
		return "10-20 players"
	case TierMedium:
		return "20-40 players"
	case TierLarge:
		return "40-80 players"
	case TierXLarge:
		return "80-150 players"
	default:
		return "Custom"
	}
}

// GetContainersPerNode calculates how many containers of a tier fit on a 16GB worker node
func GetContainersPerNode(tier string, nodeRAMMB int) int {
	if nodeRAMMB == 0 {
		nodeRAMMB = 16384 // Default to 16GB cpx41
	}

	tierRAM, err := GetTierRAM(tier)
	if err != nil {
		return 0
	}

	return nodeRAMMB / tierRAM
}

// CalculatePerfectPackingNodes calculates the minimum nodes needed for perfect packing
func CalculatePerfectPackingNodes(containersByTier map[string]int, nodeRAMMB int) int {
	if nodeRAMMB == 0 {
		nodeRAMMB = 16384 // Default to 16GB cpx41
	}

	totalNodes := 0

	for tier, count := range containersByTier {
		if count == 0 {
			continue
		}

		containersPerNode := GetContainersPerNode(tier, nodeRAMMB)
		if containersPerNode == 0 {
			// Tier doesn't fit on node (e.g., 32GB container on 16GB node)
			// Each container needs its own node
			totalNodes += count
		} else {
			// Perfect packing: divide and round up
			nodes := (count + containersPerNode - 1) / containersPerNode
			totalNodes += nodes
		}
	}

	return totalNodes
}
