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
	Status              NodeStatus        `json:"status"`
	LastHealthCheck     time.Time         `json:"last_health_check"`
	ContainerCount      int               `json:"container_count"`
	AllocatedRAMMB      int               `json:"allocated_ram_mb"`
	SystemReservedRAMMB int               `json:"system_reserved_ram_mb"` // RAM reserved for system processes
	DockerSocketPath    string            `json:"docker_socket_path"`     // Docker socket path (default: /var/run/docker.sock)
	SSHUser             string            `json:"ssh_user"`               // SSH user for remote access
	SSHPort             int               `json:"ssh_port"`               // SSH port (default: 22)
	SSHKeyPath          string            `json:"ssh_key_path"`           // Path to SSH private key for authentication
	CreatedAt           time.Time         `json:"created_at"`
	Labels              map[string]string `json:"labels,omitempty"`  // Cloud provider labels
	HourlyCostEUR       float64           `json:"hourly_cost_eur"`   // For cost tracking
	CloudProviderID     string            `json:"cloud_provider_id"` // External provider ID (e.g., Hetzner server ID)
}

// UsableRAMMB returns the maximum RAM available for containers (Total - System Reserve)
// This is the "capacity" that can be allocated to containers
func (n *Node) UsableRAMMB() int {
	return n.TotalRAMMB - n.SystemReservedRAMMB
}

// AvailableRAMMB returns the currently available RAM for NEW containers
// This accounts for system reserve AND already allocated RAM
func (n *Node) AvailableRAMMB() int {
	usable := n.UsableRAMMB()
	available := usable - n.AllocatedRAMMB
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

// CalculateSystemReserve calculates the intelligent system reserve for this node
// Uses 3-tier strategy:
// - Dedicated nodes (< 8GB): Fixed base + dynamic scaling
// - Cloud nodes (>= 8GB): Percentage-based (15% minimum)
// - Dynamic: +50MB per 10 containers (for Containerd shims)
func (n *Node) CalculateSystemReserve(baseReserveMB int, reservePercent float64) int {
	const (
		smallNodeThreshold = 8192 // 8GB in MB
		mbPerTenContainers = 50   // 50MB per 10 containers
	)

	var reserve int

	if n.TotalRAMMB < smallNodeThreshold {
		// Small/Dedicated nodes: Fixed base + dynamic scaling
		reserve = baseReserveMB

		// Add dynamic reserve based on container count (50MB per 10 containers)
		if n.ContainerCount > 0 {
			dynamicReserve := (n.ContainerCount / 10) * mbPerTenContainers
			reserve += dynamicReserve
		}
	} else {
		// Large/Cloud nodes: Percentage-based
		percentReserve := int(float64(n.TotalRAMMB) * (reservePercent / 100.0))

		// Use the larger of: base reserve OR percentage reserve
		if percentReserve > baseReserveMB {
			reserve = percentReserve
		} else {
			reserve = baseReserveMB
		}
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
