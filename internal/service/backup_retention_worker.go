package service

import (
	"context"
	"sync"
	"time"

	"github.com/payperplay/hosting/pkg/logger"
)

// BackupRetentionWorker periodically cleans up expired backups
type BackupRetentionWorker struct {
	backupService *BackupService
	cleanupInterval time.Duration // How often to run cleanup (default: 24h)
	running       bool
	ctx           context.Context
	cancel        context.CancelFunc
	cleanupMutex  sync.Mutex // Prevents concurrent cleanup runs
}

// NewBackupRetentionWorker creates a new backup retention worker
func NewBackupRetentionWorker(backupService *BackupService) *BackupRetentionWorker {
	return &BackupRetentionWorker{
		backupService:   backupService,
		cleanupInterval: 24 * time.Hour, // Run daily
		running:         false,
	}
}

// Start begins the retention worker
func (w *BackupRetentionWorker) Start() {
	if w.running {
		logger.Warn("BACKUP-RETENTION: Worker already running", nil)
		return
	}

	w.ctx, w.cancel = context.WithCancel(context.Background())
	w.running = true

	logger.Info("BACKUP-RETENTION: Starting retention worker", map[string]interface{}{
		"cleanup_interval": w.cleanupInterval,
	})

	// Run immediately on startup
	go w.runCleanup()

	// Then run periodically
	go func() {
		ticker := time.NewTicker(w.cleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.runCleanup()
			case <-w.ctx.Done():
				logger.Info("BACKUP-RETENTION: Worker stopped", nil)
				return
			}
		}
	}()
}

// Stop halts the retention worker
func (w *BackupRetentionWorker) Stop() {
	if !w.running {
		return
	}

	logger.Info("BACKUP-RETENTION: Stopping retention worker", nil)
	w.cancel()
	w.running = false
}

// runCleanup performs the actual cleanup operation
func (w *BackupRetentionWorker) runCleanup() {
	// Prevent concurrent cleanup runs
	if !w.cleanupMutex.TryLock() {
		logger.Warn("BACKUP-RETENTION: Cleanup already in progress, skipping this cycle", nil)
		return
	}
	defer w.cleanupMutex.Unlock()

	logger.Info("BACKUP-RETENTION: Starting cleanup of expired backups", nil)
	startTime := time.Now()

	deletedCount, err := w.backupService.CleanupExpiredBackups()
	if err != nil {
		logger.Error("BACKUP-RETENTION: Cleanup failed", err, nil)
		return
	}

	duration := time.Since(startTime)
	logger.Info("BACKUP-RETENTION: Cleanup completed successfully", map[string]interface{}{
		"deleted_backups": deletedCount,
		"duration_s":      duration.Seconds(),
		"next_cleanup":    time.Now().Add(w.cleanupInterval).Format(time.RFC3339),
	})
}

// SetCleanupInterval allows configuring the cleanup interval (for testing)
func (w *BackupRetentionWorker) SetCleanupInterval(interval time.Duration) {
	w.cleanupInterval = interval
	logger.Info("BACKUP-RETENTION: Cleanup interval updated", map[string]interface{}{
		"new_interval": interval,
	})
}

// GetStats returns statistics about the retention worker
func (w *BackupRetentionWorker) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"running":          w.running,
		"cleanup_interval": w.cleanupInterval.String(),
	}
}
