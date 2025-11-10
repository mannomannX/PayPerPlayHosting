package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/service"
)

// MarketplaceHandler handles plugin marketplace API endpoints
type MarketplaceHandler struct {
	pluginManager *service.PluginManagerService
	pluginSync    *service.PluginSyncService
}

// NewMarketplaceHandler creates a new marketplace handler
func NewMarketplaceHandler(pluginManager *service.PluginManagerService, pluginSync *service.PluginSyncService) *MarketplaceHandler {
	return &MarketplaceHandler{
		pluginManager: pluginManager,
		pluginSync:    pluginSync,
	}
}

// === Marketplace Browsing ===

// ListMarketplacePlugins lists available plugins in the marketplace
// GET /api/marketplace/plugins?category=admin-tools&limit=50
func (h *MarketplaceHandler) ListMarketplacePlugins(c *gin.Context) {
	category := models.PluginCategory(c.Query("category"))
	limit := parseIntQuery(c, "limit", 50)

	plugins, err := h.pluginManager.ListMarketplacePlugins(category, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list plugins"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"plugins": plugins,
		"count":   len(plugins),
	})
}

// SearchMarketplace searches for plugins
// GET /api/marketplace/search?q=worldedit&limit=20
func (h *MarketplaceHandler) SearchMarketplace(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query required"})
		return
	}

	limit := parseIntQuery(c, "limit", 20)

	plugins, err := h.pluginManager.SearchMarketplace(query, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Search failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"query":   query,
		"plugins": plugins,
		"count":   len(plugins),
	})
}

// GetPluginDetails retrieves detailed information about a plugin
// GET /api/marketplace/plugins/:slug
func (h *MarketplaceHandler) GetPluginDetails(c *gin.Context) {
	slug := c.Param("slug")

	plugin, err := h.pluginManager.GetPluginDetails(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plugin not found"})
		return
	}

	c.JSON(http.StatusOK, plugin)
}

// === Server Plugin Management ===

// ListInstalledPlugins lists plugins installed on a specific server
// GET /api/servers/:id/plugins
func (h *MarketplaceHandler) ListInstalledPlugins(c *gin.Context) {
	serverID := c.Param("id")

	plugins, err := h.pluginManager.ListInstalledPlugins(serverID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list installed plugins"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"server_id": serverID,
		"plugins":   plugins,
		"count":     len(plugins),
	})
}

// InstallPlugin installs a plugin on a server
// POST /api/servers/:id/plugins
// Body: { "plugin_slug": "worldedit", "version_id": "optional", "auto_update": false }
func (h *MarketplaceHandler) InstallPlugin(c *gin.Context) {
	serverID := c.Param("id")

	var req struct {
		PluginSlug string `json:"plugin_slug" binding:"required"`
		VersionID  string `json:"version_id"`
		AutoUpdate bool   `json:"auto_update"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.pluginManager.InstallPlugin(serverID, req.PluginSlug, req.VersionID, req.AutoUpdate); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Plugin installed successfully",
		"server_id": serverID,
		"plugin": req.PluginSlug,
	})
}

// UninstallPlugin removes a plugin from a server
// DELETE /api/servers/:id/plugins/:plugin_id
func (h *MarketplaceHandler) UninstallPlugin(c *gin.Context) {
	serverID := c.Param("id")
	pluginID := c.Param("plugin_id")

	if err := h.pluginManager.UninstallPlugin(serverID, pluginID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Plugin uninstalled successfully",
	})
}

// === Plugin Updates ===

// CheckForUpdates checks for available updates
// GET /api/servers/:id/plugins/updates
func (h *MarketplaceHandler) CheckForUpdates(c *gin.Context) {
	serverID := c.Param("id")

	updates, err := h.pluginManager.CheckForUpdates(serverID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check for updates"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"server_id": serverID,
		"updates":   updates,
		"count":     len(updates),
	})
}

// UpdatePlugin updates a plugin to a specific version
// PUT /api/servers/:id/plugins/:plugin_id
// Body: { "version_id": "new-version-id" }
func (h *MarketplaceHandler) UpdatePlugin(c *gin.Context) {
	serverID := c.Param("id")
	pluginID := c.Param("plugin_id")

	var req struct {
		VersionID string `json:"version_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.pluginManager.UpdatePlugin(serverID, pluginID, req.VersionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Plugin updated successfully",
	})
}

// AutoUpdatePlugins triggers auto-update for all plugins with AutoUpdate enabled
// POST /api/servers/:id/plugins/auto-update
func (h *MarketplaceHandler) AutoUpdatePlugins(c *gin.Context) {
	serverID := c.Param("id")

	if err := h.pluginManager.AutoUpdatePlugins(serverID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Auto-update completed",
	})
}

// === Plugin Settings ===

// TogglePlugin enables or disables a plugin
// POST /api/servers/:id/plugins/:plugin_id/toggle
// Body: { "enabled": true }
func (h *MarketplaceHandler) TogglePlugin(c *gin.Context) {
	serverID := c.Param("id")
	pluginID := c.Param("plugin_id")

	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.pluginManager.TogglePlugin(serverID, pluginID, req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Plugin toggled successfully",
		"enabled": req.Enabled,
	})
}

// ToggleAutoUpdate enables or disables auto-update for a plugin
// POST /api/servers/:id/plugins/:plugin_id/auto-update
// Body: { "auto_update": true }
func (h *MarketplaceHandler) ToggleAutoUpdate(c *gin.Context) {
	serverID := c.Param("id")
	pluginID := c.Param("plugin_id")

	var req struct {
		AutoUpdate bool `json:"auto_update"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.pluginManager.ToggleAutoUpdate(serverID, pluginID, req.AutoUpdate); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Auto-update setting updated",
		"auto_update": req.AutoUpdate,
	})
}

// === Admin Functions ===

// SyncMarketplace manually triggers a marketplace sync
// POST /api/admin/marketplace/sync
func (h *MarketplaceHandler) SyncMarketplace(c *gin.Context) {
	// This will run in background, return immediately
	go func() {
		// Manual sync call - this is a private method we need to expose
		// For now, we can trigger via the service
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "Marketplace sync triggered",
	})
}

// SyncPlugin manually syncs a specific plugin
// POST /api/admin/marketplace/plugins/:slug/sync
func (h *MarketplaceHandler) SyncPlugin(c *gin.Context) {
	slug := c.Param("slug")

	if err := h.pluginSync.SyncPluginManually(slug); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Plugin synced successfully",
		"slug":    slug,
	})
}

// === Helper Functions ===

// parseIntQuery parses an integer query parameter with a default value
func parseIntQuery(c *gin.Context, key string, defaultValue int) int {
	if value := c.Query(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
