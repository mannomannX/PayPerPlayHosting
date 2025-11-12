package cloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/payperplay/hosting/pkg/logger"
)

const (
	HetznerAPIBaseURL = "https://api.hetzner.cloud/v1"
)

// HetznerProvider implements CloudProvider for Hetzner Cloud
type HetznerProvider struct {
	token      string
	httpClient *http.Client
}

// NewHetznerProvider creates a new Hetzner Cloud provider
func NewHetznerProvider(token string) *HetznerProvider {
	return &HetznerProvider{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ===== Server Management =====

// CreateServer creates a new cloud server
func (p *HetznerProvider) CreateServer(spec ServerSpec) (*Server, error) {
	reqBody := map[string]interface{}{
		"name":        spec.Name,
		"server_type": spec.Type,
		"image":       spec.Image,
		"location":    spec.Location,
		"user_data":   spec.CloudInit,
		"labels":      spec.Labels,
		"ssh_keys":    spec.SSHKeys,
		"start_after_create": true,
	}

	resp, err := p.request("POST", "/servers", reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create server: %w", err)
	}

	var result struct {
		Server hetznerServer `json:"server"`
		Action hetznerAction `json:"action"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	server := p.convertServer(&result.Server)

	logger.Info("Hetzner server created", map[string]interface{}{
		"server_id":   server.ID,
		"server_name": server.Name,
		"type":        server.Type,
		"ip":          server.IPAddress,
	})

	return server, nil
}

// DeleteServer deletes a cloud server
func (p *HetznerProvider) DeleteServer(serverID string) error {
	_, err := p.request("DELETE", "/servers/"+serverID, nil)
	if err != nil {
		return fmt.Errorf("failed to delete server: %w", err)
	}

	logger.Info("Hetzner server deleted", map[string]interface{}{
		"server_id": serverID,
	})

	return nil
}

// ListServers lists all servers with optional label filters
func (p *HetznerProvider) ListServers(labels map[string]string) ([]*Server, error) {
	endpoint := "/servers"

	// Add label selector if provided
	if len(labels) > 0 {
		endpoint += "?label_selector="
		first := true
		for k, v := range labels {
			if !first {
				endpoint += ","
			}
			endpoint += k + "=" + v
			first = false
		}
	}

	resp, err := p.request("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}

	var result struct {
		Servers []hetznerServer `json:"servers"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	servers := make([]*Server, 0, len(result.Servers))
	for i := range result.Servers {
		servers = append(servers, p.convertServer(&result.Servers[i]))
	}

	return servers, nil
}

// GetServer retrieves a single server by ID
func (p *HetznerProvider) GetServer(serverID string) (*Server, error) {
	resp, err := p.request("GET", "/servers/"+serverID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get server: %w", err)
	}

	var result struct {
		Server hetznerServer `json:"server"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return p.convertServer(&result.Server), nil
}

// GetServerMetrics retrieves CPU, disk, and network metrics for a server
// Returns average CPU usage percentage over the last 5 minutes
func (p *HetznerProvider) GetServerMetrics(serverID string) (float64, error) {
	// Get metrics for the last 5 minutes
	now := time.Now()
	start := now.Add(-5 * time.Minute).Unix()
	end := now.Unix()

	endpoint := fmt.Sprintf("/servers/%s/metrics?type=cpu&start=%d&end=%d", serverID, start, end)

	resp, err := p.request("GET", endpoint, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get server metrics: %w", err)
	}

	var result struct {
		Metrics struct {
			TimeSeries map[string]struct {
				Values [][]interface{} `json:"values"`
			} `json:"time_series"`
		} `json:"metrics"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return 0, fmt.Errorf("failed to parse metrics response: %w", err)
	}

	// Extract CPU values from time series
	cpuSeries, exists := result.Metrics.TimeSeries["cpu"]
	if !exists || len(cpuSeries.Values) == 0 {
		return 0, nil // No data available
	}

	// Calculate average CPU usage
	var totalCPU float64
	count := 0
	for _, point := range cpuSeries.Values {
		if len(point) >= 2 {
			// point[0] is timestamp, point[1] is CPU value
			if cpuVal, ok := point[1].(float64); ok {
				totalCPU += cpuVal
				count++
			}
		}
	}

	if count == 0 {
		return 0, nil
	}

	avgCPU := totalCPU / float64(count)
	return avgCPU, nil
}

// ===== Server Types =====

// GetServerTypes returns all available server types
func (p *HetznerProvider) GetServerTypes() ([]*ServerType, error) {
	resp, err := p.request("GET", "/server_types", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get server types: %w", err)
	}

	var result struct {
		ServerTypes []hetznerServerType `json:"server_types"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	types := make([]*ServerType, 0, len(result.ServerTypes))
	for i := range result.ServerTypes {
		types = append(types, p.convertServerType(&result.ServerTypes[i]))
	}

	return types, nil
}

// GetUbuntuImage finds the latest Ubuntu LTS image by version
func (p *HetznerProvider) GetUbuntuImage(version string) (string, error) {
	resp, err := p.request("GET", "/images?type=system&architecture=x86", nil)
	if err != nil {
		return "", fmt.Errorf("failed to get images: %w", err)
	}

	var result struct {
		Images []struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			OSFlavor    string `json:"os_flavor"`
			OSVersion   string `json:"os_version"`
		} `json:"images"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Find Ubuntu image matching version (e.g., "22.04")
	for _, img := range result.Images {
		if img.OSFlavor == "ubuntu" && img.OSVersion == version {
			return fmt.Sprintf("%d", img.ID), nil
		}
	}

	return "", fmt.Errorf("ubuntu %s image not found", version)
}

// GetServerType returns a specific server type by name
func (p *HetznerProvider) GetServerType(name string) (*ServerType, error) {
	resp, err := p.request("GET", "/server_types/"+name, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get server type: %w", err)
	}

	var result struct {
		ServerType hetznerServerType `json:"server_type"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return p.convertServerType(&result.ServerType), nil
}

// ===== Health & Status =====

// WaitForServerReady waits until the server is running and ready
func (p *HetznerProvider) WaitForServerReady(serverID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		server, err := p.GetServer(serverID)
		if err != nil {
			return fmt.Errorf("failed to check server status: %w", err)
		}

		if server.Status == ServerStatusRunning {
			logger.Info("Server is ready", map[string]interface{}{
				"server_id": serverID,
			})
			return nil
		}

		logger.Debug("Waiting for server to be ready", map[string]interface{}{
			"server_id": serverID,
			"status":    server.Status,
		})

		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("timeout waiting for server to be ready")
}

// GetServerStatus returns the current status of a server
func (p *HetznerProvider) GetServerStatus(serverID string) (ServerStatus, error) {
	server, err := p.GetServer(serverID)
	if err != nil {
		return ServerStatusUnknown, err
	}
	return server.Status, nil
}

// ===== Server Actions =====

// PowerOnServer powers on a stopped server
func (p *HetznerProvider) PowerOnServer(serverID string) error {
	_, err := p.request("POST", "/servers/"+serverID+"/actions/poweron", nil)
	if err != nil {
		return fmt.Errorf("failed to power on server: %w", err)
	}
	return nil
}

// PowerOffServer powers off a running server
func (p *HetznerProvider) PowerOffServer(serverID string) error {
	_, err := p.request("POST", "/servers/"+serverID+"/actions/poweroff", nil)
	if err != nil {
		return fmt.Errorf("failed to power off server: %w", err)
	}
	return nil
}

// RebootServer reboots a server
func (p *HetznerProvider) RebootServer(serverID string) error {
	_, err := p.request("POST", "/servers/"+serverID+"/actions/reboot", nil)
	if err != nil {
		return fmt.Errorf("failed to reboot server: %w", err)
	}
	return nil
}

// ===== Snapshots =====

// CreateSnapshot creates a snapshot of a server
func (p *HetznerProvider) CreateSnapshot(serverID string, description string) (*Snapshot, error) {
	reqBody := map[string]interface{}{
		"description": description,
	}

	resp, err := p.request("POST", "/servers/"+serverID+"/actions/create_image", reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	var result struct {
		Image hetznerImage `json:"image"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	snapshot := &Snapshot{
		ID:          strconv.FormatInt(result.Image.ID, 10),
		Name:        result.Image.Name,
		Description: result.Image.Description,
		ImageSize:   result.Image.ImageSize,
		CreatedAt:   result.Image.Created,
	}

	logger.Info("Snapshot created", map[string]interface{}{
		"snapshot_id": snapshot.ID,
		"server_id":   serverID,
	})

	return snapshot, nil
}

// DeleteSnapshot deletes a snapshot
func (p *HetznerProvider) DeleteSnapshot(snapshotID string) error {
	_, err := p.request("DELETE", "/images/"+snapshotID, nil)
	if err != nil {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}

	logger.Info("Snapshot deleted", map[string]interface{}{
		"snapshot_id": snapshotID,
	})

	return nil
}

// CreateServerFromSnapshot creates a new server from a snapshot
func (p *HetznerProvider) CreateServerFromSnapshot(snapshotID string, spec ServerSpec) (*Server, error) {
	// Convert snapshot ID to int for Hetzner API
	imageID, err := strconv.ParseInt(snapshotID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid snapshot ID: %w", err)
	}

	// Use snapshot as image instead of OS image
	spec.Image = strconv.FormatInt(imageID, 10)

	return p.CreateServer(spec)
}

// ===== Pricing =====

// GetServerPricing returns pricing information for a server type
func (p *HetznerProvider) GetServerPricing(serverType string) (*Pricing, error) {
	st, err := p.GetServerType(serverType)
	if err != nil {
		return nil, err
	}

	return &Pricing{
		HourlyCostEUR:  st.HourlyCostEUR,
		MonthlyCostEUR: st.MonthlyCostEUR,
		Currency:       "EUR",
	}, nil
}

// ===== HTTP Request Helper =====

func (p *HetznerProvider) request(method, endpoint string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, HetznerAPIBaseURL+endpoint, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// ===== Conversion Helpers =====

func (p *HetznerProvider) convertServer(hs *hetznerServer) *Server {
	var publicIP string
	if hs.PublicNet.IPv4.IP != "" {
		publicIP = hs.PublicNet.IPv4.IP
	}

	// Calculate hourly cost from monthly price
	hourlyCost := 0.0
	if hs.ServerType.Prices != nil && len(hs.ServerType.Prices) > 0 {
		monthlyStr := hs.ServerType.Prices[0].Monthly.Gross
		if monthly, err := strconv.ParseFloat(monthlyStr, 64); err == nil {
			hourlyCost = monthly / 730.0 // ~730 hours per month
		}
	}

	return &Server{
		ID:            strconv.FormatInt(hs.ID, 10),
		Name:          hs.Name,
		Type:          hs.ServerType.Name,
		Status:        p.convertStatus(hs.Status),
		IPAddress:     publicIP,
		Location:      hs.Datacenter.Location.Name,
		CreatedAt:     hs.Created,
		Labels:        hs.Labels,
		HourlyCostEUR: hourlyCost,
	}
}

func (p *HetznerProvider) convertServerType(hst *hetznerServerType) *ServerType {
	hourlyCost := 0.0
	monthlyCost := 0.0

	// Find pricing for deployment location (nbg1 or fallback to fsn1/hel1/first available)
	preferredLocations := []string{"nbg1", "fsn1", "hel1"} // German datacenters

	if hst.Prices != nil && len(hst.Prices) > 0 {
		// Try to find price for preferred locations
		var priceData *hetznerPrice
		for _, prefLoc := range preferredLocations {
			for i := range hst.Prices {
				if hst.Prices[i].Location == prefLoc {
					priceData = &hst.Prices[i]
					break
				}
			}
			if priceData != nil {
				break
			}
		}

		// Fallback to first price if no preferred location found
		if priceData == nil {
			priceData = &hst.Prices[0]
		}

		if monthly, err := strconv.ParseFloat(priceData.Monthly.Gross, 64); err == nil {
			monthlyCost = monthly
			hourlyCost = monthly / 730.0
		}
	}

	return &ServerType{
		ID:             strconv.FormatInt(hst.ID, 10),
		Name:           hst.Name,
		Description:    hst.Description,
		Cores:          hst.Cores,
		RAMMB:          int(hst.Memory * 1024), // GB to MB
		DiskGB:         hst.Disk,
		HourlyCostEUR:  hourlyCost,
		MonthlyCostEUR: monthlyCost,
		Available:      true,
	}
}

func (p *HetznerProvider) convertStatus(status string) ServerStatus {
	switch status {
	case "initializing":
		return ServerStatusInitializing
	case "starting":
		return ServerStatusStarting
	case "running":
		return ServerStatusRunning
	case "stopping":
		return ServerStatusStopping
	case "off":
		return ServerStatusStopped
	case "deleting":
		return ServerStatusDeleting
	default:
		return ServerStatusUnknown
	}
}

// ===== Hetzner API Response Types =====

type hetznerServer struct {
	ID         int64                  `json:"id"`
	Name       string                 `json:"name"`
	Status     string                 `json:"status"`
	PublicNet  hetznerPublicNet       `json:"public_net"`
	ServerType hetznerServerType      `json:"server_type"`
	Datacenter hetznerDatacenter      `json:"datacenter"`
	Created    time.Time              `json:"created"`
	Labels     map[string]string      `json:"labels"`
}

type hetznerPublicNet struct {
	IPv4 hetznerIPv4 `json:"ipv4"`
}

type hetznerIPv4 struct {
	IP string `json:"ip"`
}

type hetznerDatacenter struct {
	Location hetznerLocation `json:"location"`
}

type hetznerLocation struct {
	Name string `json:"name"`
}

type hetznerServerType struct {
	ID          int64           `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Cores       int             `json:"cores"`
	Memory      float64         `json:"memory"` // in GB
	Disk        int             `json:"disk"`   // in GB
	Prices      []hetznerPrice  `json:"prices"`
}

type hetznerPrice struct {
	Location string             `json:"location"`
	Monthly  hetznerPriceDetail `json:"price_monthly"`
	Hourly   hetznerPriceDetail `json:"price_hourly"`
}

type hetznerPriceDetail struct {
	Gross string `json:"gross"`
}

type hetznerAction struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
}

type hetznerImage struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ImageSize   float64   `json:"image_size"` // in GB
	Created     time.Time `json:"created"`
}
