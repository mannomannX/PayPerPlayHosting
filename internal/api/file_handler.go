package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/service"
	"github.com/payperplay/hosting/pkg/logger"
)

// FileHandler handles file upload/management endpoints
type FileHandler struct {
	fileService             *service.FileService
	fileIntegrationService *service.FileIntegrationService
}

// NewFileHandler creates a new file handler
func NewFileHandler(fileService *service.FileService, fileIntegrationService *service.FileIntegrationService) *FileHandler {
	return &FileHandler{
		fileService:             fileService,
		fileIntegrationService: fileIntegrationService,
	}
}

// UploadFile handles file uploads
// POST /api/servers/{id}/uploads
func (h *FileHandler) UploadFile(c *gin.Context) {
	serverID := c.Param("id")

	// Get user ID from context (set by auth middleware)
	userID := c.GetString("user_id")
	if userID == "" {
		userID = "default" // Fallback for MVP
	}

	// Parse multipart form (max 150 MB)
	if err := c.Request.ParseMultipartForm(150 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Failed to parse form: %v", err),
		})
		return
	}

	// Get file from form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Failed to get file: %v", err),
		})
		return
	}
	defer file.Close()

	// Get file type
	fileTypeStr := c.PostForm("type")
	if fileTypeStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "File type is required",
		})
		return
	}

	fileType, err := service.ParseFileType(fileTypeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Get optional metadata
	metadata := c.PostForm("metadata")

	// Get auto-activate flag
	autoActivate := c.PostForm("auto_activate") == "true"

	// Upload file
	uploadReq := service.UploadFileRequest{
		ServerID:     serverID,
		UserID:       userID,
		FileType:     fileType,
		File:         file,
		Header:       header,
		Metadata:     metadata,
		AutoActivate: autoActivate,
	}

	serverFile, err := h.fileService.UploadFile(uploadReq)
	if err != nil {
		logger.Error("Failed to upload file", err, map[string]interface{}{
			"server_id": serverID,
			"file_type": fileType,
			"user_id":   userID,
		})
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Upload failed: %v", err),
		})
		return
	}

	logger.Info("File uploaded via API", map[string]interface{}{
		"file_id":   serverFile.ID,
		"server_id": serverID,
		"file_type": fileType,
		"user_id":   userID,
	})

	c.JSON(http.StatusCreated, serverFile)
}

// ListFiles lists all files for a server
// GET /api/servers/{id}/uploads?type=resource_pack
func (h *FileHandler) ListFiles(c *gin.Context) {
	serverID := c.Param("id")

	// Get optional type filter
	typeStr := c.Query("type")

	var files []models.ServerFile
	var err error

	if typeStr != "" {
		fileType, err := service.ParseFileType(typeStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		files, err = h.fileService.GetFilesByServerAndType(serverID, fileType)
	} else {
		files, err = h.fileService.GetFilesByServer(serverID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to get files: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, files)
}

// GetFile gets a specific file
// GET /api/servers/{id}/uploads/{fileId}
func (h *FileHandler) GetFile(c *gin.Context) {
	serverID := c.Param("id")
	fileID := c.Param("fileId")

	// Get file path
	filePath, err := h.fileService.GetFilePath(serverID, fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": fmt.Sprintf("File not found: %v", err),
		})
		return
	}

	// Serve file
	c.File(filePath)
}

// ActivateFile activates a file
// PUT /api/servers/{id}/uploads/{fileId}/activate
func (h *FileHandler) ActivateFile(c *gin.Context) {
	serverID := c.Param("id")
	fileID := c.Param("fileId")

	// Activate the file
	if err := h.fileService.ActivateFile(fileID, serverID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Failed to activate file: %v", err),
		})
		return
	}

	// Apply the file to the server configuration
	if err := h.fileIntegrationService.ApplyFile(serverID, fileID); err != nil {
		logger.Warn("Failed to apply file to server", map[string]interface{}{
			"server_id": serverID,
			"file_id":   fileID,
			"error":     err.Error(),
		})
		// Don't fail the activation - the file is activated, just not applied yet
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "File activated",
	})
}

// DeactivateFile deactivates a file
// PUT /api/servers/{id}/uploads/{fileId}/deactivate
func (h *FileHandler) DeactivateFile(c *gin.Context) {
	serverID := c.Param("id")
	fileID := c.Param("fileId")

	// Get file info before deactivating (to know the type)
	file, err := h.fileService.GetActiveFile(serverID, models.FileTypeResourcePack)
	if err != nil || file == nil || file.ID != fileID {
		// Try other file types
		file, _ = h.fileService.GetActiveFile(serverID, models.FileTypeDataPack)
		if file == nil || file.ID != fileID {
			file, _ = h.fileService.GetActiveFile(serverID, models.FileTypeServerIcon)
			if file == nil || file.ID != fileID {
				file, _ = h.fileService.GetActiveFile(serverID, models.FileTypeWorldGen)
			}
		}
	}
	var fileType models.FileType
	if file != nil && file.ID == fileID {
		fileType = file.FileType
	}

	// Deactivate the file
	if err := h.fileService.DeactivateFile(fileID, serverID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Failed to deactivate file: %v", err),
		})
		return
	}

	// Remove from server configuration
	if fileType != "" {
		if err := h.fileIntegrationService.RemoveFile(serverID, fileID, fileType); err != nil {
			logger.Warn("Failed to remove file from server", map[string]interface{}{
				"server_id": serverID,
				"file_id":   fileID,
				"file_type": fileType,
				"error":     err.Error(),
			})
			// Don't fail the deactivation - the file is deactivated, just not removed from config yet
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "File deactivated",
	})
}

// DeleteFile deletes a file
// DELETE /api/servers/{id}/uploads/{fileId}
func (h *FileHandler) DeleteFile(c *gin.Context) {
	serverID := c.Param("id")
	fileID := c.Param("fileId")

	if err := h.fileService.DeleteFile(fileID, serverID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Failed to delete file: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "File deleted",
	})
}
