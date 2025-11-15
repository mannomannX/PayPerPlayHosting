package service

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/config"
	"github.com/payperplay/hosting/pkg/logger"
)

// WorldInfo represents information about a world
type WorldInfo struct {
	Name        string `json:"name"`         // "world", "world_nether", "world_the_end"
	DisplayName string `json:"display_name"` // "Overworld", "Nether", "The End"
	Exists      bool   `json:"exists"`
	Size        int64  `json:"size"`       // Size in bytes
	SizeFormatted string `json:"size_formatted"` // Human-readable size
	LastModified time.Time `json:"last_modified"`
	CanDelete   bool   `json:"can_delete"`  // Nether and End can be deleted, Overworld cannot
}

// WorldService handles world management operations
type WorldService struct {
	serverRepo    *repository.ServerRepository
	backupService *BackupService
	config        *config.Config
}

// NewWorldService creates a new world service
func NewWorldService(
	serverRepo *repository.ServerRepository,
	backupService *BackupService,
	config *config.Config,
) *WorldService {
	return &WorldService{
		serverRepo:    serverRepo,
		backupService: backupService,
		config:        config,
	}
}

// ListWorlds returns information about all worlds for a server
func (s *WorldService) ListWorlds(serverID string) ([]WorldInfo, error) {
	server, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return nil, fmt.Errorf("server not found: %w", err)
	}

	serverPath := filepath.Join(s.config.ServersBasePath, server.ID)

	// Standard Minecraft world folders
	worldFolders := []struct {
		name        string
		displayName string
		canDelete   bool
	}{
		{"world", "Overworld", false},        // Main world - cannot delete
		{"world_nether", "Nether", true},     // Can be deleted, will regenerate
		{"world_the_end", "The End", true},   // Can be deleted, will regenerate
	}

	var worlds []WorldInfo

	for _, wf := range worldFolders {
		worldPath := filepath.Join(serverPath, wf.name)

		info := WorldInfo{
			Name:        wf.name,
			DisplayName: wf.displayName,
			CanDelete:   wf.canDelete,
			Exists:      false,
		}

		// Check if world exists
		if stat, err := os.Stat(worldPath); err == nil && stat.IsDir() {
			info.Exists = true
			info.LastModified = stat.ModTime()

			// Calculate world size
			size, err := s.calculateDirSize(worldPath)
			if err != nil {
				logger.Warn("Failed to calculate world size", map[string]interface{}{
					"world": wf.name,
					"error": err.Error(),
				})
			} else {
				info.Size = size
				info.SizeFormatted = formatBytes(size)
			}
		}

		worlds = append(worlds, info)
	}

	return worlds, nil
}

// DownloadWorld creates a ZIP archive of a world and returns the file path
func (s *WorldService) DownloadWorld(serverID, worldName string) (string, error) {
	server, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return "", fmt.Errorf("server not found: %w", err)
	}

	// Validate world name
	if !isValidWorldName(worldName) {
		return "", fmt.Errorf("invalid world name: %s", worldName)
	}

	worldPath := filepath.Join(s.config.ServersBasePath, server.ID, worldName)

	// Check if world exists
	if _, err := os.Stat(worldPath); os.IsNotExist(err) {
		return "", fmt.Errorf("world %s does not exist", worldName)
	}

	// Create temp directory for ZIP file
	tempDir := filepath.Join(s.config.ServersBasePath, "temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Generate ZIP filename
	timestamp := time.Now().Format("20060102-150405")
	zipFileName := fmt.Sprintf("%s-%s-%s.zip", server.ID, worldName, timestamp)
	zipPath := filepath.Join(tempDir, zipFileName)

	// Create ZIP file
	if err := s.zipDirectory(worldPath, zipPath); err != nil {
		return "", fmt.Errorf("failed to create world archive: %w", err)
	}

	logger.Info("World download prepared", map[string]interface{}{
		"server_id": serverID,
		"world":     worldName,
		"zip_path":  zipPath,
	})

	return zipPath, nil
}

// UploadWorld extracts an uploaded world ZIP and replaces the existing world
func (s *WorldService) UploadWorld(serverID, worldName, zipPath string) error {
	server, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Server must be stopped for world upload
	if server.Status == models.StatusRunning {
		return fmt.Errorf("server must be stopped before uploading a world")
	}

	// Validate world name
	if !isValidWorldName(worldName) {
		return fmt.Errorf("invalid world name: %s", worldName)
	}

	worldPath := filepath.Join(s.config.ServersBasePath, server.ID, worldName)

	// Create automatic backup before replacing world
	if _, err := os.Stat(worldPath); err == nil {
		logger.Info("Creating automatic backup before world upload", map[string]interface{}{
			"server_id": serverID,
			"world":     worldName,
		})

		if _, err := s.backupService.CreateBackup(
			serverID,
			models.BackupTypePreUpdate,
			fmt.Sprintf("Pre-world-upload backup for %s", worldName),
			nil, // No user ID for automated backups
			0,   // Use default retention (7 days)
		); err != nil {
			logger.Warn("Failed to create automatic backup", map[string]interface{}{
				"error": err.Error(),
			})
		}

		// Remove existing world
		if err := os.RemoveAll(worldPath); err != nil {
			return fmt.Errorf("failed to remove existing world: %w", err)
		}
	}

	// Extract ZIP to world directory
	if err := s.unzipFile(zipPath, worldPath); err != nil {
		return fmt.Errorf("failed to extract world archive: %w", err)
	}

	// Validate extracted world
	if err := s.validateWorld(worldPath); err != nil {
		// Rollback - remove invalid world
		os.RemoveAll(worldPath)
		return fmt.Errorf("invalid world data: %w", err)
	}

	logger.Info("World uploaded successfully", map[string]interface{}{
		"server_id": serverID,
		"world":     worldName,
	})

	return nil
}

// ResetWorld deletes a world folder (it will be regenerated on server start)
func (s *WorldService) ResetWorld(serverID, worldName string) error {
	server, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Server must be stopped for world reset
	if server.Status == models.StatusRunning {
		return fmt.Errorf("server must be stopped before resetting a world")
	}

	// Validate world name
	if !isValidWorldName(worldName) {
		return fmt.Errorf("invalid world name: %s", worldName)
	}

	// Cannot reset main overworld (too dangerous)
	if worldName == "world" {
		return fmt.Errorf("cannot reset main overworld - use upload instead")
	}

	worldPath := filepath.Join(s.config.ServersBasePath, server.ID, worldName)

	// Check if world exists
	if _, err := os.Stat(worldPath); os.IsNotExist(err) {
		return fmt.Errorf("world %s does not exist", worldName)
	}

	// Create automatic backup before reset
	logger.Info("Creating automatic backup before world reset", map[string]interface{}{
		"server_id": serverID,
		"world":     worldName,
	})

	if _, err := s.backupService.CreateBackup(
		serverID,
		models.BackupTypePreUpdate,
		fmt.Sprintf("Pre-world-upload backup for %s", worldName),
		nil, // No user ID for automated backups
		0,   // Use default retention (7 days)
	); err != nil {
		logger.Warn("Failed to create automatic backup", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Delete world folder
	if err := os.RemoveAll(worldPath); err != nil {
		return fmt.Errorf("failed to delete world: %w", err)
	}

	logger.Info("World reset successfully", map[string]interface{}{
		"server_id": serverID,
		"world":     worldName,
	})

	return nil
}

// DeleteWorld permanently deletes a world (same as reset for Minecraft)
func (s *WorldService) DeleteWorld(serverID, worldName string) error {
	// For Minecraft, delete and reset are the same operation
	return s.ResetWorld(serverID, worldName)
}

// Helper functions

// calculateDirSize calculates the total size of a directory
func (s *WorldService) calculateDirSize(path string) (int64, error) {
	var size int64

	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}

// zipDirectory creates a ZIP archive of a directory
func (s *WorldService) zipDirectory(source, target string) error {
	// Create ZIP file
	zipFile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	// Walk through source directory
	err = filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Create ZIP header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}

		header.Name = filepath.ToSlash(relPath)

		// Set compression method
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		// Create header in archive
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		// If not a directory, write file content
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(writer, file)
			if err != nil {
				return err
			}
		}

		return nil
	})

	return err
}

// unzipFile extracts a ZIP archive to a target directory
func (s *WorldService) unzipFile(zipPath, targetDir string) error {
	// Open ZIP file
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	// Create target directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	// Extract files
	for _, file := range reader.File {
		// Construct target path
		targetPath := filepath.Join(targetDir, file.Name)

		// Security check: prevent zip slip vulnerability
		if !strings.HasPrefix(targetPath, filepath.Clean(targetDir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path in archive: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			// Create directory
			if err := os.MkdirAll(targetPath, file.Mode()); err != nil {
				return err
			}
		} else {
			// Create parent directories
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}

			// Extract file
			outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
			if err != nil {
				return err
			}

			rc, err := file.Open()
			if err != nil {
				outFile.Close()
				return err
			}

			_, err = io.Copy(outFile, rc)
			outFile.Close()
			rc.Close()

			if err != nil {
				return err
			}
		}
	}

	return nil
}

// validateWorld checks if an extracted world contains valid Minecraft data
func (s *WorldService) validateWorld(worldPath string) error {
	// Check for essential Minecraft world files
	requiredFiles := []string{
		"level.dat", // Essential world metadata
	}

	for _, file := range requiredFiles {
		filePath := filepath.Join(worldPath, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return fmt.Errorf("missing required file: %s", file)
		}
	}

	return nil
}

// isValidWorldName checks if a world name is valid
func isValidWorldName(name string) bool {
	validNames := []string{"world", "world_nether", "world_the_end"}
	for _, valid := range validNames {
		if name == valid {
			return true
		}
	}
	return false
}

// formatBytes formats bytes into human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
