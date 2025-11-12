package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/payperplay/hosting/internal/conductor"
	"github.com/payperplay/hosting/pkg/logger"
)

// WebSocket upgrader configuration
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Restrict to admin domain in production
		return true
	},
}

// DashboardWebSocket manages WebSocket connections for the admin dashboard
type DashboardWebSocket struct {
	conductor     *conductor.Conductor
	clients       map[*websocket.Conn]bool
	clientsMutex  sync.RWMutex
	broadcast     chan DashboardEvent
	register      chan *websocket.Conn
	unregister    chan *websocket.Conn
	shutdownChan  chan struct{}
}

// DashboardEvent represents a WebSocket message sent to dashboard clients
type DashboardEvent struct {
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// NewDashboardWebSocket creates a new dashboard WebSocket manager
func NewDashboardWebSocket(conductor *conductor.Conductor) *DashboardWebSocket {
	return &DashboardWebSocket{
		conductor:    conductor,
		clients:      make(map[*websocket.Conn]bool),
		broadcast:    make(chan DashboardEvent, 256),
		register:     make(chan *websocket.Conn),
		unregister:   make(chan *websocket.Conn),
		shutdownChan: make(chan struct{}),
	}
}

// Run starts the WebSocket manager (run in goroutine)
func (ws *DashboardWebSocket) Run() {
	logger.Info("DashboardWebSocket: Starting WebSocket manager", nil)

	// Start periodic stats broadcaster
	statsTicker := time.NewTicker(5 * time.Second)
	defer statsTicker.Stop()

	for {
		select {
		case client := <-ws.register:
			ws.clientsMutex.Lock()
			ws.clients[client] = true
			ws.clientsMutex.Unlock()

			logger.Info("DashboardWebSocket: Client connected", map[string]interface{}{
				"total_clients": len(ws.clients),
			})

			// Send initial state to new client
			go ws.sendInitialState(client)

		case client := <-ws.unregister:
			ws.clientsMutex.Lock()
			if _, ok := ws.clients[client]; ok {
				delete(ws.clients, client)
				client.Close()
			}
			ws.clientsMutex.Unlock()

			logger.Info("DashboardWebSocket: Client disconnected", map[string]interface{}{
				"total_clients": len(ws.clients),
			})

		case event := <-ws.broadcast:
			ws.clientsMutex.RLock()
			for client := range ws.clients {
				go ws.sendToClient(client, event)
			}
			ws.clientsMutex.RUnlock()

		case <-statsTicker.C:
			// Broadcast periodic stats updates
			ws.broadcastFleetStats()

		case <-ws.shutdownChan:
			logger.Info("DashboardWebSocket: Shutting down", nil)
			return
		}
	}
}

// HandleConnection handles WebSocket upgrade and client connection
// GET /api/admin/dashboard/stream
func (ws *DashboardWebSocket) HandleConnection(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Info("DashboardWebSocket: Failed to upgrade connection", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Register client
	ws.register <- conn

	// Handle client messages (ping/pong)
	go ws.handleClientMessages(conn)
}

// handleClientMessages handles incoming messages from clients (mostly ping/pong)
func (ws *DashboardWebSocket) handleClientMessages(conn *websocket.Conn) {
	defer func() {
		ws.unregister <- conn
	}()

	// Set read deadline for ping/pong
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Keep connection alive with ping
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	go func() {
		for {
			select {
			case <-pingTicker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	// Read messages (for future bidirectional communication)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Info("DashboardWebSocket: Unexpected close error", map[string]interface{}{
					"error": err.Error(),
				})
			}
			break
		}
	}
}

// sendToClient sends an event to a specific client
func (ws *DashboardWebSocket) sendToClient(client *websocket.Conn, event DashboardEvent) {
	client.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := client.WriteJSON(event); err != nil {
		logger.Info("DashboardWebSocket: Failed to send message", map[string]interface{}{
			"error": err.Error(),
		})
		ws.unregister <- client
	}
}

// sendInitialState sends the current system state to a newly connected client
func (ws *DashboardWebSocket) sendInitialState(client *websocket.Conn) {
	// Send all nodes
	nodes := ws.conductor.NodeRegistry.GetAllNodes()
	for _, node := range nodes {
		// Provider and location derived from labels or defaults
		provider := "hetzner"
		location := "nbg1"
		if loc, ok := node.Labels["location"]; ok {
			location = loc
		}

		// Send node.created event
		event := DashboardEvent{
			Type:      "node.created",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"node_id":        node.ID,
				"node_type":      node.Type,
				"provider":       provider,
				"location":       location,
				"total_ram_mb":   node.TotalRAMMB,
				"usable_ram_mb":  node.UsableRAMMB(),
				"status":         string(node.Status),
				"ip_address":     node.IPAddress,
				"is_system_node": node.IsSystemNode,
			},
		}
		ws.sendToClient(client, event)

		// Send node.stats event with current allocations
		containerCount, allocatedRAM := ws.conductor.ContainerRegistry.GetNodeAllocation(node.ID)
		capacityPercent := 0.0
		if node.UsableRAMMB() > 0 {
			capacityPercent = (float64(allocatedRAM) / float64(node.UsableRAMMB())) * 100
		}

		statsEvent := DashboardEvent{
			Type:      "node.stats",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"node_id":           node.ID,
				"allocated_ram_mb":  allocatedRAM,
				"free_ram_mb":       node.AvailableRAMMB(),
				"container_count":   containerCount,
				"capacity_percent":  capacityPercent,
				"cpu_usage_percent": node.CPUUsagePercent,
			},
		}
		ws.sendToClient(client, statsEvent)
	}

	// Send all containers
	if ws.conductor.ContainerRegistry != nil {
		containers := ws.conductor.ContainerRegistry.GetAllContainers()
		for _, container := range containers {
			event := DashboardEvent{
				Type:      "container.created",
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"server_id":    container.ServerID,
					"server_name":  container.ServerName,
					"container_id": container.ContainerID,
					"node_id":      container.NodeID,
					"ram_mb":       container.RAMMb,
					"status":       string(container.Status),
					"port":         container.DockerPort,
				},
			}
			ws.sendToClient(client, event)
		}
	}

	// Send deployment queue
	if ws.conductor.StartQueue != nil {
		queuedServers := ws.conductor.StartQueue.GetAll()
		queueEvent := DashboardEvent{
			Type:      "queue.updated",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"queue_size": len(queuedServers),
				"servers":    queuedServers,
			},
		}
		ws.sendToClient(client, queueEvent)
	}

	// Send current fleet stats
	stats := ws.conductor.NodeRegistry.GetFleetStats()
	capacityPercent := 0.0
	if stats.UsableRAMMB > 0 {
		capacityPercent = (float64(stats.AllocatedRAMMB) / float64(stats.UsableRAMMB)) * 100
	}

	event := DashboardEvent{
		Type:      "stats.fleet",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"total_nodes":      stats.TotalNodes,
			"dedicated_nodes":  stats.DedicatedNodes,
			"cloud_nodes":      stats.CloudNodes,
			"total_ram_mb":     stats.TotalRAMMB,
			"usable_ram_mb":    stats.UsableRAMMB,
			"allocated_ram_mb": stats.AllocatedRAMMB,
			"free_ram_mb":      stats.AvailableRAMMB,
			"capacity_percent": capacityPercent,
			"total_servers":    stats.TotalContainers,
		},
	}
	ws.sendToClient(client, event)

	logger.Info("DashboardWebSocket: Sent initial state to client", map[string]interface{}{
		"nodes":      len(nodes),
		"containers": len(ws.conductor.ContainerRegistry.GetAllContainers()),
	})
}

// broadcastFleetStats broadcasts current fleet statistics
func (ws *DashboardWebSocket) broadcastFleetStats() {
	stats := ws.conductor.NodeRegistry.GetFleetStats()
	capacityPercent := 0.0
	if stats.UsableRAMMB > 0 {
		capacityPercent = (float64(stats.AllocatedRAMMB) / float64(stats.UsableRAMMB)) * 100
	}

	event := DashboardEvent{
		Type:      "stats.fleet",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"total_nodes":      stats.TotalNodes,
			"dedicated_nodes":  stats.DedicatedNodes,
			"cloud_nodes":      stats.CloudNodes,
			"total_ram_mb":     stats.TotalRAMMB,
			"usable_ram_mb":    stats.UsableRAMMB,
			"allocated_ram_mb": stats.AllocatedRAMMB,
			"free_ram_mb":      stats.AvailableRAMMB,
			"capacity_percent": capacityPercent,
			"total_servers":    stats.TotalContainers,
			"queue_size":       ws.conductor.StartQueue.Size(),
		},
	}
	ws.broadcast <- event
}

// PublishEvent publishes an event to all connected clients
func (ws *DashboardWebSocket) PublishEvent(eventType string, data interface{}) {
	event := DashboardEvent{
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      data,
	}

	// Non-blocking send
	select {
	case ws.broadcast <- event:
	default:
		logger.Warn("DashboardWebSocket: Broadcast channel full, dropping event", map[string]interface{}{
			"event_type": eventType,
		})
	}
}

// Shutdown gracefully shuts down the WebSocket manager
func (ws *DashboardWebSocket) Shutdown() {
	close(ws.shutdownChan)

	// Close all client connections
	ws.clientsMutex.Lock()
	for client := range ws.clients {
		client.Close()
	}
	ws.clientsMutex.Unlock()
}

// Helper function to create event data structures

// NodeEventData represents node-related event data
type NodeEventData struct {
	NodeID      string `json:"node_id"`
	NodeType    string `json:"node_type"`
	Provider    string `json:"provider"`
	Location    string `json:"location"`
	TotalRAMMB  int    `json:"total_ram_mb"`
	UsableRAMMB int    `json:"usable_ram_mb"`
	Status      string `json:"status"`
	IPAddress   string `json:"ip_address"`
}

// NodeStatsEventData represents node statistics
type NodeStatsEventData struct {
	NodeID          string  `json:"node_id"`
	AllocatedRAMMB  int     `json:"allocated_ram_mb"`
	FreeRAMMB       int     `json:"free_ram_mb"`
	ContainerCount  int     `json:"container_count"`
	CapacityPercent float64 `json:"capacity_percent"`
	CPUUsagePercent float64 `json:"cpu_usage_percent,omitempty"`
}

// ContainerEventData represents container-related event data
type ContainerEventData struct {
	ServerID   string `json:"server_id"`
	ServerName string `json:"server_name"`
	NodeID     string `json:"node_id"`
	RAMMb      int    `json:"ram_mb"`
	Status     string `json:"status"`
	Port       int    `json:"port,omitempty"`
}

// MigrationEventData represents migration operation data
type MigrationEventData struct {
	OperationID string `json:"operation_id"`
	ServerID    string `json:"server_id"`
	ServerName  string `json:"server_name"`
	FromNode    string `json:"from_node"`
	ToNode      string `json:"to_node"`
	RAMMb       int    `json:"ram_mb"`
	PlayerCount int    `json:"player_count,omitempty"`
	Status      string `json:"status"` // started, progress, completed, failed
	Progress    int    `json:"progress,omitempty"` // 0-100
	Error       string `json:"error,omitempty"`
}

// ScalingDecisionEventData represents scaling decision data
type ScalingDecisionEventData struct {
	PolicyName      string                 `json:"policy_name"`
	Action          string                 `json:"action"` // scale_up, scale_down, consolidate, none
	ServerType      string                 `json:"server_type,omitempty"`
	Count           int                    `json:"count,omitempty"`
	Reason          string                 `json:"reason"`
	Urgency         string                 `json:"urgency"`
	DecisionTree    map[string]interface{} `json:"decision_tree,omitempty"`
	CapacityPercent float64                `json:"capacity_percent"`
}
