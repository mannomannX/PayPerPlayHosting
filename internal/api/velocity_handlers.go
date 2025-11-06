package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/service"
	"github.com/payperplay/hosting/internal/velocity"
	"github.com/payperplay/hosting/pkg/logger"
)

type VelocityHandler struct {
	velocityService *velocity.VelocityService
	mcService       *service.MinecraftService
}

func NewVelocityHandler(
	velocityService *velocity.VelocityService,
	mcService *service.MinecraftService,
) *VelocityHandler {
	return &VelocityHandler{
		velocityService: velocityService,
		mcService:       mcService,
	}
}

// WakeupServer handles POST /api/internal/servers/:id/wakeup
// This endpoint is called by the Velocity plugin when a player tries to connect to an offline server
func (h *VelocityHandler) WakeupServer(c *gin.Context) {
	serverID := c.Param("id")

	logger.Info("Server wakeup requested", map[string]interface{}{
		"server_id": serverID,
		"source":    "velocity",
	})

	// Get server from database
	server, err := h.mcService.GetServer(serverID)
	if err != nil {
		logger.Error("Server not found for wakeup", err, map[string]interface{}{
			"server_id": serverID,
		})
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Server not found",
			"success": false,
		})
		return
	}

	// Check if server is already running
	if server.Status == models.StatusRunning {
		c.JSON(http.StatusOK, velocity.WakeupStatus{
			ServerID: serverID,
			Status:   string(server.Status),
			Message:  "Server is already running",
			Port:     server.Port,
			Ready:    true,
		})
		return
	}

	// Check if server is already starting
	if server.Status == models.StatusStarting {
		c.JSON(http.StatusAccepted, velocity.WakeupStatus{
			ServerID: serverID,
			Status:   string(server.Status),
			Message:  "Server is starting",
			Port:     server.Port,
			Ready:    false,
		})
		return
	}

	// Start the server
	err = h.mcService.StartServer(serverID)
	if err != nil {
		logger.Error("Failed to start server for wakeup", err, map[string]interface{}{
			"server_id": serverID,
		})
		c.JSON(http.StatusInternalServerError, velocity.WakeupStatus{
			ServerID: serverID,
			Status:   "failed",
			Message:  "Failed to start server: " + err.Error(),
			Ready:    false,
		})
		return
	}

	logger.Info("Server wakeup initiated", map[string]interface{}{
		"server_id": serverID,
		"port":      server.Port,
	})

	c.JSON(http.StatusAccepted, velocity.WakeupStatus{
		ServerID: serverID,
		Status:   "starting",
		Message:  "Server is starting, please wait...",
		Port:     server.Port,
		Ready:    false,
	})
}

// GetServerStatus handles GET /api/internal/servers/:id/status
// This endpoint is polled by the Velocity plugin to check if the server is ready
func (h *VelocityHandler) GetServerStatus(c *gin.Context) {
	serverID := c.Param("id")

	server, err := h.mcService.GetServer(serverID)
	if err != nil {
		c.JSON(http.StatusNotFound, velocity.WakeupStatus{
			ServerID: serverID,
			Status:   "not_found",
			Message:  "Server not found",
			Ready:    false,
		})
		return
	}

	// Check if server is ready (running status)
	ready := server.Status == models.StatusRunning

	c.JSON(http.StatusOK, velocity.WakeupStatus{
		ServerID: serverID,
		Status:   string(server.Status),
		Port:     server.Port,
		Ready:    ready,
	})
}

// ReloadVelocity handles POST /api/internal/velocity/reload
// This endpoint reloads the Velocity proxy configuration
func (h *VelocityHandler) ReloadVelocity(c *gin.Context) {
	logger.Info("Velocity reload requested", nil)

	if err := h.velocityService.ReloadConfig(); err != nil {
		logger.Error("Failed to reload Velocity", err, nil)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Velocity configuration reloaded",
	})
}

// GetVelocityServers handles GET /api/internal/velocity/servers
// This endpoint returns all servers registered with Velocity
func (h *VelocityHandler) GetVelocityServers(c *gin.Context) {
	servers, err := h.mcService.ListServers("default")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Filter for Velocity-registered servers
	var velocityServers []map[string]interface{}
	for _, server := range servers {
		if server.VelocityRegistered {
			velocityServers = append(velocityServers, map[string]interface{}{
				"id":          server.ID,
				"name":        server.Name,
				"server_name": server.VelocityServerName,
				"port":        server.Port,
				"status":      server.Status,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"servers": velocityServers,
		"count":   len(velocityServers),
	})
}

// GetVelocityStatus handles GET /api/velocity/status
// Public endpoint to check Velocity proxy status
func (h *VelocityHandler) GetVelocityStatus(c *gin.Context) {
	running := h.velocityService.IsRunning()

	status := "stopped"
	if running {
		status = "running"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":       status,
		"running":      running,
		"container_id": h.velocityService.GetContainerID(),
		"port":         "25565",
		"checked_at":   time.Now().Format(time.RFC3339),
	})
}

// StartVelocity handles POST /api/velocity/start
// Public endpoint to start the Velocity proxy
func (h *VelocityHandler) StartVelocity(c *gin.Context) {
	if h.velocityService.IsRunning() {
		c.JSON(http.StatusOK, gin.H{
			"message": "Velocity proxy is already running",
			"running": true,
		})
		return
	}

	if err := h.velocityService.Start(); err != nil {
		logger.Error("Failed to start Velocity", err, nil)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   err.Error(),
			"running": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Velocity proxy started successfully",
		"running": true,
		"port":    "25565",
	})
}

// StopVelocity handles POST /api/velocity/stop
// Public endpoint to stop the Velocity proxy
func (h *VelocityHandler) StopVelocity(c *gin.Context) {
	if !h.velocityService.IsRunning() {
		c.JSON(http.StatusOK, gin.H{
			"message": "Velocity proxy is already stopped",
			"running": false,
		})
		return
	}

	if err := h.velocityService.Stop(); err != nil {
		logger.Error("Failed to stop Velocity", err, nil)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   err.Error(),
			"running": true,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Velocity proxy stopped successfully",
		"running": false,
	})
}
