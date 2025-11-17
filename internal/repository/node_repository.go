package repository

import (
	"time"

	"github.com/payperplay/hosting/internal/models"
	"gorm.io/gorm"
)

// NodeRepository handles database operations for nodes
type NodeRepository struct {
	db *gorm.DB
}

// NewNodeRepository creates a new node repository
func NewNodeRepository(db *gorm.DB) *NodeRepository {
	return &NodeRepository{db: db}
}

// Create creates a new node in the database
func (r *NodeRepository) Create(node *models.Node) error {
	return r.db.Create(node).Error
}

// Update updates an existing node
func (r *NodeRepository) Update(node *models.Node) error {
	return r.db.Save(node).Error
}

// FindByID finds a node by ID
func (r *NodeRepository) FindByID(id string) (*models.Node, error) {
	var node models.Node
	err := r.db.Where("id = ?", id).First(&node).Error
	if err != nil {
		return nil, err
	}
	return &node, nil
}

// FindAll returns all nodes
func (r *NodeRepository) FindAll() ([]*models.Node, error) {
	var nodes []*models.Node
	err := r.db.Find(&nodes).Error
	return nodes, err
}

// FindByType returns all nodes of a specific type
func (r *NodeRepository) FindByType(nodeType string) ([]*models.Node, error) {
	var nodes []*models.Node
	err := r.db.Where("type = ?", nodeType).Find(&nodes).Error
	return nodes, err
}

// FindHealthyNodes returns all healthy nodes
func (r *NodeRepository) FindHealthyNodes() ([]*models.Node, error) {
	var nodes []*models.Node
	err := r.db.Where("status = ?", "healthy").Find(&nodes).Error
	return nodes, err
}

// FindWorkerNodes returns all non-system nodes
func (r *NodeRepository) FindWorkerNodes() ([]*models.Node, error) {
	var nodes []*models.Node
	err := r.db.Where("is_system_node = ?", false).Find(&nodes).Error
	return nodes, err
}

// Delete deletes a node by ID
func (r *NodeRepository) Delete(id string) error {
	return r.db.Where("id = ?", id).Delete(&models.Node{}).Error
}

// UpdateStatus updates the status and last health check time
func (r *NodeRepository) UpdateStatus(id string, status string) error {
	return r.db.Model(&models.Node{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":            status,
			"last_health_check": time.Now(),
		}).Error
}

// UpdateResources updates container count and allocated RAM
func (r *NodeRepository) UpdateResources(id string, containerCount int, allocatedRAMMB int) error {
	return r.db.Model(&models.Node{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"container_count":  containerCount,
			"allocated_ram_mb": allocatedRAMMB,
		}).Error
}

// UpdateLastContainerAdded updates the last container added timestamp
func (r *NodeRepository) UpdateLastContainerAdded(id string) error {
	return r.db.Model(&models.Node{}).
		Where("id = ?", id).
		Update("last_container_added", time.Now()).Error
}

// UpdateLastContainerRemoved updates the last container removed timestamp
func (r *NodeRepository) UpdateLastContainerRemoved(id string) error {
	return r.db.Model(&models.Node{}).
		Where("id = ?", id).
		Update("last_container_removed", time.Now()).Error
}

// Exists checks if a node with the given ID exists
func (r *NodeRepository) Exists(id string) (bool, error) {
	var count int64
	err := r.db.Model(&models.Node{}).Where("id = ?", id).Count(&count).Error
	return count > 0, err
}
