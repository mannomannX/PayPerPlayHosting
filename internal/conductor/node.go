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
	ID               string            `json:"id"`
	Hostname         string            `json:"hostname"`
	IPAddress        string            `json:"ip_address"`
	Type             string            `json:"type"` // "dedicated", "cloud", or "spare"
	TotalRAMMB       int               `json:"total_ram_mb"`
	TotalCPUCores    int               `json:"total_cpu_cores"`
	Status           NodeStatus        `json:"status"`
	LastHealthCheck  time.Time         `json:"last_health_check"`
	ContainerCount   int               `json:"container_count"`
	AllocatedRAMMB   int               `json:"allocated_ram_mb"`
	DockerSocketPath string            `json:"docker_socket_path"`
	SSHUser          string            `json:"ssh_user"`
	CreatedAt        time.Time         `json:"created_at"`
	Labels           map[string]string `json:"labels,omitempty"`     // Cloud provider labels
	HourlyCostEUR    float64           `json:"hourly_cost_eur"`      // For cost tracking
	CloudProviderID  string            `json:"cloud_provider_id"`    // External provider ID (e.g., Hetzner server ID)
}

// AvailableRAMMB returns the available RAM on this node
func (n *Node) AvailableRAMMB() int {
	return n.TotalRAMMB - n.AllocatedRAMMB
}

// RAMUtilizationPercent returns the RAM utilization percentage
func (n *Node) RAMUtilizationPercent() float64 {
	if n.TotalRAMMB == 0 {
		return 0
	}
	return (float64(n.AllocatedRAMMB) / float64(n.TotalRAMMB)) * 100.0
}

// IsHealthy returns true if the node is healthy
func (n *Node) IsHealthy() bool {
	return n.Status == NodeStatusHealthy
}
