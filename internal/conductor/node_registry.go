package conductor

import (
	"sync"
	"time"

	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/logger"
)

// NodeRegistry manages the fleet of nodes
type NodeRegistry struct {
	nodes    map[string]*Node
	mu       sync.RWMutex
	nodeRepo *repository.NodeRepository
}

// NewNodeRegistry creates a new node registry
func NewNodeRegistry(nodeRepo *repository.NodeRepository) *NodeRegistry {
	return &NodeRegistry{
		nodes:    make(map[string]*Node),
		nodeRepo: nodeRepo,
	}
}

// RegisterNode adds or updates a node in the registry
func (r *NodeRegistry) RegisterNode(node *Node) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Auto-detect if this is a system node (API/Proxy nodes cannot run MC containers)
	node.IsSystemNode = isSystemNodeByID(node.ID)

	if node.CreatedAt.IsZero() {
		node.CreatedAt = time.Now()
	}

	r.nodes[node.ID] = node

	// Persist to database if repository is available
	if r.nodeRepo != nil {
		dbNode := r.nodeToDBModel(node)

		// Check if node exists in database
		exists, err := r.nodeRepo.Exists(node.ID)
		if err != nil {
			logger.Warn("NODE-REGISTRY: Failed to check if node exists in database", map[string]interface{}{
				"node_id": node.ID,
				"error":   err.Error(),
			})
			return
		}

		if exists {
			// Update existing node
			if err := r.nodeRepo.Update(dbNode); err != nil {
				logger.Warn("NODE-REGISTRY: Failed to update node in database", map[string]interface{}{
					"node_id": node.ID,
					"error":   err.Error(),
				})
			}
		} else {
			// Create new node
			if err := r.nodeRepo.Create(dbNode); err != nil {
				logger.Warn("NODE-REGISTRY: Failed to persist node to database", map[string]interface{}{
					"node_id": node.ID,
					"error":   err.Error(),
				})
			} else {
				logger.Info("NODE-REGISTRY: Node persisted to database", map[string]interface{}{
					"node_id": node.ID,
					"type":    node.Type,
				})
			}
		}
	}
}

// isSystemNodeByID checks if a node ID represents a system node (Control Plane or Proxy)
// System nodes are reserved for infrastructure and should not run Minecraft servers
func isSystemNodeByID(nodeID string) bool {
	return nodeID == "local-node" ||
		nodeID == "control-plane" ||
		nodeID == "proxy-node" ||
		(len(nodeID) >= 5 && nodeID[:5] == "local") ||
		(len(nodeID) >= 7 && nodeID[:7] == "control") ||
		(len(nodeID) >= 5 && nodeID[:5] == "proxy")
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

		// Persist to database if repository is available
		if r.nodeRepo != nil {
			if err := r.nodeRepo.UpdateStatus(nodeID, string(status)); err != nil {
				logger.Warn("NODE-REGISTRY: Failed to update node status in database", map[string]interface{}{
					"node_id": nodeID,
					"error":   err.Error(),
				})
			}
		}
	}
}

// UpdateNodeResources updates the resource allocation for a node
func (r *NodeRegistry) UpdateNodeResources(nodeID string, containerCount int, allocatedRAMMB int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if node, exists := r.nodes[nodeID]; exists {
		node.ContainerCount = containerCount
		node.AllocatedRAMMB = allocatedRAMMB

		// Persist to database if repository is available
		if r.nodeRepo != nil {
			if err := r.nodeRepo.UpdateResources(nodeID, containerCount, allocatedRAMMB); err != nil {
				logger.Warn("NODE-REGISTRY: Failed to update node resources in database", map[string]interface{}{
					"node_id": nodeID,
					"error":   err.Error(),
				})
			}
		}
	}
}

// UpdateNodeCPU updates the CPU usage for a node
func (r *NodeRegistry) UpdateNodeCPU(nodeID string, cpuUsagePercent float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if node, exists := r.nodes[nodeID]; exists {
		node.CPUUsagePercent = cpuUsagePercent
	}
}

// RemoveNode removes a node from the registry
func (r *NodeRegistry) RemoveNode(nodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.nodes, nodeID)

	// Remove from database if repository is available
	if r.nodeRepo != nil {
		if err := r.nodeRepo.Delete(nodeID); err != nil {
			logger.Warn("NODE-REGISTRY: Failed to delete node from database", map[string]interface{}{
				"node_id": nodeID,
				"error":   err.Error(),
			})
		} else {
			logger.Info("NODE-REGISTRY: Node deleted from database", map[string]interface{}{
				"node_id": nodeID,
			})
		}
	}
}

// UnregisterNode is an alias for RemoveNode (used by VMProvisioner)
func (r *NodeRegistry) UnregisterNode(nodeID string) {
	r.RemoveNode(nodeID)
}

// AtomicAllocateRAM atomically reserves RAM on the local node
// Returns true if allocation succeeded, false if insufficient capacity
// THIS IS CRITICAL FOR PREVENTING RACE CONDITIONS!
// DEPRECATED: Use AtomicAllocateRAMOnNode() for multi-node support
func (r *NodeRegistry) AtomicAllocateRAM(ramMB int) bool {
	return r.AtomicAllocateRAMOnNode("local-node", ramMB)
}

// AtomicAllocateRAMOnNode atomically reserves RAM on a specific node
// Returns true if allocation succeeded, false if insufficient capacity
// This is the Multi-Node version that supports any node ID
func (r *NodeRegistry) AtomicAllocateRAMOnNode(nodeID string, ramMB int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Find the specified node
	node, exists := r.nodes[nodeID]
	if !exists {
		logger.Error("AtomicAllocateRAMOnNode: node not found", nil, map[string]interface{}{
			"node_id": nodeID,
		})
		return false
	}

	// Check if we have capacity (accounting for system reserve)
	usableRAM := node.UsableRAMMB()
	availableRAM := usableRAM - node.AllocatedRAMMB

	logger.Debug("AtomicAllocateRAMOnNode", map[string]interface{}{
		"node_id":           nodeID,
		"requested_ram_mb":  ramMB,
		"total_ram_mb":      node.TotalRAMMB,
		"system_reserve_mb": node.SystemReservedRAMMB,
		"usable_ram_mb":     usableRAM,
		"allocated_ram_mb":  node.AllocatedRAMMB,
		"available_ram_mb":  availableRAM,
		"container_count":   node.ContainerCount,
	})

	if availableRAM < ramMB {
		// Insufficient capacity
		logger.Info("AtomicAllocateRAMOnNode: Insufficient capacity", map[string]interface{}{
			"node_id":          nodeID,
			"requested_ram_mb": ramMB,
			"available_ram_mb": availableRAM,
			"result":           "REJECTED",
		})
		return false
	}

	// Atomically allocate the RAM
	node.AllocatedRAMMB += ramMB
	node.ContainerCount++

	logger.Info("AtomicAllocateRAMOnNode: Success", map[string]interface{}{
		"node_id":               nodeID,
		"requested_ram_mb":      ramMB,
		"new_allocated_ram_mb":  node.AllocatedRAMMB,
		"new_available_ram_mb":  usableRAM - node.AllocatedRAMMB,
		"new_container_count":   node.ContainerCount,
		"result":                "ALLOCATED",
	})

	return true
}

// ReleaseRAM atomically releases RAM from the local node
// DEPRECATED: Use ReleaseRAMOnNode() for multi-node support
func (r *NodeRegistry) ReleaseRAM(ramMB int) {
	r.ReleaseRAMOnNode("local-node", ramMB)
}

// ReleaseRAMOnNode atomically releases RAM from a specific node
// This is the Multi-Node version that supports any node ID
func (r *NodeRegistry) ReleaseRAMOnNode(nodeID string, ramMB int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	node, exists := r.nodes[nodeID]
	if !exists {
		logger.Warn("ReleaseRAMOnNode: node not found", map[string]interface{}{
			"node_id": nodeID,
		})
		return
	}

	// Release the RAM
	node.AllocatedRAMMB -= ramMB
	if node.AllocatedRAMMB < 0 {
		logger.Warn("ReleaseRAMOnNode: AllocatedRAMMB went negative, resetting to 0", map[string]interface{}{
			"node_id":           nodeID,
			"allocated_ram_mb":  node.AllocatedRAMMB,
		})
		node.AllocatedRAMMB = 0 // Safety check
	}

	node.ContainerCount--
	if node.ContainerCount < 0 {
		logger.Warn("ReleaseRAMOnNode: ContainerCount went negative, resetting to 0", map[string]interface{}{
			"node_id":         nodeID,
			"container_count": node.ContainerCount,
		})
		node.ContainerCount = 0 // Safety check
	}

	logger.Info("ReleaseRAMOnNode: RAM released", map[string]interface{}{
		"node_id":               nodeID,
		"released_ram_mb":       ramMB,
		"new_allocated_ram_mb":  node.AllocatedRAMMB,
		"new_available_ram_mb":  node.UsableRAMMB() - node.AllocatedRAMMB,
		"new_container_count":   node.ContainerCount,
	})
}

// GetFleetStats returns aggregate statistics for the entire fleet
func (r *NodeRegistry) GetFleetStats() FleetStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := FleetStats{}

	for _, node := range r.nodes {
		// CRITICAL FIX: Skip system nodes (local-node, proxy-node) for capacity calculations!
		// System nodes don't host Minecraft containers, so they shouldn't be counted in fleet capacity.
		// This was causing the Scaling Engine to think there was plenty of capacity when worker nodes were full.
		// Example: Worker node 8192 MB full + System nodes 7628 MB empty = 51% capacity (WRONG!)
		// Should be: Worker node 8192/8192 MB = 100% capacity → triggers scale-up
		if node.IsSystemNode {
			// Still count system nodes in total node count, but skip capacity calculations
			stats.TotalNodes++
			if node.Type == "dedicated" {
				stats.DedicatedNodes++
			}
			if node.IsHealthy() {
				stats.HealthyNodes++
			} else {
				stats.UnhealthyNodes++
			}
			continue // Skip capacity calculations for system nodes
		}

		// Worker nodes (non-system): count everything for capacity planning
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

	// PROPORTIONAL OVERHEAD SYSTEM: Calculate based on TOTAL RAM (not UsableRAM!)
	// UsableRAMMB is kept for backwards compatibility but no longer used for capacity decisions
	stats.UsableRAMMB = stats.TotalRAMMB - stats.SystemReservedRAMMB

	// Calculate available RAM based on TOTAL RAM (proportional overhead system)
	// Allocation uses BOOKED RAM (8GB = 8192MB), but containers get ActualRAM (less)
	stats.AvailableRAMMB = stats.TotalRAMMB - stats.AllocatedRAMMB
	if stats.AvailableRAMMB < 0 {
		stats.AvailableRAMMB = 0
	}

	// Calculate utilization based on TOTAL RAM (not UsableRAM!)
	// This is the CORRECT way with proportional overhead
	if stats.TotalRAMMB > 0 {
		stats.RAMUtilizationPercent = (float64(stats.AllocatedRAMMB) / float64(stats.TotalRAMMB)) * 100.0
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

// LoadNodesFromDB loads all nodes from the database into the in-memory registry
// This is called on conductor startup to restore node state after a restart
func (r *NodeRegistry) LoadNodesFromDB() error {
	if r.nodeRepo == nil {
		logger.Warn("NODE-REGISTRY: Cannot load nodes from database, repository not available", nil)
		return nil
	}

	dbNodes, err := r.nodeRepo.FindAll()
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	loadedCount := 0
	for _, dbNode := range dbNodes {
		node := r.dbModelToNode(dbNode)
		r.nodes[node.ID] = node
		loadedCount++
	}

	logger.Info("NODE-REGISTRY: Nodes loaded from database", map[string]interface{}{
		"count": loadedCount,
	})

	return nil
}

// nodeToDBModel converts a conductor.Node to a models.Node for database persistence
func (r *NodeRegistry) nodeToDBModel(node *Node) *models.Node {
	statusStr := string(node.Status)
	if statusStr == "" {
		statusStr = "unknown"
	}

	return &models.Node{
		ID:                   node.ID,
		Hostname:             node.Hostname,
		IPAddress:            node.IPAddress,
		Type:                 node.Type,
		IsSystemNode:         node.IsSystemNode,
		TotalRAMMB:           node.TotalRAMMB,
		TotalCPUCores:        node.TotalCPUCores,
		Status:               statusStr,
		LifecycleState:       string(node.LifecycleState),
		LastHealthCheck:      node.LastHealthCheck,
		ContainerCount:       node.ContainerCount,
		AllocatedRAMMB:       node.AllocatedRAMMB,
		SystemReservedRAMMB:  node.SystemReservedRAMMB,
		DockerSocketPath:     node.DockerSocketPath,
		SSHUser:              node.SSHUser,
		SSHPort:              node.SSHPort,
		SSHKeyPath:           node.SSHKeyPath,
		CreatedAt:            node.CreatedAt,
		UpdatedAt:            time.Now(),
		LastContainerAdded:   node.LastContainerAdded,
		LastContainerRemoved: node.LastContainerRemoved,
		HourlyCostEUR:        node.HourlyCostEUR,
		CloudProviderID:      node.CloudProviderID,
		CPUUsagePercent:      node.CPUUsagePercent,
	}
}

// dbModelToNode converts a models.Node to a conductor.Node for in-memory use
func (r *NodeRegistry) dbModelToNode(dbNode *models.Node) *Node {
	return &Node{
		ID:                   dbNode.ID,
		Hostname:             dbNode.Hostname,
		IPAddress:            dbNode.IPAddress,
		Type:                 dbNode.Type,
		IsSystemNode:         dbNode.IsSystemNode,
		TotalRAMMB:           dbNode.TotalRAMMB,
		TotalCPUCores:        dbNode.TotalCPUCores,
		CPUUsagePercent:      dbNode.CPUUsagePercent,
		Status:               NodeStatus(dbNode.Status),
		LifecycleState:       NodeLifecycleState(dbNode.LifecycleState),
		HealthStatus:         HealthStatus(dbNode.Status), // Map status to health status
		Metrics:              NodeLifecycleMetrics{},      // Initialize empty metrics
		LastHealthCheck:      dbNode.LastHealthCheck,
		ContainerCount:       dbNode.ContainerCount,
		AllocatedRAMMB:       dbNode.AllocatedRAMMB,
		SystemReservedRAMMB:  dbNode.SystemReservedRAMMB,
		DockerSocketPath:     dbNode.DockerSocketPath,
		SSHUser:              dbNode.SSHUser,
		SSHPort:              dbNode.SSHPort,
		SSHKeyPath:           dbNode.SSHKeyPath,
		CreatedAt:            dbNode.CreatedAt,
		LastContainerAdded:   dbNode.LastContainerAdded,
		LastContainerRemoved: dbNode.LastContainerRemoved,
		Labels:               make(map[string]string),
		HourlyCostEUR:        dbNode.HourlyCostEUR,
		CloudProviderID:      dbNode.CloudProviderID,
	}
}
