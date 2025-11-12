package conductor

import (
	"fmt"
	"time"

	"github.com/payperplay/hosting/internal/cloud"
	"github.com/payperplay/hosting/internal/events"
	"github.com/payperplay/hosting/pkg/config"
	"github.com/payperplay/hosting/pkg/logger"
)

// VMProvisioner handles automated VM provisioning and setup
type VMProvisioner struct {
	cloudProvider cloud.CloudProvider
	nodeRegistry  *NodeRegistry
	sshKeyName    string // SSH key configured in cloud provider
	agentVersion  string // PayPerPlay agent version to install
}

// NewVMProvisioner creates a new VM provisioner
func NewVMProvisioner(cloudProvider cloud.CloudProvider, nodeRegistry *NodeRegistry, sshKeyName string) *VMProvisioner {
	return &VMProvisioner{
		cloudProvider: cloudProvider,
		nodeRegistry:  nodeRegistry,
		sshKeyName:    sshKeyName,
		agentVersion:  "latest", // TODO: Make configurable
	}
}

// ProvisionNode creates a new cloud node with Docker and PayPerPlay agent installed
func (p *VMProvisioner) ProvisionNode(serverType string) (*Node, error) {
	logger.Info("Starting VM provisioning", map[string]interface{}{
		"server_type": serverType,
	})

	// Generate unique name
	nodeName := fmt.Sprintf("payperplay-node-%d", time.Now().Unix())

	// Generate Cloud-Init script
	cloudInit := p.generateCloudInit()

	// Create server specification
	spec := cloud.ServerSpec{
		Name:      nodeName,
		Type:      serverType,
		Image:     "ubuntu-22.04", // LTS version
		Location:  "nbg1",          // Nuremberg, Germany (default)
		CloudInit: cloudInit,
		Labels: map[string]string{
			"managed_by": "payperplay",
			"type":       "cloud", // vs "dedicated"
			"created_at": fmt.Sprintf("%d", time.Now().Unix()), // Unix timestamp - Hetzner-compliant
		},
		SSHKeys: []string{p.sshKeyName},
	}

	// Create server via cloud provider
	server, err := p.cloudProvider.CreateServer(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to create server: %w", err)
	}

	logger.Info("Server created, waiting for ready", map[string]interface{}{
		"server_id": server.ID,
		"ip":        server.IPAddress,
	})

	// Wait for server to be ready
	if err := p.cloudProvider.WaitForServerReady(server.ID, 5*time.Minute); err != nil {
		// Cleanup on failure
		p.cloudProvider.DeleteServer(server.ID)
		return nil, fmt.Errorf("server failed to become ready: %w", err)
	}

	// Get server type info for resource allocation
	// Note: GetServerType(name) fails with 404, so we get all types and find the right one
	serverTypeInfo, err := p.getServerTypeInfo(server.Type)
	if err != nil {
		logger.Warn("Failed to get server type info", map[string]interface{}{
			"server_type": server.Type,
			"error":       err.Error(),
		})
		serverTypeInfo = &cloud.ServerType{
			Name:   server.Type,
			RAMMB:  4096, // Fallback default
			Cores:  2,
		}
	}

	// Wait for Cloud-Init to complete (Docker + Agent installation)
	logger.Info("Waiting for Cloud-Init to complete", map[string]interface{}{
		"server_id": server.ID,
	})
	time.Sleep(2 * time.Minute) // Cloud-Init typically takes 1-2 minutes

	// Create Node object
	cfg := config.AppConfig
	node := &Node{
		ID:               server.ID,
		Hostname:         server.Name,
		IPAddress:        server.IPAddress,
		Type:             "cloud", // vs "dedicated"
		TotalRAMMB:       serverTypeInfo.RAMMB,
		TotalCPUCores:    serverTypeInfo.Cores,
		Status:           NodeStatusHealthy,
		LastHealthCheck:  time.Now(),
		ContainerCount:   0,
		AllocatedRAMMB:   0,
		DockerSocketPath: "/var/run/docker.sock",
		SSHUser:          "root",
		CreatedAt:        time.Now(),
		Labels: map[string]string{
			"type":       "cloud",
			"managed_by": "payperplay",
		},
		HourlyCostEUR: server.HourlyCostEUR,
	}

	// Calculate intelligent system reserve for cloud node (3-tier strategy)
	node.UpdateSystemReserve(cfg.SystemReservedRAMMB, cfg.SystemReservedRAMPercent)

	// Register node in NodeRegistry
	p.nodeRegistry.RegisterNode(node)

	logger.Info("Cloud node provisioned with intelligent system reserve", map[string]interface{}{
		"node_id":            node.ID,
		"ip":                 node.IPAddress,
		"total_ram_mb":       node.TotalRAMMB,
		"system_reserved_mb": node.SystemReservedRAMMB,
		"usable_ram_mb":      node.UsableRAMMB(),
		"cpu_cores":          node.TotalCPUCores,
		"cost_eur_hr":        node.HourlyCostEUR,
	})

	// Publish event (old and new)
	events.PublishNodeAdded(node.ID, node.Type)
	// Provider and location are derived from cloud provider or labels
	provider := "hetzner" // TODO: Get from cloud provider
	location := "nbg1"    // Default location for now
	if loc, ok := node.Labels["location"]; ok {
		location = loc
	}
	events.PublishNodeCreated(node.ID, node.Type, provider, location, string(node.Status), node.IPAddress, node.TotalRAMMB, node.UsableRAMMB(), node.IsSystemNode, node.CreatedAt)

	return node, nil
}

// DecommissionNode removes a cloud node
func (p *VMProvisioner) DecommissionNode(nodeID string) error {
	logger.Info("Decommissioning node", map[string]interface{}{
		"node_id": nodeID,
	})

	// Get node from registry
	node, exists := p.nodeRegistry.GetNode(nodeID)
	if !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	// Only decommission cloud nodes (never dedicated nodes)
	if node.Type != "cloud" {
		return fmt.Errorf("cannot decommission dedicated node: %s", nodeID)
	}

	// Check if node has containers
	if node.ContainerCount > 0 {
		return fmt.Errorf("cannot decommission node with active containers: %s (count: %d)", nodeID, node.ContainerCount)
	}

	// Delete server via cloud provider
	if err := p.cloudProvider.DeleteServer(nodeID); err != nil {
		return fmt.Errorf("failed to delete server: %w", err)
	}

	// Unregister from NodeRegistry
	p.nodeRegistry.UnregisterNode(nodeID)

	logger.Info("Node decommissioned successfully", map[string]interface{}{
		"node_id": nodeID,
	})

	// Publish event
	events.PublishNodeRemoved(nodeID, "decommissioned")

	return nil
}

// generateCloudInit generates the Cloud-Init script for VM setup
func (p *VMProvisioner) generateCloudInit() string {
	return `#cloud-config
package_update: true
package_upgrade: true

packages:
  - docker.io
  - docker-compose
  - curl
  - git

runcmd:
  # Enable and start Docker
  - systemctl enable docker
  - systemctl start docker

  # Add root to docker group (for convenience)
  - usermod -aG docker root

  # Configure Docker daemon for better performance
  - |
    cat > /etc/docker/daemon.json <<EOF
    {
      "log-driver": "json-file",
      "log-opts": {
        "max-size": "10m",
        "max-file": "3"
      },
      "storage-driver": "overlay2"
    }
    EOF
  - systemctl restart docker

  # Download and install PayPerPlay Agent (TODO: implement)
  # - curl -sSL https://install.payperplay.host/agent.sh | bash

  # Configure firewall (allow Docker + SSH)
  - ufw allow 22/tcp
  - ufw allow 2375/tcp  # Docker API (for remote management)
  - ufw allow 25565/tcp # Minecraft default port
  - ufw allow 25565-25600/tcp # Minecraft port range
  - ufw --force enable

  # Mark Cloud-Init as complete
  - touch /var/lib/cloud/instance/boot-finished

write_files:
  - path: /etc/payperplay/node.conf
    content: |
      NODE_TYPE=cloud
      MANAGED_BY=payperplay
      PROVISIONED_AT=` + fmt.Sprintf("%d", time.Now().Unix()) + `
    owner: root:root
    permissions: '0644'

final_message: "PayPerPlay node is ready after $UPTIME seconds"
`
}

// ProvisionSpareNode creates a pre-configured spare node (for B6 - Hot-Spare Pool)
func (p *VMProvisioner) ProvisionSpareNode() (*Node, error) {
	// Use smallest server type for spares
	return p.ProvisionNode("cx11") // 1 vCPU, 2GB RAM, cheapest option
}

// CreateNodeSnapshot creates a snapshot of a node (for B6 - Hot-Spare Pool)
func (p *VMProvisioner) CreateNodeSnapshot(nodeID string) (*cloud.Snapshot, error) {
	logger.Info("Creating node snapshot", map[string]interface{}{
		"node_id": nodeID,
	})

	description := fmt.Sprintf("PayPerPlay node snapshot - %s", time.Now().Format(time.RFC3339))
	snapshot, err := p.cloudProvider.CreateSnapshot(nodeID, description)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	logger.Info("Node snapshot created", map[string]interface{}{
		"node_id":     nodeID,
		"snapshot_id": snapshot.ID,
	})

	return snapshot, nil
}

// ProvisionNodeFromSnapshot creates a new node from a snapshot (for B6 - Hot-Spare Pool)
func (p *VMProvisioner) ProvisionNodeFromSnapshot(snapshotID string, serverType string) (*Node, error) {
	logger.Info("Provisioning node from snapshot", map[string]interface{}{
		"snapshot_id": snapshotID,
		"server_type": serverType,
	})

	nodeName := fmt.Sprintf("payperplay-node-%d", time.Now().Unix())

	spec := cloud.ServerSpec{
		Name:     nodeName,
		Type:     serverType,
		Location: "nbg1",
		Labels: map[string]string{
			"managed_by":    "payperplay",
			"type":          "cloud",
			"from_snapshot": "true",
			"created_at":    fmt.Sprintf("%d", time.Now().Unix()), // Unix timestamp - Hetzner-compliant
		},
		SSHKeys: []string{p.sshKeyName},
	}

	// Create from snapshot (provider will use snapshot as image)
	server, err := p.cloudProvider.CreateServerFromSnapshot(snapshotID, spec)
	if err != nil {
		return nil, fmt.Errorf("failed to create server from snapshot: %w", err)
	}

	// Wait for ready
	if err := p.cloudProvider.WaitForServerReady(server.ID, 3*time.Minute); err != nil {
		p.cloudProvider.DeleteServer(server.ID)
		return nil, fmt.Errorf("server failed to become ready: %w", err)
	}

	// Get server type info
	serverTypeInfo, err := p.cloudProvider.GetServerType(server.Type)
	if err != nil {
		serverTypeInfo = &cloud.ServerType{
			RAMMB: 4096,
			Cores: 2,
		}
	}

	// Create Node
	node := &Node{
		ID:               server.ID,
		Hostname:         server.Name,
		IPAddress:        server.IPAddress,
		Type:             "cloud",
		TotalRAMMB:       serverTypeInfo.RAMMB,
		TotalCPUCores:    serverTypeInfo.Cores,
		Status:           NodeStatusHealthy,
		LastHealthCheck:  time.Now(),
		ContainerCount:   0,
		AllocatedRAMMB:   0,
		DockerSocketPath: "/var/run/docker.sock",
		SSHUser:          "root",
		CreatedAt:        time.Now(),
		Labels: map[string]string{
			"type":          "cloud",
			"from_snapshot": "true",
		},
		HourlyCostEUR: server.HourlyCostEUR,
	}

	p.nodeRegistry.RegisterNode(node)

	logger.Info("Node provisioned from snapshot", map[string]interface{}{
		"node_id":     node.ID,
		"snapshot_id": snapshotID,
	})

	events.PublishNodeAdded(node.ID, "cloud-from-snapshot")

	return node, nil
}

// getServerTypeInfo gets server type info from cloud provider by searching all types
// This is needed because GetServerType(name) fails with 404 (Hetzner API expects ID, not name)
func (p *VMProvisioner) getServerTypeInfo(typeName string) (*cloud.ServerType, error) {
	// Get all server types from API
	allTypes, err := p.cloudProvider.GetServerTypes()
	if err != nil {
		return nil, fmt.Errorf("failed to get server types: %w", err)
	}

	// Find the matching type by name
	for _, st := range allTypes {
		if st.Name == typeName {
			return st, nil
		}
	}

	return nil, fmt.Errorf("server type %s not found", typeName)
}
