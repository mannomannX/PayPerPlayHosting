package events

import (
	"sync"
	"time"

	"github.com/payperplay/hosting/pkg/logger"
)

// EventType represents the type of event
type EventType string

const (
	// Server lifecycle events
	EventServerCreated       EventType = "server.created"
	EventServerStarted       EventType = "server.started"
	EventServerStopped       EventType = "server.stopped"
	EventServerDeleted       EventType = "server.deleted"
	EventServerCrashed       EventType = "server.crashed"
	EventServerRestarted     EventType = "server.restarted"
	EventServerStateChanged  EventType = "server.state_changed"

	// Player events
	EventPlayerJoined        EventType = "player.joined"
	EventPlayerLeft          EventType = "player.left"
	EventPlayerCountChanged  EventType = "player.count_changed"

	// Billing events
	EventBillingStarted      EventType = "billing.started"
	EventBillingStopped      EventType = "billing.stopped"
	EventBillingPhaseChanged EventType = "billing.phase_changed"

	// Backup events
	EventBackupCreated       EventType = "backup.created"
	EventBackupRestored      EventType = "backup.restored"
	EventBackupDeleted       EventType = "backup.deleted"
	EventBackupFailed        EventType = "backup.failed"

	// System events
	EventNodeAdded           EventType = "node.added"
	EventNodeRemoved         EventType = "node.removed"
	EventNodeHealthChanged   EventType = "node.health_changed"
	EventScalingTriggered    EventType = "scaling.triggered"
)

// Event represents a system event
type Event struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Source    string                 `json:"source"`     // e.g., "minecraft_service", "conductor"
	ServerID  string                 `json:"server_id,omitempty"`
	UserID    string                 `json:"user_id,omitempty"`
	Data      map[string]interface{} `json:"data"`
}

// EventHandler is a function that handles events
type EventHandler func(event Event)

// EventBus manages event publishing and subscription
type EventBus struct {
	subscribers map[EventType][]EventHandler
	mu          sync.RWMutex
	storage     EventStorage
}

// EventStorage defines the interface for storing events
type EventStorage interface {
	Store(event Event) error
	Query(filters EventFilters) ([]Event, error)
}

// EventFilters for querying events
type EventFilters struct {
	Types     []EventType
	ServerID  string
	UserID    string
	StartTime time.Time
	EndTime   time.Time
	Limit     int
}

var (
	globalBus     *EventBus
	globalBusOnce sync.Once
)

// GetEventBus returns the global event bus instance (singleton)
func GetEventBus() *EventBus {
	globalBusOnce.Do(func() {
		globalBus = NewEventBus(nil)
	})
	return globalBus
}

// SetEventStorage sets the event storage backend
func SetEventStorage(storage EventStorage) {
	bus := GetEventBus()
	bus.mu.Lock()
	defer bus.mu.Unlock()
	bus.storage = storage
}

// NewEventBus creates a new event bus
func NewEventBus(storage EventStorage) *EventBus {
	return &EventBus{
		subscribers: make(map[EventType][]EventHandler),
		storage:     storage,
	}
}

// Subscribe registers a handler for a specific event type
func (eb *EventBus) Subscribe(eventType EventType, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.subscribers[eventType] = append(eb.subscribers[eventType], handler)
	logger.Info("Event handler subscribed", map[string]interface{}{
		"event_type": eventType,
	})
}

// Publish publishes an event to all subscribers
func (eb *EventBus) Publish(event Event) {
	// Set timestamp if not set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Generate ID if not set
	if event.ID == "" {
		event.ID = generateEventID()
	}

	// Store event if storage is configured
	if eb.storage != nil {
		if err := eb.storage.Store(event); err != nil {
			logger.Error("Failed to store event", err, map[string]interface{}{
				"event_id":   event.ID,
				"event_type": event.Type,
			})
		}
	}

	// Notify subscribers
	eb.mu.RLock()
	handlers := eb.subscribers[event.Type]
	eb.mu.RUnlock()

	for _, handler := range handlers {
		// Run handlers in goroutines to avoid blocking
		go func(h EventHandler) {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Event handler panicked", nil, map[string]interface{}{
						"event_type": event.Type,
						"panic":      r,
					})
				}
			}()
			h(event)
		}(handler)
	}

	logger.Info("Event published", map[string]interface{}{
		"event_id":   event.ID,
		"event_type": event.Type,
		"source":     event.Source,
	})
}

// Query retrieves events based on filters
func (eb *EventBus) Query(filters EventFilters) ([]Event, error) {
	if eb.storage == nil {
		return nil, nil
	}
	return eb.storage.Query(filters)
}

// generateEventID generates a unique event ID
func generateEventID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
