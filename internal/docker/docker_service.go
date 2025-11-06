package docker

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/payperplay/hosting/pkg/config"
)

type DockerService struct {
	client     *client.Client
	cfg        *config.Config
	serversDir string
}

func NewDockerService(cfg *config.Config) (*DockerService, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	// Ensure servers directory exists
	serversDir, err := filepath.Abs(cfg.ServersBasePath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(serversDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create servers directory: %w", err)
	}

	return &DockerService{
		client:     cli,
		cfg:        cfg,
		serversDir: serversDir,
	}, nil
}

// CreateContainer creates a Docker container for a Minecraft server
func (d *DockerService) CreateContainer(
	serverID string,
	serverType string,
	minecraftVersion string,
	ramMB int,
	port int,
) (string, error) {
	ctx := context.Background()

	// Create server directory (inside container or on host)
	serverDir := filepath.Join(d.serversDir, serverID)
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create server directory: %w", err)
	}

	// Determine host path for Docker bind mount
	// If HostServersBasePath is set, use it (for when API runs in container)
	// Otherwise use the serverDir directly (for when API runs on host)
	hostPath := serverDir
	if d.cfg.HostServersBasePath != "" {
		hostPath = filepath.Join(d.cfg.HostServersBasePath, serverID)
	}

	// Determine Docker image (using itzg/minecraft-server)
	imageName := "itzg/minecraft-server:latest"

	// Pull image if not exists
	if err := d.ensureImage(ctx, imageName); err != nil {
		log.Printf("Warning: failed to pull image %s: %v", imageName, err)
	}

	// Container configuration
	containerName := fmt.Sprintf("mc-%s", serverID)
	portBinding := nat.PortBinding{
		HostIP:   "0.0.0.0",
		HostPort: strconv.Itoa(port),
	}

	// Environment variables for itzg/minecraft-server
	env := []string{
		"EULA=TRUE",
		fmt.Sprintf("TYPE=%s", d.getServerTypeEnv(serverType)),
		fmt.Sprintf("VERSION=%s", minecraftVersion),
		fmt.Sprintf("MEMORY=%dM", ramMB),
		"ONLINE_MODE=TRUE",
		"SERVER_NAME=PayPerPlay Server",
		// Enable RCON for monitoring
		"ENABLE_RCON=true",
		"RCON_PASSWORD=minecraft",
		"RCON_PORT=25575",
	}

	// Port bindings for both game and RCON
	rconPortBinding := nat.PortBinding{
		HostIP:   "127.0.0.1", // RCON only on localhost for security
		HostPort: "25575",
	}

	// Create container
	resp, err := d.client.ContainerCreate(
		ctx,
		&container.Config{
			Image: imageName,
			Env:   env,
			ExposedPorts: nat.PortSet{
				"25565/tcp": struct{}{},
				"25575/tcp": struct{}{}, // RCON port
			},
			Labels: map[string]string{
				"payperplay.server_id": serverID,
				"payperplay.type":      serverType,
				"payperplay.version":   minecraftVersion,
			},
		},
		&container.HostConfig{
			PortBindings: nat.PortMap{
				"25565/tcp": []nat.PortBinding{portBinding},
				"25575/tcp": []nat.PortBinding{rconPortBinding},
			},
			Binds: []string{
				fmt.Sprintf("%s:/data", hostPath),
			},
			RestartPolicy: container.RestartPolicy{
				Name: "no",
			},
			Resources: container.Resources{
				// Add 25% overhead for JVM native memory, threads, GC, etc.
				// This prevents OOM kills when Java heap is set to ramMB
				Memory: int64(float64(ramMB)*1.25) * 1024 * 1024, // MB to bytes
			},
		},
		nil,
		nil,
		containerName,
	)

	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	log.Printf("Created container %s for server %s", resp.ID[:12], serverID)
	return resp.ID, nil
}

// StartContainer starts a Docker container
func (d *DockerService) StartContainer(containerID string) error {
	ctx := context.Background()
	err := d.client.ContainerStart(ctx, containerID, container.StartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}
	log.Printf("Started container %s", containerID[:12])
	return nil
}

// StopContainer stops a Docker container gracefully
func (d *DockerService) StopContainer(containerID string, timeoutSeconds int) error {
	ctx := context.Background()
	timeout := timeoutSeconds
	err := d.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})
	if err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}
	log.Printf("Stopped container %s", containerID[:12])
	return nil
}

// RemoveContainer removes a Docker container
func (d *DockerService) RemoveContainer(containerID string, force bool) error {
	ctx := context.Background()
	err := d.client.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force: force,
	})
	if err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}
	log.Printf("Removed container %s", containerID[:12])
	return nil
}

// RemoveContainerByName removes a Docker container by name
func (d *DockerService) RemoveContainerByName(containerName string) error {
	ctx := context.Background()

	// Try to remove the container (force=true to handle any state)
	err := d.client.ContainerRemove(ctx, containerName, container.RemoveOptions{
		Force: true,
	})

	if err != nil {
		// Check if error is "not found" - that's okay
		if client.IsErrNotFound(err) {
			log.Printf("Container %s does not exist (already removed)", containerName)
			return nil
		}
		return fmt.Errorf("failed to remove container by name: %w", err)
	}

	log.Printf("Removed existing container: %s", containerName)
	return nil
}

// GetContainerStatus gets the status of a container
func (d *DockerService) GetContainerStatus(containerID string) (string, error) {
	ctx := context.Background()
	inspect, err := d.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", err
	}
	return inspect.State.Status, nil
}

// GetContainerLogs retrieves logs from a container
func (d *DockerService) GetContainerLogs(containerID string, tail string) (string, error) {
	ctx := context.Background()
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tail,
	}

	logs, err := d.client.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", err
	}
	defer logs.Close()

	content, err := io.ReadAll(logs)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// FindAvailablePort finds an available port in the configured range
func (d *DockerService) FindAvailablePort(usedPorts []int) (int, error) {
	usedPortsMap := make(map[int]bool)
	for _, port := range usedPorts {
		usedPortsMap[port] = true
	}

	for port := d.cfg.MCPortStart; port <= d.cfg.MCPortEnd; port++ {
		if !usedPortsMap[port] {
			return port, nil
		}
	}

	return 0, fmt.Errorf("no available ports in range %d-%d", d.cfg.MCPortStart, d.cfg.MCPortEnd)
}

// ensureImage pulls a Docker image if it doesn't exist locally
func (d *DockerService) ensureImage(ctx context.Context, imageName string) error {
	_, _, err := d.client.ImageInspectWithRaw(ctx, imageName)
	if err == nil {
		// Image already exists
		return nil
	}

	log.Printf("Pulling image %s...", imageName)
	reader, err := d.client.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	// Wait for pull to complete
	_, err = io.Copy(io.Discard, reader)
	return err
}

// getServerTypeEnv converts our server type to itzg/minecraft-server TYPE env
func (d *DockerService) getServerTypeEnv(serverType string) string {
	switch serverType {
	case "paper":
		return "PAPER"
	case "spigot":
		return "SPIGOT"
	case "forge":
		return "FORGE"
	case "fabric":
		return "FABRIC"
	case "purpur":
		return "PURPUR"
	case "vanilla":
		return "VANILLA"
	default:
		return "PAPER" // Default to Paper
	}
}

// GetClient returns the Docker client (needed for Velocity service)
func (d *DockerService) GetClient() *client.Client {
	return d.client
}

// Close closes the Docker client
func (d *DockerService) Close() error {
	return d.client.Close()
}
