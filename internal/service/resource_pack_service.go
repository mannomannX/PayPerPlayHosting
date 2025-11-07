package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/config"
	"github.com/payperplay/hosting/pkg/logger"
)

// ResourcePackService handles resource pack integration with Minecraft servers
type ResourcePackService struct {
	fileRepo   *repository.FileRepository
	serverRepo *repository.ServerRepository
	config     *config.Config
}

// NewResourcePackService creates a new resource pack service
func NewResourcePackService(
	fileRepo *repository.FileRepository,
	serverRepo *repository.ServerRepository,
	config *config.Config,
) *ResourcePackService {
	return &ResourcePackService{
		fileRepo:   fileRepo,
		serverRepo: serverRepo,
		config:     config,
	}
}

// ApplyResourcePack applies the active resource pack to a server's configuration
// This is called after a resource pack is activated
func (s *ResourcePackService) ApplyResourcePack(serverID string) error {
	// Get server
	server, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Get active resource pack
	resourcePack, err := s.fileRepo.FindActiveByServerIDAndType(serverID, models.FileTypeResourcePack)
	if err != nil {
		// No active resource pack - clear configuration
		return s.clearResourcePack(serverID)
	}

	// Generate resource pack URL
	resourcePackURL := s.generateResourcePackURL(serverID, resourcePack.ID)

	// Update server.properties in container
	err = s.updateServerProperties(server, resourcePackURL, resourcePack.SHA1Hash)
	if err != nil {
		return fmt.Errorf("failed to update server.properties: %w", err)
	}

	logger.Info("Resource pack applied to server", map[string]interface{}{
		"server_id":   serverID,
		"file_id":     resourcePack.ID,
		"file_name":   resourcePack.FileName,
		"url":         resourcePackURL,
	})

	return nil
}

// RemoveResourcePack removes resource pack configuration from a server
// This is called after a resource pack is deactivated
func (s *ResourcePackService) RemoveResourcePack(serverID string) error {
	return s.clearResourcePack(serverID)
}

// clearResourcePack removes resource pack settings from server.properties
func (s *ResourcePackService) clearResourcePack(serverID string) error {
	server, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Clear resource pack settings
	err = s.updateServerProperties(server, "", "")
	if err != nil {
		return fmt.Errorf("failed to clear server.properties: %w", err)
	}

	logger.Info("Resource pack removed from server", map[string]interface{}{
		"server_id": serverID,
	})

	return nil
}

// updateServerProperties updates the server.properties file with resource pack settings
func (s *ResourcePackService) updateServerProperties(server *models.MinecraftServer, resourcePackURL, sha1Hash string) error {
	// Path to server.properties on host filesystem
	propertiesPath := filepath.Join(s.config.ServersBasePath, server.ID, "server.properties")

	// Read existing properties
	properties, err := s.readProperties(propertiesPath)
	if err != nil {
		return fmt.Errorf("failed to read server.properties: %w", err)
	}

	// Update resource pack settings
	if resourcePackURL != "" {
		properties["resource-pack"] = resourcePackURL
		properties["resource-pack-sha1"] = sha1Hash
		// Optional: require-resource-pack can be made configurable
		// For now, we don't force it (false is default)
	} else {
		// Clear resource pack settings
		delete(properties, "resource-pack")
		delete(properties, "resource-pack-sha1")
	}

	// Write updated properties
	err = s.writeProperties(propertiesPath, properties)
	if err != nil {
		return fmt.Errorf("failed to write server.properties: %w", err)
	}

	// Log restart requirement
	if server.Status == models.StatusRunning {
		logger.Warn("Server restart required for resource pack changes to take effect", map[string]interface{}{
			"server_id": server.ID,
		})
	}

	return nil
}

// generateResourcePackURL generates a publicly accessible URL for the resource pack
func (s *ResourcePackService) generateResourcePackURL(serverID, fileID string) string {
	// Generate URL that points to our API endpoint
	// Format: http://<host>:<port>/api/servers/<serverID>/uploads/<fileID>
	//
	// TODO: Make this configurable via environment variable (PUBLIC_URL or BASE_URL)
	// In production, this should be:
	// - HTTPS
	// - Use a proper domain name (e.g., https://api.payperplay.com)
	// - Consider using a CDN for better distribution
	//
	// For now, we construct from config
	// The API endpoint serves the file directly (no auth required for file download)

	// Default to localhost for development
	baseURL := "http://localhost:8000"

	// TODO: Read from environment variable like PUBLIC_URL or construct from server config
	// Example: baseURL = os.Getenv("PUBLIC_URL")

	return fmt.Sprintf("%s/api/servers/%s/uploads/%s", baseURL, serverID, fileID)
}

// GetResourcePackInfo returns information about the active resource pack for a server
func (s *ResourcePackService) GetResourcePackInfo(serverID string) (*models.ServerFile, error) {
	return s.fileRepo.FindActiveByServerIDAndType(serverID, models.FileTypeResourcePack)
}

// readProperties reads a server.properties file and returns a map
func (s *ResourcePackService) readProperties(filePath string) (map[string]string, error) {
	properties := make(map[string]string)

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet - return empty properties
			return properties, nil
		}
		return nil, err
	}

	// Parse properties
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse key=value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			properties[key] = value
		}
	}

	return properties, nil
}

// writeProperties writes a server.properties file from a map
func (s *ResourcePackService) writeProperties(filePath string, properties map[string]string) error {
	// Build properties file content
	var lines []string

	// Add header comment
	lines = append(lines, "#Minecraft server properties")
	lines = append(lines, fmt.Sprintf("#Updated by PayPerPlay at %s", time.Now().Format(time.RFC3339)))
	lines = append(lines, "")

	// Add all properties (sorted for consistency)
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}

	// Simple alphabetical sort
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}

	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", key, properties[key]))
	}

	// Write to file
	content := strings.Join(lines, "\n") + "\n"
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
