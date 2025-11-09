package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus metrics for PayPerPlay monitoring
var (
	// Server resource metrics
	ServerRAMUsageMB = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "payperplay_server_ram_mb",
			Help: "Current RAM usage of Minecraft server in megabytes",
		},
		[]string{"server_id", "server_name", "version"},
	)

	ServerCPUPercent = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "payperplay_server_cpu_percent",
			Help: "Current CPU usage of Minecraft server in percent",
		},
		[]string{"server_id", "server_name", "version"},
	)

	ServerDiskUsageMB = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "payperplay_server_disk_mb",
			Help: "Current disk usage of Minecraft server in megabytes",
		},
		[]string{"server_id", "server_name", "version"},
	)

	// Server status metrics
	ServerStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "payperplay_server_status",
			Help: "Server status (0=stopped, 1=starting, 2=running, 3=stopping, 4=error)",
		},
		[]string{"server_id", "server_name", "version"},
	)

	ServerUptime = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "payperplay_server_uptime_seconds",
			Help: "Server uptime in seconds",
		},
		[]string{"server_id", "server_name", "version"},
	)

	// Player metrics
	ServerPlayerCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "payperplay_server_players",
			Help: "Current number of online players",
		},
		[]string{"server_id", "server_name", "version"},
	)

	ServerPlayerLimit = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "payperplay_server_player_limit",
			Help: "Maximum number of players allowed",
		},
		[]string{"server_id", "server_name", "version"},
	)

	// Performance metrics (via RCON)
	ServerTPS = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "payperplay_server_tps",
			Help: "Server ticks per second (TPS), target is 20.0",
		},
		[]string{"server_id", "server_name", "version"},
	)

	// Fleet-wide metrics
	FleetTotalServers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "payperplay_fleet_total_servers",
			Help: "Total number of servers in the fleet",
		},
	)

	FleetRunningServers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "payperplay_fleet_running_servers",
			Help: "Number of currently running servers",
		},
	)

	FleetTotalRAMMB = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "payperplay_fleet_total_ram_mb",
			Help: "Total RAM allocated across all running servers in MB",
		},
	)

	FleetTotalPlayers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "payperplay_fleet_total_players",
			Help: "Total number of players across all servers",
		},
	)

	// Event counters
	ServerStartTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payperplay_server_starts_total",
			Help: "Total number of server starts",
		},
		[]string{"server_id", "server_name"},
	)

	ServerStopTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payperplay_server_stops_total",
			Help: "Total number of server stops",
		},
		[]string{"server_id", "server_name"},
	)

	ServerCrashTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payperplay_server_crashes_total",
			Help: "Total number of server crashes",
		},
		[]string{"server_id", "server_name"},
	)

	BackupCreatedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payperplay_backups_created_total",
			Help: "Total number of backups created",
		},
		[]string{"server_id", "server_name"},
	)

	// Billing metrics
	ServerBillingSecondsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payperplay_billing_seconds_total",
			Help: "Total billable seconds per server",
		},
		[]string{"server_id", "server_name", "phase"},
	)

	// API metrics
	APIRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payperplay_api_requests_total",
			Help: "Total number of API requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	APIRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "payperplay_api_request_duration_seconds",
			Help:    "API request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)
)

// StatusToFloat converts server status string to numeric value for Prometheus
func StatusToFloat(status string) float64 {
	switch status {
	case "stopped":
		return 0
	case "starting":
		return 1
	case "running":
		return 2
	case "stopping":
		return 3
	case "error":
		return 4
	default:
		return -1
	}
}
