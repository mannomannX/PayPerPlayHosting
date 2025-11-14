package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/service"
)

// CostOptimizationHandler handles cost optimization API requests
type CostOptimizationHandler struct {
	costOptService *service.CostOptimizationService
}

// NewCostOptimizationHandler creates a new cost optimization handler
func NewCostOptimizationHandler(costOptService *service.CostOptimizationService) *CostOptimizationHandler {
	return &CostOptimizationHandler{
		costOptService: costOptService,
	}
}

// GetSuggestions returns all current cost optimization suggestions
// GET /api/cost-optimization/suggestions
func (h *CostOptimizationHandler) GetSuggestions(c *gin.Context) {
	suggestions := h.costOptService.GetCurrentSuggestions()

	c.JSON(http.StatusOK, gin.H{
		"status":      "ok",
		"suggestions": suggestions,
		"count":       len(suggestions),
	})
}

// GetStatus returns the status of the cost optimization service
// GET /api/cost-optimization/status
func (h *CostOptimizationHandler) GetStatus(c *gin.Context) {
	status := h.costOptService.GetStatus()

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"data":   status,
	})
}

// TriggerAnalysis manually triggers a cost optimization analysis
// POST /api/cost-optimization/analyze
func (h *CostOptimizationHandler) TriggerAnalysis(c *gin.Context) {
	// Trigger analysis in background
	go h.costOptService.TriggerImmediateAnalysis()

	c.JSON(http.StatusAccepted, gin.H{
		"status":  "ok",
		"message": "Cost optimization analysis triggered",
	})
}
