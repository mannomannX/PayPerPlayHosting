package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/conductor"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/logger"
)

// ContainerSyncHandler handles container metadata synchronization
type ContainerSyncHandler struct {
	conductor *conductor.Conductor
	serverRepo *repository.ServerRepository
}

// NewContainerSyncHandler creates a new handler
func NewContainerSyncHandler(c *conductor.Conductor, serverRepo *repository.ServerRepository) *ContainerSyncHandler {
	return &ContainerSyncHandler{
		conductor:  c,
		serverRepo: serverRepo,
	}
}

// SyncContainerMetadata syncs minecraft_version and server_type from database to ContainerRegistry
func (h *ContainerSyncHandler) SyncContainerMetadata(c *gin.Context) {
	logger.Info("API: Starting container metadata sync", nil)

	containers := h.conductor.ContainerRegistry.GetAllContainers()
	syncedCount := 0

	for _, container := range containers {
		// Fetch server from database
		server, err := h.serverRepo.FindByID(container.ServerID)
		if err != nil {
			logger.Warn("SYNC: Failed to fetch server from DB", map[string]interface{}{
				"server_id": container.ServerID,
				"error":     err.Error(),
			})
			continue
		}

		// Update container metadata
		container.MinecraftVersion = server.MinecraftVersion
		container.ServerType = server.ServerType

		logger.Info("SYNC: Updated container metadata", map[string]interface{}{
			"server_id":         container.ServerID,
			"minecraft_version": container.MinecraftVersion,
			"server_type":       container.ServerType,
		})

		syncedCount++
	}

	// Save updated state to file
	if err := h.conductor.SaveContainerState("/app/data/container_state.json"); err != nil {
		logger.Error("SYNC: Failed to save container state", err, map[string]interface{}{})
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Container metadata synced successfully",
		"total":   len(containers),
		"synced":  syncedCount,
	})
}
