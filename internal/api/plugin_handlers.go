package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/service"
)

type PluginHandler struct {
	pluginService *service.PluginService
}

func NewPluginHandler(pluginService *service.PluginService) *PluginHandler {
	return &PluginHandler{
		pluginService: pluginService,
	}
}

// InstallPlugin handles POST /api/servers/:id/plugins
func (h *PluginHandler) InstallPlugin(c *gin.Context) {
	serverID := c.Param("id")

	var req struct {
		PluginURL string `json:"plugin_url" binding:"required"`
		Filename  string `json:"filename" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.pluginService.InstallPlugin(serverID, req.PluginURL, req.Filename); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "plugin installed successfully"})
}

// ListPlugins handles GET /api/servers/:id/plugins
func (h *PluginHandler) ListPlugins(c *gin.Context) {
	serverID := c.Param("id")

	plugins, err := h.pluginService.ListInstalledPlugins(serverID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, plugins)
}

// RemovePlugin handles DELETE /api/servers/:id/plugins/:filename
func (h *PluginHandler) RemovePlugin(c *gin.Context) {
	serverID := c.Param("id")
	filename := c.Param("filename")

	if err := h.pluginService.RemovePlugin(serverID, filename); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "plugin removed successfully"})
}

// SearchPlugins handles GET /api/plugins/search
func (h *PluginHandler) SearchPlugins(c *gin.Context) {
	query := c.DefaultQuery("q", "")

	plugins, err := h.pluginService.SearchSpigotPlugins(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, plugins)
}

// SearchModPacks handles GET /api/modpacks/search
func (h *PluginHandler) SearchModPacks(c *gin.Context) {
	query := c.DefaultQuery("q", "")
	mcVersion := c.DefaultQuery("version", "")
	loader := c.DefaultQuery("loader", "forge")

	modpacks, err := h.pluginService.SearchModPacks(query, mcVersion, loader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, modpacks)
}

// InstallModPack handles POST /api/servers/:id/modpack
func (h *PluginHandler) InstallModPack(c *gin.Context) {
	serverID := c.Param("id")

	var req struct {
		ModPackURL string `json:"modpack_url" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.pluginService.InstallModPack(serverID, req.ModPackURL); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "modpack installation started"})
}
