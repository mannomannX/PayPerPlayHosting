package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/service"
	"github.com/payperplay/hosting/pkg/logger"
)

// TemplateHandler handles template-related endpoints
type TemplateHandler struct {
	templateService *service.TemplateService
}

// NewTemplateHandler creates a new template handler
func NewTemplateHandler(templateService *service.TemplateService) *TemplateHandler {
	return &TemplateHandler{
		templateService: templateService,
	}
}

// GetAllTemplates returns all available templates
// GET /api/templates
func (h *TemplateHandler) GetAllTemplates(c *gin.Context) {
	templates := h.templateService.GetAllTemplates()

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"count":     len(templates),
	})
}

// GetTemplate returns a specific template by ID
// GET /api/templates/:id
func (h *TemplateHandler) GetTemplate(c *gin.Context) {
	templateID := c.Param("id")

	template, err := h.templateService.GetTemplateByID(templateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Template not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"template": template,
	})
}

// GetTemplatesByCategory returns templates filtered by category
// GET /api/templates/category/:category
func (h *TemplateHandler) GetTemplatesByCategory(c *gin.Context) {
	category := c.Param("category")

	templates := h.templateService.GetTemplatesByCategory(category)

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"category":  category,
		"count":     len(templates),
	})
}

// GetPopularTemplates returns only popular templates
// GET /api/templates/popular
func (h *TemplateHandler) GetPopularTemplates(c *gin.Context) {
	templates := h.templateService.GetPopularTemplates()

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"count":     len(templates),
	})
}

// GetCategories returns all template categories
// GET /api/templates/categories
func (h *TemplateHandler) GetCategories(c *gin.Context) {
	categories := h.templateService.GetCategories()

	c.JSON(http.StatusOK, gin.H{
		"categories": categories,
		"count":      len(categories),
	})
}

// SearchTemplates searches templates by query
// GET /api/templates/search?q=query
func (h *TemplateHandler) SearchTemplates(c *gin.Context) {
	query := c.Query("q")

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Search query is required",
		})
		return
	}

	templates := h.templateService.SearchTemplates(query)

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"query":     query,
		"count":     len(templates),
	})
}

// ApplyTemplate applies a template to an existing server
// POST /api/servers/:id/apply-template
// Body: {"template_id": "vanilla-1-21"}
func (h *TemplateHandler) ApplyTemplate(c *gin.Context) {
	serverID := c.Param("id")

	var request struct {
		TemplateID string `json:"template_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	// Check if template exists
	template, err := h.templateService.GetTemplateByID(request.TemplateID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Template not found",
		})
		return
	}

	// Apply template to server
	if err := h.templateService.ApplyTemplateToServer(serverID, request.TemplateID); err != nil {
		logger.Error("Failed to apply template", err, map[string]interface{}{
			"server_id":   serverID,
			"template_id": request.TemplateID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to apply template: " + err.Error(),
		})
		return
	}

	logger.Info("Template applied successfully", map[string]interface{}{
		"server_id":   serverID,
		"template_id": request.TemplateID,
		"template":    template.Name,
	})

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"message":  "Template applied successfully. Restart server to apply changes.",
		"template": template,
	})
}

// GetRecommendations returns template recommendations based on criteria
// GET /api/templates/recommendations?players=20&modded=false
func (h *TemplateHandler) GetRecommendations(c *gin.Context) {
	playerCount := 10 // default
	modded := false   // default

	if p := c.Query("players"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			playerCount = parsed
		}
	}

	if m := c.Query("modded"); m == "true" {
		modded = true
	}

	templates := h.templateService.GetTemplateRecommendations(playerCount, modded)

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"criteria": gin.H{
			"players": playerCount,
			"modded":  modded,
		},
		"count": len(templates),
	})
}
