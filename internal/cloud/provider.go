package cloud

import "time"

// CloudProvider defines the interface for cloud infrastructure providers
// Implementations: Hetzner, AWS, GCP, Azure, etc.
type CloudProvider interface {
	// Server Management
	CreateServer(spec ServerSpec) (*Server, error)
	DeleteServer(serverID string) error
	ListServers(labels map[string]string) ([]*Server, error)
	GetServer(serverID string) (*Server, error)

	// Server Types (for capacity planning)
	GetServerTypes() ([]*ServerType, error)
	GetServerType(name string) (*ServerType, error)

	// Health & Status
	WaitForServerReady(serverID string, timeout time.Duration) error
	GetServerStatus(serverID string) (ServerStatus, error)

	// Server Actions
	PowerOnServer(serverID string) error
	PowerOffServer(serverID string) error
	RebootServer(serverID string) error

	// Snapshots (for B6 - Hot-Spare Pool)
	CreateSnapshot(serverID string, description string) (*Snapshot, error)
	DeleteSnapshot(snapshotID string) error
	CreateServerFromSnapshot(snapshotID string, spec ServerSpec) (*Server, error)

	// Pricing (for cost tracking)
	GetServerPricing(serverType string) (*Pricing, error)

	// Metrics (for monitoring)
	GetServerMetrics(serverID string) (float64, error) // Returns CPU usage percentage
}

// ServerSpec defines what we want to create
type ServerSpec struct {
	Name      string            // "payperplay-node-1"
	Type      string            // "cx21" (Hetzner), "t2.micro" (AWS)
	Image     string            // "ubuntu-22.04"
	Location  string            // "nbg1", "fsn1", "hel1" (Hetzner)
	CloudInit string            // Cloud-Init script
	Labels    map[string]string // {"managed_by": "payperplay", "type": "cloud"}
	SSHKeys   []string          // SSH key names/IDs
}

// Server represents a cloud server instance
type Server struct {
	ID            string
	Name          string
	Type          string
	Status        ServerStatus
	IPAddress     string        // Public IPv4
	PrivateIP     string        // Private network IP (if available)
	Location      string        // Data center location
	CreatedAt     time.Time
	Labels        map[string]string
	HourlyCostEUR float64       // Cost per hour
}

// ServerStatus represents the current state of a server
type ServerStatus string

const (
	ServerStatusInitializing ServerStatus = "initializing"
	ServerStatusStarting     ServerStatus = "starting"
	ServerStatusRunning      ServerStatus = "running"
	ServerStatusStopping     ServerStatus = "stopping"
	ServerStatusStopped      ServerStatus = "off"
	ServerStatusDeleting     ServerStatus = "deleting"
	ServerStatusDeleted      ServerStatus = "deleted"
	ServerStatusUnknown      ServerStatus = "unknown"
)

// ServerType represents an available VM size/type
type ServerType struct {
	ID            string
	Name          string
	Description   string
	Cores         int     // CPU cores
	RAMMB         int     // RAM in MB
	DiskGB        int     // Disk size in GB
	HourlyCostEUR float64 // Cost per hour
	MonthlyCostEUR float64 // Cost per month
	Available     bool    // Currently available?
}

// Snapshot represents a server snapshot (for B6 - Spare Pool)
type Snapshot struct {
	ID          string
	Name        string
	Description string
	ImageSize   float64   // Size in GB
	CreatedAt   time.Time
}

// Pricing represents cost information
type Pricing struct {
	HourlyCostEUR  float64
	MonthlyCostEUR float64
	Currency       string // "EUR"
}
