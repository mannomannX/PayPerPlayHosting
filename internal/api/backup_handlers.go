package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/internal/service"
	"github.com/payperplay/hosting/pkg/logger"
)

type BackupHandler struct {
	backupService      *service.BackupService
	backupRepo         *repository.BackupRepository
	backupQuotaService *service.BackupQuotaService
	serverRepo         *repository.ServerRepository
}

func NewBackupHandler(
	backupService *service.BackupService,
	backupRepo *repository.BackupRepository,
	backupQuotaService *service.BackupQuotaService,
	serverRepo *repository.ServerRepository,
) *BackupHandler {
	return &BackupHandler{
		backupService:      backupService,
		backupRepo:         backupRepo,
		backupQuotaService: backupQuotaService,
		serverRepo:         serverRepo,
	}
}

// CreateBackupRequest represents the request body for creating a backup
type CreateBackupRequest struct {
	Type          models.BackupType `json:"type" binding:"required"`
	Description   string            `json:"description"`
	RetentionDays int               `json:"retention_days"` // 0 = use default based on type
}

// RestoreBackupRequest represents the request body for restoring a backup
type RestoreBackupRequest struct {
	BackupID string `json:"backup_id" binding:"required"`
}

// CreateBackup handles POST /api/servers/:id/backups
func (h *BackupHandler) CreateBackup(c *gin.Context) {
	serverID := c.Param("id")

	var req CreateBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate backup type
	validTypes := []models.BackupType{
		models.BackupTypeManual,
		models.BackupTypeScheduled,
		models.BackupTypePreMigration,
		models.BackupTypePreDeletion,
		models.BackupTypePreRestore,
	}
	isValid := false
	for _, t := range validTypes {
		if req.Type == t {
			isValid = true
			break
		}
	}
	if !isValid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid backup type"})
		return
	}

	// Get user ID from context if available (from auth middleware)
	var userID *string
	if uid, exists := c.Get("user_id"); exists {
		uidStr := uid.(string)
		userID = &uidStr
	}

	backup, err := h.backupService.CreateBackup(serverID, req.Type, req.Description, userID, req.RetentionDays)
	if err != nil {
		logger.Error("BACKUP-API: Failed to create backup", err, map[string]interface{}{
			"server_id": serverID,
			"type":      req.Type,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "backup created",
		"backup":  backup,
	})
}

// ListBackups handles GET /api/servers/:id/backups
func (h *BackupHandler) ListBackups(c *gin.Context) {
	serverID := c.Param("id")

	backups, err := h.backupRepo.FindByServerID(serverID)
	if err != nil {
		logger.Error("BACKUP-API: Failed to list backups", err, map[string]interface{}{
			"server_id": serverID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"backups": backups,
		"count":   len(backups),
	})
}

// GetBackup handles GET /api/backups/:id
func (h *BackupHandler) GetBackup(c *gin.Context) {
	backupID := c.Param("id")

	backup, err := h.backupRepo.FindByID(backupID)
	if err != nil {
		logger.Error("BACKUP-API: Failed to get backup", err, map[string]interface{}{
			"backup_id": backupID,
		})
		c.JSON(http.StatusNotFound, gin.H{"error": "backup not found"})
		return
	}

	c.JSON(http.StatusOK, backup)
}

// RestoreBackup handles POST /api/servers/:id/backups/restore
func (h *BackupHandler) RestoreBackup(c *gin.Context) {
	serverID := c.Param("id")

	var req RestoreBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify backup belongs to this server
	backup, err := h.backupRepo.FindByID(req.BackupID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "backup not found"})
		return
	}

	if backup.ServerID != serverID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "backup does not belong to this server"})
		return
	}

	if err := h.backupService.RestoreBackup(req.BackupID, serverID); err != nil {
		logger.Error("BACKUP-API: Failed to restore backup", err, map[string]interface{}{
			"server_id": serverID,
			"backup_id": req.BackupID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "backup restored successfully",
		"server_id": serverID,
		"backup_id": req.BackupID,
	})
}

// DeleteBackup handles DELETE /api/backups/:id
func (h *BackupHandler) DeleteBackup(c *gin.Context) {
	backupID := c.Param("id")

	// Verify backup exists
	backup, err := h.backupRepo.FindByID(backupID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "backup not found"})
		return
	}

	if err := h.backupService.DeleteBackup(backupID); err != nil {
		logger.Error("BACKUP-API: Failed to delete backup", err, map[string]interface{}{
			"backup_id": backupID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "backup deleted successfully",
		"backup_id": backup.ID,
	})
}

// GetBackupStats handles GET /api/backups/stats
func (h *BackupHandler) GetBackupStats(c *gin.Context) {
	stats, err := h.backupService.GetBackupStats()
	if err != nil {
		logger.Error("BACKUP-API: Failed to get backup stats", err, nil)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// CleanupExpiredBackups handles POST /api/backups/cleanup (admin only)
func (h *BackupHandler) CleanupExpiredBackups(c *gin.Context) {
	deletedCount, err := h.backupService.CleanupExpiredBackups()
	if err != nil {
		logger.Error("BACKUP-API: Failed to cleanup expired backups", err, nil)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "expired backups cleaned up",
		"deleted": deletedCount,
	})
}

// GetServerBackupStats handles GET /api/servers/:id/backups/stats
func (h *BackupHandler) GetServerBackupStats(c *gin.Context) {
	serverID := c.Param("id")

	count, err := h.backupRepo.CountByServerID(serverID)
	if err != nil {
		logger.Error("BACKUP-API: Failed to count backups", err, map[string]interface{}{
			"server_id": serverID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	totalSize, err := h.backupRepo.GetServerBackupSize(serverID)
	if err != nil {
		logger.Error("BACKUP-API: Failed to get server backup size", err, map[string]interface{}{
			"server_id": serverID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"server_id":     serverID,
		"backup_count":  count,
		"total_size_mb": totalSize / 1024 / 1024,
		"total_size_gb": float64(totalSize) / 1024 / 1024 / 1024,
	})
}

// ========================================
// User Backup Management Endpoints
// ========================================

// GetUserBackups handles GET /api/users/:id/backups
// Returns all backups for a specific user
func (h *BackupHandler) GetUserBackups(c *gin.Context) {
	userID := c.Param("id")

	backups, err := h.backupRepo.FindByUserID(userID)
	if err != nil {
		logger.Error("Failed to fetch user backups", err, map[string]interface{}{
			"user_id": userID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch backups"})
		return
	}

	// Enrich with server names
	type BackupResponse struct {
		models.Backup
		ServerName string `json:"server_name"`
	}

	var response []BackupResponse
	for _, backup := range backups {
		server, err := h.serverRepo.FindByID(backup.ServerID)
		serverName := "Unknown"
		if err == nil && server != nil {
			serverName = server.Name
		}

		response = append(response, BackupResponse{
			Backup:     backup,
			ServerName: serverName,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"backups": response,
		"count":   len(response),
	})
}

// GetUserBackupQuota handles GET /api/users/:id/backups/quota
// Returns quota information for a user
func (h *BackupHandler) GetUserBackupQuota(c *gin.Context) {
	userID := c.Param("id")

	quotaInfo, err := h.backupQuotaService.GetUserQuotaInfo(userID)
	if err != nil {
		logger.Error("Failed to fetch user quota info", err, map[string]interface{}{
			"user_id": userID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch quota information"})
		return
	}

	c.JSON(http.StatusOK, quotaInfo)
}

// RestoreUserBackup handles POST /api/users/:user_id/backups/:backup_id/restore
// Restores a backup for a user with quota enforcement
func (h *BackupHandler) RestoreUserBackup(c *gin.Context) {
	userID := c.Param("user_id")
	backupID := c.Param("backup_id")

	// Validate backup belongs to user
	backup, err := h.backupRepo.FindByID(backupID)
	if err != nil {
		logger.Error("Failed to find backup", err, map[string]interface{}{
			"backup_id": backupID,
		})
		c.JSON(http.StatusNotFound, gin.H{"error": "Backup not found"})
		return
	}

	if backup.UserID == nil || *backup.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Restore backup (quota check happens inside)
	if err := h.backupService.RestoreBackup(backupID, backup.ServerID, &userID); err != nil {
		logger.Error("Failed to restore backup", err, map[string]interface{}{
			"backup_id": backupID,
			"user_id":   userID,
		})

		// Check if error is quota-related
		if err.Error() == "restore quota exceeded" ||
		   (len(err.Error()) > 20 && err.Error()[:20] == "restore quota exceeded") {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Backup restored successfully",
		"backup_id": backupID,
		"server_id": backup.ServerID,
	})
}
