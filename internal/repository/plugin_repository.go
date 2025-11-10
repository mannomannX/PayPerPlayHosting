package repository

import (
	"fmt"

	"github.com/payperplay/hosting/internal/models"
	"gorm.io/gorm"
)

// PluginRepository handles plugin database operations
type PluginRepository struct {
	db *gorm.DB
}

// NewPluginRepository creates a new plugin repository
func NewPluginRepository(db *gorm.DB) *PluginRepository {
	return &PluginRepository{db: db}
}

// === Plugin CRUD ===

// CreatePlugin creates a new plugin
func (r *PluginRepository) CreatePlugin(plugin *models.Plugin) error {
	return r.db.Create(plugin).Error
}

// FindPluginByID finds a plugin by ID
func (r *PluginRepository) FindPluginByID(id string) (*models.Plugin, error) {
	var plugin models.Plugin
	err := r.db.Preload("Versions").First(&plugin, "id = ?", id).Error
	return &plugin, err
}

// FindPluginBySlug finds a plugin by slug
func (r *PluginRepository) FindPluginBySlug(slug string) (*models.Plugin, error) {
	var plugin models.Plugin
	err := r.db.Preload("Versions").First(&plugin, "slug = ?", slug).Error
	return &plugin, err
}

// FindPluginByExternalID finds a plugin by external source ID
func (r *PluginRepository) FindPluginByExternalID(source models.PluginSource, externalID string) (*models.Plugin, error) {
	var plugin models.Plugin
	err := r.db.First(&plugin, "source = ? AND external_id = ?", source, externalID).Error
	return &plugin, err
}

// UpdatePlugin updates a plugin
func (r *PluginRepository) UpdatePlugin(plugin *models.Plugin) error {
	return r.db.Save(plugin).Error
}

// DeletePlugin deletes a plugin
func (r *PluginRepository) DeletePlugin(id string) error {
	return r.db.Delete(&models.Plugin{}, "id = ?", id).Error
}

// ListPlugins lists plugins with optional filters
func (r *PluginRepository) ListPlugins(category models.PluginCategory, source models.PluginSource, limit int) ([]models.Plugin, error) {
	query := r.db.Model(&models.Plugin{})

	if category != "" {
		query = query.Where("category = ?", category)
	}

	if source != "" {
		query = query.Where("source = ?", source)
	}

	if limit > 0 {
		query = query.Limit(limit)
	} else {
		query = query.Limit(100) // Default limit
	}

	var plugins []models.Plugin
	err := query.Order("download_count DESC").Find(&plugins).Error
	return plugins, err
}

// SearchPlugins searches plugins by name or description
func (r *PluginRepository) SearchPlugins(searchTerm string, limit int) ([]models.Plugin, error) {
	query := r.db.Model(&models.Plugin{}).
		Where("name ILIKE ? OR description ILIKE ?", "%"+searchTerm+"%", "%"+searchTerm+"%")

	if limit > 0 {
		query = query.Limit(limit)
	} else {
		query = query.Limit(50)
	}

	var plugins []models.Plugin
	err := query.Order("download_count DESC").Find(&plugins).Error
	return plugins, err
}

// === PluginVersion CRUD ===

// CreatePluginVersion creates a new plugin version
func (r *PluginRepository) CreatePluginVersion(version *models.PluginVersion) error {
	return r.db.Create(version).Error
}

// FindVersionByID finds a version by ID
func (r *PluginRepository) FindVersionByID(id string) (*models.PluginVersion, error) {
	var version models.PluginVersion
	err := r.db.Preload("Plugin").First(&version, "id = ?", id).Error
	return &version, err
}

// FindVersionsByPluginID finds all versions for a plugin
func (r *PluginRepository) FindVersionsByPluginID(pluginID string) ([]models.PluginVersion, error) {
	var versions []models.PluginVersion
	err := r.db.Where("plugin_id = ?", pluginID).
		Order("release_date DESC").
		Find(&versions).Error
	return versions, err
}

// FindLatestStableVersion finds the latest stable version for a plugin
func (r *PluginRepository) FindLatestStableVersion(pluginID string) (*models.PluginVersion, error) {
	var version models.PluginVersion
	err := r.db.Where("plugin_id = ? AND is_stable = true", pluginID).
		Order("release_date DESC").
		First(&version).Error
	return &version, err
}

// FindCompatibleVersions finds versions compatible with server specs
func (r *PluginRepository) FindCompatibleVersions(pluginID string, minecraftVersion string, serverType string) ([]models.PluginVersion, error) {
	var versions []models.PluginVersion

	// Query versions and filter in-memory for JSON array contains
	// PostgreSQL JSON operators could be used for better performance
	err := r.db.Where("plugin_id = ?", pluginID).
		Order("release_date DESC").
		Find(&versions).Error

	if err != nil {
		return nil, err
	}

	// Filter compatible versions (would be more efficient with JSON operators in PostgreSQL)
	// For now, we return all and filter in service layer
	return versions, nil
}

// UpdatePluginVersion updates a plugin version
func (r *PluginRepository) UpdatePluginVersion(version *models.PluginVersion) error {
	return r.db.Save(version).Error
}

// DeletePluginVersion deletes a plugin version
func (r *PluginRepository) DeletePluginVersion(id string) error {
	return r.db.Delete(&models.PluginVersion{}, "id = ?", id).Error
}

// === InstalledPlugin CRUD ===

// InstallPlugin records a plugin installation
func (r *PluginRepository) InstallPlugin(installed *models.InstalledPlugin) error {
	return r.db.Create(installed).Error
}

// FindInstalledPlugin finds an installed plugin by server and plugin ID
func (r *PluginRepository) FindInstalledPlugin(serverID string, pluginID string) (*models.InstalledPlugin, error) {
	var installed models.InstalledPlugin
	err := r.db.Preload("Plugin").Preload("Version").
		First(&installed, "server_id = ? AND plugin_id = ?", serverID, pluginID).Error
	return &installed, err
}

// ListInstalledPlugins lists all plugins installed on a server
func (r *PluginRepository) ListInstalledPlugins(serverID string) ([]models.InstalledPlugin, error) {
	var installed []models.InstalledPlugin
	err := r.db.Preload("Plugin").Preload("Version").
		Where("server_id = ?", serverID).
		Order("installed_at DESC").
		Find(&installed).Error
	return installed, err
}

// UpdateInstalledPlugin updates an installed plugin
func (r *PluginRepository) UpdateInstalledPlugin(installed *models.InstalledPlugin) error {
	return r.db.Save(installed).Error
}

// UninstallPlugin removes an installed plugin record
func (r *PluginRepository) UninstallPlugin(serverID string, pluginID string) error {
	return r.db.Delete(&models.InstalledPlugin{}, "server_id = ? AND plugin_id = ?", serverID, pluginID).Error
}

// FindPluginsWithAutoUpdate finds all servers with a specific plugin that has auto-update enabled
func (r *PluginRepository) FindPluginsWithAutoUpdate(pluginID string) ([]models.InstalledPlugin, error) {
	var installed []models.InstalledPlugin
	err := r.db.Preload("Plugin").Preload("Version").
		Where("plugin_id = ? AND auto_update = true", pluginID).
		Find(&installed).Error
	return installed, err
}

// === Batch Operations ===

// UpsertPlugin creates or updates a plugin (for sync operations)
func (r *PluginRepository) UpsertPlugin(plugin *models.Plugin) error {
	// Check if plugin exists by external ID
	existing, err := r.FindPluginByExternalID(plugin.Source, plugin.ExternalID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create new plugin
			return r.CreatePlugin(plugin)
		}
		return err
	}

	// Update existing plugin
	plugin.ID = existing.ID
	return r.UpdatePlugin(plugin)
}

// UpsertPluginVersion creates or updates a plugin version
func (r *PluginRepository) UpsertPluginVersion(version *models.PluginVersion) error {
	// Check if version exists
	var existing models.PluginVersion
	err := r.db.First(&existing, "plugin_id = ? AND version = ?", version.PluginID, version.Version).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create new version
			return r.CreatePluginVersion(version)
		}
		return err
	}

	// Update existing version
	version.ID = existing.ID
	return r.UpdatePluginVersion(version)
}

// GetPluginStats returns statistics about the plugin marketplace
func (r *PluginRepository) GetPluginStats() (map[string]interface{}, error) {
	var totalPlugins int64
	var totalVersions int64
	var totalInstalled int64

	if err := r.db.Model(&models.Plugin{}).Count(&totalPlugins).Error; err != nil {
		return nil, fmt.Errorf("failed to count plugins: %w", err)
	}

	if err := r.db.Model(&models.PluginVersion{}).Count(&totalVersions).Error; err != nil {
		return nil, fmt.Errorf("failed to count versions: %w", err)
	}

	if err := r.db.Model(&models.InstalledPlugin{}).Count(&totalInstalled).Error; err != nil {
		return nil, fmt.Errorf("failed to count installed: %w", err)
	}

	return map[string]interface{}{
		"total_plugins":   totalPlugins,
		"total_versions":  totalVersions,
		"total_installed": totalInstalled,
	}, nil
}
