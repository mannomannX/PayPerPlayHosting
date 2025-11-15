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
	cloudProvider  cloud.CloudProvider
	conductor *Conductor // Reference to parent conductor for audit logging
	nodeRegistry   *NodeRegistry
	debugLogBuffer *DebugLogBuffer
	sshKeyName     string // SSH key configured in cloud provider
	agentVersion   string // PayPerPlay agent version to install
}

// NewVMProvisioner creates a new VM provisioner
func NewVMProvisioner(cloudProvider cloud.CloudProvider, nodeRegistry *NodeRegistry, debugLogBuffer *DebugLogBuffer, sshKeyName string) *VMProvisioner {
	return &VMProvisioner{
		cloudProvider:  cloudProvider,
		nodeRegistry:   nodeRegistry,
		debugLogBuffer: debugLogBuffer,
		sshKeyName:     sshKeyName,
		agentVersion:   "latest", // TODO: Make configurable
	}
}

// ProvisionNode creates a new cloud node with Docker and PayPerPlay agent installed
func (p *VMProvisioner) ProvisionNode(serverType string) (*Node, error) {
	logger.Info("Starting VM provisioning", map[string]interface{}{
		"server_type": serverType,
	})

	// CRITICAL FIX: Create placeholder node IMMEDIATELY to prevent duplicate provisioning
	// This ensures the next scaling cycle sees "Worker-Node exists (provisioning)" instead of "0 nodes"
	cfg := config.AppConfig
	placeholderID := fmt.Sprintf("provisioning-%d", time.Now().UnixNano())

	// Estimate RAM based on server type (common Hetzner types)
	// This prevents the placeholder from being invisible due to TotalRAMMB=0
	estimatedRAM := map[string]int{
		"cx11":  2048,  // 2 GB
		"cpx11": 2048,  // 2 GB
		"cx22":  4096,  // 4 GB
		"cpx22": 4096,  // 4 GB
		"cx32":  8192,  // 8 GB
		"cpx32": 8192,  // 8 GB
		"cx42":  16384, // 16 GB
		"cpx42": 16384, // 16 GB
		"cx52":  32768, // 32 GB
		"cpx52": 32768, // 32 GB
	}
	ramMB := estimatedRAM[serverType]
	if ramMB == 0 {
		ramMB = 8192 // Default fallback: 8GB
	}

	placeholderNode := &Node{
		ID:               placeholderID,
		Hostname:         fmt.Sprintf("provisioning-node-%d", time.Now().Unix()),
		IPAddress:        "0.0.0.0", // Temporary IP until server is created
		Type:             "cloud",
		TotalRAMMB:       ramMB, // CRITICAL: Set estimated RAM so node counts towards capacity!
		TotalCPUCores:    0,
		Status:           NodeStatusUnhealthy, // Unhealthy until fully provisioned
		LastHealthCheck:  time.Now(),
		ContainerCount:   0,
		AllocatedRAMMB:   0,
		DockerSocketPath: "/var/run/docker.sock",
		SSHUser:          "root",
		CreatedAt:        time.Now(),
		Labels: map[string]string{
			"type":          "cloud",
			"managed_by":    "payperplay",
			"status":        "provisioning", // Special label to indicate provisioning in progress
			"auto_scalable": "true",         // Auto-provisioned nodes can be scaled down
		},
		HourlyCostEUR: 0,
	}

	// Register placeholder immediately BEFORE starting slow Hetzner API calls
	p.nodeRegistry.RegisterNode(placeholderNode)

	fields := map[string]interface{}{
		"placeholder_id": placeholderID,
		"server_type":    serverType,
	}
	logger.Info("Placeholder node registered, starting Hetzner provisioning", fields)

	// Add to debug log buffer for dashboard
	if p.debugLogBuffer != nil {
		p.debugLogBuffer.Add("INFO", fmt.Sprintf("Provisioning Worker-Node (%s)", serverType), fields)
	}

	// Get Ubuntu 22.04 image ID from Hetzner API
	imageID, err := p.cloudProvider.GetUbuntuImage("22.04")
	if err != nil {
		// Cleanup: Remove placeholder on failure
		p.nodeRegistry.UnregisterNode(placeholderID)
		return nil, fmt.Errorf("failed to get Ubuntu image: %w", err)
	}

	// Generate unique name
	nodeName := fmt.Sprintf("payperplay-node-%d", time.Now().Unix())

	// Generate Cloud-Init script
	cloudInit := p.generateCloudInit()

	// Create server specification
	spec := cloud.ServerSpec{
		Name:      nodeName,
		Type:      serverType,
		Image:     imageID, // Ubuntu 22.04 LTS (retrieved from API)
		Location:  "nbg1",  // Nuremberg, Germany (default)
		CloudInit: cloudInit,
		Labels: map[string]string{
			"managed_by":    "payperplay",
			"type":          "cloud", // vs "dedicated"
			"created_at":    fmt.Sprintf("%d", time.Now().Unix()), // Unix timestamp - Hetzner-compliant
			"auto_scalable": "true",  // Auto-provisioned nodes can be scaled down
		},
		SSHKeys: []string{p.sshKeyName},
	}

	// Create server via cloud provider (THIS TAKES ~20 SECONDS!)
	server, err := p.cloudProvider.CreateServer(spec)
	if err != nil {
		// Cleanup: Remove placeholder on failure
		p.nodeRegistry.UnregisterNode(placeholderID)
		return nil, fmt.Errorf("failed to create server: %w", err)
	}

	logger.Info("Server created, waiting for ready", map[string]interface{}{
		"server_id": server.ID,
		"ip":        server.IPAddress,
	})

	// Wait for server to be ready
	if err := p.cloudProvider.WaitForServerReady(server.ID, 5*time.Minute); err != nil {
		// Cleanup on failure: Remove placeholder AND delete Hetzner server
		p.nodeRegistry.UnregisterNode(placeholderID)
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

	// Server successfully created! Now replace placeholder with real node
	// Remove placeholder first
	p.nodeRegistry.UnregisterNode(placeholderID)

	fields2 := map[string]interface{}{
		"placeholder_id": placeholderID,
		"server_id":      server.ID,
		"ip":             server.IPAddress,
	}
	logger.Info("Hetzner server created, replacing placeholder with real node", fields2)

	// Add to debug log buffer for dashboard
	if p.debugLogBuffer != nil {
		p.debugLogBuffer.Add("INFO", fmt.Sprintf("Worker-Node created: %s (%s)", server.IPAddress, server.Name), fields2)
	}

	// Create real Node object with Hetzner server details
	now := time.Now()
	node := &Node{
		ID:               server.ID,
		Hostname:         server.Name,
		IPAddress:        server.IPAddress,
		Type:             "cloud", // vs "dedicated"
		TotalRAMMB:       serverTypeInfo.RAMMB,
		TotalCPUCores:    serverTypeInfo.Cores,
		Status:           NodeStatusUnhealthy, // DEPRECATED - use HealthStatus
		LifecycleState:   NodeStateProvisioning, // NEW: Start in provisioning state
		HealthStatus:     HealthStatusUnknown,   // NEW: Unknown until health checked
		Metrics: NodeLifecycleMetrics{
			ProvisionedAt:       now,
			InitializedAt:       nil, // Set when Cloud-Init completes
			FirstContainerAt:    nil,
			LastContainerAt:     nil,
			TotalContainersEver: 0,
			CurrentContainers:   0,
		},
		LastHealthCheck:  now,
		ContainerCount:   0,
		AllocatedRAMMB:   0,
		DockerSocketPath: "/var/run/docker.sock",
		SSHUser:          "root",
		CreatedAt:        now,
		Labels: map[string]string{
			"type":          "cloud",
			"managed_by":    "payperplay",
			"auto_scalable": "true", // Auto-provisioned nodes can be scaled down
		},
		HourlyCostEUR: server.HourlyCostEUR,
	}

	// Calculate intelligent system reserve for cloud node (3-tier strategy)
	node.UpdateSystemReserve(cfg.SystemReservedRAMMB, cfg.SystemReservedRAMPercent)

	// Register real node as unhealthy (will become healthy after Cloud-Init)
	p.nodeRegistry.RegisterNode(node)

	logger.Info("Node registered as unhealthy, waiting for Cloud-Init", map[string]interface{}{
		"node_id":   node.ID,
		"ip":        node.IPAddress,
		"status":    "unhealthy",
		"wait_time": "2 minutes",
	})

	// Wait for Cloud-Init to complete (Docker + Agent installation)
	// Node is already registered but unhealthy - will be marked healthy after this
	logger.Info("Waiting for Cloud-Init to complete", map[string]interface{}{
		"server_id": server.ID,
	})
	time.Sleep(2 * time.Minute) // Cloud-Init typically takes 1-2 minutes

	// Mark node as ready now that Cloud-Init is complete
	initTime := time.Now()
	node.Status = NodeStatusHealthy // DEPRECATED
	node.HealthStatus = HealthStatusHealthy
	node.LastHealthCheck = initTime
	node.Metrics.InitializedAt = &initTime

	// Transition from provisioning → initializing → ready
	node.TransitionLifecycleState(NodeStateInitializing, "cloud_init_started")
	node.TransitionLifecycleState(NodeStateReady, "cloud_init_completed")

	// Re-register node to ensure status update is reflected in registry
	// (Even though we store pointers, explicit re-registration ensures consistency)
	p.nodeRegistry.RegisterNode(node)

	logger.Info("Node marked as HEALTHY after Cloud-Init", map[string]interface{}{
		"node_id": node.ID,
		"status":  "healthy",
	})

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

// DecommissionNode removes a cloud node with safety checks
func (p *VMProvisioner) DecommissionNode(nodeID string, decisionBy string) error {
	logger.Info("Attempting to decommission node", map[string]interface{}{
		"node_id":     nodeID,
		"decision_by": decisionBy,
	})

	// Get node from registry
	node, exists := p.nodeRegistry.GetNode(nodeID)
	if !exists {
		err := fmt.Errorf("node not found: %s", nodeID)
		if p.conductor != nil && p.conductor.AuditLog != nil {
			p.conductor.AuditLog.RecordNodeDecommission(nodeID, "node_not_found", decisionBy, nil, "failed", err)
		}
		return err
	}

	// Only decommission cloud nodes (never dedicated nodes)
	if node.Type != "cloud" {
		err := fmt.Errorf("cannot decommission dedicated node: %s", nodeID)
		if p.conductor != nil && p.conductor.AuditLog != nil {
			p.conductor.AuditLog.RecordNodeDecommission(nodeID, "not_cloud_node", decisionBy, map[string]interface{}{"type": node.Type}, "rejected", err)
		}
		return err
	}

	// CRITICAL: Only decommission auto-scalable nodes (never pre-existing/manual nodes)
	// Pre-existing nodes (recovered from Hetzner on startup) have auto_scalable="false"
	// Auto-provisioned nodes (created by scaling engine) have auto_scalable="true"
	if autoScalable, exists := node.Labels["auto_scalable"]; !exists || autoScalable != "true" {
		err := fmt.Errorf("cannot decommission non-auto-scalable node: %s (auto_scalable=%s)", nodeID, autoScalable)
		logger.Warn("Decommission rejected - node is not auto-scalable", map[string]interface{}{
			"node_id":       nodeID,
			"auto_scalable": autoScalable,
			"reason":        "Pre-existing or manual nodes should not be auto-scaled",
		})
		if p.conductor != nil && p.conductor.AuditLog != nil {
			p.conductor.AuditLog.RecordNodeDecommission(nodeID, "not_auto_scalable", decisionBy, map[string]interface{}{
				"auto_scalable": autoScalable,
				"labels":        node.Labels,
			}, "rejected", err)
		}
		return err
	}

	// CRITICAL: Safety check using new lifecycle system
	canDecommission, reason := node.CanBeDecommissioned()
	if !canDecommission {
		err := fmt.Errorf("safety check failed: %s", reason)

		// Audit log the rejection
		if p.conductor != nil && p.conductor.AuditLog != nil {
			snapshot := map[string]interface{}{
				"lifecycle_state":       node.LifecycleState,
				"health_status":         node.HealthStatus,
				"container_count":       node.ContainerCount,
				"allocated_ram_mb":      node.AllocatedRAMMB,
				"total_containers_ever": node.Metrics.TotalContainersEver,
				"age_minutes":           time.Since(node.CreatedAt).Minutes(),
			}
			if node.Metrics.InitializedAt != nil {
				snapshot["initialized_age_minutes"] = time.Since(*node.Metrics.InitializedAt).Minutes()
			}

			p.conductor.AuditLog.RecordNodeDecommission(nodeID, reason, decisionBy, snapshot, "rejected", err)
		}

		logger.Warn("Decommission rejected by safety check", map[string]interface{}{
			"node_id": nodeID,
			"reason":  reason,
			"lifecycle_state": node.LifecycleState,
			"containers": node.ContainerCount,
		})

		return err
	}

	// Capture state snapshot for audit log
	stateSnapshot := map[string]interface{}{
		"lifecycle_state":       node.LifecycleState,
		"health_status":         node.HealthStatus,
		"container_count":       node.ContainerCount,
		"allocated_ram_mb":      node.AllocatedRAMMB,
		"total_containers_ever": node.Metrics.TotalContainersEver,
		"age_minutes":           time.Since(node.CreatedAt).Minutes(),
		"hourly_cost_eur":       node.HourlyCostEUR,
	}
	if node.Metrics.FirstContainerAt != nil {
		stateSnapshot["first_container_at"] = node.Metrics.FirstContainerAt
	}
	if node.Metrics.LastContainerAt != nil {
		stateSnapshot["last_container_at"] = node.Metrics.LastContainerAt
		stateSnapshot["idle_minutes"] = time.Since(*node.Metrics.LastContainerAt).Minutes()
	}

	// Transition to draining state before decommission
	if err := node.TransitionLifecycleState(NodeStateDraining, "decommission_requested"); err != nil {
		logger.Warn("Failed to transition to draining state", map[string]interface{}{
			"node_id": nodeID,
			"error":   err.Error(),
		})
		// Continue anyway - not critical
	}

	// Delete server via cloud provider
	if err := p.cloudProvider.DeleteServer(nodeID); err != nil {
		deleteErr := fmt.Errorf("failed to delete server: %w", err)
		if p.conductor != nil && p.conductor.AuditLog != nil {
			p.conductor.AuditLog.RecordNodeDecommission(nodeID, reason, decisionBy, stateSnapshot, "failed", deleteErr)
		}
		return deleteErr
	}

	// Transition to decommissioned state
	if err := node.TransitionLifecycleState(NodeStateDecommissioned, "hetzner_delete_success"); err != nil {
		logger.Warn("Failed to transition to decommissioned state", map[string]interface{}{
			"node_id": nodeID,
			"error":   err.Error(),
		})
	}

	// Unregister from NodeRegistry
	p.nodeRegistry.UnregisterNode(nodeID)

	// Audit log success
	if p.conductor != nil && p.conductor.AuditLog != nil {
		p.conductor.AuditLog.RecordNodeDecommission(nodeID, reason, decisionBy, stateSnapshot, "success", nil)
	}

	logger.Info("Node decommissioned successfully", map[string]interface{}{
		"node_id":     nodeID,
		"reason":      reason,
		"decision_by": decisionBy,
		"cost_saved":  fmt.Sprintf("€%.4f/hour", node.HourlyCostEUR),
	})

	// Publish event
	events.PublishNodeRemoved(nodeID, "decommissioned")

	return nil
}

// generateCloudInit generates the Cloud-Init script for VM setup
func (p *VMProvisioner) generateCloudInit() string {
	// CRITICAL: Add conductor's public SSH key to allow health checks
	// This is read from /root/.ssh/id_rsa.pub on the conductor node
	conductorPubKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDfaN2p3gNtatuhvad5b6JVkr05UVmELZl9KzI84Q/8xQQxOmmSI4N7Vy48n03t9xJlbbztyXa2aE1loxZ3GxKdh9kokyavvDxSB7UebeZTOH/A/UkOiruh9Nq47rACtvTgFS/QNRe4IfeswSHsRcAWVALz5rkZ53FfLd9JwgHwazeBf6avT5fcRxJ5NdQ8iDTtvuKZ81mwRoDVq4Q61uy5NGdeILDfWxUqX3N0WXOSmbEO0LqPsp4fb6I1GyT/9C/rC3JNrb2iD51AtAlAoMKg8y1dzyvJHh1TSBL6xPn0EavyzqFLW0ignvX8aLwKB0NIwrPsbEgOgqKknbBlsudAJxic/wS1mSjDjJl8SDY1VaDJo9n0uW4T2KyvPEovsCOyXFXd5Vnl/VQ4YdmdInuM+27+CnD1RGOJhuOA1TXvG2DIGzZe81adTCZS+kZwE7d6E2JCnYBpurUTZfsQVNJVy0+SjnoDlT0qnS1I+Mx361e6+YSFvJAPGDOF7jdUlK4Jwi0sz4zIWgOKGjpA8uITaXN/Qkv8M2v3FJ3EHeijxKPo/5W0nrJXyfMcn+qewuywuLSSjsphr1oy3+nVKIBJghmjvaeE4GAaXdbgHQEQ9E/+Azdk49ipiSsGfBytLXTIOlh4QjXzeQNxSn8i4FfjFJ9xHAquKNUBGsrv9nAcfQ== payperplay-conductor"

	return `#cloud-config
package_update: true
package_upgrade: true

packages:
  - docker.io
  - docker-compose
  - curl
  - git

# CRITICAL: Add conductor's SSH public key for health checks
ssh_authorized_keys:
  - ` + conductorPubKey + `

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

  # Configure firewall (allow SSH + Minecraft ports)
  - ufw allow 22/tcp
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
	now := time.Now()
	node := &Node{
		ID:               server.ID,
		Hostname:         server.Name,
		IPAddress:        server.IPAddress,
		Type:             "cloud",
		TotalRAMMB:       serverTypeInfo.RAMMB,
		TotalCPUCores:    serverTypeInfo.Cores,
		Status:           NodeStatusHealthy, // DEPRECATED
		LifecycleState:   NodeStateReady,    // NEW: Snapshot nodes start as ready (already initialized)
		HealthStatus:     HealthStatusHealthy, // NEW: Healthy from snapshot
		Metrics: NodeLifecycleMetrics{
			ProvisionedAt:       now,
			InitializedAt:       &now, // Snapshot is pre-initialized
			FirstContainerAt:    nil,
			LastContainerAt:     nil,
			TotalContainersEver: 0,
			CurrentContainers:   0,
		},
		LastHealthCheck:  now,
		ContainerCount:   0,
		AllocatedRAMMB:   0,
		DockerSocketPath: "/var/run/docker.sock",
		SSHUser:          "root",
		CreatedAt:        now,
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
