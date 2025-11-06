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

	// Minecraft
	ServersBasePath    string
	DefaultIdleTimeout int
	MCPortStart        int
	MCPortEnd          int

	// Billing rates (EUR/hour)
	Rate2GB  float64
	Rate4GB  float64
	Rate8GB  float64
	Rate16GB float64
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
		JWTSecret:          getEnv("JWT_SECRET", "change-me-in-production-please-use-a-random-string"),
		ServersBasePath:    getEnv("SERVERS_BASE_PATH", "./minecraft/servers"),
		DefaultIdleTimeout: getEnvInt("DEFAULT_IDLE_TIMEOUT", 300),
		MCPortStart:        getEnvInt("MC_PORT_START", 25565),
		MCPortEnd:          getEnvInt("MC_PORT_END", 25665),
		Rate2GB:            getEnvFloat("RATE_2GB", 0.10),
		Rate4GB:            getEnvFloat("RATE_4GB", 0.20),
		Rate8GB:            getEnvFloat("RATE_8GB", 0.40),
		Rate16GB:           getEnvFloat("RATE_16GB", 0.80),
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
