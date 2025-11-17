package api

import (
	"log"
	"net/http"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/service"
)

type Handler struct {
	mcService *service.MinecraftService
}

func NewHandler(mcService *service.MinecraftService) *Handler {
	return &Handler{mcService: mcService}
}

// CreateServerRequest represents the request body for creating a server
type CreateServerRequest struct {
	Name             string `json:"name" binding:"required"`
	ServerType       string `json:"server_type" binding:"required"`
	MinecraftVersion string `json:"minecraft_version" binding:"required"`
	RAMMb            int    `json:"ram_mb" binding:"required,min=1024"`
}

// CreateServer handles POST /api/servers
func (h *Handler) CreateServer(c *gin.Context) {
	var req CreateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// FIX SERVER-5: Validate server name (alphanumeric, dash, underscore only)
	serverNameRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)
	if !serverNameRegex.MatchString(req.Name) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Server name must be 3-32 characters and contain only letters, numbers, dashes, and underscores",
		})
		return
	}

	// Validate server type
	serverType := models.ServerType(req.ServerType)
	validTypes := []models.ServerType{
		models.ServerTypePaper,
		models.ServerTypeSpigot,
		models.ServerTypeForge,
		models.ServerTypeFabric,
		models.ServerTypeVanilla,
		models.ServerTypePurpur,
	}

	valid := false
	for _, t := range validTypes {
		if serverType == t {
			valid = true
			break
		}
	}
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid server type"})
		return
	}

	// Get owner ID from auth context
	ownerID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	server, err := h.mcService.CreateServer(
		req.Name,
		serverType,
		req.MinecraftVersion,
		req.RAMMb,
		ownerID.(string),
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// FIX BILLING-5: Show cost estimate to user
	c.JSON(http.StatusCreated, gin.H{
		"server":                server,
		"estimated_hourly_cost": server.GetHourlyRate(),
		"estimated_monthly_cost": server.GetMonthlyRate(),
		"billing_plan":          server.Plan,
		"tier":                  server.RAMTier,
	})
}

// ListServers handles GET /api/servers
func (h *Handler) ListServers(c *gin.Context) {
	// Get owner ID from auth context
	ownerID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	servers, err := h.mcService.ListServers(ownerID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, servers)
}

// GetServer handles GET /api/servers/:id
func (h *Handler) GetServer(c *gin.Context) {
	serverID := c.Param("id")

	server, err := h.mcService.GetServer(serverID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	c.JSON(http.StatusOK, server)
}

// GetServerConnectionInfo handles GET /api/servers/:id/connection
// Returns IP address and port for connecting to a running Minecraft server
func (h *Handler) GetServerConnectionInfo(c *gin.Context) {
	serverID := c.Param("id")

	info, err := h.mcService.GetServerConnectionInfo(serverID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, info)
}

// StartServer handles POST /api/servers/:id/start
func (h *Handler) StartServer(c *gin.Context) {
	serverID := c.Param("id")

	err := h.mcService.StartServer(serverID)
	if err != nil {
		log.Printf("ERROR starting server %s: %v", serverID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "server starting"})
}

// StopServer handles POST /api/servers/:id/stop
func (h *Handler) StopServer(c *gin.Context) {
	serverID := c.Param("id")

	err := h.mcService.StopServer(serverID, "manual")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "server stopped"})
}

// DeleteServer handles DELETE /api/servers/:id
func (h *Handler) DeleteServer(c *gin.Context) {
	serverID := c.Param("id")

	err := h.mcService.DeleteServer(serverID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "server deleted"})
}

// GetServerUsage handles GET /api/servers/:id/usage
func (h *Handler) GetServerUsage(c *gin.Context) {
	serverID := c.Param("id")

	usage, err := h.mcService.GetServerUsage(serverID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, usage)
}

// GetServerLogs handles GET /api/servers/:id/logs
func (h *Handler) GetServerLogs(c *gin.Context) {
	serverID := c.Param("id")
	tailStr := c.DefaultQuery("tail", "100")

	tail, err := strconv.Atoi(tailStr)
	if err != nil {
		tail = 100
	}

	logs, err := h.mcService.GetServerLogs(serverID, tail)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

// ListAllServers handles GET /api/admin/servers (shows ALL servers, not filtered by owner)
func (h *Handler) ListAllServers(c *gin.Context) {
	servers, err := h.mcService.ListAllServers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, servers)
}

// CleanOrphanedServers handles POST /api/admin/cleanup
func (h *Handler) CleanOrphanedServers(c *gin.Context) {
	count, err := h.mcService.CleanOrphanedServers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "cleaned orphaned servers",
		"count":   count,
	})
}

// ListArchivedServers handles GET /api/servers/archived
func (h *Handler) ListArchivedServers(c *gin.Context) {
	// Get owner ID from auth context (optional - admin can see all)
	ownerID := ""
	if userID, exists := c.Get("user_id"); exists {
		ownerID = userID.(string)
	}

	servers, err := h.mcService.ListArchivedServers(ownerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch archived servers"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"servers": servers,
		"count":   len(servers),
	})
}

// HealthCheck handles GET /health
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"service": "payperplay-hosting",
	})
}
