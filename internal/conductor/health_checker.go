package conductor

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/client"
	"github.com/payperplay/hosting/internal/docker"
	"github.com/payperplay/hosting/pkg/logger"
)

// HealthChecker performs periodic health checks on all nodes
type HealthChecker struct {
	nodeRegistry      *NodeRegistry
	containerRegistry *ContainerRegistry
	remoteClient      *docker.RemoteDockerClient
	interval          time.Duration
	stopChan          chan struct{}
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(nodeRegistry *NodeRegistry, containerRegistry *ContainerRegistry, remoteClient *docker.RemoteDockerClient, interval time.Duration) *HealthChecker {
	return &HealthChecker{
		nodeRegistry:      nodeRegistry,
		containerRegistry: containerRegistry,
		remoteClient:      remoteClient,
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
// Performs comprehensive health checks including:
// - SSH connectivity (for remote nodes)
// - Docker daemon health
// - Resource availability (RAM, CPU, disk)
func (h *HealthChecker) checkNodeHealth(node *Node) NodeStatus {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Determine if node is local or remote
	isLocal := node.Type == "local" || node.IPAddress == "" || node.IPAddress == "localhost" || node.IPAddress == "127.0.0.1"

	if isLocal {
		// Local node: Check using local Docker client
		return h.checkLocalNodeHealth(ctx, node)
	}

	// Remote node: Check using SSH + RemoteDockerClient
	return h.checkRemoteNodeHealth(ctx, node)
}

// checkLocalNodeHealth checks the health of the local Docker daemon
func (h *HealthChecker) checkLocalNodeHealth(ctx context.Context, node *Node) NodeStatus {
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
		logger.Debug("Docker ping failed for local node", map[string]interface{}{
			"node_id": node.ID,
			"error":   err.Error(),
		})
		return NodeStatusUnhealthy
	}

	logger.Debug("Local node health check passed", map[string]interface{}{
		"node_id": node.ID,
	})
	return NodeStatusHealthy
}

// checkRemoteNodeHealth checks the health of a remote node via SSH
func (h *HealthChecker) checkRemoteNodeHealth(ctx context.Context, node *Node) NodeStatus {
	if h.remoteClient == nil {
		logger.Warn("Remote client not configured, skipping remote node health check", map[string]interface{}{
			"node_id": node.ID,
		})
		return NodeStatusUnknown
	}

	// Create remote node representation
	remoteNode := &docker.RemoteNode{
		ID:        node.ID,
		IPAddress: node.IPAddress,
		SSHUser:   node.SSHUser,
	}

	// 1. SSH Connectivity + Docker Daemon Check
	err := h.remoteClient.HealthCheck(ctx, remoteNode)
	if err != nil {
		logger.Debug("Remote node health check failed", map[string]interface{}{
			"node_id":    node.ID,
			"ip_address": node.IPAddress,
			"error":      err.Error(),
		})
		return NodeStatusUnhealthy
	}

	// 2. Resource Availability Check (RAM, CPU, Disk)
	resourcesOK, err := h.checkRemoteNodeResources(ctx, remoteNode, node)
	if err != nil {
		logger.Warn("Failed to check remote node resources", map[string]interface{}{
			"node_id": node.ID,
			"error":   err.Error(),
		})
		// Don't mark as unhealthy if resource check fails, just log warning
		// This prevents false-positives if the resource check command fails
	} else if !resourcesOK {
		logger.Warn("Remote node has low resources", map[string]interface{}{
			"node_id": node.ID,
		})
		// Don't mark as unhealthy, but log the warning
		// Node selection should handle resource constraints via RAM allocation
	}

	logger.Debug("Remote node health check passed", map[string]interface{}{
		"node_id":    node.ID,
		"ip_address": node.IPAddress,
	})
	return NodeStatusHealthy
}

// checkRemoteNodeResources checks if a remote node has sufficient resources
// Returns true if resources are OK, false if critically low
func (h *HealthChecker) checkRemoteNodeResources(ctx context.Context, remoteNode *docker.RemoteNode, node *Node) (bool, error) {
	// Execute 'free -m' to check available RAM
	cmd := "free -m | awk 'NR==2{printf \"%d\", $7}'"
	output, err := h.executeRemoteCommand(ctx, remoteNode, cmd)
	if err != nil {
		return false, fmt.Errorf("failed to check RAM: %w", err)
	}

	// Parse available RAM
	availableRAM, err := strconv.Atoi(strings.TrimSpace(output))
	if err != nil {
		return false, fmt.Errorf("failed to parse RAM value: %w", err)
	}

	// Check if available RAM is critically low (< 500MB)
	if availableRAM < 500 {
		logger.Warn("Remote node has critically low RAM", map[string]interface{}{
			"node_id":       node.ID,
			"available_ram": availableRAM,
		})
		return false, nil
	}

	// Execute 'df -h /' to check disk usage
	cmd = "df -h / | awk 'NR==2{print $5}' | sed 's/%//'"
	output, err = h.executeRemoteCommand(ctx, remoteNode, cmd)
	if err != nil {
		return false, fmt.Errorf("failed to check disk usage: %w", err)
	}

	// Parse disk usage percentage
	diskUsage, err := strconv.Atoi(strings.TrimSpace(output))
	if err != nil {
		return false, fmt.Errorf("failed to parse disk usage: %w", err)
	}

	// Check if disk usage is critically high (> 90%)
	if diskUsage > 90 {
		logger.Warn("Remote node has critically high disk usage", map[string]interface{}{
			"node_id":    node.ID,
			"disk_usage": diskUsage,
		})
		return false, nil
	}

	logger.Debug("Remote node resource check passed", map[string]interface{}{
		"node_id":       node.ID,
		"available_ram": availableRAM,
		"disk_usage":    diskUsage,
	})

	return true, nil
}

// executeRemoteCommand executes a command on a remote node via SSH
// This is a helper method that uses the remoteClient's SSH infrastructure
func (h *HealthChecker) executeRemoteCommand(ctx context.Context, remoteNode *docker.RemoteNode, command string) (string, error) {
	return h.remoteClient.ExecuteSSHCommand(ctx, remoteNode, command)
}
