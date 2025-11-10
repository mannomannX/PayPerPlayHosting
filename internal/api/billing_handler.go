package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/service"
	"github.com/payperplay/hosting/pkg/logger"
)

// BillingHandler handles billing and cost analytics endpoints
type BillingHandler struct {
	billingService *service.BillingService
}

// NewBillingHandler creates a new billing handler
func NewBillingHandler(billingService *service.BillingService) *BillingHandler {
	return &BillingHandler{
		billingService: billingService,
	}
}

// GetServerCosts returns cost summary for a specific server
// GET /api/servers/:id/costs
func (h *BillingHandler) GetServerCosts(c *gin.Context) {
	serverID := c.Param("id")

	summary, err := h.billingService.GetServerCosts(serverID)
	if err != nil {
		logger.Error("Failed to get server costs", err, map[string]interface{}{
			"server_id": serverID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get server costs",
		})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// GetOwnerCosts returns total costs for all servers owned by the authenticated user
// GET /api/billing/costs
func (h *BillingHandler) GetOwnerCosts(c *gin.Context) {
	ownerID := c.GetString("user_id")

	if ownerID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	summary, err := h.billingService.GetOwnerCosts(ownerID)
	if err != nil {
		logger.Error("Failed to get owner costs", err, map[string]interface{}{
			"owner_id": ownerID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get owner costs",
		})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// GetBillingEvents returns billing events for a server
// GET /api/servers/:id/billing/events
func (h *BillingHandler) GetBillingEvents(c *gin.Context) {
	serverID := c.Param("id")

	events, err := h.billingService.GetBillingEvents(serverID)
	if err != nil {
		logger.Error("Failed to get billing events", err, map[string]interface{}{
			"server_id": serverID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get billing events",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"server_id": serverID,
		"events":    events,
	})
}

// GetUsageSessions returns usage sessions for a server
// GET /api/servers/:id/billing/sessions
func (h *BillingHandler) GetUsageSessions(c *gin.Context) {
	serverID := c.Param("id")

	sessions, err := h.billingService.GetUsageSessions(serverID)
	if err != nil {
		logger.Error("Failed to get usage sessions", err, map[string]interface{}{
			"server_id": serverID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get usage sessions",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"server_id": serverID,
		"sessions":  sessions,
	})
}
