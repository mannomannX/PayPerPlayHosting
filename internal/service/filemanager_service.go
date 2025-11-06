package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/config"
	"github.com/payperplay/hosting/pkg/logger"
)

type FileManagerService struct {
	repo *repository.ServerRepository
	cfg  *config.Config
}

func NewFileManagerService(repo *repository.ServerRepository, cfg *config.Config) *FileManagerService {
	return &FileManagerService{
		repo: repo,
		cfg:  cfg,
	}
}

// AllowedFile represents a file that can be edited
type AllowedFile struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description"`
	Editable    bool   `json:"editable"`
}

// GetAllowedFiles returns list of files that can be edited for a server
func (fm *FileManagerService) GetAllowedFiles(serverID string) ([]AllowedFile, error) {
	server, err := fm.repo.FindByID(serverID)
	if err != nil {
		return nil, fmt.Errorf("server not found: %w", err)
	}

	serverPath := filepath.Join(fm.cfg.ServersBasePath, serverID)

	allowedFiles := []AllowedFile{
		{
			Name:        "server.properties",
			Path:        "server.properties",
			Description: "Main server configuration",
			Editable:    true,
		},
		{
			Name:        "whitelist.json",
			Path:        "whitelist.json",
			Description: "Whitelisted players",
			Editable:    true,
		},
		{
			Name:        "ops.json",
			Path:        "ops.json",
			Description: "Server operators",
			Editable:    true,
		},
		{
			Name:        "banned-players.json",
			Path:        "banned-players.json",
			Description: "Banned players",
			Editable:    true,
		},
		{
			Name:        "banned-ips.json",
			Path:        "banned-ips.json",
			Description: "Banned IP addresses",
			Editable:    true,
		},
	}

	// Add plugin/mod specific configs based on server type
	switch server.ServerType {
	case "paper", "spigot", "purpur":
		allowedFiles = append(allowedFiles, AllowedFile{
			Name:        "bukkit.yml",
			Path:        "bukkit.yml",
			Description: "Bukkit configuration",
			Editable:    true,
		})
		allowedFiles = append(allowedFiles, AllowedFile{
			Name:        "spigot.yml",
			Path:        "spigot.yml",
			Description: "Spigot configuration",
			Editable:    true,
		})
		if server.ServerType == "paper" || server.ServerType == "purpur" {
			allowedFiles = append(allowedFiles, AllowedFile{
				Name:        "paper.yml",
				Path:        "config/paper-global.yml",
				Description: "Paper global configuration",
				Editable:    true,
			})
		}
	}

	// Check which files actually exist
	var existingFiles []AllowedFile
	for _, file := range allowedFiles {
		fullPath := filepath.Join(serverPath, file.Path)
		if _, err := os.Stat(fullPath); err == nil {
			existingFiles = append(existingFiles, file)
		}
	}

	return existingFiles, nil
}

// ReadFile reads a configuration file
func (fm *FileManagerService) ReadFile(serverID string, filePath string) (string, error) {
	// Validate server exists
	_, err := fm.repo.FindByID(serverID)
	if err != nil {
		return "", fmt.Errorf("server not found: %w", err)
	}

	// Security: Ensure file path is within server directory
	if err := fm.validateFilePath(serverID, filePath); err != nil {
		return "", err
	}

	serverPath := filepath.Join(fm.cfg.ServersBasePath, serverID)
	fullPath := filepath.Join(serverPath, filePath)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	logger.Info("File read", map[string]interface{}{
		"server_id": serverID,
		"file":      filePath,
	})

	return string(content), nil
}

// WriteFile writes content to a configuration file
func (fm *FileManagerService) WriteFile(serverID string, filePath string, content string) error {
	// Validate server exists
	_, err := fm.repo.FindByID(serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Security: Ensure file path is within server directory
	if err := fm.validateFilePath(serverID, filePath); err != nil {
		return err
	}

	serverPath := filepath.Join(fm.cfg.ServersBasePath, serverID)
	fullPath := filepath.Join(serverPath, filePath)

	// Create backup before writing
	if err := fm.createBackup(fullPath); err != nil {
		logger.Warn("Failed to create backup", map[string]interface{}{
			"error": err.Error(),
			"file":  fullPath,
		})
	}

	// Write new content
	err = os.WriteFile(fullPath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	logger.Info("File written", map[string]interface{}{
		"server_id": serverID,
		"file":      filePath,
		"size":      len(content),
	})

	return nil
}

// validateFilePath ensures the file path is safe and within the server directory
func (fm *FileManagerService) validateFilePath(serverID string, filePath string) error {
	// Prevent directory traversal
	if strings.Contains(filePath, "..") {
		return fmt.Errorf("invalid file path: directory traversal not allowed")
	}

	// Check if path is absolute (should be relative)
	if filepath.IsAbs(filePath) {
		return fmt.Errorf("invalid file path: absolute paths not allowed")
	}

	// Allowed file extensions
	allowedExtensions := []string{".properties", ".yml", ".yaml", ".json", ".txt", ".conf"}
	ext := strings.ToLower(filepath.Ext(filePath))

	allowed := false
	for _, allowedExt := range allowedExtensions {
		if ext == allowedExt {
			allowed = true
			break
		}
	}

	if !allowed {
		return fmt.Errorf("invalid file type: %s not allowed", ext)
	}

	return nil
}

// createBackup creates a backup of the file before modifying it
func (fm *FileManagerService) createBackup(filePath string) error {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // No backup needed if file doesn't exist
	}

	// Read current content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Create backup file
	backupPath := filePath + ".backup"
	err = os.WriteFile(backupPath, content, 0644)
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	return nil
}

// ListFiles lists all files in the server directory (for advanced users)
func (fm *FileManagerService) ListFiles(serverID string, subPath string) ([]FileInfo, error) {
	// Validate server exists
	_, err := fm.repo.FindByID(serverID)
	if err != nil {
		return nil, fmt.Errorf("server not found: %w", err)
	}

	// Security check
	if subPath != "" {
		if err := fm.validateFilePath(serverID, subPath); err != nil {
			return nil, err
		}
	}

	serverPath := filepath.Join(fm.cfg.ServersBasePath, serverID)
	fullPath := filepath.Join(serverPath, subPath)

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, FileInfo{
			Name:    entry.Name(),
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}

	return files, nil
}

// FileInfo represents file/directory information
type FileInfo struct {
	Name    string      `json:"name"`
	IsDir   bool        `json:"is_dir"`
	Size    int64       `json:"size"`
	ModTime interface{} `json:"mod_time"`
}
