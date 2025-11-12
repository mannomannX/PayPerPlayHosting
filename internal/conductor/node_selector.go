package conductor

import (
	"fmt"
	"sort"

	"github.com/payperplay/hosting/pkg/logger"
)

// NodeSelector implements intelligent node selection for container placement
// Uses Best-Fit algorithm: Select node with smallest available RAM that still fits the requirement
type NodeSelector struct {
	nodeRegistry *NodeRegistry
}

// NewNodeSelector creates a new node selector
func NewNodeSelector(registry *NodeRegistry) *NodeSelector {
	return &NodeSelector{
		nodeRegistry: registry,
	}
}

// SelectionStrategy defines how nodes are prioritized
type SelectionStrategy string

const (
	StrategyBestFit      SelectionStrategy = "best_fit"       // Minimize wasted capacity
	StrategyWorstFit     SelectionStrategy = "worst_fit"      // Balance load across nodes
	StrategyLocalFirst   SelectionStrategy = "local_first"    // Prefer local nodes for lower latency
	StrategyCloudFirst   SelectionStrategy = "cloud_first"    // Prefer cloud nodes for cost optimization
	StrategyRoundRobin   SelectionStrategy = "round_robin"    // Distribute evenly
)

// SelectNode selects the best node for a new container based on the strategy
// Returns (nodeID, error)
func (ns *NodeSelector) SelectNode(requiredRAMMB int, strategy SelectionStrategy) (string, error) {
	ns.nodeRegistry.mu.RLock()
	defer ns.nodeRegistry.mu.RUnlock()

	// Get all healthy nodes with sufficient capacity
	candidates := ns.getCandidates(requiredRAMMB)

	if len(candidates) == 0 {
		// No suitable nodes available
		return "", fmt.Errorf("no nodes available with sufficient capacity (%d MB required)", requiredRAMMB)
	}

	// Apply strategy
	var selectedNode *Node
	switch strategy {
	case StrategyBestFit:
		selectedNode = ns.selectBestFit(candidates, requiredRAMMB)
	case StrategyWorstFit:
		selectedNode = ns.selectWorstFit(candidates, requiredRAMMB)
	case StrategyLocalFirst:
		selectedNode = ns.selectLocalFirst(candidates, requiredRAMMB)
	case StrategyCloudFirst:
		selectedNode = ns.selectCloudFirst(candidates, requiredRAMMB)
	case StrategyRoundRobin:
		selectedNode = ns.selectRoundRobin(candidates)
	default:
		// Default to best-fit
		selectedNode = ns.selectBestFit(candidates, requiredRAMMB)
	}

	if selectedNode == nil {
		return "", fmt.Errorf("node selection failed (strategy: %s)", strategy)
	}

	logger.Info("Node selected for container placement", map[string]interface{}{
		"node_id":       selectedNode.ID,
		"node_type":     selectedNode.Type,
		"strategy":      strategy,
		"required_ram":  requiredRAMMB,
		"available_ram": selectedNode.AvailableRAMMB(),
		"utilization":   fmt.Sprintf("%.1f%%", selectedNode.RAMUtilizationPercent()),
	})

	return selectedNode.ID, nil
}

// getCandidates returns all healthy nodes with sufficient capacity
func (ns *NodeSelector) getCandidates(requiredRAMMB int) []*Node {
	var candidates []*Node

	for _, node := range ns.nodeRegistry.nodes {
		// Filter criteria:
		// 1. Node must be healthy
		// 2. Node must have sufficient available RAM (checked against TotalRAM for proportional overhead)
		// 3. Node must NOT be a system node (Control Plane or Proxy)
		//    - Minecraft servers should only run on worker nodes

		// PROPORTIONAL OVERHEAD: Check against TotalRAM, not UsableRAM
		// System overhead is now distributed proportionally across all containers
		// Example: cpx32 (8GB) can fit 2x 4GB bookings, each gets ~3.5GB actual
		availableRAM := node.TotalRAMMB - node.AllocatedRAMMB

		if node.IsHealthy() && availableRAM >= requiredRAMMB && !node.IsSystemNode {
			candidates = append(candidates, node)
		}
	}

	return candidates
}

// selectBestFit selects the node with the smallest available RAM that still fits
// This minimizes wasted capacity and keeps nodes efficiently packed
func (ns *NodeSelector) selectBestFit(candidates []*Node, requiredRAMMB int) *Node {
	if len(candidates) == 0 {
		return nil
	}

	// Sort by available RAM (ascending)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].AvailableRAMMB() < candidates[j].AvailableRAMMB()
	})

	// Return the node with the least available RAM (but still enough)
	return candidates[0]
}

// selectWorstFit selects the node with the most available RAM
// This balances load across nodes and avoids hotspots
func (ns *NodeSelector) selectWorstFit(candidates []*Node, requiredRAMMB int) *Node {
	if len(candidates) == 0 {
		return nil
	}

	// Sort by available RAM (descending)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].AvailableRAMMB() > candidates[j].AvailableRAMMB()
	})

	// Return the node with the most available RAM
	return candidates[0]
}

// selectLocalFirst prefers local nodes over remote nodes for lower latency
// Falls back to remote nodes if local nodes don't have capacity
func (ns *NodeSelector) selectLocalFirst(candidates []*Node, requiredRAMMB int) *Node {
	// Separate local and remote nodes
	var localNodes, remoteNodes []*Node
	for _, node := range candidates {
		if node.Type == "dedicated" || node.ID == "local-node" {
			localNodes = append(localNodes, node)
		} else {
			remoteNodes = append(remoteNodes, node)
		}
	}

	// Prefer local nodes
	if len(localNodes) > 0 {
		return ns.selectBestFit(localNodes, requiredRAMMB)
	}

	// Fall back to remote nodes
	return ns.selectBestFit(remoteNodes, requiredRAMMB)
}

// selectCloudFirst prefers cloud nodes over dedicated nodes for cost optimization
// Uses dedicated nodes only when cloud capacity is exhausted
func (ns *NodeSelector) selectCloudFirst(candidates []*Node, requiredRAMMB int) *Node {
	// Separate cloud and dedicated nodes
	var cloudNodes, dedicatedNodes []*Node
	for _, node := range candidates {
		if node.Type == "cloud" {
			cloudNodes = append(cloudNodes, node)
		} else {
			dedicatedNodes = append(dedicatedNodes, node)
		}
	}

	// Prefer cloud nodes
	if len(cloudNodes) > 0 {
		return ns.selectBestFit(cloudNodes, requiredRAMMB)
	}

	// Fall back to dedicated nodes
	return ns.selectBestFit(dedicatedNodes, requiredRAMMB)
}

// selectRoundRobin distributes containers evenly across nodes
// Uses container count as the balancing metric
func (ns *NodeSelector) selectRoundRobin(candidates []*Node) *Node {
	if len(candidates) == 0 {
		return nil
	}

	// Sort by container count (ascending)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ContainerCount < candidates[j].ContainerCount
	})

	// Return the node with the fewest containers
	return candidates[0]
}

// GetRecommendedStrategy returns the recommended node selection strategy
// based on the current fleet composition
func (ns *NodeSelector) GetRecommendedStrategy() SelectionStrategy {
	ns.nodeRegistry.mu.RLock()
	defer ns.nodeRegistry.mu.RUnlock()

	cloudNodes := 0
	dedicatedNodes := 0

	for _, node := range ns.nodeRegistry.nodes {
		if node.Type == "cloud" {
			cloudNodes++
		} else {
			dedicatedNodes++
		}
	}

	// If we have both cloud and dedicated nodes, prefer local-first for latency
	if cloudNodes > 0 && dedicatedNodes > 0 {
		return StrategyLocalFirst
	}

	// If we only have cloud nodes, use best-fit for cost efficiency
	if cloudNodes > 0 && dedicatedNodes == 0 {
		return StrategyBestFit
	}

	// If we only have dedicated nodes, use best-fit for efficiency
	return StrategyBestFit
}

// HasAvailableWorkerNodes returns true if there are any healthy worker nodes with ANY available capacity
// This is used to check if we can deploy containers or need to provision a new worker node
func (ns *NodeSelector) HasAvailableWorkerNodes() bool {
	ns.nodeRegistry.mu.RLock()
	defer ns.nodeRegistry.mu.RUnlock()

	for _, node := range ns.nodeRegistry.nodes {
		// Check if this is a healthy worker node (not a system node) with any available RAM
		// PROPORTIONAL OVERHEAD: Check against TotalRAM
		availableRAM := node.TotalRAMMB - node.AllocatedRAMMB
		if !node.IsSystemNode && node.IsHealthy() && availableRAM > 0 {
			return true
		}
	}

	return false
}

// GetWorkerNodeCount returns the total number of worker nodes (excluding system nodes)
// This is used by the scaling engine to track worker node fleet size
func (ns *NodeSelector) GetWorkerNodeCount() int {
	ns.nodeRegistry.mu.RLock()
	defer ns.nodeRegistry.mu.RUnlock()

	count := 0
	for _, node := range ns.nodeRegistry.nodes {
		if !node.IsSystemNode {
			count++
		}
	}

	return count
}
