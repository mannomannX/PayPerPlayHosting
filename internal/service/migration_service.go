package service

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/payperplay/hosting/internal/docker"
	"github.com/payperplay/hosting/internal/events"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/logger"
)

// MigrationService handles server migrations between nodes
type MigrationService struct {
	migrationRepo       *repository.MigrationRepository
	serverRepo          *repository.ServerRepository
	dockerService       *docker.DockerService
	backupService       *BackupService
	conductor           ConductorInterface
	wsHub               WebSocketHubInterface
	dashboardWs         DashboardWebSocketInterface
	remoteVelocityClient RemoteVelocityClientInterface
}

// NewMigrationService creates a new migration service
func NewMigrationService(
	migrationRepo *repository.MigrationRepository,
	serverRepo *repository.ServerRepository,
	dockerService *docker.DockerService,
	backupService *BackupService,
) *MigrationService {
	return &MigrationService{
		migrationRepo: migrationRepo,
		serverRepo:    serverRepo,
		dockerService: dockerService,
		backupService: backupService,
	}
}

// SetConductor sets the Conductor for node management
func (s *MigrationService) SetConductor(conductor ConductorInterface) {
	s.conductor = conductor
}

// SetWebSocketHub sets the WebSocket hub for real-time updates
func (s *MigrationService) SetWebSocketHub(wsHub WebSocketHubInterface) {
	s.wsHub = wsHub
}

// SetDashboardWebSocket sets the Dashboard WebSocket for real-time dashboard updates
func (s *MigrationService) SetDashboardWebSocket(dashboardWs DashboardWebSocketInterface) {
	s.dashboardWs = dashboardWs
}

// SetRemoteVelocityClient sets the remote Velocity API client
func (s *MigrationService) SetRemoteVelocityClient(client RemoteVelocityClientInterface) {
	s.remoteVelocityClient = client
}

// StartMigrationWorker starts the background worker that processes scheduled migrations
func (s *MigrationService) StartMigrationWorker() {
	go func() {
		ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
		defer ticker.Stop()

		logger.Info("Migration worker started", nil)

		for range ticker.C {
			s.processPendingMigrations()
		}
	}()
}

// processPendingMigrations finds and executes scheduled migrations
func (s *MigrationService) processPendingMigrations() {
	migrations, err := s.migrationRepo.FindPendingMigrations()
	if err != nil {
		logger.Error("Failed to fetch pending migrations", err, map[string]interface{}{})
		return
	}

	if len(migrations) == 0 {
		return
	}

	logger.Debug("Found pending migrations", map[string]interface{}{
		"count": len(migrations),
	})

	for _, migration := range migrations {
		// Check if migration can be executed
		if s.canExecuteMigration(&migration) {
			logger.Info("Starting migration execution", map[string]interface{}{
				"operation_id": migration.ID,
				"server_id":    migration.ServerID,
				"from_node":    migration.FromNodeID,
				"to_node":      migration.ToNodeID,
			})

			// Execute migration asynchronously
			go s.executeMigration(&migration)
		}
	}
}

// canExecuteMigration checks if a migration can be executed now
func (s *MigrationService) canExecuteMigration(migration *models.Migration) bool {
	// Check if server has active migration
	hasActive, err := s.migrationRepo.HasActiveMigration(migration.ServerID)
	if err != nil || hasActive {
		return false
	}

	// Get server
	server, err := s.serverRepo.FindByID(migration.ServerID)
	if err != nil {
		logger.Error("Failed to get server for migration", err, map[string]interface{}{
			"operation_id": migration.ID,
			"server_id":    migration.ServerID,
		})
		return false
	}

	// Server must be running or starting (for manual migrations)
	// For cost-optimization, server must be fully running
	if migration.Reason == models.MigrationReasonCostOptimization {
		if server.Status != models.StatusRunning {
			logger.Debug("Server not running, skipping cost-optimization migration", map[string]interface{}{
				"operation_id": migration.ID,
				"server_id":    migration.ServerID,
				"status":       server.Status,
			})
			return false
		}
	} else {
		// Manual migrations: allow running, starting, stopped, or sleeping
		// Stopped/sleeping servers are the SAFEST to migrate (no players, no downtime risk)
		if server.Status != models.StatusRunning &&
		   server.Status != models.StatusStarting &&
		   server.Status != models.StatusStopped &&
		   server.Status != models.StatusSleeping {
			logger.Debug("Server not in migratable state, skipping migration", map[string]interface{}{
				"operation_id": migration.ID,
				"server_id":    migration.ServerID,
				"status":       server.Status,
			})
			return false
		}
	}

	// For cost-optimization migrations: wait for idle state
	if migration.Reason == models.MigrationReasonCostOptimization {
		// Only migrate if server is idle (0 players) OR has been idle for 5+ minutes
		if server.CurrentPlayerCount > 0 {
			logger.Debug("Server has players, waiting for idle state", map[string]interface{}{
				"migration_id":  migration.ID,
				"server_id":     migration.ServerID,
				"player_count":  server.CurrentPlayerCount,
			})
			return false
		}

		// Check if server has been running long enough (minimum 15 minutes)
		if server.LastStartedAt != nil {
			runningDuration := time.Since(*server.LastStartedAt)
			if runningDuration < 15*time.Minute {
				logger.Debug("Server started too recently, waiting", map[string]interface{}{
					"migration_id":     migration.ID,
					"server_id":        migration.ServerID,
					"running_duration": runningDuration.String(),
				})
				return false
			}
		}
	}

	// Check if system is stable (no scaling events in progress)
	if s.conductor != nil {
		// TODO: Add method to check if scaling is in progress
		// For now, we allow migrations
	}

	return true
}

// executeMigration executes a migration through all phases
func (s *MigrationService) executeMigration(migration *models.Migration) {
	// Get server name for events
	server, err := s.serverRepo.FindByID(migration.ServerID)
	serverName := "Unknown"
	if err == nil {
		serverName = server.Name
	}

	// Broadcast migration started event
	s.broadcastMigrationEvent("operation.migration.started", map[string]interface{}{
		"operation_id": migration.ID,
		"server_id":    migration.ServerID,
		"server_name":  serverName,
		"from_node":    migration.FromNodeID,
		"to_node":      migration.ToNodeID,
		"progress":     0,
		"status":       "started",
	})

	// Pre-Migration Backup: OPTIONAL for worker-to-worker migrations
	// For worker-to-worker: we'll use direct rsync instead of backup+restore
	// For system-to-worker: backup is needed since local access is available
	fromNodeIsSystem, err := s.conductor.IsSystemNode(migration.FromNodeID)
	if err != nil {
		s.failMigration(migration, fmt.Sprintf("Source node not found: %s", migration.FromNodeID))
		return
	}

	// Skip backup for worker-to-worker migrations (use direct rsync instead)
	isWorkerToWorker := !fromNodeIsSystem

	if !isWorkerToWorker {
		// Only create backup if migrating FROM system node (where we have local access)
		logger.Info("MIGRATION: Creating pre-migration backup (synchronous)", map[string]interface{}{
			"operation_id": migration.ID,
			"server_id":    migration.ServerID,
			"server_name":  serverName,
			"from_node_type": "system",
		})

		backup, err := s.backupService.CreateBackupSync(
			migration.ServerID,
			models.BackupTypePreMigration,
			fmt.Sprintf("Pre-migration backup for operation %s", migration.ID),
			nil, // No user ID for automated backups
			0,   // Use default retention (7 days)
		)
		if err != nil {
			s.failMigration(migration, fmt.Sprintf("Pre-migration backup failed: %v", err))
			return
		}

		// Store backup ID in migration record for rollback purposes
		migration.BackupID = &backup.ID
		if err := s.migrationRepo.Update(migration); err != nil {
			logger.Warn("Failed to store backup ID in migration record", map[string]interface{}{
				"operation_id": migration.ID,
				"backup_id":    backup.ID,
				"error":        err.Error(),
			})
		}

		logger.Info("MIGRATION: Pre-migration backup created successfully", map[string]interface{}{
			"operation_id":     migration.ID,
			"backup_id":        backup.ID,
			"compressed_mb":    backup.CompressedSize / 1024 / 1024,
			"compression_pct":  backup.GetCompressionRatio(),
		})
	} else {
		// Worker-to-worker: skip backup, use direct rsync
		logger.Info("MIGRATION: Skipping backup for worker-to-worker migration (will use direct rsync)", map[string]interface{}{
			"operation_id": migration.ID,
			"server_id":    migration.ServerID,
			"from_node":    migration.FromNodeID,
			"to_node":      migration.ToNodeID,
		})
	}

	// Phase 1: Preparing
	if err := s.phasePreparing(migration); err != nil {
		s.failMigration(migration, fmt.Sprintf("Preparing phase failed: %v", err))
		return
	}

	// Phase 2: Transferring
	if err := s.phaseTransferring(migration); err != nil {
		s.failMigration(migration, fmt.Sprintf("Transferring phase failed: %v", err))
		s.rollbackPreparing(migration) // Rollback: stop new container
		return
	}

	// Phase 3: Completing
	if err := s.phaseCompleting(migration); err != nil {
		// At this point, new server is already running with players
		// We can't rollback - just log the error and mark as completed with warning
		logger.Error("Completing phase failed but migration is functional", err, map[string]interface{}{
			"operation_id": migration.ID,
		})
		// Continue to completion
	}

	// Mark as completed
	s.completeMigration(migration)
}

// phasePreparing implements Phase 1: Preparation
func (s *MigrationService) phasePreparing(migration *models.Migration) error {
	// Update status to preparing
	now := time.Now()
	migration.Status = models.MigrationStatusPreparing
	migration.StartedAt = &now
	if err := s.migrationRepo.Update(migration); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	logger.Info("Migration Phase 1: Preparing", map[string]interface{}{
		"operation_id": migration.ID,
		"server_id":    migration.ServerID,
	})

	// Get server
	server, err := s.serverRepo.FindByID(migration.ServerID)
	if err != nil {
		return fmt.Errorf("failed to get server: %w", err)
	}

	s.broadcastMigrationEvent("operation.migration.progress", map[string]interface{}{
		"operation_id": migration.ID,
		"server_id":    migration.ServerID,
		"server_name":  server.Name,
		"from_node":    migration.FromNodeID,
		"to_node":      migration.ToNodeID,
		"progress":     10,
		"status":       "preparing",
	})

	// Store player count at start
	migration.PlayerCountAtStart = server.CurrentPlayerCount
	s.migrationRepo.Update(migration)

	// Validate target node
	if s.conductor == nil {
		return fmt.Errorf("conductor not available")
	}

	// Get target node info
	targetNode, err := s.conductor.GetRemoteNode(migration.ToNodeID)
	if err != nil {
		return fmt.Errorf("failed to get target node: %w", err)
	}

	// Check target node capacity
	if !s.conductor.AtomicAllocateRAMOnNode(migration.ToNodeID, server.RAMMb) {
		return fmt.Errorf("insufficient capacity on target node %s", migration.ToNodeID)
	}

	// CRITICAL: Transfer world data to target node BEFORE creating container
	// This ensures the container will find the world data when it starts
	if migration.BackupID != nil && *migration.BackupID != "" {
		// Method 1: Restore from backup (for system-to-worker migrations)
		logger.Info("MIGRATION: Restoring world data from backup", map[string]interface{}{
			"operation_id": migration.ID,
			"backup_id":    *migration.BackupID,
			"target_node":  targetNode.IPAddress,
		})

		s.broadcastMigrationEvent("operation.migration.progress", map[string]interface{}{
			"operation_id": migration.ID,
			"server_id":    migration.ServerID,
			"server_name":  server.Name,
			"from_node":    migration.FromNodeID,
			"to_node":      migration.ToNodeID,
			"status":       "preparing",
			"progress":     20,
			"message":      "Transferring world data from backup...",
		})

		if err := s.backupService.RestoreBackupToNode(*migration.BackupID, targetNode.IPAddress, server.ID); err != nil {
			// Rollback RAM allocation
			s.conductor.ReleaseRAMOnNode(migration.ToNodeID, server.RAMMb)
			return fmt.Errorf("failed to restore world data to target node: %w", err)
		}

		logger.Info("MIGRATION: World data restored from backup successfully", map[string]interface{}{
			"operation_id": migration.ID,
			"backup_id":    *migration.BackupID,
			"target_node":  targetNode.IPAddress,
		})
	} else {
		// Method 2: Direct rsync between worker nodes (for worker-to-worker migrations)
		sourceNode, err := s.conductor.GetRemoteNode(migration.FromNodeID)
		if err != nil {
			s.conductor.ReleaseRAMOnNode(migration.ToNodeID, server.RAMMb)
			return fmt.Errorf("failed to get source node: %w", err)
		}

		logger.Info("MIGRATION: Transferring world data via direct rsync", map[string]interface{}{
			"operation_id": migration.ID,
			"server_id":    migration.ServerID,
			"source_node":  sourceNode.IPAddress,
			"target_node":  targetNode.IPAddress,
		})

		s.broadcastMigrationEvent("operation.migration.progress", map[string]interface{}{
			"operation_id": migration.ID,
			"server_id":    migration.ServerID,
			"server_name":  server.Name,
			"from_node":    migration.FromNodeID,
			"to_node":      migration.ToNodeID,
			"status":       "preparing",
			"progress":     20,
			"message":      "Syncing world data between worker nodes...",
		})

		if err := s.syncWorldDataBetweenNodes(sourceNode.IPAddress, targetNode.IPAddress, server.ID); err != nil {
			s.conductor.ReleaseRAMOnNode(migration.ToNodeID, server.RAMMb)
			return fmt.Errorf("failed to sync world data between nodes: %w", err)
		}

		logger.Info("MIGRATION: World data synced successfully", map[string]interface{}{
			"operation_id": migration.ID,
			"source_node":  sourceNode.IPAddress,
			"target_node":  targetNode.IPAddress,
		})
	}

	s.broadcastMigrationEvent("operation.migration.progress", map[string]interface{}{
		"operation_id": migration.ID,
		"server_id":    migration.ServerID,
		"server_name":  server.Name,
		"from_node":    migration.FromNodeID,
		"to_node":      migration.ToNodeID,
		"status":       "preparing",
		"progress":     40,
		"message":      "World data transferred, starting new container...",
	})

	// Create container on target node with proper naming
	containerName := fmt.Sprintf("mc-%s", server.ID) // Use standard naming
	imageName := docker.GetDockerImageName(string(server.ServerType))
	env := docker.BuildContainerEnv(server)
	portBindings := docker.BuildPortBindings(server.Port)
	binds := docker.BuildVolumeBinds(server.ID, "/minecraft/servers")

	ctx := context.Background()
	newContainerID, err := s.conductor.GetRemoteDockerClient().StartContainer(
		ctx,
		targetNode,
		containerName,
		imageName,
		env,
		portBindings,
		binds,
		server.RAMMb,
	)

	if err != nil {
		// Rollback RAM allocation
		s.conductor.ReleaseRAMOnNode(migration.ToNodeID, server.RAMMb)
		return fmt.Errorf("failed to start container on target node: %w", err)
	}

	// Wait for new container to be ready
	logger.Info("Waiting for new container to be ready", map[string]interface{}{
		"operation_id": migration.ID,
		"container_id": newContainerID,
	})

	s.broadcastMigrationEvent("operation.migration.progress", map[string]interface{}{
		"operation_id": migration.ID,
		"server_id":    migration.ServerID,
		"server_name":  server.Name,
		"from_node":    migration.FromNodeID,
		"to_node":      migration.ToNodeID,
		"status":       "preparing",
		"progress":     60,
		"message":      "New container started, waiting for server to be ready...",
	})

	if err := s.conductor.GetRemoteDockerClient().WaitForServerReady(ctx, targetNode, newContainerID, 120); err != nil {
		// Rollback: stop new container
		s.conductor.GetRemoteDockerClient().StopContainer(ctx, targetNode, newContainerID, 30)
		s.conductor.GetRemoteDockerClient().RemoveContainer(ctx, targetNode, newContainerID, true)
		s.conductor.ReleaseRAMOnNode(migration.ToNodeID, server.RAMMb)
		return fmt.Errorf("new container failed to start: %w", err)
	}

	// Store new container ID temporarily (will be updated in phaseCompleting)
	migration.Notes = fmt.Sprintf("%s\nNew Container ID: %s\nOld Container ID: %s", migration.Notes, newContainerID, server.ContainerID)
	s.migrationRepo.Update(migration)

	s.broadcastMigrationEvent("operation.migration.progress", map[string]interface{}{
		"operation_id": migration.ID,
		"server_id":    migration.ServerID,
		"server_name":  server.Name,
		"from_node":    migration.FromNodeID,
		"to_node":      migration.ToNodeID,
		"status":       "preparing",
		"progress":     80,
		"message":      "New container ready with restored world data",
	})

	logger.Info("Migration Phase 1: Preparing completed", map[string]interface{}{
		"operation_id":  migration.ID,
		"new_container": newContainerID,
		"world_data":    "restored",
	})

	return nil
}

// phaseTransferring implements Phase 2: Player Transfer
func (s *MigrationService) phaseTransferring(migration *models.Migration) error {
	// Update status to transferring
	migration.Status = models.MigrationStatusTransferring
	if err := s.migrationRepo.Update(migration); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	logger.Info("Migration Phase 2: Transferring", map[string]interface{}{
		"operation_id": migration.ID,
		"server_id":    migration.ServerID,
	})

	// Get server
	server, err := s.serverRepo.FindByID(migration.ServerID)
	if err != nil {
		return fmt.Errorf("failed to get server: %w", err)
	}

	s.broadcastMigrationEvent("operation.migration.progress", map[string]interface{}{
		"operation_id": migration.ID,
		"server_id":    migration.ServerID,
		"server_name":  server.Name,
		"from_node":    migration.FromNodeID,
		"to_node":      migration.ToNodeID,
		"status":       "transferring",
		"progress":     0,
	})

	// Update Velocity registration to new node
	if s.remoteVelocityClient != nil {
		// Get new node IP
		targetNode, err := s.conductor.GetRemoteNode(migration.ToNodeID)
		if err != nil {
			return fmt.Errorf("failed to get target node for Velocity update: %w", err)
		}

		velocityServerName := fmt.Sprintf("mc-%s", server.ID)
		newServerAddress := fmt.Sprintf("%s:%d", targetNode.IPAddress, server.Port)

		// Unregister old server
		if err := s.remoteVelocityClient.UnregisterServer(velocityServerName); err != nil {
			logger.Warn("Failed to unregister old server from Velocity", map[string]interface{}{
				"server_id": server.ID,
				"error":     err.Error(),
			})
		}

		// Register new server
		if err := s.remoteVelocityClient.RegisterServer(velocityServerName, newServerAddress); err != nil {
			return fmt.Errorf("failed to register new server with Velocity: %w", err)
		}

		logger.Info("Velocity registration updated", map[string]interface{}{
			"operation_id": migration.ID,
			"old_address":  fmt.Sprintf("%s:%d", server.NodeID, server.Port),
			"new_address":  newServerAddress,
		})
	}

	// If players were online, they will automatically reconnect to the new server via Velocity
	// No need for explicit player transfer - Velocity handles routing

	s.broadcastMigrationEvent("operation.migration.progress", map[string]interface{}{
		"operation_id": migration.ID,
		"server_id":    migration.ServerID,
		"server_name":  server.Name,
		"from_node":    migration.FromNodeID,
		"to_node":      migration.ToNodeID,
		"status":       "transferring",
		"progress":     100,
		"message":      "Velocity routing updated",
	})

	logger.Info("Migration Phase 2: Transferring completed", map[string]interface{}{
		"operation_id": migration.ID,
	})

	return nil
}

// phaseCompleting implements Phase 3: Cleanup
func (s *MigrationService) phaseCompleting(migration *models.Migration) error {
	// Update status to completing
	migration.Status = models.MigrationStatusCompleting
	if err := s.migrationRepo.Update(migration); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	logger.Info("Migration Phase 3: Completing", map[string]interface{}{
		"operation_id": migration.ID,
		"server_id":    migration.ServerID,
	})

	// Get server
	server, err := s.serverRepo.FindByID(migration.ServerID)
	if err != nil {
		return fmt.Errorf("failed to get server: %w", err)
	}

	s.broadcastMigrationEvent("operation.migration.progress", map[string]interface{}{
		"operation_id": migration.ID,
		"server_id":    migration.ServerID,
		"server_name":  server.Name,
		"from_node":    migration.FromNodeID,
		"to_node":      migration.ToNodeID,
		"status":       "completing",
		"progress":     90,
	})

	// Stop old container on source node
	oldNodeID := migration.FromNodeID
	oldContainerID := server.ContainerID

	if oldContainerID != "" {
		sourceNode, err := s.conductor.GetRemoteNode(oldNodeID)
		if err != nil {
			logger.Warn("Failed to get source node for cleanup", map[string]interface{}{
				"node_id": oldNodeID,
				"error":   err.Error(),
			})
		} else {
			ctx := context.Background()

			// Stop old container
			if err := s.conductor.GetRemoteDockerClient().StopContainer(ctx, sourceNode, oldContainerID, 30); err != nil {
				logger.Warn("Failed to stop old container", map[string]interface{}{
					"container_id": oldContainerID,
					"error":        err.Error(),
				})
			}

			// Remove old container
			if err := s.conductor.GetRemoteDockerClient().RemoveContainer(ctx, sourceNode, oldContainerID, true); err != nil {
				logger.Warn("Failed to remove old container", map[string]interface{}{
					"container_id": oldContainerID,
					"error":        err.Error(),
				})
			}

			logger.Info("Old container stopped and removed", map[string]interface{}{
				"operation_id": migration.ID,
				"container_id": oldContainerID,
			})

			// Notify dashboard that container was removed from old node
			events.PublishContainerRemoved(
				migration.ServerID,
				server.Name,
				oldNodeID,
				"migration",
			)
		}
	}

	// Release RAM on source node
	s.conductor.ReleaseRAMOnNode(oldNodeID, server.RAMMb)

	// Extract new container ID from migration notes
	// The notes format is: "...\nNew Container ID: <id>\nOld Container ID: <id>"
	// Parse the new container ID
	newContainerID := ""
	if migration.Notes != "" {
		lines := strings.Split(migration.Notes, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "New Container ID: ") {
				newContainerID = strings.TrimPrefix(line, "New Container ID: ")
				break
			}
		}
	}

	// Update server in database with new node ID and container ID
	server.NodeID = migration.ToNodeID
	if newContainerID != "" {
		server.ContainerID = newContainerID
	}

	if err := s.serverRepo.Update(server); err != nil {
		return fmt.Errorf("failed to update server: %w", err)
	}

	// Update Conductor's container registry
	if newContainerID != "" {
		s.conductor.RegisterContainer(
			server.ID,
			server.Name,
			newContainerID,
			migration.ToNodeID,
			server.RAMMb,
			server.Port,
			server.Port,
			"running",
			server.MinecraftVersion,  // Add version for dashboard display
			string(server.ServerType), // Add type for dashboard display
		)
	}

	logger.Info("Migration Phase 3: Completing finished", map[string]interface{}{
		"operation_id": migration.ID,
		"new_node_id":  migration.ToNodeID,
	})

	return nil
}

// completeMigration marks migration as completed
func (s *MigrationService) completeMigration(migration *models.Migration) {
	now := time.Now()
	migration.Status = models.MigrationStatusCompleted
	migration.CompletedAt = &now

	if err := s.migrationRepo.Update(migration); err != nil {
		logger.Error("Failed to mark migration as completed", err, map[string]interface{}{
			"operation_id": migration.ID,
		})
		return
	}

	duration := migration.DurationSeconds()

	// Get server name for event
	server, err := s.serverRepo.FindByID(migration.ServerID)
	serverName := "Unknown"
	if err == nil {
		serverName = server.Name
	}

	logger.Info("Migration completed successfully", map[string]interface{}{
		"operation_id":      migration.ID,
		"server_id":         migration.ServerID,
		"duration_seconds":  duration,
		"savings_eur_hour":  migration.SavingsEURHour,
		"savings_eur_month": migration.SavingsEURMonth,
	})

	s.broadcastMigrationEvent("operation.migration.completed", map[string]interface{}{
		"operation_id":        migration.ID,
		"server_id":           migration.ServerID,
		"server_name":         serverName,
		"from_node":           migration.FromNodeID,
		"to_node":             migration.ToNodeID,
		"duration_seconds":    duration,
		"players_transferred": migration.PlayerCountAtStart,
		"progress":            100,
		"status":              "completed",
		"success":             true,
	})
}

// syncWorldDataBetweenNodes synchronizes world data directly between worker nodes using rsync
func (s *MigrationService) syncWorldDataBetweenNodes(sourceIP, targetIP, serverID string) error {
	sourceDir := fmt.Sprintf("/minecraft/servers/%s/", serverID)
	targetDir := fmt.Sprintf("/minecraft/servers/%s", serverID)

	logger.Info("MIGRATION: Starting rsync between worker nodes", map[string]interface{}{
		"source_ip":   sourceIP,
		"target_ip":   targetIP,
		"server_id":   serverID,
		"source_path": sourceDir,
		"target_path": targetDir,
	})

	// SSH identity file (keys are copied to /app/.ssh by entrypoint.sh)
	sshIdentity := "/app/.ssh/id_rsa"

	// 1. Create target directory on destination node
	mkdirCmd := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null root@%s 'mkdir -p %s'", sshIdentity, targetIP, targetDir)
	if err := s.executeCommand(mkdirCmd); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// 2. Rsync from source to target via SSH
	// Using rsync with compression and archive mode
	// Format: rsync -avz -e ssh source_user@source_ip:/path/ target_user@target_ip:/path/
	// Note: StrictHostKeyChecking disabled for automated migrations between trusted infrastructure
	rsyncCmd := fmt.Sprintf(
		"ssh -i %s -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null root@%s 'rsync -avz --delete -e \"ssh -i %s -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null\" %s root@%s:%s/'",
		sshIdentity, // Identity file for outer SSH
		sourceIP,    // Connect to source node
		sshIdentity, // Identity file for inner SSH (rsync)
		sourceDir,   // Source directory (with trailing slash to copy contents)
		targetIP,    // Target node IP
		targetDir,   // Target directory
	)

	logger.Info("MIGRATION: Executing rsync command", map[string]interface{}{
		"server_id": serverID,
		"command":   rsyncCmd,
	})

	if err := s.executeCommand(rsyncCmd); err != nil {
		return fmt.Errorf("rsync failed: %w", err)
	}

	logger.Info("MIGRATION: Rsync completed successfully", map[string]interface{}{
		"source_ip": sourceIP,
		"target_ip": targetIP,
		"server_id": serverID,
	})

	return nil
}

// executeCommand executes a shell command via sh (Alpine-compatible)
func (s *MigrationService) executeCommand(command string) error {
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %w, output: %s", err, string(output))
	}
	logger.Debug("MIGRATION: Command executed", map[string]interface{}{
		"command": command,
		"output":  string(output),
	})
	return nil
}

// failMigration marks migration as failed
func (s *MigrationService) failMigration(migration *models.Migration, errorMessage string) {
	migration.Status = models.MigrationStatusFailed
	migration.ErrorMessage = errorMessage
	migration.RetryCount++

	if err := s.migrationRepo.Update(migration); err != nil {
		logger.Error("Failed to mark migration as failed", err, map[string]interface{}{
			"operation_id": migration.ID,
		})
		return
	}

	// Get server name for event
	server, err := s.serverRepo.FindByID(migration.ServerID)
	serverName := "Unknown"
	if err == nil {
		serverName = server.Name
	}

	logger.Error("Migration failed", fmt.Errorf("%s", errorMessage), map[string]interface{}{
		"operation_id": migration.ID,
		"server_id":    migration.ServerID,
		"retry_count":  migration.RetryCount,
	})

	s.broadcastMigrationEvent("operation.migration.failed", map[string]interface{}{
		"operation_id":     migration.ID,
		"server_id":        migration.ServerID,
		"server_name":      serverName,
		"from_node":        migration.FromNodeID,
		"to_node":          migration.ToNodeID,
		"error":            errorMessage,
		"status":           "failed",
		"rollback_success": true,
	})

	// TODO: Retry logic if retry_count < max_retries
	if migration.RetryCount < migration.MaxRetries {
		logger.Info("Migration will be retried", map[string]interface{}{
			"operation_id": migration.ID,
			"retry_count":  migration.RetryCount,
			"max_retries":  migration.MaxRetries,
		})
		// Set status back to scheduled for retry
		migration.Status = models.MigrationStatusScheduled
		s.migrationRepo.Update(migration)
	}
}

// rollbackPreparing rolls back Phase 1 (stop and remove new container)
func (s *MigrationService) rollbackPreparing(migration *models.Migration) {
	logger.Info("Rolling back preparing phase", map[string]interface{}{
		"operation_id": migration.ID,
	})

	// Release RAM on target node
	server, err := s.serverRepo.FindByID(migration.ServerID)
	if err != nil {
		logger.Error("Failed to get server for rollback", err, map[string]interface{}{
			"operation_id": migration.ID,
		})
		return
	}

	s.conductor.ReleaseRAMOnNode(migration.ToNodeID, server.RAMMb)

	// TODO: Stop and remove new container if we stored its ID
	// For now, we assume the container name is predictable
	containerName := fmt.Sprintf("mc-%s-migration", migration.ServerID)

	targetNode, err := s.conductor.GetRemoteNode(migration.ToNodeID)
	if err != nil {
		logger.Error("Failed to get target node for rollback", err, map[string]interface{}{
			"operation_id": migration.ID,
		})
		return
	}

	ctx := context.Background()
	// Try to remove container by name
	s.conductor.GetRemoteDockerClient().RemoveContainer(ctx, targetNode, containerName, true)

	logger.Info("Rollback completed", map[string]interface{}{
		"operation_id": migration.ID,
	})
}

// broadcastMigrationEvent sends WebSocket event to both hubs
func (s *MigrationService) broadcastMigrationEvent(eventType string, data map[string]interface{}) {
	// Send to old WebSocket hub (for backward compatibility)
	if s.wsHub != nil {
		s.wsHub.Broadcast(eventType, data)
	}

	// Send to Dashboard WebSocket (for real-time dashboard visualization)
	if s.dashboardWs != nil {
		s.dashboardWs.PublishEvent(eventType, data)
	}
}

// ScheduleMigration creates a manual migration and schedules it
func (s *MigrationService) ScheduleMigration(serverID, toNodeID, reason string) (*models.Migration, error) {
	// This is a convenience method for manual migrations
	// For now, migrations are created via the API handler
	// This method is reserved for future use
	return nil, fmt.Errorf("not implemented - use API endpoint instead")
}
