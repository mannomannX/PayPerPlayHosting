package conductor

import (
	"context"
	"time"

	"github.com/docker/docker/client"
	"github.com/payperplay/hosting/pkg/logger"
)

// HealthChecker performs periodic health checks on all nodes
type HealthChecker struct {
	nodeRegistry      *NodeRegistry
	containerRegistry *ContainerRegistry
	interval          time.Duration
	stopChan          chan struct{}
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(nodeRegistry *NodeRegistry, containerRegistry *ContainerRegistry, interval time.Duration) *HealthChecker {
	return &HealthChecker{
		nodeRegistry:      nodeRegistry,
		containerRegistry: containerRegistry,
		interval:          interval,
		stopChan:          make(chan struct{}),
	}
}

// Start begins the health check loop
func (h *HealthChecker) Start() {
	ticker := time.NewTicker(h.interval)
	go func() {
		// Perform initial health check immediately
		h.performHealthCheck()

		for {
			select {
			case <-ticker.C:
				h.performHealthCheck()
			case <-h.stopChan:
				ticker.Stop()
				return
			}
		}
	}()

	logger.Info("Health checker started", map[string]interface{}{
		"interval": h.interval.String(),
	})
}

// Stop stops the health check loop
func (h *HealthChecker) Stop() {
	close(h.stopChan)
}

// performHealthCheck checks the health of all registered nodes
func (h *HealthChecker) performHealthCheck() {
	nodes := h.nodeRegistry.GetAllNodes()

	for _, node := range nodes {
		status := h.checkNodeHealth(node)
		h.nodeRegistry.UpdateNodeStatus(node.ID, status)

		if status == NodeStatusHealthy {
			// Update resource allocation from container registry
			containerCount, allocatedRAMMB := h.containerRegistry.GetNodeAllocation(node.ID)
			h.nodeRegistry.UpdateNodeResources(node.ID, containerCount, allocatedRAMMB)
		}

		logger.Debug("Node health check completed", map[string]interface{}{
			"node_id":    node.ID,
			"hostname":   node.Hostname,
			"status":     status,
			"containers": node.ContainerCount,
			"ram_mb":     node.AllocatedRAMMB,
		})
	}

	// Log fleet statistics
	stats := h.nodeRegistry.GetFleetStats()
	logger.Debug("Fleet health check completed", map[string]interface{}{
		"total_nodes":     stats.TotalNodes,
		"healthy_nodes":   stats.HealthyNodes,
		"unhealthy_nodes": stats.UnhealthyNodes,
		"total_ram_mb":    stats.TotalRAMMB,
		"allocated_ram":   stats.AllocatedRAMMB,
		"utilization":     stats.RAMUtilizationPercent,
	})
}

// checkNodeHealth checks if a node is healthy by attempting to connect to Docker
func (h *HealthChecker) checkNodeHealth(node *Node) NodeStatus {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// For now, we check the local Docker daemon
	// In production, this would SSH to remote nodes or use Docker TCP sockets
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		logger.Debug("Failed to create Docker client for health check", map[string]interface{}{
			"node_id": node.ID,
			"error":   err.Error(),
		})
		return NodeStatusUnhealthy
	}
	defer dockerClient.Close()

	// Ping Docker daemon
	_, err = dockerClient.Ping(ctx)
	if err != nil {
		logger.Debug("Docker ping failed for node", map[string]interface{}{
			"node_id": node.ID,
			"error":   err.Error(),
		})
		return NodeStatusUnhealthy
	}

	return NodeStatusHealthy
}
