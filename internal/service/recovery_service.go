package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/payperplay/hosting/internal/docker"
	"github.com/payperplay/hosting/internal/events"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/config"
	"github.com/payperplay/hosting/pkg/logger"
)

// RecoveryService handles automatic crash detection and recovery
type RecoveryService struct {
	serverRepo    *repository.ServerRepository
	dockerService *docker.DockerService
	cfg           *config.Config
	wsHub         WebSocketHubInterface
	recoveryQueue chan *models.MinecraftServer
	stopChan      chan struct{}
}

// NewRecoveryService creates a new recovery service
func NewRecoveryService(
	serverRepo *repository.ServerRepository,
	dockerService *docker.DockerService,
	cfg *config.Config,
) *RecoveryService {
	return &RecoveryService{
		serverRepo:    serverRepo,
		dockerService: dockerService,
		cfg:           cfg,
		recoveryQueue: make(chan *models.MinecraftServer, 10),
		stopChan:      make(chan struct{}),
	}
}

// SetWebSocketHub sets the WebSocket hub for real-time updates
func (s *RecoveryService) SetWebSocketHub(wsHub WebSocketHubInterface) {
	s.wsHub = wsHub
}

// Start starts the recovery service
func (s *RecoveryService) Start() {
	logger.Info("Starting recovery service", nil)
	go s.processRecoveryQueue()
}

// Stop stops the recovery service
func (s *RecoveryService) Stop() {
	logger.Info("Stopping recovery service", nil)
	close(s.stopChan)
}

// RecoverServer attempts to recover a crashed server
func (s *RecoveryService) RecoverServer(server *models.MinecraftServer) {
	select {
	case s.recoveryQueue <- server:
		logger.Info("Server queued for recovery", map[string]interface{}{
			"server_id": server.ID,
		})
	default:
		logger.Warn("Recovery queue full, skipping", map[string]interface{}{
			"server_id": server.ID,
		})
	}
}

// processRecoveryQueue processes servers in the recovery queue
func (s *RecoveryService) processRecoveryQueue() {
	for {
		select {
		case <-s.stopChan:
			return
		case server := <-s.recoveryQueue:
			s.attemptRecovery(server)
		}
	}
}

// attemptRecovery attempts to recover a crashed server
func (s *RecoveryService) attemptRecovery(server *models.MinecraftServer) {
	logger.Info("Starting recovery attempt", map[string]interface{}{
		"server_id": server.ID,
		"status":    server.Status,
	})

	// Get container logs to diagnose the issue
	logs, err := s.dockerService.GetContainerLogs(server.ContainerID, "200")
	if err != nil {
		logger.Warn("Failed to get container logs for diagnosis", map[string]interface{}{
			"server_id": server.ID,
			"error":     err.Error(),
		})
	}

	// Analyze logs to determine crash cause
	crashCause := s.analyzeCrashLogs(logs)
	logger.Info("Crash diagnosis", map[string]interface{}{
		"server_id": server.ID,
		"cause":     crashCause,
	})

	// Apply appropriate recovery strategy
	var recovered bool
	switch crashCause {
	case "system_oom":
		// CRITICAL: System has insufficient memory - restart will NOT help
		recovered = s.recoverFromSystemOOM(server)
	case "version_mismatch":
		recovered = s.recoverFromVersionMismatch(server)
	case "config_corruption":
		recovered = s.recoverFromConfigCorruption(server)
	case "oom":
		recovered = s.recoverFromOOM(server)
	case "port_conflict":
		recovered = s.recoverFromPortConflict(server)
	default:
		recovered = s.recoverGeneric(server)
	}

	if recovered {
		logger.Info("Server recovered successfully", map[string]interface{}{
			"server_id": server.ID,
		})

		// Publish event
		events.PublishServerRestarted(server.ID, fmt.Sprintf("Auto-recovery from %s", crashCause))

		// Broadcast recovery success via WebSocket
		if s.wsHub != nil {
			s.wsHub.Broadcast("server_recovered", map[string]interface{}{
				"server_id": server.ID,
				"status":    string(models.StatusRunning),
				"cause":     crashCause,
			})
		}
	} else {
		logger.Error("Failed to recover server", fmt.Errorf("recovery failed"), map[string]interface{}{
			"server_id": server.ID,
			"cause":     crashCause,
		})
		server.Status = models.StatusError
		s.serverRepo.Update(server)

		// Broadcast recovery failure via WebSocket
		if s.wsHub != nil {
			s.wsHub.Broadcast("server_status", map[string]interface{}{
				"server_id": server.ID,
				"status":    string(models.StatusError),
				"error":     "Failed to recover from crash",
			})
		}
	}
}

// analyzeCrashLogs analyzes container logs to determine crash cause
func (s *RecoveryService) analyzeCrashLogs(logs string) string {
	logsLower := strings.ToLower(logs)

	// Check for version mismatch (chunk version or config version)
	if strings.Contains(logsLower, "chunk saved with newer version") ||
		strings.Contains(logsLower, "loading a newer configuration than is supported") {
		return "version_mismatch"
	}

	// Check for config corruption (NumberFormatException, YAML errors, etc.)
	if strings.Contains(logsLower, "numberformatexception") ||
		strings.Contains(logsLower, "for input string: \"default\"") ||
		strings.Contains(logsLower, "serializationexception") ||
		strings.Contains(logsLower, "yaml") {
		return "config_corruption"
	}

	// Check for System OOM (insufficient memory - cannot allocate)
	// This is FATAL and restart won't help
	if strings.Contains(logsLower, "insufficient memory") ||
		strings.Contains(logsLower, "cannot allocate memory") ||
		strings.Contains(logsLower, "failed to map") ||
		strings.Contains(logsLower, "error='not enough space'") {
		return "system_oom"
	}

	// Check for Java OOM (heap exhausted - restart might help)
	if strings.Contains(logsLower, "out of memory") ||
		strings.Contains(logsLower, "java.lang.outofmemoryerror") {
		return "oom"
	}

	// Check for port conflicts
	if strings.Contains(logsLower, "address already in use") ||
		strings.Contains(logsLower, "bind") {
		return "port_conflict"
	}

	return "unknown"
}

// recoverFromConfigCorruption recovers a server from config corruption
func (s *RecoveryService) recoverFromConfigCorruption(server *models.MinecraftServer) bool {
	logger.Info("Attempting config corruption recovery", map[string]interface{}{
		"server_id": server.ID,
	})

	// Get server directory path
	serverDir := filepath.Join(s.cfg.ServersBasePath, server.ID)
	configFile := filepath.Join(serverDir, "config", "paper-world-defaults.yml")

	// Check if config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		logger.Warn("Config file doesn't exist, will be generated on restart", map[string]interface{}{
			"server_id":   server.ID,
			"config_file": configFile,
		})
		return s.restartContainer(server)
	}

	// Run the fix script
	logger.Info("Running config repair script", map[string]interface{}{
		"server_id":   server.ID,
		"config_file": configFile,
	})

	if err := s.fixPaperConfig(serverDir); err != nil {
		logger.Error("Config repair failed", err, map[string]interface{}{
			"server_id": server.ID,
		})
		return false
	}

	logger.Info("Config repaired successfully", map[string]interface{}{
		"server_id": server.ID,
	})

	// Restart container
	return s.restartContainer(server)
}

// fixPaperConfig runs the config fix script
func (s *RecoveryService) fixPaperConfig(serverDir string) error {
	configFile := filepath.Join(serverDir, "config", "paper-world-defaults.yml")

	// Check if file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil // File doesn't exist yet, nothing to fix
	}

	// Read file
	content, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	originalContent := string(content)
	fixedContent := originalContent

	// Step 1: Fix max-leash-distance (ONLY field that expects float)
	if strings.Contains(fixedContent, "max-leash-distance: default") {
		logger.Info("Fixing max-leash-distance", map[string]interface{}{
			"file": configFile,
		})
		fixedContent = strings.ReplaceAll(fixedContent, "max-leash-distance: default", "max-leash-distance: 10.0")
	}

	// Step 2: Fix ALL other fields that were incorrectly set to 10.0
	// Paper expects these to be integers, booleans, or "default" - NOT floats
	// This fixes the bug where our old script replaced ALL "default" with "10.0"
	invalidFloats := []string{
		"auto-save-interval: 10.0",
		"delay-chunk-unloads-by: 10.0",
		"entity-per-chunk-save-limit: 10.0",
		"fixed-chunk-inhabited-time: 10.0",
		"max-auto-save-chunks-per-tick: 10.0",
		"prevent-moving-into-unloaded-chunks: 10.0",
		"non-player-arrow-despawn-rate: 10.0",
		"creative-arrow-despawn-rate: 10.0",
		// Boolean fields that got corrupted
		"loot-tables: 10.0",
		"villager-trade: 10.0",
	}

	for _, invalidField := range invalidFloats {
		if strings.Contains(fixedContent, invalidField) {
			correctField := strings.Replace(invalidField, ": 10.0", ": default", 1)
			logger.Info("Fixing field with invalid 10.0 value", map[string]interface{}{
				"field": invalidField,
				"fixed": correctField,
				"file":  configFile,
			})
			fixedContent = strings.ReplaceAll(fixedContent, invalidField, correctField)
		}
	}

	// Only write if changes were made
	if fixedContent != originalContent {
		// Create backup
		backupFile := fmt.Sprintf("%s.backup.%d", configFile, time.Now().Unix())
		if err := os.WriteFile(backupFile, []byte(originalContent), 0644); err != nil {
			logger.Warn("Failed to create backup", map[string]interface{}{
				"file":  configFile,
				"error": err.Error(),
			})
		} else {
			logger.Info("Created config backup", map[string]interface{}{
				"backup": backupFile,
			})
		}

		// Write fixed config
		if err := os.WriteFile(configFile, []byte(fixedContent), 0644); err != nil {
			return fmt.Errorf("failed to write fixed config: %w", err)
		}

		logger.Info("Config file repaired", map[string]interface{}{
			"file": configFile,
		})
	}

	return nil
}

// recoverFromVersionMismatch handles version mismatch errors
func (s *RecoveryService) recoverFromVersionMismatch(server *models.MinecraftServer) bool {
	logger.Error("Server crashed due to version mismatch", fmt.Errorf("world was created with newer Minecraft version"), map[string]interface{}{
		"server_id":         server.ID,
		"current_version":   server.MinecraftVersion,
	})

	// Version mismatch cannot be automatically recovered
	// The user must update the server version via Configuration tab
	// We set status to error with a helpful message

	server.Status = models.StatusError
	s.serverRepo.Update(server)

	// Broadcast specific error message via WebSocket
	if s.wsHub != nil {
		s.wsHub.Broadcast("server_status", map[string]interface{}{
			"server_id": server.ID,
			"status":    string(models.StatusError),
			"error":     fmt.Sprintf("Version mismatch: World was created with newer Minecraft version than %s. Please update server version via Configuration tab.", server.MinecraftVersion),
		})
	}

	return false // Cannot auto-recover
}

// recoverFromOOM recovers a server from OOM
// recoverFromSystemOOM handles system-level out of memory errors
// These are FATAL - the host system has insufficient RAM and restart will NOT help
func (s *RecoveryService) recoverFromSystemOOM(server *models.MinecraftServer) bool {
	logger.Error("CRITICAL: System has insufficient memory to run server", fmt.Errorf("system oom"), map[string]interface{}{
		"server_id":      server.ID,
		"requested_ram":  server.RAMMb,
		"error_type":     "SYSTEM_OOM",
		"recovery_action": "NONE - Host system needs more RAM or fewer servers",
	})

	// DO NOT restart - this will cause an infinite loop
	// Set server to error state permanently
	server.Status = models.StatusError
	s.serverRepo.Update(server)

	// Publish critical event
	events.PublishServerStopped(server.ID, "CRITICAL: System has insufficient memory. Cannot restart.")

	// Alert via WebSocket
	if s.wsHub != nil {
		s.wsHub.Broadcast("server_critical_error", map[string]interface{}{
			"server_id": server.ID,
			"error":     "System has insufficient memory to run this server",
			"action":    "Server stopped to prevent system instability",
		})
	}

	return false // Recovery FAILED - server stays in error state
}

func (s *RecoveryService) recoverFromOOM(server *models.MinecraftServer) bool {
	logger.Warn("Server crashed due to Java OOM - consider increasing RAM", map[string]interface{}{
		"server_id":   server.ID,
		"current_ram": server.RAMMb,
	})

	// For now, just try restarting
	// In the future, could automatically increase RAM or notify admins
	return s.restartContainer(server)
}

// recoverFromPortConflict recovers a server from port conflict
func (s *RecoveryService) recoverFromPortConflict(server *models.MinecraftServer) bool {
	logger.Warn("Server crashed due to port conflict", map[string]interface{}{
		"server_id": server.ID,
		"port":      server.Port,
	})

	// Try to clean up any conflicting containers
	s.cleanupOrphanedContainers(server)

	return s.restartContainer(server)
}

// recoverGeneric performs generic recovery
func (s *RecoveryService) recoverGeneric(server *models.MinecraftServer) bool {
	logger.Info("Attempting generic recovery", map[string]interface{}{
		"server_id": server.ID,
	})

	return s.restartContainer(server)
}

// restartContainer restarts a container
func (s *RecoveryService) restartContainer(server *models.MinecraftServer) bool {
	logger.Info("Restarting container", map[string]interface{}{
		"server_id":    server.ID,
		"container_id": server.ContainerID,
	})

	ctx := context.Background()

	// Get Docker client
	cli := s.dockerService.GetClient()

	// Stop container if running
	if server.ContainerID != "" {
		timeout := 30
		err := cli.ContainerStop(ctx, server.ContainerID, container.StopOptions{Timeout: &timeout})
		if err != nil {
			logger.Warn("Failed to stop container during recovery", map[string]interface{}{
				"server_id": server.ID,
				"error":     err.Error(),
			})
		}

		// Remove container
		err = cli.ContainerRemove(ctx, server.ContainerID, container.RemoveOptions{Force: true})
		if err != nil {
			logger.Warn("Failed to remove container during recovery", map[string]interface{}{
				"server_id": server.ID,
				"error":     err.Error(),
			})
		}
	}

	// Create new container
	containerID, err := s.dockerService.CreateContainer(
		server.ID,
		string(server.ServerType),
		server.MinecraftVersion,
		server.RAMMb,
		server.Port,
		// Phase 1 Parameters
		server.MaxPlayers,
		server.Gamemode,
		server.Difficulty,
		server.PVP,
		server.EnableCommandBlock,
		server.LevelSeed,
		// Phase 2 Parameters - Performance
		server.ViewDistance,
		server.SimulationDistance,
		// Phase 2 Parameters - World Generation
		server.AllowNether,
		server.AllowEnd,
		server.GenerateStructures,
		server.WorldType,
		server.BonusChest,
		server.MaxWorldSize,
		// Phase 2 Parameters - Spawn Settings
		server.SpawnProtection,
		server.SpawnAnimals,
		server.SpawnMonsters,
		server.SpawnNPCs,
		// Phase 2 Parameters - Network & Performance
		server.MaxTickTime,
		server.NetworkCompressionThreshold,
		// Phase 4 Parameters - Server Description
		server.MOTD,
	)
	if err != nil {
		logger.Error("Failed to create container during recovery", err, map[string]interface{}{
			"server_id": server.ID,
		})
		return false
	}

	server.ContainerID = containerID
	server.Status = models.StatusStopped
	s.serverRepo.Update(server)

	// Start container (only for local nodes - remote containers are handled by RemoteDockerClient)
	if s.isLocalNode(server.NodeID) {
		if err := s.dockerService.StartContainer(containerID); err != nil {
		logger.Error("Failed to start container during recovery", err, map[string]interface{}{
			"server_id": server.ID,
		})
		server.Status = models.StatusError
		s.serverRepo.Update(server)
		return false
	}

		// Wait for server to be ready (with shorter timeout for recovery)
		err = s.dockerService.WaitForServerReady(containerID, 90)
		if err != nil {
			logger.Warn("Server may not be fully ready after recovery", map[string]interface{}{
				"server_id": server.ID,
				"error":     err.Error(),
			})
			// Don't fail recovery - server might still come up
		}
	} else {
		// SAFEGUARD: Recovery not yet supported for remote nodes
		logger.Warn("Container recovery not yet supported for remote nodes", map[string]interface{}{
			"server_id": server.ID,
			"node_id":   server.NodeID,
		})
		server.Status = models.StatusError
		s.serverRepo.Update(server)
		return false
	}

	server.Status = models.StatusRunning
	s.serverRepo.Update(server)

	return true
}

// cleanupOrphanedContainers removes orphaned containers that might be blocking the port
func (s *RecoveryService) cleanupOrphanedContainers(server *models.MinecraftServer) {
	containerName := fmt.Sprintf("mc-%s", server.ID)
	err := s.dockerService.RemoveContainerByName(containerName)
	if err != nil {
		logger.Warn("Failed to cleanup orphaned container", map[string]interface{}{
			"server_id": server.ID,
			"error":     err.Error(),
		})
	}
}

// CheckAndRecoverCrashedServers checks all servers and recovers crashed ones
// This is called periodically by the monitoring service
func (s *RecoveryService) CheckAndRecoverCrashedServers() error {
	ctx := context.Background()
	cli := s.dockerService.GetClient()

	// Get all servers
	servers, err := s.serverRepo.FindAll()
	if err != nil {
		return fmt.Errorf("failed to get servers: %w", err)
	}

	for _, server := range servers {
		// Skip servers that are supposed to be stopped
		if server.Status == models.StatusStopped || server.Status == models.StatusError {
			continue
		}

		// Check if container exists and its state
		if server.ContainerID == "" {
			continue
		}

		inspect, err := cli.ContainerInspect(ctx, server.ContainerID)
		if err != nil {
			logger.Warn("Failed to inspect container", map[string]interface{}{
				"server_id":    server.ID,
				"container_id": server.ContainerID,
				"error":        err.Error(),
			})
			continue
		}

		// Check if container has exited unexpectedly
		if inspect.State.Status == "exited" && server.Status == models.StatusRunning {
			logger.Warn("Detected crashed server", map[string]interface{}{
				"server_id":   server.ID,
				"exit_code":   inspect.State.ExitCode,
				"exit_reason": inspect.State.Error,
			})

			// Publish event
			errorMessage := inspect.State.Error
			if errorMessage == "" {
				errorMessage = "Container exited unexpectedly"
			}
			events.PublishServerCrashed(server.ID, inspect.State.ExitCode, errorMessage)

			// Broadcast crash detection via WebSocket
			if s.wsHub != nil {
				s.wsHub.Broadcast("server_crashed", map[string]interface{}{
					"server_id":   server.ID,
					"status":      "crashed",
					"exit_code":   inspect.State.ExitCode,
					"exit_reason": inspect.State.Error,
				})
			}

			// Queue for recovery
			s.RecoverServer(&server)
		}
	}

	return nil
}

// isLocalNode checks if a node ID represents the local Docker daemon
// Returns true if nodeID is "local-node" or empty (backward compatibility)
func (s *RecoveryService) isLocalNode(nodeID string) bool {
	return nodeID == "" || nodeID == "local-node"
}
