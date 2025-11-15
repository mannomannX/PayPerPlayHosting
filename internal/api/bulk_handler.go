package api

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/service"
	"github.com/payperplay/hosting/pkg/logger"
)

// BulkHandler handles bulk operations on multiple servers
type BulkHandler struct {
	mcService     *service.MinecraftService
	backupService *service.BackupService
}

// NewBulkHandler creates a new bulk handler
func NewBulkHandler(mcService *service.MinecraftService, backupService *service.BackupService) *BulkHandler {
	return &BulkHandler{
		mcService:     mcService,
		backupService: backupService,
	}
}

// BulkRequest represents a request to perform bulk actions
type BulkRequest struct {
	ServerIDs []string `json:"server_ids" binding:"required,min=1"`
}

// BulkResult represents the result of a bulk operation
type BulkResult struct {
	Success []BulkItem `json:"success"`
	Failed  []BulkItem `json:"failed"`
}

// BulkItem represents a single item in a bulk operation result
type BulkItem struct {
	ServerID string `json:"server_id"`
	Message  string `json:"message,omitempty"`
}

// BulkStartServers starts multiple servers
// POST /api/servers/bulk/start
func (h *BulkHandler) BulkStartServers(c *gin.Context) {
	userID := c.GetString("user_id")

	var req BulkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	result := h.executeBulkOperation(req.ServerIDs, userID, func(serverID string) error {
		return h.mcService.StartServer(serverID)
	})

	logger.Info("Bulk start operation completed", map[string]interface{}{
		"user_id":       userID,
		"total":         len(req.ServerIDs),
		"success_count": len(result.Success),
		"failed_count":  len(result.Failed),
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "Bulk start operation completed",
		"result":  result,
	})
}

// BulkStopServers stops multiple servers
// POST /api/servers/bulk/stop
func (h *BulkHandler) BulkStopServers(c *gin.Context) {
	userID := c.GetString("user_id")

	var req BulkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	result := h.executeBulkOperation(req.ServerIDs, userID, func(serverID string) error {
		return h.mcService.StopServer(serverID, "Bulk stop operation")
	})

	logger.Info("Bulk stop operation completed", map[string]interface{}{
		"user_id":       userID,
		"total":         len(req.ServerIDs),
		"success_count": len(result.Success),
		"failed_count":  len(result.Failed),
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "Bulk stop operation completed",
		"result":  result,
	})
}

// BulkDeleteServers deletes multiple servers
// POST /api/servers/bulk/delete
func (h *BulkHandler) BulkDeleteServers(c *gin.Context) {
	userID := c.GetString("user_id")

	var req BulkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	result := h.executeBulkOperation(req.ServerIDs, userID, func(serverID string) error {
		return h.mcService.DeleteServer(serverID)
	})

	logger.Info("Bulk delete operation completed", map[string]interface{}{
		"user_id":       userID,
		"total":         len(req.ServerIDs),
		"success_count": len(result.Success),
		"failed_count":  len(result.Failed),
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "Bulk delete operation completed",
		"result":  result,
	})
}

// BulkBackupServers creates backups for multiple servers
// POST /api/servers/bulk/backup
func (h *BulkHandler) BulkBackupServers(c *gin.Context) {
	userID := c.GetString("user_id")

	var req BulkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	userIDPtr := &userID
	result := h.executeBulkOperation(req.ServerIDs, userID, func(serverID string) error {
		_, err := h.backupService.CreateBackup(
			serverID,
			models.BackupTypeManual,
			"Bulk manual backup",
			userIDPtr,
			0, // Use default retention (30 days for manual)
		)
		return err
	})

	logger.Info("Bulk backup operation completed", map[string]interface{}{
		"user_id":       userID,
		"total":         len(req.ServerIDs),
		"success_count": len(result.Success),
		"failed_count":  len(result.Failed),
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "Bulk backup operation completed",
		"result":  result,
	})
}

// executeBulkOperation executes a bulk operation in parallel
func (h *BulkHandler) executeBulkOperation(serverIDs []string, userID string, operation func(string) error) BulkResult {
	var wg sync.WaitGroup
	var mu sync.Mutex

	result := BulkResult{
		Success: []BulkItem{},
		Failed:  []BulkItem{},
	}

	// Execute operations in parallel with max concurrency of 10
	semaphore := make(chan struct{}, 10)

	for _, serverID := range serverIDs {
		wg.Add(1)
		go func(sid string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			err := operation(sid)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				result.Failed = append(result.Failed, BulkItem{
					ServerID: sid,
					Message:  err.Error(),
				})
			} else {
				result.Success = append(result.Success, BulkItem{
					ServerID: sid,
				})
			}
		}(serverID)
	}

	wg.Wait()

	return result
}
