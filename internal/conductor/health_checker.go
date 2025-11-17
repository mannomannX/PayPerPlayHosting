package conductor

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/payperplay/hosting/internal/docker"
	"github.com/payperplay/hosting/pkg/logger"
)

// HealthChecker performs periodic health checks on all nodes
type HealthChecker struct {
	nodeRegistry      *NodeRegistry
	containerRegistry *ContainerRegistry
	remoteClient      *docker.RemoteDockerClient
	debugLogBuffer    *DebugLogBuffer
	interval          time.Duration
	stopChan          chan struct{}

	// FIX BILLING-2: Track failed Minecraft health checks for auto-recovery
	crashCounters     map[string]int  // serverID -> consecutive failed checks
	crashTimestamps   map[string]time.Time // serverID -> first failure time
	minecraftService  MinecraftServiceInterface // For stopping crashed servers
}

// MinecraftServiceInterface defines methods needed from MinecraftService
// Used to avoid circular dependency
type MinecraftServiceInterface interface {
	StopServer(serverID string, reason string) error
	// GAP-1: Handle containers on failed nodes
	HandleNodeFailure(serverID string) error
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(nodeRegistry *NodeRegistry, containerRegistry *ContainerRegistry, remoteClient *docker.RemoteDockerClient, debugLogBuffer *DebugLogBuffer, interval time.Duration) *HealthChecker {
	return &HealthChecker{
		nodeRegistry:      nodeRegistry,
		containerRegistry: containerRegistry,
		remoteClient:      remoteClient,
		debugLogBuffer:    debugLogBuffer,
		interval:          interval,
		stopChan:          make(chan struct{}),
		crashCounters:     make(map[string]int),
		crashTimestamps:   make(map[string]time.Time),
	}
}

// SetMinecraftService sets the Minecraft service for auto-recovery
// Called after initialization to avoid circular dependency
func (h *HealthChecker) SetMinecraftService(service MinecraftServiceInterface) {
	h.minecraftService = service
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
		oldStatus := node.Status
		status := h.checkNodeHealth(node)
		h.nodeRegistry.UpdateNodeStatus(node.ID, status)

		// LOG STATUS CHANGES (not just debug!)
		if oldStatus != status {
			if status == NodeStatusUnhealthy {
				fields := map[string]interface{}{
					"node_id":     node.ID,
					"hostname":    node.Hostname,
					"ip":          node.IPAddress,
					"old_status":  oldStatus,
					"new_status":  status,
					"type":        node.Type,
				}
				logger.Warn("Node became UNHEALTHY", fields)

				// Add to debug log buffer for dashboard
				if h.debugLogBuffer != nil {
					h.debugLogBuffer.Add("WARN", fmt.Sprintf("Node %s became UNHEALTHY (%s)", node.Hostname, node.IPAddress), fields)
				}

				// GAP-1: Handle node failure - cleanup containers, close billing, update status
				h.handleNodeFailure(node)
			} else {
				fields := map[string]interface{}{
					"node_id":     node.ID,
					"hostname":    node.Hostname,
					"old_status":  oldStatus,
					"new_status":  status,
				}
				logger.Info("Node status changed", fields)

				// Add to debug log buffer for dashboard
				if h.debugLogBuffer != nil {
					h.debugLogBuffer.Add("INFO", fmt.Sprintf("Node %s status: %s → %s", node.Hostname, oldStatus, status), fields)
				}
			}
		}

		if status == NodeStatusHealthy {
			// Sync actual containers from Docker to prevent ghost containers
			h.syncContainersFromNode(node)

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

	// FIX #7: Minecraft Health Check - Check if Minecraft is responding on port 25565
	// This detects when Minecraft crashes but the container keeps running
	h.checkMinecraftHealth()

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

// syncContainersFromNode fetches actual containers from Docker and syncs with the registry
// This prevents ghost containers by removing entries that no longer exist
func (h *HealthChecker) syncContainersFromNode(node *Node) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Determine if node is local or remote
	isLocal := node.Type == "local" || node.IPAddress == "" || node.IPAddress == "localhost" || node.IPAddress == "127.0.0.1"

	var actualContainerIDs map[string]bool
	var err error

	if isLocal {
		actualContainerIDs, err = h.getLocalContainerIDs(ctx)
	} else {
		actualContainerIDs, err = h.getRemoteContainerIDs(ctx, node)
	}

	if err != nil {
		logger.Warn("Failed to sync containers from node", map[string]interface{}{
			"node_id": node.ID,
			"error":   err.Error(),
		})
		return
	}

	// Sync with container registry
	h.containerRegistry.SyncNodeContainers(node.ID, actualContainerIDs)
}

// getLocalContainerIDs fetches all mc-* container IDs from the local Docker daemon
func (h *HealthChecker) getLocalContainerIDs(ctx context.Context) (map[string]bool, error) {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	// List all mc-* containers (including stopped ones)
	containers, err := dockerClient.ContainerList(ctx, container.ListOptions{
		All: true, // Include stopped containers
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	// Build map of container IDs
	containerIDs := make(map[string]bool)
	for _, container := range containers {
		// Only include mc-* containers
		for _, name := range container.Names {
			if strings.HasPrefix(name, "/mc-") || strings.HasPrefix(name, "mc-") {
				containerIDs[container.ID] = true
				break
			}
		}
	}

	logger.Debug("Local container sync", map[string]interface{}{
		"total_containers": len(containerIDs),
	})

	return containerIDs, nil
}

// getRemoteContainerIDs fetches all mc-* container IDs from a remote node via SSH
func (h *HealthChecker) getRemoteContainerIDs(ctx context.Context, node *Node) (map[string]bool, error) {
	if h.remoteClient == nil {
		return nil, fmt.Errorf("remote client not configured")
	}

	remoteNode := &docker.RemoteNode{
		ID:        node.ID,
		IPAddress: node.IPAddress,
		SSHUser:   node.SSHUser,
	}

	// Execute: docker ps -a --filter "name=mc-" --format "{{.ID}}"
	cmd := `docker ps -a --filter "name=mc-" --format "{{.ID}}"`
	output, err := h.remoteClient.ExecuteSSHCommand(ctx, remoteNode, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	// Parse container IDs from output
	containerIDs := make(map[string]bool)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		containerID := strings.TrimSpace(line)
		if containerID != "" {
			containerIDs[containerID] = true
		}
	}

	logger.Debug("Remote container sync", map[string]interface{}{
		"node_id":          node.ID,
		"total_containers": len(containerIDs),
	})

	return containerIDs, nil
}

// checkMinecraftHealth checks if Minecraft is responding on port 25565 for all running containers
// FIX #7: Detects when Minecraft crashes internally but the container keeps running
// FIX BILLING-2: Auto-stop servers that are crashed for >5 minutes
func (h *HealthChecker) checkMinecraftHealth() {
	if h.containerRegistry == nil {
		return
	}

	// Get all containers
	containers := h.containerRegistry.GetAllContainers()

	for _, container := range containers {
		// Only check running containers
		if container.Status != "running" {
			continue
		}

		// Get node info to determine IP address
		node, nodeExists := h.nodeRegistry.GetNode(container.NodeID)
		if !nodeExists {
			continue
		}

		// Build connection address
		// For local nodes, use localhost
		// For remote nodes, use node IP address
		var address string
		if node.Type == "local" || node.IPAddress == "" || node.IPAddress == "localhost" || node.IPAddress == "127.0.0.1" {
			address = fmt.Sprintf("localhost:%d", container.MinecraftPort)
		} else {
			address = fmt.Sprintf("%s:%d", node.IPAddress, container.MinecraftPort)
		}

		// Try to connect to Minecraft port with 3 second timeout
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", address)
		cancel()

		if err != nil {
			// Port not responding - Minecraft might be crashed
			// FIX BILLING-2: Track consecutive failures and auto-stop if crashed
			h.handleMinecraftCrash(container, address, err)
		} else {
			// Successfully connected - Minecraft is responding
			conn.Close()

			// Reset crash counter if server recovered
			if h.crashCounters[container.ServerID] > 0 {
				logger.Info("MC-HEALTH: Server recovered", map[string]interface{}{
					"server_id":           container.ServerID,
					"server_name":         container.ServerName,
					"previous_failures":   h.crashCounters[container.ServerID],
				})
				delete(h.crashCounters, container.ServerID)
				delete(h.crashTimestamps, container.ServerID)
			}

			logger.Debug("MC-HEALTH: Minecraft port responding", map[string]interface{}{
				"server_id":      container.ServerID,
				"minecraft_port": container.MinecraftPort,
			})
		}
	}
}

// handleMinecraftCrash handles a detected Minecraft crash
// FIX BILLING-2: Auto-stop crashed servers after 5 minutes of consecutive failures
func (h *HealthChecker) handleMinecraftCrash(container *ContainerInfo, address string, connErr error) {
	serverID := container.ServerID

	// Increment crash counter
	h.crashCounters[serverID]++

	// Track first failure timestamp
	if h.crashCounters[serverID] == 1 {
		h.crashTimestamps[serverID] = time.Now()
	}

	crashDuration := time.Since(h.crashTimestamps[serverID])
	failureCount := h.crashCounters[serverID]

	logger.Warn("MC-HEALTH: Minecraft not responding on port", map[string]interface{}{
		"server_id":      serverID,
		"server_name":    container.ServerName,
		"node_id":        container.NodeID,
		"minecraft_port": container.MinecraftPort,
		"address":        address,
		"error":          connErr.Error(),
		"failure_count":  failureCount,
		"crash_duration": crashDuration.String(),
	})

	// FIX BILLING-2: Auto-stop server if crashed for >5 minutes (5 consecutive failed checks at 60s intervals)
	// This prevents billing users for non-functional servers
	if failureCount >= 5 && h.minecraftService != nil {
		logger.Error("MC-HEALTH: Server crashed for >5 minutes - auto-stopping", fmt.Errorf("minecraft unresponsive"), map[string]interface{}{
			"server_id":      serverID,
			"server_name":    container.ServerName,
			"failure_count":  failureCount,
			"crash_duration": crashDuration.String(),
		})

		// Stop the server (this will also stop billing)
		go func() {
			if err := h.minecraftService.StopServer(serverID, "crashed"); err != nil {
				logger.Error("MC-HEALTH: Failed to auto-stop crashed server", err, map[string]interface{}{
					"server_id": serverID,
				})
			} else {
				logger.Info("MC-HEALTH: Crashed server stopped successfully", map[string]interface{}{
					"server_id":      serverID,
					"crash_duration": crashDuration.String(),
				})

				// Clear counters after successful stop
				delete(h.crashCounters, serverID)
				delete(h.crashTimestamps, serverID)
			}
		}()
	}
}

// ===================================
// GAP-1: Node Failure Handling
// ===================================

// handleNodeFailure handles a node that has become unhealthy
// This fixes GAP-1 (Worker Node Total Failure) by:
// 1. Closing billing sessions for all containers on the failed node
// 2. Updating server status from "running" to "crashed"
// 3. Removing containers from registry
// 4. Logging for user notification
func (h *HealthChecker) handleNodeFailure(node *Node) {
	if h.containerRegistry == nil {
		return
	}

	// Get all containers on this node
	containers := h.containerRegistry.GetAllContainers()
	affectedServers := []string{}

	for _, container := range containers {
		if container.NodeID == node.ID {
			affectedServers = append(affectedServers, container.ServerID)
		}
	}

	if len(affectedServers) == 0 {
		logger.Info("NODE-FAILURE: No containers on failed node", map[string]interface{}{
			"node_id":  node.ID,
			"hostname": node.Hostname,
		})
		return
	}

	logger.Error("NODE-FAILURE: Node failed with running containers", fmt.Errorf("node unhealthy"), map[string]interface{}{
		"node_id":          node.ID,
		"hostname":         node.Hostname,
		"affected_servers": len(affectedServers),
	})

	// Handle each affected server
	for _, serverID := range affectedServers {
		if h.minecraftService == nil {
			logger.Warn("NODE-FAILURE: Cannot handle server - MinecraftService not set", map[string]interface{}{
				"server_id": serverID,
			})
			continue
		}

		// Call MinecraftService to handle the failure (closes billing, updates status)
		go func(sid string) {
			if err := h.minecraftService.HandleNodeFailure(sid); err != nil {
				logger.Error("NODE-FAILURE: Failed to handle server on failed node", err, map[string]interface{}{
					"server_id": sid,
					"node_id":   node.ID,
				})
			} else {
				logger.Info("NODE-FAILURE: Server handled on failed node", map[string]interface{}{
					"server_id": sid,
					"node_id":   node.ID,
				})
			}
		}(serverID)
	}

	// Remove all containers from registry (in-memory cleanup)
	h.containerRegistry.RemoveContainersByNode(node.ID)

	logger.Info("NODE-FAILURE: Containers removed from registry", map[string]interface{}{
		"node_id": node.ID,
		"count":   len(affectedServers),
	})
}
