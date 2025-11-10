package events

import (
	"context"

	"github.com/payperplay/hosting/internal/storage"
)

// InfluxDBEventStorage stores events in InfluxDB for time-series analytics
type InfluxDBEventStorage struct {
	client *storage.InfluxDBClient
}

// NewInfluxDBEventStorage creates a new InfluxDB event storage
func NewInfluxDBEventStorage(client *storage.InfluxDBClient) *InfluxDBEventStorage {
	return &InfluxDBEventStorage{client: client}
}

// Store saves an event to InfluxDB
func (s *InfluxDBEventStorage) Store(event Event) error {
	// Convert events.Event to storage.EventData
	eventData := storage.EventData{
		ID:        event.ID,
		Type:      string(event.Type),
		Timestamp: event.Timestamp,
		Source:    event.Source,
		ServerID:  event.ServerID,
		UserID:    event.UserID,
		Data:      event.Data,
	}
	return s.client.WriteEvent(eventData)
}

// Query retrieves events from InfluxDB based on filters
func (s *InfluxDBEventStorage) Query(filters EventFilters) ([]Event, error) {
	// Convert events.EventFilters to storage.EventFilters
	storageFilters := storage.EventFilters{
		Types:     make([]string, len(filters.Types)),
		ServerID:  filters.ServerID,
		UserID:    filters.UserID,
		StartTime: filters.StartTime,
		EndTime:   filters.EndTime,
		Limit:     filters.Limit,
	}

	// Convert event types
	for i, t := range filters.Types {
		storageFilters.Types[i] = string(t)
	}

	// Query InfluxDB
	ctx := context.Background()
	storageEvents, err := s.client.QueryEvents(ctx, storageFilters)
	if err != nil {
		return nil, err
	}

	// Convert storage.EventData to events.Event
	events := make([]Event, len(storageEvents))
	for i, se := range storageEvents {
		events[i] = Event{
			ID:        se.ID,
			Type:      EventType(se.Type),
			Timestamp: se.Timestamp,
			Source:    se.Source,
			ServerID:  se.ServerID,
			UserID:    se.UserID,
			Data:      se.Data,
		}
	}

	return events, nil
}
