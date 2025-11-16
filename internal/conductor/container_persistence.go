package conductor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/payperplay/hosting/pkg/logger"
)

// PersistedContainerState represents container state for recovery after restart
// NOTE: Timing information (LastStartedAt, LastStoppedAt) is stored in the database (minecraft_servers table)
// This struct only tracks WHICH containers exist on WHICH nodes for state recovery
type PersistedContainerState struct {
	ServerID         string `json:"server_id"`
	ServerName       string `json:"server_name"`
	ContainerID      string `json:"container_id"`
	NodeID           string `json:"node_id"`
	Status           string `json:"status"` // running, stopped, sleeping
	RAMMb            int    `json:"ram_mb"`
	Port             int    `json:"port"`
	MinecraftPort    int    `json:"minecraft_port"`
	MinecraftVersion string `json:"minecraft_version"`
	ServerType       string `json:"server_type"`
}

// SaveContainerState persists all container states to JSON file
// Called during graceful shutdown to preserve container-to-node mappings
// NOTE: Timing info (LastStartedAt/LastStoppedAt) is stored in database, not here
func (c *Conductor) SaveContainerState(filePath string) error {
	containers := []PersistedContainerState{}

	c.ContainerRegistry.mu.RLock()
	for _, container := range c.ContainerRegistry.containers {
		state := PersistedContainerState{
			ServerID:         container.ServerID,
			ServerName:       container.ServerName,
			ContainerID:      container.ContainerID,
			NodeID:           container.NodeID,
			Status:           container.Status,
			RAMMb:            container.RAMMb,
			Port:             container.DockerPort,
			MinecraftPort:    container.MinecraftPort,
			MinecraftVersion: container.MinecraftVersion,
			ServerType:       container.ServerType,
		}
		containers = append(containers, state)
	}
	c.ContainerRegistry.mu.RUnlock()

	logger.Info("CONTAINER-PERSIST: Saving container state", map[string]interface{}{
		"containers": len(containers),
		"file":       filePath,
	})

	// Marshal to JSON
	data, err := json.MarshalIndent(containers, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal container state: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Write atomically
	tempFile := filePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	if err := os.Rename(tempFile, filePath); err != nil {
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	logger.Info("CONTAINER-PERSIST: Container state saved successfully", map[string]interface{}{
		"containers": len(containers),
	})

	return nil
}

// LoadContainerState loads persisted container state from JSON file
func (c *Conductor) LoadContainerState(filePath string) ([]PersistedContainerState, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info("CONTAINER-PERSIST: No container state file found (clean start)", nil)
			return []PersistedContainerState{}, nil
		}
		return nil, fmt.Errorf("failed to read container state file: %w", err)
	}

	var containers []PersistedContainerState
	if err := json.Unmarshal(data, &containers); err != nil {
		return nil, fmt.Errorf("failed to unmarshal container state: %w", err)
	}

	logger.Info("CONTAINER-PERSIST: Loaded container state from file", map[string]interface{}{
		"containers": len(containers),
	})

	return containers, nil
}

// RestoreContainersFromState restores containers from persisted state
// This reconciles state file with actual containers running on nodes
// Returns: (syncedCount, errors)
func (c *Conductor) RestoreContainersFromState(filePath string, serverRepo interface{}) (int, error) {
	states, err := c.LoadContainerState(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to load container state: %w", err)
	}

	if len(states) == 0 {
		logger.Info("CONTAINER-PERSIST: No containers to restore from state", nil)
		return 0, nil
	}

	logger.Info("CONTAINER-PERSIST: Starting container restoration", map[string]interface{}{
		"containers_in_state": len(states),
	})

	syncedCount := 0
	errorCount := 0

	// Group containers by node for efficient verification
	containersByNode := make(map[string][]PersistedContainerState)
	for _, container := range states {
		containersByNode[container.NodeID] = append(containersByNode[container.NodeID], container)
	}

	// Verify each node's containers still exist
	for nodeID, nodeContainers := range containersByNode {
		// Check if node still exists
		node, exists := c.NodeRegistry.GetNode(nodeID)
		if !exists {
			logger.Warn("CONTAINER-PERSIST: Node no longer exists, containers lost", map[string]interface{}{
				"node_id":    nodeID,
				"containers": len(nodeContainers),
			})
			errorCount += len(nodeContainers)

			// Mark servers as error state in DB
			c.markServersAsLost(nodeContainers, serverRepo, fmt.Sprintf("Node %s no longer exists", nodeID))
			continue
		}

		logger.Info("CONTAINER-PERSIST: Verifying containers on node", map[string]interface{}{
			"node_id":             nodeID,
			"containers_expected": len(nodeContainers),
		})

		// Sync containers on this node
		synced, lost := c.syncContainersOnNode(node, nodeContainers, serverRepo)
		syncedCount += synced
		errorCount += lost
	}

	logger.Info("CONTAINER-PERSIST: Container restoration completed", map[string]interface{}{
		"total":  len(states),
		"synced": syncedCount,
		"lost":   errorCount,
	})

	// Mark node sync as complete (ends grace period timer)
	c.NodeRegistry.mu.Lock()
	for nodeID := range containersByNode {
		if node, exists := c.NodeRegistry.nodes[nodeID]; exists {
			node.Metrics.ContainerSyncCompletedAt = timePtr(time.Now())
		}
	}
	c.NodeRegistry.mu.Unlock()

	return syncedCount, nil
}

// syncContainersOnNode verifies and restores containers on a specific node
func (c *Conductor) syncContainersOnNode(node *Node, expectedContainers []PersistedContainerState, serverRepo interface{}) (int, int) {
	synced := 0
	skipped := 0

	for _, container := range expectedContainers {
		// CRITICAL: Check database status before restoring
		// Persistence file might be outdated (e.g. "running" but DB says "sleeping")

		// Use reflection to call FindByID on serverRepo (interface{})
		serverRepoVal := reflect.ValueOf(serverRepo)

		// Debug: log type information
		logger.Info("CONTAINER-PERSIST: Reflection debug", map[string]interface{}{
			"server_repo_type": serverRepoVal.Type().String(),
			"server_repo_kind": serverRepoVal.Kind().String(),
			"num_methods":      serverRepoVal.NumMethod(),
		})

		findByIDMethod := serverRepoVal.MethodByName("FindByID")

		if !findByIDMethod.IsValid() {
			// List all available methods for debugging
			methodNames := []string{}
			for i := 0; i < serverRepoVal.NumMethod(); i++ {
				methodNames = append(methodNames, serverRepoVal.Type().Method(i).Name)
			}

			logger.Warn("CONTAINER-PERSIST: ServerRepo doesn't have FindByID method, restoring without DB check", map[string]interface{}{
				"available_methods": methodNames,
			})
			// Fallback: register container as-is
			c.ContainerRegistry.RegisterContainer(&ContainerInfo{
				ServerID:         container.ServerID,
				ServerName:       container.ServerName,
				ContainerID:      container.ContainerID,
				NodeID:           container.NodeID,
				RAMMb:            container.RAMMb,
				DockerPort:       container.Port,
				MinecraftPort:    container.MinecraftPort,
				MinecraftVersion: container.MinecraftVersion,
				ServerType:       container.ServerType,
				Status:           container.Status,
				LastSeenAt:       time.Now(),
			})
			synced++
			continue
		}

		// Call FindByID via reflection
		args := []reflect.Value{reflect.ValueOf(container.ServerID)}
		results := findByIDMethod.Call(args)

		// Check for error (second return value)
		if len(results) < 2 {
			logger.Warn("CONTAINER-PERSIST: Unexpected FindByID return values", map[string]interface{}{
				"server_id": container.ServerID,
			})
			skipped++
			continue
		}

		errVal := results[1]
		if !errVal.IsNil() {
			logger.Warn("CONTAINER-PERSIST: Server not found in DB, skipping", map[string]interface{}{
				"server_id": container.ServerID,
				"error":     errVal.Interface(),
			})
			skipped++
			continue
		}

		// Extract server object (first return value)
		serverVal := results[0]
		if serverVal.Kind() == reflect.Ptr {
			serverVal = serverVal.Elem()
		}

		// Extract Status field
		statusField := serverVal.FieldByName("Status")
		if !statusField.IsValid() || statusField.Kind() != reflect.String {
			logger.Warn("CONTAINER-PERSIST: Cannot extract Status field from server", map[string]interface{}{
				"server_id": container.ServerID,
			})
			skipped++
			continue
		}

		dbStatus := statusField.String()

		// ONLY restore containers that are actually running/starting in DB
		if dbStatus != "running" && dbStatus != "starting" && dbStatus != "provisioning" {
			logger.Info("CONTAINER-PERSIST: Skipping container with non-running DB status", map[string]interface{}{
				"server_id":   container.ServerID,
				"server_name": container.ServerName,
				"db_status":   dbStatus,
				"file_status": container.Status,
			})
			skipped++
			continue
		}

		// Register container with ACTUAL status from database (not from file!)
		c.ContainerRegistry.RegisterContainer(&ContainerInfo{
			ServerID:         container.ServerID,
			ServerName:       container.ServerName,
			ContainerID:      container.ContainerID,
			NodeID:           container.NodeID,
			RAMMb:            container.RAMMb,
			DockerPort:       container.Port,
			MinecraftPort:    container.MinecraftPort,
			MinecraftVersion: container.MinecraftVersion,
			ServerType:       container.ServerType,
			Status:           dbStatus, // Use DB status, NOT file status!
			LastSeenAt:       time.Now(),
		})

		logger.Info("CONTAINER-PERSIST: Container restored with DB status", map[string]interface{}{
			"server_id":   container.ServerID,
			"server_name": container.ServerName,
			"node_id":     container.NodeID,
			"db_status":   dbStatus,
			"file_status": container.Status,
		})

		synced++
	}

	logger.Info("CONTAINER-PERSIST: Sync summary", map[string]interface{}{
		"node_id":  node.ID,
		"synced":   synced,
		"skipped":  skipped,
	})

	return synced, skipped
}

// markServersAsLost marks servers in database as lost due to node failure
func (c *Conductor) markServersAsLost(containers []PersistedContainerState, serverRepo interface{}, reason string) {
	for _, container := range containers {
		logger.Error("CONTAINER-PERSIST: Container data lost", fmt.Errorf(reason), map[string]interface{}{
			"server_id":   container.ServerID,
			"server_name": container.ServerName,
			"node_id":     container.NodeID,
		})

		// TODO: Update server status to "error" in database with reason
		// This requires serverRepo.UpdateStatus() method
	}
}

// Helper function to create time pointer
func timePtr(t time.Time) *time.Time {
	return &t
}
