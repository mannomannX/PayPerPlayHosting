package conductor

import (
	"github.com/payperplay/hosting/pkg/logger"
)

// CostNodeInfo contains node information for cost optimization
type CostNodeInfo struct {
	ID          string
	Type        string // "dedicated", "cloud", "proxy"
	CostPerHour float64
	TotalRAMMB  int
	UsedRAMMB   int
	IsHealthy   bool
}

// GetAllNodesForCostAnalysis returns all nodes with cost information
func (c *Conductor) GetAllNodesForCostAnalysis() []CostNodeInfo {
	nodes := c.NodeRegistry.GetAllNodes()
	costNodes := make([]CostNodeInfo, 0, len(nodes))

	for _, node := range nodes {
		costNode := CostNodeInfo{
			ID:          node.ID,
			Type:        node.Type,
			CostPerHour: node.HourlyCostEUR,
			TotalRAMMB:  node.TotalRAMMB,
			UsedRAMMB:   node.AllocatedRAMMB,
			IsHealthy:   node.HealthStatus == HealthStatusHealthy,
		}
		costNodes = append(costNodes, costNode)
	}

	return costNodes
}

// GetNodeCostPerHour returns the hourly cost for a specific node
func (c *Conductor) GetNodeCostPerHour(nodeID string) float64 {
	node, exists := c.NodeRegistry.GetNode(nodeID)
	if !exists {
		return 0
	}

	return node.HourlyCostEUR
}

// CanFitServerOnNode checks if a server can fit on a specific node
func (c *Conductor) CanFitServerOnNode(nodeID string, ramMB int) bool {
	node, exists := c.NodeRegistry.GetNode(nodeID)
	if !exists {
		return false
	}

	// Check if node is healthy
	if node.HealthStatus != HealthStatusHealthy {
		return false
	}

	// Check if enough RAM available
	availableRAM := node.TotalRAMMB - node.AllocatedRAMMB - node.SystemReservedRAMMB
	return availableRAM >= ramMB
}

// IsScalingSystemStable checks if the system is stable (no recent scaling events)
func (c *Conductor) IsScalingSystemStable() bool {
	// Check if queue is being processed
	if c.StartQueue.Size() > 0 {
		logger.Debug("System not stable: queue has pending servers", map[string]interface{}{
			"queue_length": c.StartQueue.Size(),
		})
		return false
	}

	// System is considered stable if queue is empty
	return true
}
