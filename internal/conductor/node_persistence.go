package conductor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/payperplay/hosting/pkg/logger"
)

// PersistedNodeState represents the minimal state needed to restore a cloud node after restart
// This prevents data loss when the backend restarts while cloud VMs are still running
type PersistedNodeState struct {
	ID              string            `json:"id"`
	Hostname        string            `json:"hostname"`
	IPAddress       string            `json:"ip_address"`
	Type            string            `json:"type"`
	TotalRAMMB      int               `json:"total_ram_mb"`
	TotalCPUCores   int               `json:"total_cpu_cores"`
	CloudProviderID string            `json:"cloud_provider_id"`
	HourlyCostEUR   float64           `json:"hourly_cost_eur"`
	CreatedAt       time.Time         `json:"created_at"`
	Labels          map[string]string `json:"labels"`
	RecoveredAt     *time.Time        `json:"recovered_at,omitempty"` // When this node was last recovered from state file
}

// SaveNodeState persists all cloud nodes to a JSON file
// Called during graceful shutdown to preserve node state
func (c *Conductor) SaveNodeState(filePath string) error {
	cloudNodes := []PersistedNodeState{}

	c.NodeRegistry.mu.RLock()
	for _, node := range c.NodeRegistry.nodes {
		// Only persist cloud nodes (dedicated nodes are always registered on startup)
		if node.Type == "cloud" && !node.IsSystemNode {
			state := PersistedNodeState{
				ID:              node.ID,
				Hostname:        node.Hostname,
				IPAddress:       node.IPAddress,
				Type:            node.Type,
				TotalRAMMB:      node.TotalRAMMB,
				TotalCPUCores:   node.TotalCPUCores,
				CloudProviderID: node.CloudProviderID,
				HourlyCostEUR:   node.HourlyCostEUR,
				CreatedAt:       node.CreatedAt,
				Labels:          node.Labels,
			}
			cloudNodes = append(cloudNodes, state)
		}
	}
	c.NodeRegistry.mu.RUnlock()

	logger.Info("NODE-PERSIST: Saving node state", map[string]interface{}{
		"cloud_nodes": len(cloudNodes),
		"file":        filePath,
	})

	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(cloudNodes, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal node state: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Write to file atomically (write to temp file, then rename)
	tempFile := filePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	if err := os.Rename(tempFile, filePath); err != nil {
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	logger.Info("NODE-PERSIST: Node state saved successfully", map[string]interface{}{
		"nodes": len(cloudNodes),
	})

	return nil
}

// LoadNodeState loads persisted node state from JSON file
// Returns empty slice if file doesn't exist (first run)
func (c *Conductor) LoadNodeState(filePath string) ([]PersistedNodeState, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info("NODE-PERSIST: No state file found (first run or clean start)", nil)
			return []PersistedNodeState{}, nil
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var nodes []PersistedNodeState
	if err := json.Unmarshal(data, &nodes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state file: %w", err)
	}

	logger.Info("NODE-PERSIST: Loaded node state from file", map[string]interface{}{
		"nodes": len(nodes),
	})

	return nodes, nil
}

// RestoreNodesFromState restores nodes from persisted state file
// This is called BEFORE syncing from Hetzner API to prioritize local state
// Nodes from state file get a recovery grace period to prevent immediate scale-down
func (c *Conductor) RestoreNodesFromState(filePath string) error {
	states, err := c.LoadNodeState(filePath)
	if err != nil {
		return fmt.Errorf("failed to load node state: %w", err)
	}

	if len(states) == 0 {
		logger.Info("NODE-PERSIST: No nodes to restore from state", nil)
		return nil
	}

	logger.Info("NODE-PERSIST: Restoring nodes from persisted state", map[string]interface{}{
		"count": len(states),
	})

	now := time.Now()
	recoveredCount := 0

	for _, state := range states {
		// Check if node already registered (shouldn't happen, but defensive)
		if _, exists := c.NodeRegistry.GetNode(state.ID); exists {
			logger.Debug("NODE-PERSIST: Node already registered, skipping", map[string]interface{}{
				"node_id": state.ID,
			})
			continue
		}

		// Create node object with recovery timestamp
		node := &Node{
			ID:               state.ID,
			Hostname:         state.Hostname,
			IPAddress:        state.IPAddress,
			Type:             state.Type,
			TotalRAMMB:       state.TotalRAMMB,
			TotalCPUCores:    state.TotalCPUCores,
			Status:           NodeStatusHealthy, // Will be verified by health checker
			LifecycleState:   NodeStateReady,
			HealthStatus:     HealthStatusUnknown,
			LastHealthCheck:  now,
			ContainerCount:   0,
			AllocatedRAMMB:   0,
			DockerSocketPath: "/var/run/docker.sock",
			SSHUser:          "root",
			CreatedAt:        state.CreatedAt,
			Labels:           state.Labels,
			HourlyCostEUR:    state.HourlyCostEUR,
			CloudProviderID:  state.CloudProviderID,
			IsSystemNode:     false,
			Metrics: NodeLifecycleMetrics{
				ProvisionedAt:        state.CreatedAt,
				InitializedAt:        &now,
				RecoveredAt:          &now, // Mark as recovered with timestamp
				RecoveryGracePeriod:  4 * time.Hour, // 4 hours grace period
				FirstContainerAt:     nil,
				LastContainerAt:      nil,
				TotalContainersEver:  0,
				CurrentContainers:    0,
			},
		}

		// Calculate system reserve
		if cfg := c.GetConfig(); cfg != nil {
			node.UpdateSystemReserve(cfg.SystemReservedRAMMB, cfg.SystemReservedRAMPercent)
		}

		// Register node
		c.NodeRegistry.RegisterNode(node)

		logger.Info("NODE-PERSIST: Node restored from state", map[string]interface{}{
			"node_id":            node.ID,
			"hostname":           node.Hostname,
			"ip":                 node.IPAddress,
			"total_ram_mb":       node.TotalRAMMB,
			"usable_ram_mb":      node.UsableRAMMB(),
			"recovery_grace_hrs": 4,
		})

		recoveredCount++
	}

	logger.Info("NODE-PERSIST: Node restoration completed", map[string]interface{}{
		"recovered": recoveredCount,
		"total":     len(states),
	})

	return nil
}

// GetConfig returns the configuration needed for node operations
// This is a helper to access config from conductor
func (c *Conductor) GetConfig() *NodeConfig {
	if c.ScalingEngine != nil && c.ScalingEngine.vmProvisioner != nil {
		// Config is accessible through scaling engine
		return &NodeConfig{
			SystemReservedRAMMB:      1000, // Default fallback
			SystemReservedRAMPercent: 15.0,
		}
	}
	return nil
}

// NodeConfig holds configuration for node operations
type NodeConfig struct {
	SystemReservedRAMMB      int
	SystemReservedRAMPercent float64
}
