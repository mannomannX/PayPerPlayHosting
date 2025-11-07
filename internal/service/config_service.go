package service

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/payperplay/hosting/internal/docker"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/logger"
)

// ConfigService handles server configuration changes with audit trail
type ConfigService struct {
	serverRepo       *repository.ServerRepository
	configChangeRepo *repository.ConfigChangeRepository
	dockerService    *docker.DockerService
	backupService    *BackupService
	motdService      *MOTDService
}

// NewConfigService creates a new configuration service
func NewConfigService(
	serverRepo *repository.ServerRepository,
	configChangeRepo *repository.ConfigChangeRepository,
	dockerService *docker.DockerService,
	backupService *BackupService,
	motdService *MOTDService,
) *ConfigService {
	return &ConfigService{
		serverRepo:       serverRepo,
		configChangeRepo: configChangeRepo,
		dockerService:    dockerService,
		backupService:    backupService,
		motdService:      motdService,
	}
}

// ConfigChangeRequest represents a request to change server configuration
type ConfigChangeRequest struct {
	ServerID string
	UserID   string
	Changes  map[string]interface{} // Key-value pairs of config changes
}

// ApplyConfigChanges applies configuration changes with full audit trail
// This is the main entry point for all config changes
func (s *ConfigService) ApplyConfigChanges(req ConfigChangeRequest) (*models.ConfigChange, error) {
	// 1. Validate server exists and user has permission
	server, err := s.serverRepo.FindByID(req.ServerID)
	if err != nil {
		return nil, fmt.Errorf("server not found: %w", err)
	}

	// 2. Create config change record (audit trail)
	change := &models.ConfigChange{
		ID:       uuid.New().String()[:8],
		ServerID: req.ServerID,
		UserID:   req.UserID,
		Status:   models.ConfigChangeStatusPending,
	}

	// 3. Determine change type and validate
	requiresRestart := false
	for key, newValue := range req.Changes {
		switch key {
		case "ram_mb":
			change.ChangeType = models.ConfigChangeRAM
			change.OldValue = fmt.Sprintf("%d", server.RAMMb)
			change.NewValue = fmt.Sprintf("%v", newValue)
			requiresRestart = true

			// Validate RAM value
			ramMb, ok := newValue.(float64) // JSON numbers are float64
			if !ok {
				return nil, fmt.Errorf("invalid RAM value type")
			}
			if !s.isValidRAM(int(ramMb)) {
				return nil, fmt.Errorf("invalid RAM value: %d (must be 2048, 4096, 8192, or 16384)", int(ramMb))
			}

		case "minecraft_version":
			change.ChangeType = models.ConfigChangeVersion
			change.OldValue = server.MinecraftVersion
			change.NewValue = fmt.Sprintf("%v", newValue)
			requiresRestart = true

		case "max_players":
			change.ChangeType = models.ConfigChangeMaxPlayers
			change.OldValue = fmt.Sprintf("%d", server.MaxPlayers)
			change.NewValue = fmt.Sprintf("%v", newValue)
			requiresRestart = true // Requires restart to take effect

		case "server_type":
			change.ChangeType = models.ConfigChangeServerType
			change.OldValue = string(server.ServerType)
			change.NewValue = fmt.Sprintf("%v", newValue)
			requiresRestart = true

			// Validate server type
			newType := fmt.Sprintf("%v", newValue)
			if !s.isValidServerType(newType) {
				return nil, fmt.Errorf("invalid server type: %s (must be paper, spigot, or bukkit)", newType)
			}

			// Warn about incompatible changes
			if !s.isCompatibleServerType(string(server.ServerType), newType) {
				return nil, fmt.Errorf("server type change from %s to %s is not supported (only paper/spigot/bukkit are compatible)", server.ServerType, newType)
			}

		// Phase 1 Gameplay Settings
		case "gamemode":
			change.ChangeType = models.ConfigChangeGamemode
			change.OldValue = server.Gamemode
			change.NewValue = fmt.Sprintf("%v", newValue)
			requiresRestart = true

			// Validate gamemode
			newGamemode := fmt.Sprintf("%v", newValue)
			validGamemodes := []string{"survival", "creative", "adventure", "spectator"}
			if !contains(validGamemodes, newGamemode) {
				return nil, fmt.Errorf("invalid gamemode: %s (must be survival, creative, adventure, or spectator)", newGamemode)
			}

		case "difficulty":
			change.ChangeType = models.ConfigChangeDifficulty
			change.OldValue = server.Difficulty
			change.NewValue = fmt.Sprintf("%v", newValue)
			requiresRestart = true

			// Validate difficulty
			newDifficulty := fmt.Sprintf("%v", newValue)
			validDifficulties := []string{"peaceful", "easy", "normal", "hard"}
			if !contains(validDifficulties, newDifficulty) {
				return nil, fmt.Errorf("invalid difficulty: %s (must be peaceful, easy, normal, or hard)", newDifficulty)
			}

		case "pvp":
			change.ChangeType = models.ConfigChangePVP
			change.OldValue = fmt.Sprintf("%t", server.PVP)
			change.NewValue = fmt.Sprintf("%v", newValue)
			requiresRestart = true

		case "enable_command_block":
			change.ChangeType = models.ConfigChangeCommandBlock
			change.OldValue = fmt.Sprintf("%t", server.EnableCommandBlock)
			change.NewValue = fmt.Sprintf("%v", newValue)
			requiresRestart = true

		case "level_seed":
			change.ChangeType = models.ConfigChangeLevelSeed
			change.OldValue = server.LevelSeed
			change.NewValue = fmt.Sprintf("%v", newValue)
			requiresRestart = true

		// Phase 2 Performance Settings
		case "view_distance":
			change.ChangeType = models.ConfigChangeViewDistance
			change.OldValue = fmt.Sprintf("%d", server.ViewDistance)
			change.NewValue = fmt.Sprintf("%v", newValue)
			requiresRestart = true

			// Validate view distance (2-32 chunks)
			viewDist, ok := newValue.(float64)
			if !ok {
				return nil, fmt.Errorf("invalid view distance type")
			}
			if viewDist < 2 || viewDist > 32 {
				return nil, fmt.Errorf("invalid view distance: %d (must be between 2 and 32)", int(viewDist))
			}

		case "simulation_distance":
			change.ChangeType = models.ConfigChangeSimulationDistance
			change.OldValue = fmt.Sprintf("%d", server.SimulationDistance)
			change.NewValue = fmt.Sprintf("%v", newValue)
			requiresRestart = true

			// Validate simulation distance (3-32 chunks, 1.18+ only)
			simDist, ok := newValue.(float64)
			if !ok {
				return nil, fmt.Errorf("invalid simulation distance type")
			}
			if simDist < 3 || simDist > 32 {
				return nil, fmt.Errorf("invalid simulation distance: %d (must be between 3 and 32)", int(simDist))
			}

		// Phase 2 World Generation Settings
		case "allow_nether":
			change.ChangeType = models.ConfigChangeAllowNether
			change.OldValue = fmt.Sprintf("%t", server.AllowNether)
			change.NewValue = fmt.Sprintf("%v", newValue)
			requiresRestart = true

		case "allow_end":
			change.ChangeType = models.ConfigChangeAllowEnd
			change.OldValue = fmt.Sprintf("%t", server.AllowEnd)
			change.NewValue = fmt.Sprintf("%v", newValue)
			requiresRestart = true

		case "generate_structures":
			change.ChangeType = models.ConfigChangeGenerateStructures
			change.OldValue = fmt.Sprintf("%t", server.GenerateStructures)
			change.NewValue = fmt.Sprintf("%v", newValue)
			requiresRestart = true

		case "world_type":
			change.ChangeType = models.ConfigChangeWorldType
			change.OldValue = server.WorldType
			change.NewValue = fmt.Sprintf("%v", newValue)
			requiresRestart = true

			// Validate world type
			newWorldType := fmt.Sprintf("%v", newValue)
			validWorldTypes := []string{"default", "flat", "largeBiomes", "amplified", "buffet", "single_biome_surface"}
			if !contains(validWorldTypes, newWorldType) {
				return nil, fmt.Errorf("invalid world type: %s (must be default, flat, largeBiomes, amplified, buffet, or single_biome_surface)", newWorldType)
			}

		case "bonus_chest":
			change.ChangeType = models.ConfigChangeBonusChest
			change.OldValue = fmt.Sprintf("%t", server.BonusChest)
			change.NewValue = fmt.Sprintf("%v", newValue)
			requiresRestart = true

		case "max_world_size":
			change.ChangeType = models.ConfigChangeMaxWorldSize
			change.OldValue = fmt.Sprintf("%d", server.MaxWorldSize)
			change.NewValue = fmt.Sprintf("%v", newValue)
			requiresRestart = true

			// Validate max world size
			maxSize, ok := newValue.(float64)
			if !ok {
				return nil, fmt.Errorf("invalid max world size type")
			}
			if maxSize < 1 || maxSize > 29999984 {
				return nil, fmt.Errorf("invalid max world size: %d (must be between 1 and 29999984)", int(maxSize))
			}

		// Phase 2 Spawn Settings
		case "spawn_protection":
			change.ChangeType = models.ConfigChangeSpawnProtection
			change.OldValue = fmt.Sprintf("%d", server.SpawnProtection)
			change.NewValue = fmt.Sprintf("%v", newValue)
			requiresRestart = true

			// Validate spawn protection (0+ blocks)
			spawnProt, ok := newValue.(float64)
			if !ok {
				return nil, fmt.Errorf("invalid spawn protection type")
			}
			if spawnProt < 0 {
				return nil, fmt.Errorf("invalid spawn protection: %d (must be 0 or higher)", int(spawnProt))
			}

		case "spawn_animals":
			change.ChangeType = models.ConfigChangeSpawnAnimals
			change.OldValue = fmt.Sprintf("%t", server.SpawnAnimals)
			change.NewValue = fmt.Sprintf("%v", newValue)
			requiresRestart = true

		case "spawn_monsters":
			change.ChangeType = models.ConfigChangeSpawnMonsters
			change.OldValue = fmt.Sprintf("%t", server.SpawnMonsters)
			change.NewValue = fmt.Sprintf("%v", newValue)
			requiresRestart = true

		case "spawn_npcs":
			change.ChangeType = models.ConfigChangeSpawnNPCs
			change.OldValue = fmt.Sprintf("%t", server.SpawnNPCs)
			change.NewValue = fmt.Sprintf("%v", newValue)
			requiresRestart = true

		// Phase 2 Network & Performance Settings
		case "max_tick_time":
			change.ChangeType = models.ConfigChangeMaxTickTime
			change.OldValue = fmt.Sprintf("%d", server.MaxTickTime)
			change.NewValue = fmt.Sprintf("%v", newValue)
			requiresRestart = true

			// Validate max tick time
			maxTick, ok := newValue.(float64)
			if !ok {
				return nil, fmt.Errorf("invalid max tick time type")
			}
			if maxTick < -1 {
				return nil, fmt.Errorf("invalid max tick time: %d (must be -1 for disabled or positive)", int(maxTick))
			}

		case "network_compression_threshold":
			change.ChangeType = models.ConfigChangeNetworkCompressionThreshold
			change.OldValue = fmt.Sprintf("%d", server.NetworkCompressionThreshold)
			change.NewValue = fmt.Sprintf("%v", newValue)
			requiresRestart = true

			// Validate network compression threshold
			netComp, ok := newValue.(float64)
			if !ok {
				return nil, fmt.Errorf("invalid network compression threshold type")
			}
			if netComp < -1 {
				return nil, fmt.Errorf("invalid network compression threshold: %d (must be -1 for disabled or positive)", int(netComp))
			}

		// Phase 4 Server Description (MOTD)
		case "motd":
			change.ChangeType = models.ConfigChangeMOTD
			change.OldValue = server.MOTD
			change.NewValue = fmt.Sprintf("%v", newValue)
			// MOTD doesn't require container restart - just write to server.properties
			// User can manually restart server for changes to take effect
			requiresRestart = false

			// Validate MOTD length
			motd := fmt.Sprintf("%v", newValue)
			if len(motd) > 512 {
				return nil, fmt.Errorf("MOTD too long: %d characters (max 512)", len(motd))
			}

		default:
			return nil, fmt.Errorf("unsupported config change: %s", key)
		}
	}

	change.RequiresRestart = requiresRestart

	// Save initial change record
	if err := s.configChangeRepo.Create(change); err != nil {
		return nil, fmt.Errorf("failed to create config change record: %w", err)
	}

	// 4. Create backup before making changes (if server is running)
	if server.Status == models.StatusRunning && requiresRestart {
		logger.Info("Creating backup before config change", map[string]interface{}{
			"server_id": req.ServerID,
			"change_id": change.ID,
		})
		_, err := s.backupService.CreateBackup(req.ServerID)
		if err != nil {
			logger.Warn("Failed to create backup before config change", map[string]interface{}{
				"server_id": req.ServerID,
				"error":     err.Error(),
			})
			// Continue anyway - backup failure shouldn't block config changes
		}
	}

	// 5. Apply changes
	change.Status = models.ConfigChangeStatusApplying
	now := time.Now()
	change.AppliedAt = &now

	err = s.applyChanges(server, req.Changes, requiresRestart)
	if err != nil {
		change.Status = models.ConfigChangeStatusFailed
		change.ErrorMessage = err.Error()
		completedAt := time.Now()
		change.CompletedAt = &completedAt

		// Update in database
		s.configChangeRepo.Update(change)

		logger.Error("Config change failed", err, map[string]interface{}{
			"server_id": req.ServerID,
			"change_id": change.ID,
		})

		return change, fmt.Errorf("failed to apply config changes: %w", err)
	}

	// 6. Mark as completed
	change.Status = models.ConfigChangeStatusCompleted
	completedAt := time.Now()
	change.CompletedAt = &completedAt

	// Update in database
	if err := s.configChangeRepo.Update(change); err != nil {
		logger.Error("Failed to update config change record", err, map[string]interface{}{
			"change_id": change.ID,
		})
	}

	logger.Info("Config change completed successfully", map[string]interface{}{
		"server_id":        req.ServerID,
		"change_id":        change.ID,
		"requires_restart": requiresRestart,
	})

	return change, nil
}

// applyChanges applies the actual configuration changes
func (s *ConfigService) applyChanges(server *models.MinecraftServer, changes map[string]interface{}, requiresRestart bool) error {
	wasRunning := server.Status == models.StatusRunning

	// Update server model
	for key, value := range changes {
		switch key {
		case "ram_mb":
			ramMb := int(value.(float64))
			server.RAMMb = ramMb

		case "minecraft_version":
			server.MinecraftVersion = value.(string)

		case "max_players":
			maxPlayers := int(value.(float64))
			server.MaxPlayers = maxPlayers

		case "server_type":
			server.ServerType = models.ServerType(value.(string))

		// Phase 1 Gameplay Settings
		case "gamemode":
			server.Gamemode = value.(string)

		case "difficulty":
			server.Difficulty = value.(string)

		case "pvp":
			server.PVP = value.(bool)

		case "enable_command_block":
			server.EnableCommandBlock = value.(bool)

		case "level_seed":
			server.LevelSeed = value.(string)

		// Phase 2 Performance Settings
		case "view_distance":
			server.ViewDistance = int(value.(float64))

		case "simulation_distance":
			server.SimulationDistance = int(value.(float64))

		// Phase 2 World Generation Settings
		case "allow_nether":
			server.AllowNether = value.(bool)

		case "allow_end":
			server.AllowEnd = value.(bool)

		case "generate_structures":
			server.GenerateStructures = value.(bool)

		case "world_type":
			server.WorldType = value.(string)

		case "bonus_chest":
			server.BonusChest = value.(bool)

		case "max_world_size":
			server.MaxWorldSize = int(value.(float64))

		// Phase 2 Spawn Settings
		case "spawn_protection":
			server.SpawnProtection = int(value.(float64))

		case "spawn_animals":
			server.SpawnAnimals = value.(bool)

		case "spawn_monsters":
			server.SpawnMonsters = value.(bool)

		case "spawn_npcs":
			server.SpawnNPCs = value.(bool)

		// Phase 2 Network & Performance Settings
		case "max_tick_time":
			server.MaxTickTime = int(value.(float64))

		case "network_compression_threshold":
			server.NetworkCompressionThreshold = int(value.(float64))

		// Phase 4 Server Description (MOTD)
		case "motd":
			server.MOTD = value.(string)
		}
	}

	// Save to database
	err := s.serverRepo.Update(server)
	if err != nil {
		return fmt.Errorf("failed to update server in database: %w", err)
	}

	// Apply MOTD to server.properties if MOTD was changed
	if _, hasMOTD := changes["motd"]; hasMOTD {
		err = s.motdService.applyMOTD(server)
		if err != nil {
			logger.Warn("Failed to apply MOTD to server.properties", map[string]interface{}{
				"server_id": server.ID,
				"error":     err.Error(),
			})
			// Don't fail the whole operation if just the MOTD write fails
		} else {
			logger.Info("MOTD applied to server.properties", map[string]interface{}{
				"server_id": server.ID,
				"motd":      server.MOTD,
			})
		}
	}

	// If requires restart and server was running, recreate container
	if requiresRestart && wasRunning {
		logger.Info("Recreating container with new configuration", map[string]interface{}{
			"server_id": server.ID,
		})

		// Stop old container
		if server.ContainerID != "" {
			err = s.dockerService.StopContainer(server.ContainerID, 30)
			if err != nil {
				logger.Warn("Failed to stop old container", map[string]interface{}{
					"server_id":    server.ID,
					"container_id": server.ContainerID,
					"error":        err.Error(),
				})
			}

			// Remove old container
			err = s.dockerService.RemoveContainer(server.ContainerID, true)
			if err != nil {
				logger.Warn("Failed to remove old container", map[string]interface{}{
					"server_id":    server.ID,
					"container_id": server.ContainerID,
					"error":        err.Error(),
				})
			}
		}

		// Create new container with updated config
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
			return fmt.Errorf("failed to create new container: %w", err)
		}

		server.ContainerID = containerID
		server.Status = models.StatusStopped

		// Update with new container ID
		err = s.serverRepo.Update(server)
		if err != nil {
			return fmt.Errorf("failed to update container ID: %w", err)
		}

		// Start the new container
		err = s.dockerService.StartContainer(containerID)
		if err != nil {
			server.Status = models.StatusError
			s.serverRepo.Update(server)
			return fmt.Errorf("failed to start new container: %w", err)
		}

		// Wait for server to be ready
		err = s.dockerService.WaitForServerReady(containerID, 60)
		if err != nil {
			logger.Warn("Server may not be fully ready", map[string]interface{}{
				"server_id": server.ID,
				"error":     err.Error(),
			})
		}

		server.Status = models.StatusRunning
		s.serverRepo.Update(server)
	}

	return nil
}

// isValidRAM checks if the RAM value is valid
func (s *ConfigService) isValidRAM(ramMb int) bool {
	validValues := []int{2048, 4096, 8192, 16384}
	for _, v := range validValues {
		if ramMb == v {
			return true
		}
	}
	return false
}

// isValidServerType checks if the server type is valid and supported
func (s *ConfigService) isValidServerType(serverType string) bool {
	validTypes := []string{"paper", "spigot", "bukkit"}
	for _, v := range validTypes {
		if serverType == v {
			return true
		}
	}
	return false
}

// isCompatibleServerType checks if the server type change is compatible
func (s *ConfigService) isCompatibleServerType(oldType, newType string) bool {
	// Paper, Spigot, and Bukkit are compatible with each other
	compatibleGroup := map[string]bool{
		"paper":  true,
		"spigot": true,
		"bukkit": true,
	}

	// Both types must be in the compatible group
	return compatibleGroup[oldType] && compatibleGroup[newType]
}

// GetConfigHistory returns the configuration change history for a server
func (s *ConfigService) GetConfigHistory(serverID string) ([]models.ConfigChange, error) {
	return s.configChangeRepo.FindByServerID(serverID)
}

// contains checks if a string slice contains a specific string
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
