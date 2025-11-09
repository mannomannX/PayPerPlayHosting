package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/internal/service"
	"github.com/payperplay/hosting/pkg/logger"
)

// WebhookHandler handles webhook-related endpoints
type WebhookHandler struct {
	webhookService *service.WebhookService
	serverRepo     *repository.ServerRepository
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(webhookService *service.WebhookService, serverRepo *repository.ServerRepository) *WebhookHandler {
	return &WebhookHandler{
		webhookService: webhookService,
		serverRepo:     serverRepo,
	}
}

// GetWebhook returns the webhook configuration for a server
// GET /api/servers/:id/webhook
func (h *WebhookHandler) GetWebhook(c *gin.Context) {
	serverID := c.Param("id")
	id, err := strconv.ParseUint(serverID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	// Verify server exists and user has access
	server, err := h.serverRepo.FindByID(uint(id))
	if err != nil || server == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}

	webhook, err := h.webhookService.GetWebhook(uint(id))
	if err != nil {
		logger.Error("Failed to get webhook", err, map[string]interface{}{
			"server_id": id,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get webhook"})
		return
	}

	if webhook == nil {
		c.JSON(http.StatusOK, gin.H{
			"configured": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"configured": true,
		"webhook":    webhook,
	})
}

// CreateWebhook creates a new webhook configuration
// POST /api/servers/:id/webhook
// Body: {"webhook_url": "https://discord.com/api/webhooks/..."}
func (h *WebhookHandler) CreateWebhook(c *gin.Context) {
	serverID := c.Param("id")
	id, err := strconv.ParseUint(serverID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	// Verify server exists and user has access
	server, err := h.serverRepo.FindByID(uint(id))
	if err != nil || server == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}

	var request struct {
		WebhookURL string `json:"webhook_url" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "webhook_url is required"})
		return
	}

	// Basic URL validation
	if len(request.WebhookURL) < 50 || request.WebhookURL[:8] != "https://" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook URL"})
		return
	}

	webhook, err := h.webhookService.CreateWebhook(uint(id), request.WebhookURL)
	if err != nil {
		logger.Error("Failed to create webhook", err, map[string]interface{}{
			"server_id": id,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	logger.Info("Webhook created", map[string]interface{}{
		"server_id": id,
	})

	c.JSON(http.StatusCreated, gin.H{
		"status":  "success",
		"message": "Webhook created successfully",
		"webhook": webhook,
	})
}

// UpdateWebhook updates webhook configuration
// PUT /api/servers/:id/webhook
// Body: {"enabled": true, "on_server_start": true, ...}
func (h *WebhookHandler) UpdateWebhook(c *gin.Context) {
	serverID := c.Param("id")
	id, err := strconv.ParseUint(serverID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	// Verify server exists and user has access
	server, err := h.serverRepo.FindByID(uint(id))
	if err != nil || server == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Only allow specific fields to be updated
	allowedFields := map[string]bool{
		"enabled":           true,
		"webhook_url":       true,
		"on_server_start":   true,
		"on_server_stop":    true,
		"on_server_crash":   true,
		"on_player_join":    true,
		"on_player_leave":   true,
		"on_backup_created": true,
	}

	filteredUpdates := make(map[string]interface{})
	for key, value := range updates {
		if allowedFields[key] {
			filteredUpdates[key] = value
		}
	}

	webhook, err := h.webhookService.UpdateWebhook(uint(id), filteredUpdates)
	if err != nil {
		logger.Error("Failed to update webhook", err, map[string]interface{}{
			"server_id": id,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	logger.Info("Webhook updated", map[string]interface{}{
		"server_id": id,
	})

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Webhook updated successfully",
		"webhook": webhook,
	})
}

// DeleteWebhook deletes a webhook configuration
// DELETE /api/servers/:id/webhook
func (h *WebhookHandler) DeleteWebhook(c *gin.Context) {
	serverID := c.Param("id")
	id, err := strconv.ParseUint(serverID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	// Verify server exists and user has access
	server, err := h.serverRepo.FindByID(uint(id))
	if err != nil || server == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}

	if err := h.webhookService.DeleteWebhook(uint(id)); err != nil {
		logger.Error("Failed to delete webhook", err, map[string]interface{}{
			"server_id": id,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete webhook"})
		return
	}

	logger.Info("Webhook deleted", map[string]interface{}{
		"server_id": id,
	})

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Webhook deleted successfully",
	})
}

// TestWebhook sends a test message to the webhook
// POST /api/servers/:id/webhook/test
func (h *WebhookHandler) TestWebhook(c *gin.Context) {
	serverID := c.Param("id")
	id, err := strconv.ParseUint(serverID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	// Verify server exists and user has access
	server, err := h.serverRepo.FindByID(uint(id))
	if err != nil || server == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}

	webhook, err := h.webhookService.GetWebhook(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get webhook"})
		return
	}

	if webhook == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No webhook configured for this server"})
		return
	}

	if err := h.webhookService.TestWebhook(webhook.WebhookURL, server.Name); err != nil {
		logger.Error("Failed to send test webhook", err, map[string]interface{}{
			"server_id": id,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send test webhook: " + err.Error()})
		return
	}

	logger.Info("Test webhook sent", map[string]interface{}{
		"server_id": id,
	})

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Test webhook sent successfully! Check your Discord channel.",
	})
}
