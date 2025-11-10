package conductor

import (
	"sync"
	"time"

	"github.com/payperplay/hosting/pkg/logger"
)

// QueuedServer represents a server waiting to start
type QueuedServer struct {
	ServerID      string
	ServerName    string
	RequiredRAMMB int
	QueuedAt      time.Time
	UserID        string
}

// StartQueue manages servers waiting for available capacity
type StartQueue struct {
	queue []*QueuedServer
	mu    sync.RWMutex
}

// NewStartQueue creates a new start queue
func NewStartQueue() *StartQueue {
	return &StartQueue{
		queue: make([]*QueuedServer, 0),
	}
}

// Enqueue adds a server to the queue
func (q *StartQueue) Enqueue(server *QueuedServer) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check if server is already queued (prevent duplicates)
	for _, s := range q.queue {
		if s.ServerID == server.ServerID {
			logger.Warn("Server already in queue, skipping", map[string]interface{}{
				"server_id":   server.ServerID,
				"server_name": server.ServerName,
			})
			return
		}
	}

	q.queue = append(q.queue, server)

	logger.Info("Server added to start queue", map[string]interface{}{
		"server_id":      server.ServerID,
		"server_name":    server.ServerName,
		"required_ram":   server.RequiredRAMMB,
		"queue_position": len(q.queue),
		"queued_at":      server.QueuedAt,
	})
}

// Dequeue removes and returns the next server from the queue
func (q *StartQueue) Dequeue() *QueuedServer {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.queue) == 0 {
		return nil
	}

	// FIFO: Get the first server in the queue
	server := q.queue[0]
	q.queue = q.queue[1:]

	logger.Info("Server dequeued from start queue", map[string]interface{}{
		"server_id":       server.ServerID,
		"server_name":     server.ServerName,
		"queue_remaining": len(q.queue),
	})

	return server
}

// Peek returns the next server without removing it
func (q *StartQueue) Peek() *QueuedServer {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if len(q.queue) == 0 {
		return nil
	}

	return q.queue[0]
}

// Remove removes a specific server from the queue (e.g., if deleted)
func (q *StartQueue) Remove(serverID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, server := range q.queue {
		if server.ServerID == serverID {
			q.queue = append(q.queue[:i], q.queue[i+1:]...)
			logger.Info("Server removed from start queue", map[string]interface{}{
				"server_id":   serverID,
				"server_name": server.ServerName,
			})
			return true
		}
	}

	return false
}

// GetPosition returns the queue position for a server (1-based)
func (q *StartQueue) GetPosition(serverID string) int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	for i, server := range q.queue {
		if server.ServerID == serverID {
			return i + 1 // 1-based position
		}
	}

	return 0 // Not in queue
}

// Size returns the current queue size
func (q *StartQueue) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return len(q.queue)
}

// GetAll returns a copy of all queued servers
func (q *StartQueue) GetAll() []*QueuedServer {
	q.mu.RLock()
	defer q.mu.RUnlock()

	// Return a copy to avoid race conditions
	queueCopy := make([]*QueuedServer, len(q.queue))
	copy(queueCopy, q.queue)

	return queueCopy
}

// Clear removes all servers from the queue
func (q *StartQueue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.queue = make([]*QueuedServer, 0)
	logger.Info("Start queue cleared", nil)
}

// GetTotalRequiredRAM calculates total RAM needed by all queued servers
func (q *StartQueue) GetTotalRequiredRAM() int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	totalRAM := 0
	for _, server := range q.queue {
		totalRAM += server.RequiredRAMMB
	}

	return totalRAM
}
