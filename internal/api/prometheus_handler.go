package api

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusHandler handles Prometheus metrics endpoint
type PrometheusHandler struct{}

// NewPrometheusHandler creates a new Prometheus handler
func NewPrometheusHandler() *PrometheusHandler {
	return &PrometheusHandler{}
}

// MetricsEndpoint serves Prometheus metrics
// GET /metrics
func (h *PrometheusHandler) MetricsEndpoint(c *gin.Context) {
	handler := promhttp.Handler()
	handler.ServeHTTP(c.Writer, c.Request)
}
