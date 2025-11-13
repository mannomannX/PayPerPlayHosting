package conductor

import (
	"fmt"
	"time"
)

// NodeLifecycleState represents the lifecycle stage of a node
type NodeLifecycleState string

const (
	// Pre-Production States
	NodeStateProvisioning  NodeLifecycleState = "provisioning"  // Hetzner is creating the VM
	NodeStateInitializing  NodeLifecycleState = "initializing"  // Cloud-Init is running
	NodeStateReady         NodeLifecycleState = "ready"         // Healthy, but never had containers

	// Production States
	NodeStateActive NodeLifecycleState = "active" // Has/had containers, productive
	NodeStateIdle   NodeLifecycleState = "idle"   // Active, but currently 0 containers

	// Decommission States
	NodeStateDraining       NodeLifecycleState = "draining"       // No new containers, wait until empty
	NodeStateDecommissioned NodeLifecycleState = "decommissioned" // Deleted

	// Error States
	NodeStateUnhealthy NodeLifecycleState = "unhealthy" // Was active, now broken
)

// HealthStatus represents the health of a node (separate from lifecycle)
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// NodeLifecycleMetrics tracks lifecycle-related metrics
type NodeLifecycleMetrics struct {
	// Lifecycle Timestamps
	ProvisionedAt    time.Time
	InitializedAt    *time.Time // Cloud-Init completed
	FirstContainerAt *time.Time // First time productive
	LastContainerAt  *time.Time // Last container removed

	// Safety Tracking
	TotalContainersEver int // How many containers ever ran
	CurrentContainers   int // Currently running (cached from ContainerRegistry)
}

// CanBeDecommissioned checks if a node can safely be decommissioned
// Returns (canDecommission, reason)
func (n *Node) CanBeDecommissioned() (bool, string) {
	// RULE 1: Never during Provisioning/Init
	if n.LifecycleState == NodeStateProvisioning ||
		n.LifecycleState == NodeStateInitializing {
		return false, "Node is still starting up"
	}

	// RULE 2: Never if containers are running
	if n.ContainerCount > 0 {
		return false, fmt.Sprintf("Node has %d containers", n.ContainerCount)
	}

	// Double-check with allocated RAM (safety redundancy)
	if n.AllocatedRAMMB > 0 {
		return false, fmt.Sprintf("Node has %d MB RAM allocated", n.AllocatedRAMMB)
	}

	// RULE 3: If "ready" (never used), require grace period
	if n.LifecycleState == NodeStateReady {
		if n.Metrics.InitializedAt == nil {
			return false, "Node initialization timestamp missing"
		}

		age := time.Since(*n.Metrics.InitializedAt)
		gracePeriod := 30 * time.Minute

		if age < gracePeriod {
			remaining := gracePeriod - age
			return false, fmt.Sprintf("Ready node too young (%v), wait %v more", age.Round(time.Second), remaining.Round(time.Second))
		}

		return true, fmt.Sprintf("Ready node past grace period (age: %v)", age.Round(time.Minute))
	}

	// RULE 4: If "active"/"idle", always OK if it was productive
	if n.LifecycleState == NodeStateActive || n.LifecycleState == NodeStateIdle {
		if n.Metrics.TotalContainersEver > 0 {
			idleTime := time.Duration(0)
			if n.Metrics.LastContainerAt != nil {
				idleTime = time.Since(*n.Metrics.LastContainerAt)
			}
			return true, fmt.Sprintf("Idle node, served %d containers (idle for %v)", n.Metrics.TotalContainersEver, idleTime.Round(time.Minute))
		}

		// Edge case: Active but never had containers (shouldn't happen)
		return false, "Active node with zero container history (invalid state)"
	}

	// RULE 5: Draining nodes can be decommissioned when empty
	if n.LifecycleState == NodeStateDraining {
		return true, "Node is draining and empty"
	}

	// RULE 6: Unhealthy nodes can be removed immediately (emergency)
	if n.LifecycleState == NodeStateUnhealthy {
		return true, "Unhealthy node cleanup"
	}

	return false, fmt.Sprintf("Unknown lifecycle state: %s", n.LifecycleState)
}

// TransitionLifecycleState transitions a node to a new lifecycle state
// Returns error if transition is invalid
func (n *Node) TransitionLifecycleState(newState NodeLifecycleState, reason string) error {
	oldState := n.LifecycleState

	// Validate transition
	if !isValidTransition(oldState, newState) {
		return fmt.Errorf("invalid lifecycle transition from %s to %s", oldState, newState)
	}

	// Update state
	n.LifecycleState = newState

	// Update timestamps based on new state
	now := time.Now()
	switch newState {
	case NodeStateInitializing:
		// Transitioning from provisioning to initializing
		if n.Metrics.InitializedAt == nil {
			n.Metrics.InitializedAt = &now
		}

	case NodeStateActive:
		// First container started
		if n.Metrics.FirstContainerAt == nil {
			n.Metrics.FirstContainerAt = &now
		}

	case NodeStateIdle:
		// Last container removed
		n.Metrics.LastContainerAt = &now
	}

	return nil
}

// isValidTransition checks if a lifecycle state transition is allowed
func isValidTransition(from, to NodeLifecycleState) bool {
	// Define valid transitions
	validTransitions := map[NodeLifecycleState][]NodeLifecycleState{
		NodeStateProvisioning: {NodeStateInitializing, NodeStateUnhealthy, NodeStateDecommissioned},
		NodeStateInitializing: {NodeStateReady, NodeStateUnhealthy, NodeStateDecommissioned},
		NodeStateReady:        {NodeStateActive, NodeStateIdle, NodeStateDraining, NodeStateUnhealthy, NodeStateDecommissioned},
		NodeStateActive:       {NodeStateIdle, NodeStateUnhealthy, NodeStateDraining},
		NodeStateIdle:         {NodeStateActive, NodeStateDraining, NodeStateUnhealthy, NodeStateDecommissioned},
		NodeStateDraining:     {NodeStateDecommissioned, NodeStateUnhealthy},
		NodeStateUnhealthy:    {NodeStateDecommissioned, NodeStateReady}, // Can recover
		NodeStateDecommissioned: {}, // Terminal state
	}

	allowedStates, exists := validTransitions[from]
	if !exists {
		return false
	}

	for _, allowed := range allowedStates {
		if allowed == to {
			return true
		}
	}

	return false
}

// ShouldTransitionToActive checks if a node should transition from ready to active
func (n *Node) ShouldTransitionToActive() bool {
	return n.LifecycleState == NodeStateReady && n.ContainerCount > 0
}

// ShouldTransitionToIdle checks if a node should transition from active to idle
func (n *Node) ShouldTransitionToIdle() bool {
	return n.LifecycleState == NodeStateActive && n.ContainerCount == 0
}

// ShouldTransitionFromIdle checks if a node should transition from idle back to active
func (n *Node) ShouldTransitionFromIdle() bool {
	return n.LifecycleState == NodeStateIdle && n.ContainerCount > 0
}
