package api

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/repository"
)

type HealthHandler struct {
	startTime time.Time
	dbProvider repository.DatabaseProvider
}

func NewHealthHandler(dbProvider repository.DatabaseProvider) *HealthHandler {
	return &HealthHandler{
		startTime: time.Now(),
		dbProvider: dbProvider,
	}
}

// HealthCheck handles GET /health
func (h *HealthHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "payperplay-hosting",
		"version": "2.0",
		"uptime":  time.Since(h.startTime).String(),
	})
}

// ReadinessCheck handles GET /ready
func (h *HealthHandler) ReadinessCheck(c *gin.Context) {
	// Check database connection
	if err := h.dbProvider.Ping(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not_ready",
			"reason": "database_unavailable",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "ready",
		"database": "connected",
		"uptime":   time.Since(h.startTime).String(),
	})
}

// LivenessCheck handles GET /live
func (h *HealthHandler) LivenessCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "alive",
		"uptime": time.Since(h.startTime).String(),
	})
}

// MetricsCheck handles GET /metrics (basic version)
func (h *HealthHandler) MetricsCheck(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	c.JSON(http.StatusOK, gin.H{
		"uptime_seconds": time.Since(h.startTime).Seconds(),
		"memory": gin.H{
			"alloc_mb":       m.Alloc / 1024 / 1024,
			"total_alloc_mb": m.TotalAlloc / 1024 / 1024,
			"sys_mb":         m.Sys / 1024 / 1024,
			"num_gc":         m.NumGC,
		},
		"goroutines": runtime.NumGoroutine(),
	})
}
