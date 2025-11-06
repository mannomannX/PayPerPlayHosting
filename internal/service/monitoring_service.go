package service

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/rcon"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/config"
)

// MonitoringService monitors running servers and handles auto-shutdown
type MonitoringService struct {
	mcService      *MinecraftService
	repo           *repository.ServerRepository
	cfg            *config.Config
	recoveryService *RecoveryService

	// Track idle timers per server
	idleTimers map[string]*IdleTimer
	mu         sync.RWMutex

	// Control
	ctx    context.Context
	cancel context.CancelFunc
}

// IdleTimer tracks how long a server has been idle
type IdleTimer struct {
	ServerID       string
	IdleSince      time.Time
	LastPlayerCount int
	CheckInterval  time.Duration
	TimeoutSeconds int
}

func NewMonitoringService(
	mcService *MinecraftService,
	repo *repository.ServerRepository,
	cfg *config.Config,
) *MonitoringService {
	ctx, cancel := context.WithCancel(context.Background())

	return &MonitoringService{
		mcService:  mcService,
		repo:       repo,
		cfg:        cfg,
		idleTimers: make(map[string]*IdleTimer),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start begins monitoring all running servers
func (m *MonitoringService) Start() {
	log.Println("Starting monitoring service...")

	// Initial scan for running servers
	go m.scanRunningServers()

	// Start monitoring loop
	go m.monitorLoop()
}

// Stop stops the monitoring service
func (m *MonitoringService) Stop() {
	log.Println("Stopping monitoring service...")
	m.cancel()
}

// SetRecoveryService sets the recovery service for crash handling
func (m *MonitoringService) SetRecoveryService(recoveryService *RecoveryService) {
	m.recoveryService = recoveryService
	log.Println("Recovery service linked to monitoring")
}

// monitorLoop runs the main monitoring loop
func (m *MonitoringService) monitorLoop() {
	ticker := time.NewTicker(60 * time.Second) // Check every 60 seconds
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkAllServers()

			// Also check for crashed servers if recovery service is available
			if m.recoveryService != nil {
				if err := m.recoveryService.CheckAndRecoverCrashedServers(); err != nil {
					log.Printf("Error checking for crashed servers: %v", err)
				}
			}
		}
	}
}

// scanRunningServers finds all currently running servers and starts monitoring them
func (m *MonitoringService) scanRunningServers() {
	servers, err := m.repo.FindAll()
	if err != nil {
		log.Printf("Error scanning servers: %v", err)
		return
	}

	for _, server := range servers {
		if server.Status == models.StatusRunning && server.AutoShutdownEnabled {
			m.StartMonitoring(server.ID)
		}
	}
}

// StartMonitoring starts monitoring a specific server
func (m *MonitoringService) StartMonitoring(serverID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Don't start if already monitoring
	if _, exists := m.idleTimers[serverID]; exists {
		return
	}

	server, err := m.repo.FindByID(serverID)
	if err != nil {
		log.Printf("Error finding server %s: %v", serverID, err)
		return
	}

	timer := &IdleTimer{
		ServerID:       serverID,
		IdleSince:      time.Now(),
		LastPlayerCount: 0,
		CheckInterval:  60 * time.Second,
		TimeoutSeconds: server.IdleTimeoutSeconds,
	}

	m.idleTimers[serverID] = timer
	log.Printf("Started monitoring server %s (timeout: %ds)", serverID, timer.TimeoutSeconds)
}

// StopMonitoring stops monitoring a specific server
func (m *MonitoringService) StopMonitoring(serverID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.idleTimers, serverID)
	log.Printf("Stopped monitoring server %s", serverID)
}

// checkAllServers checks all monitored servers
func (m *MonitoringService) checkAllServers() {
	m.mu.RLock()
	serverIDs := make([]string, 0, len(m.idleTimers))
	for id := range m.idleTimers {
		serverIDs = append(serverIDs, id)
	}
	m.mu.RUnlock()

	for _, serverID := range serverIDs {
		go m.checkServer(serverID)
	}
}

// checkServer checks a specific server's player count and idle status
func (m *MonitoringService) checkServer(serverID string) {
	server, err := m.repo.FindByID(serverID)
	if err != nil {
		log.Printf("Error finding server %s: %v", serverID, err)
		m.StopMonitoring(serverID)
		return
	}

	// Skip if not running
	if server.Status != models.StatusRunning {
		m.StopMonitoring(serverID)
		return
	}

	// Skip if auto-shutdown disabled
	if !server.AutoShutdownEnabled {
		m.StopMonitoring(serverID)
		return
	}

	// Try to get player count via RCON
	playerCount, err := m.getPlayerCount(server)

	m.mu.Lock()
	timer, exists := m.idleTimers[serverID]
	if !exists {
		m.mu.Unlock()
		return
	}

	if err != nil {
		log.Printf("Warning: Could not get player count for %s: %v", serverID, err)
		// Don't shutdown on error - might be server still starting
		m.mu.Unlock()
		return
	}

	// Update timer
	timer.LastPlayerCount = playerCount

	if playerCount > 0 {
		// Server has players, reset idle timer
		timer.IdleSince = time.Now()
		log.Printf("Server %s has %d players online", serverID, playerCount)

		// Update usage log with peak player count
		m.updatePeakPlayerCount(serverID, playerCount)

		m.mu.Unlock()
	} else {
		// Server is empty, check if timeout reached
		idleDuration := time.Since(timer.IdleSince)
		timeoutDuration := time.Duration(timer.TimeoutSeconds) * time.Second

		log.Printf("Server %s idle for %v (timeout: %v)", serverID, idleDuration.Round(time.Second), timeoutDuration)

		if idleDuration >= timeoutDuration {
			m.mu.Unlock()
			log.Printf("Server %s reached idle timeout, shutting down...", serverID)

			// Auto-shutdown
			if err := m.mcService.StopServer(serverID, "idle"); err != nil {
				log.Printf("Error stopping server %s: %v", serverID, err)
			} else {
				log.Printf("Successfully stopped idle server %s", serverID)
				m.StopMonitoring(serverID)
			}
		} else {
			m.mu.Unlock()
		}
	}
}

// getPlayerCount attempts to get the current player count via RCON
func (m *MonitoringService) getPlayerCount(server *models.MinecraftServer) (int, error) {
	// RCON is on port 25575 by default for itzg/minecraft-server
	// We need to enable RCON in the container
	rconPort := 25575
	rconPassword := "minecraft" // Default password, should be configurable

	client, err := rcon.NewClient("localhost", rconPort, rconPassword)
	if err != nil {
		return 0, err
	}
	defer client.Close()

	return client.GetPlayerCount()
}

// updatePeakPlayerCount updates the peak player count in the usage log
func (m *MonitoringService) updatePeakPlayerCount(serverID string, playerCount int) {
	usageLog, err := m.repo.GetActiveUsageLog(serverID)
	if err != nil {
		return
	}

	if playerCount > usageLog.PlayerCountPeak {
		usageLog.PlayerCountPeak = playerCount
		m.repo.UpdateUsageLog(usageLog)
	}
}

// GetServerStatus returns real-time status for a server
func (m *MonitoringService) GetServerStatus(serverID string) *ServerStatus {
	m.mu.RLock()
	timer, exists := m.idleTimers[serverID]
	m.mu.RUnlock()

	status := &ServerStatus{
		ServerID:      serverID,
		IsMonitored:   exists,
		PlayerCount:   0,
		IdleSeconds:   0,
		TimeoutSeconds: 0,
	}

	if exists {
		status.PlayerCount = timer.LastPlayerCount
		status.IdleSeconds = int(time.Since(timer.IdleSince).Seconds())
		status.TimeoutSeconds = timer.TimeoutSeconds
	}

	return status
}

// ServerStatus represents the real-time status of a server
type ServerStatus struct {
	ServerID       string `json:"server_id"`
	IsMonitored    bool   `json:"is_monitored"`
	PlayerCount    int    `json:"player_count"`
	IdleSeconds    int    `json:"idle_seconds"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

// GetAllStatuses returns status for all monitored servers
func (m *MonitoringService) GetAllStatuses() map[string]*ServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make(map[string]*ServerStatus)
	for serverID := range m.idleTimers {
		statuses[serverID] = m.GetServerStatus(serverID)
	}

	return statuses
}

// EnableAutoShutdown enables auto-shutdown for a server
func (m *MonitoringService) EnableAutoShutdown(serverID string) error {
	server, err := m.repo.FindByID(serverID)
	if err != nil {
		return err
	}

	server.AutoShutdownEnabled = true
	if err := m.repo.Update(server); err != nil {
		return err
	}

	// Start monitoring if server is running
	if server.Status == models.StatusRunning {
		m.StartMonitoring(serverID)
	}

	return nil
}

// DisableAutoShutdown disables auto-shutdown for a server
func (m *MonitoringService) DisableAutoShutdown(serverID string) error {
	server, err := m.repo.FindByID(serverID)
	if err != nil {
		return err
	}

	server.AutoShutdownEnabled = false
	if err := m.repo.Update(server); err != nil {
		return err
	}

	// Stop monitoring
	m.StopMonitoring(serverID)

	return nil
}
