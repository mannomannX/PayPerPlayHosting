package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/middleware"
	"github.com/payperplay/hosting/internal/service"
	"github.com/payperplay/hosting/pkg/logger"
)

type ConfigHandler struct {
	configService *service.ConfigService
	serverService *service.MinecraftService
}

func NewConfigHandler(configService *service.ConfigService, serverService *service.MinecraftService) *ConfigHandler {
	return &ConfigHandler{
		configService: configService,
		serverService: serverService,
	}
}

// ApplyConfigChangeRequest is the request body for config changes
type ApplyConfigChangeRequest struct {
	Changes map[string]interface{} `json:"changes" binding:"required"`
}

// ApplyConfigChanges handles POST /api/servers/:id/config
func (h *ConfigHandler) ApplyConfigChanges(c *gin.Context) {
	serverID := c.Param("id")
	userID := middleware.GetUserID(c)

	var req ApplyConfigChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"code":  "INVALID_REQUEST",
		})
		return
	}

	// Validate that changes map is not empty
	if len(req.Changes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No configuration changes provided",
			"code":  "EMPTY_CHANGES",
		})
		return
	}

	// Verify server exists and user has access
	server, err := h.serverService.GetServer(serverID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Server not found",
			"code":  "SERVER_NOT_FOUND",
		})
		return
	}

	// Check ownership
	if server.OwnerID != userID {
		isAdmin, _ := c.Get("is_admin")
		if !isAdmin.(bool) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "You don't have permission to modify this server",
				"code":  "FORBIDDEN",
			})
			return
		}
	}

	// Apply configuration changes
	changeRequest := service.ConfigChangeRequest{
		ServerID: serverID,
		UserID:   userID,
		Changes:  req.Changes,
	}

	configChange, err := h.configService.ApplyConfigChanges(changeRequest)
	if err != nil {
		logger.Error("Failed to apply config changes", err, map[string]interface{}{
			"server_id": serverID,
			"user_id":   userID,
			"changes":   req.Changes,
		})

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to apply configuration changes",
			"code":    "CONFIG_CHANGE_FAILED",
			"details": err.Error(),
		})
		return
	}

	logger.Info("Configuration changes applied successfully", map[string]interface{}{
		"server_id":  serverID,
		"user_id":    userID,
		"change_id":  configChange.ID,
		"change_type": configChange.ChangeType,
	})

	c.JSON(http.StatusOK, gin.H{
		"message":          "Configuration changes applied successfully",
		"change_id":        configChange.ID,
		"change_type":      configChange.ChangeType,
		"status":           configChange.Status,
		"requires_restart": configChange.RequiresRestart,
		"old_value":        configChange.OldValue,
		"new_value":        configChange.NewValue,
	})
}

// GetConfigHistory handles GET /api/servers/:id/config/history
func (h *ConfigHandler) GetConfigHistory(c *gin.Context) {
	serverID := c.Param("id")
	userID := middleware.GetUserID(c)

	// Verify server exists and user has access
	server, err := h.serverService.GetServer(serverID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Server not found",
			"code":  "SERVER_NOT_FOUND",
		})
		return
	}

	// Check ownership
	if server.OwnerID != userID {
		isAdmin, _ := c.Get("is_admin")
		if !isAdmin.(bool) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "You don't have permission to view this server's configuration history",
				"code":  "FORBIDDEN",
			})
			return
		}
	}

	// Get configuration history
	history, err := h.configService.GetConfigHistory(serverID)
	if err != nil {
		logger.Error("Failed to get config history", err, map[string]interface{}{
			"server_id": serverID,
		})

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve configuration history",
			"code":  "HISTORY_FETCH_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"server_id": serverID,
		"history":   history,
		"count":     len(history),
	})
}
