package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/service"
)

// MetricsHandler handles metrics endpoints
type MetricsHandler struct{}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler() *MetricsHandler {
	return &MetricsHandler{}
}

// GetFileMetrics returns file upload/management metrics
// GET /api/metrics/files
func (h *MetricsHandler) GetFileMetrics(c *gin.Context) {
	metrics := service.GetFileMetrics()
	snapshot := metrics.GetSnapshot()

	c.JSON(http.StatusOK, snapshot)
}

// ResetFileMetrics resets file metrics (admin only)
// POST /api/metrics/files/reset
func (h *MetricsHandler) ResetFileMetrics(c *gin.Context) {
	metrics := service.GetFileMetrics()
	metrics.Reset()

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "File metrics reset successfully",
	})
}
