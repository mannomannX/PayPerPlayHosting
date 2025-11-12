package docker

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
	// Phase 1 Parameters
	maxPlayers int,
	gamemode string,
	difficulty string,
	pvp bool,
	enableCommandBlock bool,
	levelSeed string,
	// Phase 2 Parameters - Performance
	viewDistance int,
	simulationDistance int,
	// Phase 2 Parameters - World Generation
	allowNether bool,
	allowEnd bool,
	generateStructures bool,
	worldType string,
	bonusChest bool,
	maxWorldSize int,
	// Phase 2 Parameters - Spawn Settings
	spawnProtection int,
	spawnAnimals bool,
	spawnMonsters bool,
	spawnNPCs bool,
	// Phase 2 Parameters - Network & Performance
	maxTickTime int,
	networkCompressionThreshold int,
	// Phase 4 Parameters - Server Description
	motd string,
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
		fmt.Sprintf("MAX_PLAYERS=%d", maxPlayers),
		"ONLINE_MODE=TRUE",
		"SERVER_NAME=PayPerPlay Server",
		// Enable RCON for monitoring
		"ENABLE_RCON=true",
		"RCON_PASSWORD=minecraft",
		"RCON_PORT=25575",

		// === Phase 1 - Gameplay Settings ===
		fmt.Sprintf("MODE=%s", gamemode),
		fmt.Sprintf("DIFFICULTY=%s", difficulty),
		fmt.Sprintf("PVP=%t", pvp),
		fmt.Sprintf("ENABLE_COMMAND_BLOCK=%t", enableCommandBlock),

		// === Phase 2 - Performance Settings ===
		fmt.Sprintf("VIEW_DISTANCE=%d", viewDistance),
		fmt.Sprintf("SIMULATION_DISTANCE=%d", simulationDistance),

		// === Phase 2 - World Generation Settings ===
		fmt.Sprintf("ALLOW_NETHER=%t", allowNether),
		fmt.Sprintf("GENERATE_STRUCTURES=%t", generateStructures),
		fmt.Sprintf("LEVEL_TYPE=%s", worldType),
		fmt.Sprintf("ENABLE_BONUS_CHEST=%t", bonusChest),
		fmt.Sprintf("MAX_WORLD_SIZE=%d", maxWorldSize),

		// === Phase 2 - Spawn Settings ===
		fmt.Sprintf("SPAWN_PROTECTION=%d", spawnProtection),
		fmt.Sprintf("SPAWN_ANIMALS=%t", spawnAnimals),
		fmt.Sprintf("SPAWN_MONSTERS=%t", spawnMonsters),
		fmt.Sprintf("SPAWN_NPCS=%t", spawnNPCs),

		// === Phase 2 - Network & Performance Settings ===
		fmt.Sprintf("MAX_TICK_TIME=%d", maxTickTime),
		fmt.Sprintf("NETWORK_COMPRESSION_THRESHOLD=%d", networkCompressionThreshold),

		// === Phase 4 - Server Description ===
		fmt.Sprintf("MOTD=%s", motd),
	}

	// Add SEED only if provided (empty = random)
	if levelSeed != "" {
		env = append(env, fmt.Sprintf("SEED=%s", levelSeed))
	}

	// Note: Allow End is set via server.properties, not ENV
	// We'll need to handle this after container creation

	// Note: RCON port is NOT mapped to host for security
	// Commands are executed via docker exec instead

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
				// RCON port (25575) is NOT mapped - we use docker exec for commands
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

// WaitForServerReady waits for the Minecraft server to be ready by monitoring logs
func (d *DockerService) WaitForServerReady(containerID string, timeoutSeconds int) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	// Stream container logs
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
	}

	reader, err := d.client.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return fmt.Errorf("failed to get container logs: %w", err)
	}
	defer reader.Close()

	// Read logs line by line and look for "Done ("
	buf := make([]byte, 8192)
	logBuffer := ""

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for server to be ready")
		default:
			n, err := reader.Read(buf)
			if err != nil {
				if err == io.EOF {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				return fmt.Errorf("error reading logs: %w", err)
			}

			// Skip Docker log header (first 8 bytes of each frame)
			// Docker log format: [8 byte header][message]
			if n > 8 {
				logBuffer += string(buf[8:n])
			}

			// Check if server is ready
			if containsReadyMarker(logBuffer) {
				log.Printf("Minecraft server %s is ready!", containerID[:12])
				return nil
			}
		}
	}
}

// containsReadyMarker checks if the log contains the server ready marker
func containsReadyMarker(logText string) bool {
	// Look for the "Done (X.XXXs)!" message that indicates server is ready
	return strings.Contains(logText, "Done (") && strings.Contains(logText, "s)!")
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

// StreamContainerLogs streams container logs to a channel
// Returns a channel that receives log lines and a cancel function
func (d *DockerService) StreamContainerLogs(containerID string) (<-chan string, context.CancelFunc, error) {
	ctx, cancel := context.WithCancel(context.Background())

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "100", // Show last 100 lines initially
		Timestamps: false,
	}

	reader, err := d.client.ContainerLogs(ctx, containerID, options)
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("failed to get container logs: %w", err)
	}

	logChan := make(chan string, 100)

	// Start goroutine to read logs and send to channel
	go func() {
		defer close(logChan)
		defer reader.Close()

		buf := make([]byte, 8192)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				n, err := reader.Read(buf)
				if err != nil {
					if err != io.EOF {
						log.Printf("Error reading container logs: %v", err)
					}
					return
				}

				if n > 0 {
					// Skip Docker log header (first 8 bytes)
					// Docker multiplexes stdout/stderr with 8-byte headers
					logLine := string(buf[8:n])

					// Split by newlines and send each line
					lines := strings.Split(strings.TrimSpace(logLine), "\n")
					for _, line := range lines {
						if line != "" {
							select {
							case logChan <- line:
							case <-ctx.Done():
								return
							}
						}
					}
				}
			}
		}
	}()

	return logChan, cancel, nil
}

// ExecuteCommand executes a Minecraft command in a container
func (d *DockerService) ExecuteCommand(containerID, command string) (string, error) {
	ctx := context.Background()

	// Create exec instance with RCON command
	rconCommand := []string{"rcon-cli", command}
	execConfig := container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          rconCommand,
	}

	resp, err := d.client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	// Start exec and capture output
	attachResp, err := d.client.ContainerExecAttach(ctx, resp.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer attachResp.Close()

	// Read output
	output, err := io.ReadAll(attachResp.Reader)
	if err != nil {
		return "", fmt.Errorf("failed to read exec output: %w", err)
	}

	// Skip Docker stream header (8 bytes)
	if len(output) > 8 {
		output = output[8:]
	}

	return strings.TrimSpace(string(output)), nil
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

// ListRunningMinecraftContainers returns all currently running mc-* containers
// Used by Conductor to sync state after restarts
func (d *DockerService) ListRunningMinecraftContainers() ([]struct {
	ContainerID string
	ServerID    string
}, error) {
	ctx := context.Background()

	// List all containers with name prefix "mc-"
	containers, err := d.client.ContainerList(ctx, container.ListOptions{
		All: false, // Only running containers
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var result []struct {
		ContainerID string
		ServerID    string
	}

	for _, c := range containers {
		// Check if container name starts with "mc-"
		if len(c.Names) > 0 && strings.HasPrefix(c.Names[0], "/mc-") {
			// Extract server ID from container name (format: mc-{serverID})
			serverID := strings.TrimPrefix(c.Names[0], "/mc-")

			result = append(result, struct {
				ContainerID string
				ServerID    string
			}{
				ContainerID: c.ID,
				ServerID:    serverID,
			})
		}
	}

	return result, nil
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

// UpdateContainerMemory updates the memory limit of a running container
// This is used for RAM upgrades without full recreation
func (d *DockerService) UpdateContainerMemory(ctx context.Context, containerID string, ramMB int) error {
	// Calculate memory with 25% overhead for JVM
	memoryBytes := int64(float64(ramMB)*1.25) * 1024 * 1024

	// Prepare update configuration
	updateConfig := container.UpdateConfig{
		Resources: container.Resources{
			Memory: memoryBytes,
		},
	}

	// Update the container
	_, err := d.client.ContainerUpdate(ctx, containerID, updateConfig)
	if err != nil {
		return fmt.Errorf("failed to update container memory: %w", err)
	}

	log.Printf("[Docker] Updated container %s memory limit to %d MB (with overhead: %d bytes)",
		containerID[:12], ramMB, memoryBytes)
	return nil
}

// Close closes the Docker client
func (d *DockerService) Close() error {
	return d.client.Close()
}
