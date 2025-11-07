package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/service"
	"github.com/payperplay/hosting/pkg/logger"
)

// PlayerHandler handles player management endpoints
type PlayerHandler struct {
	playerListService *service.PlayerListService
}

// NewPlayerHandler creates a new player handler
func NewPlayerHandler(playerListService *service.PlayerListService) *PlayerHandler {
	return &PlayerHandler{
		playerListService: playerListService,
	}
}

// GetPlayerList returns a specific player list (whitelist, ops, or banned)
// GET /api/servers/:id/players/:listType
func (h *PlayerHandler) GetPlayerList(c *gin.Context) {
	serverID := c.Param("id")
	listTypeStr := c.Param("listType")

	// Convert string to PlayerListType
	listType := service.PlayerListType(listTypeStr)

	// Validate list type
	validTypes := []service.PlayerListType{
		service.ListTypeWhitelist,
		service.ListTypeOps,
		service.ListTypeBanned,
	}
	valid := false
	for _, t := range validTypes {
		if listType == t {
			valid = true
			break
		}
	}
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid list type. Must be: whitelist, ops, or banned-players",
		})
		return
	}

	// Get list
	list, err := h.playerListService.GetList(serverID, listType)
	if err != nil {
		logger.Error("Failed to get player list", err, map[string]interface{}{
			"server_id": serverID,
			"list_type": listType,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get player list",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"list_type": listType,
		"players":   list,
	})
}

// AddToPlayerList adds a player to a specific list
// POST /api/servers/:id/players/:listType/add
// Body: { "username": "PlayerName" }
func (h *PlayerHandler) AddToPlayerList(c *gin.Context) {
	serverID := c.Param("id")
	listTypeStr := c.Param("listType")
	listType := service.PlayerListType(listTypeStr)

	var req struct {
		Username string `json:"username" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body. 'username' is required",
		})
		return
	}

	// Add player to list
	err := h.playerListService.AddToList(serverID, req.Username, listType)
	if err != nil {
		logger.Error("Failed to add player to list", err, map[string]interface{}{
			"server_id": serverID,
			"username":  req.Username,
			"list_type": listType,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	logger.Info("Player added to list", map[string]interface{}{
		"server_id": serverID,
		"username":  req.Username,
		"list_type": listType,
	})

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"message":   "Player added successfully",
		"username":  req.Username,
		"list_type": listType,
	})
}

// RemoveFromPlayerList removes a player from a specific list
// DELETE /api/servers/:id/players/:listType/:username
func (h *PlayerHandler) RemoveFromPlayerList(c *gin.Context) {
	serverID := c.Param("id")
	listTypeStr := c.Param("listType")
	username := c.Param("username")
	listType := service.PlayerListType(listTypeStr)

	// Remove player from list
	err := h.playerListService.RemoveFromList(serverID, username, listType)
	if err != nil {
		logger.Error("Failed to remove player from list", err, map[string]interface{}{
			"server_id": serverID,
			"username":  username,
			"list_type": listType,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	logger.Info("Player removed from list", map[string]interface{}{
		"server_id": serverID,
		"username":  username,
		"list_type": listType,
	})

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"message":   "Player removed successfully",
		"username":  username,
		"list_type": listType,
	})
}

// GetOnlinePlayers returns currently online players
// GET /api/servers/:id/players/online
func (h *PlayerHandler) GetOnlinePlayers(c *gin.Context) {
	serverID := c.Param("id")

	players, err := h.playerListService.GetOnlinePlayers(serverID)
	if err != nil {
		logger.Error("Failed to get online players", err, map[string]interface{}{
			"server_id": serverID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get online players",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"players": players,
		"count":   len(players),
	})
}

// GetHistoricPlayers returns all players who ever joined the server
// GET /api/servers/:id/players/history
func (h *PlayerHandler) GetHistoricPlayers(c *gin.Context) {
	serverID := c.Param("id")

	players, err := h.playerListService.GetHistoricPlayers(serverID)
	if err != nil {
		logger.Error("Failed to get historic players", err, map[string]interface{}{
			"server_id": serverID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get historic players",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"players": players,
		"count":   len(players),
	})
}
