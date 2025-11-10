package conductor

import (
	"sync"
	"time"
)

// NodeRegistry manages the fleet of nodes
type NodeRegistry struct {
	nodes map[string]*Node
	mu    sync.RWMutex
}

// NewNodeRegistry creates a new node registry
func NewNodeRegistry() *NodeRegistry {
	return &NodeRegistry{
		nodes: make(map[string]*Node),
	}
}

// RegisterNode adds or updates a node in the registry
func (r *NodeRegistry) RegisterNode(node *Node) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if node.CreatedAt.IsZero() {
		node.CreatedAt = time.Now()
	}

	r.nodes[node.ID] = node
}

// GetNode retrieves a node by ID
func (r *NodeRegistry) GetNode(nodeID string) (*Node, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	node, exists := r.nodes[nodeID]
	return node, exists
}

// GetAllNodes returns all registered nodes
func (r *NodeRegistry) GetAllNodes() []*Node {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nodes := make([]*Node, 0, len(r.nodes))
	for _, node := range r.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// GetHealthyNodes returns only healthy nodes
func (r *NodeRegistry) GetHealthyNodes() []*Node {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nodes := make([]*Node, 0)
	for _, node := range r.nodes {
		if node.IsHealthy() {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// GetNodesByType returns all nodes of a specific type (dedicated, cloud, spare)
func (r *NodeRegistry) GetNodesByType(nodeType string) []*Node {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nodes := make([]*Node, 0)
	for _, node := range r.nodes {
		if node.Type == nodeType {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// UpdateNodeStatus updates the health status of a node
func (r *NodeRegistry) UpdateNodeStatus(nodeID string, status NodeStatus) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if node, exists := r.nodes[nodeID]; exists {
		node.Status = status
		node.LastHealthCheck = time.Now()
	}
}

// UpdateNodeResources updates the resource allocation for a node
func (r *NodeRegistry) UpdateNodeResources(nodeID string, containerCount int, allocatedRAMMB int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if node, exists := r.nodes[nodeID]; exists {
		node.ContainerCount = containerCount
		node.AllocatedRAMMB = allocatedRAMMB
	}
}

// RemoveNode removes a node from the registry
func (r *NodeRegistry) RemoveNode(nodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.nodes, nodeID)
}

// UnregisterNode is an alias for RemoveNode (used by VMProvisioner)
func (r *NodeRegistry) UnregisterNode(nodeID string) {
	r.RemoveNode(nodeID)
}

// GetFleetStats returns aggregate statistics for the entire fleet
func (r *NodeRegistry) GetFleetStats() FleetStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := FleetStats{}

	for _, node := range r.nodes {
		stats.TotalNodes++
		stats.TotalRAMMB += node.TotalRAMMB
		stats.SystemReservedRAMMB += node.SystemReservedRAMMB
		stats.TotalCPUCores += node.TotalCPUCores
		stats.AllocatedRAMMB += node.AllocatedRAMMB
		stats.TotalContainers += node.ContainerCount

		if node.IsHealthy() {
			stats.HealthyNodes++
		} else {
			stats.UnhealthyNodes++
		}

		if node.Type == "dedicated" {
			stats.DedicatedNodes++
		} else if node.Type == "cloud" {
			stats.CloudNodes++
		}
	}

	// Calculate usable RAM (Total - System Reserved)
	stats.UsableRAMMB = stats.TotalRAMMB - stats.SystemReservedRAMMB

	// Calculate available RAM (Usable - Allocated)
	stats.AvailableRAMMB = stats.UsableRAMMB - stats.AllocatedRAMMB
	if stats.AvailableRAMMB < 0 {
		stats.AvailableRAMMB = 0
	}

	// Calculate utilization based on USABLE RAM (not total)
	if stats.UsableRAMMB > 0 {
		stats.RAMUtilizationPercent = (float64(stats.AllocatedRAMMB) / float64(stats.UsableRAMMB)) * 100.0
	}

	return stats
}

// FleetStats contains aggregate statistics for the fleet
type FleetStats struct {
	TotalNodes            int     `json:"total_nodes"`
	HealthyNodes          int     `json:"healthy_nodes"`
	UnhealthyNodes        int     `json:"unhealthy_nodes"`
	DedicatedNodes        int     `json:"dedicated_nodes"`
	CloudNodes            int     `json:"cloud_nodes"`
	TotalRAMMB            int     `json:"total_ram_mb"`             // Total physical RAM across all nodes
	SystemReservedRAMMB   int     `json:"system_reserved_ram_mb"`   // RAM reserved for system processes
	UsableRAMMB           int     `json:"usable_ram_mb"`            // Total - SystemReserved (capacity for containers)
	AllocatedRAMMB        int     `json:"allocated_ram_mb"`         // RAM currently allocated to containers
	AvailableRAMMB        int     `json:"available_ram_mb"`         // Usable - Allocated (free for new containers)
	RAMUtilizationPercent float64 `json:"ram_utilization_percent"`  // Allocated / Usable * 100
	TotalCPUCores         int     `json:"total_cpu_cores"`
	TotalContainers       int     `json:"total_containers"`
}
