package velocity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/payperplay/hosting/pkg/logger"
)

// RemoteVelocityClient communicates with the Velocity Remote API Plugin
// running on a dedicated Velocity proxy server (Tier 2 in 3-Tier Architecture).
//
// Architecture:
// - Tier 1 (Control Plane): PayPerPlay API + DB (this code)
// - Tier 2 (Proxy Layer): Velocity Proxy + RemoteAPI Plugin (target of this client)
// - Tier 3 (Workload Layer): Minecraft servers on Cloud nodes
type RemoteVelocityClient struct {
	apiURL     string
	httpClient *http.Client
}

// ServerRegistration represents the payload for registering a server
type ServerRegistration struct {
	Name    string `json:"name"`
	Address string `json:"address"` // format: "host:port"
}

// ServerListResponse represents the response from GET /api/servers
type ServerListResponse struct {
	Status  string                   `json:"status"`
	Count   int                      `json:"count"`
	Servers []VelocityServerInfo     `json:"servers"`
}

// VelocityServerInfo represents a registered server in Velocity
type VelocityServerInfo struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Players int    `json:"players"`
}

// HealthCheckResponse represents the response from GET /health
type HealthCheckResponse struct {
	Status       string `json:"status"`
	Version      string `json:"version"`
	ServersCount int    `json:"servers_count"`
	PlayersOnline int   `json:"players_online"`
}

// NewRemoteVelocityClient creates a new client for the Velocity Remote API
func NewRemoteVelocityClient(apiURL string) *RemoteVelocityClient {
	return &RemoteVelocityClient{
		apiURL:     apiURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// RegisterServer registers a new backend server with Velocity proxy
//
// Example: RegisterServer("survival-1", "91.98.202.235:25566")
func (c *RemoteVelocityClient) RegisterServer(name, address string) error {
	payload := ServerRegistration{
		Name:    name,
		Address: address,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	resp, err := c.httpClient.Post(
		c.apiURL+"/api/servers",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	logger.Info("Server registered with Velocity", map[string]interface{}{
		"name":    name,
		"address": address,
	})

	return nil
}

// UnregisterServer removes a backend server from Velocity proxy
func (c *RemoteVelocityClient) UnregisterServer(name string) error {
	req, err := http.NewRequest(
		http.MethodDelete,
		fmt.Sprintf("%s/api/servers/%s", c.apiURL, name),
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	logger.Info("Server unregistered from Velocity", map[string]interface{}{
		"name": name,
	})

	return nil
}

// ListServers returns all registered servers from Velocity
func (c *RemoteVelocityClient) ListServers() ([]VelocityServerInfo, error) {
	resp, err := c.httpClient.Get(c.apiURL + "/api/servers")
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var response ServerListResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Servers, nil
}

// GetPlayerCount returns the player count for a specific server
func (c *RemoteVelocityClient) GetPlayerCount(serverName string) (int, error) {
	resp, err := c.httpClient.Get(fmt.Sprintf("%s/api/players/%s", c.apiURL, serverName))
	if err != nil {
		return 0, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return 0, fmt.Errorf("server not found: %s", serverName)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Status  string `json:"status"`
		Server  string `json:"server"`
		Players int    `json:"players"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Players, nil
}

// HealthCheck pings the Velocity Remote API to verify connectivity
func (c *RemoteVelocityClient) HealthCheck() (*HealthCheckResponse, error) {
	resp, err := c.httpClient.Get(c.apiURL + "/health")
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var health HealthCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &health, nil
}

// SyncServerState syncs the current server state with Velocity
// This should be called after API restarts to ensure Velocity knows about running servers
func (c *RemoteVelocityClient) SyncServerState(servers []ServerRegistration) error {
	// Get current Velocity servers
	velocityServers, err := c.ListServers()
	if err != nil {
		return fmt.Errorf("failed to list Velocity servers: %w", err)
	}

	// Build map of existing servers
	existingServers := make(map[string]bool)
	for _, vs := range velocityServers {
		existingServers[vs.Name] = true
	}

	// Register missing servers
	for _, server := range servers {
		if !existingServers[server.Name] {
			if err := c.RegisterServer(server.Name, server.Address); err != nil {
				logger.Warn("Failed to sync server with Velocity", map[string]interface{}{
					"name":  server.Name,
					"error": err.Error(),
				})
			}
		}
	}

	logger.Info("Server state synced with Velocity", map[string]interface{}{
		"synced_count": len(servers),
	})

	return nil
}
