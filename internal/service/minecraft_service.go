package service

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/payperplay/hosting/internal/docker"
	"github.com/payperplay/hosting/internal/events"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/rcon"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/config"
	"github.com/payperplay/hosting/pkg/logger"
)

type MinecraftService struct {
	repo                  *repository.ServerRepository
	dockerService         *docker.DockerService
	cfg                   *config.Config
	velocityService       VelocityServiceInterface // Interface to avoid circular dependency (DEPRECATED - use remoteVelocityClient)
	remoteVelocityClient  RemoteVelocityClientInterface // NEW: HTTP API client for remote Velocity server
	wsHub                 WebSocketHubInterface    // Interface for WebSocket broadcasting
	conductor             ConductorInterface        // Interface for capacity management
	archiveService        ArchiveServiceInterface   // Interface for archive management (Phase 3 lifecycle)
	backupService         *BackupService            // Backup service for pre-operation backups
}

// WebSocketHubInterface defines the methods needed from WebSocket Hub
type WebSocketHubInterface interface {
	Broadcast(messageType string, data interface{})
}

// DashboardWebSocketInterface defines the methods needed from Dashboard WebSocket
type DashboardWebSocketInterface interface {
	PublishEvent(eventType string, data interface{})
}

// VelocityServiceInterface defines the methods needed from VelocityService (DEPRECATED)
type VelocityServiceInterface interface {
	RegisterServer(server *models.MinecraftServer) error
	UnregisterServer(serverID string) error
	IsRunning() bool
}

// ArchiveServiceInterface defines the methods needed from ArchiveService
type ArchiveServiceInterface interface {
	UnarchiveServer(serverID string) error
	ArchiveServer(serverID string) error
}

// RemoteVelocityClientInterface defines the methods needed from RemoteVelocityClient
// This is the NEW way of communicating with Velocity via HTTP API
type RemoteVelocityClientInterface interface {
	RegisterServer(name, address string) error
	UnregisterServer(name string) error
}

// ConductorInterface defines the methods needed from Conductor for capacity management
type ConductorInterface interface {
	// CheckCapacity checks if there's enough capacity to start a server
	// DEPRECATED: Use AtomicAllocateRAM() instead to prevent race conditions
	CheckCapacity(requiredRAMMB int) (bool, int) // returns (hasCapacity, availableRAMMB)

	// CanStartServer checks if a server can start now (startup-delay + CPU + RAM guard)
	// Returns (canStart bool, reason string)
	CanStartServer(ramMB int) (bool, string)

	// AtomicReserveStartSlot atomically reserves a "starting" slot for CPU-Guard
	// Returns true if slot reserved, false if another server is already starting
	// CRITICAL: This must be called BEFORE Docker starts to prevent race conditions
	AtomicReserveStartSlot(serverID, serverName string, ramMB int) bool

	// ReleaseStartSlot removes a "starting" reservation if start fails
	ReleaseStartSlot(serverID string)

	// UpdateContainerStatus updates the status of a container in the registry
	// Used to transition from "starting" to "running" after server is ready
	UpdateContainerStatus(serverID, status string)

	// AtomicAllocateRAM atomically reserves RAM for a server
	// Returns true if allocation succeeded, false if insufficient capacity
	// THIS IS THE SAFE METHOD - prevents race conditions!
	// DEPRECATED: Use AtomicAllocateRAMOnNode() for multi-node support
	AtomicAllocateRAM(ramMB int) bool

	// ReleaseRAM atomically releases RAM when a server stops
	// DEPRECATED: Use ReleaseRAMOnNode() for multi-node support
	ReleaseRAM(ramMB int)

	// Multi-Node Support: Node Selection and RAM Management

	// SelectNodeForContainerAuto selects the best node for a new container
	// Uses the recommended node selection strategy based on fleet composition
	// Returns (nodeID, error)
	SelectNodeForContainerAuto(requiredRAMMB int) (string, error)

	// AtomicAllocateRAMOnNode atomically reserves RAM on a specific node
	// Returns true if allocation succeeded, false if insufficient capacity
	AtomicAllocateRAMOnNode(nodeID string, ramMB int) bool

	// ReleaseRAMOnNode atomically releases RAM from a specific node
	ReleaseRAMOnNode(nodeID string, ramMB int)

	// RegisterContainer registers a container in the registry with node tracking
	RegisterContainer(serverID, serverName, containerID, nodeID string, ramMB, dockerPort, minecraftPort int, status, minecraftVersion, serverType string)

	// GetContainer retrieves container info including node assignment
	GetContainer(serverID string) (containerInfo interface{}, exists bool)

	// EnqueueServer adds a server to the start queue if capacity is insufficient
	EnqueueServer(serverID, serverName string, requiredRAMMB int, userID string)

	// IsServerQueued checks if a server is in the start queue
	IsServerQueued(serverID string) bool

	// RemoveFromQueue removes a server from the start queue
	RemoveFromQueue(serverID string)

	// ProcessStartQueue attempts to start servers from the queue when capacity is available
	ProcessStartQueue()

	// TriggerScalingCheck triggers an immediate scaling evaluation
	// This should be called when a server is created, updated, or deleted
	// to ensure capacity is provisioned without waiting for the next scaling interval
	TriggerScalingCheck()

	// GetRemoteNode builds a RemoteNode struct from a nodeID for remote Docker operations
	GetRemoteNode(nodeID string) (*docker.RemoteNode, error)

	// GetRemoteDockerClient returns the RemoteDockerClient for remote node operations
	GetRemoteDockerClient() *docker.RemoteDockerClient

	// GetNode retrieves node information by nodeID (needed for proportional RAM calculations)
	// Returns (*conductor.Node, bool) where bool indicates if node exists
	GetNode(nodeID string) (interface{}, bool)

	// IsSystemNode checks if a node is a system node (cannot host Minecraft containers)
	// Returns (isSystemNode bool, error)
	IsSystemNode(nodeID string) (bool, error)
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
// DEPRECATED: Use SetRemoteVelocityClient instead
func (s *MinecraftService) SetVelocityService(velocityService VelocityServiceInterface) {
	s.velocityService = velocityService
}

// SetRemoteVelocityClient sets the remote Velocity API client (NEW)
func (s *MinecraftService) SetRemoteVelocityClient(client RemoteVelocityClientInterface) {
	s.remoteVelocityClient = client
}

// SetWebSocketHub sets the WebSocket hub for real-time updates
func (s *MinecraftService) SetWebSocketHub(wsHub WebSocketHubInterface) {
	s.wsHub = wsHub
}

// SetConductor sets the Conductor for capacity management
func (s *MinecraftService) SetConductor(conductor ConductorInterface) {
	s.conductor = conductor
}

// SetArchiveService sets the archive service for unarchiving servers on start
func (s *MinecraftService) SetArchiveService(archiveService ArchiveServiceInterface) {
	s.archiveService = archiveService
}

// SetBackupService sets the backup service for pre-operation backups
func (s *MinecraftService) SetBackupService(backupService *BackupService) {
	s.backupService = backupService
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
		Status:               models.StatusQueued, // Start in queue - Conductor will assign node
		IdleTimeoutSeconds:   s.cfg.DefaultIdleTimeout,
		AutoShutdownEnabled:  true,
		MaxPlayers:           20,
	}

	// FIX CONFIG-2: Validate configuration values before creating server
	if err := server.ValidateConfig(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
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

	// Add server to queue and trigger immediate scaling check
	if s.conductor != nil {
		// Enqueue the server - Conductor will assign it to a node when capacity is available
		s.conductor.EnqueueServer(server.ID, server.Name, server.RAMMb, server.OwnerID)

		// Trigger immediate scaling check to provision capacity if needed
		s.conductor.TriggerScalingCheck()
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

	// FIX #4: Multi-Start Deduplication
	// Prevent race condition from multiple start button clicks
	if server.Status == models.StatusRunning {
		return fmt.Errorf("server already running")
	}
	if server.Status == models.StatusStarting {
		return fmt.Errorf("server is already starting, please wait")
	}

	// PHASE 3 LIFECYCLE: Auto-unarchive if server is archived
	// This restores the server from Storage Box before starting
	if server.Status == models.StatusArchived {
		if s.archiveService == nil {
			return fmt.Errorf("server is archived but archive service not available")
		}

		logger.Info("LIFECYCLE: Server is archived, unarchiving before start", map[string]interface{}{
			"server_id":        serverID,
			"server_name":      server.Name,
			"archive_location": server.ArchiveLocation,
			"archive_size_mb":  server.ArchiveSize / 1024 / 1024,
		})

		if err := s.archiveService.UnarchiveServer(serverID); err != nil {
			return fmt.Errorf("failed to unarchive server: %w", err)
		}

		// Reload server to get updated status (should be 'stopped' now)
		server, err = s.repo.FindByID(serverID)
		if err != nil {
			return fmt.Errorf("failed to reload server after unarchive: %w", err)
		}

		logger.Info("LIFECYCLE: Server unarchived successfully, proceeding with start", map[string]interface{}{
			"server_id":   serverID,
			"server_name": server.Name,
			"new_status":  server.Status,
		})
	}

	// PRE-START RESOURCE GUARD: CPU + RAM protection
	// This is a CRITICAL FIX to prevent multiple parallel requests from overloading the system
	var selectedNodeID string
	ramAllocated := false
	startSlotReserved := false
	if s.conductor != nil {
		// Check if already queued
		if s.conductor.IsServerQueued(server.ID) {
			return fmt.Errorf("server is already queued for start (waiting for capacity)")
		}

		// CPU-GUARD: Check if we can start a server now (CPU + RAM checks)
		canStart, reason := s.conductor.CanStartServer(server.RAMMb)
		if !canStart {
			// Cannot start now - add to queue
			s.conductor.EnqueueServer(server.ID, server.Name, server.RAMMb, server.OwnerID)

			log.Printf("CPU_GUARD: Cannot start server %s (%s) - Added to queue", server.ID, reason)

			return fmt.Errorf("cannot start server (%s) - server queued for start, will auto-start when capacity available", reason)
		}

		// ATOMIC START SLOT RESERVATION: Immediately reserve the "starting" slot
		// This MUST happen BEFORE Docker starts to prevent race conditions!
		if !s.conductor.AtomicReserveStartSlot(server.ID, server.Name, server.RAMMb) {
			// Another server is already starting (race condition detected)
			s.conductor.EnqueueServer(server.ID, server.Name, server.RAMMb, server.OwnerID)

			log.Printf("CPU_GUARD: Start slot already taken for server %s - Added to queue", server.ID)

			return fmt.Errorf("another server is currently starting (CPU protection) - server queued for start, will auto-start when capacity available")
		}
		startSlotReserved = true

		// MULTI-NODE: Intelligent Node Selection
		// Select the best node for this container using automatic strategy selection
		nodeID, err := s.conductor.SelectNodeForContainerAuto(server.RAMMb)
		if err != nil {
			// No nodes available with sufficient capacity
			s.conductor.ReleaseStartSlot(server.ID)
			startSlotReserved = false

			// Add to queue - will auto-start when nodes become available
			s.conductor.EnqueueServer(server.ID, server.Name, server.RAMMb, server.OwnerID)

			log.Printf("NODE_SELECTION: No nodes available for server %s (%d MB required) - Added to queue: %v",
				server.ID, server.RAMMb, err)

			return fmt.Errorf("no healthy nodes available with sufficient capacity (%d MB required) - server queued for start", server.RAMMb)
		}
		selectedNodeID = nodeID

		// ATOMIC RAM ALLOCATION: Lock, check, and allocate in ONE operation on selected node
		// This prevents race conditions where multiple threads check capacity simultaneously
		if !s.conductor.AtomicAllocateRAMOnNode(selectedNodeID, server.RAMMb) {
			// Allocation failed - insufficient capacity
			// ROLLBACK: Release start slot
			s.conductor.ReleaseStartSlot(server.ID)
			startSlotReserved = false

			// Add to queue instead of starting
			s.conductor.EnqueueServer(server.ID, server.Name, server.RAMMb, server.OwnerID)

			log.Printf("RESOURCE_GUARD: Insufficient capacity on node %s for server %s (%d MB required) - Added to queue",
				selectedNodeID, server.ID, server.RAMMb)

			return fmt.Errorf("insufficient capacity to start server (%d MB required) - server queued for start, will auto-start when capacity available", server.RAMMb)
		}

		// RAM successfully allocated!
		ramAllocated = true
		log.Printf("RESOURCE_GUARD: RAM allocated atomically for server %s (%d MB) on node %s", server.ID, server.RAMMb, selectedNodeID)

		// Remove from queue if it was queued (in case of manual retry)
		s.conductor.RemoveFromQueue(server.ID)
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

	// Store the selected node ID in the database
	server.NodeID = selectedNodeID
	if err := s.repo.Update(server); err != nil {
		// ROLLBACK: Release RAM and start slot if database update failed
		if s.conductor != nil {
			if ramAllocated {
				s.conductor.ReleaseRAMOnNode(selectedNodeID, server.RAMMb)
				log.Printf("ROLLBACK: Released %d MB RAM on node %s for server %s after nodeID update failure", server.RAMMb, selectedNodeID, server.ID)
			}
			if startSlotReserved {
				s.conductor.ReleaseStartSlot(server.ID)
				log.Printf("ROLLBACK: Released start slot for server %s after nodeID update failure", server.ID)
			}
		}
		return fmt.Errorf("failed to update server with nodeID: %w", err)
	}

	log.Printf("Server %s assigned to node %s", server.ID, selectedNodeID)

	// CRITICAL: Remove any existing container with the same name before creating a new one
	// This prevents "port already allocated" errors from zombie containers
	containerName := fmt.Sprintf("mc-%s", server.ID)
	log.Printf("Checking for existing container %s before start", containerName)
	if err := s.dockerService.RemoveContainerByName(containerName); err != nil {
		log.Printf("Warning: failed to remove old container %s: %v", containerName, err)
	}

	// MULTI-NODE: Create container on selected node (local or remote)
	if server.ContainerID == "" || server.ContainerID != "" {
		// Always create a fresh container to avoid state issues
		var containerID string
		var err error

		if s.isLocalNode(selectedNodeID) {
			// LOCAL NODE: Use existing dockerService.CreateContainer()
			log.Printf("Creating container for server %s on local node", server.ID)
			containerID, err = s.dockerService.CreateContainer(
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
		} else {
			// REMOTE NODE: Use RemoteDockerClient with environment builder
			log.Printf("Creating container for server %s on remote node %s", server.ID, selectedNodeID)

			// Get remote node info
			remoteNode, err := s.conductor.GetRemoteNode(selectedNodeID)
			if err != nil {
				// ROLLBACK: Release RAM and start slot
				if s.conductor != nil {
					if ramAllocated {
						s.conductor.ReleaseRAMOnNode(selectedNodeID, server.RAMMb)
						log.Printf("ROLLBACK: Released %d MB RAM on node %s for server %s after GetRemoteNode failure", server.RAMMb, selectedNodeID, server.ID)
					}
					if startSlotReserved {
						s.conductor.ReleaseStartSlot(server.ID)
						log.Printf("ROLLBACK: Released start slot for server %s after GetRemoteNode failure", server.ID)
					}
				}
				return fmt.Errorf("failed to get remote node info: %w", err)
			}

			// Build container configuration using helper methods
			containerName := fmt.Sprintf("mc-%s", server.ID)
			imageName := docker.GetDockerImageName(string(server.ServerType))
			env := docker.BuildContainerEnv(server)
			portBindings := docker.BuildPortBindings(server.Port)
			binds := docker.BuildVolumeBinds(server.ID, "/minecraft/servers")

			// Create and start container on remote node
			ctx := context.Background()
			containerID, err = s.conductor.GetRemoteDockerClient().StartContainer(
				ctx,
				remoteNode,
				containerName,
				imageName,
				env,
				portBindings,
				binds,
				server.RAMMb,
			)
		}

		if err != nil {
			// FIX #1: Volume Loss Fallback
			// If volume not found and server was stopped, try to restore from archive
			errorMsg := err.Error()
			if (strings.Contains(errorMsg, "volume") || strings.Contains(errorMsg, "bind source path does not exist")) &&
			   server.Status == models.StatusStopped && s.archiveService != nil {
				logger.Warn("VOLUME-LOSS: Volume missing for stopped server, attempting archive restore", map[string]interface{}{
					"server_id": server.ID,
					"error":     errorMsg,
				})

				// Try to restore from archive
				if archiveErr := s.archiveService.UnarchiveServer(serverID); archiveErr == nil {
					logger.Info("VOLUME-LOSS: Successfully restored from archive, retrying start", map[string]interface{}{
						"server_id": server.ID,
					})
					// Retry container creation after unarchive
					if s.isLocalNode(selectedNodeID) {
						containerID, err = s.dockerService.CreateContainer(
							server.ID, string(server.ServerType), server.MinecraftVersion, server.RAMMb, server.Port,
							server.MaxPlayers, server.Gamemode, server.Difficulty, server.PVP, server.EnableCommandBlock, server.LevelSeed,
							server.ViewDistance, server.SimulationDistance, server.AllowNether, server.AllowEnd, server.GenerateStructures,
							server.WorldType, server.BonusChest, server.MaxWorldSize, server.SpawnProtection, server.SpawnAnimals,
							server.SpawnMonsters, server.SpawnNPCs, server.MaxTickTime, server.NetworkCompressionThreshold, server.MOTD,
						)
					} else {
						remoteNode, _ := s.conductor.GetRemoteNode(selectedNodeID)
						containerName := fmt.Sprintf("mc-%s", server.ID)
						imageName := docker.GetDockerImageName(string(server.ServerType))
						env := docker.BuildContainerEnv(server)
						portBindings := docker.BuildPortBindings(server.Port)
						binds := docker.BuildVolumeBinds(server.ID, "/minecraft/servers")
						ctx := context.Background()
						containerID, err = s.conductor.GetRemoteDockerClient().StartContainer(ctx, remoteNode, containerName, imageName, env, portBindings, binds, server.RAMMb)
					}
				}
			}

			// If still error, rollback
			if err != nil {
				if s.conductor != nil {
					if ramAllocated {
						s.conductor.ReleaseRAMOnNode(selectedNodeID, server.RAMMb)
						log.Printf("ROLLBACK: Released %d MB RAM on node %s for server %s after container creation failure", server.RAMMb, selectedNodeID, server.ID)
					}
					if startSlotReserved {
						s.conductor.ReleaseStartSlot(server.ID)
						log.Printf("ROLLBACK: Released start slot for server %s after container creation failure", server.ID)
					}
				}
				return fmt.Errorf("failed to create container: %w", err)
			}
		}

		server.ContainerID = containerID
		if err := s.repo.Update(server); err != nil {
			// ROLLBACK: Release RAM and start slot if database update failed
			if s.conductor != nil {
				if ramAllocated {
					s.conductor.ReleaseRAMOnNode(selectedNodeID, server.RAMMb)
					log.Printf("ROLLBACK: Released %d MB RAM on node %s for server %s after database update failure", server.RAMMb, selectedNodeID, server.ID)
				}
				if startSlotReserved {
					s.conductor.ReleaseStartSlot(server.ID)
					log.Printf("ROLLBACK: Released start slot for server %s after database update failure", server.ID)
				}
			}
			return err
		}
	}

	// Start container
	server.Status = models.StatusStarting
	if err := s.repo.Update(server); err != nil {
		// ROLLBACK: Release RAM and start slot if database update failed
		if s.conductor != nil {
			if ramAllocated {
				s.conductor.ReleaseRAMOnNode(selectedNodeID, server.RAMMb)
				log.Printf("ROLLBACK: Released %d MB RAM on node %s for server %s after status update failure", server.RAMMb, selectedNodeID, server.ID)
			}
			if startSlotReserved {
				s.conductor.ReleaseStartSlot(server.ID)
				log.Printf("ROLLBACK: Released start slot for server %s after status update failure", server.ID)
			}
		}
		return err
	}

	// Register container in ContainerRegistry with "starting" status for dashboard tracking
	// This allows dashboard to show blue (starting) → green (running) transition
	if s.conductor != nil {
		s.conductor.RegisterContainer(
			server.ID,
			server.Name,
			server.ContainerID, // Use server.ContainerID (set earlier in the function)
			selectedNodeID,
			server.RAMMb,
			server.Port, // DockerPort = same as MinecraftPort (1:1 port mapping)
			server.Port, // MinecraftPort
			string(models.StatusStarting), // Use "starting" status to show blue in dashboard
			server.MinecraftVersion,
			string(server.ServerType),
		)
	}

	// Only call StartContainer for LOCAL nodes (remote containers are already started by RemoteDockerClient.StartContainer)
	if s.isLocalNode(selectedNodeID) {
		if err := s.dockerService.StartContainer(server.ContainerID); err != nil {
			server.Status = models.StatusError
			s.repo.Update(server)
			// ROLLBACK: Release RAM and start slot if container start failed
			if s.conductor != nil {
				if ramAllocated {
					s.conductor.ReleaseRAMOnNode(selectedNodeID, server.RAMMb)
					log.Printf("ROLLBACK: Released %d MB RAM on node %s for server %s after container start failure", server.RAMMb, selectedNodeID, server.ID)
				}
				if startSlotReserved {
					s.conductor.ReleaseStartSlot(server.ID)
					log.Printf("ROLLBACK: Released start slot for server %s after container start failure", server.ID)
				}
			}
			return fmt.Errorf("failed to start container: %w", err)
		}
	} else {
		log.Printf("Skipping StartContainer call for server %s on remote node %s (already started by RemoteDockerClient)", server.ID, selectedNodeID)
	}

	// Wait for Minecraft server to be ready before marking as running
	// This prevents OOM kills when players try to join during startup
	log.Printf("Waiting for Minecraft server %s to be ready...", server.ID)

	// MULTI-NODE FIX: Route readiness check based on node type (local vs remote)
	if s.isLocalNode(selectedNodeID) {
		// LOCAL NODE: Use local Docker client
		if err := s.dockerService.WaitForServerReady(server.ContainerID, 60); err != nil {
			log.Printf("Warning: Minecraft server %s may not be fully ready: %v", server.ID, err)
			// Continue anyway - server might still work
		}
	} else {
		// REMOTE NODE: Use RemoteDockerClient with SSH
		if s.conductor != nil {
			remoteNode, err := s.conductor.GetRemoteNode(selectedNodeID)
			if err != nil {
				log.Printf("Warning: Failed to get remote node for readiness check: %v", err)
			} else {
				ctx := context.Background()
				if err := s.conductor.GetRemoteDockerClient().WaitForServerReady(ctx, remoteNode, server.ContainerID, 60); err != nil {
					log.Printf("Warning: Remote Minecraft server %s may not be fully ready: %v", server.ID, err)
					// Continue anyway - server might still work
				}
			}
		}
	}

	// Update status
	now := time.Now()
	server.Status = models.StatusRunning
	server.LastStartedAt = &now
	server.LifecyclePhase = models.PhaseActive // Mark as active when running
	if err := s.repo.Update(server); err != nil {
		return err
	}

	// CPU-GUARD: Update ContainerRegistry status from "starting" to "running"
	// This releases the CPU-Guard and allows queued servers to start
	if s.conductor != nil {
		s.conductor.UpdateContainerStatus(server.ID, "running")

		// Trigger queue processing - now that this server is running, queued servers can start
		go s.conductor.ProcessStartQueue()
	}

	// VELOCITY: Register server with Velocity proxy via HTTP API
	if s.remoteVelocityClient != nil {
		// Build server address for Velocity to connect to
		// Format: "host:port" where host is the actual Node IP and port is the Docker host port
		velocityServerName := fmt.Sprintf("mc-%s", server.ID)

		// Get the actual node IP where the server is running
		var serverIP string
		if s.isLocalNode(selectedNodeID) {
			// Local node: use Control Plane IP
			serverIP = s.cfg.ControlPlaneIP
		} else {
			// Remote node: get node IP from Conductor
			remoteNode, err := s.conductor.GetRemoteNode(selectedNodeID)
			if err != nil {
				log.Printf("Warning: Failed to get node IP for Velocity registration: %v", err)
				serverIP = s.cfg.ControlPlaneIP // Fallback to Control Plane
			} else {
				serverIP = remoteNode.IPAddress
			}
		}

		serverAddress := fmt.Sprintf("%s:%d", serverIP, server.Port)

		if err := s.remoteVelocityClient.RegisterServer(velocityServerName, serverAddress); err != nil {
			log.Printf("Warning: Failed to register server %s with Velocity: %v", server.ID, err)
			// Don't fail the entire operation - server is still usable, just not via Velocity
		} else {
			log.Printf("Server %s registered with Velocity as %s at %s", server.ID, velocityServerName, serverAddress)
		}
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

// StartServerFromQueue starts a server that was dequeued from the start queue
// This method BYPASSES queue checks since capacity was already verified during dequeue
// However, it STILL maintains CPU-Guard and atomic RAM allocation for race condition protection
func (s *MinecraftService) StartServerFromQueue(serverID string) error {
	server, err := s.repo.FindByID(serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	if server.Status == models.StatusRunning {
		return fmt.Errorf("server already running")
	}

	// QUEUE-BYPASS: Skip capacity and queue checks - we know capacity was available when dequeued
	// However, we STILL need CPU-Guard slot reservation and RAM allocation for thread safety!

	var selectedNodeID string
	ramAllocated := false
	startSlotReserved := false
	if s.conductor != nil {
		// ATOMIC START SLOT RESERVATION: Prevent multiple parallel starts
		if !s.conductor.AtomicReserveStartSlot(server.ID, server.Name, server.RAMMb) {
			// Another server is starting - this shouldn't happen but handle it
			log.Printf("CPU_GUARD: Start slot taken for queued server %s - re-queuing", server.ID)
			s.conductor.EnqueueServer(server.ID, server.Name, server.RAMMb, server.OwnerID)
			return fmt.Errorf("start slot unavailable - server re-queued")
		}
		startSlotReserved = true

		// MULTI-NODE: Intelligent Node Selection for queued server
		nodeID, err := s.conductor.SelectNodeForContainerAuto(server.RAMMb)
		if err != nil {
			// No nodes available - re-queue
			s.conductor.ReleaseStartSlot(server.ID)
			startSlotReserved = false

			s.conductor.EnqueueServer(server.ID, server.Name, server.RAMMb, server.OwnerID)
			log.Printf("QUEUE_START: No nodes available for queued server %s (%d MB) - re-queued: %v",
				server.ID, server.RAMMb, err)

			return fmt.Errorf("no healthy nodes available - server re-queued")
		}
		selectedNodeID = nodeID

		// ATOMIC RAM ALLOCATION: Even though capacity was checked during dequeue,
		// we still need to atomically allocate to prevent race conditions
		if !s.conductor.AtomicAllocateRAMOnNode(selectedNodeID, server.RAMMb) {
			// Allocation failed - insufficient capacity (capacity changed since dequeue)
			s.conductor.ReleaseStartSlot(server.ID)
			startSlotReserved = false

			// Re-queue for retry
			s.conductor.EnqueueServer(server.ID, server.Name, server.RAMMb, server.OwnerID)
			log.Printf("QUEUE_START: RAM allocation failed on node %s for queued server %s (%d MB) - re-queued",
				selectedNodeID, server.ID, server.RAMMb)

			return fmt.Errorf("insufficient capacity (changed since dequeue) - server re-queued")
		}

		ramAllocated = true
		log.Printf("QUEUE_START: Starting queued server %s (%d MB RAM allocated) on node %s", server.ID, server.RAMMb, selectedNodeID)
	}

	// From here, the logic is IDENTICAL to StartServer (lines 285-447)

	// Store the selected node ID in the database
	server.NodeID = selectedNodeID
	if err := s.repo.Update(server); err != nil {
		// ROLLBACK: Release RAM and start slot if database update failed
		if s.conductor != nil {
			if ramAllocated {
				s.conductor.ReleaseRAMOnNode(selectedNodeID, server.RAMMb)
				log.Printf("ROLLBACK: Released %d MB RAM on node %s for queued server %s after nodeID update failure", server.RAMMb, selectedNodeID, server.ID)
			}
			if startSlotReserved {
				s.conductor.ReleaseStartSlot(server.ID)
				log.Printf("ROLLBACK: Released start slot for queued server %s after nodeID update failure", server.ID)
			}
		}
		return fmt.Errorf("failed to update queued server with nodeID: %w", err)
	}

	log.Printf("Queued server %s assigned to node %s", server.ID, selectedNodeID)

	// PROPORTIONAL RAM OVERHEAD: Calculate actual RAM allocation based on node's reduction factor
	if s.conductor != nil {
		nodeInterface, exists := s.conductor.GetNode(selectedNodeID)
		if exists {
			// Type assert to *conductor.Node (via reflection-safe type switch)
			type NodeWithRAMCalculation interface {
				CalculateActualRAM(bookedRAMMB int) int
				GetReductionFactor() float64
			}

			if node, ok := nodeInterface.(NodeWithRAMCalculation); ok {
				server.ActualRAMMB = node.CalculateActualRAM(server.RAMMb)

				logger.Info("Container RAM calculated with proportional overhead", map[string]interface{}{
					"server_id":        server.ID,
					"booked_ram_mb":    server.RAMMb,
					"actual_ram_mb":    server.ActualRAMMB,
					"reduction_factor": node.GetReductionFactor(),
					"system_share_mb":  server.RAMMb - server.ActualRAMMB,
					"node_id":          selectedNodeID,
				})

				// Update database with actual RAM
				if err := s.repo.Update(server); err != nil {
					log.Printf("Warning: Failed to update ActualRAMMB for server %s: %v", server.ID, err)
				}
			} else {
				// Fallback: Type assertion failed
				log.Printf("Warning: Node type assertion failed for %s, using booked RAM as actual", selectedNodeID)
				server.ActualRAMMB = server.RAMMb
			}
		} else {
			// Fallback: If node not found, use booked RAM (shouldn't happen)
			log.Printf("Warning: Node %s not found, using booked RAM as actual", selectedNodeID)
			server.ActualRAMMB = server.RAMMb
		}
	} else {
		// Fallback: If conductor not available, use booked RAM
		server.ActualRAMMB = server.RAMMb
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
	containerName := fmt.Sprintf("mc-%s", server.ID)
	log.Printf("Checking for existing container %s before start", containerName)
	if err := s.dockerService.RemoveContainerByName(containerName); err != nil {
		log.Printf("Warning: failed to remove old container %s: %v", containerName, err)
	}

	// Create container with local/remote routing
	// PROPORTIONAL OVERHEAD: Use ActualRAMMB for Docker container limits
	actualRAM := server.ActualRAMMB
	if actualRAM == 0 {
		// Fallback to booked RAM if ActualRAM not calculated
		actualRAM = server.RAMMb
		log.Printf("Warning: ActualRAMMB not set for server %s, using booked RAM %d MB", server.ID, actualRAM)
	}

	var containerID string
	if server.ContainerID == "" || server.ContainerID != "" {
		// Route container creation based on node type
		if s.isLocalNode(selectedNodeID) {
			// LOCAL NODE: Use local dockerService
			log.Printf("Creating container for queued server %s on LOCAL node with %d MB actual RAM", server.ID, actualRAM)
			containerID, err = s.dockerService.CreateContainer(
				server.ID,
				string(server.ServerType),
				server.MinecraftVersion,
				actualRAM,
				server.Port,
				server.MaxPlayers,
				server.Gamemode,
				server.Difficulty,
				server.PVP,
				server.EnableCommandBlock,
				server.LevelSeed,
				server.ViewDistance,
				server.SimulationDistance,
				server.AllowNether,
				server.AllowEnd,
				server.GenerateStructures,
				server.WorldType,
				server.BonusChest,
				server.MaxWorldSize,
				server.SpawnProtection,
				server.SpawnAnimals,
				server.SpawnMonsters,
				server.SpawnNPCs,
				server.MaxTickTime,
				server.NetworkCompressionThreshold,
				server.MOTD,
			)
		} else {
			// REMOTE NODE: Use RemoteDockerClient with environment builder
			log.Printf("Creating container for queued server %s on remote node %s", server.ID, selectedNodeID)

			// Get remote node info
			remoteNode, err := s.conductor.GetRemoteNode(selectedNodeID)
			if err != nil {
				// ROLLBACK: Release RAM and start slot
				s.conductor.ReleaseRAMOnNode(selectedNodeID, server.RAMMb)
				s.conductor.ReleaseStartSlot(server.ID)
				return fmt.Errorf("failed to get remote node info for queued server: %w", err)
			}

			// Build container configuration using helper methods
			containerName := fmt.Sprintf("mc-%s", server.ID)
			imageName := docker.GetDockerImageName(string(server.ServerType))
			env := docker.BuildContainerEnv(server)
			portBindings := docker.BuildPortBindings(server.Port)
			binds := docker.BuildVolumeBinds(server.ID, "/minecraft/servers")

			// Create and start container on remote node
			ctx := context.Background()
			containerID, err = s.conductor.GetRemoteDockerClient().StartContainer(
				ctx,
				remoteNode,
				containerName,
				imageName,
				env,
				portBindings,
				binds,
				server.RAMMb,
			)
		}

		if err != nil {
			// ROLLBACK
			if s.conductor != nil {
				if ramAllocated {
					s.conductor.ReleaseRAMOnNode(selectedNodeID, server.RAMMb)
					log.Printf("ROLLBACK: Released %d MB RAM on node %s for queued server %s after container creation failure", server.RAMMb, selectedNodeID, server.ID)
				}
				if startSlotReserved {
					s.conductor.ReleaseStartSlot(server.ID)
					log.Printf("ROLLBACK: Released start slot for queued server %s after container creation failure", server.ID)
				}
			}
			return fmt.Errorf("failed to create container: %w", err)
		}

		server.ContainerID = containerID
		if err := s.repo.Update(server); err != nil {
			// ROLLBACK
			if s.conductor != nil {
				if ramAllocated {
					s.conductor.ReleaseRAMOnNode(selectedNodeID, server.RAMMb)
					log.Printf("ROLLBACK: Released %d MB RAM on node %s for queued server %s after database update failure", server.RAMMb, selectedNodeID, server.ID)
				}
				if startSlotReserved {
					s.conductor.ReleaseStartSlot(server.ID)
					log.Printf("ROLLBACK: Released start slot for queued server %s after database update failure", server.ID)
				}
			}
			return err
		}
	}

	// Start container (only for LOCAL nodes - remote nodes are already started)
	server.Status = models.StatusStarting
	if err := s.repo.Update(server); err != nil {
		// ROLLBACK
		if s.conductor != nil {
			if ramAllocated {
				s.conductor.ReleaseRAMOnNode(selectedNodeID, server.RAMMb)
				log.Printf("ROLLBACK: Released %d MB RAM on node %s for queued server %s after status update failure", server.RAMMb, selectedNodeID, server.ID)
			}
			if startSlotReserved {
				s.conductor.ReleaseStartSlot(server.ID)
				log.Printf("ROLLBACK: Released start slot for queued server %s after status update failure", server.ID)
			}
		}
		return err
	}

	// Register container in ContainerRegistry with "starting" status for dashboard tracking
	// This allows dashboard to show blue (starting) → green (running) transition
	if s.conductor != nil {
		s.conductor.RegisterContainer(
			server.ID,
			server.Name,
			server.ContainerID, // Use server.ContainerID (set earlier in the function)
			selectedNodeID,
			server.RAMMb,
			server.Port, // DockerPort = same as MinecraftPort (1:1 port mapping)
			server.Port, // MinecraftPort
			string(models.StatusStarting), // Use "starting" status to show blue in dashboard
			server.MinecraftVersion,
			string(server.ServerType),
		)
	}

	// Only call StartContainer for LOCAL nodes (remote containers are already started by RemoteDockerClient.StartContainer)
	if s.isLocalNode(selectedNodeID) {
		if err := s.dockerService.StartContainer(server.ContainerID); err != nil {
			server.Status = models.StatusError
			s.repo.Update(server)
			// ROLLBACK
			if s.conductor != nil {
				if ramAllocated {
					s.conductor.ReleaseRAMOnNode(selectedNodeID, server.RAMMb)
					log.Printf("ROLLBACK: Released %d MB RAM on node %s for queued server %s after container start failure", server.RAMMb, selectedNodeID, server.ID)
				}
				if startSlotReserved {
					s.conductor.ReleaseStartSlot(server.ID)
					log.Printf("ROLLBACK: Released start slot for queued server %s after container start failure", server.ID)
				}
			}
			return fmt.Errorf("failed to start container: %w", err)
		}
	} else {
		log.Printf("Skipping StartContainer call for queued server %s on remote node %s (already started by RemoteDockerClient)", server.ID, selectedNodeID)
	}

	// Wait for Minecraft server to be ready
	log.Printf("Waiting for Minecraft server %s to be ready...", server.ID)

	// MULTI-NODE FIX: Route readiness check based on node type (local vs remote)
	if s.isLocalNode(selectedNodeID) {
		// LOCAL NODE: Use local Docker client
		if err := s.dockerService.WaitForServerReady(server.ContainerID, 60); err != nil {
			log.Printf("Warning: Minecraft server %s may not be fully ready: %v", server.ID, err)
		}
	} else {
		// REMOTE NODE: Use RemoteDockerClient with SSH
		if s.conductor != nil {
			remoteNode, err := s.conductor.GetRemoteNode(selectedNodeID)
			if err != nil {
				log.Printf("Warning: Failed to get remote node for readiness check: %v", err)
			} else {
				ctx := context.Background()
				if err := s.conductor.GetRemoteDockerClient().WaitForServerReady(ctx, remoteNode, server.ContainerID, 60); err != nil {
					log.Printf("Warning: Remote Minecraft server %s may not be fully ready: %v", server.ID, err)
				}
			}
		}
	}

	// Update status
	now := time.Now()
	server.Status = models.StatusRunning
	server.LastStartedAt = &now
	server.LifecyclePhase = models.PhaseActive
	if err := s.repo.Update(server); err != nil {
		return err
	}

	// CPU-GUARD: Update ContainerRegistry status from "starting" to "running"
	// Container was already registered with "starting" status earlier
	if s.conductor != nil {
		s.conductor.UpdateContainerStatus(server.ID, "running")

		// Trigger queue processing - queued servers can now start
		go s.conductor.ProcessStartQueue()
	}

	// VELOCITY: Register server with Velocity proxy via HTTP API
	if s.remoteVelocityClient != nil {
		velocityServerName := fmt.Sprintf("mc-%s", server.ID)

		// Get the actual node IP where the server is running
		var serverIP string
		if s.isLocalNode(selectedNodeID) {
			// Local node: use Control Plane IP
			serverIP = s.cfg.ControlPlaneIP
		} else {
			// Remote node: get node IP from Conductor
			remoteNode, err := s.conductor.GetRemoteNode(selectedNodeID)
			if err != nil {
				log.Printf("Warning: Failed to get node IP for Velocity registration: %v", err)
				serverIP = s.cfg.ControlPlaneIP // Fallback to Control Plane
			} else {
				serverIP = remoteNode.IPAddress
			}
		}

		serverAddress := fmt.Sprintf("%s:%d", serverIP, server.Port)

		if err := s.remoteVelocityClient.RegisterServer(velocityServerName, serverAddress); err != nil {
			log.Printf("Warning: Failed to register queued server %s with Velocity: %v", server.ID, err)
		} else {
			log.Printf("Queued server %s registered with Velocity as %s at %s", server.ID, velocityServerName, serverAddress)
		}
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

	log.Printf("QUEUE_START: Successfully started queued server %s", serverID)
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

	// FIX SERVER-8: Send graceful shutdown warning via RCON before stopping
	// Give players time to save their progress and disconnect gracefully
	s.sendShutdownWarning(server)

	// Stop container (MULTI-NODE: Support both local and remote containers)
	// Determine if container is on remote node or local node
	nodeID := server.NodeID
	if nodeID == "" {
		nodeID = "local-node" // Fallback for legacy servers
	}

	isRemote := nodeID != "local-node"

	var stopErr error
	if isRemote && s.conductor != nil && s.conductor.GetRemoteDockerClient() != nil {
		// REMOTE: Stop container via SSH on remote worker node
		log.Printf("Stopping remote container %s on node %s", server.ContainerID, nodeID)

		// Get remote node info with IP address from Conductor
		remoteNode, err := s.conductor.GetRemoteNode(nodeID)
		if err != nil {
			log.Printf("ERROR: Failed to get remote node %s: %v", nodeID, err)
			stopErr = fmt.Errorf("failed to get remote node: %w", err)
		} else {
			// Stop container via remote client
			ctx := context.Background()
			stopErr = s.conductor.GetRemoteDockerClient().StopContainer(ctx, remoteNode, server.ContainerID, 30)
		}
		if stopErr != nil {
			log.Printf("ERROR: Failed to stop remote container %s on node %s: %v", server.ContainerID, nodeID, stopErr)
		} else {
			log.Printf("Remote container %s stopped successfully on node %s", server.ContainerID, nodeID)
		}
	} else {
		// LOCAL: Stop container via local Docker daemon
		log.Printf("Stopping local container %s", server.ContainerID)
		stopErr = s.dockerService.StopContainer(server.ContainerID, 30)
		if stopErr != nil {
			log.Printf("ERROR: Failed to stop local container %s: %v", server.ContainerID, stopErr)
		}
	}

	if stopErr != nil {
		server.Status = models.StatusError
		s.repo.Update(server)
		return fmt.Errorf("failed to stop container: %w", stopErr)
	}

	// Update status
	now := time.Now()
	server.Status = models.StatusStopped
	server.LastStoppedAt = &now
	if err := s.repo.Update(server); err != nil {
		return err
	}

	// FIX #5: Archive Timing Gap - Immediate archive check on stop
	// If server was already stopped for >48h before this stop, archive immediately
	if server.LastStoppedAt != nil && time.Since(*server.LastStoppedAt) > 48*time.Hour {
		if s.archiveService != nil {
			logger.Info("LIFECYCLE: Server stopped after >48h idle, triggering immediate archival", map[string]interface{}{
				"server_id":       server.ID,
				"idle_duration_h": time.Since(*server.LastStoppedAt).Hours(),
			})
			// Archive asynchronously to not block the stop operation
			go func() {
				if err := s.archiveService.ArchiveServer(server.ID); err != nil {
					logger.Error("LIFECYCLE: Failed to archive server immediately after stop", err, map[string]interface{}{
						"server_id": server.ID,
					})
				}
			}()
		}
	}

	// Release RAM when server stops (critical for capacity management)
	// MULTI-NODE: Use the node ID stored in the database
	if s.conductor != nil {
		// Use nodeID from database (defaults to "local-node" for backward compatibility)
		nodeID := server.NodeID
		if nodeID == "" {
			nodeID = "local-node" // Fallback for legacy servers
		}

		s.conductor.ReleaseRAMOnNode(nodeID, server.RAMMb)
		log.Printf("RESOURCE_RELEASE: Released %d MB RAM on node %s for server %s", server.RAMMb, nodeID, server.ID)

		// Trigger queue processing - maybe now we have capacity for queued servers
		go s.conductor.ProcessStartQueue()
	}

	// VELOCITY: Unregister server from Velocity proxy via HTTP API
	if s.remoteVelocityClient != nil {
		velocityServerName := fmt.Sprintf("mc-%s", server.ID)

		if err := s.remoteVelocityClient.UnregisterServer(velocityServerName); err != nil {
			log.Printf("Warning: Failed to unregister server %s from Velocity: %v", server.ID, err)
		} else {
			log.Printf("Server %s unregistered from Velocity", server.ID)
		}
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

	// FIX SERVER-2: Block deletion if server is starting or queued to prevent race conditions
	if server.Status == models.StatusStarting || server.Status == models.StatusQueued {
		logger.Warn("DELETE: Cannot delete server in transitional state", map[string]interface{}{
			"server_id": serverID,
			"status":    server.Status,
		})
		return fmt.Errorf("cannot delete server while %s - please wait or stop the server first", server.Status)
	}

	// FIX #8: Pre-Deletion Backup Failure - Block deletion if backup fails (except quota)
	if s.backupService != nil {
		logger.Info("DELETE: Creating pre-deletion backup", map[string]interface{}{
			"server_id":   serverID,
			"server_name": server.Name,
		})

		_, err := s.backupService.CreateBackup(
			serverID,
			models.BackupTypePreDeletion,
			fmt.Sprintf("Pre-deletion safety backup for %s", server.Name),
			nil, // No user ID for automated backups
			0,   // Use default retention (30 days for safety)
		)
		if err != nil {
			// Check if error is quota-related (allow deletion to proceed)
			errorMsg := err.Error()
			isQuotaError := strings.Contains(errorMsg, "quota exceeded") ||
							strings.Contains(errorMsg, "quota limit") ||
							strings.Contains(errorMsg, "insufficient quota")

			if isQuotaError {
				logger.Warn("DELETE: Pre-deletion backup skipped due to quota (deletion allowed)", map[string]interface{}{
					"server_id": serverID,
					"error":     errorMsg,
				})
				// Allow deletion to proceed - user has reached backup quota
			} else {
				// Technical failure (not quota) - block deletion to prevent data loss
				logger.Error("DELETE: Pre-deletion backup failed - blocking deletion", err, map[string]interface{}{
					"server_id": serverID,
				})
				return fmt.Errorf("pre-deletion backup failed: %w (deletion blocked to prevent data loss)", err)
			}
		} else {
			logger.Info("DELETE: Pre-deletion backup created successfully", map[string]interface{}{
				"server_id": serverID,
			})
		}
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

	// Remove container (MULTI-NODE: Support both local and remote containers)
	if server.ContainerID != "" {
		nodeID := server.NodeID
		if nodeID == "" {
			nodeID = "local-node" // Fallback for legacy servers
		}

		isRemote := nodeID != "local-node"

		log.Printf("Removing container %s from node %s", server.ContainerID, nodeID)

		var removeErr error
		if isRemote && s.conductor != nil && s.conductor.GetRemoteDockerClient() != nil {
			// REMOTE: Remove container via SSH on remote worker node
			remoteNode, err := s.conductor.GetRemoteNode(nodeID)
			if err != nil {
				log.Printf("ERROR: Failed to get remote node %s: %v", nodeID, err)
				removeErr = fmt.Errorf("failed to get remote node: %w", err)
			} else {
				ctx := context.Background()
				removeErr = s.conductor.GetRemoteDockerClient().RemoveContainer(ctx, remoteNode, server.ContainerID, true)
			}
			if removeErr != nil {
				log.Printf("Warning: failed to remove remote container %s on node %s: %v", server.ContainerID, nodeID, removeErr)
			} else {
				log.Printf("Remote container %s removed successfully from node %s", server.ContainerID, nodeID)
			}
		} else {
			// LOCAL: Remove container via local Docker daemon
			removeErr = s.dockerService.RemoveContainer(server.ContainerID, true)
			if removeErr != nil {
				log.Printf("Warning: failed to remove local container %s: %v", server.ContainerID, removeErr)
			} else {
				log.Printf("Container %s removed successfully", server.ContainerID)
			}
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

	// Trigger immediate scaling check to scale down if needed
	if s.conductor != nil {
		s.conductor.TriggerScalingCheck()
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

// ListAllServers lists ALL servers (admin function)
func (s *MinecraftService) ListAllServers() ([]models.MinecraftServer, error) {
	return s.repo.FindAll()
}

// ListArchivedServers lists archived servers (optionally filtered by owner)
func (s *MinecraftService) ListArchivedServers(ownerID string) ([]models.MinecraftServer, error) {
	return s.repo.FindArchivedServers(ownerID)
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

// CalculateCost calculates the cost based on RAM and duration (exported for billing integration)
func (s *MinecraftService) CalculateCost(ramMB int, durationSeconds float64) float64 {
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

// UpgradeServerRAM upgrades the RAM allocation for a server
// This implements the Stop-Update-Start workflow for RAM upgrades
func (s *MinecraftService) UpgradeServerRAM(serverID string, newRAMMB int) error {
	// Validation
	if newRAMMB < 512 || newRAMMB > 16384 {
		return fmt.Errorf("invalid RAM size: must be between 512 MB and 16384 MB")
	}

	// Get server
	server, err := s.repo.FindByID(serverID)
	if err != nil {
		return fmt.Errorf("failed to get server: %w", err)
	}

	oldRAMMB := server.RAMMb

	// No change needed
	if oldRAMMB == newRAMMB {
		return fmt.Errorf("server already has %d MB RAM", newRAMMB)
	}

	// Store original status to restore later
	wasRunning := (server.Status == models.StatusRunning)

	log.Printf("[RAM-UPGRADE] Starting RAM upgrade for server %s: %d MB -> %d MB (was running: %v)",
		serverID, oldRAMMB, newRAMMB, wasRunning)

	// STEP 1: Stop server if running
	if wasRunning {
		log.Printf("[RAM-UPGRADE] Stopping server %s for RAM upgrade", serverID)
		if err := s.StopServer(serverID, "RAM upgrade"); err != nil {
			return fmt.Errorf("failed to stop server for RAM upgrade: %w", err)
		}
	}

	// STEP 2: Update RAM allocation atomically
	// MULTI-NODE: Use the node ID stored in the database
	// Get nodeID from database (fallback to "local-node" for backward compatibility)
	nodeID := server.NodeID
	if nodeID == "" {
		nodeID = "local-node" // Fallback for legacy servers
	}

	// Release old allocation
	if s.conductor != nil {
		log.Printf("[RAM-UPGRADE] Releasing old RAM allocation: %d MB on node %s", oldRAMMB, nodeID)
		s.conductor.ReleaseRAMOnNode(nodeID, oldRAMMB)

		// Reserve new allocation
		log.Printf("[RAM-UPGRADE] Reserving new RAM allocation: %d MB on node %s", newRAMMB, nodeID)
		if !s.conductor.AtomicAllocateRAMOnNode(nodeID, newRAMMB) {
			// Failed to allocate - rollback
			log.Printf("[RAM-UPGRADE] Failed to allocate new RAM on node %s - rolling back to %d MB", nodeID, oldRAMMB)
			s.conductor.AtomicAllocateRAMOnNode(nodeID, oldRAMMB) // Re-allocate old amount

			// Restart server if it was running
			if wasRunning {
				go s.StartServer(serverID)
			}

			return fmt.Errorf("insufficient capacity on node %s for RAM upgrade (required: %d MB)", nodeID, newRAMMB)
		}
	}

	// STEP 3: Update database
	server.RAMMb = newRAMMB
	if err := s.repo.Update(server); err != nil {
		// Rollback RAM allocation
		if s.conductor != nil {
			s.conductor.ReleaseRAMOnNode(nodeID, newRAMMB)
			s.conductor.AtomicAllocateRAMOnNode(nodeID, oldRAMMB)
		}
		return fmt.Errorf("failed to update server in database: %w", err)
	}

	log.Printf("[RAM-UPGRADE] Database updated successfully for server %s", serverID)

	// STEP 4: Update container memory limit if container exists
	if server.ContainerID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		log.Printf("[RAM-UPGRADE] Updating container memory limit for %s", server.ContainerID[:12])
		if err := s.dockerService.UpdateContainerMemory(ctx, server.ContainerID, newRAMMB); err != nil {
			log.Printf("[RAM-UPGRADE] Warning: Failed to update container memory limit: %v", err)
			// Don't fail the upgrade - container will get new limits on next start
		}
	}

	// STEP 5: Restart server if it was running
	if wasRunning {
		log.Printf("[RAM-UPGRADE] Restarting server %s with new RAM allocation", serverID)
		if err := s.StartServer(serverID); err != nil {
			log.Printf("[RAM-UPGRADE] Warning: Failed to restart server after RAM upgrade: %v", err)
			// Don't rollback - upgrade succeeded, just restart failed
			return fmt.Errorf("RAM upgrade succeeded but failed to restart server: %w", err)
		}
	}

	log.Printf("[RAM-UPGRADE] RAM upgrade completed successfully for server %s: %d MB -> %d MB",
		serverID, oldRAMMB, newRAMMB)

	// Broadcast update via WebSocket
	if s.wsHub != nil {
		s.wsHub.Broadcast("server_updated", map[string]interface{}{
			"server_id": serverID,
			"ram_mb":    newRAMMB,
			"status":    server.Status,
		})
	}

	return nil
}

// ServerConnectionInfo holds the connection information for a running server
type ServerConnectionInfo struct {
	ServerID         string `json:"server_id"`
	Name             string `json:"name"`
	Status           string `json:"status"`
	IPAddress        string `json:"ip_address,omitempty"`
	Port             int    `json:"port"`
	ConnectionString string `json:"connection_string,omitempty"` // "IP:Port" - only for running servers
	NodeID           string `json:"node_id,omitempty"`
	MinecraftVersion string `json:"minecraft_version"`
	ServerType       string `json:"server_type"`
	RAMMb            int    `json:"ram_mb"`
}

// GetServerConnectionInfo returns the connection information for a server
// This is used by clients to determine how to connect to a Minecraft server
func (s *MinecraftService) GetServerConnectionInfo(serverID string) (*ServerConnectionInfo, error) {
	// Get server from database
	server, err := s.repo.FindByID(serverID)
	if err != nil {
		return nil, fmt.Errorf("server not found: %w", err)
	}

	// Build basic info
	info := &ServerConnectionInfo{
		ServerID:         server.ID,
		Name:             server.Name,
		Status:           string(server.Status),
		Port:             server.Port,
		MinecraftVersion: server.MinecraftVersion,
		ServerType:       string(server.ServerType),
		RAMMb:            server.RAMMb,
	}

	// Only add connection info for running servers
	if server.Status != models.StatusRunning {
		return info, nil // Return partial info (no IP/connection string)
	}

	// Get node info from Conductor
	if server.NodeID == "" {
		return nil, fmt.Errorf("server is running but has no node assigned (invalid state)")
	}

	info.NodeID = server.NodeID

	// Get node IP address
	remoteNode, err := s.conductor.GetRemoteNode(server.NodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node info: %w", err)
	}

	// Add connection details
	info.IPAddress = remoteNode.IPAddress
	info.ConnectionString = fmt.Sprintf("%s:%d", remoteNode.IPAddress, server.Port)

	return info, nil
}

// isLocalNode checks if a node ID represents the local Docker daemon
// Returns true if nodeID is "local-node" or empty (backward compatibility)
func (s *MinecraftService) isLocalNode(nodeID string) bool {
	return nodeID == "" || nodeID == "local-node"
}

// sendShutdownWarning sends a graceful shutdown warning to players via RCON
// FIX SERVER-8: Give players time to save and disconnect before server stops
func (s *MinecraftService) sendShutdownWarning(server *models.MinecraftServer) {
	// Get node info to determine RCON address
	var rconHost string
	nodeID := server.NodeID
	if nodeID == "" || nodeID == "local-node" {
		rconHost = "localhost"
	} else if s.conductor != nil {
		remoteNode, err := s.conductor.GetRemoteNode(nodeID)
		if err != nil {
			logger.Warn("SHUTDOWN: Cannot send warning - failed to get node info", map[string]interface{}{
				"server_id": server.ID,
				"node_id":   nodeID,
				"error":     err.Error(),
			})
			return
		}
		rconHost = remoteNode.IPAddress
	} else {
		return
	}

	// Connect to RCON
	client, err := rcon.NewClient(rconHost, server.RCONPort, server.RCONPassword)
	if err != nil {
		logger.Warn("SHUTDOWN: Cannot send warning - RCON connection failed", map[string]interface{}{
			"server_id": server.ID,
			"error":     err.Error(),
		})
		return
	}
	defer client.Close()

	// Send shutdown warnings
	warnings := []struct {
		message string
		delay   time.Duration
	}{
		{"Server shutting down in 10 seconds. Please disconnect!", 0},
		{"Server shutting down in 5 seconds!", 5 * time.Second},
		{"Server shutting down NOW!", 9 * time.Second},
	}

	for _, warning := range warnings {
		if warning.delay > 0 {
			time.Sleep(warning.delay)
		}

		command := fmt.Sprintf("say %s", warning.message)
		_, err := client.SendCommand(command)
		if err != nil {
			logger.Warn("SHUTDOWN: Failed to send warning via RCON", map[string]interface{}{
				"server_id": server.ID,
				"message":   warning.message,
				"error":     err.Error(),
			})
			return
		}

		logger.Info("SHUTDOWN: Warning sent to players", map[string]interface{}{
			"server_id": server.ID,
			"message":   warning.message,
		})
	}

	// Wait 1 more second for final message to be displayed
	time.Sleep(1 * time.Second)
}
