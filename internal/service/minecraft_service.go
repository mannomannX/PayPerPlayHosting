package service

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/payperplay/hosting/internal/docker"
	"github.com/payperplay/hosting/internal/events"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/config"
)

type MinecraftService struct {
	repo            *repository.ServerRepository
	dockerService   *docker.DockerService
	cfg             *config.Config
	velocityService VelocityServiceInterface // Interface to avoid circular dependency
	wsHub           WebSocketHubInterface    // Interface for WebSocket broadcasting
	conductor       ConductorInterface        // Interface for capacity management
}

// WebSocketHubInterface defines the methods needed from WebSocket Hub
type WebSocketHubInterface interface {
	Broadcast(messageType string, data interface{})
}

// VelocityServiceInterface defines the methods needed from VelocityService
type VelocityServiceInterface interface {
	RegisterServer(server *models.MinecraftServer) error
	UnregisterServer(serverID string) error
	IsRunning() bool
}

// ConductorInterface defines the methods needed from Conductor for capacity management
type ConductorInterface interface {
	// CheckCapacity checks if there's enough capacity to start a server
	CheckCapacity(requiredRAMMB int) (bool, int) // returns (hasCapacity, availableRAMMB)

	// EnqueueServer adds a server to the start queue if capacity is insufficient
	EnqueueServer(serverID, serverName string, requiredRAMMB int, userID string)

	// IsServerQueued checks if a server is in the start queue
	IsServerQueued(serverID string) bool

	// RemoveFromQueue removes a server from the start queue
	RemoveFromQueue(serverID string)
}

func NewMinecraftService(
	repo *repository.ServerRepository,
	dockerService *docker.DockerService,
	cfg *config.Config,
) *MinecraftService {
	return &MinecraftService{
		repo:          repo,
		dockerService: dockerService,
		cfg:           cfg,
	}
}

// SetVelocityService sets the velocity service (called after initialization to avoid circular dependency)
func (s *MinecraftService) SetVelocityService(velocityService VelocityServiceInterface) {
	s.velocityService = velocityService
}

// SetWebSocketHub sets the WebSocket hub for real-time updates
func (s *MinecraftService) SetWebSocketHub(wsHub WebSocketHubInterface) {
	s.wsHub = wsHub
}

// SetConductor sets the Conductor for capacity management
func (s *MinecraftService) SetConductor(conductor ConductorInterface) {
	s.conductor = conductor
}

// CreateServer creates a new Minecraft server
func (s *MinecraftService) CreateServer(
	name string,
	serverType models.ServerType,
	minecraftVersion string,
	ramMB int,
	ownerID string,
) (*models.MinecraftServer, error) {
	// Generate server ID
	serverID := uuid.New().String()[:8]

	// Find available port
	usedPorts, err := s.repo.GetUsedPorts()
	if err != nil {
		return nil, fmt.Errorf("failed to get used ports: %w", err)
	}

	port, err := s.dockerService.FindAvailablePort(usedPorts)
	if err != nil {
		return nil, err
	}

	// Create server record
	server := &models.MinecraftServer{
		ID:                   serverID,
		Name:                 name,
		OwnerID:              ownerID,
		ServerType:           serverType,
		MinecraftVersion:     minecraftVersion,
		RAMMb:                ramMB,
		Port:                 port,
		Status:               models.StatusStopped,
		IdleTimeoutSeconds:   s.cfg.DefaultIdleTimeout,
		AutoShutdownEnabled:  true,
		MaxPlayers:           20,
	}

	// Try to create server record
	err = s.repo.Create(server)
	if err != nil {
		// Check if it's a port conflict (duplicate key constraint)
		if strings.Contains(err.Error(), "duplicate key") && strings.Contains(err.Error(), "port") {
			log.Printf("Port conflict detected for port %d, attempting automatic cleanup...", port)

			// Find the server blocking this port
			blockingServer, findErr := s.repo.FindByPort(port)
			if findErr == nil && blockingServer != nil {
				log.Printf("Found blocking server: %s (ContainerID: %s)", blockingServer.ID, blockingServer.ContainerID)

				// Check if it's a ghost server (no container) or has missing container
				shouldDelete := false
				if blockingServer.ContainerID == "" {
					log.Printf("Blocking server %s is a ghost server (no container ID)", blockingServer.ID)
					shouldDelete = true
				} else {
					// Check if container actually exists
					_, statusErr := s.dockerService.GetContainerStatus(blockingServer.ContainerID)
					if statusErr != nil {
						log.Printf("Blocking server %s has missing container", blockingServer.ID)
						shouldDelete = true
					}
				}

				if shouldDelete {
					log.Printf("Auto-deleting orphaned server %s to free port %d", blockingServer.ID, port)
					if delErr := s.DeleteServer(blockingServer.ID); delErr != nil {
						log.Printf("Failed to auto-delete blocking server: %v", delErr)
					} else {
						log.Printf("Successfully removed blocking server, retrying creation...")
						// Retry creation once
						if retryErr := s.repo.Create(server); retryErr != nil {
							return nil, fmt.Errorf("failed to create server after cleanup: %w", retryErr)
						}
						// Success after cleanup!
						log.Printf("Server created successfully after automatic cleanup")
						err = nil // Clear the error
					}
				} else {
					log.Printf("Blocking server %s has a valid container, cannot auto-delete", blockingServer.ID)
				}
			}
		}

		// If we still have an error, return it
		if err != nil {
			return nil, fmt.Errorf("failed to create server record: %w", err)
		}
	}

	// Register with Velocity if available
	if s.velocityService != nil && s.velocityService.IsRunning() {
		if err := s.velocityService.RegisterServer(server); err != nil {
			log.Printf("Warning: failed to register server with Velocity: %v", err)
			// Don't fail the entire operation if Velocity registration fails
		} else {
			log.Printf("Server registered with Velocity as %s", server.VelocityServerName)
		}
	}

	// Publish event
	events.PublishServerCreated(server.ID, server.OwnerID, string(server.ServerType))

	log.Printf("Created server %s (%s) on port %d", serverID, name, port)
	return server, nil
}

// StartServer starts a Minecraft server
func (s *MinecraftService) StartServer(serverID string) error {
	server, err := s.repo.FindByID(serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	if server.Status == models.StatusRunning {
		return fmt.Errorf("server already running")
	}

	// PRE-START RESOURCE GUARD: Check if we have capacity to start this server
	if s.conductor != nil {
		hasCapacity, availableRAM := s.conductor.CheckCapacity(server.RAMMb)

		if !hasCapacity {
			// Check if already queued
			if s.conductor.IsServerQueued(server.ID) {
				return fmt.Errorf("server is already queued for start (position: waiting for %d MB, available: %d MB)", server.RAMMb, availableRAM)
			}

			// Not enough capacity - add to queue instead of starting
			s.conductor.EnqueueServer(server.ID, server.Name, server.RAMMb, server.OwnerID)

			log.Printf("RESOURCE_GUARD: Insufficient capacity for server %s (%d MB required, %d MB available) - Added to queue",
				server.ID, server.RAMMb, availableRAM)

			return fmt.Errorf("insufficient capacity to start server (%d MB required, %d MB available) - server queued for start, will auto-start when capacity available", server.RAMMb, availableRAM)
		}

		// Remove from queue if it was queued (in case of manual retry)
		s.conductor.RemoveFromQueue(server.ID)

		log.Printf("RESOURCE_GUARD: Capacity check passed for server %s (%d MB required, %d MB available)",
			server.ID, server.RAMMb, availableRAM)
	}

	// Wake from sleep if necessary
	if server.LifecyclePhase == models.PhaseSleep || server.Status == models.StatusSleeping {
		log.Printf("Waking server %s from sleep phase", serverID)
		server.LifecyclePhase = models.PhaseActive
		server.Status = models.StatusStopped
		err := s.repo.Update(server)
		if err != nil {
			return fmt.Errorf("failed to wake from sleep: %w", err)
		}
	}

	// CRITICAL: Remove any existing container with the same name before creating a new one
	// This prevents "port already allocated" errors from zombie containers
	containerName := fmt.Sprintf("mc-%s", server.ID)
	log.Printf("Checking for existing container %s before start", containerName)
	if err := s.dockerService.RemoveContainerByName(containerName); err != nil {
		log.Printf("Warning: failed to remove old container %s: %v", containerName, err)
	}

	// Create container if it doesn't exist (or we just removed the old one)
	if server.ContainerID == "" || server.ContainerID != "" {
		// Always create a fresh container to avoid state issues
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
			return fmt.Errorf("failed to create container: %w", err)
		}

		server.ContainerID = containerID
		if err := s.repo.Update(server); err != nil {
			return err
		}
	}

	// Start container
	server.Status = models.StatusStarting
	if err := s.repo.Update(server); err != nil {
		return err
	}

	if err := s.dockerService.StartContainer(server.ContainerID); err != nil {
		server.Status = models.StatusError
		s.repo.Update(server)
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Wait for Minecraft server to be ready before marking as running
	// This prevents OOM kills when players try to join during startup
	log.Printf("Waiting for Minecraft server %s to be ready...", server.ID)
	if err := s.dockerService.WaitForServerReady(server.ContainerID, 60); err != nil {
		log.Printf("Warning: Minecraft server %s may not be fully ready: %v", server.ID, err)
		// Continue anyway - server might still work
	}

	// Update status
	now := time.Now()
	server.Status = models.StatusRunning
	server.LastStartedAt = &now
	server.LifecyclePhase = models.PhaseActive // Mark as active when running
	if err := s.repo.Update(server); err != nil {
		return err
	}

	// Broadcast WebSocket event
	if s.wsHub != nil {
		s.wsHub.Broadcast("server_started", map[string]interface{}{
			"server_id": server.ID,
			"name":      server.Name,
			"status":    server.Status,
			"port":      server.Port,
		})
	}

	// Publish event
	events.PublishServerStarted(server.ID, server.OwnerID)

	log.Printf("Started server %s", serverID)
	return nil
}

// StopServer stops a Minecraft server
func (s *MinecraftService) StopServer(serverID string, reason string) error {
	server, err := s.repo.FindByID(serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	if server.Status != models.StatusRunning {
		return fmt.Errorf("server not running (status: %s)", server.Status)
	}

	// Update status
	server.Status = models.StatusStopping
	if err := s.repo.Update(server); err != nil {
		return err
	}

	// Stop container
	if err := s.dockerService.StopContainer(server.ContainerID, 30); err != nil {
		server.Status = models.StatusError
		s.repo.Update(server)
		return fmt.Errorf("failed to stop container: %w", err)
	}

	// Update status
	now := time.Now()
	server.Status = models.StatusStopped
	server.LastStoppedAt = &now
	if err := s.repo.Update(server); err != nil {
		return err
	}

	// Broadcast WebSocket event
	if s.wsHub != nil {
		s.wsHub.Broadcast("server_stopped", map[string]interface{}{
			"server_id": server.ID,
			"name":      server.Name,
			"status":    server.Status,
			"reason":    reason,
		})
	}

	// Publish event
	events.PublishServerStopped(server.ID, reason)

	log.Printf("Stopped server %s (reason: %s)", serverID, reason)
	return nil
}

// DeleteServer deletes a server and its container
func (s *MinecraftService) DeleteServer(serverID string) error {
	log.Printf("Starting deletion of server %s", serverID)

	server, err := s.repo.FindByID(serverID)
	if err != nil {
		log.Printf("ERROR: server %s not found: %v", serverID, err)
		return fmt.Errorf("server not found: %w", err)
	}

	// Unregister from Velocity first
	if s.velocityService != nil && server.VelocityRegistered {
		if err := s.velocityService.UnregisterServer(serverID); err != nil {
			log.Printf("Warning: failed to unregister server from Velocity: %v", err)
		} else {
			log.Printf("Server unregistered from Velocity")
		}
	}

	// Stop if running
	if server.Status == models.StatusRunning {
		log.Printf("Stopping running server %s before deletion", serverID)
		if err := s.StopServer(serverID, "deleted"); err != nil {
			log.Printf("Warning: failed to stop server before deletion: %v", err)
		}
	}

	// Remove container
	if server.ContainerID != "" {
		log.Printf("Removing container %s", server.ContainerID)
		if err := s.dockerService.RemoveContainer(server.ContainerID, true); err != nil {
			log.Printf("Warning: failed to remove container: %v", err)
		} else {
			log.Printf("Container %s removed successfully", server.ContainerID)
		}
	}

	// Delete usage logs first (in case CASCADE is not set up yet)
	log.Printf("Deleting usage logs for server %s", serverID)
	if err := s.repo.DeleteServerUsageLogs(serverID); err != nil {
		log.Printf("Warning: failed to delete usage logs: %v", err)
	}

	// Delete from database
	log.Printf("Deleting server %s from database", serverID)
	if err := s.repo.Delete(serverID); err != nil {
		log.Printf("ERROR: failed to delete server from database: %v", err)
		return fmt.Errorf("failed to delete server: %w", err)
	}

	// Publish event
	events.PublishServerDeleted(server.ID, server.OwnerID)

	log.Printf("Successfully deleted server %s", serverID)
	return nil
}

// GetServer retrieves a server by ID
func (s *MinecraftService) GetServer(serverID string) (*models.MinecraftServer, error) {
	return s.repo.FindByID(serverID)
}

// ListServers lists all servers for an owner
func (s *MinecraftService) ListServers(ownerID string) ([]models.MinecraftServer, error) {
	if ownerID == "" {
		ownerID = "default"
	}
	return s.repo.FindByOwner(ownerID)
}

// ListAllServers lists ALL servers (admin function)
func (s *MinecraftService) ListAllServers() ([]models.MinecraftServer, error) {
	return s.repo.FindAll()
}

// CleanOrphanedServers removes servers with missing or stopped containers (admin function)
func (s *MinecraftService) CleanOrphanedServers() (int, error) {
	servers, err := s.repo.FindAll()
	if err != nil {
		return 0, err
	}

	count := 0
	for _, server := range servers {
		shouldDelete := false
		reason := ""

		// Case 1: Server has no container ID at all (ghost server)
		if server.ContainerID == "" {
			shouldDelete = true
			reason = "no container ID"
		} else {
			// Case 2: Server has container ID but container doesn't exist
			status, err := s.dockerService.GetContainerStatus(server.ContainerID)
			if err != nil || status == "" {
				shouldDelete = true
				reason = fmt.Sprintf("container %s not found", server.ContainerID[:12])
			}
		}

		if shouldDelete {
			log.Printf("Cleaning orphaned server %s (%s)", server.ID, reason)
			if err := s.DeleteServer(server.ID); err != nil {
				log.Printf("Warning: failed to delete orphaned server %s: %v", server.ID, err)
			} else {
				count++
			}
		}
	}

	log.Printf("Cleaned %d orphaned servers", count)
	return count, nil
}

// GetServerUsage retrieves usage logs for a server
func (s *MinecraftService) GetServerUsage(serverID string) ([]models.UsageLog, error) {
	return s.repo.GetServerUsageLogs(serverID)
}

// GetServerLogs retrieves Docker logs for a server with application events
func (s *MinecraftService) GetServerLogs(serverID string, tail int) (string, error) {
	server, err := s.repo.FindByID(serverID)
	if err != nil {
		return "", err
	}

	// Build header with server info and recent events
	var logOutput strings.Builder
	logOutput.WriteString("=== PayPerPlay Server Logs ===\n")
	logOutput.WriteString(fmt.Sprintf("Server: %s (ID: %s)\n", server.Name, server.ID))
	logOutput.WriteString(fmt.Sprintf("Status: %s\n", server.Status))
	logOutput.WriteString(fmt.Sprintf("Type: %s %s | RAM: %d MB | Port: %d\n",
		server.ServerType, server.MinecraftVersion, server.RAMMb, server.Port))

	// Show last start/stop times
	if server.LastStartedAt != nil {
		logOutput.WriteString(fmt.Sprintf("Last Started: %s\n", server.LastStartedAt.Format("2006-01-02 15:04:05")))
	}
	if server.LastStoppedAt != nil {
		logOutput.WriteString(fmt.Sprintf("Last Stopped: %s\n", server.LastStoppedAt.Format("2006-01-02 15:04:05")))
	}

	// Show recent usage logs (last 3 sessions)
	usageLogs, err := s.repo.GetServerUsageLogs(serverID)
	if err == nil && len(usageLogs) > 0 {
		logOutput.WriteString("\n=== Recent Sessions ===\n")
		count := 0
		for _, log := range usageLogs {
			if count >= 3 {
				break
			}
			duration := "running"
			if log.StoppedAt != nil {
				duration = fmt.Sprintf("%d seconds", log.DurationSeconds)
			}
			logOutput.WriteString(fmt.Sprintf("- Started: %s | Duration: %s | Reason: %s | Cost: €%.4f\n",
				log.StartedAt.Format("2006-01-02 15:04:05"), duration, log.ShutdownReason, log.CostEUR))
			count++
		}
	}

	logOutput.WriteString("\n=== Container Logs ===\n")

	// Get container logs if available
	if server.ContainerID == "" {
		logOutput.WriteString("No container created yet. Container will be created on first start.\n")
		return logOutput.String(), nil
	}

	containerLogs, err := s.dockerService.GetContainerLogs(server.ContainerID, fmt.Sprintf("%d", tail))
	if err != nil {
		logOutput.WriteString(fmt.Sprintf("Error fetching container logs: %v\n", err))
		logOutput.WriteString("Container might have been removed. Try starting the server to create a new container.\n")
		return logOutput.String(), nil
	}

	logOutput.WriteString(containerLogs)
	return logOutput.String(), nil
}

// calculateCost calculates the cost based on RAM and duration
func (s *MinecraftService) calculateCost(ramMB int, durationSeconds float64) float64 {
	durationHours := durationSeconds / 3600.0

	var rate float64
	if ramMB <= 2048 {
		rate = s.cfg.Rate2GB
	} else if ramMB <= 4096 {
		rate = s.cfg.Rate4GB
	} else if ramMB <= 8192 {
		rate = s.cfg.Rate8GB
	} else {
		rate = s.cfg.Rate16GB
	}

	cost := durationHours * rate
	return math.Round(cost*10000) / 10000 // Round to 4 decimals
}
