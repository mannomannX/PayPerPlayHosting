package repository

import (
	"fmt"
	"log"

	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/pkg/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB
var dbProvider DatabaseProvider

// InitDB initializes the database connection
func InitDB(cfg *config.Config) error {
	var err error

	// Configure GORM logger
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	if cfg.Debug {
		gormConfig.Logger = logger.Default.LogMode(logger.Info)
	}

	// Initialize database provider based on config
	switch cfg.DatabaseType {
	case "postgres", "postgresql":
		// PostgreSQL provider
		if cfg.DatabaseURL == "" {
			return fmt.Errorf("DATABASE_URL is required for PostgreSQL")
		}

		log.Printf("Connecting to PostgreSQL: %s", maskPassword(cfg.DatabaseURL))
		DB, err = gorm.Open(postgres.Open(cfg.DatabaseURL), gormConfig)
		if err != nil {
			return fmt.Errorf("failed to connect to PostgreSQL: %w", err)
		}
		dbProvider = &PostgreSQLProvider{db: DB}
		log.Println("PostgreSQL connection established")

	default:
		return fmt.Errorf("unsupported database type: %s (only 'postgres' is supported)", cfg.DatabaseType)
	}

	// Auto-migrate models
	err = dbProvider.Migrate(
		&models.User{},
		&models.MinecraftServer{},
		&models.UsageLog{},
		&models.ConfigChange{},
		&models.ServerFile{},
		&models.ServerWebhook{},
		&models.ServerBackupSchedule{},
		&models.BillingEvent{},
		&models.UsageSession{},
		&models.TrustedDevice{},
		&models.SecurityEvent{},
		&models.OAuthAccount{},
		&models.OAuthState{},
		&models.SystemEvent{},
		&models.Plugin{},
		&models.PluginVersion{},
		&models.InstalledPlugin{},
		&models.Migration{},
		&models.Backup{},
		&models.BackupRestoreTracking{},
		&models.Node{},
	)
	if err != nil {
		return err
	}

	log.Println("Database initialized successfully")
	return nil
}

// GetDB returns the database instance
func GetDB() *gorm.DB {
	return DB
}

// GetDBProvider returns the database provider instance
func GetDBProvider() DatabaseProvider {
	return dbProvider
}

// maskPassword masks the password in a connection string for logging
func maskPassword(url string) string {
	// Simple masking: postgres://user:PASSWORD@host:port/db -> postgres://user:****@host:port/db
	if len(url) < 20 {
		return "****"
	}

	// Find password section (between : and @)
	start := -1
	end := -1
	for i := 0; i < len(url); i++ {
		if url[i] == ':' && start == -1 && i > 10 {
			start = i + 1
		}
		if url[i] == '@' && start != -1 {
			end = i
			break
		}
	}

	if start == -1 || end == -1 || start >= end {
		return "****"
	}

	return url[:start] + "****" + url[end:]
}
