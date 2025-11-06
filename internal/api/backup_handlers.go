package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/service"
)

type BackupHandler struct {
	backupService *service.BackupService
}

func NewBackupHandler(backupService *service.BackupService) *BackupHandler {
	return &BackupHandler{
		backupService: backupService,
	}
}

// CreateBackup handles POST /api/servers/:id/backups
func (h *BackupHandler) CreateBackup(c *gin.Context) {
	serverID := c.Param("id")

	backupPath, err := h.backupService.CreateBackup(serverID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "backup created",
		"path":    backupPath,
	})
}

// ListBackups handles GET /api/servers/:id/backups
func (h *BackupHandler) ListBackups(c *gin.Context) {
	serverID := c.Param("id")

	backups, err := h.backupService.ListBackups(serverID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, backups)
}

// RestoreBackup handles POST /api/servers/:id/backups/restore
func (h *BackupHandler) RestoreBackup(c *gin.Context) {
	serverID := c.Param("id")

	var req struct {
		BackupPath string `json:"backup_path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.backupService.RestoreBackup(serverID, req.BackupPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "backup restored successfully"})
}

// DeleteBackup handles DELETE /api/servers/:id/backups/:filename
func (h *BackupHandler) DeleteBackup(c *gin.Context) {
	serverID := c.Param("id")
	filename := c.Param("filename")

	// Get backup path
	backups, err := h.backupService.ListBackups(serverID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var backupPath string
	for _, backup := range backups {
		if backup.Filename == filename {
			backupPath = backup.Path
			break
		}
	}

	if backupPath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "backup not found"})
		return
	}

	if err := h.backupService.DeleteBackup(backupPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "backup deleted"})
}
