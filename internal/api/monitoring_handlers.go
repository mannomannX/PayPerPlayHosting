package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/service"
)

type MonitoringHandler struct {
	monitoringService *service.MonitoringService
	mcService         *service.MinecraftService
}

func NewMonitoringHandler(monitoringService *service.MonitoringService) *MonitoringHandler {
	return &MonitoringHandler{
		monitoringService: monitoringService,
	}
}

// SetMinecraftService sets the minecraft service for database operations
func (h *MonitoringHandler) SetMinecraftService(mcService *service.MinecraftService) {
	h.mcService = mcService
}

// GetServerStatus handles GET /api/servers/:id/status
func (h *MonitoringHandler) GetServerStatus(c *gin.Context) {
	serverID := c.Param("id")

	status := h.monitoringService.GetServerStatus(serverID)

	c.JSON(http.StatusOK, status)
}

// GetAllStatuses handles GET /api/monitoring/status
func (h *MonitoringHandler) GetAllStatuses(c *gin.Context) {
	statuses := h.monitoringService.GetAllStatuses()

	c.JSON(http.StatusOK, statuses)
}

// EnableAutoShutdown handles POST /api/servers/:id/auto-shutdown/enable
func (h *MonitoringHandler) EnableAutoShutdown(c *gin.Context) {
	serverID := c.Param("id")

	// Enable auto-shutdown via monitoring service
	if err := h.monitoringService.EnableAutoShutdown(serverID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "auto-shutdown enabled"})
}

// DisableAutoShutdown handles POST /api/servers/:id/auto-shutdown/disable
func (h *MonitoringHandler) DisableAutoShutdown(c *gin.Context) {
	serverID := c.Param("id")

	// Disable auto-shutdown via monitoring service
	if err := h.monitoringService.DisableAutoShutdown(serverID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "auto-shutdown disabled"})
}
