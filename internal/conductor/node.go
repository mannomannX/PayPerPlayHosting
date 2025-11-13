package conductor

import "time"

// NodeStatus represents the health status of a node
type NodeStatus string

const (
	NodeStatusHealthy   NodeStatus = "healthy"
	NodeStatusUnhealthy NodeStatus = "unhealthy"
	NodeStatusUnknown   NodeStatus = "unknown"
)

// Node represents a physical or virtual server in the fleet
type Node struct {
	ID                  string            `json:"id"`
	Hostname            string            `json:"hostname"`
	IPAddress           string            `json:"ip_address"`
	Type                string            `json:"type"` // "dedicated", "cloud", "local", or "spare"
	IsSystemNode        bool              `json:"is_system_node"` // System nodes (API/Proxy) cannot run MC containers
	TotalRAMMB          int               `json:"total_ram_mb"`
	TotalCPUCores       int               `json:"total_cpu_cores"`
	CPUUsagePercent     float64           `json:"cpu_usage_percent"`     // Current CPU usage (0-100%)
	Status              NodeStatus        `json:"status"`                // DEPRECATED: Use HealthStatus instead
	LifecycleState      NodeLifecycleState `json:"lifecycle_state"`      // Lifecycle stage (provisioning, ready, active, etc.)
	HealthStatus        HealthStatus      `json:"health_status"`         // Health status (healthy, unhealthy, unknown)
	Metrics             NodeLifecycleMetrics `json:"metrics"`            // Lifecycle metrics and tracking
	LastHealthCheck     time.Time         `json:"last_health_check"`
	ContainerCount      int               `json:"container_count"`
	AllocatedRAMMB      int               `json:"allocated_ram_mb"`
	SystemReservedRAMMB int               `json:"system_reserved_ram_mb"` // RAM reserved for system processes
	DockerSocketPath    string            `json:"docker_socket_path"`     // Docker socket path (default: /var/run/docker.sock)
	SSHUser             string            `json:"ssh_user"`               // SSH user for remote access
	SSHPort             int               `json:"ssh_port"`               // SSH port (default: 22)
	SSHKeyPath          string            `json:"ssh_key_path"`           // Path to SSH private key for authentication
	CreatedAt             time.Time         `json:"created_at"`
	LastContainerAdded    time.Time         `json:"last_container_added"`    // When last container was added
	LastContainerRemoved  time.Time         `json:"last_container_removed"`  // When last container was removed
	Labels                map[string]string `json:"labels,omitempty"`  // Cloud provider labels
	HourlyCostEUR         float64           `json:"hourly_cost_eur"`   // For cost tracking
	CloudProviderID       string            `json:"cloud_provider_id"` // External provider ID (e.g., Hetzner server ID)
}

// UsableRAMMB returns the maximum RAM available for BOOKING
// PROPORTIONAL OVERHEAD MODEL: Returns TotalRAM (100% bookable!)
// SystemReserved is a FIXED budget (12.5%), not subtracted from bookable capacity
// Containers get (100% - SystemOverhead%) of booked RAM via ReductionFactor
func (n *Node) UsableRAMMB() int {
	return n.TotalRAMMB // Voll buchbar!
}

// AvailableRAMMB returns the currently available RAM for NEW containers
// PROPORTIONAL OVERHEAD SYSTEM: Uses TotalRAM (not UsableRAM!)
// Containers get ActualRAM (less), but capacity planning uses TotalRAM
func (n *Node) AvailableRAMMB() int {
	available := n.TotalRAMMB - n.AllocatedRAMMB
	if available < 0 {
		return 0
	}
	return available
}

// RAMUtilizationPercent returns the RAM utilization percentage (based on USABLE RAM, not total)
func (n *Node) RAMUtilizationPercent() float64 {
	usable := n.UsableRAMMB()
	if usable == 0 {
		return 0
	}
	return (float64(n.AllocatedRAMMB) / float64(usable)) * 100.0
}

// UptimeDuration returns how long this node has been alive
func (n *Node) UptimeDuration() time.Duration {
	return time.Since(n.CreatedAt)
}

// IdleDuration returns how long this node has been empty (0 containers)
// Returns 0 if node is not empty or has never been empty
func (n *Node) IdleDuration() time.Duration {
	if n.ContainerCount > 0 {
		return 0 // Not idle
	}
	if n.LastContainerRemoved.IsZero() {
		return 0 // Never had containers
	}
	return time.Since(n.LastContainerRemoved)
}

// IsEmpty returns true if node has no containers
func (n *Node) IsEmpty() bool {
	return n.ContainerCount == 0
}

// CanBeConsolidated checks if this node is eligible for consolidation
// Requirements:
// - Must be empty (0 containers)
// - Must be alive for at least 30 minutes (prevent deleting fresh nodes)
// - Must be idle for at least 15 minutes (prevent deleting recently emptied nodes)
// - Must NOT be a System Node
func (n *Node) CanBeConsolidated(minUptime time.Duration, minIdleTime time.Duration) bool {
	if n.IsSystemNode {
		return false // Never consolidate system nodes
	}
	if !n.IsEmpty() {
		return false // Has containers
	}
	if n.UptimeDuration() < minUptime {
		return false // Too young
	}
	if n.IdleDuration() < minIdleTime {
		return false // Recently emptied
	}
	return true
}

// CalculateSystemReserve calculates system overhead as a FIXED percentage of TotalRAM
// PROPORTIONAL OVERHEAD MODEL: System gets a fixed budget (e.g. 12.5% = 1/8)
// Example: 8GB Node → 1GB System Budget (containers give up 12.5% of their booked RAM)
// The reservePercent parameter should match (1 - ReductionFactor)
func (n *Node) CalculateSystemReserve(baseReserveMB int, reservePercent float64) int {
	// Simple percentage-based calculation for ALL nodes
	// reservePercent = 12.5% means containers get 87.5% of booked RAM
	reserve := int(float64(n.TotalRAMMB) * (reservePercent / 100.0))

	// Minimum 256MB system reserve for very small nodes
	const minSystemReserve = 256
	if reserve < minSystemReserve {
		reserve = minSystemReserve
	}

	// Safety check: Reserve cannot exceed 50% of total RAM
	maxReserve := n.TotalRAMMB / 2
	if reserve > maxReserve {
		reserve = maxReserve
	}

	return reserve
}

// UpdateSystemReserve recalculates and updates the system reserve
func (n *Node) UpdateSystemReserve(baseReserveMB int, reservePercent float64) {
	n.SystemReservedRAMMB = n.CalculateSystemReserve(baseReserveMB, reservePercent)
}

// IsHealthy returns true if the node is healthy
func (n *Node) IsHealthy() bool {
	return n.Status == NodeStatusHealthy
}

// GetReductionFactor returns the proportional RAM reduction factor for containers
// This factor accounts for system overhead distributed proportionally across all containers
// Formula: (TotalRAM - SystemReserve) / TotalRAM
// Example: (8192 - 1228) / 8192 = 0.85 (15% overhead per container)
func (n *Node) GetReductionFactor() float64 {
	if n.TotalRAMMB == 0 {
		return 1.0 // No reduction if no total RAM (shouldn't happen)
	}
	return float64(n.TotalRAMMB - n.SystemReservedRAMMB) / float64(n.TotalRAMMB)
}

// CalculateActualRAM calculates the actual RAM a container receives
// after proportional system overhead is deducted
// Example: 4096 MB booked × 0.85 factor = 3482 MB actual
func (n *Node) CalculateActualRAM(bookedRAMMB int) int {
	actualRAM := int(float64(bookedRAMMB) * n.GetReductionFactor())
	// Ensure minimum 512MB even after reduction
	if actualRAM < 512 {
		actualRAM = 512
	}
	return actualRAM
}
