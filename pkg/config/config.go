package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	// Application
	AppName string
	Debug   bool
	Port    string

	// Logging
	LogLevel string
	LogJSON  bool

	// Database
	DatabasePath string
	DatabaseType string
	DatabaseURL  string

	// Authentication
	JWTSecret string
	BaseURL   string // Base URL for OAuth callbacks (e.g., https://yourdomain.com)

	// OAuth Providers
	DiscordClientID     string
	DiscordClientSecret string
	GoogleClientID      string
	GoogleClientSecret  string
	GitHubClientID      string
	GitHubClientSecret  string

	// Minecraft
	ServersBasePath     string // Container path for server data
	HostServersBasePath string // Host path for Docker bind mounts (when API runs in container)
	DefaultIdleTimeout  int
	MCPortStart         int
	MCPortEnd           int
	ControlPlaneIP      string // Public IP address of Control Plane for Velocity to connect to Minecraft servers

	// Billing rates (EUR/hour)
	Rate2GB  float64
	Rate4GB  float64
	Rate8GB  float64
	Rate16GB float64

	// InfluxDB (Time-Series Event Storage)
	InfluxDBURL    string
	InfluxDBToken  string
	InfluxDBOrg    string
	InfluxDBBucket string

	// B5 Auto-Scaling (Hetzner Cloud)
	HetznerCloudToken         string
	HetznerSSHKeyName         string
	SSHPrivateKeyPath         string // Path to SSH private key for remote node access (e.g., /root/.ssh/id_rsa)
	ScalingEnabled            bool
	ScalingCheckInterval      string
	ScalingScaleUpThreshold   float64
	ScalingScaleDownThreshold float64
	ScalingMaxCloudNodes      int

	// B8 Container Migration & Cost Optimization
	CostOptimizationEnabled      bool    // Enable automatic container consolidation
	ConsolidationInterval        string  // How often to check for consolidation opportunities (e.g., "30m")
	ConsolidationThreshold       int     // Minimum number of nodes to save for consolidation (default: 2)
	ConsolidationMaxCapacity     float64 // Don't consolidate if fleet capacity > this % (default: 70.0)
	AllowMigrationWithPlayers    bool    // Allow migration of servers with active players (default: false - safety first!)

	// System Resource Reservation (prevents OOM for system processes)
	SystemReservedRAMMB      int     // Base RAM reserved for system (API, Postgres, Docker, OS)
	SystemReservedCPUCores   float64 // CPU cores reserved for system
	SystemReservedRAMPercent float64 // For cloud nodes: percentage of RAM to reserve (minimum)

	// 3-Tier Architecture: Velocity Proxy Layer (Tier 2)
	VelocityAPIURL string // URL to Velocity Remote API (e.g., http://91.98.232.193:8080)
}

var AppConfig *Config

// Load loads configuration from environment
func Load() *Config {
	// Load .env file if exists
	_ = godotenv.Load()

	config := &Config{
		AppName:            getEnv("APP_NAME", "PayPerPlay"),
		Debug:              getEnvBool("DEBUG", true),
		Port:               getEnv("PORT", "8000"),
		LogLevel:           getEnv("LOG_LEVEL", "INFO"),
		LogJSON:            getEnvBool("LOG_JSON", false),
		DatabasePath:       getEnv("DATABASE_PATH", "./payperplay.db"),
		DatabaseType:       getEnv("DATABASE_TYPE", "sqlite"),
		DatabaseURL:        getEnv("DATABASE_URL", ""),
		JWTSecret:           getEnv("JWT_SECRET", "change-me-in-production-please-use-a-random-string"),
		BaseURL:            getEnv("BASE_URL", "http://localhost:8000"),
		DiscordClientID:     getEnv("DISCORD_CLIENT_ID", ""),
		DiscordClientSecret: getEnv("DISCORD_CLIENT_SECRET", ""),
		GoogleClientID:      getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:  getEnv("GOOGLE_CLIENT_SECRET", ""),
		GitHubClientID:      getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret:  getEnv("GITHUB_CLIENT_SECRET", ""),
		ServersBasePath:     getEnv("SERVERS_BASE_PATH", "./minecraft/servers"),
		HostServersBasePath: getEnv("HOST_SERVERS_BASE_PATH", ""), // If empty, use ServersBasePath
		DefaultIdleTimeout:  getEnvInt("DEFAULT_IDLE_TIMEOUT", 300),
		MCPortStart:        getEnvInt("MC_PORT_START", 25565),
		MCPortEnd:          getEnvInt("MC_PORT_END", 25665),
		ControlPlaneIP:     getEnv("CONTROL_PLANE_IP", "91.98.202.235"),
		Rate2GB:            getEnvFloat("RATE_2GB", 0.10),
		Rate4GB:            getEnvFloat("RATE_4GB", 0.20),
		Rate8GB:            getEnvFloat("RATE_8GB", 0.40),
		Rate16GB:           getEnvFloat("RATE_16GB", 0.80),
		InfluxDBURL:        getEnv("INFLUXDB_URL", ""),
		InfluxDBToken:      getEnv("INFLUXDB_TOKEN", ""),
		InfluxDBOrg:        getEnv("INFLUXDB_ORG", "payperplay"),
		InfluxDBBucket:     getEnv("INFLUXDB_BUCKET", "events"),

		// B5 Auto-Scaling
		HetznerCloudToken:         getEnv("HETZNER_CLOUD_TOKEN", ""),
		HetznerSSHKeyName:         getEnv("HETZNER_SSH_KEY_NAME", "payperplay-main"),
		SSHPrivateKeyPath:         getEnv("SSH_PRIVATE_KEY_PATH", "/root/.ssh/id_rsa"),
		ScalingEnabled:            getEnvBool("SCALING_ENABLED", false),
		ScalingCheckInterval:      getEnv("SCALING_CHECK_INTERVAL", "2m"),
		ScalingScaleUpThreshold:   getEnvFloat("SCALING_SCALE_UP_THRESHOLD", 85.0),
		ScalingScaleDownThreshold: getEnvFloat("SCALING_SCALE_DOWN_THRESHOLD", 30.0),
		ScalingMaxCloudNodes:      getEnvInt("SCALING_MAX_CLOUD_NODES", 10),

		// B8 Container Migration & Cost Optimization
		CostOptimizationEnabled:   getEnvBool("COST_OPTIMIZATION_ENABLED", true),
		ConsolidationInterval:     getEnv("CONSOLIDATION_INTERVAL", "30m"),
		ConsolidationThreshold:    getEnvInt("CONSOLIDATION_THRESHOLD", 2),
		ConsolidationMaxCapacity:  getEnvFloat("CONSOLIDATION_MAX_CAPACITY", 70.0),
		AllowMigrationWithPlayers: getEnvBool("ALLOW_MIGRATION_WITH_PLAYERS", false),

		// System Resource Reservation (3-tier intelligent reservation)
		SystemReservedRAMMB:      getEnvInt("SYSTEM_RESERVED_RAM_MB", 1000),       // 1GB base reserve
		SystemReservedCPUCores:   getEnvFloat("SYSTEM_RESERVED_CPU_CORES", 0.5),   // 0.5 cores for system
		SystemReservedRAMPercent: getEnvFloat("SYSTEM_RESERVED_RAM_PERCENT", 15.0), // 15% for cloud nodes

		// 3-Tier Architecture: Velocity Proxy Layer (Tier 2)
		VelocityAPIURL: getEnv("VELOCITY_API_URL", ""),
	}

	AppConfig = config
	return config
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			log.Printf("Invalid boolean for %s, using default: %v", key, defaultValue)
			return defaultValue
		}
		return boolVal
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		intVal, err := strconv.Atoi(value)
		if err != nil {
			log.Printf("Invalid integer for %s, using default: %d", key, defaultValue)
			return defaultValue
		}
		return intVal
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		floatVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			log.Printf("Invalid float for %s, using default: %.2f", key, defaultValue)
			return defaultValue
		}
		return floatVal
	}
	return defaultValue
}
