package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/internal/service"
	"github.com/payperplay/hosting/pkg/logger"
)

// BackupScheduleHandler handles backup schedule endpoints
type BackupScheduleHandler struct {
	schedulerService *service.BackupScheduler
	serverRepo       *repository.ServerRepository
}

// NewBackupScheduleHandler creates a new backup schedule handler
func NewBackupScheduleHandler(schedulerService *service.BackupScheduler, serverRepo *repository.ServerRepository) *BackupScheduleHandler {
	return &BackupScheduleHandler{
		schedulerService: schedulerService,
		serverRepo:       serverRepo,
	}
}

// GetSchedule returns the backup schedule for a server
// GET /api/servers/:id/backup-schedule
func (h *BackupScheduleHandler) GetSchedule(c *gin.Context) {
	serverID := c.Param("id")

	// Verify server exists and user has access
	server, err := h.serverRepo.FindByID(serverID)
	if err != nil || server == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}

	schedule, err := h.schedulerService.GetSchedule(server.ID)
	if err != nil {
		logger.Error("Failed to get backup schedule", err, map[string]interface{}{
			"server_id": serverID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get backup schedule"})
		return
	}

	if schedule == nil {
		c.JSON(http.StatusOK, gin.H{
			"configured": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"configured": true,
		"schedule":   schedule,
	})
}

// CreateSchedule creates a new backup schedule
// POST /api/servers/:id/backup-schedule
// Body: {"enabled": true, "frequency": "daily", "schedule_time": "03:00", "max_backups": 7}
func (h *BackupScheduleHandler) CreateSchedule(c *gin.Context) {
	serverID := c.Param("id")

	// Verify server exists and user has access
	server, err := h.serverRepo.FindByID(serverID)
	if err != nil || server == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}

	var request struct {
		Enabled      bool   `json:"enabled"`
		Frequency    string `json:"frequency" binding:"required"`
		ScheduleTime string `json:"schedule_time" binding:"required"`
		MaxBackups   int    `json:"max_backups"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate frequency
	if request.Frequency != "daily" && request.Frequency != "weekly" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid frequency (must be 'daily' or 'weekly')"})
		return
	}

	// Default max_backups to 7 if not provided
	if request.MaxBackups <= 0 {
		request.MaxBackups = 7
	}

	schedule, err := h.schedulerService.CreateSchedule(
		server.ID,
		request.Enabled,
		request.Frequency,
		request.ScheduleTime,
		request.MaxBackups,
	)
	if err != nil {
		logger.Error("Failed to create backup schedule", err, map[string]interface{}{
			"server_id": serverID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	logger.Info("Backup schedule created", map[string]interface{}{
		"server_id": serverID,
		"enabled":   request.Enabled,
		"frequency": request.Frequency,
	})

	c.JSON(http.StatusCreated, gin.H{
		"status":   "success",
		"message":  "Backup schedule created successfully",
		"schedule": schedule,
	})
}

// UpdateSchedule updates a backup schedule
// PUT /api/servers/:id/backup-schedule
// Body: {"enabled": true, "frequency": "daily", "schedule_time": "03:00", "max_backups": 7}
func (h *BackupScheduleHandler) UpdateSchedule(c *gin.Context) {
	serverID := c.Param("id")

	// Verify server exists and user has access
	server, err := h.serverRepo.FindByID(serverID)
	if err != nil || server == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate frequency if provided
	if freq, ok := updates["frequency"].(string); ok {
		if freq != "daily" && freq != "weekly" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid frequency (must be 'daily' or 'weekly')"})
			return
		}
	}

	// Only allow specific fields to be updated
	allowedFields := map[string]bool{
		"enabled":       true,
		"frequency":     true,
		"schedule_time": true,
		"max_backups":   true,
	}

	filteredUpdates := make(map[string]interface{})
	for key, value := range updates {
		if allowedFields[key] {
			filteredUpdates[key] = value
		}
	}

	schedule, err := h.schedulerService.UpdateSchedule(server.ID, filteredUpdates)
	if err != nil {
		logger.Error("Failed to update backup schedule", err, map[string]interface{}{
			"server_id": serverID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	logger.Info("Backup schedule updated", map[string]interface{}{
		"server_id": serverID,
	})

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"message":  "Backup schedule updated successfully",
		"schedule": schedule,
	})
}

// DeleteSchedule deletes a backup schedule
// DELETE /api/servers/:id/backup-schedule
func (h *BackupScheduleHandler) DeleteSchedule(c *gin.Context) {
	serverID := c.Param("id")

	// Verify server exists and user has access
	server, err := h.serverRepo.FindByID(serverID)
	if err != nil || server == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}

	if err := h.schedulerService.DeleteSchedule(server.ID); err != nil {
		logger.Error("Failed to delete backup schedule", err, map[string]interface{}{
			"server_id": serverID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete backup schedule"})
		return
	}

	logger.Info("Backup schedule deleted", map[string]interface{}{
		"server_id": serverID,
	})

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Backup schedule deleted successfully",
	})
}
