package conductor

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	dockerclient "github.com/docker/docker/client"
	"github.com/payperplay/hosting/internal/audit"
	"github.com/payperplay/hosting/internal/cloud"
	"github.com/payperplay/hosting/internal/docker"
	"github.com/payperplay/hosting/internal/events"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/pkg/config"
	"github.com/payperplay/hosting/pkg/logger"
)

// ServerStarter interface allows Conductor to start servers without direct coupling to MinecraftService
// Future: Can be implemented by LocalStarter, RemoteNodeStarter (B5), etc.
type ServerStarter interface {
	// StartServerFromQueue starts a server that was dequeued (bypasses queue checks)
	StartServerFromQueue(serverID string) error
}

// Conductor is the central fleet orchestrator
type Conductor struct {
	NodeRegistry      *NodeRegistry
	ContainerRegistry *ContainerRegistry
	HealthChecker     *HealthChecker
	NodeSelector      *NodeSelector              // Multi-Node: Intelligent node selection for container placement
	ScalingEngine     *ScalingEngine             // B5 - Auto-Scaling
	RemoteClient      *docker.RemoteDockerClient // For remote node operations (SSH-based)
	CloudProvider     cloud.CloudProvider        // Cloud provider for metrics (optional)
	StartQueue        *StartQueue                // Queue for servers waiting for capacity
	DebugLogBuffer    *DebugLogBuffer            // Buffer for dashboard debug console
	StartedAt         time.Time                  // When Conductor started (for startup delay)
	serverStarter     ServerStarter              // Interface to start servers (injected)
	nodeRepo          NodeRepositoryInterface    // For persisting nodes to database
	ServerRepo        ServerRepositoryInterface  // For ghost container cleanup
	stopChan          chan struct{}              // For graceful shutdown of background workers
	AuditLog          *audit.AuditLogger         // Audit log for tracking destructive actions
	queueProcessMu    sync.Mutex                 // Prevents concurrent ProcessStartQueue() calls
}

// NodeRepositoryInterface defines the interface for node persistence
// This allows for dependency injection and easier testing
type NodeRepositoryInterface interface {
	Create(node interface{}) error
	FindByID(id string) (interface{}, error)
	FindAll() (interface{}, error)
	Update(node interface{}) error
	UpsertNode(node interface{}) error
}

// ServerRepositoryInterface defines minimal interface for ghost container cleanup
type ServerRepositoryInterface interface {
	FindByID(id string) (*models.MinecraftServer, error)
}

// NewConductor creates a new conductor instance
// sshKeyPath is optional - if empty, remote node health checks will be skipped
func NewConductor(healthCheckInterval time.Duration, sshKeyPath string) *Conductor {
	nodeRegistry := NewNodeRegistry()
	containerRegistry := NewContainerRegistry()

	// Inject NodeRegistry into ContainerRegistry for lifecycle tracking
	containerRegistry.SetNodeRegistry(nodeRegistry)

	// Create RemoteDockerClient if SSH key path is provided
	var remoteClient *docker.RemoteDockerClient
	if sshKeyPath != "" {
		var err error
		remoteClient, err = docker.NewRemoteDockerClient(sshKeyPath)
		if err != nil {
			logger.Warn("Failed to create RemoteDockerClient, remote health checks will be skipped", map[string]interface{}{
				"ssh_key_path": sshKeyPath,
				"error":        err.Error(),
			})
			remoteClient = nil
		} else {
			logger.Info("RemoteDockerClient initialized successfully", map[string]interface{}{
				"ssh_key_path": sshKeyPath,
			})
		}
	}

	debugLogBuffer := NewDebugLogBuffer(200) // Keep last 200 debug events
	healthChecker := NewHealthChecker(nodeRegistry, containerRegistry, remoteClient, debugLogBuffer, healthCheckInterval)
	nodeSelector := NewNodeSelector(nodeRegistry)

	return &Conductor{
		NodeRegistry:      nodeRegistry,
		ContainerRegistry: containerRegistry,
		HealthChecker:     healthChecker,
		NodeSelector:      nodeSelector,
		RemoteClient:      remoteClient,
		ScalingEngine:     nil, // Initialized later with cloud provider
		StartQueue:        NewStartQueue(),
		DebugLogBuffer:    debugLogBuffer,
		StartedAt:         time.Now(), // Track startup time for delay
		stopChan:          make(chan struct{}),
		AuditLog:          audit.NewAuditLogger(1000), // Keep last 1000 audit entries
	}
}

// InitializeScaling initializes the scaling engine with a cloud provider
// This is called after conductor creation once cloud credentials are available
func (c *Conductor) InitializeScaling(cloudProvider cloud.CloudProvider, sshKeyName string, enabled bool, velocityClient VelocityClient) {
	if c.ScalingEngine != nil {
		logger.Warn("Scaling engine already initialized", nil)
		return
	}

	// Store cloud provider for CPU metrics
	c.CloudProvider = cloudProvider

	vmProvisioner := NewVMProvisioner(cloudProvider, c.NodeRegistry, c.DebugLogBuffer, sshKeyName)
	c.ScalingEngine = NewScalingEngine(cloudProvider, vmProvisioner, c.NodeRegistry, c.StartQueue, c.DebugLogBuffer, enabled, velocityClient)
	c.ScalingEngine.SetConductor(c) // Set back-reference for migrations (B8)

	logger.Info("Scaling engine initialized", map[string]interface{}{
		"ssh_key": sshKeyName,
		"enabled": enabled,
		"consolidation_enabled": velocityClient != nil,
	})
}

// Start starts the conductor and all its subsystems
func (c *Conductor) Start() {
	logger.Info("Starting Conductor Core", nil)

	// Start health checker
	c.HealthChecker.Start()

	// Bootstrap: Register the current node (localhost)
	c.bootstrapLocalNode()

	// Bootstrap: Register proxy node if configured (Tier 2 - Proxy Layer)
	if c.RemoteClient != nil {
		c.bootstrapProxyNode()
	}

	// Start scaling engine if initialized
	if c.ScalingEngine != nil {
		c.ScalingEngine.Start()
		logger.Info("Scaling engine started", nil)
	} else {
		logger.Warn("Scaling engine not initialized, skipping", nil)
	}

	// Start startup delay timer (triggers queue after 2 minutes)
	go c.startupDelayWorker()
	logger.Info("Startup delay timer started (2-minute countdown)", nil)

	// Start periodic queue processor (checks every 30 seconds as failsafe)
	go c.periodicQueueWorker()
	logger.Info("Periodic queue worker started (30-second intervals)", nil)

	// Start reservation timeout cleaner (checks every 5 minutes)
	go c.reservationTimeoutWorker()
	logger.Info("Reservation timeout worker started (5-minute intervals, 30-minute timeout)", nil)

	// Start CPU metrics collector (checks every 60 seconds)
	go c.cpuMetricsWorker()
	logger.Info("CPU metrics worker started (60-second intervals)", nil)

	// Start ghost container cleanup worker (checks every minute)
	go c.ghostContainerCleanupWorker()
	logger.Info("Ghost container cleanup worker started (1-minute intervals)", nil)

	// NOTE: Worker-Node sync is now called explicitly from main.go AFTER queue sync
	// This ensures the queue is populated before scaling decisions are made
	// See cmd/api/main.go for the startup sequence

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

// SyncQueuedServers synchronizes queued servers from database into StartQueue
// CRITICAL: Prevents queue loss after container restart, ensures Worker-Nodes aren't decommissioned prematurely
// This must be called from main.go after services are initialized
// If triggerScaling is false, no scaling check will be triggered (useful during startup sequence)
func (c *Conductor) SyncQueuedServers(serverRepo interface{}, triggerScaling bool) {
	logger.Info("QUEUE_SYNC: Detecting queued servers from database...", nil)

	// Use reflection to call FindByStatus on serverRepo
	repoVal := reflect.ValueOf(serverRepo)
	findMethod := repoVal.MethodByName("FindByStatus")
	if !findMethod.IsValid() {
		logger.Error("QUEUE_SYNC: Repository missing FindByStatus method", nil, nil)
		return
	}

	// Call FindByStatus("queued")
	results := findMethod.Call([]reflect.Value{reflect.ValueOf("queued")})
	if len(results) != 2 {
		logger.Error("QUEUE_SYNC: Unexpected return from FindByStatus", nil, nil)
		return
	}

	// Check for error (second return value)
	if !results[1].IsNil() {
		err := results[1].Interface().(error)
		logger.Error("QUEUE_SYNC: Failed to query queued servers", err, nil)
		return
	}

	// Get servers slice
	serversVal := results[0]
	if serversVal.Len() == 0 {
		logger.Info("QUEUE_SYNC: No queued servers found (clean state)", nil)
		return
	}

	logger.Info("QUEUE_SYNC: Found queued servers, re-enqueuing...", map[string]interface{}{
		"count": serversVal.Len(),
	})

	enqueuedCount := 0

	// Iterate over servers
	for i := 0; i < serversVal.Len(); i++ {
		server := serversVal.Index(i)

		// Extract fields
		serverID := server.FieldByName("ID").String()
		serverName := server.FieldByName("Name").String()
		ownerID := server.FieldByName("OwnerID").String()

		// Get RAM via GetRAMMb() method (need Addr() for pointer receiver)
		getRamMethod := serversVal.Index(i).Addr().MethodByName("GetRAMMb")
		if !getRamMethod.IsValid() {
			logger.Warn("QUEUE_SYNC: Server missing GetRAMMb method", map[string]interface{}{
				"server_id": serverID[:8],
			})
			continue
		}

		ramResults := getRamMethod.Call(nil)
		if len(ramResults) != 1 {
			logger.Warn("QUEUE_SYNC: Unexpected return from GetRAMMb", map[string]interface{}{
				"server_id": serverID[:8],
			})
			continue
		}

		ramMB := int(ramResults[0].Int())

		// Enqueue the server
		queuedServer := &QueuedServer{
			ServerID:      serverID,
			ServerName:    serverName,
			RequiredRAMMB: ramMB,
			QueuedAt:      time.Now(), // Use current time since we don't have original queue time
			UserID:        ownerID,
		}

		c.StartQueue.Enqueue(queuedServer)
		enqueuedCount++

		logger.Info("QUEUE_SYNC: Server re-enqueued", map[string]interface{}{
			"server_id":   serverID[:8],
			"server_name": serverName,
			"ram_mb":      ramMB,
		})
	}

	logger.Info("QUEUE_SYNC: Completed", map[string]interface{}{
		"enqueued": enqueuedCount,
	})

	// Trigger immediate scaling check to provision capacity for queued servers (only if requested)
	if triggerScaling && enqueuedCount > 0 {
		logger.Info("QUEUE_SYNC: Triggering scaling check to provision capacity", nil)
		c.TriggerScalingCheck()
	}
}

// Stop stops the conductor and all its subsystems
func (c *Conductor) Stop() {
	logger.Info("Stopping Conductor Core", nil)

	// Stop background workers
	close(c.stopChan)

	// Stop scaling engine
	if c.ScalingEngine != nil {
		c.ScalingEngine.Stop()
	}

	// Stop health checker
	c.HealthChecker.Stop()

	logger.Info("Conductor Core stopped", nil)
}

// bootstrapLocalNode registers the local Docker host as a node
// Auto-detects system resources using Docker API
func (c *Conductor) bootstrapLocalNode() {
	cfg := config.AppConfig

	// Auto-detect system resources via Docker API
	totalRAMMB, totalCPU := c.detectSystemResources()

	now := time.Now()
	localNode := &Node{
		ID:               "local-node",
		Hostname:         "localhost",
		IPAddress:        "127.0.0.1",
		Type:             "dedicated",
		TotalRAMMB:       totalRAMMB,
		TotalCPUCores:    totalCPU,
		Status:           NodeStatusUnknown,  // DEPRECATED - use HealthStatus
		LifecycleState:   NodeStateActive,    // System nodes start as active
		HealthStatus:     HealthStatusHealthy,
		Metrics: NodeLifecycleMetrics{
			ProvisionedAt:       now,
			InitializedAt:       &now,
			FirstContainerAt:    nil,
			LastContainerAt:     nil,
			TotalContainersEver: 0,
			CurrentContainers:   0,
		},
		LastHealthCheck:  time.Now(),
		ContainerCount:   0,
		AllocatedRAMMB:   0,
		DockerSocketPath: "/var/run/docker.sock",
		SSHUser:          "root",
		Labels: map[string]string{
			"provider": "hetzner",
			"location": "nbg1",
			"tier":     "control-plane",
		},
	}

	// Calculate proportional system reserve (same as worker nodes for consistency)
	// PROPORTIONAL OVERHEAD MODEL: Control plane also uses 12.5% (1/8) system budget
	localNode.UpdateSystemReserve(cfg.SystemReservedRAMMB, cfg.SystemReservedRAMPercent)

	c.NodeRegistry.RegisterNode(localNode)

	// Publish node created event for dashboard
	events.PublishNodeCreated(
		localNode.ID,
		localNode.Type,
		"hetzner",
		"nbg1",
		string(localNode.Status),
		localNode.IPAddress,
		localNode.TotalRAMMB,
		localNode.UsableRAMMB(),
		localNode.IsSystemNode,
		localNode.CreatedAt,
	)

	logger.Info("Local node registered with auto-detected resources", map[string]interface{}{
		"node_id":              localNode.ID,
		"total_ram_mb":         localNode.TotalRAMMB,
		"system_reserved_mb":   localNode.SystemReservedRAMMB,
		"usable_ram_mb":        localNode.UsableRAMMB(),
		"total_cpu":            localNode.TotalCPUCores,
		"reservation_strategy": "3-tier intelligent",
		"detection_method":     "docker-api",
	})
}

// detectSystemResources auto-detects total RAM and CPU cores using Docker API
// Returns (totalRAMMB, totalCPUCores)
func (c *Conductor) detectSystemResources() (int, int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create temporary Docker client for resource detection
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		logger.Warn("Failed to create Docker client for resource detection, using fallback values", map[string]interface{}{
			"error": err.Error(),
		})
		return 3500, 2 // Fallback to conservative estimate
	}
	defer cli.Close()

	info, err := cli.Info(ctx)
	if err != nil {
		logger.Warn("Failed to get Docker info for resource detection, using fallback values", map[string]interface{}{
			"error": err.Error(),
		})
		return 3500, 2 // Fallback to conservative estimate
	}

	// Convert bytes to MB
	totalRAMMB := int(info.MemTotal / 1024 / 1024)
	totalCPU := info.NCPU

	logger.Info("Auto-detected system resources via Docker API", map[string]interface{}{
		"total_ram_mb":   totalRAMMB,
		"total_cpu":      totalCPU,
		"os_type":        info.OSType,
		"architecture":   info.Architecture,
		"docker_version": info.ServerVersion,
	})

	return totalRAMMB, totalCPU
}

// bootstrapProxyNode registers the proxy node (Tier 2 - Proxy Layer) if configured
// Auto-detects resources via SSH + Docker API
func (c *Conductor) bootstrapProxyNode() {
	cfg := config.AppConfig

	// Skip if proxy node IP not configured
	if cfg.ProxyNodeIP == "" {
		logger.Info("Proxy node not configured, skipping registration", nil)
		return
	}

	// Skip if no remote client
	if c.RemoteClient == nil {
		logger.Warn("RemoteDockerClient not available, cannot register proxy node", nil)
		return
	}

	logger.Info("Registering proxy node (Tier 2)", map[string]interface{}{
		"ip_address": cfg.ProxyNodeIP,
		"ssh_user":   cfg.ProxyNodeSSHUser,
	})

	// Build RemoteNode struct for SSH operations
	remoteNode := &docker.RemoteNode{
		ID:        "proxy-node",
		IPAddress: cfg.ProxyNodeIP,
		SSHUser:   cfg.ProxyNodeSSHUser,
	}

	// Auto-detect system resources via SSH + Docker API
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	totalRAMMB, totalCPU, err := c.RemoteClient.GetSystemResources(ctx, remoteNode)
	if err != nil {
		logger.Error("Failed to detect proxy node resources, skipping registration", err, map[string]interface{}{
			"node_ip": cfg.ProxyNodeIP,
		})
		return
	}

	// Register proxy node
	proxyNow := time.Now()
	proxyNode := &Node{
		ID:               "proxy-node",
		Hostname:         "velocity-proxy",
		IPAddress:        cfg.ProxyNodeIP,
		Type:             "dedicated",
		TotalRAMMB:       totalRAMMB,
		TotalCPUCores:    totalCPU,
		Status:           NodeStatusUnknown,  // DEPRECATED - use HealthStatus
		LifecycleState:   NodeStateActive,    // System nodes start as active
		HealthStatus:     HealthStatusHealthy,
		Metrics: NodeLifecycleMetrics{
			ProvisionedAt:       proxyNow,
			InitializedAt:       &proxyNow,
			FirstContainerAt:    nil,
			LastContainerAt:     nil,
			TotalContainersEver: 0,
			CurrentContainers:   0,
		},
		LastHealthCheck:  time.Now(),
		ContainerCount:   0,
		AllocatedRAMMB:   0,
		DockerSocketPath: "/var/run/docker.sock",
		SSHUser:          cfg.ProxyNodeSSHUser,
		Labels: map[string]string{
			"provider": "hetzner",
			"location": "nbg1",
			"tier":     "proxy-layer",
		},
	}

	// Calculate proportional system reserve (same as worker nodes for consistency)
	// PROPORTIONAL OVERHEAD MODEL: Proxy layer also uses 12.5% (1/8) system budget
	proxyNode.UpdateSystemReserve(cfg.SystemReservedRAMMB, cfg.SystemReservedRAMPercent)

	c.NodeRegistry.RegisterNode(proxyNode)

	// Publish node created event for dashboard
	events.PublishNodeCreated(
		proxyNode.ID,
		proxyNode.Type,
		"hetzner",
		"nbg1",
		string(proxyNode.Status),
		proxyNode.IPAddress,
		proxyNode.TotalRAMMB,
		proxyNode.UsableRAMMB(),
		proxyNode.IsSystemNode,
		proxyNode.CreatedAt,
	)

	logger.Info("Proxy node registered with auto-detected resources", map[string]interface{}{
		"node_id":              proxyNode.ID,
		"total_ram_mb":         proxyNode.TotalRAMMB,
		"system_reserved_mb":   proxyNode.SystemReservedRAMMB,
		"usable_ram_mb":        proxyNode.UsableRAMMB(),
		"total_cpu":            proxyNode.TotalCPUCores,
		"tier":                 "proxy-layer",
		"detection_method":     "ssh-docker-api",
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

// AtomicAllocateRAMOnNode atomically reserves RAM on a specific node
// This is a wrapper for NodeRegistry.AtomicAllocateRAMOnNode()
func (c *Conductor) AtomicAllocateRAMOnNode(nodeID string, ramMB int) bool {
	return c.NodeRegistry.AtomicAllocateRAMOnNode(nodeID, ramMB)
}

// ReleaseRAMOnNode atomically releases RAM from a specific node
// This is a wrapper for NodeRegistry.ReleaseRAMOnNode()
func (c *Conductor) ReleaseRAMOnNode(nodeID string, ramMB int) {
	c.NodeRegistry.ReleaseRAMOnNode(nodeID, ramMB)
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

// AtomicReserveStartSlot atomically reserves a "starting" slot for CPU-Guard
// Returns true if slot reserved, false if another server is already starting
// CRITICAL: This must be called BEFORE Docker starts to prevent race conditions
func (c *Conductor) AtomicReserveStartSlot(serverID, serverName string, ramMB int) bool {
	c.ContainerRegistry.mu.Lock()
	defer c.ContainerRegistry.mu.Unlock()

	// Check if another server is already starting or reserving
	// We count both "starting" and "reserving" to prevent race conditions
	startingCount := 0
	for _, container := range c.ContainerRegistry.containers {
		if container.Status == "starting" || container.Status == "reserving" {
			startingCount++
		}
	}

	if startingCount > 0 {
		return false // Another server is starting/reserving, reject
	}

	// Reserve the slot immediately by registering with "starting" status
	// CPU-GUARD: Create reservation WITHOUT NodeID to avoid ghost containers
	// Status "reserving" indicates that node assignment is pending
	reservation := &ContainerInfo{
		ServerID:   serverID,
		ServerName: serverName,
		NodeID:     "", // Empty - node not assigned yet (prevents ghost on local-node)
		RAMMb:      ramMB,
		Status:     "reserving", // Special status for pending assignment
	}
	reservation.LastSeenAt = time.Now()
	// Direct map access is OK here because we hold the lock (line 691)
	c.ContainerRegistry.containers[serverID] = reservation

	logger.Info("CPU-GUARD: Start slot reserved atomically", map[string]interface{}{
		"server_id":   serverID,
		"server_name": serverName,
		"ram_mb":      ramMB,
	})

	return true
}

// ReleaseStartSlot removes a "starting" reservation if start fails
func (c *Conductor) ReleaseStartSlot(serverID string) {
	c.ContainerRegistry.RemoveContainer(serverID)
	logger.Info("CPU-GUARD: Start slot released", map[string]interface{}{
		"server_id": serverID,
	})
}

// UpdateContainerStatus updates the status of a container in the registry
// Used to transition from "starting" to "running" after server is ready
// This releases the CPU-Guard and allows queued servers to start
func (c *Conductor) UpdateContainerStatus(serverID, status string) {
	c.ContainerRegistry.mu.Lock()
	defer c.ContainerRegistry.mu.Unlock()

	if container, exists := c.ContainerRegistry.containers[serverID]; exists {
		oldStatus := container.Status
		container.Status = status
		container.LastSeenAt = time.Now()

		logger.Info("CPU-GUARD: Container status updated", map[string]interface{}{
			"server_id":  serverID,
			"old_status": oldStatus,
			"new_status": status,
		})
	}
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

	// Trigger queue processing - now that RAM is freed, queued servers might be able to start
	go c.ProcessStartQueue()
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

	// NOTE: DO NOT automatically trigger ProcessStartQueue() here!
	// This was causing endless cascade - every re-queue triggered a new ProcessStartQueue()
	// The Periodic Worker (30s) will process the queue, or explicit TriggerScalingCheck()
	// Removing this fixes the endless loop: EnqueueServer → ProcessStartQueue → EnqueueServer → ...
}

// IsServerQueued checks if a server is currently in the start queue
func (c *Conductor) IsServerQueued(serverID string) bool {
	return c.StartQueue.GetPosition(serverID) > 0
}

// SetServerStarter injects the ServerStarter implementation (typically MinecraftService)
func (c *Conductor) SetServerStarter(starter ServerStarter) {
	c.serverStarter = starter
}

// SetServerRepo sets the server repository for ghost container cleanup
func (c *Conductor) SetServerRepo(repo ServerRepositoryInterface) {
	c.ServerRepo = repo
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
	// Prevent concurrent processing - only one goroutine processes queue at a time
	// This prevents race conditions and duplicate server starts
	c.queueProcessMu.Lock()
	defer c.queueProcessMu.Unlock()

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

		// CRITICAL: Check capacity ONLY on Worker Nodes (MC servers cannot run on System nodes)
		// ONLY count HEALTHY nodes! Unhealthy nodes cannot accept containers
		workerNodeRAM := 0
		workerNodeCount := 0
		for _, node := range c.NodeRegistry.GetAllNodes() {
			if !node.IsSystemNode && node.Status == NodeStatusHealthy {
				workerNodeRAM += node.AvailableRAMMB()
				workerNodeCount++
			}
		}

		if workerNodeRAM < queuedServer.RequiredRAMMB {
			logger.Info("Insufficient Worker-Node capacity for queued server", map[string]interface{}{
				"server_id":            queuedServer.ServerID,
				"required_ram":         queuedServer.RequiredRAMMB,
				"worker_node_ram":      workerNodeRAM,
				"worker_node_count":    workerNodeCount,
				"queue_position":       1,
			})

			// Trigger scaling if enabled
			if c.ScalingEngine != nil && c.ScalingEngine.IsEnabled() {
				logger.Info("Queued servers waiting for Worker-Node capacity, scaling will be triggered in next cycle", map[string]interface{}{
					"queue_size":     c.StartQueue.Size(),
					"total_required": c.StartQueue.GetTotalRequiredRAM(),
				})
				// ScalingEngine will check and scale if needed in its next cycle (every 2 minutes)
			}

			break // Stop processing, wait for more capacity
		}

		// We have Worker-Node capacity - dequeue and signal that server can start
		server := c.StartQueue.Dequeue()

		// Safety check: Dequeue could return nil if queue was emptied by another goroutine
		if server == nil {
			logger.Warn("Dequeue returned nil (race condition), breaking queue processing", nil)
			break
		}

		logger.Info("Worker-Node capacity available for queued server", map[string]interface{}{
			"server_id":           server.ServerID,
			"server_name":         server.ServerName,
			"required_ram":        server.RequiredRAMMB,
			"worker_node_ram":     workerNodeRAM,
			"worker_node_count":   workerNodeCount,
			"wait_time":           time.Since(server.QueuedAt).String(),
		})

		// Start the server asynchronously
		if c.serverStarter != nil {
			go func(serverID string) {
				logger.Info("Starting queued server", map[string]interface{}{
					"server_id": serverID,
				})

				if err := c.serverStarter.StartServerFromQueue(serverID); err != nil {
					logger.Error("Failed to start queued server", err, map[string]interface{}{
						"server_id": serverID,
					})
					// NOTE: StartServerFromQueue() has ALREADY re-enqueued the server if needed
					// DO NOT re-queue here again, as it would trigger another ProcessStartQueue() cycle
					// This was causing the endless loop - double enqueue → double ProcessStartQueue() calls
				}
			}(server.ServerID)
		} else {
			logger.Warn("ServerStarter not configured, cannot start queued server", map[string]interface{}{
				"server_id": server.ServerID,
			})
		}

		// CRITICAL: Break after starting ONE server to prevent endless loop
		// The goroutine above is async and may re-queue the server if CPU-GUARD blocks
		// If we continue looping, we'll immediately dequeue the same server again → endless loop
		// The Periodic Worker (30s) will call ProcessStartQueue() again for the next server
		break
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

// TriggerScalingCheck triggers an immediate scaling evaluation
// This should be called when a new server is created, updated, or deleted
// to ensure capacity is provisioned without waiting for the next scaling interval
func (c *Conductor) TriggerScalingCheck() {
	if c.ScalingEngine != nil {
		c.ScalingEngine.TriggerImmediateCheck()
	} else {
		logger.Debug("Scaling check skipped (ScalingEngine not available)", nil)
	}
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

// startupDelayWorker triggers queue processing after the 2-minute startup delay expires
// This handles the edge case where servers are queued due to startup delay, but no other
// trigger fires when the delay period ends.
func (c *Conductor) startupDelayWorker() {
	// Calculate remaining time until delay expires
	elapsed := time.Since(c.StartedAt)
	delayDuration := 2 * time.Minute

	var timer *time.Timer
	if elapsed >= delayDuration {
		// Delay already expired - trigger immediately
		timer = time.NewTimer(0)
	} else {
		// Wait for remaining time
		remaining := delayDuration - elapsed
		timer = time.NewTimer(remaining)
	}
	defer timer.Stop()

	select {
	case <-timer.C:
		logger.Info("QUEUE-TRIGGER: Startup delay expired, processing queued servers", map[string]interface{}{
			"elapsed_seconds": int(time.Since(c.StartedAt).Seconds()),
		})
		c.ProcessStartQueue()
	case <-c.stopChan:
		logger.Info("Startup delay worker stopped", nil)
		return
	}
}

// periodicQueueWorker checks the queue every 30 seconds as a failsafe
// This ensures queued servers eventually get processed even if other triggers fail
func (c *Conductor) periodicQueueWorker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Only log if queue has items (avoid spam)
			if c.StartQueue.Size() > 0 {
				logger.Info("QUEUE-TRIGGER: Periodic check processing queue", map[string]interface{}{
					"queue_size": c.StartQueue.Size(),
				})
			}
			c.ProcessStartQueue()
		case <-c.stopChan:
			logger.Info("Periodic queue worker stopped", nil)
			return
		}
	}
}

// reservationTimeoutWorker checks for stale "starting" reservations every 5 minutes
// and automatically releases them after 30 minutes to prevent permanent deadlocks
func (c *Conductor) reservationTimeoutWorker() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	const reservationTimeout = 30 * time.Minute

	for {
		select {
		case <-ticker.C:
			c.cleanStaleReservations(reservationTimeout)
		case <-c.stopChan:
			logger.Info("Reservation timeout worker stopped", nil)
			return
		}
	}
}

// cleanStaleReservations finds and releases reservations that have been "starting" for too long
func (c *Conductor) cleanStaleReservations(timeout time.Duration) {
	c.ContainerRegistry.mu.Lock()
	defer c.ContainerRegistry.mu.Unlock()

	now := time.Now()
	staleReservations := []string{}

	// Find stale reservations
	for serverID, container := range c.ContainerRegistry.containers {
		if container.Status == "starting" {
			age := now.Sub(container.LastSeenAt)
			if age > timeout {
				staleReservations = append(staleReservations, serverID)
			}
		}
	}

	// Release stale reservations
	for _, serverID := range staleReservations {
		container := c.ContainerRegistry.containers[serverID]
		age := now.Sub(container.LastSeenAt)

		logger.Warn("RESERVATION-TIMEOUT: Releasing stale start reservation", map[string]interface{}{
			"server_id":   serverID,
			"server_name": container.ServerName,
			"ram_mb":      container.RAMMb,
			"age_minutes": int(age.Minutes()),
			"timeout":     int(timeout.Minutes()),
		})

		// Remove the reservation
		delete(c.ContainerRegistry.containers, serverID)

		// Note: We do NOT release RAM here because RAM is only allocated AFTER
		// the start slot is reserved, not during reservation itself.
		// The RAM release happens in ReleaseRAM() when the server actually fails to start.
	}

	// Only log if we found stale reservations (avoid spam)
	if len(staleReservations) > 0 {
		logger.Info("RESERVATION-TIMEOUT: Cleanup completed", map[string]interface{}{
			"released_count": len(staleReservations),
		})

		// Trigger queue processing - now that CPU guard is freed, queued servers can start
		go c.ProcessStartQueue()
	}
}

// SelectNodeForContainer selects the best node for a new container using the configured strategy
// Returns (nodeID, error)
// This is the Multi-Node equivalent of the old hardcoded "local-node" logic
func (c *Conductor) SelectNodeForContainer(requiredRAMMB int, strategy SelectionStrategy) (string, error) {
	// Use NodeSelector to find the best node
	nodeID, err := c.NodeSelector.SelectNode(requiredRAMMB, strategy)
	if err != nil {
		return "", err
	}

	logger.Info("Node selected for new container", map[string]interface{}{
		"node_id":      nodeID,
		"required_ram": requiredRAMMB,
		"strategy":     strategy,
	})

	return nodeID, nil
}

// SelectNodeForContainerAuto selects the best node using the recommended strategy
// This is a convenience method that automatically chooses the best strategy based on fleet composition
// Returns error if no worker nodes are available (caller should queue and provision)
func (c *Conductor) SelectNodeForContainerAuto(requiredRAMMB int) (string, error) {
	// First check if we have ANY worker nodes at all
	// If no worker nodes exist, we need to provision one before deployment
	if c.NodeSelector.GetWorkerNodeCount() == 0 {
		return "", fmt.Errorf("no worker nodes available - need to provision worker node first")
	}

	// Try to select a node with the recommended strategy
	recommendedStrategy := c.NodeSelector.GetRecommendedStrategy()
	nodeID, err := c.SelectNodeForContainer(requiredRAMMB, recommendedStrategy)

	// If selection failed due to capacity but we have worker nodes, return specific error
	// This allows the caller to distinguish between "need more capacity" vs "need first worker node"
	if err != nil && c.NodeSelector.GetWorkerNodeCount() > 0 {
		return "", fmt.Errorf("no worker nodes with sufficient capacity (%d MB required) - need to provision additional worker node", requiredRAMMB)
	}

	return nodeID, err
}

// GetNode retrieves node information by nodeID
// Used for proportional RAM calculations and node capacity checks
// Returns (interface{}, bool) where interface{} is *Node and bool indicates if node exists
func (c *Conductor) GetNode(nodeID string) (interface{}, bool) {
	node, exists := c.NodeRegistry.GetNode(nodeID)
	return node, exists
}

// GetRemoteNode builds a RemoteNode struct from a nodeID
// Returns (RemoteNode, error) - error if node not found or is the local node
func (c *Conductor) GetRemoteNode(nodeID string) (*docker.RemoteNode, error) {
	// Get node from registry
	node, exists := c.NodeRegistry.GetNode(nodeID)
	if !exists {
		return nil, fmt.Errorf("node %s not found in registry", nodeID)
	}

	// Check if this is the local node (no remote operations needed)
	if node.Type == "local" || node.IPAddress == "" || node.IPAddress == "localhost" || node.IPAddress == "127.0.0.1" {
		return nil, fmt.Errorf("node %s is local, remote operations not supported", nodeID)
	}

	// Build RemoteNode struct
	remoteNode := &docker.RemoteNode{
		ID:        node.ID,
		IPAddress: node.IPAddress,
		SSHUser:   node.SSHUser,
	}

	// Use default SSH user if not specified
	if remoteNode.SSHUser == "" {
		remoteNode.SSHUser = "root"
	}

	return remoteNode, nil
}

// GetRemoteDockerClient returns the RemoteDockerClient for remote node operations
func (c *Conductor) GetRemoteDockerClient() *docker.RemoteDockerClient {
	return c.RemoteClient
}

// RegisterContainer registers a container in the registry with node tracking
// This is used by MinecraftService to track which containers are running on which nodes
func (c *Conductor) RegisterContainer(serverID, serverName, containerID, nodeID string, ramMB, dockerPort, minecraftPort int, status string) {
	containerInfo := &ContainerInfo{
		ServerID:      serverID,
		ServerName:    serverName,
		ContainerID:   containerID,
		NodeID:        nodeID,
		RAMMb:         ramMB,
		Status:        status,
		DockerPort:    dockerPort,
		MinecraftPort: minecraftPort,
	}

	c.ContainerRegistry.RegisterContainer(containerInfo)

	// Publish container created event to dashboard
	events.PublishContainerCreated(serverID, serverName, nodeID, ramMB, minecraftPort, status)

	logger.Info("Container registered in registry", map[string]interface{}{
		"server_id":      serverID,
		"container_id":   containerID[:12],
		"node_id":        nodeID,
		"ram_mb":         ramMB,
		"minecraft_port": minecraftPort,
		"status":         status,
	})
}

// GetContainer retrieves container info including node assignment
// Returns (containerInfo, exists)
func (c *Conductor) GetContainer(serverID string) (interface{}, bool) {
	return c.ContainerRegistry.GetContainer(serverID)
}

// MigrateServer migrates a server from one node to another with minimal downtime (B8)
// This is the core of the ConsolidationPolicy - enables cost optimization via bin-packing
func (c *Conductor) MigrateServer(serverID, fromNodeID, toNodeID string, velocityClient VelocityRemoteClient) error {
	startTime := time.Now()
	operationID := fmt.Sprintf("migration-%s-%d", serverID[:8], time.Now().Unix())

	logger.Info("MIGRATION: Starting server migration", map[string]interface{}{
		"server_id":    serverID,
		"from_node":    fromNodeID,
		"to_node":      toNodeID,
		"operation_id": operationID,
	})

	// 1. Get container info
	container, exists := c.ContainerRegistry.GetContainer(serverID)
	if !exists {
		return fmt.Errorf("server %s not found in container registry", serverID)
	}

	// Verify current node matches
	if container.NodeID != fromNodeID {
		return fmt.Errorf("server %s is not on node %s (currently on %s)",
			serverID, fromNodeID, container.NodeID)
	}

	// Publish migration started event
	events.PublishMigrationStarted(operationID, serverID, container.ServerName, fromNodeID, toNodeID, container.RAMMb, 0)

	// 2. Unregister from Velocity (prevent new player connections during migration)
	events.PublishMigrationProgress(operationID, serverID, 10, "Unregistering from Velocity")
	if velocityClient != nil {
		logger.Info("MIGRATION: Unregistering from Velocity", map[string]interface{}{
			"server_name": container.ServerName,
		})

		if err := velocityClient.UnregisterServer(container.ServerName); err != nil {
			logger.Warn("MIGRATION: Failed to unregister from Velocity (continuing anyway)", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	// 3. Stop container on source node
	events.PublishMigrationProgress(operationID, serverID, 30, "Stopping container on source node")
	logger.Info("MIGRATION: Stopping container on source node", map[string]interface{}{
		"node_id":      fromNodeID,
		"container_id": container.ContainerID[:12],
	})

	// Get source node
	sourceNode, err := c.GetRemoteNode(fromNodeID)
	if err != nil {
		events.PublishMigrationFailed(operationID, serverID, fmt.Sprintf("Failed to get source node: %v", err))
		return fmt.Errorf("failed to get source node: %w", err)
	}

	// Stop container via remote client
	ctx := context.Background()
	if err := c.RemoteClient.StopContainer(ctx, sourceNode, container.ContainerID, 30); err != nil {
		events.PublishMigrationFailed(operationID, serverID, fmt.Sprintf("Failed to stop container: %v", err))
		return fmt.Errorf("failed to stop container: %w", err)
	}

	// Get current allocations for source node
	sourceContainerCount, sourceAllocatedRAM := c.ContainerRegistry.GetNodeAllocation(fromNodeID)

	// Update source node resources (decrement)
	events.PublishMigrationProgress(operationID, serverID, 60, "Updating node resources")
	c.NodeRegistry.UpdateNodeResources(fromNodeID, sourceContainerCount-1, sourceAllocatedRAM-container.RAMMb)

	// 4. Start container on target node
	events.PublishMigrationProgress(operationID, serverID, 80, "Preparing container on target node")
	logger.Info("MIGRATION: Starting container on target node", map[string]interface{}{
		"node_id": toNodeID,
	})

	// This requires MinecraftService to restart the server
	// We'll mark it as stopped in the registry and let the normal start flow handle it
	c.ContainerRegistry.RemoveContainer(serverID)

	// NOTE: The actual restart on the new node will be handled by MinecraftService
	// We just update the registry to reflect the migration intent
	// MinecraftService will see the server is stopped and start it on the new node via SelectNode()

	// 5. Re-register with Velocity on new node (will happen automatically when server starts)
	// MinecraftService will call velocityClient.RegisterServer() after successful start

	downtimeMs := time.Since(startTime).Milliseconds()

	logger.Info("MIGRATION: Server migration completed", map[string]interface{}{
		"server_id":   serverID,
		"from_node":   fromNodeID,
		"to_node":     toNodeID,
		"downtime_ms": downtimeMs,
	})

	// Publish migration completed event
	events.PublishMigrationCompleted(operationID, serverID)

	return nil
}

// cpuMetricsWorker collects CPU metrics from cloud nodes every 60 seconds
// and publishes NodeStatsEvents for dashboard visualization
func (c *Conductor) cpuMetricsWorker() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	// Perform initial collection after 10 seconds (let nodes settle)
	time.Sleep(10 * time.Second)
	c.collectCPUMetrics()

	for {
		select {
		case <-ticker.C:
			c.collectCPUMetrics()
		case <-c.stopChan:
			logger.Info("CPU metrics worker stopped", nil)
			return
		}
	}
}

// collectCPUMetrics collects CPU usage from all nodes and publishes stats events
func (c *Conductor) collectCPUMetrics() {
	nodes := c.NodeRegistry.GetAllNodes()

	for _, node := range nodes {
		var cpuUsage float64

		// Get CPU metrics based on node type
		if node.CloudProviderID != "" && c.CloudProvider != nil {
			// Cloud node - get metrics from Hetzner API
			cpu, err := c.CloudProvider.GetServerMetrics(node.CloudProviderID)
			if err != nil {
				logger.Warn("Failed to get CPU metrics from cloud provider", map[string]interface{}{
					"node_id": node.ID,
					"error":   err.Error(),
				})
				continue
			}
			cpuUsage = cpu
		} else {
			// Local/Dedicated node - use Docker stats (TODO: implement local CPU collection)
			// For now, skip local nodes
			continue
		}

		// Update node CPU in registry
		c.NodeRegistry.UpdateNodeCPU(node.ID, cpuUsage)

		// Publish NodeStatsEvent for dashboard
		containerCount, allocatedRAM := c.ContainerRegistry.GetNodeAllocation(node.ID)
		capacityPercent := 0.0
		if node.UsableRAMMB() > 0 {
			capacityPercent = (float64(allocatedRAM) / float64(node.UsableRAMMB())) * 100
		}

		events.PublishNodeStats(
			node.ID,
			allocatedRAM,
			node.AvailableRAMMB(),
			containerCount,
			capacityPercent,
			cpuUsage,
		)

		logger.Debug("CPU metrics collected", map[string]interface{}{
			"node_id":           node.ID,
			"cpu_usage_percent": cpuUsage,
		})
	}
}

// ghostContainerCleanupWorker periodically cleans up ghost containers from registry
// Runs every minute to remove containers that no longer exist in database
func (c *Conductor) ghostContainerCleanupWorker() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// Perform initial cleanup after 30 seconds (let system settle)
	time.Sleep(30 * time.Second)
	c.cleanupGhostContainers()

	for {
		select {
		case <-ticker.C:
			c.cleanupGhostContainers()
		case <-c.stopChan:
			logger.Info("Ghost container cleanup worker stopped", nil)
			return
		}
	}
}

// cleanupGhostContainers removes containers from registry that don't exist in database
func (c *Conductor) cleanupGhostContainers() {
	if c.ServerRepo == nil {
		logger.Warn("CLEANUP: ServerRepo not set, skipping ghost container cleanup", nil)
		return
	}

	removed := c.ContainerRegistry.CleanupGhostContainers(c.ServerRepo)

	if removed > 0 {
		logger.Info("CLEANUP: Ghost containers removed", map[string]interface{}{
			"removed_count": removed,
		})
	} else {
		logger.Debug("CLEANUP: No ghost containers found", nil)
	}
}

// SyncExistingWorkerNodes queries Hetzner API and registers all existing Worker-Nodes
// CRITICAL: Prevents infinite provisioning loop by recovering nodes after container restart
// If triggerScaling is false, no scaling check will be triggered (useful during startup sequence)
func (c *Conductor) SyncExistingWorkerNodes(triggerScaling bool) {
	logger.Info("WORKER-NODE-SYNC: Starting Worker-Node synchronization from Hetzner API", nil)

	// Query Hetzner for all PayPerPlay-managed cloud nodes
	labels := map[string]string{
		"managed_by": "payperplay",
		"type":       "cloud",
	}

	servers, err := c.CloudProvider.ListServers(labels)
	if err != nil {
		logger.Error("WORKER-NODE-SYNC: Failed to list servers from Hetzner", err, map[string]interface{}{
			"labels": labels,
		})
		return
	}

	if len(servers) == 0 {
		logger.Info("WORKER-NODE-SYNC: No existing Worker-Nodes found at Hetzner", nil)
		return
	}

	logger.Info("WORKER-NODE-SYNC: Found existing Worker-Nodes", map[string]interface{}{
		"count": len(servers),
	})

	cfg := config.AppConfig
	recoveredCount := 0
	skippedCount := 0

	for _, server := range servers {
		// Check if node is already registered (might happen if sync runs multiple times)
		if _, exists := c.NodeRegistry.GetNode(server.ID); exists {
			logger.Debug("WORKER-NODE-SYNC: Node already registered, skipping", map[string]interface{}{
				"node_id":   server.ID,
				"node_name": server.Name,
			})
			skippedCount++
			continue
		}

		// Get server type info for correct RAM values
		serverTypeInfo, err := c.ScalingEngine.vmProvisioner.getServerTypeInfo(server.Type)
		if err != nil {
			logger.Warn("WORKER-NODE-SYNC: Failed to get server type info, using fallback", map[string]interface{}{
				"server_id":   server.ID,
				"server_type": server.Type,
				"error":       err.Error(),
			})
			serverTypeInfo = &cloud.ServerType{
				Name:  server.Type,
				RAMMB: 4096, // Fallback
				Cores: 2,
			}
		}

		// Create Node object (matching VMProvisioner.ProvisionNode logic)
		now := time.Now()
		node := &Node{
			ID:               server.ID,
			Hostname:         server.Name,
			IPAddress:        server.IPAddress,
			Type:             "cloud",
			TotalRAMMB:       serverTypeInfo.RAMMB,
			TotalCPUCores:    serverTypeInfo.Cores,
			Status:           NodeStatusHealthy,           // DEPRECATED - use HealthStatus
			LifecycleState:   NodeStateReady,              // Recovered nodes start as ready (unknown history)
			HealthStatus:     HealthStatusUnknown,         // Will be checked by health checker
			Metrics: NodeLifecycleMetrics{
				ProvisionedAt:       now, // Use current time (don't have original creation time)
				InitializedAt:       &now, // Assume already initialized since it exists
				FirstContainerAt:    nil,
				LastContainerAt:     nil,
				TotalContainersEver: 0,
				CurrentContainers:   0,
			},
			LastHealthCheck:  time.Now(),
			ContainerCount:   0,
			AllocatedRAMMB:   0,
			DockerSocketPath: "/var/run/docker.sock",
			SSHUser:          "root",
			CreatedAt:        time.Now(), // We don't have the original creation time
			Labels: map[string]string{
				"type":       "cloud",
				"managed_by": "payperplay",
			},
			HourlyCostEUR:     server.HourlyCostEUR,
			CloudProviderID:   server.ID,
			IsSystemNode:      false, // Worker-Nodes are not system nodes
		}

		// Calculate system reserve (matching VMProvisioner logic)
		node.UpdateSystemReserve(cfg.SystemReservedRAMMB, cfg.SystemReservedRAMPercent)

		// Register node in NodeRegistry
		c.NodeRegistry.RegisterNode(node)

		logger.Info("WORKER-NODE-SYNC: Worker-Node recovered and registered", map[string]interface{}{
			"node_id":            node.ID,
			"node_name":          node.Hostname,
			"ip":                 node.IPAddress,
			"server_type":        server.Type,
			"total_ram_mb":       node.TotalRAMMB,
			"system_reserved_mb": node.SystemReservedRAMMB,
			"usable_ram_mb":      node.UsableRAMMB(),
			"cpu_cores":          node.TotalCPUCores,
			"cost_eur_hr":        node.HourlyCostEUR,
		})

		// Publish events (matching VMProvisioner logic)
		events.PublishNodeAdded(node.ID, node.Type)
		provider := "hetzner"
		location := "nbg1"
		if loc, ok := node.Labels["location"]; ok {
			location = loc
		}
		events.PublishNodeCreated(node.ID, node.Type, provider, location, string(node.Status), node.IPAddress, node.TotalRAMMB, node.UsableRAMMB(), node.IsSystemNode, node.CreatedAt)

		recoveredCount++
	}

	logger.Info("WORKER-NODE-SYNC: Worker-Node synchronization completed", map[string]interface{}{
		"recovered": recoveredCount,
		"skipped":   skippedCount,
		"total":     len(servers),
	})

	// After recovering nodes, trigger scaling check to assign queued servers (only if requested)
	if triggerScaling && recoveredCount > 0 {
		logger.Info("WORKER-NODE-SYNC: Triggering scaling check to assign queued servers", nil)
		c.TriggerScalingCheck()
	}
}

// SyncRemoteNodeContainers syncs running containers from all remote worker nodes
// Called after worker node sync to immediately discover containers on remote nodes
// Prevents capacity calculation errors after backend restarts
func (c *Conductor) SyncRemoteNodeContainers(serverRepo interface{}) {
	logger.Info("CONTAINER-SYNC: Detecting running containers on remote worker nodes...", nil)

	if c.RemoteClient == nil {
		logger.Warn("CONTAINER-SYNC: RemoteClient not initialized, skipping remote sync", nil)
		return
	}

	// Get all registered nodes
	c.NodeRegistry.mu.RLock()
	nodes := make([]*Node, 0, len(c.NodeRegistry.nodes))
	for _, node := range c.NodeRegistry.nodes {
		nodes = append(nodes, node)
	}
	c.NodeRegistry.mu.RUnlock()

	syncedCount := 0
	totalRAM := 0

	// Iterate through all nodes and sync containers
	for _, node := range nodes {
		// Skip system nodes (local-node, proxy-node)
		if node.IsSystemNode {
			continue
		}

		// Skip unhealthy nodes
		if node.Status != NodeStatusHealthy {
			logger.Info("CONTAINER-SYNC: Skipping unhealthy node", map[string]interface{}{
				"node_id": node.ID,
				"status":  node.Status,
			})
			continue
		}

		// List containers on this remote node
		ctx := context.Background()
		remoteNode := &docker.RemoteNode{
			ID:        node.ID,
			IPAddress: node.IPAddress,
			SSHUser:   node.SSHUser,
		}

		containers, err := c.RemoteClient.ListRunningContainers(ctx, remoteNode)
		if err != nil {
			logger.Warn("CONTAINER-SYNC: Failed to list containers on node", map[string]interface{}{
				"node_id": node.ID,
				"error":   err.Error(),
			})
			continue
		}

		if len(containers) == 0 {
			logger.Info("CONTAINER-SYNC: No containers on node", map[string]interface{}{
				"node_id": node.ID,
			})
			continue
		}

		logger.Info("CONTAINER-SYNC: Found containers on node", map[string]interface{}{
			"node_id": node.ID,
			"count":   len(containers),
		})

		// Sync each container
		for _, container := range containers {
			// Look up server in database to get RAM allocation
			serverVal := reflect.ValueOf(serverRepo)
			findMethod := serverVal.MethodByName("FindByID")
			if !findMethod.IsValid() {
				logger.Error("CONTAINER-SYNC: Repository missing FindByID method", nil, nil)
				continue
			}

			findResults := findMethod.Call([]reflect.Value{reflect.ValueOf(container.ServerID)})
			if len(findResults) != 2 || !findResults[1].IsNil() {
				logger.Warn("CONTAINER-SYNC: Container found but server not in DB", map[string]interface{}{
					"container": container.ContainerID[:12],
					"server_id": container.ServerID[:8],
					"node_id":   node.ID,
				})
				continue
			}

			server := findResults[0]
			if server.IsNil() {
				continue
			}

			// Get RAM allocation
			getRamMethod := server.MethodByName("GetRAMMb")
			if !getRamMethod.IsValid() {
				continue
			}

			ramResults := getRamMethod.Call(nil)
			if len(ramResults) != 1 {
				continue
			}

			ramMB := int(ramResults[0].Int())

			// Register container in Container Registry
			containerInfo := &ContainerInfo{
				ContainerID: container.ContainerID,
				ServerID:    container.ServerID,
				NodeID:      node.ID,
				RAMMb:       ramMB,
				Status:      "running",
			}
			c.ContainerRegistry.RegisterContainer(containerInfo)

			// Update node's RAM allocation
			c.NodeRegistry.mu.Lock()
			if n, exists := c.NodeRegistry.nodes[node.ID]; exists {
				n.AllocatedRAMMB += ramMB
				n.ContainerCount++
			}
			c.NodeRegistry.mu.Unlock()

			totalRAM += ramMB
			syncedCount++

			logger.Info("CONTAINER-SYNC: Container synced", map[string]interface{}{
				"container": container.ContainerID[:12],
				"server":    container.ServerID[:8],
				"node_id":   node.ID,
				"ram_mb":    ramMB,
			})
		}
	}

	if syncedCount > 0 {
		logger.Info("CONTAINER-SYNC: Remote container synchronization completed", map[string]interface{}{
			"synced_containers": syncedCount,
			"total_ram_mb":      totalRAM,
		})
	} else {
		logger.Info("CONTAINER-SYNC: No containers found on remote nodes (clean state)", nil)
	}
}

// VelocityRemoteClient interface for Velocity integration (dependency injection)
type VelocityRemoteClient interface {
	RegisterServer(name, address string) error
	UnregisterServer(name string) error
	GetPlayerCount(serverName string) (int, error)
}
