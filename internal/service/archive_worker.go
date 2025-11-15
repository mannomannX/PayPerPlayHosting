package service

import (
	"context"
	"sync"
	"time"

	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/logger"
)

// ArchiveWorker periodically scans for servers eligible for archiving
// Criteria: Status = sleeping/stopped AND last_stopped_at > 48 hours
type ArchiveWorker struct {
	serverRepo     *repository.ServerRepository
	archiveService *ArchiveService
	scanInterval   time.Duration
	archiveAfter   time.Duration // Duration after which to archive (default: 48h)
	running        bool
	ctx            context.Context
	cancel         context.CancelFunc
	scanMutex      sync.Mutex                // Prevents concurrent scans
	archivingSet   map[string]bool           // Tracks servers currently being archived
	archivingMutex sync.Mutex                // Protects archivingSet
}

// NewArchiveWorker creates a new archive worker
func NewArchiveWorker(serverRepo *repository.ServerRepository, archiveService *ArchiveService) *ArchiveWorker {
	return &ArchiveWorker{
		serverRepo:     serverRepo,
		archiveService: archiveService,
		scanInterval:   1 * time.Hour,  // Scan every hour
		archiveAfter:   48 * time.Hour, // Archive after 48h of sleeping
		running:        false,
		archivingSet:   make(map[string]bool), // Track in-progress archives
	}
}

// Start begins the archive worker
func (w *ArchiveWorker) Start() {
	if w.running {
		logger.Warn("ARCHIVE-WORKER: Already running", nil)
		return
	}

	w.ctx, w.cancel = context.WithCancel(context.Background())
	w.running = true

	logger.Info("ARCHIVE-WORKER: Starting archive worker", map[string]interface{}{
		"scan_interval":  w.scanInterval,
		"archive_after":  w.archiveAfter,
	})

	// Run immediately on startup
	go w.scanAndArchive()

	// Then run periodically
	go func() {
		ticker := time.NewTicker(w.scanInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.scanAndArchive()
			case <-w.ctx.Done():
				logger.Info("ARCHIVE-WORKER: Stopped", nil)
				return
			}
		}
	}()
}

// Stop halts the archive worker
func (w *ArchiveWorker) Stop() {
	if !w.running {
		return
	}

	logger.Info("ARCHIVE-WORKER: Stopping archive worker", nil)
	w.cancel()
	w.running = false
}

// scanAndArchive scans for eligible servers and archives them
func (w *ArchiveWorker) scanAndArchive() {
	// Prevent concurrent scans
	if !w.scanMutex.TryLock() {
		logger.Warn("ARCHIVE-WORKER: Scan already in progress, skipping this cycle", nil)
		return
	}
	defer w.scanMutex.Unlock()

	logger.Info("ARCHIVE-WORKER: Starting archive scan", nil)

	// Find all sleeping servers
	sleepingServers, err := w.serverRepo.FindByStatus(string(models.StatusSleeping))
	if err != nil {
		logger.Error("ARCHIVE-WORKER: Failed to fetch sleeping servers", err, nil)
		return
	}

	// Also check stopped servers (same lifecycle phase)
	stoppedServers, err := w.serverRepo.FindByStatus(string(models.StatusStopped))
	if err != nil {
		logger.Error("ARCHIVE-WORKER: Failed to fetch stopped servers", err, nil)
		return
	}

	// Combine both lists
	candidateServers := append(sleepingServers, stoppedServers...)

	logger.Info("ARCHIVE-WORKER: Found candidate servers", map[string]interface{}{
		"sleeping_servers": len(sleepingServers),
		"stopped_servers":  len(stoppedServers),
		"total_candidates": len(candidateServers),
	})

	eligibleCount := 0
	archivedCount := 0
	errorCount := 0

	for _, server := range candidateServers {
		// Check if server is eligible for archiving
		if !w.isEligibleForArchiving(&server) {
			continue
		}

		// Check if already being archived (race condition prevention)
		w.archivingMutex.Lock()
		if w.archivingSet[server.ID] {
			w.archivingMutex.Unlock()
			logger.Debug("ARCHIVE-WORKER: Server already being archived, skipping", map[string]interface{}{
				"server_id":   server.ID,
				"server_name": server.Name,
			})
			continue
		}
		w.archivingSet[server.ID] = true
		w.archivingMutex.Unlock()

		eligibleCount++

		logger.Info("ARCHIVE-WORKER: Archiving eligible server", map[string]interface{}{
			"server_id":       server.ID,
			"server_name":     server.Name,
			"status":          server.Status,
			"last_stopped_at": server.LastStoppedAt,
			"stopped_for":     time.Since(*server.LastStoppedAt).Round(time.Hour),
		})

		// Archive the server
		if err := w.archiveService.ArchiveServer(server.ID); err != nil {
			logger.Error("ARCHIVE-WORKER: Failed to archive server", err, map[string]interface{}{
				"server_id":   server.ID,
				"server_name": server.Name,
			})
			errorCount++
		} else {
			archivedCount++
			logger.Info("ARCHIVE-WORKER: Server archived successfully", map[string]interface{}{
				"server_id":   server.ID,
				"server_name": server.Name,
			})
		}

		// Remove from archiving set (whether success or failure)
		w.archivingMutex.Lock()
		delete(w.archivingSet, server.ID)
		w.archivingMutex.Unlock()
	}

	logger.Info("ARCHIVE-WORKER: Archive scan completed", map[string]interface{}{
		"candidates":       len(candidateServers),
		"eligible":         eligibleCount,
		"archived":         archivedCount,
		"errors":           errorCount,
		"next_scan":        time.Now().Add(w.scanInterval).Format(time.RFC3339),
	})
}

// isEligibleForArchiving checks if a server meets archiving criteria
func (w *ArchiveWorker) isEligibleForArchiving(server *models.MinecraftServer) bool {
	// 1. Server must be sleeping or stopped
	if server.Status != models.StatusSleeping && server.Status != models.StatusStopped {
		return false
	}

	// 2. Server must not already be archived
	if server.Status == models.StatusArchived || server.Status == models.StatusArchiving {
		return false
	}

	// 3. Server must have a last_stopped_at timestamp
	if server.LastStoppedAt == nil {
		logger.Debug("ARCHIVE-WORKER: Server has no last_stopped_at timestamp", map[string]interface{}{
			"server_id":   server.ID,
			"server_name": server.Name,
		})
		return false
	}

	// 4. Server must have been stopped for at least 48 hours
	timeSinceStopped := time.Since(*server.LastStoppedAt)
	if timeSinceStopped < w.archiveAfter {
		logger.Debug("ARCHIVE-WORKER: Server not stopped long enough", map[string]interface{}{
			"server_id":      server.ID,
			"server_name":    server.Name,
			"stopped_for":    timeSinceStopped.Round(time.Hour),
			"required":       w.archiveAfter,
			"time_remaining": (w.archiveAfter - timeSinceStopped).Round(time.Hour),
		})
		return false
	}

	// 5. Reserved plan servers: never auto-archive (customer pays for 24/7 availability)
	if server.Plan == models.PlanReserved {
		logger.Debug("ARCHIVE-WORKER: Reserved plan server - skipping auto-archive", map[string]interface{}{
			"server_id":   server.ID,
			"server_name": server.Name,
			"plan":        server.Plan,
		})
		return false
	}

	return true
}

// GetStats returns statistics about the archive worker
func (w *ArchiveWorker) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"running":        w.running,
		"scan_interval":  w.scanInterval.String(),
		"archive_after":  w.archiveAfter.String(),
	}
}

// SetArchiveAfter allows configuring the archive duration (for testing)
func (w *ArchiveWorker) SetArchiveAfter(duration time.Duration) {
	w.archiveAfter = duration
	logger.Info("ARCHIVE-WORKER: Archive duration updated", map[string]interface{}{
		"new_duration": duration,
	})
}

// SetScanInterval allows configuring the scan interval (for testing)
func (w *ArchiveWorker) SetScanInterval(duration time.Duration) {
	w.scanInterval = duration
	logger.Info("ARCHIVE-WORKER: Scan interval updated", map[string]interface{}{
		"new_interval": duration,
	})
}
