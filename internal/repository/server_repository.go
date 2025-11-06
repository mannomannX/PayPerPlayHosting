package repository

import (
	"github.com/payperplay/hosting/internal/models"
	"gorm.io/gorm"
)

type ServerRepository struct {
	db *gorm.DB
}

func NewServerRepository(db *gorm.DB) *ServerRepository {
	return &ServerRepository{db: db}
}

func (r *ServerRepository) Create(server *models.MinecraftServer) error {
	return r.db.Create(server).Error
}

func (r *ServerRepository) FindByID(id string) (*models.MinecraftServer, error) {
	var server models.MinecraftServer
	err := r.db.Where("id = ?", id).First(&server).Error
	if err != nil {
		return nil, err
	}
	return &server, nil
}

func (r *ServerRepository) FindAll() ([]models.MinecraftServer, error) {
	var servers []models.MinecraftServer
	err := r.db.Find(&servers).Error
	return servers, err
}

func (r *ServerRepository) FindByOwner(ownerID string) ([]models.MinecraftServer, error) {
	var servers []models.MinecraftServer
	err := r.db.Where("owner_id = ?", ownerID).Find(&servers).Error
	return servers, err
}

func (r *ServerRepository) FindByPort(port int) (*models.MinecraftServer, error) {
	var server models.MinecraftServer
	err := r.db.Where("port = ?", port).First(&server).Error
	if err != nil {
		return nil, err
	}
	return &server, nil
}

func (r *ServerRepository) Update(server *models.MinecraftServer) error {
	return r.db.Save(server).Error
}

func (r *ServerRepository) Delete(id string) error {
	// Use Unscoped() to perform a hard delete (not soft delete)
	return r.db.Unscoped().Where("id = ?", id).Delete(&models.MinecraftServer{}).Error
}

func (r *ServerRepository) GetUsedPorts() ([]int, error) {
	var ports []int
	err := r.db.Model(&models.MinecraftServer{}).
		Where("port IS NOT NULL").
		Pluck("port", &ports).Error
	return ports, err
}

// Usage Log Repository Methods

func (r *ServerRepository) CreateUsageLog(log *models.UsageLog) error {
	return r.db.Create(log).Error
}

func (r *ServerRepository) GetActiveUsageLog(serverID string) (*models.UsageLog, error) {
	var log models.UsageLog
	err := r.db.Where("server_id = ? AND stopped_at IS NULL", serverID).
		First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *ServerRepository) UpdateUsageLog(log *models.UsageLog) error {
	return r.db.Save(log).Error
}

func (r *ServerRepository) GetServerUsageLogs(serverID string) ([]models.UsageLog, error) {
	var logs []models.UsageLog
	err := r.db.Where("server_id = ?", serverID).
		Order("started_at DESC").
		Find(&logs).Error
	return logs, err
}

func (r *ServerRepository) DeleteServerUsageLogs(serverID string) error {
	// Use Unscoped() to perform a hard delete (not soft delete)
	return r.db.Unscoped().Where("server_id = ?", serverID).Delete(&models.UsageLog{}).Error
}
