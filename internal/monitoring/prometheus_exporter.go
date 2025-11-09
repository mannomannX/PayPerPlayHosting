package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/docker/docker/client"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/logger"
)

// PrometheusExporter collects and exports metrics for Prometheus
type PrometheusExporter struct {
	serverRepo   *repository.ServerRepository
	dockerClient *client.Client
}

// NewPrometheusExporter creates a new Prometheus exporter
func NewPrometheusExporter(serverRepo *repository.ServerRepository, dockerClient *client.Client) *PrometheusExporter {
	return &PrometheusExporter{
		serverRepo:   serverRepo,
		dockerClient: dockerClient,
	}
}

// CollectMetrics collects all metrics from servers
func (e *PrometheusExporter) CollectMetrics() error {
	// Get all servers
	servers, err := e.serverRepo.FindAll()
	if err != nil {
		return fmt.Errorf("failed to fetch servers: %w", err)
	}

	// Reset fleet metrics
	var totalServers, runningServers, totalPlayers int
	var totalRAMMB float64

	for _, server := range servers {
		// Update server metrics and get player count
		playerCount := e.updateServerMetrics(&server)

		// Aggregate fleet metrics
		totalServers++
		if server.Status == models.StatusRunning {
			runningServers++
			totalRAMMB += float64(server.RAMMb)
			totalPlayers += playerCount
		}
	}

	// Update fleet-wide metrics
	FleetTotalServers.Set(float64(totalServers))
	FleetRunningServers.Set(float64(runningServers))
	FleetTotalRAMMB.Set(totalRAMMB)
	FleetTotalPlayers.Set(float64(totalPlayers))

	logger.Debug("Prometheus metrics collected", map[string]interface{}{
		"total_servers":   totalServers,
		"running_servers": runningServers,
		"total_ram_mb":    totalRAMMB,
	})

	return nil
}

// updateServerMetrics updates Prometheus metrics for a single server and returns player count
func (e *PrometheusExporter) updateServerMetrics(server *models.MinecraftServer) int {
	labels := []string{server.ID, server.Name, server.MinecraftVersion}

	// Server status
	ServerStatus.WithLabelValues(labels...).Set(StatusToFloat(string(server.Status)))

	// Server uptime
	if server.Status == models.StatusRunning && server.LastStartedAt != nil {
		uptime := time.Since(*server.LastStartedAt).Seconds()
		ServerUptime.WithLabelValues(labels...).Set(uptime)
	} else {
		ServerUptime.WithLabelValues(labels...).Set(0)
	}

	// Resource allocation (static from config)
	ServerRAMUsageMB.WithLabelValues(labels...).Set(float64(server.RAMMb))

	playerCount := 0

	// Docker container stats and RCON metrics (if running)
	if server.Status == models.StatusRunning {
		e.updateContainerStats(server, labels)

		// Get RCON metrics (player count and TPS)
		if server.RCONEnabled && server.RCONPassword != "" {
			current, max := SafeGetPlayerCount("localhost", server.RCONPort, server.RCONPassword)
			if current >= 0 {
				ServerPlayerCount.WithLabelValues(labels...).Set(float64(current))
				playerCount = current
			}
			if max > 0 {
				ServerPlayerLimit.WithLabelValues(labels...).Set(float64(max))
			}

			// Get TPS (only for Paper/Spigot)
			tps := SafeGetTPS("localhost", server.RCONPort, server.RCONPassword)
			if tps > 0 {
				ServerTPS.WithLabelValues(labels...).Set(tps)
			}
		}
	} else {
		// Reset metrics for stopped servers
		ServerCPUPercent.WithLabelValues(labels...).Set(0)
		ServerPlayerCount.WithLabelValues(labels...).Set(0)
		ServerTPS.WithLabelValues(labels...).Set(0)
	}

	// Player limit (from config if RCON not available)
	if server.MaxPlayers > 0 {
		ServerPlayerLimit.WithLabelValues(labels...).Set(float64(server.MaxPlayers))
	}

	return playerCount
}

// updateContainerStats retrieves Docker container stats and updates metrics
func (e *PrometheusExporter) updateContainerStats(server *models.MinecraftServer, labels []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	containerName := fmt.Sprintf("minecraft-%s", server.ID)

	// Get container stats
	statsResponse, err := e.dockerClient.ContainerStats(ctx, containerName, false)
	if err != nil {
		logger.Debug("Failed to get container stats", map[string]interface{}{
			"server_id": server.ID,
			"error":     err.Error(),
		})
		return
	}
	defer statsResponse.Body.Close()

	// Decode stats - use a simple map for now to avoid type issues
	var stats map[string]interface{}
	if err := json.NewDecoder(statsResponse.Body).Decode(&stats); err != nil {
		logger.Debug("Failed to decode container stats", map[string]interface{}{
			"server_id": server.ID,
			"error":     err.Error(),
		})
		return
	}

	// Extract CPU stats
	if cpuStats, ok := stats["cpu_stats"].(map[string]interface{}); ok {
		if preCPUStats, ok2 := stats["precpu_stats"].(map[string]interface{}); ok2 {
			if cpuUsage, ok3 := cpuStats["cpu_usage"].(map[string]interface{}); ok3 {
				if preCPUUsage, ok4 := preCPUStats["cpu_usage"].(map[string]interface{}); ok4 {
					totalUsage := cpuUsage["total_usage"].(float64)
					preTotalUsage := preCPUUsage["total_usage"].(float64)
					systemUsage := cpuStats["system_cpu_usage"].(float64)
					preSystemUsage := preCPUStats["system_cpu_usage"].(float64)

					cpuDelta := totalUsage - preTotalUsage
					systemDelta := systemUsage - preSystemUsage

					if systemDelta > 0 && cpuDelta > 0 {
						if percpuUsage, ok5 := cpuUsage["percpu_usage"].([]interface{}); ok5 {
							cpuPercent := (cpuDelta / systemDelta) * float64(len(percpuUsage)) * 100.0
							ServerCPUPercent.WithLabelValues(labels...).Set(cpuPercent)
						}
					}
				}
			}
		}
	}

	// Extract memory stats
	if memStats, ok := stats["memory_stats"].(map[string]interface{}); ok {
		if usage, ok2 := memStats["usage"].(float64); ok2 {
			memUsageMB := usage / 1024 / 1024
			ServerRAMUsageMB.WithLabelValues(labels...).Set(memUsageMB)
		}
	}
}

// StartMetricsCollector starts a background goroutine that collects metrics periodically
func (e *PrometheusExporter) StartMetricsCollector(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		// Collect immediately on start
		if err := e.CollectMetrics(); err != nil {
			logger.Error("Failed to collect Prometheus metrics", err, nil)
		}

		for range ticker.C {
			if err := e.CollectMetrics(); err != nil {
				logger.Error("Failed to collect Prometheus metrics", err, nil)
			}
		}
	}()

	logger.Info("Prometheus metrics collector started", map[string]interface{}{
		"interval": interval.String(),
	})
}

// RecordServerStart increments the server start counter
func RecordServerStart(serverID, serverName string) {
	ServerStartTotal.WithLabelValues(serverID, serverName).Inc()
}

// RecordServerStop increments the server stop counter
func RecordServerStop(serverID, serverName string) {
	ServerStopTotal.WithLabelValues(serverID, serverName).Inc()
}

// RecordServerCrash increments the server crash counter
func RecordServerCrash(serverID, serverName string) {
	ServerCrashTotal.WithLabelValues(serverID, serverName).Inc()
}

// RecordBackupCreated increments the backup created counter
func RecordBackupCreated(serverID, serverName string) {
	BackupCreatedTotal.WithLabelValues(serverID, serverName).Inc()
}

// RecordBillingSeconds increments the billing seconds counter
func RecordBillingSeconds(serverID, serverName, phase string, seconds float64) {
	ServerBillingSecondsTotal.WithLabelValues(serverID, serverName, phase).Add(seconds)
}

// RecordAPIRequest increments the API request counter and records duration
func RecordAPIRequest(method, endpoint, status string, duration time.Duration) {
	APIRequestsTotal.WithLabelValues(method, endpoint, status).Inc()
	APIRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}
