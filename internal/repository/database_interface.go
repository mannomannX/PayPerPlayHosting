package repository

import (
	"github.com/payperplay/hosting/internal/models"
	"gorm.io/gorm"
)

// Database interface for multi-database support
// This allows easy switching between SQLite and PostgreSQL

type DatabaseProvider interface {
	GetDB() *gorm.DB
	Migrate(models ...interface{}) error
	Close() error
	Ping() error
}

// SQLiteProvider implements DatabaseProvider for SQLite
type SQLiteProvider struct {
	db *gorm.DB
}

func (p *SQLiteProvider) GetDB() *gorm.DB {
	return p.db
}

func (p *SQLiteProvider) Migrate(models ...interface{}) error {
	return p.db.AutoMigrate(models...)
}

func (p *SQLiteProvider) Close() error {
	sqlDB, err := p.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (p *SQLiteProvider) Ping() error {
	sqlDB, err := p.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// PostgreSQLProvider implements DatabaseProvider for PostgreSQL
// Placeholder for future implementation
type PostgreSQLProvider struct {
	db *gorm.DB
}

func (p *PostgreSQLProvider) GetDB() *gorm.DB {
	return p.db
}

func (p *PostgreSQLProvider) Migrate(models ...interface{}) error {
	return p.db.AutoMigrate(models...)
}

func (p *PostgreSQLProvider) Close() error {
	sqlDB, err := p.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (p *PostgreSQLProvider) Ping() error {
	sqlDB, err := p.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// Repository interfaces for clean architecture

type ServerRepositoryInterface interface {
	Create(server *models.MinecraftServer) error
	FindByID(id string) (*models.MinecraftServer, error)
	FindAll() ([]models.MinecraftServer, error)
	FindByOwner(ownerID string) ([]models.MinecraftServer, error)
	Update(server *models.MinecraftServer) error
	Delete(id string) error
	GetUsedPorts() ([]int, error)

	// Usage logs
	CreateUsageLog(log *models.UsageLog) error
	GetActiveUsageLog(serverID string) (*models.UsageLog, error)
	UpdateUsageLog(log *models.UsageLog) error
	GetServerUsageLogs(serverID string) ([]models.UsageLog, error)
}

// Ensure ServerRepository implements the interface
var _ ServerRepositoryInterface = (*ServerRepository)(nil)
