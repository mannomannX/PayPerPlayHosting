package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/config"
	"github.com/payperplay/hosting/pkg/logger"
)

// FileIntegrationService handles integration of uploaded files into server containers
// Covers: Resource Packs, Data Packs, Server Icons, World Gen
type FileIntegrationService struct {
	fileRepo            *repository.FileRepository
	serverRepo          *repository.ServerRepository
	resourcePackService *ResourcePackService
	config              *config.Config
}

// NewFileIntegrationService creates a new file integration service
func NewFileIntegrationService(
	fileRepo *repository.FileRepository,
	serverRepo *repository.ServerRepository,
	resourcePackService *ResourcePackService,
	config *config.Config,
) *FileIntegrationService {
	return &FileIntegrationService{
		fileRepo:            fileRepo,
		serverRepo:          serverRepo,
		resourcePackService: resourcePackService,
		config:              config,
	}
}

// ApplyFile applies an activated file to the server based on its type
func (s *FileIntegrationService) ApplyFile(serverID, fileID string) error {
	// Get the file
	file, err := s.fileRepo.FindByID(fileID)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	// Verify server ID
	if file.ServerID != serverID {
		return fmt.Errorf("file does not belong to this server")
	}

	// Apply based on file type
	switch file.FileType {
	case models.FileTypeResourcePack:
		return s.resourcePackService.ApplyResourcePack(serverID)

	case models.FileTypeDataPack:
		return s.applyDataPack(serverID, file)

	case models.FileTypeServerIcon:
		return s.applyServerIcon(serverID, file)

	case models.FileTypeWorldGen:
		return s.applyWorldGen(serverID, file)

	default:
		return fmt.Errorf("unsupported file type: %s", file.FileType)
	}
}

// RemoveFile removes a deactivated file from the server based on its type
func (s *FileIntegrationService) RemoveFile(serverID, fileID string, fileType models.FileType) error {
	switch fileType {
	case models.FileTypeResourcePack:
		return s.resourcePackService.RemoveResourcePack(serverID)

	case models.FileTypeDataPack:
		return s.removeDataPack(serverID, fileID)

	case models.FileTypeServerIcon:
		return s.removeServerIcon(serverID)

	case models.FileTypeWorldGen:
		return s.removeWorldGen(serverID, fileID)

	default:
		return fmt.Errorf("unsupported file type: %s", fileType)
	}
}

// applyDataPack copies a data pack ZIP to the server's datapacks directory
func (s *FileIntegrationService) applyDataPack(serverID string, file *models.ServerFile) error {
	server, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Source: uploaded file location
	sourcePath := filepath.Join(s.config.ServersBasePath, serverID, file.FilePath)

	// Destination: /world/datapacks/ directory
	datapacksDir := filepath.Join(s.config.ServersBasePath, serverID, "world", "datapacks")

	// Create datapacks directory if it doesn't exist
	if err := os.MkdirAll(datapacksDir, 0755); err != nil {
		return fmt.Errorf("failed to create datapacks directory: %w", err)
	}

	// Destination file: use original filename
	destPath := filepath.Join(datapacksDir, file.FileName)

	// Copy file
	if err := s.copyFile(sourcePath, destPath); err != nil {
		return fmt.Errorf("failed to copy data pack: %w", err)
	}

	logger.Info("Data pack applied to server", map[string]interface{}{
		"server_id": serverID,
		"file_id":   file.ID,
		"file_name": file.FileName,
	})

	// Log restart requirement
	if server.Status == models.StatusRunning {
		logger.Warn("Server restart required for data pack changes to take effect", map[string]interface{}{
			"server_id": serverID,
		})
	}

	return nil
}

// removeDataPack removes a data pack from the server's datapacks directory
func (s *FileIntegrationService) removeDataPack(serverID, fileID string) error {
	// Get file info
	file, err := s.fileRepo.FindByID(fileID)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	// Path to data pack in datapacks directory
	datapackPath := filepath.Join(s.config.ServersBasePath, serverID, "world", "datapacks", file.FileName)

	// Remove file
	if err := os.Remove(datapackPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove data pack: %w", err)
		}
		// If file doesn't exist, that's fine - already removed
	}

	logger.Info("Data pack removed from server", map[string]interface{}{
		"server_id": serverID,
		"file_id":   fileID,
	})

	return nil
}

// applyServerIcon copies a server icon to the server root as server-icon.png
func (s *FileIntegrationService) applyServerIcon(serverID string, file *models.ServerFile) error {
	server, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Source: uploaded file location
	sourcePath := filepath.Join(s.config.ServersBasePath, serverID, file.FilePath)

	// Destination: server-icon.png in server root
	destPath := filepath.Join(s.config.ServersBasePath, serverID, "server-icon.png")

	// Copy file
	if err := s.copyFile(sourcePath, destPath); err != nil {
		return fmt.Errorf("failed to copy server icon: %w", err)
	}

	logger.Info("Server icon applied", map[string]interface{}{
		"server_id": serverID,
		"file_id":   file.ID,
	})

	// Server icon takes effect immediately (no restart needed)
	// Players will see it on next server list refresh
	if server.Status == models.StatusRunning {
		logger.Info("Server icon will be visible on next server list refresh", map[string]interface{}{
			"server_id": serverID,
		})
	}

	return nil
}

// removeServerIcon removes the server icon
func (s *FileIntegrationService) removeServerIcon(serverID string) error {
	// Path to server icon
	iconPath := filepath.Join(s.config.ServersBasePath, serverID, "server-icon.png")

	// Remove file
	if err := os.Remove(iconPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove server icon: %w", err)
		}
		// If file doesn't exist, that's fine - already removed
	}

	logger.Info("Server icon removed", map[string]interface{}{
		"server_id": serverID,
	})

	return nil
}

// applyWorldGen applies world generation configuration
func (s *FileIntegrationService) applyWorldGen(serverID string, file *models.ServerFile) error {
	server, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Source: uploaded file location
	sourcePath := filepath.Join(s.config.ServersBasePath, serverID, file.FilePath)

	// Destination: depends on Minecraft version and server type
	// For vanilla/paper 1.16+: custom world generation goes in datapacks
	// For now, we'll copy to a world_gen directory for manual application
	worldGenDir := filepath.Join(s.config.ServersBasePath, serverID, "world_gen")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(worldGenDir, 0755); err != nil {
		return fmt.Errorf("failed to create world_gen directory: %w", err)
	}

	// Destination file
	destPath := filepath.Join(worldGenDir, file.FileName)

	// Copy file
	if err := s.copyFile(sourcePath, destPath); err != nil {
		return fmt.Errorf("failed to copy world gen config: %w", err)
	}

	logger.Info("World generation config applied", map[string]interface{}{
		"server_id": serverID,
		"file_id":   file.ID,
	})

	// World gen requires world regeneration or new world
	if server.Status == models.StatusRunning {
		logger.Warn("World generation changes require world regeneration or new world", map[string]interface{}{
			"server_id": serverID,
		})
	}

	return nil
}

// removeWorldGen removes world generation configuration
func (s *FileIntegrationService) removeWorldGen(serverID, fileID string) error {
	// Get file info
	file, err := s.fileRepo.FindByID(fileID)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	// Path to world gen file
	worldGenPath := filepath.Join(s.config.ServersBasePath, serverID, "world_gen", file.FileName)

	// Remove file
	if err := os.Remove(worldGenPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove world gen config: %w", err)
		}
	}

	logger.Info("World generation config removed", map[string]interface{}{
		"server_id": serverID,
		"file_id":   fileID,
	})

	return nil
}

// copyFile copies a file from source to destination
func (s *FileIntegrationService) copyFile(src, dst string) error {
	// Open source file
	source, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer source.Close()

	// Create destination file
	destination, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destination.Close()

	// Copy content
	_, err = io.Copy(destination, source)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	// Sync to ensure data is written
	err = destination.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	return nil
}
