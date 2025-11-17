package conductor

import (
	"sync"
	"time"

	"github.com/payperplay/hosting/internal/events"
	"github.com/payperplay/hosting/pkg/logger"
)

// QueuedServer represents a server waiting to start
type QueuedServer struct {
	ServerID      string
	ServerName    string
	RequiredRAMMB int
	QueuedAt      time.Time
	UserID        string
	// GAP-5: Retry tracking for queue poisoning prevention
	RetryCount    int       // Number of retry attempts (0 = first attempt)
	FirstQueuedAt time.Time // Original queue time (never changes)
	LastRetryAt   time.Time // Last time we attempted to start
	NextRetryAt   time.Time // When we can retry next (exponential backoff)
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

	// GAP-5: Check if server is already queued (retry case)
	for i, s := range q.queue {
		if s.ServerID == server.ServerID {
			// This is a retry - update retry tracking
			s.RetryCount++
			s.LastRetryAt = time.Now()

			// Calculate exponential backoff: 1min, 2min, 4min
			backoffDurations := []time.Duration{
				1 * time.Minute,  // First retry after 1min
				2 * time.Minute,  // Second retry after 2min
				4 * time.Minute,  // Third retry after 4min
			}

			backoffIndex := s.RetryCount - 1
			if backoffIndex >= len(backoffDurations) {
				backoffIndex = len(backoffDurations) - 1
			}
			s.NextRetryAt = time.Now().Add(backoffDurations[backoffIndex])

			// Update the queue entry in place
			q.queue[i] = s

			logger.Info("GAP-5: Server retry queued with exponential backoff", map[string]interface{}{
				"server_id":     server.ServerID,
				"server_name":   server.ServerName,
				"retry_count":   s.RetryCount,
				"next_retry_at": s.NextRetryAt,
				"backoff":       backoffDurations[backoffIndex].String(),
			})

			// Publish queue update events
			events.PublishQueueUpdated(len(q.queue), q.queue)
			return
		}
	}

	// First time queuing - initialize tracking fields
	now := time.Now()
	server.FirstQueuedAt = now
	server.QueuedAt = now
	server.LastRetryAt = now
	server.RetryCount = 0
	server.NextRetryAt = now // Can process immediately

	q.queue = append(q.queue, server)

	logger.Info("Server added to start queue", map[string]interface{}{
		"server_id":      server.ServerID,
		"server_name":    server.ServerName,
		"required_ram":   server.RequiredRAMMB,
		"queue_position": len(q.queue),
		"queued_at":      server.QueuedAt,
	})

	// Publish queue update events
	events.PublishServerQueued(server.ServerID, server.ServerName, server.RequiredRAMMB, len(q.queue))
	events.PublishQueueUpdated(len(q.queue), q.queue)
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

	// Publish queue update events
	events.PublishServerDequeued(server.ServerID, server.ServerName)
	events.PublishQueueUpdated(len(q.queue), q.queue)

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

			// Publish queue update events
			events.PublishServerDequeued(server.ServerID, server.ServerName)
			events.PublishQueueUpdated(len(q.queue), q.queue)

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
