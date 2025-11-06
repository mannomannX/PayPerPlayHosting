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

	// Check for config corruption (NumberFormatException, YAML errors, etc.)
	if strings.Contains(logsLower, "numberformatexception") ||
		strings.Contains(logsLower, "for input string: \"default\"") ||
		strings.Contains(logsLower, "serializationexception") ||
		strings.Contains(logsLower, "yaml") {
		return "config_corruption"
	}

	// Check for OOM (Out of Memory)
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

	// Fix max-leash-distance
	if strings.Contains(fixedContent, "max-leash-distance: default") {
		logger.Info("Fixing max-leash-distance", map[string]interface{}{
			"file": configFile,
		})
		fixedContent = strings.ReplaceAll(fixedContent, "max-leash-distance: default", "max-leash-distance: 10.0")
	}

	// Fix other "default" values in numeric fields
	lines := strings.Split(fixedContent, "\n")
	for i, line := range lines {
		// Match lines ending with ": default"
		if strings.HasSuffix(strings.TrimSpace(line), ": default") {
			logger.Info("Fixing numeric field with 'default'", map[string]interface{}{
				"line": line,
				"file": configFile,
			})
			lines[i] = strings.Replace(line, ": default", ": 10.0", 1)
		}
	}
	fixedContent = strings.Join(lines, "\n")

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

// recoverFromOOM recovers a server from OOM
func (s *RecoveryService) recoverFromOOM(server *models.MinecraftServer) bool {
	logger.Warn("Server crashed due to OOM - consider increasing RAM", map[string]interface{}{
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

	// Start container
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
