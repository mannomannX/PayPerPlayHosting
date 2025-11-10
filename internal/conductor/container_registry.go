package conductor

import (
	"sync"
	"time"
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
	containers map[string]*ContainerInfo // key: serverID
	mu         sync.RWMutex
}

// NewContainerRegistry creates a new container registry
func NewContainerRegistry() *ContainerRegistry {
	return &ContainerRegistry{
		containers: make(map[string]*ContainerInfo),
	}
}

// RegisterContainer adds or updates a container in the registry
func (r *ContainerRegistry) RegisterContainer(info *ContainerInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	info.LastSeenAt = time.Now()
	r.containers[info.ServerID] = info
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

	delete(r.containers, serverID)
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
