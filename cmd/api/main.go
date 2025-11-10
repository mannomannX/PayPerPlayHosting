package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/payperplay/hosting/internal/api"
	"github.com/payperplay/hosting/internal/conductor"
	"github.com/payperplay/hosting/internal/docker"
	"github.com/payperplay/hosting/internal/middleware"
	"github.com/payperplay/hosting/internal/monitoring"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/internal/service"
	"github.com/payperplay/hosting/internal/velocity"
	"github.com/payperplay/hosting/internal/websocket"
	"github.com/payperplay/hosting/pkg/config"
	"github.com/payperplay/hosting/pkg/logger"
)

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

	// Initialize Docker service
	dockerService, err := docker.NewDockerService(cfg)
	if err != nil {
		logger.Fatal("Failed to initialize Docker service", err, nil)
	}
	defer dockerService.Close()
	logger.Info("Docker service initialized", nil)

	// Initialize repositories
	db := repository.GetDB()
	serverRepo := repository.NewServerRepository(db)
	userRepo := repository.NewUserRepository(db)
	configChangeRepo := repository.NewConfigChangeRepository(db)
	fileRepo := repository.NewFileRepository(db)

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
	logger.Info("Billing service initialized", nil)

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

	// Link Billing Service to MinecraftService for cost tracking
	mcService.SetBillingService(billingService)

	// Link Billing Service to LifecycleService for phase change tracking
	lifecycleService.SetBillingService(billingService)

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

	// Start monitoring service
	monitoringService.Start()
	defer monitoringService.Stop()
	logger.Info("Monitoring service started", nil)

	// Initialize Prometheus metrics exporter
	prometheusExporter := monitoring.NewPrometheusExporter(serverRepo, dockerService.GetClient())
	prometheusExporter.StartMetricsCollector(30 * time.Second) // Collect metrics every 30 seconds
	logger.Info("Prometheus metrics exporter started", nil)

	// Initialize Conductor Core for fleet orchestration
	cond := conductor.NewConductor(60 * time.Second) // Health check every 60 seconds
	cond.Start()
	defer cond.Stop()
	logger.Info("Conductor Core initialized", nil)

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

	// Bulk operations handler for multi-server management
	bulkHandler := api.NewBulkHandler(mcService, backupService)

	// Setup router
	router := api.SetupRouter(authHandler, oauthHandler, handler, monitoringHandler, backupHandler, pluginHandler, velocityHandler, wsHandler, fileManagerHandler, consoleHandler, configHandler, fileHandler, motdHandler, metricsHandler, playerHandler, worldHandler, templateHandler, webhookHandler, backupScheduleHandler, prometheusHandler, conductorHandler, billingHandler, bulkHandler, cfg)

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		logger.Info("Shutting down gracefully...", nil)
		// Leave servers running - they will be managed by auto-shutdown
		// This allows maintenance without disrupting active servers
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
