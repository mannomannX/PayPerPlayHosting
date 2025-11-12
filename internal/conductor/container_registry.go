package conductor

import (
	"fmt"
	"sync"
	"time"

	"github.com/payperplay/hosting/pkg/logger"
)

// ContainerInfo represents a Minecraft server container running on a node
type ContainerInfo struct {
	ServerID      string    `json:"server_id"`
	ServerName    string    `json:"server_name"`
	ContainerID   string    `json:"container_id"`
	NodeID        string    `json:"node_id"`
	RAMMb         int       `json:"ram_mb"`
	Status        string    `json:"status"`
	LastSeenAt    time.Time `json:"last_seen_at"`
	DockerPort    int       `json:"docker_port"`
	MinecraftPort int       `json:"minecraft_port"`
}

// ContainerRegistry tracks which containers are running on which nodes
type ContainerRegistry struct {
	containers   map[string]*ContainerInfo // key: serverID
	mu           sync.RWMutex
	nodeRegistry *NodeRegistry // For updating node lifecycle timestamps
}

// NewContainerRegistry creates a new container registry
func NewContainerRegistry() *ContainerRegistry {
	return &ContainerRegistry{
		containers: make(map[string]*ContainerInfo),
	}
}

// SetNodeRegistry injects the NodeRegistry for lifecycle tracking
func (r *ContainerRegistry) SetNodeRegistry(nodeRegistry *NodeRegistry) {
	r.nodeRegistry = nodeRegistry
}

// RegisterContainer adds or updates a container in the registry
func (r *ContainerRegistry) RegisterContainer(info *ContainerInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	info.LastSeenAt = time.Now()

	// Check if this is a NEW container (not just an update)
	_, existingContainer := r.containers[info.ServerID]

	r.containers[info.ServerID] = info

	// Track container lifecycle on node
	if !existingContainer && r.nodeRegistry != nil {
		// New container added - update node's LastContainerAdded timestamp
		if node, exists := r.nodeRegistry.GetNode(info.NodeID); exists {
			node.LastContainerAdded = time.Now()
			logger.Debug("Node container added timestamp updated", map[string]interface{}{
				"node_id":   info.NodeID,
				"server_id": info.ServerID,
			})
		}
	}
}

// GetContainer retrieves a container by server ID
func (r *ContainerRegistry) GetContainer(serverID string) (*ContainerInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	container, exists := r.containers[serverID]
	return container, exists
}

// GetAllContainers returns all registered containers
func (r *ContainerRegistry) GetAllContainers() []*ContainerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	containers := make([]*ContainerInfo, 0, len(r.containers))
	for _, container := range r.containers {
		containers = append(containers, container)
	}
	return containers
}

// GetContainersByNode returns all containers running on a specific node
func (r *ContainerRegistry) GetContainersByNode(nodeID string) []*ContainerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	containers := make([]*ContainerInfo, 0)
	for _, container := range r.containers {
		if container.NodeID == nodeID {
			containers = append(containers, container)
		}
	}
	return containers
}

// RemoveContainer removes a container from the registry
func (r *ContainerRegistry) RemoveContainer(serverID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Get container info before deleting (for node lifecycle tracking)
	container, exists := r.containers[serverID]
	if !exists {
		return // Container doesn't exist, nothing to remove
	}

	nodeID := container.NodeID
	delete(r.containers, serverID)

	// Track container lifecycle on node
	if r.nodeRegistry != nil {
		// Check if node is now empty after removing this container
		remainingOnNode := 0
		for _, c := range r.containers {
			if c.NodeID == nodeID {
				remainingOnNode++
			}
		}

		// If node is now empty, update LastContainerRemoved timestamp
		if remainingOnNode == 0 {
			if node, exists := r.nodeRegistry.GetNode(nodeID); exists {
				node.LastContainerRemoved = time.Now()
				logger.Info("Node is now empty - idle tracking started", map[string]interface{}{
					"node_id":   nodeID,
					"server_id": serverID,
				})
			}
		}
	}
}

// RemoveContainersByNode removes all containers from a specific node
func (r *ContainerRegistry) RemoveContainersByNode(nodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for serverID, container := range r.containers {
		if container.NodeID == nodeID {
			delete(r.containers, serverID)
		}
	}
}

// GetNodeAllocation returns the total RAM allocated on a specific node
func (r *ContainerRegistry) GetNodeAllocation(nodeID string) (containerCount int, allocatedRAMMB int) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, container := range r.containers {
		if container.NodeID == nodeID {
			containerCount++
			allocatedRAMMB += container.RAMMb
		}
	}
	return
}

// GetStartingCount returns the number of containers currently in "starting" status
// CPU-GUARD: Used to limit parallel server starts to prevent CPU overload
func (r *ContainerRegistry) GetStartingCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, container := range r.containers {
		if container.Status == "starting" {
			count++
		}
	}
	return count
}

// GetStartingCountOnNode returns the number of containers in "starting" status on a specific node
// Multi-Node CPU-GUARD: Limits parallel starts per node
func (r *ContainerRegistry) GetStartingCountOnNode(nodeID string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, container := range r.containers {
		if container.NodeID == nodeID && container.Status == "starting" {
			count++
		}
	}
	return count
}

// GetContainersByStatus returns all containers with a specific status
func (r *ContainerRegistry) GetContainersByStatus(status string) []*ContainerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	containers := make([]*ContainerInfo, 0)
	for _, container := range r.containers {
		if container.Status == status {
			containers = append(containers, container)
		}
	}
	return containers
}

// GetNodeStats returns aggregated statistics for a specific node
func (r *ContainerRegistry) GetNodeStats(nodeID string) NodeStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := NodeStats{
		NodeID: nodeID,
	}

	for _, container := range r.containers {
		if container.NodeID == nodeID {
			stats.TotalContainers++
			stats.TotalRAMMB += container.RAMMb

			switch container.Status {
			case "running":
				stats.RunningContainers++
			case "starting":
				stats.StartingContainers++
			case "stopped":
				stats.StoppedContainers++
			}
		}
	}

	return stats
}

// GetAllNodeStats returns aggregated statistics for all nodes
func (r *ContainerRegistry) GetAllNodeStats() map[string]NodeStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	statsMap := make(map[string]NodeStats)

	for _, container := range r.containers {
		stats, exists := statsMap[container.NodeID]
		if !exists {
			stats = NodeStats{
				NodeID: container.NodeID,
			}
		}

		stats.TotalContainers++
		stats.TotalRAMMB += container.RAMMb

		switch container.Status {
		case "running":
			stats.RunningContainers++
		case "starting":
			stats.StartingContainers++
		case "stopped":
			stats.StoppedContainers++
		}

		statsMap[container.NodeID] = stats
	}

	return statsMap
}

// NodeStats contains aggregated statistics for a single node
type NodeStats struct {
	NodeID              string `json:"node_id"`
	TotalContainers     int    `json:"total_containers"`
	RunningContainers   int    `json:"running_containers"`
	StartingContainers  int    `json:"starting_containers"`
	StoppedContainers   int    `json:"stopped_containers"`
	TotalRAMMB          int    `json:"total_ram_mb"`
}

// UpdateContainerNode moves a container to a different node (for migration scenarios)
// This is useful for live migration or failover scenarios
func (r *ContainerRegistry) UpdateContainerNode(serverID, newNodeID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	container, exists := r.containers[serverID]
	if !exists {
		return fmt.Errorf("container not found: %s", serverID)
	}

	oldNodeID := container.NodeID
	container.NodeID = newNodeID
	container.LastSeenAt = time.Now()

	logger.Info("Container moved to new node", map[string]interface{}{
		"server_id":   serverID,
		"old_node_id": oldNodeID,
		"new_node_id": newNodeID,
	})

	return nil
}

// GetStaleContainers returns containers that haven't been seen for a specified duration
// Useful for detecting orphaned containers or node failures
func (r *ContainerRegistry) GetStaleContainers(staleDuration time.Duration) []*ContainerInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stale := make([]*ContainerInfo, 0)
	now := time.Now()

	for _, container := range r.containers {
		if now.Sub(container.LastSeenAt) > staleDuration {
			stale = append(stale, container)
		}
	}

	return stale
}

// SyncNodeContainers synchronizes the registry with actual containers running on a node
// This prevents ghost containers by removing entries that no longer exist in Docker
// actualContainerIDs: List of container IDs actually running on the node (from docker ps -a)
func (r *ContainerRegistry) SyncNodeContainers(nodeID string, actualContainerIDs map[string]bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	removed := 0
	for serverID, container := range r.containers {
		if container.NodeID != nodeID {
			continue // Not on this node
		}

		// Check if container still exists in Docker
		if !actualContainerIDs[container.ContainerID] {
			// Container no longer exists, remove from registry
			delete(r.containers, serverID)
			removed++

			logger.Info("Ghost container removed from registry", map[string]interface{}{
				"server_id":    serverID,
				"container_id": container.ContainerID,
				"node_id":      nodeID,
			})
		}
	}

	if removed > 0 {
		logger.Info("Container sync completed", map[string]interface{}{
			"node_id":         nodeID,
			"removed_ghosts":  removed,
			"active_on_node":  len(actualContainerIDs),
			"registry_total":  len(r.containers),
		})

		// Check if node is now empty after sync
		if r.nodeRegistry != nil {
			remainingOnNode := 0
			for _, c := range r.containers {
				if c.NodeID == nodeID {
					remainingOnNode++
				}
			}

			// If node is now empty, update LastContainerRemoved timestamp
			if remainingOnNode == 0 {
				if node, exists := r.nodeRegistry.GetNode(nodeID); exists {
					node.LastContainerRemoved = time.Now()
					logger.Info("Node is now empty after sync - idle tracking started", map[string]interface{}{
						"node_id": nodeID,
					})
				}
			}
		}
	}
}
