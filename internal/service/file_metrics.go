package service

import (
	"sync"
	"time"

	"github.com/payperplay/hosting/internal/models"
)

// FileMetrics tracks file upload and management metrics
type FileMetrics struct {
	mu sync.RWMutex

	// Upload metrics
	TotalUploads          int64
	SuccessfulUploads     int64
	FailedUploads         int64
	UploadsByType         map[models.FileType]int64
	TotalUploadedSizeMB   float64
	UploadedSizeByType    map[models.FileType]float64

	// Management metrics
	TotalActivations      int64
	TotalDeactivations    int64
	TotalDeletions        int64
	ActivationsByType     map[models.FileType]int64

	// Performance metrics
	AverageUploadTimeMs   float64
	LastUploadTime        time.Time

	// Error tracking
	RecentErrors          []FileError
	MaxRecentErrors       int
}

// FileError represents an upload error for tracking
type FileError struct {
	Timestamp time.Time
	ServerID  string
	FileType  models.FileType
	Error     string
	UserID    string
}

// FileMetricsSnapshot represents a point-in-time view of metrics
type FileMetricsSnapshot struct {
	TotalUploads          int64                       `json:"total_uploads"`
	SuccessfulUploads     int64                       `json:"successful_uploads"`
	FailedUploads         int64                       `json:"failed_uploads"`
	UploadsByType         map[models.FileType]int64   `json:"uploads_by_type"`
	TotalUploadedSizeMB   float64                     `json:"total_uploaded_size_mb"`
	UploadedSizeByType    map[models.FileType]float64 `json:"uploaded_size_by_type"`
	TotalActivations      int64                       `json:"total_activations"`
	TotalDeactivations    int64                       `json:"total_deactivations"`
	TotalDeletions        int64                       `json:"total_deletions"`
	ActivationsByType     map[models.FileType]int64   `json:"activations_by_type"`
	AverageUploadTimeMs   float64                     `json:"average_upload_time_ms"`
	LastUploadTime        time.Time                   `json:"last_upload_time"`
	RecentErrors          []FileError                 `json:"recent_errors"`
}

// Global metrics instance
var globalFileMetrics = &FileMetrics{
	UploadsByType:      make(map[models.FileType]int64),
	UploadedSizeByType: make(map[models.FileType]float64),
	ActivationsByType:  make(map[models.FileType]int64),
	RecentErrors:       make([]FileError, 0),
	MaxRecentErrors:    50, // Keep last 50 errors
}

// GetFileMetrics returns the global file metrics instance
func GetFileMetrics() *FileMetrics {
	return globalFileMetrics
}

// RecordUploadStart records the start of an upload
func (m *FileMetrics) RecordUploadStart() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalUploads++
}

// RecordUploadSuccess records a successful upload
func (m *FileMetrics) RecordUploadSuccess(fileType models.FileType, sizeMB float64, durationMs float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SuccessfulUploads++
	m.UploadsByType[fileType]++
	m.TotalUploadedSizeMB += sizeMB
	m.UploadedSizeByType[fileType] += sizeMB
	m.LastUploadTime = time.Now()

	// Update average upload time (simple moving average)
	if m.AverageUploadTimeMs == 0 {
		m.AverageUploadTimeMs = durationMs
	} else {
		m.AverageUploadTimeMs = (m.AverageUploadTimeMs + durationMs) / 2
	}
}

// RecordUploadFailure records a failed upload
func (m *FileMetrics) RecordUploadFailure(serverID, userID string, fileType models.FileType, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.FailedUploads++

	// Add to recent errors
	fileError := FileError{
		Timestamp: time.Now(),
		ServerID:  serverID,
		FileType:  fileType,
		Error:     err.Error(),
		UserID:    userID,
	}

	m.RecentErrors = append(m.RecentErrors, fileError)

	// Keep only the last N errors
	if len(m.RecentErrors) > m.MaxRecentErrors {
		m.RecentErrors = m.RecentErrors[len(m.RecentErrors)-m.MaxRecentErrors:]
	}
}

// RecordActivation records a file activation
func (m *FileMetrics) RecordActivation(fileType models.FileType) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalActivations++
	m.ActivationsByType[fileType]++
}

// RecordDeactivation records a file deactivation
func (m *FileMetrics) RecordDeactivation() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalDeactivations++
}

// RecordDeletion records a file deletion
func (m *FileMetrics) RecordDeletion() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalDeletions++
}

// GetSnapshot returns a snapshot of current metrics
func (m *FileMetrics) GetSnapshot() FileMetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Copy maps to avoid race conditions
	uploadsByType := make(map[models.FileType]int64)
	for k, v := range m.UploadsByType {
		uploadsByType[k] = v
	}

	uploadedSizeByType := make(map[models.FileType]float64)
	for k, v := range m.UploadedSizeByType {
		uploadedSizeByType[k] = v
	}

	activationsByType := make(map[models.FileType]int64)
	for k, v := range m.ActivationsByType {
		activationsByType[k] = v
	}

	// Copy recent errors
	recentErrors := make([]FileError, len(m.RecentErrors))
	copy(recentErrors, m.RecentErrors)

	return FileMetricsSnapshot{
		TotalUploads:        m.TotalUploads,
		SuccessfulUploads:   m.SuccessfulUploads,
		FailedUploads:       m.FailedUploads,
		UploadsByType:       uploadsByType,
		TotalUploadedSizeMB: m.TotalUploadedSizeMB,
		UploadedSizeByType:  uploadedSizeByType,
		TotalActivations:    m.TotalActivations,
		TotalDeactivations:  m.TotalDeactivations,
		TotalDeletions:      m.TotalDeletions,
		ActivationsByType:   activationsByType,
		AverageUploadTimeMs: m.AverageUploadTimeMs,
		LastUploadTime:      m.LastUploadTime,
		RecentErrors:        recentErrors,
	}
}

// Reset resets all metrics (for testing or periodic reset)
func (m *FileMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalUploads = 0
	m.SuccessfulUploads = 0
	m.FailedUploads = 0
	m.UploadsByType = make(map[models.FileType]int64)
	m.TotalUploadedSizeMB = 0
	m.UploadedSizeByType = make(map[models.FileType]float64)
	m.TotalActivations = 0
	m.TotalDeactivations = 0
	m.TotalDeletions = 0
	m.ActivationsByType = make(map[models.FileType]int64)
	m.AverageUploadTimeMs = 0
	m.LastUploadTime = time.Time{}
	m.RecentErrors = make([]FileError, 0)
}
