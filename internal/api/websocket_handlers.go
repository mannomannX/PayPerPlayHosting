package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	ws "github.com/payperplay/hosting/internal/websocket"
	"github.com/payperplay/hosting/pkg/logger"
)

// createUpgrader creates a WebSocket upgrader with appropriate CORS settings
func createUpgrader(allowAllOrigins bool) websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			if allowAllOrigins {
				// Development mode: allow all origins
				return true
			}
			// Production mode: only allow same origin
			origin := r.Header.Get("Origin")
			return origin == "" || origin == r.Host
		},
	}
}

type WebSocketHandler struct {
	hub      *ws.Hub
	upgrader websocket.Upgrader
}

func NewWebSocketHandler(hub *ws.Hub) *WebSocketHandler {
	return &WebSocketHandler{
		hub:      hub,
		upgrader: createUpgrader(true), // Allow all origins for MVP
	}
}

// HandleWebSocket upgrades HTTP connection to WebSocket
func (h *WebSocketHandler) HandleWebSocket(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("Failed to upgrade to WebSocket", err, map[string]interface{}{
			"remote_addr": c.Request.RemoteAddr,
		})
		return
	}

	client := ws.NewClient(h.hub, conn)
	h.hub.Register(client)

	// Start client goroutines
	go client.WritePump()
	go client.ReadPump()
}

// GetStats returns WebSocket statistics
func (h *WebSocketHandler) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"connected_clients": h.hub.GetClientCount(),
	})
}
