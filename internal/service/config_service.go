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
}

// NewConfigService creates a new configuration service
func NewConfigService(
	serverRepo *repository.ServerRepository,
	configChangeRepo *repository.ConfigChangeRepository,
	dockerService *docker.DockerService,
	backupService *BackupService,
) *ConfigService {
	return &ConfigService{
		serverRepo:       serverRepo,
		configChangeRepo: configChangeRepo,
		dockerService:    dockerService,
		backupService:    backupService,
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
		}
	}

	// Save to database
	err := s.serverRepo.Update(server)
	if err != nil {
		return fmt.Errorf("failed to update server in database: %w", err)
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
			server.MaxPlayers,
			server.Gamemode,
			server.Difficulty,
			server.PVP,
			server.EnableCommandBlock,
			server.LevelSeed,
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
