package service

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/config"
)

type PluginService struct {
	repo *repository.ServerRepository
	cfg  *config.Config
}

func NewPluginService(repo *repository.ServerRepository, cfg *config.Config) *PluginService {
	return &PluginService{
		repo: repo,
		cfg:  cfg,
	}
}

// InstallPlugin installs a plugin from URL or SpigotMC
func (p *PluginService) InstallPlugin(serverID string, pluginURL string, filename string) error {
	server, err := p.repo.FindByID(serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Only for plugin-based servers
	if server.ServerType != "paper" && server.ServerType != "spigot" && server.ServerType != "purpur" {
		return fmt.Errorf("plugins are only supported for Paper/Spigot/Purpur servers")
	}

	// Create plugins directory
	pluginsDir := filepath.Join(p.cfg.ServersBasePath, serverID, "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugins directory: %w", err)
	}

	// Download plugin
	pluginPath := filepath.Join(pluginsDir, filename)

	log.Printf("Downloading plugin from %s to %s", pluginURL, pluginPath)

	if err := downloadFile(pluginURL, pluginPath); err != nil {
		return fmt.Errorf("failed to download plugin: %w", err)
	}

	log.Printf("Plugin %s installed successfully for server %s", filename, serverID)

	return nil
}

// ListInstalledPlugins lists all installed plugins for a server
func (p *PluginService) ListInstalledPlugins(serverID string) ([]PluginInfo, error) {
	pluginsDir := filepath.Join(p.cfg.ServersBasePath, serverID, "plugins")

	// Check if directory exists
	if _, err := os.Stat(pluginsDir); os.IsNotExist(err) {
		return []PluginInfo{}, nil
	}

	// Read directory
	files, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, err
	}

	plugins := make([]PluginInfo, 0)

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Only JAR files
		if !strings.HasSuffix(file.Name(), ".jar") {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		plugins = append(plugins, PluginInfo{
			Filename:  file.Name(),
			SizeBytes: info.Size(),
		})
	}

	return plugins, nil
}

// RemovePlugin removes a plugin from a server
func (p *PluginService) RemovePlugin(serverID string, filename string) error {
	pluginPath := filepath.Join(p.cfg.ServersBasePath, serverID, "plugins", filename)

	if err := os.Remove(pluginPath); err != nil {
		return fmt.Errorf("failed to remove plugin: %w", err)
	}

	log.Printf("Plugin %s removed from server %s", filename, serverID)

	return nil
}

// SearchSpigotPlugins searches for plugins on SpigotMC
// Note: This is a simplified version, real implementation would use SpigotMC API
func (p *PluginService) SearchSpigotPlugins(query string) ([]SpigotPlugin, error) {
	// TODO: Implement actual SpigotMC API integration
	// For now, return popular plugins as examples

	popularPlugins := []SpigotPlugin{
		{
			Name:        "EssentialsX",
			Description: "Essential commands and features for your server",
			Version:     "2.20.1",
			DownloadURL: "https://github.com/EssentialsX/Essentials/releases/download/2.20.1/EssentialsX-2.20.1.jar",
		},
		{
			Name:        "LuckPerms",
			Description: "Permissions plugin for Minecraft servers",
			Version:     "5.4.102",
			DownloadURL: "https://download.luckperms.net/1542/bukkit/loader/LuckPerms-Bukkit-5.4.102.jar",
		},
		{
			Name:        "WorldEdit",
			Description: "In-game world editor",
			Version:     "7.2.16",
			DownloadURL: "https://dev.bukkit.org/projects/worldedit/files/latest",
		},
		{
			Name:        "Vault",
			Description: "Economy API for plugins",
			Version:     "1.7.3",
			DownloadURL: "https://github.com/MilkBowl/Vault/releases/download/1.7.3/Vault.jar",
		},
	}

	// Filter by query
	if query != "" {
		filtered := make([]SpigotPlugin, 0)
		queryLower := strings.ToLower(query)
		for _, plugin := range popularPlugins {
			if strings.Contains(strings.ToLower(plugin.Name), queryLower) ||
				strings.Contains(strings.ToLower(plugin.Description), queryLower) {
				filtered = append(filtered, plugin)
			}
		}
		return filtered, nil
	}

	return popularPlugins, nil
}

// InstallModPack installs a modpack from CurseForge
func (p *PluginService) InstallModPack(serverID string, modpackURL string) error {
	server, err := p.repo.FindByID(serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Only for Forge/Fabric servers
	if server.ServerType != "forge" && server.ServerType != "fabric" {
		return fmt.Errorf("modpacks are only supported for Forge/Fabric servers")
	}

	// TODO: Implement CurseForge API integration
	// This is a complex feature that requires:
	// 1. CurseForge API key
	// 2. Downloading modpack manifest
	// 3. Downloading all mods
	// 4. Installing server-side mods only

	return fmt.Errorf("modpack installation not yet implemented - coming soon!")
}

// PluginInfo represents information about an installed plugin
type PluginInfo struct {
	Filename  string `json:"filename"`
	SizeBytes int64  `json:"size_bytes"`
}

// SpigotPlugin represents a plugin from SpigotMC
type SpigotPlugin struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
}

// downloadFile downloads a file from URL to local path
func downloadFile(url string, filepath string) error {
	// Create HTTP client
	client := &http.Client{}

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	// Set User-Agent (some servers require it)
	req.Header.Set("User-Agent", "PayPerPlay-Hosting/1.0")

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Create file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write response body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

// CurseForgeModPack represents a modpack from CurseForge
type CurseForgeModPack struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
}

// SearchModPacks searches for modpacks on CurseForge
// Placeholder for future implementation
func (p *PluginService) SearchModPacks(query string, mcVersion string, loader string) ([]CurseForgeModPack, error) {
	// TODO: Implement CurseForge API
	// Requires API key from https://console.curseforge.com/

	// Example popular modpacks
	popularPacks := []CurseForgeModPack{
		{
			ID:          455984,
			Name:        "All The Mods 9",
			Description: "Kitchen sink modpack with 400+ mods",
			Version:     "0.2.60",
		},
		{
			ID:          285109,
			Name:        "RLCraft",
			Description: "Hardcore survival modpack",
			Version:     "2.9.3",
		},
		{
			ID:          327933,
			Name:        "SkyFactory 4",
			Description: "Skyblock-style automation modpack",
			Version:     "4.2.4",
		},
	}

	return popularPacks, nil
}
