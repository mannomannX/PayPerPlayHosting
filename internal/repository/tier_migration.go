package repository

import (
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/pkg/logger"
)

// MigrateTierFields populates tier and plan fields for existing servers
// This should be called once after deploying the tier system
func MigrateTierFields() error {
	db := GetDB()
	if db == nil {
		return nil
	}

	logger.Info("Starting tier migration for existing servers", nil)

	// Get all servers
	var servers []models.MinecraftServer
	if err := db.Find(&servers).Error; err != nil {
		logger.Error("Failed to fetch servers for tier migration", err, nil)
		return err
	}

	logger.Info("Found servers to migrate", map[string]interface{}{
		"count": len(servers),
	})

	migratedCount := 0
	for i := range servers {
		server := &servers[i]

		// Skip if already has tier assigned (not empty and not default)
		if server.RAMTier != "" && server.RAMTier != "small" {
			continue
		}

		// Calculate tier based on RAM
		server.CalculateTier()

		// Set default plan if not set
		if server.Plan == "" {
			server.Plan = models.GetRecommendedPlan(server.RAMTier)
		}

		// Validate plan
		if !models.ValidatePlan(server.Plan) {
			server.Plan = models.PlanPayPerPlay // Safe default
		}

		// Update server
		if err := db.Model(server).Updates(map[string]interface{}{
			"ram_tier":       server.RAMTier,
			"plan":           server.Plan,
			"is_custom_tier": server.IsCustomTier,
		}).Error; err != nil {
			logger.Error("Failed to update server tier", err, map[string]interface{}{
				"server_id":   server.ID,
				"server_name": server.Name,
			})
			continue
		}

		migratedCount++
		logger.Info("Server tier migrated", map[string]interface{}{
			"server_id":      server.ID,
			"server_name":    server.Name,
			"ram_mb":         server.RAMMb,
			"tier":           server.RAMTier,
			"plan":           server.Plan,
			"is_custom_tier": server.IsCustomTier,
		})
	}

	logger.Info("Tier migration completed", map[string]interface{}{
		"total_servers":    len(servers),
		"migrated_servers": migratedCount,
		"skipped_servers":  len(servers) - migratedCount,
	})

	return nil
}

// GetTierStatistics returns statistics about server tiers
func GetTierStatistics() (map[string]interface{}, error) {
	db := GetDB()
	if db == nil {
		return nil, nil
	}

	stats := make(map[string]interface{})

	// Count servers by tier
	var tierCounts []struct {
		RAMTier string
		Count   int64
	}

	if err := db.Model(&models.MinecraftServer{}).
		Select("ram_tier, count(*) as count").
		Group("ram_tier").
		Scan(&tierCounts).Error; err != nil {
		return nil, err
	}

	tierMap := make(map[string]int64)
	for _, tc := range tierCounts {
		tierMap[tc.RAMTier] = tc.Count
	}
	stats["by_tier"] = tierMap

	// Count servers by plan
	var planCounts []struct {
		Plan  string
		Count int64
	}

	if err := db.Model(&models.MinecraftServer{}).
		Select("plan, count(*) as count").
		Group("plan").
		Scan(&planCounts).Error; err != nil {
		return nil, err
	}

	planMap := make(map[string]int64)
	for _, pc := range planCounts {
		planMap[pc.Plan] = pc.Count
	}
	stats["by_plan"] = planMap

	// Count custom tier servers
	var customTierCount int64
	if err := db.Model(&models.MinecraftServer{}).
		Where("is_custom_tier = ?", true).
		Count(&customTierCount).Error; err != nil {
		return nil, err
	}
	stats["custom_tier_count"] = customTierCount

	// Calculate total RAM by tier
	var ramByTier []struct {
		RAMTier string
		TotalRAM int64
	}

	if err := db.Model(&models.MinecraftServer{}).
		Select("ram_tier, sum(ram_mb) as total_ram").
		Group("ram_tier").
		Scan(&ramByTier).Error; err != nil {
		return nil, err
	}

	ramMap := make(map[string]int64)
	for _, r := range ramByTier {
		ramMap[r.RAMTier] = r.TotalRAM
	}
	stats["total_ram_by_tier"] = ramMap

	return stats, nil
}
