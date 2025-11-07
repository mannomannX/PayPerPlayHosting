package api

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/service"
	"github.com/payperplay/hosting/pkg/logger"
)

// WorldHandler handles world management endpoints
type WorldHandler struct {
	worldService *service.WorldService
}

// NewWorldHandler creates a new world handler
func NewWorldHandler(worldService *service.WorldService) *WorldHandler {
	return &WorldHandler{
		worldService: worldService,
	}
}

// ListWorlds returns information about all worlds for a server
// GET /api/servers/:id/worlds
func (h *WorldHandler) ListWorlds(c *gin.Context) {
	serverID := c.Param("id")

	worlds, err := h.worldService.ListWorlds(serverID)
	if err != nil {
		logger.Error("Failed to list worlds", err, map[string]interface{}{
			"server_id": serverID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list worlds: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"worlds": worlds,
	})
}

// DownloadWorld creates and serves a ZIP archive of a world
// GET /api/servers/:id/worlds/:name/download
func (h *WorldHandler) DownloadWorld(c *gin.Context) {
	serverID := c.Param("id")
	worldName := c.Param("name")

	// Create ZIP archive
	zipPath, err := h.worldService.DownloadWorld(serverID, worldName)
	if err != nil {
		logger.Error("Failed to prepare world download", err, map[string]interface{}{
			"server_id": serverID,
			"world":     worldName,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to prepare world download: " + err.Error(),
		})
		return
	}

	// Clean up ZIP file after serving
	defer func() {
		if err := os.Remove(zipPath); err != nil {
			logger.Warn("Failed to clean up temporary ZIP file", map[string]interface{}{
				"zip_path": zipPath,
				"error":    err.Error(),
			})
		}
	}()

	// Get file info for headers
	fileInfo, err := os.Stat(zipPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to read world archive",
		})
		return
	}

	// Set headers for file download
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(zipPath)))
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	// Serve file
	c.File(zipPath)

	logger.Info("World download served", map[string]interface{}{
		"server_id": serverID,
		"world":     worldName,
		"zip_size":  fileInfo.Size(),
	})
}

// UploadWorld handles world ZIP upload
// POST /api/servers/:id/worlds/upload
// Form data: file (ZIP), worldName (string)
func (h *WorldHandler) UploadWorld(c *gin.Context) {
	serverID := c.Param("id")

	// Get world name from form
	worldName := c.PostForm("worldName")
	if worldName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "worldName is required",
		})
		return
	}

	// Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No file uploaded",
		})
		return
	}

	// Validate file type
	if !isZipFile(file) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "File must be a ZIP archive",
		})
		return
	}

	// Save uploaded file temporarily
	tempDir := filepath.Join("servers", "temp", "uploads")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create temp directory",
		})
		return
	}

	tempPath := filepath.Join(tempDir, file.Filename)
	if err := c.SaveUploadedFile(file, tempPath); err != nil {
		logger.Error("Failed to save uploaded file", err, map[string]interface{}{
			"server_id": serverID,
			"world":     worldName,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save uploaded file",
		})
		return
	}

	// Clean up temp file after processing
	defer func() {
		if err := os.Remove(tempPath); err != nil {
			logger.Warn("Failed to clean up temporary upload file", map[string]interface{}{
				"temp_path": tempPath,
				"error":     err.Error(),
			})
		}
	}()

	// Upload world
	if err := h.worldService.UploadWorld(serverID, worldName, tempPath); err != nil {
		logger.Error("Failed to upload world", err, map[string]interface{}{
			"server_id": serverID,
			"world":     worldName,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	logger.Info("World uploaded successfully", map[string]interface{}{
		"server_id": serverID,
		"world":     worldName,
		"file_size": file.Size,
	})

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "World uploaded successfully",
		"world":   worldName,
	})
}

// ResetWorld resets a world (deletes it so it regenerates)
// POST /api/servers/:id/worlds/:name/reset
func (h *WorldHandler) ResetWorld(c *gin.Context) {
	serverID := c.Param("id")
	worldName := c.Param("name")

	if err := h.worldService.ResetWorld(serverID, worldName); err != nil {
		logger.Error("Failed to reset world", err, map[string]interface{}{
			"server_id": serverID,
			"world":     worldName,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	logger.Info("World reset successfully", map[string]interface{}{
		"server_id": serverID,
		"world":     worldName,
	})

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "World reset successfully. It will regenerate on next server start.",
		"world":   worldName,
	})
}

// DeleteWorld permanently deletes a world
// DELETE /api/servers/:id/worlds/:name
func (h *WorldHandler) DeleteWorld(c *gin.Context) {
	serverID := c.Param("id")
	worldName := c.Param("name")

	if err := h.worldService.DeleteWorld(serverID, worldName); err != nil {
		logger.Error("Failed to delete world", err, map[string]interface{}{
			"server_id": serverID,
			"world":     worldName,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	logger.Info("World deleted successfully", map[string]interface{}{
		"server_id": serverID,
		"world":     worldName,
	})

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "World deleted successfully",
		"world":   worldName,
	})
}

// Helper functions

// isZipFile checks if the uploaded file is a ZIP archive
func isZipFile(file *multipart.FileHeader) bool {
	// Check file extension
	if filepath.Ext(file.Filename) != ".zip" {
		return false
	}

	// Check MIME type
	contentType := file.Header.Get("Content-Type")
	validTypes := []string{
		"application/zip",
		"application/x-zip-compressed",
		"application/x-zip",
	}

	for _, validType := range validTypes {
		if contentType == validType {
			return true
		}
	}

	// If Content-Type is not set or ambiguous, rely on extension
	return contentType == "" || contentType == "application/octet-stream"
}
