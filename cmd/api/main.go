package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/payperplay/hosting/internal/api"
	"github.com/payperplay/hosting/internal/docker"
	"github.com/payperplay/hosting/internal/middleware"
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

	// Initialize services
	authService := service.NewAuthService(userRepo, cfg)
	mcService := service.NewMinecraftService(serverRepo, dockerService, cfg)
	monitoringService := service.NewMonitoringService(mcService, serverRepo, cfg)

	// Link auth service to middleware
	middleware.SetAuthService(authService)
	backupService, err := service.NewBackupService(serverRepo, cfg)
	if err != nil {
		logger.Fatal("Failed to initialize backup service", err, nil)
	}
	pluginService := service.NewPluginService(serverRepo, cfg)
	fileManagerService := service.NewFileManagerService(serverRepo, cfg)

	// Initialize WebSocket Hub
	wsHub := websocket.NewHub()
	go wsHub.Run()
	logger.Info("WebSocket hub started", nil)

	// Link WebSocket Hub to services for real-time updates
	mcService.SetWebSocketHub(wsHub)

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

	// Initialize API handlers
	authHandler := api.NewAuthHandler(authService)
	handler := api.NewHandler(mcService)
	monitoringHandler := api.NewMonitoringHandler(monitoringService)
	backupHandler := api.NewBackupHandler(backupService)
	pluginHandler := api.NewPluginHandler(pluginService)
	velocityHandler := api.NewVelocityHandler(velocityService, mcService)
	wsHandler := api.NewWebSocketHandler(wsHub)
	fileManagerHandler := api.NewFileManagerHandler(fileManagerService)

	// Setup router
	router := api.SetupRouter(authHandler, handler, monitoringHandler, backupHandler, pluginHandler, velocityHandler, wsHandler, fileManagerHandler, cfg)

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
