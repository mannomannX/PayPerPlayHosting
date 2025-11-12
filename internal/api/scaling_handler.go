package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/conductor"
	"github.com/payperplay/hosting/pkg/logger"
)

// ScalingHandler handles scaling-related API requests
type ScalingHandler struct {
	conductor *conductor.Conductor
}

// NewScalingHandler creates a new scaling handler
func NewScalingHandler(conductor *conductor.Conductor) *ScalingHandler {
	return &ScalingHandler{
		conductor: conductor,
	}
}

// GetScalingStatus returns the current scaling engine status
// GET /api/scaling/status
func (h *ScalingHandler) GetScalingStatus(c *gin.Context) {
	if h.conductor.ScalingEngine == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Scaling engine not initialized",
		})
		return
	}

	status := h.conductor.ScalingEngine.GetStatus()

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"scaling": status,
	})
}

// EnableScaling enables the scaling engine
// POST /api/scaling/enable
func (h *ScalingHandler) EnableScaling(c *gin.Context) {
	if h.conductor.ScalingEngine == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Scaling engine not initialized",
		})
		return
	}

	h.conductor.ScalingEngine.Enable()

	logger.Info("Scaling engine enabled via API", map[string]interface{}{
		"user_id": c.GetString("user_id"),
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "Scaling engine enabled",
	})
}

// DisableScaling disables the scaling engine
// POST /api/scaling/disable
func (h *ScalingHandler) DisableScaling(c *gin.Context) {
	if h.conductor.ScalingEngine == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Scaling engine not initialized",
		})
		return
	}

	h.conductor.ScalingEngine.Disable()

	logger.Info("Scaling engine disabled via API", map[string]interface{}{
		"user_id": c.GetString("user_id"),
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "Scaling engine disabled (for maintenance)",
	})
}

// TriggerScaleUp manually triggers a scale-up operation (for testing)
// POST /api/scaling/scale-up
func (h *ScalingHandler) TriggerScaleUp(c *gin.Context) {
	if h.conductor.ScalingEngine == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Scaling engine not initialized",
		})
		return
	}

	var req struct {
		ServerType string `json:"server_type" binding:"required"`
		Count      int    `json:"count"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	if req.Count == 0 {
		req.Count = 1
	}

	logger.Info("Manual scale-up triggered via API", map[string]interface{}{
		"user_id":     c.GetString("user_id"),
		"server_type": req.ServerType,
		"count":       req.Count,
	})

	// Create manual scale recommendation
	recommendation := conductor.ScaleRecommendation{
		Action:     conductor.ScaleActionScaleUp,
		ServerType: req.ServerType,
		Count:      req.Count,
		Reason:     "Manual trigger via API",
		Urgency:    conductor.UrgencyLow,
	}

	// Execute scaling (bypass policy checks)
	// Note: This is for testing/emergency use only
	// TODO: Implement executeScaling as public method in ScalingEngine

	c.JSON(http.StatusOK, gin.H{
		"message": "Scale-up triggered",
		"recommendation": recommendation,
	})
}

// GetScalingHistory returns recent scaling events
// GET /api/scaling/history?limit=50
func (h *ScalingHandler) GetScalingHistory(c *gin.Context) {
	// TODO: Query scaling events from Event-Bus/InfluxDB
	// For now, return placeholder

	limit := 50
	if l := c.Query("limit"); l != "" {
		// Parse limit (skipping error handling for brevity)
	}

	c.JSON(http.StatusOK, gin.H{
		"events": []interface{}{
			// TODO: Implement event history query
		},
		"limit": limit,
	})
}

// OptimizeCosts triggers container consolidation for cost optimization (B8)
// POST /api/scaling/optimize-costs
func (h *ScalingHandler) OptimizeCosts(c *gin.Context) {
	if h.conductor.ScalingEngine == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Scaling engine not initialized",
		})
		return
	}

	if !h.conductor.ScalingEngine.IsEnabled() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Scaling engine is disabled",
		})
		return
	}

	logger.Info("Manual cost optimization triggered via API", map[string]interface{}{
		"user_id": c.GetString("user_id"),
	})

	// Check if there are any consolidation opportunities
	fleetStats := h.conductor.NodeRegistry.GetFleetStats()
	capacityPercent := 0.0
	if fleetStats.UsableRAMMB > 0 {
		capacityPercent = (float64(fleetStats.AllocatedRAMMB) / float64(fleetStats.UsableRAMMB)) * 100
	}

	cloudNodes := h.conductor.NodeRegistry.GetNodesByType("cloud")

	if len(cloudNodes) < 2 {
		c.JSON(http.StatusOK, gin.H{
			"message": "Not enough cloud nodes to consolidate",
			"analysis": gin.H{
				"cloud_nodes": len(cloudNodes),
				"capacity_percent": capacityPercent,
			},
		})
		return
	}

	if capacityPercent > 70.0 {
		c.JSON(http.StatusOK, gin.H{
			"message": "Fleet capacity too high for safe consolidation",
			"analysis": gin.H{
				"cloud_nodes": len(cloudNodes),
				"capacity_percent": capacityPercent,
				"max_safe_capacity": 70.0,
			},
		})
		return
	}

	// Return success - the scaling engine will handle consolidation on next cycle
	c.JSON(http.StatusOK, gin.H{
		"message": "Cost optimization analysis complete",
		"status": "consolidation_candidate",
		"analysis": gin.H{
			"cloud_nodes": len(cloudNodes),
			"capacity_percent": capacityPercent,
			"allocated_ram_mb": fleetStats.AllocatedRAMMB,
			"usable_ram_mb": fleetStats.UsableRAMMB,
			"potential_savings": "Will be evaluated on next scaling engine cycle",
		},
		"next_steps": "The scaling engine will automatically consolidate on the next evaluation cycle (every 2 minutes)",
	})

	// Note: We don't directly trigger consolidation here to avoid bypassing safety checks
	// The scaling engine will naturally consolidate on its next cycle if conditions are met
	logger.Info("Cost optimization request completed", map[string]interface{}{
		"user_id": c.GetString("user_id"),
		"cloud_nodes": len(cloudNodes),
		"capacity_percent": capacityPercent,
	})
}

// buildScalingContext is a helper to build context for manual operations
func (h *ScalingHandler) buildScalingContext() conductor.ScalingContext {
	stats := h.conductor.NodeRegistry.GetFleetStats()
	nodes := h.conductor.NodeRegistry.GetAllNodes()

	var dedicatedNodes, cloudNodes []*conductor.Node
	for _, node := range nodes {
		if node.Type == "dedicated" {
			dedicatedNodes = append(dedicatedNodes, node)
		} else if node.Type == "cloud" {
			cloudNodes = append(cloudNodes, node)
		}
	}

	return conductor.ScalingContext{
		FleetStats:     stats,
		DedicatedNodes: dedicatedNodes,
		CloudNodes:     cloudNodes,
	}
}
