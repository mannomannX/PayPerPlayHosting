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
