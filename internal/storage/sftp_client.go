package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/payperplay/hosting/pkg/config"
	"github.com/payperplay/hosting/pkg/logger"
)

// SFTPClient handles SFTP operations for Hetzner Storage Box
type SFTPClient struct {
	config      *config.Config
	sshClient   *ssh.Client
	sftpClient  *sftp.Client
	connected   bool
	lastUsed    time.Time // For connection timeout
	idleTimeout time.Duration
}

// NewSFTPClient creates a new SFTP client for Hetzner Storage Box
func NewSFTPClient(cfg *config.Config) (*SFTPClient, error) {
	if !cfg.StorageBoxEnabled {
		return nil, fmt.Errorf("storage box not enabled in configuration")
	}

	if cfg.StorageBoxHost == "" || cfg.StorageBoxUser == "" || cfg.StorageBoxPassword == "" {
		return nil, fmt.Errorf("storage box credentials missing in configuration")
	}

	client := &SFTPClient{
		config:      cfg,
		connected:   false,
		idleTimeout: 5 * time.Minute, // Close connection after 5min idle
	}

	return client, nil
}

// ensureConnected checks if connection is alive and reconnects if needed
func (c *SFTPClient) ensureConnected() error {
	// Check if connection is stale (idle too long)
	if c.connected && time.Since(c.lastUsed) > c.idleTimeout {
		logger.Info("SFTP: Connection idle too long, reconnecting", map[string]interface{}{
			"idle_duration": time.Since(c.lastUsed).Round(time.Second),
		})
		c.Close()
	}

	// Connect if not connected
	if !c.connected {
		return c.Connect()
	}

	// Update last used timestamp
	c.lastUsed = time.Now()
	return nil
}

// Connect establishes connection to Hetzner Storage Box via SFTP
func (c *SFTPClient) Connect() error {
	if c.connected {
		c.lastUsed = time.Now()
		return nil // Already connected
	}

	// SSH client configuration with password authentication
	sshConfig := &ssh.ClientConfig{
		User: c.config.StorageBoxUser,
		Auth: []ssh.AuthMethod{
			ssh.Password(c.config.StorageBoxPassword),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Hetzner Storage Box uses self-signed certs
		Timeout:         30 * time.Second,
	}

	// Connect to SSH server
	address := fmt.Sprintf("%s:%d", c.config.StorageBoxHost, c.config.StorageBoxPort)
	logger.Info("SFTP: Connecting to Storage Box", map[string]interface{}{
		"host": c.config.StorageBoxHost,
		"port": c.config.StorageBoxPort,
		"user": c.config.StorageBoxUser,
	})

	sshClient, err := ssh.Dial("tcp", address, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server: %w", err)
	}

	// Create SFTP client
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}

	c.sshClient = sshClient
	c.sftpClient = sftpClient
	c.connected = true
	c.lastUsed = time.Now()

	logger.Info("SFTP: Connected successfully to Storage Box", nil)

	// Ensure base path exists
	if err := c.ensureBasePath(); err != nil {
		logger.Warn("SFTP: Failed to create base path (may already exist)", map[string]interface{}{
			"path":  c.config.StorageBoxPath,
			"error": err.Error(),
		})
	}

	return nil
}

// Close closes the SFTP and SSH connections
func (c *SFTPClient) Close() error {
	if !c.connected {
		return nil
	}

	if c.sftpClient != nil {
		c.sftpClient.Close()
	}

	if c.sshClient != nil {
		c.sshClient.Close()
	}

	c.connected = false
	logger.Info("SFTP: Connection closed", nil)

	return nil
}

// ensureBasePath creates the base path if it doesn't exist
func (c *SFTPClient) ensureBasePath() error {
	return c.sftpClient.MkdirAll(c.config.StorageBoxPath)
}

// Upload uploads a local file to the Storage Box
// localPath: absolute path to local file
// remoteName: filename on Storage Box (will be placed in StorageBoxPath)
// Returns: full remote path
func (c *SFTPClient) Upload(localPath, remoteName string) (string, error) {
	if err := c.ensureConnected(); err != nil {
		return "", fmt.Errorf("failed to ensure connection: %w", err)
	}

	// Open local file
	localFile, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	// Get file size for progress tracking
	fileInfo, err := localFile.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat local file: %w", err)
	}
	fileSize := fileInfo.Size()

	// Construct remote path
	remotePath := filepath.Join(c.config.StorageBoxPath, remoteName)

	logger.Info("SFTP: Starting upload", map[string]interface{}{
		"local_path":  localPath,
		"remote_path": remotePath,
		"size_mb":     fileSize / 1024 / 1024,
	})

	// Create remote file
	remoteFile, err := c.sftpClient.Create(remotePath)
	if err != nil {
		return "", fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	// Copy with progress tracking
	startTime := time.Now()
	written, err := io.Copy(remoteFile, localFile)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	duration := time.Since(startTime)
	speed := float64(written) / duration.Seconds() / 1024 / 1024 // MB/s

	logger.Info("SFTP: Upload completed", map[string]interface{}{
		"remote_path": remotePath,
		"size_mb":     written / 1024 / 1024,
		"duration":    duration.Round(time.Second),
		"speed_mbps":  fmt.Sprintf("%.2f", speed),
	})

	return remotePath, nil
}

// Download downloads a file from Storage Box to local filesystem
// remotePath: full path on Storage Box (e.g., /minecraft-archives/server-id.tar.gz)
// localPath: absolute path where to save the file
func (c *SFTPClient) Download(remotePath, localPath string) error {
	if err := c.ensureConnected(); err != nil {
		return fmt.Errorf("failed to ensure connection: %w", err)
	}

	logger.Info("SFTP: Starting download", map[string]interface{}{
		"remote_path": remotePath,
		"local_path":  localPath,
	})

	// Open remote file
	remoteFile, err := c.sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %w", err)
	}
	defer remoteFile.Close()

	// Get file size
	remoteInfo, err := remoteFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat remote file: %w", err)
	}
	fileSize := remoteInfo.Size()

	// Ensure local directory exists
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create local directory: %w", err)
	}

	// Create local file
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer localFile.Close()

	// Copy with progress tracking
	startTime := time.Now()
	written, err := io.Copy(localFile, remoteFile)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}

	duration := time.Since(startTime)
	speed := float64(written) / duration.Seconds() / 1024 / 1024 // MB/s

	logger.Info("SFTP: Download completed", map[string]interface{}{
		"local_path":  localPath,
		"size_mb":     fileSize / 1024 / 1024,
		"duration":    duration.Round(time.Second),
		"speed_mbps":  fmt.Sprintf("%.2f", speed),
	})

	return nil
}

// Delete deletes a file from Storage Box
func (c *SFTPClient) Delete(remotePath string) error {
	if err := c.ensureConnected(); err != nil {
		return fmt.Errorf("failed to ensure connection: %w", err)
	}

	logger.Info("SFTP: Deleting file", map[string]interface{}{
		"remote_path": remotePath,
	})

	if err := c.sftpClient.Remove(remotePath); err != nil {
		return fmt.Errorf("failed to delete remote file: %w", err)
	}

	logger.Info("SFTP: File deleted successfully", map[string]interface{}{
		"remote_path": remotePath,
	})

	return nil
}

// Exists checks if a file exists on Storage Box
func (c *SFTPClient) Exists(remotePath string) (bool, error) {
	if err := c.ensureConnected(); err != nil {
		return false, fmt.Errorf("failed to ensure connection: %w", err)
	}

	_, err := c.sftpClient.Stat(remotePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file existence: %w", err)
	}

	return true, nil
}

// ListArchives lists all archive files in the Storage Box
func (c *SFTPClient) ListArchives() ([]string, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, fmt.Errorf("failed to ensure connection: %w", err)
	}

	files, err := c.sftpClient.ReadDir(c.config.StorageBoxPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list archives: %w", err)
	}

	archives := []string{}
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".gz" {
			archives = append(archives, file.Name())
		}
	}

	return archives, nil
}
