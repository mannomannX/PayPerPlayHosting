package conductor

import (
	"time"

	"github.com/payperplay/hosting/internal/cloud"
	"github.com/payperplay/hosting/pkg/logger"
)

// Conductor is the central fleet orchestrator
type Conductor struct {
	NodeRegistry      *NodeRegistry
	ContainerRegistry *ContainerRegistry
	HealthChecker     *HealthChecker
	ScalingEngine     *ScalingEngine // B5 - Auto-Scaling
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
	}
}

// InitializeScaling initializes the scaling engine with a cloud provider
// This is called after conductor creation once cloud credentials are available
func (c *Conductor) InitializeScaling(cloudProvider cloud.CloudProvider, sshKeyName string) {
	if c.ScalingEngine != nil {
		logger.Warn("Scaling engine already initialized", nil)
		return
	}

	vmProvisioner := NewVMProvisioner(cloudProvider, c.NodeRegistry, sshKeyName)
	c.ScalingEngine = NewScalingEngine(cloudProvider, vmProvisioner, c.NodeRegistry)

	logger.Info("Scaling engine initialized", map[string]interface{}{
		"ssh_key": sshKeyName,
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
	localNode := &Node{
		ID:               "local-node",
		Hostname:         "localhost",
		IPAddress:        "127.0.0.1",
		Type:             "dedicated",
		TotalRAMMB:       65536, // 64GB - adjust based on actual server
		TotalCPUCores:    16,    // Adjust based on actual server
		Status:           NodeStatusUnknown,
		LastHealthCheck:  time.Now(),
		ContainerCount:   0,
		AllocatedRAMMB:   0,
		DockerSocketPath: "/var/run/docker.sock",
		SSHUser:          "root",
	}

	c.NodeRegistry.RegisterNode(localNode)

	logger.Info("Local node registered", map[string]interface{}{
		"node_id":       localNode.ID,
		"total_ram_mb":  localNode.TotalRAMMB,
		"total_cpu":     localNode.TotalCPUCores,
	})
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
}
