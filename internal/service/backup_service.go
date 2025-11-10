package service

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/payperplay/hosting/internal/events"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/rcon"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/config"
)

type BackupService struct {
	repo       *repository.ServerRepository
	cfg        *config.Config
	backupDir  string
}

func NewBackupService(repo *repository.ServerRepository, cfg *config.Config) (*BackupService, error) {
	backupDir := filepath.Join(filepath.Dir(cfg.ServersBasePath), "backups")

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	return &BackupService{
		repo:      repo,
		cfg:       cfg,
		backupDir: backupDir,
	}, nil
}

// CreateBackup creates a backup of a server's world
func (b *BackupService) CreateBackup(serverID string) (string, error) {
	server, err := b.repo.FindByID(serverID)
	if err != nil {
		return "", fmt.Errorf("server not found: %w", err)
	}

	// If server is running, save world data before backup
	if server.Status == models.StatusRunning {
		if err := b.saveRunningServer(server); err != nil {
			log.Printf("Warning: failed to save running server: %v", err)
			// Continue with backup anyway
		}
	}

	serverDir := filepath.Join(b.cfg.ServersBasePath, serverID)
	if _, err := os.Stat(serverDir); os.IsNotExist(err) {
		return "", fmt.Errorf("server directory not found: %s", serverDir)
	}

	// Create backup filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupFilename := fmt.Sprintf("%s-%s.zip", serverID, timestamp)
	backupPath := filepath.Join(b.backupDir, backupFilename)

	// Create ZIP file
	log.Printf("Creating backup for server %s (%s)...", server.Name, serverID)

	if err := b.zipDirectory(serverDir, backupPath); err != nil {
		return "", fmt.Errorf("failed to create backup: %w", err)
	}

	// Get file size
	stat, err := os.Stat(backupPath)
	if err != nil {
		return "", err
	}

	sizeGB := float64(stat.Size()) / 1024 / 1024 / 1024

	// Publish event
	events.PublishBackupCreated(serverID, backupFilename, stat.Size())

	log.Printf("Backup created: %s (%.2f GB)", backupFilename, sizeGB)

	return backupPath, nil
}

// RestoreBackup restores a server from a backup
func (b *BackupService) RestoreBackup(serverID string, backupPath string) error {
	server, err := b.repo.FindByID(serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Server must be stopped
	if server.Status != "stopped" {
		return fmt.Errorf("server must be stopped before restoring backup")
	}

	serverDir := filepath.Join(b.cfg.ServersBasePath, serverID)

	// Backup current data (just in case)
	tempBackup := serverDir + ".pre-restore"
	if err := os.Rename(serverDir, tempBackup); err != nil {
		return fmt.Errorf("failed to backup current data: %w", err)
	}

	// Unzip backup
	log.Printf("Restoring backup for server %s...", serverID)

	if err := b.unzipArchive(backupPath, serverDir); err != nil {
		// Restore old data on error
		os.RemoveAll(serverDir)
		os.Rename(tempBackup, serverDir)
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	// Remove temporary backup
	os.RemoveAll(tempBackup)

	// Publish event
	events.PublishBackupRestored(serverID, filepath.Base(backupPath))

	log.Printf("Backup restored successfully for server %s", serverID)

	return nil
}

// ListBackups lists all backups for a server
func (b *BackupService) ListBackups(serverID string) ([]BackupInfo, error) {
	pattern := filepath.Join(b.backupDir, fmt.Sprintf("%s-*.zip", serverID))

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	backups := make([]BackupInfo, 0, len(matches))

	for _, path := range matches {
		stat, err := os.Stat(path)
		if err != nil {
			continue
		}

		backups = append(backups, BackupInfo{
			Filename:  filepath.Base(path),
			Path:      path,
			SizeBytes: stat.Size(),
			CreatedAt: stat.ModTime(),
		})
	}

	return backups, nil
}

// DeleteBackup deletes a backup file
func (b *BackupService) DeleteBackup(backupPath string) error {
	return os.Remove(backupPath)
}

// CleanupOldBackups removes backups older than specified days
func (b *BackupService) CleanupOldBackups(serverID string, daysToKeep int) error {
	backups, err := b.ListBackups(serverID)
	if err != nil {
		return err
	}

	cutoffTime := time.Now().AddDate(0, 0, -daysToKeep)

	for _, backup := range backups {
		if backup.CreatedAt.Before(cutoffTime) {
			log.Printf("Deleting old backup: %s", backup.Filename)
			if err := b.DeleteBackup(backup.Path); err != nil {
				log.Printf("Warning: failed to delete backup %s: %v", backup.Filename, err)
			}
		}
	}

	return nil
}

// AutoBackup creates automatic backups for all servers
func (b *BackupService) AutoBackup() error {
	servers, err := b.repo.FindAll()
	if err != nil {
		return err
	}

	for _, server := range servers {
		// Only backup stopped servers
		if server.Status == "stopped" {
			if _, err := b.CreateBackup(server.ID); err != nil {
				log.Printf("Warning: failed to backup server %s: %v", server.ID, err)
			}
		}
	}

	return nil
}

// BackupInfo represents information about a backup
type BackupInfo struct {
	Filename  string    `json:"filename"`
	Path      string    `json:"path"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

// zipDirectory creates a ZIP archive of a directory
func (b *BackupService) zipDirectory(source, target string) error {
	zipFile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip if directory
		if info.IsDir() {
			return nil
		}

		// Create header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// Set relative path
		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		header.Name = relPath

		// Set compression method
		header.Method = zip.Deflate

		// Create writer
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		// Open file
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		// Copy file content
		_, err = io.Copy(writer, file)
		return err
	})
}

// unzipArchive extracts a ZIP archive to a directory
func (b *BackupService) unzipArchive(source, target string) error {
	reader, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		path := filepath.Join(target, file.Name)

		// Create directory if needed
		if file.FileInfo().IsDir() {
			os.MkdirAll(path, file.Mode())
			continue
		}

		// Create parent directory
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		// Create file
		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}

		// Extract file content
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

	return nil
}

// saveRunningServer sends save commands to a running server via RCON
func (b *BackupService) saveRunningServer(server *models.MinecraftServer) error {
	// Connect to RCON
	rconPort := 25575
	rconPassword := "minecraft"

	client, err := rcon.NewClient("localhost", rconPort, rconPassword)
	if err != nil {
		return fmt.Errorf("failed to connect to RCON: %w", err)
	}
	defer client.Close()

	// Disable automatic saving
	if _, err := client.SendCommand("save-off"); err != nil {
		return fmt.Errorf("failed to disable auto-save: %w", err)
	}

	// Force save all chunks
	if _, err := client.SendCommand("save-all flush"); err != nil {
		client.SendCommand("save-on") // Re-enable saving on error
		return fmt.Errorf("failed to save world: %w", err)
	}

	// Wait a moment for save to complete
	time.Sleep(2 * time.Second)

	// Re-enable automatic saving
	if _, err := client.SendCommand("save-on"); err != nil {
		return fmt.Errorf("failed to re-enable auto-save: %w", err)
	}

	log.Printf("Successfully saved world for running server %s", server.ID)
	return nil
}
