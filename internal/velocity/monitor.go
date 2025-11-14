package velocity

import (
	"fmt"
	"sync"
	"time"

	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/config"
	"github.com/payperplay/hosting/pkg/logger"
)

// VelocityMonitor monitors Velocity health and auto-recovers from restarts
type VelocityMonitor struct {
	client       *RemoteVelocityClient
	serverRepo   *repository.ServerRepository
	cfg          *config.Config
	conductor    ConductorInterface // Interface to avoid circular dependency
	checkInterval time.Duration
	retryInterval time.Duration
	isHealthy    bool
	healthyMu    sync.RWMutex
	stopChan     chan struct{}
	wg           sync.WaitGroup
}

// RemoteNodeGetter defines the interface for getting remote node information
type RemoteNodeGetter interface {
	GetIPAddress() string
}

// ConductorInterface defines the methods we need from Conductor
type ConductorInterface interface {
	GetRemoteNode(nodeID string) (RemoteNodeGetter, error)
}

// NewVelocityMonitor creates a new Velocity monitor
func NewVelocityMonitor(
	client *RemoteVelocityClient,
	serverRepo *repository.ServerRepository,
	cfg *config.Config,
) *VelocityMonitor {
	return &VelocityMonitor{
		client:        client,
		serverRepo:    serverRepo,
		cfg:           cfg,
		checkInterval: 30 * time.Second, // Check every 30 seconds
		retryInterval: 5 * time.Second,   // Retry failed checks every 5 seconds
		isHealthy:     false,
		stopChan:      make(chan struct{}),
	}
}

// SetConductor sets the conductor instance (called after conductor is initialized)
func (m *VelocityMonitor) SetConductor(conductor ConductorInterface) {
	m.conductor = conductor
}

// Start begins health monitoring
func (m *VelocityMonitor) Start() {
	m.wg.Add(1)
	go m.healthCheckLoop()
	logger.Info("Velocity monitor started", map[string]interface{}{
		"check_interval": m.checkInterval.String(),
	})
}

// Stop stops the health monitor
func (m *VelocityMonitor) Stop() {
	close(m.stopChan)
	m.wg.Wait()
	logger.Info("Velocity monitor stopped", nil)
}

// IsHealthy returns current health status
func (m *VelocityMonitor) IsHealthy() bool {
	m.healthyMu.RLock()
	defer m.healthyMu.RUnlock()
	return m.isHealthy
}

// setHealthStatus updates health status thread-safely
func (m *VelocityMonitor) setHealthStatus(healthy bool) {
	m.healthyMu.Lock()
	defer m.healthyMu.Unlock()

	// Detect state transition (unhealthy → healthy)
	wasUnhealthy := !m.isHealthy
	m.isHealthy = healthy

	// If Velocity just recovered, trigger state sync
	if wasUnhealthy && healthy {
		logger.Info("Velocity recovery detected - triggering state sync", nil)
		go m.syncServerState()
	}
}

// healthCheckLoop runs periodic health checks
func (m *VelocityMonitor) healthCheckLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	// Initial health check
	m.performHealthCheck()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.performHealthCheck()
		}
	}
}

// performHealthCheck executes a single health check
func (m *VelocityMonitor) performHealthCheck() {
	health, err := m.client.HealthCheck()
	if err != nil {
		m.setHealthStatus(false)
		logger.Warn("Velocity health check failed", map[string]interface{}{
			"error": err.Error(),
		})

		// Retry more frequently when unhealthy
		time.AfterFunc(m.retryInterval, func() {
			if !m.IsHealthy() {
				m.performHealthCheck()
			}
		})
		return
	}

	m.setHealthStatus(true)
	logger.Debug("Velocity health check passed", map[string]interface{}{
		"version":        health.Version,
		"servers":        health.ServersCount,
		"players_online": health.PlayersOnline,
	})
}

// syncServerState re-registers all running servers with Velocity
func (m *VelocityMonitor) syncServerState() {
	if m.conductor == nil {
		logger.Warn("Cannot sync Velocity state: Conductor not set", nil)
		return
	}

	logger.Info("Syncing server state with Velocity after recovery...", nil)

	runningServers, err := m.serverRepo.FindByStatus(string(models.StatusRunning))
	if err != nil {
		logger.Error("Failed to find running servers for Velocity sync", err, nil)
		return
	}

	registered := 0
	failed := 0

	for _, server := range runningServers {
		if server.NodeID == "" {
			logger.Warn("Skipping server without node assignment", map[string]interface{}{
				"server_id": server.ID,
			})
			continue
		}

		velocityServerName := "mc-" + server.ID

		// Get node IP
		var serverIP string
		if server.NodeID == "local-node" {
			serverIP = m.cfg.ControlPlaneIP
		} else {
			remoteNode, err := m.conductor.GetRemoteNode(server.NodeID)
			if err != nil {
				logger.Warn("Failed to get node IP", map[string]interface{}{
					"server_id": server.ID,
					"node_id":   server.NodeID,
					"error":     err.Error(),
				})
				failed++
				continue
			}
			serverIP = remoteNode.GetIPAddress()
		}

		serverAddress := fmt.Sprintf("%s:%d", serverIP, server.Port)

		if err := m.client.RegisterServer(velocityServerName, serverAddress); err != nil {
			logger.Warn("Failed to register server with Velocity", map[string]interface{}{
				"server_id": server.ID,
				"error":     err.Error(),
			})
			failed++
		} else {
			registered++
		}
	}

	logger.Info("Velocity state sync completed", map[string]interface{}{
		"total_running": len(runningServers),
		"registered":    registered,
		"failed":        failed,
	})
}
