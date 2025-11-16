package conductor

import (
	"github.com/payperplay/hosting/pkg/logger"
)

// ServerRepository interface for fetching server data
type ServerRepository interface {
	GetByID(id string) (interface{}, error)
	// You can add more methods as needed
}

// SyncContainerMetadataFromDB syncs minecraft_version and server_type from database
// This is needed when containers were created before these fields were added
func (c *Conductor) SyncContainerMetadataFromDB(serverRepo interface{}) error {
	logger.Info("CONTAINER-SYNC: Starting metadata sync from database", nil)

	// Type assert to access GetByID if available
	type serverGetter interface {
		GetByID(id string) (interface{}, error)
	}

	getter, ok := serverRepo.(serverGetter)
	if !ok {
		logger.Warn("CONTAINER-SYNC: ServerRepo doesn't support GetByID, skipping sync", nil)
		return nil
	}

	containers := c.ContainerRegistry.GetAllContainers()
	syncedCount := 0

	for _, container := range containers {
		// Skip if already has version data
		if container.MinecraftVersion != "" && container.ServerType != "" {
			continue
		}

		// Fetch from database
		result, err := getter.GetByID(container.ServerID)
		if err != nil {
			logger.Warn("CONTAINER-SYNC: Failed to fetch server from DB", map[string]interface{}{
				"server_id": container.ServerID,
				"error":     err.Error(),
			})
			continue
		}

		// Type assert to get minecraft_version and server_type
		type serverWithVersion interface {
			GetMinecraftVersion() string
			GetServerType() string
		}

		if server, ok := result.(serverWithVersion); ok {
			container.MinecraftVersion = server.GetMinecraftVersion()
			container.ServerType = server.GetServerType()

			logger.Info("CONTAINER-SYNC: Updated container metadata", map[string]interface{}{
				"server_id":         container.ServerID,
				"minecraft_version": container.MinecraftVersion,
				"server_type":       container.ServerType,
			})

			syncedCount++
		}
	}

	logger.Info("CONTAINER-SYNC: Metadata sync completed", map[string]interface{}{
		"total":  len(containers),
		"synced": syncedCount,
	})

	return nil
}
