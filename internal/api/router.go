package api

import (
	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/middleware"
	"github.com/payperplay/hosting/internal/repository"
	"github.com/payperplay/hosting/pkg/config"
)

func SetupRouter(
	authHandler *AuthHandler,
	oauthHandler *OAuthHandler,
	handler *Handler,
	monitoringHandler *MonitoringHandler,
	backupHandler *BackupHandler,
	pluginHandler *PluginHandler,
	velocityHandler *VelocityHandler,
	wsHandler *WebSocketHandler,
	fileManagerHandler *FileManagerHandler,
	consoleHandler *ConsoleHandler,
	configHandler *ConfigHandler,
	fileHandler *FileHandler,
	motdHandler *MOTDHandler,
	metricsHandler *MetricsHandler,
	playerHandler *PlayerHandler,
	worldHandler *WorldHandler,
	templateHandler *TemplateHandler,
	webhookHandler *WebhookHandler,
	backupScheduleHandler *BackupScheduleHandler,
	prometheusHandler *PrometheusHandler,
	conductorHandler *ConductorHandler,
	billingHandler *BillingHandler,
	bulkHandler *BulkHandler,
	marketplaceHandler *MarketplaceHandler,
	scalingHandler *ScalingHandler,
	dashboardWsHandler *DashboardWebSocket,
	cfg *config.Config,
) *gin.Engine {
	// Set Gin mode
	if !cfg.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router with custom middleware
	router := gin.New()

	// Global middleware (in order)
	router.Use(gin.Recovery())                     // Panic recovery
	router.Use(middleware.ErrorHandler())          // Error handling
	router.Use(middleware.RequestLogger())         // Request logging
	router.Use(middleware.RateLimitMiddleware(middleware.GlobalRateLimiter)) // Global rate limiting

	// CORS middleware (for development)
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Health check endpoints (no auth required)
	dbProvider := repository.GetDBProvider()
	healthHandler := NewHealthHandler(dbProvider)
	router.GET("/health", healthHandler.HealthCheck)
	router.HEAD("/health", healthHandler.HealthCheck)  // Docker healthcheck uses HEAD
	router.GET("/ready", healthHandler.ReadinessCheck)
	router.GET("/live", healthHandler.LivenessCheck)
	router.GET("/metrics", healthHandler.MetricsCheck)

	// Prometheus metrics endpoint (no auth required for scraping)
	router.GET("/prometheus", prometheusHandler.MetricsEndpoint)

	// Conductor API endpoints (no auth required for internal monitoring)
	conductor := router.Group("/conductor")
	{
		conductor.GET("/status", conductorHandler.GetStatus)
		conductor.GET("/fleet", conductorHandler.GetFleetStats)
		conductor.GET("/nodes", conductorHandler.GetNodes)
		conductor.GET("/containers", conductorHandler.GetContainers)
	}

	// WebSocket endpoint (no auth required for MVP)
	router.GET("/ws", wsHandler.HandleWebSocket)
	router.GET("/api/ws/stats", wsHandler.GetStats)

	// Auth endpoints (no auth required, but with strict rate limiting)
	auth := router.Group("/api/auth")
	auth.Use(middleware.RateLimitMiddleware(middleware.AuthRateLimiter))  // Strict auth rate limiting
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.POST("/refresh", authHandler.RefreshToken)
		auth.POST("/logout", authHandler.Logout)

		// Email verification (no auth required)
		auth.POST("/verify-email", authHandler.VerifyEmail)
		auth.POST("/resend-verification", authHandler.ResendVerificationEmail)

		// Password reset (no auth required)
		auth.POST("/request-reset", authHandler.RequestPasswordReset)
		auth.POST("/reset-password", authHandler.ResetPassword)

		// OAuth endpoints (no auth required)
		auth.GET("/oauth/discord", oauthHandler.DiscordLogin)
		auth.GET("/oauth/discord/callback", oauthHandler.DiscordCallback)
		auth.GET("/oauth/google", oauthHandler.GoogleLogin)
		auth.GET("/oauth/google/callback", oauthHandler.GoogleCallback)
		auth.GET("/oauth/github", oauthHandler.GitHubLogin)
		auth.GET("/oauth/github/callback", oauthHandler.GitHubCallback)

		// Protected auth routes (require authentication)
		auth.GET("/profile", middleware.AuthMiddleware(), authHandler.GetProfile)
		auth.PUT("/profile", middleware.AuthMiddleware(), authHandler.UpdateProfile)
		auth.POST("/change-password", middleware.AuthMiddleware(), authHandler.ChangePassword)
		auth.DELETE("/account", middleware.AuthMiddleware(), authHandler.DeleteAccount)
	}

	// API routes (with auth and API-specific rate limiting)
	api := router.Group("/api")
	api.Use(middleware.AuthMiddleware())                                // Auth with JWT
	api.Use(middleware.RateLimitMiddleware(middleware.APIRateLimiter))  // API rate limiting
	{
		// Server Templates (public within auth)
		templates := api.Group("/templates")
		{
			templates.GET("", templateHandler.GetAllTemplates)
			templates.GET("/popular", templateHandler.GetPopularTemplates)
			templates.GET("/categories", templateHandler.GetCategories)
			templates.GET("/category/:category", templateHandler.GetTemplatesByCategory)
			templates.GET("/search", templateHandler.SearchTemplates)
			templates.GET("/recommendations", templateHandler.GetRecommendations)
			templates.GET("/:id", templateHandler.GetTemplate)
		}

		// Server management
		servers := api.Group("/servers")
		{
			servers.POST("", handler.CreateServer)
			servers.GET("", handler.ListServers)
			servers.GET("/:id", handler.GetServer)
			servers.POST("/:id/start", handler.StartServer)
			servers.POST("/:id/stop", handler.StopServer)
			servers.DELETE("/:id", handler.DeleteServer)
			servers.GET("/:id/usage", handler.GetServerUsage)
			servers.GET("/:id/logs", handler.GetServerLogs)
			servers.POST("/:id/apply-template", templateHandler.ApplyTemplate)

			// Monitoring
			servers.GET("/:id/status", monitoringHandler.GetServerStatus)
			servers.POST("/:id/auto-shutdown/enable", monitoringHandler.EnableAutoShutdown)
			servers.POST("/:id/auto-shutdown/disable", monitoringHandler.DisableAutoShutdown)

			// Backups (with stricter rate limiting for expensive operations)
			backups := servers.Group("/:id/backups")
			backups.Use(middleware.RateLimitMiddleware(middleware.ExpensiveRateLimiter))
			{
				backups.POST("", backupHandler.CreateBackup)
				backups.GET("", backupHandler.ListBackups)
				backups.POST("/restore", backupHandler.RestoreBackup)
				backups.DELETE("/:filename", backupHandler.DeleteBackup)
			}

			// Plugins
			servers.POST("/:id/plugins", pluginHandler.InstallPlugin)
			servers.GET("/:id/plugins", pluginHandler.ListPlugins)
			servers.DELETE("/:id/plugins/:filename", pluginHandler.RemovePlugin)

			// Mod packs
			servers.POST("/:id/modpack", pluginHandler.InstallModPack)

			// File Manager (server.properties, configs, etc.)
			servers.GET("/:id/files", fileManagerHandler.GetAllowedFiles)
			servers.GET("/:id/files/read", fileManagerHandler.ReadFile)
			servers.POST("/:id/files/write", fileManagerHandler.WriteFile)
			servers.GET("/:id/files/list", fileManagerHandler.ListFiles)

			// Uploaded Files (resource packs, data packs, icons, world gen)
			uploads := servers.Group("/:id/uploads")
			uploads.Use(middleware.RateLimitMiddleware(middleware.FileUploadRateLimiter))
			{
				uploads.POST("", fileHandler.UploadFile)
				uploads.GET("", fileHandler.ListFiles)
				uploads.GET("/:fileId", fileHandler.GetFile)
				uploads.PUT("/:fileId/activate", fileHandler.ActivateFile)
				uploads.PUT("/:fileId/deactivate", fileHandler.DeactivateFile)
				uploads.DELETE("/:fileId", fileHandler.DeleteFile)
			}

			// Console Access (WebSocket for real-time logs and command execution)
			servers.GET("/:id/console/stream", consoleHandler.HandleConsoleWebSocket)
			servers.GET("/:id/console/logs", consoleHandler.GetConsoleLogs)
			servers.POST("/:id/console/command", consoleHandler.ExecuteConsoleCommand)

			// Configuration Management
			servers.POST("/:id/config", configHandler.ApplyConfigChanges)
			servers.GET("/:id/config/history", configHandler.GetConfigHistory)

			// MOTD (Message of the Day)
			servers.GET("/:id/motd", motdHandler.GetMOTD)
			servers.PUT("/:id/motd", motdHandler.UpdateMOTD)

			// Server Icon (publicly accessible for display)
			servers.GET("/:id/icon", fileHandler.GetServerIcon)

			// Player Management (Whitelist, Ops, Banned)
			servers.GET("/:id/players/:listType", playerHandler.GetPlayerList)
			servers.POST("/:id/players/:listType/add", playerHandler.AddToPlayerList)
			servers.DELETE("/:id/players/:listType/:username", playerHandler.RemoveFromPlayerList)

			// Online & Historic Players
			servers.GET("/:id/players-online", playerHandler.GetOnlinePlayers)
			servers.GET("/:id/players-history", playerHandler.GetHistoricPlayers)

			// World Management
			servers.GET("/:id/worlds", worldHandler.ListWorlds)
			servers.GET("/:id/worlds/:name/download", worldHandler.DownloadWorld)
			servers.POST("/:id/worlds/upload", worldHandler.UploadWorld)
			servers.POST("/:id/worlds/:name/reset", worldHandler.ResetWorld)
			servers.DELETE("/:id/worlds/:name", worldHandler.DeleteWorld)

			// Cost Analytics & Billing
			servers.GET("/:id/costs", billingHandler.GetServerCosts)
			servers.GET("/:id/billing/events", billingHandler.GetBillingEvents)
			servers.GET("/:id/billing/sessions", billingHandler.GetUsageSessions)

			// Discord Webhooks
			servers.GET("/:id/webhook", webhookHandler.GetWebhook)
			servers.POST("/:id/webhook", webhookHandler.CreateWebhook)
			servers.PUT("/:id/webhook", webhookHandler.UpdateWebhook)
			servers.DELETE("/:id/webhook", webhookHandler.DeleteWebhook)
			servers.POST("/:id/webhook/test", webhookHandler.TestWebhook)

			// Backup Schedules
			servers.GET("/:id/backup-schedule", backupScheduleHandler.GetSchedule)
			servers.POST("/:id/backup-schedule", backupScheduleHandler.CreateSchedule)
			servers.PUT("/:id/backup-schedule", backupScheduleHandler.UpdateSchedule)
			servers.DELETE("/:id/backup-schedule", backupScheduleHandler.DeleteSchedule)

			// Plugin Marketplace (new marketplace system)
			servers.GET("/:id/marketplace/plugins", marketplaceHandler.ListInstalledPlugins)
			servers.POST("/:id/marketplace/plugins", marketplaceHandler.InstallPlugin)
			servers.DELETE("/:id/marketplace/plugins/:plugin_id", marketplaceHandler.UninstallPlugin)
			servers.GET("/:id/marketplace/updates", marketplaceHandler.CheckForUpdates)
			servers.PUT("/:id/marketplace/plugins/:plugin_id", marketplaceHandler.UpdatePlugin)
			servers.POST("/:id/marketplace/auto-update", marketplaceHandler.AutoUpdatePlugins)
			servers.POST("/:id/marketplace/plugins/:plugin_id/toggle", marketplaceHandler.TogglePlugin)
			servers.POST("/:id/marketplace/plugins/:plugin_id/auto-update", marketplaceHandler.ToggleAutoUpdate)

			// Bulk Operations (multi-server management)
			bulk := servers.Group("/bulk")
			{
				bulk.POST("/start", bulkHandler.BulkStartServers)
				bulk.POST("/stop", bulkHandler.BulkStopServers)
				bulk.POST("/delete", bulkHandler.BulkDeleteServers)
				bulk.POST("/backup", bulkHandler.BulkBackupServers)
			}
		}

		// Dashboard WebSocket (public - read-only visualization)
		api.GET("/admin/dashboard/stream", dashboardWsHandler.HandleConnection)

		// Admin endpoints
		admin := api.Group("/admin")
		{
			admin.GET("/servers", handler.ListAllServers)             // List ALL servers
			admin.POST("/cleanup", handler.CleanOrphanedServers)      // Clean orphaned servers
		}

		// Global monitoring
		api.GET("/monitoring/status", monitoringHandler.GetAllStatuses)

		// Plugin/Mod marketplace
		api.GET("/plugins/search", pluginHandler.SearchPlugins)
		api.GET("/modpacks/search", pluginHandler.SearchModPacks)

		// Metrics
		metrics := api.Group("/metrics")
		{
			metrics.GET("/files", metricsHandler.GetFileMetrics)
			metrics.POST("/files/reset", metricsHandler.ResetFileMetrics) // Admin only
		}

		// Billing (owner-level costs)
		billing := api.Group("/billing")
		{
			billing.GET("/costs", billingHandler.GetOwnerCosts)
		}

		// Plugin Marketplace (browsing and discovery)
		marketplace := api.Group("/marketplace")
		{
			marketplace.GET("/plugins", marketplaceHandler.ListMarketplacePlugins)
			marketplace.GET("/search", marketplaceHandler.SearchMarketplace)
			marketplace.GET("/plugins/:slug", marketplaceHandler.GetPluginDetails)
		}

		// Admin marketplace management
		admin.POST("/marketplace/sync", marketplaceHandler.SyncMarketplace)
		admin.POST("/marketplace/plugins/:slug/sync", marketplaceHandler.SyncPlugin)

		// Scaling API (B5 Auto-Scaling + B8 Cost Optimization) - Admin only
		scaling := api.Group("/scaling")
		{
			scaling.GET("/status", scalingHandler.GetScalingStatus)
			scaling.POST("/enable", scalingHandler.EnableScaling)
			scaling.POST("/disable", scalingHandler.DisableScaling)
			scaling.GET("/history", scalingHandler.GetScalingHistory)
			scaling.POST("/optimize-costs", scalingHandler.OptimizeCosts) // B8: Manual cost optimization trigger
		}
	}

	// Internal API (for Velocity plugin - NO AUTH required, network isolation)
	internal := router.Group("/api/internal")
	{
		internal.POST("/servers/:id/wakeup", velocityHandler.WakeupServer)
		internal.GET("/servers/:id/status", velocityHandler.GetServerStatus)
		internal.POST("/velocity/reload", velocityHandler.ReloadVelocity)
		internal.GET("/velocity/servers", velocityHandler.GetVelocityServers)
	}

	// Public Velocity management endpoints (with auth)
	velocity := api.Group("/velocity")
	{
		velocity.GET("/status", velocityHandler.GetVelocityStatus)
		velocity.POST("/start", velocityHandler.StartVelocity)
		velocity.POST("/stop", velocityHandler.StopVelocity)
	}

	// Serve static files and frontend (we'll add this later)
	router.Static("/static", "./web/static")
	router.LoadHTMLGlob("web/templates/*")

	router.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", gin.H{
			"title": "PayPerPlay - Minecraft Hosting",
		})
	})

	return router
}
