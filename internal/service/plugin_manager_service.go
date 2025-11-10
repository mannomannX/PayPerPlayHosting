package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/config"
	"github.com/payperplay/hosting/pkg/logger"
)

// PluginManagerService handles plugin installation, updates, and removal
type PluginManagerService struct {
	pluginRepo *repository.PluginRepository
	serverRepo *repository.ServerRepository
	cfg        *config.Config
}

// NewPluginManagerService creates a new plugin manager service
func NewPluginManagerService(pluginRepo *repository.PluginRepository, serverRepo *repository.ServerRepository, cfg *config.Config) *PluginManagerService {
	return &PluginManagerService{
		pluginRepo: pluginRepo,
		serverRepo: serverRepo,
		cfg:        cfg,
	}
}

// === Installation ===

// InstallPlugin installs a plugin on a server
func (s *PluginManagerService) InstallPlugin(serverID string, pluginSlug string, versionID string, autoUpdate bool) error {
	// Fetch server
	server, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Fetch plugin
	plugin, err := s.pluginRepo.FindPluginBySlug(pluginSlug)
	if err != nil {
		return fmt.Errorf("plugin not found: %w", err)
	}

	// Determine version to install
	var version *models.PluginVersion
	if versionID != "" {
		// Install specific version
		version, err = s.pluginRepo.FindVersionByID(versionID)
		if err != nil {
			return fmt.Errorf("version not found: %w", err)
		}
	} else {
		// Install latest compatible version
		version, err = s.findBestVersion(plugin.ID, server.MinecraftVersion, string(server.ServerType))
		if err != nil {
			return fmt.Errorf("no compatible version found: %w", err)
		}
	}

	// Check if already installed
	existing, err := s.pluginRepo.FindInstalledPlugin(serverID, plugin.ID)
	if err == nil {
		return fmt.Errorf("plugin already installed (version: %s)", existing.Version.Version)
	}

	// Download plugin file
	pluginsDir := filepath.Join(s.cfg.ServersBasePath, server.ID, "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugins directory: %w", err)
	}

	pluginFile := filepath.Join(pluginsDir, fmt.Sprintf("%s.jar", plugin.Slug))
	if err := s.downloadFile(version.DownloadURL, pluginFile); err != nil {
		return fmt.Errorf("failed to download plugin: %w", err)
	}

	logger.Info("Plugin downloaded", map[string]interface{}{
		"server_id": serverID,
		"plugin":    plugin.Name,
		"version":   version.Version,
		"file":      pluginFile,
	})

	// Record installation
	installed := &models.InstalledPlugin{
		ServerID:    serverID,
		PluginID:    plugin.ID,
		VersionID:   version.ID,
		Enabled:     true,
		AutoUpdate:  autoUpdate,
		InstalledAt: time.Now(),
	}

	if err := s.pluginRepo.InstallPlugin(installed); err != nil {
		// Clean up downloaded file
		os.Remove(pluginFile)
		return fmt.Errorf("failed to record installation: %w", err)
	}

	logger.Info("Plugin installed successfully", map[string]interface{}{
		"server_id":   serverID,
		"plugin":      plugin.Name,
		"version":     version.Version,
		"auto_update": autoUpdate,
	})

	return nil
}

// === Updates ===

// UpdatePlugin updates a plugin to a newer version
func (s *PluginManagerService) UpdatePlugin(serverID string, pluginID string, newVersionID string) error {
	// Fetch installation record
	installed, err := s.pluginRepo.FindInstalledPlugin(serverID, pluginID)
	if err != nil {
		return fmt.Errorf("plugin not installed: %w", err)
	}

	// Fetch new version
	newVersion, err := s.pluginRepo.FindVersionByID(newVersionID)
	if err != nil {
		return fmt.Errorf("version not found: %w", err)
	}

	// Backup old version
	pluginsDir := filepath.Join(s.cfg.ServersBasePath, serverID, "plugins")
	oldFile := filepath.Join(pluginsDir, fmt.Sprintf("%s.jar", installed.Plugin.Slug))
	backupFile := filepath.Join(pluginsDir, fmt.Sprintf("%s.jar.backup", installed.Plugin.Slug))

	if err := os.Rename(oldFile, backupFile); err != nil {
		logger.Warn("Failed to backup old plugin version", map[string]interface{}{
			"plugin": installed.Plugin.Slug,
			"error":  err.Error(),
		})
	}

	// Download new version
	if err := s.downloadFile(newVersion.DownloadURL, oldFile); err != nil {
		// Restore backup on failure
		os.Rename(backupFile, oldFile)
		return fmt.Errorf("failed to download new version: %w", err)
	}

	// Update installation record
	installed.VersionID = newVersion.ID
	installed.LastUpdated = time.Now()
	if err := s.pluginRepo.UpdateInstalledPlugin(installed); err != nil {
		return fmt.Errorf("failed to update installation record: %w", err)
	}

	// Clean up backup after successful update
	os.Remove(backupFile)

	logger.Info("Plugin updated successfully", map[string]interface{}{
		"server_id":   serverID,
		"plugin":      installed.Plugin.Name,
		"old_version": installed.Version.Version,
		"new_version": newVersion.Version,
	})

	return nil
}

// CheckForUpdates checks if there are updates available for installed plugins
func (s *PluginManagerService) CheckForUpdates(serverID string) ([]UpdateInfo, error) {
	// Fetch server
	server, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return nil, fmt.Errorf("server not found: %w", err)
	}

	// Fetch installed plugins
	installed, err := s.pluginRepo.ListInstalledPlugins(serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to list installed plugins: %w", err)
	}

	updates := []UpdateInfo{}

	for _, inst := range installed {
		// Find latest compatible version
		latestVersion, err := s.findBestVersion(inst.PluginID, server.MinecraftVersion, string(server.ServerType))
		if err != nil {
			continue
		}

		// Compare versions
		if latestVersion.ID != inst.VersionID {
			updates = append(updates, UpdateInfo{
				PluginID:       inst.PluginID,
				PluginName:     inst.Plugin.Name,
				CurrentVersion: inst.Version.Version,
				LatestVersion:  latestVersion.Version,
				LatestVersionID: latestVersion.ID,
				AutoUpdate:     inst.AutoUpdate,
			})
		}
	}

	return updates, nil
}

// UpdateInfo represents available update information
type UpdateInfo struct {
	PluginID        string `json:"plugin_id"`
	PluginName      string `json:"plugin_name"`
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	LatestVersionID string `json:"latest_version_id"`
	AutoUpdate      bool   `json:"auto_update"`
}

// AutoUpdatePlugins automatically updates all plugins with AutoUpdate enabled
func (s *PluginManagerService) AutoUpdatePlugins(serverID string) error {
	updates, err := s.CheckForUpdates(serverID)
	if err != nil {
		return err
	}

	updatedCount := 0
	for _, update := range updates {
		if update.AutoUpdate {
			if err := s.UpdatePlugin(serverID, update.PluginID, update.LatestVersionID); err != nil {
				logger.Error("Auto-update failed", err, map[string]interface{}{
					"server_id": serverID,
					"plugin":    update.PluginName,
				})
				continue
			}
			updatedCount++
		}
	}

	logger.Info("Auto-update completed", map[string]interface{}{
		"server_id": serverID,
		"updated":   updatedCount,
	})

	return nil
}

// === Removal ===

// UninstallPlugin removes a plugin from a server
func (s *PluginManagerService) UninstallPlugin(serverID string, pluginID string) error {
	// Fetch installation record
	installed, err := s.pluginRepo.FindInstalledPlugin(serverID, pluginID)
	if err != nil {
		return fmt.Errorf("plugin not installed: %w", err)
	}

	// Delete plugin file
	pluginsDir := filepath.Join(s.cfg.ServersBasePath, serverID, "plugins")
	pluginFile := filepath.Join(pluginsDir, fmt.Sprintf("%s.jar", installed.Plugin.Slug))

	if err := os.Remove(pluginFile); err != nil {
		logger.Warn("Failed to delete plugin file", map[string]interface{}{
			"file":  pluginFile,
			"error": err.Error(),
		})
	}

	// Remove installation record
	if err := s.pluginRepo.UninstallPlugin(serverID, pluginID); err != nil {
		return fmt.Errorf("failed to remove installation record: %w", err)
	}

	logger.Info("Plugin uninstalled", map[string]interface{}{
		"server_id": serverID,
		"plugin":    installed.Plugin.Name,
	})

	return nil
}

// === Query ===

// ListMarketplacePlugins lists available plugins in the marketplace
func (s *PluginManagerService) ListMarketplacePlugins(category models.PluginCategory, limit int) ([]models.Plugin, error) {
	return s.pluginRepo.ListPlugins(category, "", limit)
}

// SearchMarketplace searches for plugins in the marketplace
func (s *PluginManagerService) SearchMarketplace(query string, limit int) ([]models.Plugin, error) {
	return s.pluginRepo.SearchPlugins(query, limit)
}

// GetPluginDetails retrieves detailed information about a plugin
func (s *PluginManagerService) GetPluginDetails(pluginSlug string) (*models.Plugin, error) {
	return s.pluginRepo.FindPluginBySlug(pluginSlug)
}

// ListInstalledPlugins lists all plugins installed on a server
func (s *PluginManagerService) ListInstalledPlugins(serverID string) ([]models.InstalledPlugin, error) {
	return s.pluginRepo.ListInstalledPlugins(serverID)
}

// === Helper Methods ===

// findBestVersion finds the best (latest stable) compatible version for a server
func (s *PluginManagerService) findBestVersion(pluginID string, minecraftVersion string, serverType string) (*models.PluginVersion, error) {
	versions, err := s.pluginRepo.FindVersionsByPluginID(pluginID)
	if err != nil {
		return nil, err
	}

	// Filter compatible versions
	var compatible []*models.PluginVersion
	for i := range versions {
		if s.isVersionCompatible(&versions[i], minecraftVersion, serverType) {
			compatible = append(compatible, &versions[i])
		}
	}

	if len(compatible) == 0 {
		return nil, fmt.Errorf("no compatible version found for MC %s on %s", minecraftVersion, serverType)
	}

	// Prefer stable versions
	for _, v := range compatible {
		if v.IsStable {
			return v, nil
		}
	}

	// Return latest non-stable if no stable found
	return compatible[0], nil
}

// isVersionCompatible checks if a version is compatible with server specs
func (s *PluginManagerService) isVersionCompatible(version *models.PluginVersion, minecraftVersion string, serverType string) bool {
	// Parse Minecraft versions from JSON
	var mcVersions []string
	if err := json.Unmarshal([]byte(version.MinecraftVersions), &mcVersions); err != nil {
		return false
	}

	// Check Minecraft version compatibility
	mcCompatible := false
	for _, v := range mcVersions {
		if v == minecraftVersion {
			mcCompatible = true
			break
		}
	}
	if !mcCompatible {
		return false
	}

	// Parse server types from JSON
	var serverTypes []string
	if err := json.Unmarshal([]byte(version.ServerTypes), &serverTypes); err != nil {
		return false
	}

	// Check server type compatibility
	for _, st := range serverTypes {
		if st == serverType || st == "paper" || st == "spigot" || st == "bukkit" {
			return true
		}
	}

	return false
}

// downloadFile downloads a file from URL to filepath
func (s *PluginManagerService) downloadFile(url string, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// TogglePlugin enables or disables a plugin
func (s *PluginManagerService) TogglePlugin(serverID string, pluginID string, enabled bool) error {
	installed, err := s.pluginRepo.FindInstalledPlugin(serverID, pluginID)
	if err != nil {
		return fmt.Errorf("plugin not installed: %w", err)
	}

	installed.Enabled = enabled
	return s.pluginRepo.UpdateInstalledPlugin(installed)
}

// ToggleAutoUpdate enables or disables auto-update for a plugin
func (s *PluginManagerService) ToggleAutoUpdate(serverID string, pluginID string, autoUpdate bool) error {
	installed, err := s.pluginRepo.FindInstalledPlugin(serverID, pluginID)
	if err != nil {
		return fmt.Errorf("plugin not installed: %w", err)
	}

	installed.AutoUpdate = autoUpdate
	return s.pluginRepo.UpdateInstalledPlugin(installed)
}
