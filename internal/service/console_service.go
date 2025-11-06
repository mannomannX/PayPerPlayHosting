package service

import (
	"fmt"

	"github.com/payperplay/hosting/internal/docker"
	"github.com/payperplay/hosting/internal/repository"
)

type ConsoleService struct {
	repo          *repository.ServerRepository
	dockerService *docker.DockerService
}

func NewConsoleService(repo *repository.ServerRepository, dockerService *docker.DockerService) *ConsoleService {
	return &ConsoleService{
		repo:          repo,
		dockerService: dockerService,
	}
}

// StreamLogs streams container logs for a server
func (s *ConsoleService) StreamLogs(serverID string) (<-chan string, func(), error) {
	// Get server from database
	server, err := s.repo.FindByID(serverID)
	if err != nil {
		return nil, nil, fmt.Errorf("server not found: %w", err)
	}

	if server.ContainerID == "" {
		return nil, nil, fmt.Errorf("server has no container")
	}

	// Stream logs from Docker
	logChan, cancel, err := s.dockerService.StreamContainerLogs(server.ContainerID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to stream logs: %w", err)
	}

	return logChan, cancel, nil
}

// ExecuteCommand executes a command on the server via docker exec
func (s *ConsoleService) ExecuteCommand(serverID, command string) (string, error) {
	// Get server from database
	server, err := s.repo.FindByID(serverID)
	if err != nil {
		return "", fmt.Errorf("server not found: %w", err)
	}

	if server.ContainerID == "" {
		return "", fmt.Errorf("server has no container")
	}

	// Execute command via docker exec (uses rcon-cli inside container)
	response, err := s.dockerService.ExecuteCommand(server.ContainerID, command)
	if err != nil {
		return "", fmt.Errorf("failed to execute command: %w", err)
	}

	return response, nil
}
