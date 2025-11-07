package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/config"
	"github.com/payperplay/hosting/pkg/logger"
)

// PlayerListType represents different types of player lists
type PlayerListType string

const (
	ListTypeWhitelist PlayerListType = "whitelist"
	ListTypeOps       PlayerListType = "ops"
	ListTypeBanned    PlayerListType = "banned-players"
)

// PlayerEntry represents a player in any list
type PlayerEntry struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

// OpEntry represents an operator with level
type OpEntry struct {
	UUID  string `json:"uuid"`
	Name  string `json:"name"`
	Level int    `json:"level"`
}

// BannedEntry represents a banned player with metadata
type BannedEntry struct {
	UUID    string `json:"uuid"`
	Name    string `json:"name"`
	Created string `json:"created"`
	Source  string `json:"source"`
	Expires string `json:"expires"`
	Reason  string `json:"reason"`
}

// PlayerListService handles all player list operations (whitelist, ops, banned)
type PlayerListService struct {
	serverRepo     *repository.ServerRepository
	consoleService *ConsoleService
	config         *config.Config
}

// NewPlayerListService creates a new player list service
func NewPlayerListService(
	serverRepo *repository.ServerRepository,
	consoleService *ConsoleService,
	config *config.Config,
) *PlayerListService {
	return &PlayerListService{
		serverRepo:     serverRepo,
		consoleService: consoleService,
		config:         config,
	}
}

// GetList retrieves a player list for a server
func (s *PlayerListService) GetList(serverID string, listType PlayerListType) (interface{}, error) {
	server, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return nil, fmt.Errorf("server not found: %w", err)
	}

	filePath := s.getListFilePath(server.ID, listType)

	// Read JSON file
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet - return empty list
			return s.emptyListForType(listType), nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", listType, err)
	}

	// Parse based on list type
	switch listType {
	case ListTypeWhitelist:
		var list []PlayerEntry
		if err := json.Unmarshal(data, &list); err != nil {
			return nil, fmt.Errorf("failed to parse whitelist: %w", err)
		}
		return list, nil

	case ListTypeOps:
		var list []OpEntry
		if err := json.Unmarshal(data, &list); err != nil {
			return nil, fmt.Errorf("failed to parse ops list: %w", err)
		}
		return list, nil

	case ListTypeBanned:
		var list []BannedEntry
		if err := json.Unmarshal(data, &list); err != nil {
			return nil, fmt.Errorf("failed to parse banned list: %w", err)
		}
		return list, nil

	default:
		return nil, fmt.Errorf("unknown list type: %s", listType)
	}
}

// AddToList adds a player to a specific list
func (s *PlayerListService) AddToList(serverID, username string, listType PlayerListType) error {
	server, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Validate username
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	// If server is running, use RCON
	if server.Status == models.StatusRunning {
		return s.addViaRCON(serverID, username, listType)
	}

	// If server is stopped, edit JSON file directly
	return s.addToFileDirectly(server.ID, username, listType)
}

// RemoveFromList removes a player from a specific list
func (s *PlayerListService) RemoveFromList(serverID, username string, listType PlayerListType) error {
	server, err := s.serverRepo.FindByID(serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// If server is running, use RCON
	if server.Status == models.StatusRunning {
		return s.removeViaRCON(serverID, username, listType)
	}

	// If server is stopped, edit JSON file directly
	return s.removeFromFileDirectly(server.ID, username, listType)
}

// addViaRCON adds a player using RCON commands (server is running)
func (s *PlayerListService) addViaRCON(serverID, username string, listType PlayerListType) error {
	var command string

	switch listType {
	case ListTypeWhitelist:
		command = fmt.Sprintf("whitelist add %s", username)
	case ListTypeOps:
		command = fmt.Sprintf("op %s", username)
	case ListTypeBanned:
		command = fmt.Sprintf("ban %s Added via PayPerPlay", username)
	default:
		return fmt.Errorf("unknown list type: %s", listType)
	}

	// Execute via Console Service (docker exec with rcon-cli)
	_, err := s.consoleService.ExecuteCommand(serverID, command)
	if err != nil {
		return fmt.Errorf("failed to execute command: %w", err)
	}

	logger.Info("Player added to list via RCON", map[string]interface{}{
		"server_id": serverID,
		"username":  username,
		"list_type": listType,
	})

	return nil
}

// removeViaRCON removes a player using RCON commands (server is running)
func (s *PlayerListService) removeViaRCON(serverID, username string, listType PlayerListType) error {
	var command string

	switch listType {
	case ListTypeWhitelist:
		command = fmt.Sprintf("whitelist remove %s", username)
	case ListTypeOps:
		command = fmt.Sprintf("deop %s", username)
	case ListTypeBanned:
		command = fmt.Sprintf("pardon %s", username)
	default:
		return fmt.Errorf("unknown list type: %s", listType)
	}

	// Execute via Console Service (docker exec with rcon-cli)
	_, err := s.consoleService.ExecuteCommand(serverID, command)
	if err != nil {
		return fmt.Errorf("failed to execute command: %w", err)
	}

	logger.Info("Player removed from list via RCON", map[string]interface{}{
		"server_id": serverID,
		"username":  username,
		"list_type": listType,
	})

	return nil
}

// addToFileDirectly adds a player to JSON file (server is stopped)
func (s *PlayerListService) addToFileDirectly(serverID, username string, listType PlayerListType) error {
	filePath := s.getListFilePath(serverID, listType)

	// Read existing list
	currentList, err := s.GetList(serverID, listType)
	if err != nil {
		return err
	}

	// Add player based on type
	switch listType {
	case ListTypeWhitelist:
		list := currentList.([]PlayerEntry)
		// Check if already exists
		for _, entry := range list {
			if strings.EqualFold(entry.Name, username) {
				return nil // Already in list
			}
		}
		list = append(list, PlayerEntry{
			UUID: "",        // Will be resolved by Minecraft
			Name: username,
		})
		return s.writeJSONFile(filePath, list)

	case ListTypeOps:
		list := currentList.([]OpEntry)
		// Check if already exists
		for _, entry := range list {
			if strings.EqualFold(entry.Name, username) {
				return nil // Already in list
			}
		}
		list = append(list, OpEntry{
			UUID:  "",        // Will be resolved by Minecraft
			Name:  username,
			Level: 4,         // Full op permissions
		})
		return s.writeJSONFile(filePath, list)

	case ListTypeBanned:
		list := currentList.([]BannedEntry)
		// Check if already exists
		for _, entry := range list {
			if strings.EqualFold(entry.Name, username) {
				return nil // Already banned
			}
		}
		list = append(list, BannedEntry{
			UUID:    "",
			Name:    username,
			Created: "PayPerPlay",
			Source:  "PayPerPlay",
			Expires: "forever",
			Reason:  "Banned via PayPerPlay",
		})
		return s.writeJSONFile(filePath, list)

	default:
		return fmt.Errorf("unknown list type: %s", listType)
	}
}

// removeFromFileDirectly removes a player from JSON file (server is stopped)
func (s *PlayerListService) removeFromFileDirectly(serverID, username string, listType PlayerListType) error {
	filePath := s.getListFilePath(serverID, listType)

	// Read existing list
	currentList, err := s.GetList(serverID, listType)
	if err != nil {
		return err
	}

	// Remove player based on type
	switch listType {
	case ListTypeWhitelist:
		list := currentList.([]PlayerEntry)
		newList := []PlayerEntry{}
		for _, entry := range list {
			if !strings.EqualFold(entry.Name, username) {
				newList = append(newList, entry)
			}
		}
		return s.writeJSONFile(filePath, newList)

	case ListTypeOps:
		list := currentList.([]OpEntry)
		newList := []OpEntry{}
		for _, entry := range list {
			if !strings.EqualFold(entry.Name, username) {
				newList = append(newList, entry)
			}
		}
		return s.writeJSONFile(filePath, newList)

	case ListTypeBanned:
		list := currentList.([]BannedEntry)
		newList := []BannedEntry{}
		for _, entry := range list {
			if !strings.EqualFold(entry.Name, username) {
				newList = append(newList, entry)
			}
		}
		return s.writeJSONFile(filePath, newList)

	default:
		return fmt.Errorf("unknown list type: %s", listType)
	}
}

// getListFilePath returns the file path for a specific list type
func (s *PlayerListService) getListFilePath(serverID string, listType PlayerListType) string {
	return filepath.Join(s.config.ServersBasePath, serverID, string(listType)+".json")
}

// writeJSONFile writes data to a JSON file
func (s *PlayerListService) writeJSONFile(filePath string, data interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// emptyListForType returns an empty list of the correct type
func (s *PlayerListService) emptyListForType(listType PlayerListType) interface{} {
	switch listType {
	case ListTypeWhitelist:
		return []PlayerEntry{}
	case ListTypeOps:
		return []OpEntry{}
	case ListTypeBanned:
		return []BannedEntry{}
	default:
		return []PlayerEntry{}
	}
}
