package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/payperplay/hosting/internal/api"
	"github.com/payperplay/hosting/internal/cloud"
	"github.com/payperplay/hosting/internal/conductor"
	"github.com/payperplay/hosting/internal/docker"
	"github.com/payperplay/hosting/internal/events"
	"github.com/payperplay/hosting/internal/middleware"
	"github.com/payperplay/hosting/internal/models"
	"github.com/payperplay/hosting/internal/monitoring"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/internal/service"
	"github.com/payperplay/hosting/internal/storage"
	"github.com/payperplay/hosting/internal/velocity"
	"github.com/payperplay/hosting/internal/websocket"
	"github.com/payperplay/hosting/pkg/config"
	"github.com/payperplay/hosting/pkg/logger"
)

// conductorAdapter adapts conductor.Conductor to velocity.ConductorInterface
type conductorAdapter struct {
	*conductor.Conductor
}

// GetRemoteNode adapts Conductor's GetRemoteNode to return velocity.RemoteNodeGetter
func (c *conductorAdapter) GetRemoteNode(nodeID string) (velocity.RemoteNodeGetter, error) {
	return c.Conductor.GetRemoteNode(nodeID)
}

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize logger
	logLevel := parseLogLevel(cfg.LogLevel)
	appLogger := logger.NewLogger(logLevel, os.Stdout, cfg.LogJSON)
	logger.SetDefault(appLogger)

	logger.Info("Starting application", map[string]interface{}{
		"app":   cfg.AppName,
		"debug": cfg.Debug,
		"port":  cfg.Port,
	})

	// Initialize database
	if err := repository.InitDB(cfg); err != nil {
		logger.Fatal("Failed to initialize database", err, nil)
	}
	logger.Info("Database initialized", nil)

	// Initialize Event-Bus with multi-storage (PostgreSQL + InfluxDB)
	db := repository.GetDB()
	dbStorage := events.NewDatabaseEventStorage(db)

	// Try to initialize InfluxDB if configured
	var eventStorage events.EventStorage = dbStorage
	if cfg.InfluxDBURL != "" && cfg.InfluxDBToken != "" {
		influxConfig := storage.InfluxDBConfig{
			URL:    cfg.InfluxDBURL,
			Token:  cfg.InfluxDBToken,
			Org:    cfg.InfluxDBOrg,
			Bucket: cfg.InfluxDBBucket,
		}

		influxClient, err := storage.NewInfluxDBClient(influxConfig)
		if err != nil {
			logger.Warn("Failed to initialize InfluxDB, falling back to database-only storage", map[string]interface{}{
				"error": err.Error(),
			})
		} else {
			defer influxClient.Close()
			influxStorage := events.NewInfluxDBEventStorage(influxClient)
			eventStorage = events.NewMultiEventStorage(dbStorage, influxStorage)
			logger.Info("Event-Bus initialized with dual storage (PostgreSQL + InfluxDB)", map[string]interface{}{
				"influxdb_url": cfg.InfluxDBURL,
				"org":          cfg.InfluxDBOrg,
				"bucket":       cfg.InfluxDBBucket,
			})
		}
	} else {
		logger.Info("Event-Bus initialized with database storage only", nil)
	}

	events.SetEventStorage(eventStorage)

	// Initialize Docker service
	dockerService, err := docker.NewDockerService(cfg)
	if err != nil {
		logger.Fatal("Failed to initialize Docker service", err, nil)
	}
	defer dockerService.Close()
	logger.Info("Docker service initialized", nil)

	// Initialize repositories
	serverRepo := repository.NewServerRepository(db)
	userRepo := repository.NewUserRepository(db)
	configChangeRepo := repository.NewConfigChangeRepository(db)
	fileRepo := repository.NewFileRepository(db)
	pluginRepo := repository.NewPluginRepository(db)
	migrationRepo := repository.NewMigrationRepository(db)

	// Initialize Email Service (using mock sender for now)
	// 🚧 TODO: Replace MockEmailSender with ResendEmailSender when ready for production
	mockEmailSender := service.NewMockEmailSender(db)
	emailService := service.NewEmailService(mockEmailSender, db)
	logger.Info("Email service initialized (🚧 MOCK MODE)", nil)

	// Initialize Security Service for device trust and security events
	securityService := service.NewSecurityService(db, emailService)
	logger.Info("Security service initialized", nil)

	// Initialize services
	authService := service.NewAuthService(userRepo, cfg, emailService, securityService)
	oauthService := service.NewOAuthService(db, userRepo, cfg, securityService, emailService)
	logger.Info("OAuth service initialized", nil)

	mcService := service.NewMinecraftService(serverRepo, dockerService, cfg)
	monitoringService := service.NewMonitoringService(mcService, serverRepo, cfg)

	// Initialize Recovery Service for automatic crash handling
	recoveryService := service.NewRecoveryService(serverRepo, dockerService, cfg)
	recoveryService.Start()
	defer recoveryService.Stop()
	logger.Info("Recovery service started", nil)

	// Note: Orphaned server cleanup is NOT run on startup to avoid race conditions
	// during container restarts. The monitoring service handles cleanup periodically.

	// Link auth service to middleware
	middleware.SetAuthService(authService)
	backupService, err := service.NewBackupService(serverRepo, cfg)
	if err != nil {
		logger.Fatal("Failed to initialize backup service", err, nil)
	}

	// Initialize Backup Scheduler for automated backups
	backupScheduler := service.NewBackupScheduler(db, backupService, serverRepo)
	backupScheduler.Start()
	defer backupScheduler.Stop()
	logger.Info("Backup scheduler started", nil)

	// Initialize Lifecycle Service for 3-phase lifecycle management
	lifecycleService := service.NewLifecycleService(db, serverRepo)
	lifecycleService.Start()
	defer lifecycleService.Stop()
	logger.Info("Lifecycle service started", nil)

	// Initialize Billing Service for cost analytics
	billingService := service.NewBillingService(db, serverRepo)
	billingService.Start() // Subscribe to Event-Bus for automatic billing tracking
	defer billingService.Stop()
	logger.Info("Billing service initialized and subscribed to Event-Bus", nil)

	// Initialize Plugin Marketplace Services
	pluginSyncService := service.NewPluginSyncService(pluginRepo)
	pluginSyncService.Start() // Start background sync worker (every 6 hours)
	defer pluginSyncService.Stop()
	logger.Info("Plugin sync service started (auto-sync from Modrinth every 6h)", nil)

	pluginManagerService := service.NewPluginManagerService(pluginRepo, serverRepo, cfg)
	logger.Info("Plugin manager service initialized", nil)

	pluginService := service.NewPluginService(serverRepo, cfg)
	fileManagerService := service.NewFileManagerService(serverRepo, cfg)
	fileService := service.NewFileService(fileRepo, serverRepo, cfg.ServersBasePath)

	// Initialize WebSocket Hub
	wsHub := websocket.NewHub()
	go wsHub.Run()
	logger.Info("WebSocket hub started", nil)

	// Link WebSocket Hub to services for real-time updates
	mcService.SetWebSocketHub(wsHub)
	recoveryService.SetWebSocketHub(wsHub)

	// Note: BillingService now automatically tracks events via Event-Bus subscription
	// No need to manually link it to services

	// Link Recovery Service to Monitoring Service for crash detection
	monitoringService.SetRecoveryService(recoveryService)

	// Initialize Velocity service
	velocityService, err := velocity.NewVelocityService(
		dockerService.GetClient(),
		serverRepo,
		cfg,
	)
	if err != nil {
		logger.Fatal("Failed to initialize Velocity service", err, nil)
	}

	// Link Velocity to MinecraftService (avoid circular dependency)
	mcService.SetVelocityService(velocityService)

	// Start Velocity proxy
	if err := velocityService.Start(); err != nil {
		logger.Warn("Failed to start Velocity proxy", map[string]interface{}{
			"error": err.Error(),
		})
	} else {
		logger.Info("Velocity proxy started", map[string]interface{}{
			"port": "25565",
		})
	}
	defer velocityService.Stop()

	// VELOCITY REMOTE API: Initialize HTTP client for remote Velocity proxy (NEW 3-tier architecture)
	var remoteVelocityClient *velocity.RemoteVelocityClient
	var velocityMonitor *velocity.VelocityMonitor
	if cfg.VelocityAPIURL != "" {
		remoteVelocityClient = velocity.NewRemoteVelocityClient(cfg.VelocityAPIURL)

		// Link Remote Velocity client to MinecraftService for automatic server registration
		mcService.SetRemoteVelocityClient(remoteVelocityClient)
		logger.Info("Remote Velocity client initialized and linked to MinecraftService", map[string]interface{}{
			"url": cfg.VelocityAPIURL,
		})

		// Initialize Velocity monitor for health checking and auto-recovery
		velocityMonitor = velocity.NewVelocityMonitor(remoteVelocityClient, serverRepo, cfg)
		logger.Info("Velocity monitor initialized", nil)

		// Initialize Player Count tracking service for accurate auto-shutdown
		playerCountService := service.NewPlayerCountService(remoteVelocityClient, serverRepo)
		playerCountService.Start()
		defer playerCountService.Stop()
		logger.Info("Player count tracking service started (Velocity-based)", map[string]interface{}{
			"check_interval": "15s",
		})
	} else {
		logger.Warn("VELOCITY_API_URL not configured, remote Velocity integration disabled", nil)
	}

	// Start monitoring service (auto-shutdown based on player counts)
	monitoringService.Start()
	defer monitoringService.Stop()
	logger.Info("Monitoring service started", nil)

	// Initialize Prometheus metrics exporter
	prometheusExporter := monitoring.NewPrometheusExporter(serverRepo, dockerService.GetClient())
	prometheusExporter.StartMetricsCollector(30 * time.Second) // Collect metrics every 30 seconds
	logger.Info("Prometheus metrics exporter started", nil)

	// Initialize Conductor Core for fleet orchestration
	cond := conductor.NewConductor(10*time.Second, cfg.SSHPrivateKeyPath) // Health check every 10 seconds for real-time dashboard updates

	// Initialize Scaling Engine (B5 + B8) if Hetzner Cloud token is configured
	if cfg.HetznerCloudToken != "" {
		hetznerProvider := cloud.NewHetznerProvider(cfg.HetznerCloudToken)
		cond.InitializeScaling(hetznerProvider, cfg.HetznerSSHKeyName, cfg.ScalingEnabled, remoteVelocityClient)
		logger.Info("Scaling engine initialized", map[string]interface{}{
			"ssh_key": cfg.HetznerSSHKeyName,
			"enabled": cfg.ScalingEnabled,
			"consolidation_enabled": remoteVelocityClient != nil && cfg.CostOptimizationEnabled,
		})
	} else {
		logger.Warn("Hetzner Cloud token not configured, scaling disabled", nil)
	}

	// Link Conductor to MinecraftService for capacity management
	mcService.SetConductor(cond)
	logger.Info("Conductor linked to MinecraftService for resource guard", nil)

	// Link MinecraftService to Conductor as ServerStarter for queue processing
	cond.SetServerStarter(mcService)
	logger.Info("MinecraftService linked to Conductor as ServerStarter for queue processing", nil)

	// Link ServerRepo to Conductor for ghost container cleanup
	cond.SetServerRepo(serverRepo)
	logger.Info("ServerRepo linked to Conductor for ghost container cleanup (1-minute intervals)", nil)

	cond.Start()
	defer cond.Stop()
	logger.Info("Conductor Core started", nil)

	// Link Velocity Monitor to Conductor and start monitoring
	if velocityMonitor != nil {
		// Create adapter to bridge conductor and velocity interfaces
		velocityMonitor.SetConductor(&conductorAdapter{cond})
		velocityMonitor.Start()
		defer velocityMonitor.Stop()
		logger.Info("Velocity monitor started", nil)
	}

	// Initialize Cost-Optimization Service for automatic server placement optimization
	costOptimizationService := service.NewCostOptimizationService(serverRepo, migrationRepo)
	costOptimizationService.SetConductor(cond)
	costOptimizationService.Start()
	defer costOptimizationService.Stop()
	logger.Info("Cost optimization service started", map[string]interface{}{
		"check_interval":    "2h",
		"min_savings":       "€0.10/hour",
		"scaling_cooldown":  "2h",
	})

	// Initialize Migration Service for live server migrations
	migrationService := service.NewMigrationService(migrationRepo, serverRepo, dockerService)
	migrationService.SetConductor(cond)
	migrationService.SetWebSocketHub(wsHub)
	if remoteVelocityClient != nil {
		migrationService.SetRemoteVelocityClient(remoteVelocityClient)
	}
	migrationService.StartMigrationWorker()
	logger.Info("Migration service started", map[string]interface{}{
		"check_interval": "30s",
		"enabled":        true,
	})

	// CRITICAL: Sync running containers with Conductor state (prevents OOM after restarts)
	logger.Info("Syncing running containers with Conductor state...", nil)
	cond.SyncRunningContainers(dockerService, serverRepo)
	logger.Info("Container state sync completed", nil)

	// CRITICAL: Sync queued servers from database into StartQueue (prevents queue loss after restarts)
	logger.Info("Syncing queued servers into StartQueue...", nil)
	cond.SyncQueuedServers(serverRepo, false) // Don't trigger scaling yet
	logger.Info("Queue sync completed", nil)

	// CRITICAL: Restore worker nodes from persisted state file FIRST
	// This prevents data loss by restoring nodes that existed before restart
	// These nodes get a recovery grace period to prevent immediate scale-down
	if cond.CloudProvider != nil {
		nodeStateFile := filepath.Join(cfg.DataDir, "node_state.json")
		logger.Info("Restoring nodes from persisted state...", map[string]interface{}{
			"state_file": nodeStateFile,
		})
		if err := cond.RestoreNodesFromState(nodeStateFile); err != nil {
			logger.Error("Failed to restore nodes from state", err, nil)
			// Continue anyway - will fall back to Hetzner sync
		}

		// CRITICAL: Sync with Hetzner Cloud API to discover any new nodes
		// This is secondary to state file restoration
		logger.Info("Syncing with Hetzner Cloud API...", nil)
		cond.SyncExistingWorkerNodes(false) // Don't trigger scaling yet
		logger.Info("Worker node sync completed", nil)

		// CRITICAL: Sync running containers from remote worker nodes
		// This must happen immediately after node restoration to prevent capacity errors
		logger.Info("Syncing containers from remote worker nodes...", nil)
		cond.SyncRemoteNodeContainers(serverRepo)
		logger.Info("Remote container sync completed", nil)
	}

	// CRITICAL: Re-register all running servers with Velocity after restart
	// This prevents "server not found" errors when backend restarts while servers are running
	if remoteVelocityClient != nil {
		logger.Info("Re-registering running servers with Velocity...", nil)
		runningServers, err := serverRepo.FindByStatus(string(models.StatusRunning))
		if err != nil {
			logger.Error("Failed to find running servers for Velocity sync", err, nil)
		} else {
			registered := 0
			for _, server := range runningServers {
				if server.NodeID == "" {
					logger.Warn("Skipping Velocity registration for server without node assignment", map[string]interface{}{
						"server_id": server.ID,
						"name":      server.Name,
					})
					continue
				}

				velocityServerName := fmt.Sprintf("mc-%s", server.ID)

				// Get node IP
				var serverIP string
				if server.NodeID == "local-node" {
					serverIP = cfg.ControlPlaneIP
				} else {
					remoteNode, err := cond.GetRemoteNode(server.NodeID)
					if err != nil {
						logger.Warn("Failed to get node IP for Velocity registration", map[string]interface{}{
							"server_id": server.ID,
							"node_id":   server.NodeID,
							"error":     err.Error(),
						})
						continue
					}
					serverIP = remoteNode.IPAddress
				}

				serverAddress := fmt.Sprintf("%s:%d", serverIP, server.Port)

				if err := remoteVelocityClient.RegisterServer(velocityServerName, serverAddress); err != nil {
					logger.Warn("Failed to re-register server with Velocity", map[string]interface{}{
						"server_id": server.ID,
						"error":     err.Error(),
					})
				} else {
					registered++
					logger.Debug("Server re-registered with Velocity", map[string]interface{}{
						"server_id": server.ID,
						"name":      velocityServerName,
						"address":   serverAddress,
					})
				}
			}
			logger.Info("Velocity state sync completed", map[string]interface{}{
				"total_running": len(runningServers),
				"registered":    registered,
			})
		}
	}

	// NOTE: No immediate scaling check after startup to prevent race conditions
	// The Scaling Engine will run normally (every 2 minutes)

	// Initialize API handlers
	authHandler := api.NewAuthHandler(authService)
	oauthHandler := api.NewOAuthHandler(oauthService)
	handler := api.NewHandler(mcService)
	monitoringHandler := api.NewMonitoringHandler(monitoringService)
	backupHandler := api.NewBackupHandler(backupService)
	pluginHandler := api.NewPluginHandler(pluginService)
	velocityHandler := api.NewVelocityHandler(velocityService, mcService)
	wsHandler := api.NewWebSocketHandler(wsHub)
	fileManagerHandler := api.NewFileManagerHandler(fileManagerService)

	// Console service for real-time logs and command execution
	consoleService := service.NewConsoleService(serverRepo, dockerService)
	consoleHandler := api.NewConsoleHandler(consoleService)

	// MOTD (Message of the Day) service
	motdService := service.NewMOTDService(serverRepo, cfg)
	motdHandler := api.NewMOTDHandler(motdService)

	// Configuration service for server configuration changes (needs motdService)
	configService := service.NewConfigService(serverRepo, configChangeRepo, dockerService, backupService, motdService)
	configHandler := api.NewConfigHandler(configService, mcService)

	// Resource pack integration service
	resourcePackService := service.NewResourcePackService(fileRepo, serverRepo, cfg)

	// File integration service (handles all file types: resource packs, data packs, icons, world gen)
	fileIntegrationService := service.NewFileIntegrationService(fileRepo, serverRepo, resourcePackService, cfg)

	// File management handler for resource packs, data packs, etc.
	fileHandler := api.NewFileHandler(fileService, fileIntegrationService)

	// Metrics handler
	metricsHandler := api.NewMetricsHandler()

	// Player list service for whitelist, ops, banned players
	playerListService := service.NewPlayerListService(serverRepo, consoleService, cfg)
	playerHandler := api.NewPlayerHandler(playerListService)

	// World management service
	worldService := service.NewWorldService(serverRepo, backupService, cfg)
	worldHandler := api.NewWorldHandler(worldService)

	// Template service
	templateService, err := service.NewTemplateService("templates/server-templates.json")
	if err != nil {
		logger.Fatal("Failed to initialize template service", err, nil)
	}
	templateHandler := api.NewTemplateHandler(templateService)

	// Webhook service
	webhookService := service.NewWebhookService(db)
	webhookHandler := api.NewWebhookHandler(webhookService, serverRepo)

	// Backup schedule handler
	backupScheduleHandler := api.NewBackupScheduleHandler(backupScheduler, serverRepo)

	// Prometheus metrics handler
	prometheusHandler := api.NewPrometheusHandler()

	// Conductor handler for fleet orchestration
	conductorHandler := api.NewConductorHandler(cond)

	// Billing handler for cost analytics
	billingHandler := api.NewBillingHandler(billingService)

	// Marketplace handler for plugin marketplace
	marketplaceHandler := api.NewMarketplaceHandler(pluginManagerService, pluginSyncService)

	// Bulk operations handler for multi-server management
	bulkHandler := api.NewBulkHandler(mcService, backupService)

	// Scaling handler for auto-scaling (B5)
	scalingHandler := api.NewScalingHandler(cond)

	// Cost optimization handler for cost analysis and suggestions (B8)
	costOptHandler := api.NewCostOptimizationHandler(costOptimizationService)

	// Migration handler for server migration management
	migrationHandler := api.NewMigrationHandler(migrationRepo, serverRepo)

	// Dashboard WebSocket for real-time visualization
	dashboardWs := api.NewDashboardWebSocket(cond)
	dashboardWs.SetRepositories(migrationRepo, serverRepo) // Enable loading active migrations on reconnect
	go dashboardWs.Run()
	defer dashboardWs.Shutdown()
	logger.Info("Dashboard WebSocket started", nil)

	// Set dashboard WebSocket as global event publisher
	events.DashboardEventPublisher = dashboardWs

	// Link Dashboard WebSocket to Migration Service for real-time migration visualization
	migrationService.SetDashboardWebSocket(dashboardWs)
	logger.Info("Migration service linked to Dashboard WebSocket", nil)

	// Setup router
	router := api.SetupRouter(authHandler, oauthHandler, handler, monitoringHandler, backupHandler, pluginHandler, velocityHandler, wsHandler, fileManagerHandler, consoleHandler, configHandler, fileHandler, motdHandler, metricsHandler, playerHandler, worldHandler, templateHandler, webhookHandler, backupScheduleHandler, prometheusHandler, conductorHandler, billingHandler, bulkHandler, marketplaceHandler, scalingHandler, costOptHandler, migrationHandler, dashboardWs, cfg)

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		logger.Info("Shutting down gracefully...", nil)

		// CRITICAL: Save node state before shutdown to prevent data loss on restart
		if cond != nil && cond.CloudProvider != nil {
			nodeStateFile := filepath.Join("./data", "node_state.json")
			logger.Info("Saving node state before shutdown...", map[string]interface{}{
				"state_file": nodeStateFile,
			})
			if err := cond.SaveNodeState(nodeStateFile); err != nil {
				logger.Error("Failed to save node state", err, nil)
			} else {
				logger.Info("Node state saved successfully", nil)
			}
		}

		// Leave servers running - they will be managed by auto-shutdown
		// This allows maintenance without disrupting active servers
		logger.Info("Shutdown complete", nil)
		os.Exit(0)
	}()

	// Start server
	addr := fmt.Sprintf(":%s", cfg.Port)
	logger.Info("Server starting", map[string]interface{}{
		"address":      addr,
		"api_endpoint": fmt.Sprintf("http://localhost%s/api", addr),
		"health_check": fmt.Sprintf("http://localhost%s/health", addr),
	})

	if err := router.Run(addr); err != nil {
		logger.Fatal("Failed to start server", err, nil)
	}
}

// parseLogLevel converts a string to a logger.LogLevel
func parseLogLevel(level string) logger.LogLevel {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return logger.DEBUG
	case "INFO":
		return logger.INFO
	case "WARN":
		return logger.WARN
	case "ERROR":
		return logger.ERROR
	case "FATAL":
		return logger.FATAL
	default:
		return logger.INFO
	}
}
