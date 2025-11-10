package events

import (
	"github.com/payperplay/hosting/pkg/logger"
)

// MultiEventStorage stores events in multiple backends simultaneously
type MultiEventStorage struct {
	storages []EventStorage
}

// NewMultiEventStorage creates a storage that writes to multiple backends
func NewMultiEventStorage(storages ...EventStorage) *MultiEventStorage {
	return &MultiEventStorage{
		storages: storages,
	}
}

// Store saves an event to all configured storage backends
func (s *MultiEventStorage) Store(event Event) error {
	var lastError error

	for _, storage := range s.storages {
		if err := storage.Store(event); err != nil {
			logger.Error("Failed to store event in backend", err, map[string]interface{}{
				"event_id":   event.ID,
				"event_type": event.Type,
			})
			lastError = err
			// Continue to next storage even if one fails
		}
	}

	// Return last error if any occurred
	return lastError
}

// Query retrieves events from the first storage backend that succeeds
// Priority order: first storage has priority, fallback to next if it fails
func (s *MultiEventStorage) Query(filters EventFilters) ([]Event, error) {
	if len(s.storages) == 0 {
		return nil, nil
	}

	// Try each storage in order until one succeeds
	for i, storage := range s.storages {
		events, err := storage.Query(filters)
		if err == nil {
			return events, nil
		}

		// Log error and try next storage
		logger.Warn("Failed to query events from storage backend", map[string]interface{}{
			"backend_index": i,
			"error":         err.Error(),
		})
	}

	// If all storages failed, return error from last one
	return s.storages[0].Query(filters)
}
