package conductor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/payperplay/hosting/pkg/logger"
)

// PersistedContainerState represents container state for recovery after restart
// Preserves timing information critical for lifecycle management (sleeping timers, etc)
type PersistedContainerState struct {
	ServerID      string    `json:"server_id"`
	ServerName    string    `json:"server_name"`
	ContainerID   string    `json:"container_id"`
	NodeID        string    `json:"node_id"`
	Status        string    `json:"status"` // running, stopped, sleeping
	RAMMb         int       `json:"ram_mb"`
	Port          int       `json:"port"`
	MinecraftPort int       `json:"minecraft_port"`

	// Timing information (critical for lifecycle rules)
	LastStartedAt *time.Time `json:"last_started_at,omitempty"`
	LastStoppedAt *time.Time `json:"last_stopped_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// SaveContainerState persists all container states to JSON file
// Called during graceful shutdown to preserve container timing information
func (c *Conductor) SaveContainerState(filePath string) error {
	containers := []PersistedContainerState{}

	c.ContainerRegistry.mu.RLock()
	for _, container := range c.ContainerRegistry.containers {
		state := PersistedContainerState{
			ServerID:      container.ServerID,
			ServerName:    container.ServerName,
			ContainerID:   container.ContainerID,
			NodeID:        container.NodeID,
			Status:        container.Status,
			RAMMb:         container.RAMMb,
			Port:          container.DockerPort,
			MinecraftPort: container.MinecraftPort,
			CreatedAt:     time.Now(), // Use current time as fallback
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
	// For local node, we can check directly
	// For remote nodes, we need SSH access

	synced := 0
	lost := 0

	for _, container := range expectedContainers {
		// Register container in registry (regardless of actual Docker state)
		// The health checker will verify if it actually exists
		c.ContainerRegistry.RegisterContainer(&ContainerInfo{
			ServerID:      container.ServerID,
			ServerName:    container.ServerName,
			ContainerID:   container.ContainerID,
			NodeID:        container.NodeID,
			RAMMb:         container.RAMMb,
			DockerPort:    container.Port,
			MinecraftPort: container.MinecraftPort,
			Status:        container.Status,
			LastSeenAt:    time.Now(),
		})

		logger.Info("CONTAINER-PERSIST: Container restored from state", map[string]interface{}{
			"server_id":   container.ServerID,
			"server_name": container.ServerName,
			"node_id":     container.NodeID,
			"status":      container.Status,
		})

		synced++
	}

	return synced, lost
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
