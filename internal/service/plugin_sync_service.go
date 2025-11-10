package service

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/payperplay/hosting/internal/external"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/logger"
	"gorm.io/datatypes"
)

// PluginSyncService handles automatic synchronization of plugins from external sources
type PluginSyncService struct {
	pluginRepo     *repository.PluginRepository
	modrinthClient *external.ModrinthClient
	stopChan       chan struct{}
	syncInterval   time.Duration
}

// NewPluginSyncService creates a new plugin sync service
func NewPluginSyncService(pluginRepo *repository.PluginRepository) *PluginSyncService {
	return &PluginSyncService{
		pluginRepo:     pluginRepo,
		modrinthClient: external.NewModrinthClient(),
		stopChan:       make(chan struct{}),
		syncInterval:   6 * time.Hour, // Sync every 6 hours
	}
}

// Start begins the background sync worker
func (s *PluginSyncService) Start() {
	logger.Info("PluginSyncService started", map[string]interface{}{
		"sync_interval": s.syncInterval.String(),
	})

	// Run initial sync immediately
	go s.runSync()

	// Start background worker
	go s.syncWorker()
}

// Stop stops the background sync worker
func (s *PluginSyncService) Stop() {
	close(s.stopChan)
	logger.Info("PluginSyncService stopped", nil)
}

// syncWorker runs periodic syncs
func (s *PluginSyncService) syncWorker() {
	ticker := time.NewTicker(s.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.runSync()
		case <-s.stopChan:
			return
		}
	}
}

// runSync performs a full synchronization from all sources
func (s *PluginSyncService) runSync() {
	logger.Info("Starting plugin marketplace sync", nil)
	startTime := time.Now()

	// Sync from Modrinth
	syncedCount, err := s.syncModrinth()
	if err != nil {
		logger.Error("Failed to sync from Modrinth", err, nil)
	}

	duration := time.Since(startTime)
	logger.Info("Plugin marketplace sync completed", map[string]interface{}{
		"synced_plugins": syncedCount,
		"duration_ms":    duration.Milliseconds(),
	})
}

// syncModrinth syncs plugins from Modrinth API
func (s *PluginSyncService) syncModrinth() (int, error) {
	logger.Info("Syncing plugins from Modrinth", nil)

	syncedCount := 0
	limit := 100
	offset := 0

	// Fetch popular plugins in batches
	for {
		searchResp, err := s.modrinthClient.SearchPlugins("", limit, offset)
		if err != nil {
			return syncedCount, fmt.Errorf("failed to search plugins: %w", err)
		}

		if len(searchResp.Hits) == 0 {
			break
		}

		// Process each plugin
		for _, modProject := range searchResp.Hits {
			if err := s.syncModrinthPlugin(modProject.ProjectID); err != nil {
				logger.Error("Failed to sync plugin", err, map[string]interface{}{
					"project_id": modProject.ProjectID,
					"slug":       modProject.Slug,
				})
				continue
			}
			syncedCount++
		}

		// Stop after syncing top 500 plugins (5 pages)
		if syncedCount >= 500 {
			break
		}

		offset += limit
		time.Sleep(200 * time.Millisecond) // Rate limiting
	}

	return syncedCount, nil
}

// syncModrinthPlugin syncs a single plugin and its versions from Modrinth
func (s *PluginSyncService) syncModrinthPlugin(projectID string) error {
	// Fetch project details
	modProject, err := s.modrinthClient.GetProject(projectID)
	if err != nil {
		return fmt.Errorf("failed to fetch project: %w", err)
	}

	// Convert to internal Plugin model
	plugin := &models.Plugin{
		Name:          modProject.Title,
		Slug:          modProject.Slug,
		Description:   modProject.Description,
		Author:        modProject.Author,
		Category:      s.mapModrinthCategory(modProject.Categories),
		IconURL:       modProject.IconURL,
		Source:        models.SourceModrinth,
		ExternalID:    modProject.ProjectID,
		DownloadCount: modProject.Downloads,
		Rating:        0, // Modrinth doesn't provide ratings in this endpoint
		LastSynced:    time.Now(),
	}

	fmt.Printf("DEBUG syncModrinthPlugin: About to upsert plugin Slug=%s, ExternalID=%s, ProjectID (param)=%s\n",
		plugin.Slug, plugin.ExternalID, projectID)

	// Upsert plugin
	if err := s.pluginRepo.UpsertPlugin(plugin); err != nil {
		return fmt.Errorf("failed to upsert plugin: %w", err)
	}

	fmt.Printf("DEBUG syncModrinthPlugin: After upsert, plugin.ID=%s, plugin.ExternalID=%s\n",
		plugin.ID, plugin.ExternalID)

	// Fetch and sync versions
	modVersions, err := s.modrinthClient.GetProjectVersions(projectID)
	if err != nil {
		return fmt.Errorf("failed to fetch versions: %w", err)
	}

	// Get the plugin from DB to get its ID
	dbPlugin, err := s.pluginRepo.FindPluginByExternalID(models.SourceModrinth, projectID)
	if err != nil {
		return fmt.Errorf("failed to find plugin after upsert: %w", err)
	}

	// Sync each version
	for _, modVersion := range modVersions {
		if err := s.syncModrinthVersion(dbPlugin.ID, &modVersion); err != nil {
			logger.Warn("Failed to sync version", map[string]interface{}{
				"plugin_id": dbPlugin.ID,
				"version":   modVersion.VersionNumber,
				"error":     err.Error(),
			})
			continue
		}
	}

	return nil
}

// syncModrinthVersion syncs a single plugin version
func (s *PluginSyncService) syncModrinthVersion(pluginID string, modVersion *external.ModrinthVersion) error {
	// Convert game versions to JSON
	mcVersionsJSON, err := json.Marshal(modVersion.GameVersions)
	if err != nil {
		return fmt.Errorf("failed to marshal minecraft versions: %w", err)
	}

	// Convert loaders (server types) to JSON
	serverTypesJSON, err := json.Marshal(modVersion.Loaders)
	if err != nil {
		return fmt.Errorf("failed to marshal server types: %w", err)
	}

	// Convert dependencies to internal format
	deps := make([]models.Dependency, 0, len(modVersion.Dependencies))
	for _, modDep := range modVersion.Dependencies {
		deps = append(deps, models.Dependency{
			PluginSlug: modDep.ProjectID, // We'll resolve to slug later if needed
			Required:   modDep.DependencyType == "required",
			MinVersion: "",
		})
	}

	depsJSON, err := json.Marshal(deps)
	if err != nil {
		return fmt.Errorf("failed to marshal dependencies: %w", err)
	}

	// Find primary download file
	var downloadURL string
	var fileHash string
	var fileSize int64

	for _, file := range modVersion.Files {
		if file.Primary {
			downloadURL = file.URL
			fileHash = file.Hashes.SHA512 // Use SHA512 for better integrity
			fileSize = file.Size
			break
		}
	}

	// If no primary file, use first file
	if downloadURL == "" && len(modVersion.Files) > 0 {
		downloadURL = modVersion.Files[0].URL
		fileHash = modVersion.Files[0].Hashes.SHA512
		fileSize = modVersion.Files[0].Size
	}

	version := &models.PluginVersion{
		PluginID:          pluginID,
		Version:           modVersion.VersionNumber,
		MinecraftVersions: datatypes.JSON(mcVersionsJSON),
		ServerTypes:       datatypes.JSON(serverTypesJSON),
		Dependencies:      datatypes.JSON(depsJSON),
		DownloadURL:       downloadURL,
		FileHash:          fileHash,
		FileSize:          fileSize,
		Changelog:         modVersion.Changelog,
		ReleaseDate:       modVersion.DatePublished,
		IsStable:          modVersion.VersionType == "release",
	}

	// Upsert version
	if err := s.pluginRepo.UpsertPluginVersion(version); err != nil {
		return fmt.Errorf("failed to upsert version: %w", err)
	}

	return nil
}

// mapModrinthCategory maps Modrinth categories to internal categories
func (s *PluginSyncService) mapModrinthCategory(categories []string) models.PluginCategory {
	// Map common Modrinth categories to internal categories
	categoryMap := map[string]models.PluginCategory{
		"worldgen":     models.CategoryWorldManagement,
		"management":   models.CategoryAdminTools,
		"economy":      models.CategoryEconomy,
		"utility":      models.CategoryUtility,
		"optimization": models.CategoryOptimization,
		"library":      models.CategoryUtility,
		"adventure":    models.CategoryMechanics,
		"cursed":       models.CategoryUtility,
		"magic":        models.CategoryMechanics,
		"storage":      models.CategoryUtility,
		"technology":   models.CategoryMechanics,
		"decoration":   models.CategoryWorldManagement,
		"social":       models.CategorySocial,
	}

	// Return first matching category
	for _, cat := range categories {
		if mapped, ok := categoryMap[cat]; ok {
			return mapped
		}
	}

	// Default to utility
	return models.CategoryUtility
}

// SyncPluginManually manually triggers a sync for a specific plugin
func (s *PluginSyncService) SyncPluginManually(pluginSlug string) error {
	logger.Info("Manually syncing plugin", map[string]interface{}{
		"slug": pluginSlug,
	})

	// Try to find plugin in Modrinth
	modProject, err := s.modrinthClient.GetProject(pluginSlug)
	if err != nil {
		return fmt.Errorf("failed to find plugin on Modrinth: %w", err)
	}

	return s.syncModrinthPlugin(modProject.ProjectID)
}
