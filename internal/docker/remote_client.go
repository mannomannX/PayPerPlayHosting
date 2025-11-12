package docker

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// RemoteDockerClient manages Docker containers on remote nodes via SSH
type RemoteDockerClient struct {
	sshKeyPath string
}

// NewRemoteDockerClient creates a new remote Docker client
func NewRemoteDockerClient(sshKeyPath string) (*RemoteDockerClient, error) {
	// Verify SSH key exists
	// Note: We don't load it here, we load it per-connection
	// This allows for different keys per node in the future
	return &RemoteDockerClient{
		sshKeyPath: sshKeyPath,
	}, nil
}

// RemoteNode represents the minimal node information needed for remote operations
type RemoteNode struct {
	ID        string
	IPAddress string
	SSHUser   string
}

// StartContainer creates and starts a Docker container on a remote node
func (r *RemoteDockerClient) StartContainer(
	ctx context.Context,
	node *RemoteNode,
	containerName string,
	imageName string,
	env []string,
	portBindings map[string]int, // internal port -> host port
	binds []string,               // volume binds
	ramMB int,
) (string, error) {
	// Build docker run command
	cmd := r.buildDockerRunCommand(containerName, imageName, env, portBindings, binds, ramMB)

	// Execute command via SSH
	output, err := r.executeSSHCommand(ctx, node, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to start container on node %s: %w (output: %s)", node.ID, err, output)
	}

	// Docker run returns container ID
	containerID := strings.TrimSpace(output)
	if containerID == "" {
		return "", fmt.Errorf("no container ID returned from docker run")
	}

	log.Printf("[RemoteDocker] Started container %s on node %s (ID: %s)", containerName, node.ID, containerID[:12])
	return containerID, nil
}

// StopContainer stops a Docker container on a remote node
func (r *RemoteDockerClient) StopContainer(ctx context.Context, node *RemoteNode, containerID string, timeoutSeconds int) error {
	cmd := fmt.Sprintf("docker stop --time %d %s", timeoutSeconds, containerID)

	output, err := r.executeSSHCommand(ctx, node, cmd)
	if err != nil {
		return fmt.Errorf("failed to stop container on node %s: %w (output: %s)", node.ID, err, output)
	}

	log.Printf("[RemoteDocker] Stopped container %s on node %s", containerID[:12], node.ID)
	return nil
}

// RemoveContainer removes a Docker container from a remote node
func (r *RemoteDockerClient) RemoveContainer(ctx context.Context, node *RemoteNode, containerID string, force bool) error {
	forceFlag := ""
	if force {
		forceFlag = " --force"
	}

	cmd := fmt.Sprintf("docker rm%s %s", forceFlag, containerID)

	output, err := r.executeSSHCommand(ctx, node, cmd)
	if err != nil {
		// Check if error is "not found" - that's okay
		if strings.Contains(output, "No such container") || strings.Contains(output, "not found") {
			log.Printf("[RemoteDocker] Container %s does not exist on node %s (already removed)", containerID[:12], node.ID)
			return nil
		}
		return fmt.Errorf("failed to remove container on node %s: %w (output: %s)", node.ID, err, output)
	}

	log.Printf("[RemoteDocker] Removed container %s from node %s", containerID[:12], node.ID)
	return nil
}

// GetContainerLogs retrieves logs from a container on a remote node
func (r *RemoteDockerClient) GetContainerLogs(ctx context.Context, node *RemoteNode, containerID string, tail string) (string, error) {
	cmd := fmt.Sprintf("docker logs --tail %s %s 2>&1", tail, containerID)

	output, err := r.executeSSHCommand(ctx, node, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to get container logs on node %s: %w", node.ID, err)
	}

	return output, nil
}

// StreamContainerLogs streams container logs from a remote node
// NOTE: This is a simplified implementation. For production, consider using SSH multiplexing
func (r *RemoteDockerClient) StreamContainerLogs(ctx context.Context, node *RemoteNode, containerID string) (<-chan string, context.CancelFunc, error) {
	logChan := make(chan string, 100)
	streamCtx, cancel := context.WithCancel(ctx)

	// Start goroutine to poll logs
	go func() {
		defer close(logChan)

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		lastLines := ""

		for {
			select {
			case <-streamCtx.Done():
				return
			case <-ticker.C:
				// Get last 10 lines
				logs, err := r.GetContainerLogs(streamCtx, node, containerID, "10")
				if err != nil {
					log.Printf("[RemoteDocker] Error streaming logs: %v", err)
					return
				}

				// Only send if new content
				if logs != lastLines {
					lines := strings.Split(logs, "\n")
					for _, line := range lines {
						if line != "" {
							select {
							case logChan <- line:
							case <-streamCtx.Done():
								return
							}
						}
					}
					lastLines = logs
				}
			}
		}
	}()

	return logChan, cancel, nil
}

// ExecuteCommand executes a Minecraft command in a container via RCON
func (r *RemoteDockerClient) ExecuteCommand(ctx context.Context, node *RemoteNode, containerID, minecraftCommand string) (string, error) {
	cmd := fmt.Sprintf("docker exec %s rcon-cli %s", containerID, minecraftCommand)

	output, err := r.executeSSHCommand(ctx, node, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to execute command on node %s: %w (output: %s)", node.ID, err, output)
	}

	return strings.TrimSpace(output), nil
}

// GetContainerStatus gets the status of a container on a remote node
func (r *RemoteDockerClient) GetContainerStatus(ctx context.Context, node *RemoteNode, containerID string) (string, error) {
	cmd := fmt.Sprintf("docker inspect --format='{{.State.Status}}' %s", containerID)

	output, err := r.executeSSHCommand(ctx, node, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to get container status on node %s: %w", node.ID, err)
	}

	return strings.TrimSpace(output), nil
}

// ListRunningContainers lists all running mc-* containers on a remote node
func (r *RemoteDockerClient) ListRunningContainers(ctx context.Context, node *RemoteNode) ([]struct {
	ContainerID string
	ServerID    string
}, error) {
	cmd := `docker ps --filter "name=mc-" --format "{{.ID}}|{{.Names}}"`

	output, err := r.executeSSHCommand(ctx, node, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers on node %s: %w", node.ID, err)
	}

	var result []struct {
		ContainerID string
		ServerID    string
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) != 2 {
			continue
		}

		containerID := strings.TrimSpace(parts[0])
		containerName := strings.TrimSpace(parts[1])

		// Extract server ID from container name (format: mc-{serverID})
		if strings.HasPrefix(containerName, "mc-") {
			serverID := strings.TrimPrefix(containerName, "mc-")

			result = append(result, struct {
				ContainerID string
				ServerID    string
			}{
				ContainerID: containerID,
				ServerID:    serverID,
			})
		}
	}

	return result, nil
}

// WaitForServerReady waits for a Minecraft server to be ready by monitoring logs
func (r *RemoteDockerClient) WaitForServerReady(ctx context.Context, node *RemoteNode, containerID string, timeoutSeconds int) error {
	deadline := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for server to be ready")
			}

			// Get last 50 lines of logs
			logs, err := r.GetContainerLogs(ctx, node, containerID, "50")
			if err != nil {
				log.Printf("[RemoteDocker] Error checking logs: %v", err)
				continue
			}

			// Check if server is ready
			if strings.Contains(logs, "Done (") && strings.Contains(logs, "s)!") {
				log.Printf("[RemoteDocker] Minecraft server %s on node %s is ready!", containerID[:12], node.ID)
				return nil
			}
		}
	}
}

// PullImage pulls a Docker image on a remote node
func (r *RemoteDockerClient) PullImage(ctx context.Context, node *RemoteNode, imageName string) error {
	cmd := fmt.Sprintf("docker pull %s", imageName)

	output, err := r.executeSSHCommand(ctx, node, cmd)
	if err != nil {
		return fmt.Errorf("failed to pull image on node %s: %w (output: %s)", node.ID, err, output)
	}

	log.Printf("[RemoteDocker] Pulled image %s on node %s", imageName, node.ID)
	return nil
}

// HealthCheck performs a basic health check on a remote node
func (r *RemoteDockerClient) HealthCheck(ctx context.Context, node *RemoteNode) error {
	cmd := "docker info > /dev/null 2>&1 && echo 'OK'"

	output, err := r.executeSSHCommand(ctx, node, cmd)
	if err != nil {
		return fmt.Errorf("health check failed on node %s: %w", node.ID, err)
	}

	if !strings.Contains(output, "OK") {
		return fmt.Errorf("health check failed on node %s: unexpected output", node.ID)
	}

	return nil
}

// ExecuteSSHCommand executes an arbitrary command on a remote node via SSH
// This is a public wrapper around executeSSHCommand for external use
func (r *RemoteDockerClient) ExecuteSSHCommand(ctx context.Context, node *RemoteNode, command string) (string, error) {
	return r.executeSSHCommand(ctx, node, command)
}

// GetSystemResources retrieves system resources (RAM, CPU) from a remote node via Docker API
// Returns (totalRAMMB, totalCPU, error)
func (r *RemoteDockerClient) GetSystemResources(ctx context.Context, node *RemoteNode) (int, int, error) {
	// Execute: docker info --format '{{.MemTotal}} {{.NCPU}}'
	cmd := "docker info --format '{{.MemTotal}} {{.NCPU}}'"

	output, err := r.executeSSHCommand(ctx, node, cmd)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get system resources on node %s: %w", node.ID, err)
	}

	// Parse output: "4096000000 2"
	parts := strings.Fields(strings.TrimSpace(output))
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected docker info output format: %s", output)
	}

	// Parse memory (bytes) and CPU cores
	var memBytes, cpuCores int64
	if _, err := fmt.Sscanf(parts[0], "%d", &memBytes); err != nil {
		return 0, 0, fmt.Errorf("failed to parse memory: %w", err)
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &cpuCores); err != nil {
		return 0, 0, fmt.Errorf("failed to parse CPU cores: %w", err)
	}

	// Convert bytes to MB
	totalRAMMB := int(memBytes / 1024 / 1024)
	totalCPU := int(cpuCores)

	log.Printf("[RemoteDocker] Detected resources on node %s: %d MB RAM, %d CPU cores",
		node.ID, totalRAMMB, totalCPU)

	return totalRAMMB, totalCPU, nil
}

// --- PRIVATE HELPER METHODS ---

// buildDockerRunCommand builds a docker run command
func (r *RemoteDockerClient) buildDockerRunCommand(
	containerName string,
	imageName string,
	env []string,
	portBindings map[string]int,
	binds []string,
	ramMB int,
) string {
	var cmd strings.Builder
	cmd.WriteString("docker run -d")

	// Container name
	cmd.WriteString(fmt.Sprintf(" --name %s", containerName))

	// Environment variables
	for _, e := range env {
		cmd.WriteString(fmt.Sprintf(" -e '%s'", e))
	}

	// Port bindings
	for internalPort, hostPort := range portBindings {
		cmd.WriteString(fmt.Sprintf(" -p %d:%s", hostPort, internalPort))
	}

	// Volume binds
	for _, bind := range binds {
		cmd.WriteString(fmt.Sprintf(" -v %s", bind))
	}

	// Memory limit (add 25% overhead for JVM)
	memoryBytes := int64(float64(ramMB)*1.25) * 1024 * 1024
	cmd.WriteString(fmt.Sprintf(" --memory=%d", memoryBytes))

	// Restart policy
	cmd.WriteString(" --restart=no")

	// Image name
	cmd.WriteString(fmt.Sprintf(" %s", imageName))

	return cmd.String()
}

// executeSSHCommand executes a command on a remote node via SSH
func (r *RemoteDockerClient) executeSSHCommand(ctx context.Context, node *RemoteNode, command string) (string, error) {
	// Load SSH key
	key, err := r.loadSSHKey()
	if err != nil {
		return "", fmt.Errorf("failed to load SSH key: %w", err)
	}

	// SSH client config
	config := &ssh.ClientConfig{
		User: node.SSHUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // FIXME: Use proper host key verification in production
		Timeout:         10 * time.Second,
	}

	// Connect to remote node
	addr := fmt.Sprintf("%s:22", node.IPAddress)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return "", fmt.Errorf("failed to connect to %s: %w", addr, err)
	}
	defer client.Close()

	// Create session
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Capture output
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// Execute command
	err = session.Run(command)
	if err != nil {
		return stdout.String() + stderr.String(), fmt.Errorf("command failed: %w", err)
	}

	return stdout.String() + stderr.String(), nil
}

// loadSSHKey loads the SSH private key from disk
func (r *RemoteDockerClient) loadSSHKey() (ssh.Signer, error) {
	// If no path specified, try default location
	keyPath := r.sshKeyPath
	if keyPath == "" {
		home := "/root" // Default on Linux
		keyPath = filepath.Join(home, ".ssh", "id_rsa")
	}

	// Read key file
	keyData, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH key file %s: %w", keyPath, err)
	}

	// Parse private key
	key, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSH private key: %w", err)
	}

	return key, nil
}
