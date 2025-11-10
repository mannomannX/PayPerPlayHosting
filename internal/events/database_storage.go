package events

import (
	"encoding/json"

	"github.com/payperplay/hosting/internal/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// DatabaseEventStorage stores events in PostgreSQL
type DatabaseEventStorage struct {
	db *gorm.DB
}

// NewDatabaseEventStorage creates a new database event storage
func NewDatabaseEventStorage(db *gorm.DB) *DatabaseEventStorage {
	return &DatabaseEventStorage{db: db}
}

// Store saves an event to the database
func (s *DatabaseEventStorage) Store(event Event) error {
	// Convert event data to JSON
	dataJSON, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}

	systemEvent := &models.SystemEvent{
		EventID:   event.ID,
		Type:      string(event.Type),
		Timestamp: event.Timestamp,
		Source:    event.Source,
		ServerID:  event.ServerID,
		UserID:    event.UserID,
		Data:      datatypes.JSON(dataJSON),
	}

	return s.db.Create(systemEvent).Error
}

// Query retrieves events based on filters
func (s *DatabaseEventStorage) Query(filters EventFilters) ([]Event, error) {
	query := s.db.Model(&models.SystemEvent{})

	// Apply filters
	if len(filters.Types) > 0 {
		types := make([]string, len(filters.Types))
		for i, t := range filters.Types {
			types[i] = string(t)
		}
		query = query.Where("type IN ?", types)
	}

	if filters.ServerID != "" {
		query = query.Where("server_id = ?", filters.ServerID)
	}

	if filters.UserID != "" {
		query = query.Where("user_id = ?", filters.UserID)
	}

	if !filters.StartTime.IsZero() {
		query = query.Where("timestamp >= ?", filters.StartTime)
	}

	if !filters.EndTime.IsZero() {
		query = query.Where("timestamp <= ?", filters.EndTime)
	}

	// Order by timestamp descending
	query = query.Order("timestamp DESC")

	// Apply limit
	if filters.Limit > 0 {
		query = query.Limit(filters.Limit)
	} else {
		query = query.Limit(1000) // Default limit
	}

	var systemEvents []models.SystemEvent
	if err := query.Find(&systemEvents).Error; err != nil {
		return nil, err
	}

	// Convert to Event objects
	events := make([]Event, len(systemEvents))
	for i, se := range systemEvents {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(se.Data), &data); err != nil {
			data = make(map[string]interface{})
		}

		events[i] = Event{
			ID:        se.EventID,
			Type:      EventType(se.Type),
			Timestamp: se.Timestamp,
			Source:    se.Source,
			ServerID:  se.ServerID,
			UserID:    se.UserID,
			Data:      data,
		}
	}

	return events, nil
}
