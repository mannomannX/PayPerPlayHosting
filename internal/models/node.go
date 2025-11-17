package models

import (
	"time"
)

// Node represents a physical or virtual server in the fleet (database model)
type Node struct {
	ID                  string    `gorm:"primaryKey;size:100" json:"id"`
	Hostname            string    `gorm:"size:255" json:"hostname"`
	IPAddress           string    `gorm:"size:45;not null;index" json:"ip_address"` // IPv4 or IPv6
	Type                string    `gorm:"size:20;not null;index" json:"type"`        // "dedicated", "cloud", "local", or "spare"
	IsSystemNode        bool      `gorm:"not null;default:false;index" json:"is_system_node"`
	TotalRAMMB          int       `gorm:"not null" json:"total_ram_mb"`
	TotalCPUCores       int       `gorm:"not null" json:"total_cpu_cores"`
	Status              string    `gorm:"size:20;not null;index" json:"status"` // "healthy", "unhealthy", "unknown"
	LifecycleState      string    `gorm:"size:30;index" json:"lifecycle_state"` // "provisioning", "ready", "active", etc.
	LastHealthCheck     time.Time `gorm:"index" json:"last_health_check"`
	ContainerCount      int       `gorm:"not null;default:0" json:"container_count"`
	AllocatedRAMMB      int       `gorm:"not null;default:0" json:"allocated_ram_mb"`
	SystemReservedRAMMB int       `gorm:"not null;default:0" json:"system_reserved_ram_mb"`
	DockerSocketPath    string    `gorm:"size:255;default:'/var/run/docker.sock'" json:"docker_socket_path"`
	SSHUser             string    `gorm:"size:50" json:"ssh_user"`
	SSHPort             int       `gorm:"default:22" json:"ssh_port"`
	SSHKeyPath          string    `gorm:"size:255" json:"ssh_key_path"`
	CreatedAt           time.Time `gorm:"not null;index" json:"created_at"`
	UpdatedAt           time.Time `gorm:"not null" json:"updated_at"`
	LastContainerAdded   time.Time `json:"last_container_added"`
	LastContainerRemoved time.Time `json:"last_container_removed"`
	HourlyCostEUR        float64   `gorm:"type:decimal(10,4);default:0" json:"hourly_cost_eur"`
	CloudProviderID      string    `gorm:"size:100;index" json:"cloud_provider_id"` // External provider ID (e.g., Hetzner server ID)

	// Additional metadata stored as JSON
	CPUUsagePercent float64 `gorm:"-" json:"cpu_usage_percent"` // Runtime metric, not persisted
}

// TableName specifies the table name
func (Node) TableName() string {
	return "nodes"
}
