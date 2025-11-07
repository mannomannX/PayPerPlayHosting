package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/config"
	"github.com/payperplay/hosting/pkg/logger"
)

// MOTDService handles Message of the Day (server description) management
type MOTDService struct {
	serverRepo *repository.ServerRepository
	config     *config.Config
}

// NewMOTDService creates a new MOTD service
func NewMOTDService(
	serverRepo *repository.ServerRepository,
	config *config.Config,
) *MOTDService {
	return &MOTDService{
		serverRepo: serverRepo,
		config:     config,
	}
}

// UpdateMOTD updates the server's MOTD and applies it to server.properties
func (s *MOTDService) UpdateMOTD(serverID, motd string) error {
	// Get server
	server, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Validate MOTD length
	if len(motd) > 512 {
		return fmt.Errorf("MOTD too long (max 512 characters)")
	}

	// Update database
	server.MOTD = motd
	if err := s.serverRepo.Update(server); err != nil {
		return fmt.Errorf("failed to update server: %w", err)
	}

	// Apply to server.properties
	if err := s.applyMOTD(server); err != nil {
		return fmt.Errorf("failed to apply MOTD to server.properties: %w", err)
	}

	logger.Info("MOTD updated", map[string]interface{}{
		"server_id": serverID,
		"motd":      motd,
	})

	return nil
}

// GetMOTD returns the current MOTD for a server
func (s *MOTDService) GetMOTD(serverID string) (string, error) {
	server, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return "", fmt.Errorf("server not found: %w", err)
	}

	return server.MOTD, nil
}

// applyMOTD updates the server.properties file with the MOTD
func (s *MOTDService) applyMOTD(server *models.MinecraftServer) error {
	// Path to server.properties
	propertiesPath := filepath.Join(s.config.ServersBasePath, server.ID, "server.properties")

	// Read existing properties
	properties, err := s.readProperties(propertiesPath)
	if err != nil {
		return fmt.Errorf("failed to read server.properties: %w", err)
	}

	// Update MOTD
	// Escape special characters for server.properties format
	escapedMOTD := s.escapeMOTD(server.MOTD)
	properties["motd"] = escapedMOTD

	// Write updated properties
	if err := s.writeProperties(propertiesPath, properties); err != nil {
		return fmt.Errorf("failed to write server.properties: %w", err)
	}

	// Log restart requirement if server is running
	if server.Status == models.StatusRunning {
		logger.Warn("Server restart required for MOTD changes to take effect", map[string]interface{}{
			"server_id": server.ID,
		})
	}

	return nil
}

// escapeMOTD escapes special characters in MOTD for server.properties format
// Minecraft supports color codes with § symbol and formatting
func (s *MOTDService) escapeMOTD(motd string) string {
	// Escape backslashes and newlines for properties file format
	escaped := strings.ReplaceAll(motd, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\n", "\\n")
	escaped = strings.ReplaceAll(escaped, "\r", "")

	// Keep Minecraft color codes (§) as-is
	// Users can use § followed by color codes (0-9, a-f, k-r)
	// Example: "§aGreen §lBold §rReset"

	return escaped
}

// readProperties reads a server.properties file and returns a map
func (s *MOTDService) readProperties(filePath string) (map[string]string, error) {
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
func (s *MOTDService) writeProperties(filePath string, properties map[string]string) error {
	// Build properties file content
	var lines []string

	// Add header comment
	lines = append(lines, "#Minecraft server properties")
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
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
