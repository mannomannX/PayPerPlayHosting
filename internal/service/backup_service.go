package service

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/internal/storage"
	"github.com/payperplay/hosting/pkg/config"
	"github.com/payperplay/hosting/pkg/logger"
)

// BackupService handles server backups with SFTP integration
type BackupService struct {
	backupRepo   *repository.BackupRepository
	serverRepo   *repository.ServerRepository
	sftpClient   *storage.SFTPClient
	storagePath  string
	quotaService *BackupQuotaService
}

// NewBackupService creates a new backup service
func NewBackupService(
	backupRepo *repository.BackupRepository,
	serverRepo *repository.ServerRepository,
	cfg *config.Config,
	quotaService *BackupQuotaService,
) *BackupService {
	service := &BackupService{
		backupRepo:   backupRepo,
		serverRepo:   serverRepo,
		storagePath:  filepath.Join(cfg.ServersBasePath, ".backups"),
		quotaService: quotaService,
	}

	// Initialize SFTP client if enabled
	if cfg.StorageBoxEnabled {
		sftpClient, err := storage.NewSFTPClient(cfg)
		if err != nil {
			logger.Warn("BACKUP-SERVICE: Failed to initialize SFTP client, using local storage", map[string]interface{}{
				"error": err.Error(),
			})
		} else {
			service.sftpClient = sftpClient
			logger.Info("BACKUP-SERVICE: SFTP client initialized for Storage Box backups", nil)
		}
	}

	// Ensure local backup directory exists (fallback + temp storage)
	if err := os.MkdirAll(service.storagePath, 0755); err != nil {
		logger.Error("BACKUP-SERVICE: Failed to create backup directory", err, map[string]interface{}{
			"path": service.storagePath,
		})
	}

	return service
}

// CreateBackup creates a new backup for a server
// backupType: manual, scheduled, pre-migration, pre-deletion, pre-restore
// description: optional user description
// userID: optional user who requested the backup
// retentionDays: 0 = use default based on type, >0 = custom retention
func (s *BackupService) CreateBackup(
	serverID string,
	backupType models.BackupType,
	description string,
	userID *string,
	retentionDays int,
) (*models.Backup, error) {
	// Validate server exists
	server, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to find server: %w", err)
	}

	// Check quota limits for manual backups
	if userID != nil && backupType == models.BackupTypeManual && s.quotaService != nil {
		canCreate, reason, err := s.quotaService.CanCreateBackup(*userID, backupType)
		if err != nil {
			return nil, fmt.Errorf("failed to check backup quota: %w", err)
		}
		if !canCreate {
			return nil, fmt.Errorf("backup quota exceeded: %s", reason)
		}
	}

	// Set default retention based on type
	if retentionDays == 0 {
		retentionDays = s.getDefaultRetentionDays(backupType)
	}

	// Create backup record
	backup := &models.Backup{
		ID:               uuid.New().String(),
		ServerID:         serverID,
		Type:             backupType,
		Status:           models.BackupStatusPending,
		Description:      description,
		RetentionDays:    retentionDays,
		MinecraftVersion: server.MinecraftVersion,
		ServerType:       string(server.ServerType),
		RAMMb:            server.RAMMb,
		UserID:           userID,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Save to database
	if err := s.backupRepo.Create(backup); err != nil {
		return nil, fmt.Errorf("failed to create backup record: %w", err)
	}

	logger.Info("BACKUP-SERVICE: Backup created", map[string]interface{}{
		"backup_id":   backup.ID,
		"server_id":   serverID,
		"server_name": server.Name,
		"type":        backupType,
		"retention":   retentionDays,
	})

	// Perform backup asynchronously
	go s.performBackup(backup, server)

	return backup, nil
}

// CreateBackupSync creates a backup and waits for it to complete (synchronous)
// This is used for migrations where we need to ensure the backup is complete
func (s *BackupService) CreateBackupSync(
	serverID string,
	backupType models.BackupType,
	description string,
	userID *string,
	retentionDays int,
) (*models.Backup, error) {
	// Validate server exists
	server, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to find server: %w", err)
	}

	// Set default retention based on type
	if retentionDays == 0 {
		retentionDays = s.getDefaultRetentionDays(backupType)
	}

	// Create backup record
	backup := &models.Backup{
		ID:               uuid.New().String(),
		ServerID:         serverID,
		Type:             backupType,
		Status:           models.BackupStatusPending,
		Description:      description,
		RetentionDays:    retentionDays,
		MinecraftVersion: server.MinecraftVersion,
		ServerType:       string(server.ServerType),
		RAMMb:            server.RAMMb,
		UserID:           userID,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Save to database
	if err := s.backupRepo.Create(backup); err != nil {
		return nil, fmt.Errorf("failed to create backup record: %w", err)
	}

	logger.Info("BACKUP-SERVICE: Creating synchronous backup", map[string]interface{}{
		"backup_id":   backup.ID,
		"server_id":   serverID,
		"server_name": server.Name,
		"type":        backupType,
	})

	// Perform backup synchronously (wait for completion)
	s.performBackup(backup, server)

	// Reload backup from database to get updated status
	backup, err = s.backupRepo.FindByID(backup.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload backup: %w", err)
	}

	// Check if backup succeeded
	if backup.Status != models.BackupStatusCompleted {
		return nil, fmt.Errorf("backup failed: %s", backup.ErrorMessage)
	}

	return backup, nil
}

// performBackup performs the actual backup operation
func (s *BackupService) performBackup(backup *models.Backup, server *models.MinecraftServer) {
	// Update status to creating
	backup.Status = models.BackupStatusCreating
	backup.UpdatedAt = time.Now()
	s.backupRepo.Update(backup)

	logger.Info("BACKUP-SERVICE: Starting backup creation", map[string]interface{}{
		"backup_id":   backup.ID,
		"server_id":   server.ID,
		"server_name": server.Name,
	})

	// 1. Get server directory path
	serverPath := filepath.Join(s.storagePath, "..", server.ID)
	if _, err := os.Stat(serverPath); os.IsNotExist(err) {
		s.markBackupFailed(backup, fmt.Sprintf("server directory not found: %s", serverPath))
		return
	}

	// 2. Calculate original size
	originalSize, err := s.calculateDirectorySize(serverPath)
	if err != nil {
		logger.Warn("BACKUP-SERVICE: Failed to calculate original size", map[string]interface{}{
			"backup_id": backup.ID,
			"error":     err.Error(),
		})
		originalSize = 0
	}
	backup.OriginalSize = originalSize

	// 3. Create compressed backup locally
	localPath := filepath.Join(s.storagePath, fmt.Sprintf("%s.tar.gz", backup.ID))
	compressedSize, err := s.compressServerData(serverPath, localPath)
	if err != nil {
		s.markBackupFailed(backup, fmt.Sprintf("failed to compress data: %v", err))
		return
	}
	backup.CompressedSize = compressedSize

	logger.Info("BACKUP-SERVICE: Server data compressed", map[string]interface{}{
		"backup_id":        backup.ID,
		"original_mb":      originalSize / 1024 / 1024,
		"compressed_mb":    compressedSize / 1024 / 1024,
		"compression_pct":  backup.GetCompressionRatio(),
	})

	// 4. Upload to Storage Box (or keep locally)
	remotePath, err := s.uploadBackup(localPath, backup.ID)
	if err != nil {
		s.markBackupFailed(backup, fmt.Sprintf("failed to upload backup: %v", err))
		return
	}
	backup.StoragePath = remotePath

	// 5. Set expiration time
	expiresAt := backup.CalculateExpiresAt()
	backup.ExpiresAt = &expiresAt

	// 6. Mark as completed
	backup.Status = models.BackupStatusCompleted
	backup.CompletedAt = timePtr(time.Now())
	backup.UpdatedAt = time.Now()

	if err := s.backupRepo.Update(backup); err != nil {
		logger.Error("BACKUP-SERVICE: Failed to update backup record", err, map[string]interface{}{
			"backup_id": backup.ID,
		})
		return
	}

	logger.Info("BACKUP-SERVICE: Backup completed successfully", map[string]interface{}{
		"backup_id":      backup.ID,
		"server_id":      server.ID,
		"compressed_mb":  compressedSize / 1024 / 1024,
		"storage_path":   remotePath,
		"expires_at":     expiresAt.Format(time.RFC3339),
	})
}

// compressServerData compresses server directory to tar.gz
func (s *BackupService) compressServerData(sourcePath, targetPath string) (int64, error) {
	startTime := time.Now()

	// Create output file
	outFile, err := os.Create(targetPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create archive file: %w", err)
	}
	defer outFile.Close()

	// Create gzip writer
	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Walk directory and add files
	err = filepath.Walk(sourcePath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("failed to create tar header: %w", err)
		}

		// Set relative path
		relPath, err := filepath.Rel(sourcePath, filePath)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}
		header.Name = relPath

		// Write header
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header: %w", err)
		}

		// Write file content (if not directory)
		if !info.IsDir() {
			file, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			defer file.Close()

			if _, err := io.Copy(tarWriter, file); err != nil {
				return fmt.Errorf("failed to write file to tar: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to compress directory: %w", err)
	}

	// Get compressed file size
	fileInfo, err := outFile.Stat()
	if err != nil {
		return 0, fmt.Errorf("failed to stat compressed file: %w", err)
	}

	duration := time.Since(startTime)
	logger.Debug("BACKUP-SERVICE: Compression completed", map[string]interface{}{
		"source":      sourcePath,
		"target":      targetPath,
		"size_mb":     fileInfo.Size() / 1024 / 1024,
		"duration_s":  duration.Seconds(),
	})

	return fileInfo.Size(), nil
}

// uploadBackup uploads backup to Storage Box or keeps locally
func (s *BackupService) uploadBackup(localPath, backupID string) (string, error) {
	remoteName := fmt.Sprintf("backup-%s.tar.gz", backupID)

	// If SFTP enabled, upload to Storage Box
	if s.sftpClient != nil {
		remotePath, err := s.sftpClient.Upload(localPath, remoteName)
		if err != nil {
			logger.Warn("BACKUP-SERVICE: SFTP upload failed, falling back to local storage", map[string]interface{}{
				"backup_id": backupID,
				"error":     err.Error(),
			})
			// Keep local file as fallback
			return localPath, nil
		}

		// Delete local file after successful upload to save space
		if err := os.Remove(localPath); err != nil {
			logger.Warn("BACKUP-SERVICE: Failed to delete local backup after upload", map[string]interface{}{
				"backup_id":  backupID,
				"local_path": localPath,
				"error":      err.Error(),
			})
		}

		return remotePath, nil
	}

	// Local storage mode
	return localPath, nil
}

// RestoreBackup restores a backup to a server directory
// userID is optional - if provided, quota limits will be checked and restore will be tracked
func (s *BackupService) RestoreBackup(backupID string, targetServerID string, userID *string) error {
	// Find backup record
	backup, err := s.backupRepo.FindByID(backupID)
	if err != nil {
		return fmt.Errorf("failed to find backup: %w", err)
	}

	if backup.Status != models.BackupStatusCompleted {
		return fmt.Errorf("backup is not in completed state: %s", backup.Status)
	}

	// Check restore quota if userID provided
	if userID != nil && s.quotaService != nil {
		canRestore, reason, err := s.quotaService.CanRestoreBackup(*userID)
		if err != nil {
			return fmt.Errorf("failed to check restore quota: %w", err)
		}
		if !canRestore {
			return fmt.Errorf("restore quota exceeded: %s", reason)
		}
	}

	logger.Info("BACKUP-SERVICE: Starting backup restore", map[string]interface{}{
		"backup_id":        backupID,
		"target_server_id": targetServerID,
		"storage_path":     backup.StoragePath,
		"user_id":          userID,
	})

	// Determine if backup is on Storage Box or local
	isRemote := s.sftpClient != nil && !filepath.IsAbs(backup.StoragePath)

	var localPath string
	if isRemote {
		// Download from Storage Box
		localPath = filepath.Join(s.storagePath, fmt.Sprintf("restore-%s.tar.gz", backupID))
		if err := s.sftpClient.Download(backup.StoragePath, localPath); err != nil {
			return fmt.Errorf("failed to download backup from Storage Box: %w", err)
		}
		defer os.Remove(localPath) // Cleanup after restore
	} else {
		// Use local file
		localPath = backup.StoragePath
	}

	// Extract to server directory
	targetPath := filepath.Join(s.storagePath, "..", targetServerID)
	if err := s.extractBackup(localPath, targetPath); err != nil {
		return fmt.Errorf("failed to extract backup: %w", err)
	}

	// Track restore operation for quota management
	if userID != nil && s.quotaService != nil {
		// Get server name for tracking
		server, err := s.serverRepo.FindByID(targetServerID)
		serverName := "Unknown"
		if err == nil && server != nil {
			serverName = server.Name
		}

		if err := s.quotaService.TrackRestore(*userID, backupID, targetServerID, serverName, backup.Type); err != nil {
			logger.Warn("BACKUP-SERVICE: Failed to track restore operation", map[string]interface{}{
				"backup_id": backupID,
				"user_id":   *userID,
				"error":     err.Error(),
			})
			// Don't fail the restore if tracking fails
		}
	}

	// Update backup metadata
	now := time.Now()
	backup.RestoredAt = &now
	backup.RestoredCount++
	if err := s.backupRepo.Update(backup); err != nil {
		logger.Warn("BACKUP-SERVICE: Failed to update backup metadata", map[string]interface{}{
			"backup_id": backupID,
			"error":     err.Error(),
		})
	}

	logger.Info("BACKUP-SERVICE: Backup restored successfully", map[string]interface{}{
		"backup_id":        backupID,
		"target_server_id": targetServerID,
		"target_path":      targetPath,
	})

	return nil
}

// RestoreBackupToNode restores a backup to a remote node via SSH/SCP
// This is used during migrations to transfer world data to the target node
func (s *BackupService) RestoreBackupToNode(backupID string, nodeIPAddress string, targetServerID string) error {
	// Find backup record
	backup, err := s.backupRepo.FindByID(backupID)
	if err != nil {
		return fmt.Errorf("failed to find backup: %w", err)
	}

	if backup.Status != models.BackupStatusCompleted {
		return fmt.Errorf("backup is not in completed state: %s", backup.Status)
	}

	logger.Info("BACKUP-SERVICE: Starting remote backup restore", map[string]interface{}{
		"backup_id":        backupID,
		"target_server_id": targetServerID,
		"target_node":      nodeIPAddress,
		"storage_path":     backup.StoragePath,
	})

	// 1. Download backup to local temp directory if on Storage Box
	isRemote := s.sftpClient != nil && !filepath.IsAbs(backup.StoragePath)
	var localPath string

	if isRemote {
		// Download from Storage Box
		localPath = filepath.Join(s.storagePath, fmt.Sprintf("migrate-%s.tar.gz", backupID))
		if err := s.sftpClient.Download(backup.StoragePath, localPath); err != nil {
			return fmt.Errorf("failed to download backup from Storage Box: %w", err)
		}
		defer os.Remove(localPath) // Cleanup after transfer
	} else {
		// Use local file
		localPath = backup.StoragePath
	}

	logger.Info("BACKUP-SERVICE: Backup ready for transfer", map[string]interface{}{
		"backup_id":  backupID,
		"local_path": localPath,
		"size_mb":    backup.CompressedSize / 1024 / 1024,
	})

	// 2. Create target directory on remote node
	targetDir := fmt.Sprintf("/minecraft/servers/%s", targetServerID)
	createDirCmd := fmt.Sprintf("ssh root@%s 'mkdir -p %s'", nodeIPAddress, targetDir)

	if err := s.executeSSHCommand(createDirCmd); err != nil {
		return fmt.Errorf("failed to create remote directory: %w", err)
	}

	// 3. Transfer backup to remote node
	remoteTempPath := fmt.Sprintf("/tmp/backup-%s.tar.gz", backupID)
	scpCmd := fmt.Sprintf("scp %s root@%s:%s", localPath, nodeIPAddress, remoteTempPath)

	logger.Info("BACKUP-SERVICE: Transferring backup to remote node", map[string]interface{}{
		"backup_id":   backupID,
		"target_node": nodeIPAddress,
		"size_mb":     backup.CompressedSize / 1024 / 1024,
	})

	if err := s.executeSSHCommand(scpCmd); err != nil {
		return fmt.Errorf("failed to transfer backup to remote node: %w", err)
	}

	// 4. Extract backup on remote node
	extractCmd := fmt.Sprintf("ssh root@%s 'cd %s && tar -xzf %s && rm %s'",
		nodeIPAddress,
		targetDir,
		remoteTempPath,
		remoteTempPath,
	)

	logger.Info("BACKUP-SERVICE: Extracting backup on remote node", map[string]interface{}{
		"backup_id":   backupID,
		"target_node": nodeIPAddress,
		"target_dir":  targetDir,
	})

	if err := s.executeSSHCommand(extractCmd); err != nil {
		return fmt.Errorf("failed to extract backup on remote node: %w", err)
	}

	logger.Info("BACKUP-SERVICE: Remote backup restore completed successfully", map[string]interface{}{
		"backup_id":        backupID,
		"target_server_id": targetServerID,
		"target_node":      nodeIPAddress,
		"target_dir":       targetDir,
	})

	// Update backup restored stats
	now := time.Now()
	backup.RestoredAt = &now
	backup.RestoredCount++
	s.backupRepo.Update(backup)

	return nil
}

// executeSSHCommand executes a shell command (used for SSH/SCP operations)
func (s *BackupService) executeSSHCommand(command string) error {
	cmd := exec.Command("bash", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %w, output: %s", err, string(output))
	}
	return nil
}

// extractBackup extracts tar.gz backup to target directory
func (s *BackupService) extractBackup(archivePath, targetPath string) error {
	startTime := time.Now()

	// Ensure target directory exists
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Open archive file
	archiveFile, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer archiveFile.Close()

	// Create gzip reader
	gzReader, err := gzip.NewReader(archiveFile)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	// Extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		targetFilePath := filepath.Join(targetPath, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(targetFilePath, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}

		case tar.TypeReg:
			// Create file
			outFile, err := os.Create(targetFilePath)
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to extract file: %w", err)
			}

			outFile.Close()
		}
	}

	duration := time.Since(startTime)
	logger.Debug("BACKUP-SERVICE: Extraction completed", map[string]interface{}{
		"archive":     archivePath,
		"target":      targetPath,
		"duration_s":  duration.Seconds(),
	})

	return nil
}

// DeleteBackup deletes a backup from storage and database
func (s *BackupService) DeleteBackup(backupID string) error {
	backup, err := s.backupRepo.FindByID(backupID)
	if err != nil {
		return fmt.Errorf("failed to find backup: %w", err)
	}

	logger.Info("BACKUP-SERVICE: Deleting backup", map[string]interface{}{
		"backup_id":    backupID,
		"storage_path": backup.StoragePath,
	})

	// Determine if backup is on Storage Box or local
	isRemote := s.sftpClient != nil && !filepath.IsAbs(backup.StoragePath)

	if isRemote {
		// Delete from Storage Box
		if err := s.sftpClient.Delete(backup.StoragePath); err != nil {
			logger.Warn("BACKUP-SERVICE: Failed to delete from Storage Box", map[string]interface{}{
				"backup_id": backupID,
				"error":     err.Error(),
			})
		}
	} else {
		// Delete local file
		if err := os.Remove(backup.StoragePath); err != nil && !os.IsNotExist(err) {
			logger.Warn("BACKUP-SERVICE: Failed to delete local backup file", map[string]interface{}{
				"backup_id": backupID,
				"path":      backup.StoragePath,
				"error":     err.Error(),
			})
		}
	}

	// Mark as deleted in database
	if err := s.backupRepo.MarkAsDeleted(backupID); err != nil {
		return fmt.Errorf("failed to mark backup as deleted: %w", err)
	}

	logger.Info("BACKUP-SERVICE: Backup deleted successfully", map[string]interface{}{
		"backup_id": backupID,
	})

	return nil
}

// CleanupExpiredBackups deletes expired backups
func (s *BackupService) CleanupExpiredBackups() (int, error) {
	logger.Info("BACKUP-SERVICE: Starting cleanup of expired backups", nil)

	expiredBackups, err := s.backupRepo.FindExpired()
	if err != nil {
		return 0, fmt.Errorf("failed to find expired backups: %w", err)
	}

	if len(expiredBackups) == 0 {
		logger.Info("BACKUP-SERVICE: No expired backups to cleanup", nil)
		return 0, nil
	}

	deletedCount := 0
	for _, backup := range expiredBackups {
		if err := s.DeleteBackup(backup.ID); err != nil {
			logger.Error("BACKUP-SERVICE: Failed to delete expired backup", err, map[string]interface{}{
				"backup_id": backup.ID,
			})
		} else {
			deletedCount++
		}
	}

	logger.Info("BACKUP-SERVICE: Expired backups cleanup completed", map[string]interface{}{
		"total_expired": len(expiredBackups),
		"deleted":       deletedCount,
		"failed":        len(expiredBackups) - deletedCount,
	})

	return deletedCount, nil
}

// GetBackupStats returns statistics about backups
func (s *BackupService) GetBackupStats() (map[string]interface{}, error) {
	totalSize, err := s.backupRepo.GetTotalBackupSize()
	if err != nil {
		return nil, fmt.Errorf("failed to get total backup size: %w", err)
	}

	return map[string]interface{}{
		"total_size_mb":  totalSize / 1024 / 1024,
		"total_size_gb":  float64(totalSize) / 1024 / 1024 / 1024,
		"storage_mode":   s.getStorageMode(),
	}, nil
}

// Helper methods

func (s *BackupService) markBackupFailed(backup *models.Backup, errorMsg string) {
	backup.Status = models.BackupStatusFailed
	backup.ErrorMessage = errorMsg
	backup.UpdatedAt = time.Now()
	s.backupRepo.Update(backup)

	logger.Error("BACKUP-SERVICE: Backup failed", nil, map[string]interface{}{
		"backup_id": backup.ID,
		"error":     errorMsg,
	})
}

func (s *BackupService) getDefaultRetentionDays(backupType models.BackupType) int {
	switch backupType {
	case models.BackupTypeManual:
		return 30 // Keep manual backups for 30 days
	case models.BackupTypeScheduled:
		return 7 // Keep scheduled backups for 7 days
	case models.BackupTypePreMigration:
		return 7 // Keep pre-migration backups for 7 days
	case models.BackupTypePreDeletion:
		return 30 // Keep pre-deletion backups for 30 days (safety)
	case models.BackupTypePreRestore:
		return 7 // Keep pre-restore backups for 7 days
	default:
		return 7
	}
}

func (s *BackupService) calculateDirectorySize(path string) (int64, error) {
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

func (s *BackupService) getStorageMode() string {
	if s.sftpClient != nil {
		return "sftp"
	}
	return "local"
}

func timePtr(t time.Time) *time.Time {
	return &t
}
