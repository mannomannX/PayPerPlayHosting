package velocity

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/config"
	"github.com/payperplay/hosting/pkg/logger"
)

const (
	velocityImage      = "itzg/bungeecord:latest"
	velocityPort       = "25565"
	velocityInternalPort = "25577"
	velocityMemory     = "512m"
)

type VelocityService struct {
	client          *client.Client
	repo            *repository.ServerRepository
	cfg             *config.Config
	configGenerator *ConfigGenerator
	containerID     string
	configPath      string
	pluginsPath     string
}

func NewVelocityService(
	dockerClient *client.Client,
	repo *repository.ServerRepository,
	cfg *config.Config,
) (*VelocityService, error) {
	// Setup paths
	basePath := cfg.ServersBasePath
	configPath := filepath.Join(basePath, "velocity", "config", "velocity.toml")
	pluginsPath := filepath.Join(basePath, "velocity", "plugins")

	// Create config generator
	motd := fmt.Sprintf("§b%s §8| §7Pay-Per-Play Hosting", cfg.AppName)
	configGen := NewConfigGenerator(configPath, motd)

	return &VelocityService{
		client:          dockerClient,
		repo:            repo,
		cfg:             cfg,
		configGenerator: configGen,
		configPath:      configPath,
		pluginsPath:     pluginsPath,
	}, nil
}

// Start starts the Velocity proxy container
func (v *VelocityService) Start() error {
	// Generate initial config
	if err := v.RegenerateConfig(); err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}

	// Check if already running
	if v.IsRunning() {
		logger.Info("Velocity proxy already running", map[string]interface{}{
			"container_id": v.containerID,
		})
		return nil
	}

	ctx := context.Background()

	// Container configuration
	containerConfig := &container.Config{
		Image: velocityImage,
		Env: []string{
			"TYPE=VELOCITY",
			"MEMORY=" + velocityMemory,
			"INIT_MEMORY=" + velocityMemory,
			"MAX_MEMORY=" + velocityMemory,
		},
		ExposedPorts: nat.PortSet{
			nat.Port(velocityInternalPort + "/tcp"): {},
		},
	}

	// Host configuration
	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			nat.Port(velocityInternalPort + "/tcp"): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: velocityPort,
				},
			},
		},
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: filepath.Dir(v.configPath),
				Target: "/config",
			},
			{
				Type:   mount.TypeBind,
				Source: v.pluginsPath,
				Target: "/plugins",
			},
		},
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
	}

	// Create container
	resp, err := v.client.ContainerCreate(
		ctx,
		containerConfig,
		hostConfig,
		nil,
		nil,
		"payperplay-velocity-proxy",
	)
	if err != nil {
		return fmt.Errorf("failed to create Velocity container: %w", err)
	}

	v.containerID = resp.ID

	// Start container
	if err := v.client.ContainerStart(ctx, v.containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start Velocity container: %w", err)
	}

	logger.Info("Velocity proxy started", map[string]interface{}{
		"container_id": v.containerID,
		"port":         velocityPort,
	})

	return nil
}

// Stop stops the Velocity proxy container
func (v *VelocityService) Stop() error {
	if v.containerID == "" {
		return nil
	}

	ctx := context.Background()
	timeout := 30
	stopOptions := container.StopOptions{
		Timeout: &timeout,
	}

	if err := v.client.ContainerStop(ctx, v.containerID, stopOptions); err != nil {
		return fmt.Errorf("failed to stop Velocity container: %w", err)
	}

	logger.Info("Velocity proxy stopped", map[string]interface{}{
		"container_id": v.containerID,
	})

	v.containerID = ""
	return nil
}

// IsRunning checks if the Velocity proxy is running
func (v *VelocityService) IsRunning() bool {
	if v.containerID == "" {
		// Try to find existing container
		ctx := context.Background()
		containers, err := v.client.ContainerList(ctx, container.ListOptions{
			All: true,
		})
		if err != nil {
			return false
		}

		for _, c := range containers {
			for _, name := range c.Names {
				if name == "/payperplay-velocity-proxy" {
					v.containerID = c.ID
					return c.State == "running"
				}
			}
		}
		return false
	}

	ctx := context.Background()
	inspect, err := v.client.ContainerInspect(ctx, v.containerID)
	if err != nil {
		return false
	}

	return inspect.State.Running
}

// RegisterServer adds a server to Velocity configuration
func (v *VelocityService) RegisterServer(server *models.MinecraftServer) error {
	// Generate Velocity server name if not set
	if server.VelocityServerName == "" {
		server.VelocityServerName = GenerateVelocityServerName(server)
	}

	// Mark as registered
	server.VelocityRegistered = true

	// Update in database
	if err := v.repo.Update(server); err != nil {
		return fmt.Errorf("failed to update server: %w", err)
	}

	// Regenerate config and reload
	if err := v.RegenerateConfig(); err != nil {
		return fmt.Errorf("failed to regenerate config: %w", err)
	}

	if err := v.ReloadConfig(); err != nil {
		logger.Warn("Failed to reload Velocity config", map[string]interface{}{
			"error": err.Error(),
		})
	}

	logger.Info("Server registered with Velocity", map[string]interface{}{
		"server_id":   server.ID,
		"server_name": server.VelocityServerName,
	})

	return nil
}

// UnregisterServer removes a server from Velocity configuration
func (v *VelocityService) UnregisterServer(serverID string) error {
	server, err := v.repo.FindByID(serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	server.VelocityRegistered = false
	server.VelocityServerName = ""

	if err := v.repo.Update(server); err != nil {
		return fmt.Errorf("failed to update server: %w", err)
	}

	// Regenerate config and reload
	if err := v.RegenerateConfig(); err != nil {
		return fmt.Errorf("failed to regenerate config: %w", err)
	}

	if err := v.ReloadConfig(); err != nil {
		logger.Warn("Failed to reload Velocity config", map[string]interface{}{
			"error": err.Error(),
		})
	}

	logger.Info("Server unregistered from Velocity", map[string]interface{}{
		"server_id": serverID,
	})

	return nil
}

// RegenerateConfig regenerates the velocity.toml from database
func (v *VelocityService) RegenerateConfig() error {
	// Get all registered servers
	allServers, err := v.repo.FindAll()
	if err != nil {
		return fmt.Errorf("failed to get servers: %w", err)
	}

	// Filter for Velocity-registered servers
	var registeredServers []models.MinecraftServer
	for _, server := range allServers {
		if server.VelocityRegistered {
			registeredServers = append(registeredServers, server)
		}
	}

	// Generate config
	if err := v.configGenerator.GenerateConfig(registeredServers); err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}

	return nil
}

// ReloadConfig sends a reload command to Velocity
func (v *VelocityService) ReloadConfig() error {
	if !v.IsRunning() {
		return fmt.Errorf("Velocity proxy is not running")
	}

	ctx := context.Background()

	// Execute "velocityreload" command in container
	execConfig := container.ExecOptions{
		Cmd:          []string{"rcon-cli", "velocity", "reload"},
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := v.client.ContainerExecCreate(ctx, v.containerID, execConfig)
	if err != nil {
		// Reload not critical, log warning
		logger.Warn("Failed to reload Velocity config", map[string]interface{}{
			"error": err.Error(),
		})
		return nil
	}

	if err := v.client.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); err != nil {
		logger.Warn("Failed to start Velocity reload", map[string]interface{}{
			"error": err.Error(),
		})
		return nil
	}

	// Wait a bit for reload to complete
	time.Sleep(1 * time.Second)

	logger.Info("Velocity config reloaded", nil)
	return nil
}

// GetContainerID returns the Velocity container ID
func (v *VelocityService) GetContainerID() string {
	return v.containerID
}
