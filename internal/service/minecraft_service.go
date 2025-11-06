package service

import (
	"fmt"
	"log"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/payperplay/hosting/internal/docker"
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

	if err := s.repo.Create(server); err != nil {
		return nil, fmt.Errorf("failed to create server record: %w", err)
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

	// Create container if it doesn't exist
	if server.ContainerID == "" {
		containerID, err := s.dockerService.CreateContainer(
			server.ID,
			string(server.ServerType),
			server.MinecraftVersion,
			server.RAMMb,
			server.Port,
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

	// Update status
	now := time.Now()
	server.Status = models.StatusRunning
	server.LastStartedAt = &now
	if err := s.repo.Update(server); err != nil {
		return err
	}

	// Create usage log
	usageLog := &models.UsageLog{
		ServerID:  server.ID,
		StartedAt: time.Now(),
	}
	if err := s.repo.CreateUsageLog(usageLog); err != nil {
		log.Printf("Warning: failed to create usage log: %v", err)
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

	// Update usage log
	usageLog, err := s.repo.GetActiveUsageLog(server.ID)
	if err == nil && usageLog != nil {
		now := time.Now()
		usageLog.StoppedAt = &now
		duration := now.Sub(usageLog.StartedAt)
		usageLog.DurationSeconds = int(duration.Seconds())
		usageLog.CostEUR = s.calculateCost(server.RAMMb, duration.Seconds())
		usageLog.ShutdownReason = reason

		if err := s.repo.UpdateUsageLog(usageLog); err != nil {
			log.Printf("Warning: failed to update usage log: %v", err)
		}
	}

	// Broadcast WebSocket event
	if s.wsHub != nil {
		eventData := map[string]interface{}{
			"server_id": server.ID,
			"name":      server.Name,
			"status":    server.Status,
			"reason":    reason,
		}
		if usageLog != nil {
			eventData["cost"] = usageLog.CostEUR
			eventData["duration_seconds"] = usageLog.DurationSeconds
		}
		s.wsHub.Broadcast("server_stopped", eventData)
	}

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

// GetServerUsage retrieves usage logs for a server
func (s *MinecraftService) GetServerUsage(serverID string) ([]models.UsageLog, error) {
	return s.repo.GetServerUsageLogs(serverID)
}

// GetServerLogs retrieves Docker logs for a server
func (s *MinecraftService) GetServerLogs(serverID string, tail int) (string, error) {
	server, err := s.repo.FindByID(serverID)
	if err != nil {
		return "", err
	}

	if server.ContainerID == "" {
		return "", fmt.Errorf("no container for this server")
	}

	return s.dockerService.GetContainerLogs(server.ContainerID, fmt.Sprintf("%d", tail))
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
