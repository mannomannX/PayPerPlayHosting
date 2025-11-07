package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/service"
	"github.com/payperplay/hosting/pkg/logger"
)

// MOTDHandler handles MOTD (Message of the Day) endpoints
type MOTDHandler struct {
	motdService *service.MOTDService
}

// NewMOTDHandler creates a new MOTD handler
func NewMOTDHandler(motdService *service.MOTDService) *MOTDHandler {
	return &MOTDHandler{
		motdService: motdService,
	}
}

// GetMOTD returns the current MOTD for a server
// GET /api/servers/:id/motd
func (h *MOTDHandler) GetMOTD(c *gin.Context) {
	serverID := c.Param("id")

	motd, err := h.motdService.GetMOTD(serverID)
	if err != nil {
		logger.Error("Failed to get MOTD", err, map[string]interface{}{
			"server_id": serverID,
		})
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Server not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"motd": motd,
	})
}

// UpdateMOTD updates the MOTD for a server
// PUT /api/servers/:id/motd
// Body: { "motd": "My Server Description" }
func (h *MOTDHandler) UpdateMOTD(c *gin.Context) {
	serverID := c.Param("id")

	var req struct {
		MOTD string `json:"motd" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	if err := h.motdService.UpdateMOTD(serverID, req.MOTD); err != nil {
		logger.Error("Failed to update MOTD", err, map[string]interface{}{
			"server_id": serverID,
			"motd":      req.MOTD,
		})
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	logger.Info("MOTD updated via API", map[string]interface{}{
		"server_id": serverID,
		"motd":      req.MOTD,
	})

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "MOTD updated successfully",
		"motd":    req.MOTD,
	})
}
