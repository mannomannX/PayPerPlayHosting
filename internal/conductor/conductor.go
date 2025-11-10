package conductor

import (
	"fmt"
	"reflect"
	"time"

	"github.com/payperplay/hosting/internal/cloud"
	"github.com/payperplay/hosting/pkg/config"
	"github.com/payperplay/hosting/pkg/logger"
)

// Conductor is the central fleet orchestrator
type Conductor struct {
	NodeRegistry      *NodeRegistry
	ContainerRegistry *ContainerRegistry
	HealthChecker     *HealthChecker
	ScalingEngine     *ScalingEngine // B5 - Auto-Scaling
	StartQueue        *StartQueue    // Queue for servers waiting for capacity
	StartedAt         time.Time      // When Conductor started (for startup delay)
}

// NewConductor creates a new conductor instance
func NewConductor(healthCheckInterval time.Duration) *Conductor {
	nodeRegistry := NewNodeRegistry()
	containerRegistry := NewContainerRegistry()
	healthChecker := NewHealthChecker(nodeRegistry, containerRegistry, healthCheckInterval)

	return &Conductor{
		NodeRegistry:      nodeRegistry,
		ContainerRegistry: containerRegistry,
		HealthChecker:     healthChecker,
		ScalingEngine:     nil, // Initialized later with cloud provider
		StartQueue:        NewStartQueue(),
		StartedAt:         time.Now(), // Track startup time for delay
	}
}

// InitializeScaling initializes the scaling engine with a cloud provider
// This is called after conductor creation once cloud credentials are available
func (c *Conductor) InitializeScaling(cloudProvider cloud.CloudProvider, sshKeyName string, enabled bool) {
	if c.ScalingEngine != nil {
		logger.Warn("Scaling engine already initialized", nil)
		return
	}

	vmProvisioner := NewVMProvisioner(cloudProvider, c.NodeRegistry, sshKeyName)
	c.ScalingEngine = NewScalingEngine(cloudProvider, vmProvisioner, c.NodeRegistry, c.StartQueue, enabled)

	logger.Info("Scaling engine initialized", map[string]interface{}{
		"ssh_key": sshKeyName,
		"enabled": enabled,
	})
}

// Start starts the conductor and all its subsystems
func (c *Conductor) Start() {
	logger.Info("Starting Conductor Core", nil)

	// Start health checker
	c.HealthChecker.Start()

	// Bootstrap: Register the current node (localhost)
	c.bootstrapLocalNode()

	// Start scaling engine if initialized
	if c.ScalingEngine != nil {
		c.ScalingEngine.Start()
		logger.Info("Scaling engine started", nil)
	} else {
		logger.Warn("Scaling engine not initialized, skipping", nil)
	}

	logger.Info("Conductor Core started successfully", nil)
}

// SyncRunningContainers synchronizes Conductor's RAM tracking with Docker reality
// CRITICAL: This prevents OOM crashes after restarts by detecting existing containers
// Called on startup to recover state after crashes/restarts/deployments
//
// This must be called from main.go after services are initialized
func (c *Conductor) SyncRunningContainers(dockerSvc interface{}, serverRepo interface{}) {
	logger.Info("STATE_SYNC: Detecting running Minecraft containers...", nil)

	// Use reflection to call ListRunningMinecraftContainers on dockerSvc
	dockerVal := reflect.ValueOf(dockerSvc)
	listMethod := dockerVal.MethodByName("ListRunningMinecraftContainers")
	if !listMethod.IsValid() {
		logger.Error("STATE_SYNC: Docker service missing ListRunningMinecraftContainers method", nil, nil)
		return
	}

	// Call the method
	results := listMethod.Call(nil)
	if len(results) != 2 {
		logger.Error("STATE_SYNC: Unexpected return from ListRunningMinecraftContainers", nil, nil)
		return
	}

	// Check for error (second return value)
	if !results[1].IsNil() {
		err := results[1].Interface().(error)
		logger.Error("STATE_SYNC: Failed to list containers", err, nil)
		return
	}

	// Get containers slice
	containersVal := results[0]
	if containersVal.Len() == 0 {
		logger.Info("STATE_SYNC: No running containers (clean state)", nil)
		return
	}

	logger.Info("STATE_SYNC: Found containers, syncing RAM allocations...", map[string]interface{}{
		"count": containersVal.Len(),
	})

	syncedCount := 0
	totalRAM := 0

	// Iterate over containers
	for i := 0; i < containersVal.Len(); i++ {
		container := containersVal.Index(i)
		containerID := container.FieldByName("ContainerID").String()
		serverID := container.FieldByName("ServerID").String()

		// Use reflection to call FindByID on serverRepo
		repoVal := reflect.ValueOf(serverRepo)
		findMethod := repoVal.MethodByName("FindByID")
		if !findMethod.IsValid() {
			logger.Error("STATE_SYNC: Repository missing FindByID method", nil, nil)
			continue
		}

		// Call FindByID(serverID)
		findResults := findMethod.Call([]reflect.Value{reflect.ValueOf(serverID)})
		if len(findResults) != 2 {
			logger.Warn("STATE_SYNC: Unexpected return from FindByID", map[string]interface{}{
				"server_id": serverID[:8],
			})
			continue
		}

		// Check for error
		if !findResults[1].IsNil() {
			logger.Warn("STATE_SYNC: Container found but server not in DB", map[string]interface{}{
				"container": containerID[:12],
				"server_id": serverID[:8],
			})
			continue
		}

		// Get server object
		serverVal := findResults[0]
		if serverVal.IsNil() {
			logger.Warn("STATE_SYNC: Server is nil", map[string]interface{}{
				"server_id": serverID[:8],
			})
			continue
		}

		// Call GetRAMMb() method on server
		getRamMethod := serverVal.MethodByName("GetRAMMb")
		if !getRamMethod.IsValid() {
			logger.Warn("STATE_SYNC: Server missing GetRAMMb method", map[string]interface{}{
				"server_id": serverID[:8],
			})
			continue
		}

		ramResults := getRamMethod.Call(nil)
		if len(ramResults) != 1 {
			logger.Warn("STATE_SYNC: Unexpected return from GetRAMMb", map[string]interface{}{
				"server_id": serverID[:8],
			})
			continue
		}

		ramMB := int(ramResults[0].Int())

		// Force allocate RAM (bypass checks - container IS running!)
		c.NodeRegistry.mu.Lock()
		if node, exists := c.NodeRegistry.nodes["local-node"]; exists {
			node.AllocatedRAMMB += ramMB
			node.ContainerCount++
		}
		c.NodeRegistry.mu.Unlock()

		// CRITICAL: Also register in ContainerRegistry to prevent HealthChecker from resetting RAM!
		// HealthChecker calls GetNodeAllocation() which reads from ContainerRegistry
		containerInfo := &ContainerInfo{
			ContainerID: containerID,
			ServerID:    serverID,
			NodeID:      "local-node",
			RAMMb:       ramMB,
			Status:      "running",
		}
		c.ContainerRegistry.RegisterContainer(containerInfo)

		totalRAM += ramMB
		syncedCount++

		logger.Info("STATE_SYNC: Container synced", map[string]interface{}{
			"container": containerID[:12],
			"server":    serverID[:8],
			"ram_mb":    ramMB,
		})
	}

	logger.Info("STATE_SYNC: Completed", map[string]interface{}{
		"synced":       syncedCount,
		"total_ram_mb": totalRAM,
	})
}

// Stop stops the conductor and all its subsystems
func (c *Conductor) Stop() {
	logger.Info("Stopping Conductor Core", nil)

	// Stop scaling engine
	if c.ScalingEngine != nil {
		c.ScalingEngine.Stop()
	}

	// Stop health checker
	c.HealthChecker.Stop()

	logger.Info("Conductor Core stopped", nil)
}

// bootstrapLocalNode registers the local Docker host as a node
func (c *Conductor) bootstrapLocalNode() {
	// TODO: Auto-detect system resources using Docker API or /proc/meminfo
	// For now, using a conservative estimate based on actual system capacity
	cfg := config.AppConfig

	localNode := &Node{
		ID:               "local-node",
		Hostname:         "localhost",
		IPAddress:        "127.0.0.1",
		Type:             "dedicated",
		TotalRAMMB:       3500, // ~3.5GB - conservative estimate for 3.7GB system
		TotalCPUCores:    2,    // Adjust based on actual server
		Status:           NodeStatusUnknown,
		LastHealthCheck:  time.Now(),
		ContainerCount:   0,
		AllocatedRAMMB:   0,
		DockerSocketPath: "/var/run/docker.sock",
		SSHUser:          "root",
	}

	// Calculate intelligent system reserve (3-tier strategy)
	localNode.UpdateSystemReserve(cfg.SystemReservedRAMMB, cfg.SystemReservedRAMPercent)

	c.NodeRegistry.RegisterNode(localNode)

	logger.Info("Local node registered with intelligent system reserve", map[string]interface{}{
		"node_id":              localNode.ID,
		"total_ram_mb":         localNode.TotalRAMMB,
		"system_reserved_mb":   localNode.SystemReservedRAMMB,
		"usable_ram_mb":        localNode.UsableRAMMB(),
		"total_cpu":            localNode.TotalCPUCores,
		"reservation_strategy": "3-tier intelligent",
	})
}

// CheckCapacity checks if there's enough capacity to start a server with the given RAM requirement
// Returns (hasCapacity bool, availableRAMMB int)
// DEPRECATED: Use AtomicAllocateRAM() instead to prevent race conditions
func (c *Conductor) CheckCapacity(requiredRAMMB int) (bool, int) {
	fleetStats := c.NodeRegistry.GetFleetStats()
	hasCapacity := fleetStats.AvailableRAMMB >= requiredRAMMB
	return hasCapacity, fleetStats.AvailableRAMMB
}

// CanStartServer checks if a server can start now (STARTUP-DELAY + CPU + RAM guard)
// Returns (canStart bool, reason string)
// STARTUP-DELAY: Prevents server starts for 2 minutes after API startup (allows CPU to settle)
// CPU-GUARD: Prevents parallel server starts to avoid CPU overload
func (c *Conductor) CanStartServer(ramMB int) (bool, string) {
	// STARTUP-DELAY: Check if API has been running for at least 2 minutes
	uptime := time.Since(c.StartedAt)
	if uptime < 2*time.Minute {
		remaining := 2*time.Minute - uptime
		return false, fmt.Sprintf("API startup delay active (%d seconds remaining)", int(remaining.Seconds()))
	}

	// CPU-GUARD: Check if another server is already starting
	startingCount := c.ContainerRegistry.GetStartingCount()
	if startingCount > 0 {
		return false, "another server is currently starting (CPU protection)"
	}

	// RAM-GUARD: Check if we have enough RAM capacity
	fleetStats := c.NodeRegistry.GetFleetStats()
	if fleetStats.AvailableRAMMB < ramMB {
		return false, "insufficient RAM capacity"
	}

	return true, ""
}

// AtomicAllocateRAM atomically reserves RAM for a server
// Returns true if allocation succeeded, false if insufficient capacity
// THIS IS THE SAFE METHOD - prevents race conditions!
func (c *Conductor) AtomicAllocateRAM(ramMB int) bool {
	return c.NodeRegistry.AtomicAllocateRAM(ramMB)
}

// ReleaseRAM atomically releases RAM when a server stops
func (c *Conductor) ReleaseRAM(ramMB int) {
	c.NodeRegistry.ReleaseRAM(ramMB)
}

// EnqueueServer adds a server to the start queue
func (c *Conductor) EnqueueServer(serverID, serverName string, requiredRAMMB int, userID string) {
	queuedServer := &QueuedServer{
		ServerID:      serverID,
		ServerName:    serverName,
		RequiredRAMMB: requiredRAMMB,
		QueuedAt:      time.Now(),
		UserID:        userID,
	}
	c.StartQueue.Enqueue(queuedServer)

	logger.Info("Server enqueued, waiting for capacity", map[string]interface{}{
		"server_id":      serverID,
		"server_name":    serverName,
		"required_ram":   requiredRAMMB,
		"queue_position": c.StartQueue.GetPosition(serverID),
	})

	// Trigger queue processing (will check if scaling needed)
	go c.ProcessStartQueue()
}

// IsServerQueued checks if a server is currently in the start queue
func (c *Conductor) IsServerQueued(serverID string) bool {
	return c.StartQueue.GetPosition(serverID) > 0
}

// RemoveFromQueue removes a server from the start queue
func (c *Conductor) RemoveFromQueue(serverID string) {
	if c.StartQueue.Remove(serverID) {
		logger.Info("Server removed from start queue", map[string]interface{}{
			"server_id": serverID,
		})
	}
}

// ProcessStartQueue attempts to start servers from the queue when capacity is available
// This should be called:
// 1. After a server is stopped (frees capacity)
// 2. After a new node comes online
// 3. Periodically by a background worker
func (c *Conductor) ProcessStartQueue() {
	if c.StartQueue.Size() == 0 {
		return // Nothing to process
	}

	logger.Info("Processing start queue", map[string]interface{}{
		"queue_size": c.StartQueue.Size(),
	})

	// Process queue until we run out of capacity or servers
	for {
		queuedServer := c.StartQueue.Peek()
		if queuedServer == nil {
			break // Queue empty
		}

		// Check if we have capacity for this server
		fleetStats := c.NodeRegistry.GetFleetStats()
		if fleetStats.AvailableRAMMB < queuedServer.RequiredRAMMB {
			logger.Info("Insufficient capacity for queued server", map[string]interface{}{
				"server_id":      queuedServer.ServerID,
				"required_ram":   queuedServer.RequiredRAMMB,
				"available_ram":  fleetStats.AvailableRAMMB,
				"queue_position": 1,
			})

			// Trigger scaling if enabled
			if c.ScalingEngine != nil && c.ScalingEngine.IsEnabled() {
				logger.Info("Queued servers waiting for capacity, scaling will be triggered in next cycle", map[string]interface{}{
					"queue_size":     c.StartQueue.Size(),
					"total_required": c.StartQueue.GetTotalRequiredRAM(),
				})
				// ScalingEngine will check and scale if needed in its next cycle (every 2 minutes)
			}

			break // Stop processing, wait for more capacity
		}

		// We have capacity - dequeue and signal that server can start
		server := c.StartQueue.Dequeue()

		logger.Info("Capacity available for queued server", map[string]interface{}{
			"server_id":     server.ServerID,
			"server_name":   server.ServerName,
			"required_ram":  server.RequiredRAMMB,
			"available_ram": fleetStats.AvailableRAMMB,
			"wait_time":     time.Since(server.QueuedAt).String(),
		})

		// Note: The actual server start will be triggered by the MinecraftService
		// after checking the queue status. We don't start it here to avoid tight coupling.
	}
}

// GetStatus returns the current conductor status
func (c *Conductor) GetStatus() ConductorStatus {
	fleetStats := c.NodeRegistry.GetFleetStats()
	nodes := c.NodeRegistry.GetAllNodes()
	containers := c.ContainerRegistry.GetAllContainers()

	status := ConductorStatus{
		FleetStats:      fleetStats,
		Nodes:           nodes,
		Containers:      containers,
		TotalContainers: len(containers),
		QueuedServers:   c.StartQueue.GetAll(),
		QueueSize:       c.StartQueue.Size(),
	}

	// Add scaling engine status if available
	if c.ScalingEngine != nil {
		scalingStatus := c.ScalingEngine.GetStatus()
		status.ScalingEngine = &scalingStatus
	}

	return status
}

// ConductorStatus contains the current status of the conductor
type ConductorStatus struct {
	FleetStats      FleetStats           `json:"fleet_stats"`
	Nodes           []*Node              `json:"nodes"`
	Containers      []*ContainerInfo     `json:"containers"`
	TotalContainers int                  `json:"total_containers"`
	ScalingEngine   *ScalingEngineStatus `json:"scaling_engine,omitempty"`
	QueuedServers   []*QueuedServer      `json:"queued_servers,omitempty"`
	QueueSize       int                  `json:"queue_size"`
}
