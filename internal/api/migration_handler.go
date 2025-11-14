package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/logger"
)

// MigrationHandler handles migration-related HTTP requests
type MigrationHandler struct {
	migrationRepo *repository.MigrationRepository
	serverRepo    *repository.ServerRepository
}

// NewMigrationHandler creates a new migration handler
func NewMigrationHandler(migrationRepo *repository.MigrationRepository, serverRepo *repository.ServerRepository) *MigrationHandler {
	return &MigrationHandler{
		migrationRepo: migrationRepo,
		serverRepo:    serverRepo,
	}
}

// ListMigrations returns all migrations with optional filters
// GET /api/migrations
func (h *MigrationHandler) ListMigrations(c *gin.Context) {
	// Parse query parameters
	filters := make(map[string]interface{})

	if status := c.Query("status"); status != "" {
		// Support comma-separated statuses
		statuses := strings.Split(status, ",")
		if len(statuses) == 1 {
			filters["status"] = models.MigrationStatus(statuses[0])
		}
		// TODO: Support multiple statuses with IN query
	}

	if serverID := c.Query("server_id"); serverID != "" {
		filters["server_id"] = serverID
	}

	if reason := c.Query("reason"); reason != "" {
		filters["reason"] = models.MigrationReason(reason)
	}

	// Pagination
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit > 200 {
		limit = 200 // Max 200 per request
	}

	migrations, err := h.migrationRepo.FindAll(filters, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch migrations",
		})
		return
	}

	// Get total count
	total, _ := h.migrationRepo.Count(filters)

	c.JSON(http.StatusOK, gin.H{
		"status":     "ok",
		"migrations": migrations,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
	})
}

// GetMigration returns a specific migration by ID
// GET /api/migrations/:id
func (h *MigrationHandler) GetMigration(c *gin.Context) {
	migrationID := c.Param("id")

	migration, err := h.migrationRepo.FindByID(migrationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Migration not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"migration": migration,
	})
}

// ApproveMigration approves a suggested migration
// POST /api/migrations/:id/approve
func (h *MigrationHandler) ApproveMigration(c *gin.Context) {
	migrationID := c.Param("id")

	migration, err := h.migrationRepo.FindByID(migrationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Migration not found",
		})
		return
	}

	// Check if migration can be approved
	if migration.Status != models.MigrationStatusSuggested {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Only suggested migrations can be approved",
		})
		return
	}

	// Update status to approved
	now := time.Now()
	migration.Status = models.MigrationStatusApproved
	migration.ApprovedAt = &now

	if err := h.migrationRepo.Update(migration); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to approve migration",
		})
		return
	}

	// TODO: Trigger WebSocket event

	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"message":   "Migration approved",
		"migration": migration,
	})
}

// ScheduleMigration schedules an approved migration
// POST /api/migrations/:id/schedule
func (h *MigrationHandler) ScheduleMigration(c *gin.Context) {
	migrationID := c.Param("id")

	migration, err := h.migrationRepo.FindByID(migrationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Migration not found",
		})
		return
	}

	// Check if migration can be scheduled
	if migration.Status != models.MigrationStatusApproved {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Only approved migrations can be scheduled",
		})
		return
	}

	// Update status to scheduled
	now := time.Now()
	migration.Status = models.MigrationStatusScheduled
	migration.ScheduledAt = &now

	if err := h.migrationRepo.Update(migration); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to schedule migration",
		})
		return
	}

	// TODO: Migration service will pick this up

	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"message":   "Migration scheduled",
		"migration": migration,
	})
}

// CancelMigration cancels a pending migration
// POST /api/migrations/:id/cancel
func (h *MigrationHandler) CancelMigration(c *gin.Context) {
	migrationID := c.Param("id")

	migration, err := h.migrationRepo.FindByID(migrationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Migration not found",
		})
		return
	}

	// Check if migration can be cancelled
	if !migration.CanBeCancelled() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Migration cannot be cancelled in current state",
		})
		return
	}

	// Update status to cancelled
	migration.Status = models.MigrationStatusCancelled

	if err := h.migrationRepo.Update(migration); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to cancel migration",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"message":   "Migration cancelled",
		"migration": migration,
	})
}

// DeleteMigration deletes a migration record
// DELETE /api/migrations/:id
func (h *MigrationHandler) DeleteMigration(c *gin.Context) {
	migrationID := c.Param("id")

	migration, err := h.migrationRepo.FindByID(migrationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Migration not found",
		})
		return
	}

	// Only allow deletion of completed/failed/cancelled migrations
	if !migration.IsCompleted() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Cannot delete active migration",
		})
		return
	}

	if err := h.migrationRepo.Delete(migrationID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete migration",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Migration deleted",
	})
}

// GetServerMigrations returns all migrations for a specific server
// GET /api/servers/:id/migrations
func (h *MigrationHandler) GetServerMigrations(c *gin.Context) {
	serverID := c.Param("id")

	// Verify server exists
	_, err := h.serverRepo.FindByID(serverID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Server not found",
		})
		return
	}

	migrations, err := h.migrationRepo.FindByServerID(serverID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch migrations",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "ok",
		"server_id":  serverID,
		"migrations": migrations,
		"count":      len(migrations),
	})
}

// GetActiveMigration returns the active migration for a server (if any)
// GET /api/servers/:id/migrations/active
func (h *MigrationHandler) GetActiveMigration(c *gin.Context) {
	serverID := c.Param("id")

	migration, err := h.migrationRepo.FindActiveMigrationForServer(serverID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch active migration",
		})
		return
	}

	if migration == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"server_id": serverID,
			"migration": nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"server_id": serverID,
		"migration": migration,
	})
}

// GetMigrationStats returns migration statistics
// GET /api/migrations/stats
func (h *MigrationHandler) GetMigrationStats(c *gin.Context) {
	stats, err := h.migrationRepo.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch statistics",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"stats":  stats,
	})
}

// CreateManualMigration creates a manual migration request
// POST /api/migrations
func (h *MigrationHandler) CreateManualMigration(c *gin.Context) {
	var req struct {
		ServerID   string `json:"server_id" binding:"required"`
		ToNodeID   string `json:"to_node_id" binding:"required"`
		Reason     string `json:"reason"`
		AutoApprove bool  `json:"auto_approve"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	// Verify server exists
	server, err := h.serverRepo.FindByID(req.ServerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Server not found",
		})
		return
	}

	// Check if server can be migrated (no cooldown for manual migrations)
	canMigrate, err := h.migrationRepo.CanMigrateServer(req.ServerID, 0)
	if err != nil {
		logger.Error("Failed to check if server can be migrated", err, map[string]interface{}{
			"server_id": req.ServerID,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to validate migration eligibility",
		})
		return
	}
	if !canMigrate {
		logger.Warn("Server cannot be migrated - active migration in progress", map[string]interface{}{
			"server_id": req.ServerID,
		})
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Server cannot be migrated (active migration in progress)",
		})
		return
	}

	// Create migration
	now := time.Now()
	status := models.MigrationStatusSuggested
	if req.AutoApprove {
		status = models.MigrationStatusScheduled
	}

	migration := &models.Migration{
		ID:         uuid.New().String(),
		ServerID:   req.ServerID,
		FromNodeID: server.NodeID,
		ToNodeID:   req.ToNodeID,
		Status:     status,
		Reason:     models.MigrationReasonManual,
		CreatedAt:  now,
		TriggeredBy: "admin",
		Notes:      req.Reason,
	}

	if req.AutoApprove {
		migration.ApprovedAt = &now
		migration.ScheduledAt = &now
	}

	if err := h.migrationRepo.Create(migration); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create migration",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":    "ok",
		"message":   "Migration created",
		"migration": migration,
	})
}
