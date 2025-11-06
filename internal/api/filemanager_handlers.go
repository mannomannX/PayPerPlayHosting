package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/service"
)

type FileManagerHandler struct {
	service *service.FileManagerService
}

func NewFileManagerHandler(service *service.FileManagerService) *FileManagerHandler {
	return &FileManagerHandler{
		service: service,
	}
}

// GetAllowedFiles returns list of editable files
// GET /api/servers/:id/files
func (h *FileManagerHandler) GetAllowedFiles(c *gin.Context) {
	serverID := c.Param("id")

	files, err := h.service.GetAllowedFiles(serverID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"files": files,
		"count": len(files),
	})
}

// ReadFile reads a configuration file
// GET /api/servers/:id/files/read?path=server.properties
func (h *FileManagerHandler) ReadFile(c *gin.Context) {
	serverID := c.Param("id")
	filePath := c.Query("path")

	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path parameter is required"})
		return
	}

	content, err := h.service.ReadFile(serverID, filePath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"path":    filePath,
		"content": content,
	})
}

// WriteFile writes content to a file
// POST /api/servers/:id/files/write
type WriteFileRequest struct {
	Path    string `json:"path" binding:"required"`
	Content string `json:"content" binding:"required"`
}

func (h *FileManagerHandler) WriteFile(c *gin.Context) {
	serverID := c.Param("id")

	var req WriteFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.WriteFile(serverID, req.Path, req.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "File written successfully",
		"path":    req.Path,
	})
}

// ListFiles lists all files in server directory
// GET /api/servers/:id/files/list?path=plugins
func (h *FileManagerHandler) ListFiles(c *gin.Context) {
	serverID := c.Param("id")
	subPath := c.Query("path")

	files, err := h.service.ListFiles(serverID, subPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"path":  subPath,
		"files": files,
		"count": len(files),
	})
}
