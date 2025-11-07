package service

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/logger"
)

// FileService handles file uploads and management
type FileService struct {
	fileRepo   *repository.FileRepository
	serverRepo *repository.ServerRepository
	baseDir    string // Base directory for all server files
}

// NewFileService creates a new file service
func NewFileService(
	fileRepo *repository.FileRepository,
	serverRepo *repository.ServerRepository,
	baseDir string,
) *FileService {
	return &FileService{
		fileRepo:   fileRepo,
		serverRepo: serverRepo,
		baseDir:    baseDir,
	}
}

// UploadFileRequest represents a file upload request
type UploadFileRequest struct {
	ServerID   string
	UserID     string
	FileType   models.FileType
	File       multipart.File
	Header     *multipart.FileHeader
	Metadata   string // JSON metadata (optional)
	AutoActivate bool  // Automatically activate after upload
}

// UploadFile uploads and validates a file for a server
func (s *FileService) UploadFile(req UploadFileRequest) (*models.ServerFile, error) {
	// Track metrics
	metrics := GetFileMetrics()
	metrics.RecordUploadStart()
	startTime := time.Now()

	// 1. Validate server exists
	_, err := s.serverRepo.FindByID(req.ServerID)
	if err != nil {
		metrics.RecordUploadFailure(req.ServerID, req.UserID, req.FileType, err)
		return nil, fmt.Errorf("server not found: %w", err)
	}

	// 2. Get validator for file type
	validator, err := GetValidatorForFileType(req.FileType)
	if err != nil {
		metrics.RecordUploadFailure(req.ServerID, req.UserID, req.FileType, err)
		return nil, err
	}

	// 3. Validate file
	if err := validator.Validate(req.File, req.Header); err != nil {
		metrics.RecordUploadFailure(req.ServerID, req.UserID, req.FileType, err)
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// 4. Calculate SHA1 hash
	sha1Hash, err := CalculateSHA1(req.File)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate SHA1: %w", err)
	}

	// 5. Create file record
	fileID := uuid.New().String()[:16]
	sizeMB := float64(req.Header.Size) / 1024.0 / 1024.0

	serverFile := &models.ServerFile{
		ID:         fileID,
		ServerID:   req.ServerID,
		FileType:   req.FileType,
		FileName:   req.Header.Filename,
		Status:     models.FileStatusUploading,
		SHA1Hash:   sha1Hash,
		SizeMB:     sizeMB,
		Version:    1,
		IsActive:   false,
		Metadata:   req.Metadata,
		UploadedBy: req.UserID,
		UploadedAt: time.Now(),
	}

	// 6. Save to database
	if err := s.fileRepo.Create(serverFile); err != nil {
		return nil, fmt.Errorf("failed to create file record: %w", err)
	}

	// 7. Save file to disk
	filePath, err := s.saveFileToDisk(req.ServerID, req.FileType, fileID, req.Header.Filename, req.File)
	if err != nil {
		s.fileRepo.UpdateStatus(fileID, models.FileStatusFailed, err.Error())
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	serverFile.FilePath = filePath

	// 8. Update file record with path
	if err := s.fileRepo.Update(serverFile); err != nil {
		return nil, fmt.Errorf("failed to update file record: %w", err)
	}

	// 9. Mark as processing
	s.fileRepo.UpdateStatus(fileID, models.FileStatusProcessing, "")

	// 10. Auto-activate if requested
	if req.AutoActivate {
		if err := s.ActivateFile(fileID, req.ServerID); err != nil {
			logger.Warn("Failed to auto-activate file", map[string]interface{}{
				"file_id":   fileID,
				"server_id": req.ServerID,
				"error":     err.Error(),
			})
		}
	} else {
		// Mark as inactive but ready
		s.fileRepo.UpdateStatus(fileID, models.FileStatusInactive, "")
	}

	logger.Info("File uploaded successfully", map[string]interface{}{
		"file_id":   fileID,
		"server_id": req.ServerID,
		"file_type": req.FileType,
		"size_mb":   sizeMB,
	})

	// Record successful upload metrics
	uploadDuration := time.Since(startTime).Milliseconds()
	metrics.RecordUploadSuccess(req.FileType, sizeMB, float64(uploadDuration))

	return serverFile, nil
}

// saveFileToDisk saves the uploaded file to disk
func (s *FileService) saveFileToDisk(serverID string, fileType models.FileType, fileID, fileName string, file multipart.File) (string, error) {
	// Determine storage path
	// Example: /app/minecraft/servers/{serverID}/files/resource_packs/{fileID}-{fileName}
	typeDir := s.getTypeDirName(fileType)
	serverFileDir := filepath.Join(s.baseDir, serverID, "files", typeDir)

	// Create directory if not exists
	if err := os.MkdirAll(serverFileDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Create destination file
	destFileName := fmt.Sprintf("%s-%s", fileID, fileName)
	destPath := filepath.Join(serverFileDir, destFileName)

	dest, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dest.Close()

	// Copy file content
	if _, err := io.Copy(dest, file); err != nil {
		return "", fmt.Errorf("failed to copy file content: %w", err)
	}

	// Return relative path from server directory
	relativePath := filepath.Join("files", typeDir, destFileName)
	return relativePath, nil
}

// getTypeDirName returns the directory name for a file type
func (s *FileService) getTypeDirName(fileType models.FileType) string {
	switch fileType {
	case models.FileTypeResourcePack:
		return "resource_packs"
	case models.FileTypeDataPack:
		return "data_packs"
	case models.FileTypeServerIcon:
		return "icons"
	case models.FileTypeWorldGen:
		return "world_gen"
	default:
		return "other"
	}
}

// ActivateFile activates a file (deactivates others of same type)
func (s *FileService) ActivateFile(fileID, serverID string) error {
	// Get file
	file, err := s.fileRepo.FindByID(fileID)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	// Verify server ID matches
	if file.ServerID != serverID {
		return fmt.Errorf("file does not belong to this server")
	}

	// Deactivate all other files of same type
	if err := s.fileRepo.DeactivateAllOfType(serverID, file.FileType); err != nil {
		return fmt.Errorf("failed to deactivate other files: %w", err)
	}

	// Activate this file
	file.IsActive = true
	file.Status = models.FileStatusActive
	if err := s.fileRepo.Update(file); err != nil {
		return fmt.Errorf("failed to activate file: %w", err)
	}

	logger.Info("File activated", map[string]interface{}{
		"file_id":   fileID,
		"server_id": serverID,
		"file_type": file.FileType,
	})

	// Record activation metrics
	GetFileMetrics().RecordActivation(file.FileType)

	return nil
}

// DeactivateFile deactivates a file
func (s *FileService) DeactivateFile(fileID, serverID string) error {
	// Get file
	file, err := s.fileRepo.FindByID(fileID)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	// Verify server ID matches
	if file.ServerID != serverID {
		return fmt.Errorf("file does not belong to this server")
	}

	// Deactivate
	file.IsActive = false
	file.Status = models.FileStatusInactive
	if err := s.fileRepo.Update(file); err != nil {
		return fmt.Errorf("failed to deactivate file: %w", err)
	}

	// Record deactivation metrics
	GetFileMetrics().RecordDeactivation()

	return nil
}

// DeleteFile deletes a file
func (s *FileService) DeleteFile(fileID, serverID string) error {
	// Get file
	file, err := s.fileRepo.FindByID(fileID)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	// Verify server ID matches
	if file.ServerID != serverID {
		return fmt.Errorf("file does not belong to this server")
	}

	// Delete from disk
	fullPath := filepath.Join(s.baseDir, serverID, file.FilePath)
	if err := os.Remove(fullPath); err != nil {
		logger.Warn("Failed to delete file from disk", map[string]interface{}{
			"file_id": fileID,
			"path":    fullPath,
			"error":   err.Error(),
		})
		// Continue with database deletion even if file removal fails
	}

	// Delete from database
	if err := s.fileRepo.Delete(fileID); err != nil {
		return fmt.Errorf("failed to delete file record: %w", err)
	}

	logger.Info("File deleted", map[string]interface{}{
		"file_id":   fileID,
		"server_id": serverID,
	})

	// Record deletion metrics
	GetFileMetrics().RecordDeletion()

	return nil
}

// GetFilesByServer returns all files for a server
func (s *FileService) GetFilesByServer(serverID string) ([]models.ServerFile, error) {
	return s.fileRepo.FindByServerID(serverID)
}

// GetFilesByServerAndType returns files for a server filtered by type
func (s *FileService) GetFilesByServerAndType(serverID string, fileType models.FileType) ([]models.ServerFile, error) {
	return s.fileRepo.FindByServerIDAndType(serverID, fileType)
}

// GetActiveFile returns the active file for a server and type
func (s *FileService) GetActiveFile(serverID string, fileType models.FileType) (*models.ServerFile, error) {
	return s.fileRepo.FindActiveByServerIDAndType(serverID, fileType)
}

// GetFilePath returns the full file path
func (s *FileService) GetFilePath(serverID, fileID string) (string, error) {
	file, err := s.fileRepo.FindByID(fileID)
	if err != nil {
		return "", err
	}

	if file.ServerID != serverID {
		return "", fmt.Errorf("file does not belong to this server")
	}

	return filepath.Join(s.baseDir, serverID, file.FilePath), nil
}

// ParseFileType converts a string to FileType
func ParseFileType(typeStr string) (models.FileType, error) {
	typeStr = strings.ToLower(strings.TrimSpace(typeStr))

	switch typeStr {
	case "resource_pack", "resourcepack":
		return models.FileTypeResourcePack, nil
	case "data_pack", "datapack":
		return models.FileTypeDataPack, nil
	case "server_icon", "icon":
		return models.FileTypeServerIcon, nil
	case "world_gen", "worldgen":
		return models.FileTypeWorldGen, nil
	default:
		return "", fmt.Errorf("invalid file type: %s", typeStr)
	}
}

// GetServerIconPath returns the path to the server's icon file
func (s *FileService) GetServerIconPath(serverID string) (string, error) {
	// Verify server exists
	_, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return "", fmt.Errorf("server not found: %w", err)
	}

	// Path to server-icon.png in server root
	iconPath := filepath.Join(s.baseDir, serverID, "server-icon.png")

	// Check if icon exists
	if _, err := os.Stat(iconPath); os.IsNotExist(err) {
		return "", fmt.Errorf("server icon not found")
	}

	return iconPath, nil
}
