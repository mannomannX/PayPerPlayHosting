package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/payperplay/hosting/internal/conductor"
)

// ConductorHandler handles Conductor API endpoints
type ConductorHandler struct {
	conductor *conductor.Conductor
}

// NewConductorHandler creates a new Conductor handler
func NewConductorHandler(cond *conductor.Conductor) *ConductorHandler {
	return &ConductorHandler{
		conductor: cond,
	}
}

// GetStatus returns the current conductor status
// GET /conductor/status
func (h *ConductorHandler) GetStatus(c *gin.Context) {
	status := h.conductor.GetStatus()

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"data":   status,
	})
}

// GetFleetStats returns fleet statistics
// GET /conductor/fleet
func (h *ConductorHandler) GetFleetStats(c *gin.Context) {
	stats := h.conductor.NodeRegistry.GetFleetStats()

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"data":   stats,
	})
}

// GetNodes returns all registered nodes
// GET /conductor/nodes
func (h *ConductorHandler) GetNodes(c *gin.Context) {
	nodes := h.conductor.NodeRegistry.GetAllNodes()

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"data":   nodes,
	})
}

// GetContainers returns all registered containers
// GET /conductor/containers
func (h *ConductorHandler) GetContainers(c *gin.Context) {
	containers := h.conductor.ContainerRegistry.GetAllContainers()

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"data":   containers,
	})
}

// GetDebugLogs returns recent debug log entries for the dashboard console
// GET /conductor/debug-logs
func (h *ConductorHandler) GetDebugLogs(c *gin.Context) {
	logs := h.conductor.DebugLogBuffer.GetAll()

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"data":   logs,
	})
}

// ClearDebugLogs clears all debug log entries
// DELETE /conductor/debug-logs
func (h *ConductorHandler) ClearDebugLogs(c *gin.Context) {
	h.conductor.DebugLogBuffer.Clear()

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Debug logs cleared",
	})
}
