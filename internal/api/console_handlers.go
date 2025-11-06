package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/payperplay/hosting/internal/service"
	"github.com/payperplay/hosting/pkg/logger"
)

type ConsoleHandler struct {
	consoleService *service.ConsoleService
	upgrader       websocket.Upgrader
}

func NewConsoleHandler(consoleService *service.ConsoleService) *ConsoleHandler {
	return &ConsoleHandler{
		consoleService: consoleService,
		upgrader:       createUpgrader(true), // Allow all origins for MVP
	}
}

// Message types for WebSocket communication
type ConsoleMessage struct {
	Type    string `json:"type"`    // "log" or "command" or "response"
	Content string `json:"content"` // log line, command, or response
}

// HandleConsoleWebSocket handles WebSocket connection for server console
func (h *ConsoleHandler) HandleConsoleWebSocket(c *gin.Context) {
	serverID := c.Param("id")

	// Upgrade to WebSocket
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("Failed to upgrade to WebSocket", err, map[string]interface{}{
			"server_id": serverID,
		})
		return
	}
	defer conn.Close()

	logger.Info("Console WebSocket connected", map[string]interface{}{
		"server_id": serverID,
	})

	// Start streaming logs
	logChan, cancel, err := h.consoleService.StreamLogs(serverID)
	if err != nil {
		logger.Error("Failed to start log stream", err, map[string]interface{}{
			"server_id": serverID,
		})
		conn.WriteJSON(ConsoleMessage{
			Type:    "error",
			Content: err.Error(),
		})
		return
	}
	defer cancel()

	// Channel to signal when client disconnects
	done := make(chan struct{})

	// Goroutine to read commands from client
	go func() {
		defer close(done)
		for {
			var msg ConsoleMessage
			err := conn.ReadJSON(&msg)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					logger.Error("WebSocket read error", err, map[string]interface{}{
						"server_id": serverID,
					})
				}
				return
			}

			if msg.Type == "command" {
				// Execute command via RCON
				response, err := h.consoleService.ExecuteCommand(serverID, msg.Content)
				if err != nil {
					logger.Error("Failed to execute command", err, map[string]interface{}{
						"server_id": serverID,
						"command":   msg.Content,
					})
					conn.WriteJSON(ConsoleMessage{
						Type:    "error",
						Content: "Failed to execute command: " + err.Error(),
					})
					continue
				}

				// Send command response back to client
				conn.WriteJSON(ConsoleMessage{
					Type:    "response",
					Content: response,
				})
			}
		}
	}()

	// Stream logs to client
	for {
		select {
		case logLine, ok := <-logChan:
			if !ok {
				// Log stream closed
				return
			}

			err := conn.WriteJSON(ConsoleMessage{
				Type:    "log",
				Content: logLine,
			})
			if err != nil {
				logger.Error("Failed to write log to WebSocket", err, map[string]interface{}{
					"server_id": serverID,
				})
				return
			}

		case <-done:
			// Client disconnected
			logger.Info("Console WebSocket disconnected", map[string]interface{}{
				"server_id": serverID,
			})
			return
		}
	}
}

// GetConsoleLogs returns recent console logs (REST endpoint for backup)
func (h *ConsoleHandler) GetConsoleLogs(c *gin.Context) {
	serverID := c.Param("id")

	// For now, return a simple message
	// In the future, we could store recent logs in memory or database
	c.JSON(http.StatusOK, gin.H{
		"message": "Use WebSocket endpoint for real-time logs",
		"ws_url":  "/api/servers/" + serverID + "/console/stream",
	})
}
