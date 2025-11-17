package service

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/internal/storage"
	"github.com/payperplay/hosting/pkg/config"
	"github.com/payperplay/hosting/pkg/logger"
)

// ArchiveService handles server archiving to Hetzner Storage Box
// Phase 3 Lifecycle: Sleeping > 48h → Compress → Upload → FREE for users
type ArchiveService struct {
	serverRepo  *repository.ServerRepository // Repository for server operations
	storagePath string                       // Local path for temporary archive files
	remotePath  string                       // Remote Storage Box path (SFTP/WebDAV)
	conductor   interface{}                  // Conductor for container operations
	sftpClient  *storage.SFTPClient          // SFTP client for Storage Box (Phase 3b)
}

// NewArchiveService creates a new archive service
func NewArchiveService(serverRepo *repository.ServerRepository, conductor interface{}) *ArchiveService {
	cfg := config.AppConfig

	// Initialize SFTP client if Storage Box is enabled
	var sftpClient *storage.SFTPClient
	if cfg.StorageBoxEnabled {
		client, err := storage.NewSFTPClient(cfg)
		if err != nil {
			logger.Warn("ARCHIVE: Failed to initialize SFTP client, falling back to local storage", map[string]interface{}{
				"error": err.Error(),
			})
		} else {
			sftpClient = client
			logger.Info("ARCHIVE: SFTP client initialized successfully", map[string]interface{}{
				"host": cfg.StorageBoxHost,
				"path": cfg.StorageBoxPath,
			})
		}
	} else {
		logger.Info("ARCHIVE: Storage Box disabled, using local storage fallback", nil)
	}

	return &ArchiveService{
		serverRepo:  serverRepo,
		storagePath: filepath.Join(cfg.ServersBasePath, ".archives"),
		remotePath:  cfg.StorageBoxPath,
		conductor:   conductor,
		sftpClient:  sftpClient,
	}
}

// ArchiveServer archives a sleeping server to Storage Box
// Steps: 1) Compress volume 2) Upload 3) Delete container/volume 4) Update DB
func (s *ArchiveService) ArchiveServer(serverID string) error {
	logger.Info("ARCHIVE: Starting server archiving", map[string]interface{}{
		"server_id": serverID,
	})

	// Get server from database
	server, err := s.getServer(serverID)
	if err != nil {
		return fmt.Errorf("failed to get server: %w", err)
	}

	// Validate server can be archived
	if err := s.canArchive(server); err != nil {
		return fmt.Errorf("server cannot be archived: %w", err)
	}

	// Update status to 'archiving'
	if err := s.updateServerStatus(serverID, models.StatusArchiving); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	// Step 1: Compress server data (world files, configs, etc)
	archivePath, archiveSize, err := s.compressServerData(server)
	if err != nil {
		s.updateServerStatus(serverID, models.StatusError)
		return fmt.Errorf("failed to compress server data: %w", err)
	}

	// Validate archive is not empty (minimum 1KB)
	if archiveSize < 1024 {
		s.updateServerStatus(serverID, models.StatusError)
		// Clean up empty archive file
		os.Remove(archivePath)
		logger.Error("ARCHIVE: Server data is empty or too small to archive", nil, map[string]interface{}{
			"server_id":    serverID,
			"archive_size": archiveSize,
		})
		return fmt.Errorf("server data is empty or corrupted (size: %d bytes)", archiveSize)
	}

	logger.Info("ARCHIVE: Server data compressed", map[string]interface{}{
		"server_id":    serverID,
		"archive_path": archivePath,
		"size_mb":      archiveSize / 1024 / 1024,
	})

	// Step 2: Upload to Hetzner Storage Box (or local fallback for now)
	remotePath, err := s.uploadToStorageBox(archivePath, serverID)
	if err != nil {
		s.updateServerStatus(serverID, models.StatusError)
		return fmt.Errorf("failed to upload to storage box: %w", err)
	}

	logger.Info("ARCHIVE: Uploaded to Storage Box", map[string]interface{}{
		"server_id":   serverID,
		"remote_path": remotePath,
	})

	// Step 3: Delete local container and volume
	if err := s.deleteContainerAndVolume(server); err != nil {
		logger.Warn("ARCHIVE: Failed to delete container/volume (will continue)", map[string]interface{}{
			"server_id": serverID,
			"error":     err.Error(),
		})
	}

	// Step 4: Update server metadata
	if err := s.updateServerArchiveMetadata(serverID, remotePath, archiveSize); err != nil {
		return fmt.Errorf("failed to update archive metadata: %w", err)
	}

	// Step 5: Update status to 'archived'
	if err := s.updateServerStatus(serverID, models.StatusArchived); err != nil {
		return fmt.Errorf("failed to update status to archived: %w", err)
	}

	logger.Info("ARCHIVE: Server successfully archived", map[string]interface{}{
		"server_id":   serverID,
		"size_mb":     archiveSize / 1024 / 1024,
		"remote_path": remotePath,
	})

	return nil
}

// UnarchiveServer restores a server from Storage Box
// Steps: 1) Download 2) Extract 3) Update DB to stopped state 4) Cleanup
func (s *ArchiveService) UnarchiveServer(serverID string) error {
	logger.Info("ARCHIVE: Starting server unarchiving", map[string]interface{}{
		"server_id": serverID,
	})

	// Get server from database
	server, err := s.getServer(serverID)
	if err != nil {
		return fmt.Errorf("failed to get server: %w", err)
	}

	// Validate server is archived
	if server.Status != models.StatusArchived {
		return fmt.Errorf("server is not archived (status: %s)", server.Status)
	}

	// Validate archive metadata exists
	if server.ArchiveLocation == "" {
		return fmt.Errorf("no archive location found for server")
	}

	// Step 1: Download from Storage Box (if using SFTP)
	localArchivePath := filepath.Join(s.storagePath, fmt.Sprintf("%s.tar.gz", serverID))

	// Check if local archive exists (for local storage fallback)
	if _, err := os.Stat(localArchivePath); os.IsNotExist(err) {
		// Archive not local - try to download from Storage Box
		if s.sftpClient != nil {
			logger.Info("ARCHIVE: Local archive not found, downloading from Storage Box", map[string]interface{}{
				"server_id":   serverID,
				"remote_path": server.ArchiveLocation,
			})

			if err := s.downloadFromStorageBox(server.ArchiveLocation, localArchivePath); err != nil {
				return fmt.Errorf("failed to download from storage box: %w", err)
			}
		} else {
			// SFTP disabled and no local file - error
			logger.Error("ARCHIVE: Archive file not found and SFTP disabled", nil, map[string]interface{}{
				"server_id":   serverID,
				"local_path":  localArchivePath,
				"remote_path": server.ArchiveLocation,
			})
			return fmt.Errorf("archive file not found locally and SFTP disabled")
		}
	}

	logger.Info("ARCHIVE: Archive file located", map[string]interface{}{
		"server_id":   serverID,
		"local_path":  localArchivePath,
		"archive_size": server.ArchiveSize,
	})

	// GAP-7: Validate archive integrity BEFORE extraction
	logger.Info("GAP-7: Validating archive integrity", map[string]interface{}{
		"server_id": serverID,
		"archive":   localArchivePath,
	})

	if err := s.validateArchiveIntegrity(localArchivePath); err != nil {
		logger.Error("GAP-7: Archive validation failed - archive is corrupted", err, map[string]interface{}{
			"server_id": serverID,
			"archive":   localArchivePath,
		})
		// TODO: Fallback to backup if available
		return fmt.Errorf("archive validation failed: %w", err)
	}

	logger.Info("GAP-7: Archive integrity validated successfully", map[string]interface{}{
		"server_id": serverID,
	})

	// Step 2: Extract archive (FIX #3: Atomic restore)
	// Extract to temp directory first to prevent data loss on failure
	tempDataPath := filepath.Join(config.AppConfig.ServersBasePath, fmt.Sprintf(".%s.tmp", serverID))
	serverDataPath := filepath.Join(config.AppConfig.ServersBasePath, serverID)

	// Clean up any existing temp directory
	if err := os.RemoveAll(tempDataPath); err != nil && !os.IsNotExist(err) {
		logger.Warn("ARCHIVE: Failed to clean existing temp directory", map[string]interface{}{
			"server_id": serverID,
			"temp_path": tempDataPath,
			"error":     err.Error(),
		})
	}

	// Extract to temp directory
	if err := s.extractArchive(localArchivePath, tempDataPath); err != nil {
		// Extraction failed - clean up temp and return error
		os.RemoveAll(tempDataPath)
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	// Validate extraction succeeded by checking if temp directory has content
	entries, err := os.ReadDir(tempDataPath)
	if err != nil || len(entries) == 0 {
		os.RemoveAll(tempDataPath)
		return fmt.Errorf("extraction validation failed: directory empty or unreadable")
	}

	// GAP-7: Validate critical Minecraft files exist after extraction
	logger.Info("GAP-7: Validating extracted Minecraft world data", map[string]interface{}{
		"server_id": serverID,
		"temp_path": tempDataPath,
	})

	if err := s.validateMinecraftWorld(tempDataPath); err != nil {
		logger.Error("GAP-7: Extracted world validation failed - critical files missing", err, map[string]interface{}{
			"server_id": serverID,
			"temp_path": tempDataPath,
		})
		os.RemoveAll(tempDataPath)
		// TODO: Fallback to backup if available
		return fmt.Errorf("world validation failed: %w", err)
	}

	logger.Info("GAP-7: Extracted world validated successfully", map[string]interface{}{
		"server_id": serverID,
	})

	logger.Info("ARCHIVE: Archive extracted to temp directory, performing atomic swap", map[string]interface{}{
		"server_id":  serverID,
		"temp_path":  tempDataPath,
		"final_path": serverDataPath,
	})

	// Atomic swap: Remove old data and rename temp to final location
	// This ensures we either have old data OR new data, never corrupted state
	if err := os.RemoveAll(serverDataPath); err != nil && !os.IsNotExist(err) {
		os.RemoveAll(tempDataPath)
		return fmt.Errorf("failed to remove old server data: %w", err)
	}

	if err := os.Rename(tempDataPath, serverDataPath); err != nil {
		os.RemoveAll(tempDataPath)
		return fmt.Errorf("failed to move extracted data to final location: %w", err)
	}

	logger.Info("ARCHIVE: Archive extracted successfully (atomic swap complete)", map[string]interface{}{
		"server_id": serverID,
		"data_path": serverDataPath,
	})

	// Step 3: Update server metadata (reset archive state, set to stopped)
	if err := s.clearArchiveMetadata(serverID); err != nil {
		return fmt.Errorf("failed to clear archive metadata: %w", err)
	}

	// Step 4: Delete local archive file (cleanup)
	if err := os.Remove(localArchivePath); err != nil {
		logger.Warn("ARCHIVE: Failed to delete local archive file", map[string]interface{}{
			"server_id": serverID,
			"path":      localArchivePath,
			"error":     err.Error(),
		})
	}

	logger.Info("ARCHIVE: Server successfully unarchived (ready to start)", map[string]interface{}{
		"server_id": serverID,
	})

	return nil
}

// compressServerData compresses server world data to .tar.gz
// Returns: (archivePath, size in bytes, error)
func (s *ArchiveService) compressServerData(server *models.MinecraftServer) (string, int64, error) {
	serverDataPath := filepath.Join(config.AppConfig.ServersBasePath, server.ID)
	archivePath := filepath.Join(s.storagePath, fmt.Sprintf("%s.tar.gz", server.ID))

	// Ensure archive directory exists
	if err := os.MkdirAll(s.storagePath, 0755); err != nil {
		return "", 0, fmt.Errorf("failed to create archive directory: %w", err)
	}

	// Create archive file
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create archive file: %w", err)
	}
	defer archiveFile.Close()

	// Create gzip writer
	gzipWriter := gzip.NewWriter(archiveFile)
	defer gzipWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Walk server data directory and add files to archive
	err = filepath.Walk(serverDataPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories (tar will create them automatically)
		if info.IsDir() {
			return nil
		}

		// Create tar header
		relPath, err := filepath.Rel(serverDataPath, path)
		if err != nil {
			return err
		}

		header := &tar.Header{
			Name:    relPath,
			Size:    info.Size(),
			Mode:    int64(info.Mode()),
			ModTime: info.ModTime(),
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// Copy file content
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		if _, err := io.Copy(tarWriter, file); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return "", 0, fmt.Errorf("failed to compress data: %w", err)
	}

	// Get archive file size
	archiveInfo, err := os.Stat(archivePath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to stat archive: %w", err)
	}

	return archivePath, archiveInfo.Size(), nil
}

// extractArchive extracts a .tar.gz archive to a destination path
func (s *ArchiveService) extractArchive(archivePath, destPath string) error {
	// Open archive file
	archiveFile, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer archiveFile.Close()

	// Create gzip reader
	gzipReader, err := gzip.NewReader(archiveFile)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzipReader)

	// Ensure destination directory exists
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Construct full path
		targetPath := filepath.Join(destPath, header.Name)

		// Create directories if needed
		if header.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}

		// Create file
		outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}

		// Copy content
		if _, err := io.Copy(outFile, tarReader); err != nil {
			outFile.Close()
			return fmt.Errorf("failed to extract file: %w", err)
		}
		outFile.Close()
	}

	return nil
}

// uploadToStorageBox uploads archive to Hetzner Storage Box via SFTP
// Falls back to local storage if SFTP is disabled or fails
func (s *ArchiveService) uploadToStorageBox(localPath, serverID string) (string, error) {
	remoteName := fmt.Sprintf("%s.tar.gz", serverID)

	// Phase 3b: SFTP upload to Hetzner Storage Box
	if s.sftpClient != nil {
		logger.Info("ARCHIVE: Uploading to Hetzner Storage Box via SFTP", map[string]interface{}{
			"local_path":  localPath,
			"remote_name": remoteName,
		})

		remotePath, err := s.sftpClient.Upload(localPath, remoteName)
		if err != nil {
			logger.Error("ARCHIVE: SFTP upload failed, keeping local copy", err, map[string]interface{}{
				"local_path": localPath,
			})
			// Return local path as fallback
			return localPath, nil
		}

		logger.Info("ARCHIVE: Successfully uploaded to Storage Box", map[string]interface{}{
			"remote_path": remotePath,
		})

		// Delete local file after successful upload to save NVMe space
		if err := os.Remove(localPath); err != nil {
			logger.Warn("ARCHIVE: Failed to delete local archive after upload", map[string]interface{}{
				"local_path": localPath,
				"error":      err.Error(),
			})
		}

		return remotePath, nil
	}

	// Phase 3a: Local storage fallback (SFTP disabled)
	remotePath := filepath.Join(s.storagePath, remoteName)

	logger.Info("ARCHIVE: SFTP disabled, using local storage", map[string]interface{}{
		"local_path":  localPath,
		"remote_path": remotePath,
	})

	// Archive is already in the correct location (storagePath)
	// No need to move it
	return remotePath, nil
}

// downloadFromStorageBox downloads archive from Hetzner Storage Box via SFTP
func (s *ArchiveService) downloadFromStorageBox(remotePath, localPath string) error {
	if s.sftpClient == nil {
		return fmt.Errorf("SFTP client not available")
	}

	logger.Info("ARCHIVE: Downloading from Hetzner Storage Box via SFTP", map[string]interface{}{
		"remote_path": remotePath,
		"local_path":  localPath,
	})

	if err := s.sftpClient.Download(remotePath, localPath); err != nil {
		return fmt.Errorf("SFTP download failed: %w", err)
	}

	logger.Info("ARCHIVE: Successfully downloaded from Storage Box", map[string]interface{}{
		"local_path": localPath,
	})

	return nil
}

// canArchive validates if a server can be archived
func (s *ArchiveService) canArchive(server *models.MinecraftServer) error {
	// Only archive sleeping/stopped servers
	if server.Status != models.StatusSleeping && server.Status != models.StatusStopped {
		return fmt.Errorf("server must be sleeping or stopped (current: %s)", server.Status)
	}

	// Check if server has been stopped for at least 48 hours
	// TODO: Implement last_stopped_at check
	// if server.LastStoppedAt != nil {
	// 	timeSinceStopped := time.Since(*server.LastStoppedAt)
	// 	if timeSinceStopped < 48*time.Hour {
	// 		return fmt.Errorf("server must be stopped for 48h (current: %s)", timeSinceStopped)
	// 	}
	// }

	return nil
}

// deleteContainerAndVolume removes container and volume to free NVMe space
func (s *ArchiveService) deleteContainerAndVolume(server *models.MinecraftServer) error {
	// TODO: Implement container/volume deletion via conductor
	logger.Info("ARCHIVE: Container and volume deletion (not yet implemented)", map[string]interface{}{
		"server_id": server.ID,
	})
	return nil
}

// Helper methods for database operations
func (s *ArchiveService) getServer(serverID string) (*models.MinecraftServer, error) {
	return s.serverRepo.FindByID(serverID)
}

func (s *ArchiveService) updateServerStatus(serverID string, status models.ServerStatus) error {
	server, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	server.Status = status

	logger.Info("ARCHIVE: Updating server status", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
	})

	return s.serverRepo.Update(server)
}

func (s *ArchiveService) updateServerArchiveMetadata(serverID, archivePath string, archiveSize int64) error {
	server, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	now := time.Now()
	server.ArchiveLocation = archivePath
	server.ArchiveSize = archiveSize
	server.ArchivedAt = &now
	server.LifecyclePhase = models.PhaseArchived

	logger.Info("ARCHIVE: Updating archive metadata", map[string]interface{}{
		"server_id":    serverID,
		"archive_path": archivePath,
		"archive_size": archiveSize,
	})

	return s.serverRepo.Update(server)
}

func (s *ArchiveService) clearArchiveMetadata(serverID string) error {
	server, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return fmt.Errorf("failed to find server: %w", err)
	}

	// Reset archive metadata
	server.ArchiveLocation = ""
	server.ArchiveSize = 0
	server.ArchivedAt = nil
	server.LifecyclePhase = models.PhaseSleep
	server.Status = models.StatusStopped

	logger.Info("ARCHIVE: Clearing archive metadata", map[string]interface{}{
		"server_id": serverID,
		"new_status": models.StatusStopped,
		"lifecycle_phase": models.PhaseSleep,
	})

	return s.serverRepo.Update(server)
}

// GAP-7: validateArchiveIntegrity checks if archive is valid before extraction
// Uses tar -tzf to list contents without extracting (integrity check)
func (s *ArchiveService) validateArchiveIntegrity(archivePath string) error {
	cmd := exec.Command("tar", "-tzf", archivePath)

	// Run command and check if it succeeds
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("archive integrity check failed: %w (output: %s)", err, string(output))
	}

	// Check if archive contains any files
	if len(output) == 0 {
		return fmt.Errorf("archive is empty")
	}

	logger.Debug("GAP-7: Archive integrity validated", map[string]interface{}{
		"archive": archivePath,
		"files":   strings.Count(string(output), "\n"),
	})

	return nil
}

// GAP-7: validateMinecraftWorld checks if extracted world has critical files
// Validates that the world is usable and not corrupted
func (s *ArchiveService) validateMinecraftWorld(worldPath string) error {
	// Check for level.dat (critical file containing world metadata)
	levelDatPath := filepath.Join(worldPath, "level.dat")
	if _, err := os.Stat(levelDatPath); os.IsNotExist(err) {
		return fmt.Errorf("critical file missing: level.dat")
	}

	// Check for world data directory (region/ or other dimensions)
	// Minecraft worlds should have at least ONE of: region/, DIM-1/, DIM1/
	hasDimension := false
	checkDirs := []string{"region", "DIM-1", "DIM1"}

	for _, dir := range checkDirs {
		dimPath := filepath.Join(worldPath, dir)
		if stat, err := os.Stat(dimPath); err == nil && stat.IsDir() {
			hasDimension = true
			break
		}
	}

	if !hasDimension {
		return fmt.Errorf("no world dimension data found (missing region/, DIM-1/, DIM1/)")
	}

	logger.Debug("GAP-7: Minecraft world validated", map[string]interface{}{
		"world_path": worldPath,
	})

	return nil
}
