package storage

import (
	"context"
	"fmt"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/payperplay/hosting/pkg/logger"
)

// EventData is a generic event structure that doesn't depend on internal/events
type EventData struct {
	ID        string
	Type      string
	Timestamp time.Time
	Source    string
	ServerID  string
	UserID    string
	Data      map[string]interface{}
}

// EventFilters for querying events
type EventFilters struct {
	Types     []string
	ServerID  string
	UserID    string
	StartTime time.Time
	EndTime   time.Time
	Limit     int
}

// InfluxDBClient manages connection to InfluxDB for time-series event storage
type InfluxDBClient struct {
	client   influxdb2.Client
	writeAPI api.WriteAPI
	queryAPI api.QueryAPI
	org      string
	bucket   string
}

// InfluxDBConfig holds InfluxDB connection configuration
type InfluxDBConfig struct {
	URL    string
	Token  string
	Org    string
	Bucket string
}

// NewInfluxDBClient creates a new InfluxDB client
func NewInfluxDBClient(config InfluxDBConfig) (*InfluxDBClient, error) {
	// Create InfluxDB client
	client := influxdb2.NewClient(config.URL, config.Token)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	health, err := client.Health(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to InfluxDB: %w", err)
	}

	if health.Status != "pass" {
		return nil, fmt.Errorf("InfluxDB health check failed: %s", health.Message)
	}

	logger.Info("InfluxDB connection established", map[string]interface{}{
		"url":    config.URL,
		"org":    config.Org,
		"bucket": config.Bucket,
		"status": health.Status,
	})

	// Get write and query APIs
	writeAPI := client.WriteAPI(config.Org, config.Bucket)
	queryAPI := client.QueryAPI(config.Org)

	return &InfluxDBClient{
		client:   client,
		writeAPI: writeAPI,
		queryAPI: queryAPI,
		org:      config.Org,
		bucket:   config.Bucket,
	}, nil
}

// WriteEvent writes an event to InfluxDB as a time-series point
func (c *InfluxDBClient) WriteEvent(event EventData) error {
	// Create InfluxDB point
	p := influxdb2.NewPoint(
		"system_event",                  // measurement name
		map[string]string{               // tags (indexed, for filtering)
			"event_id":   event.ID,
			"event_type": event.Type,
			"source":     event.Source,
			"server_id":  event.ServerID,
			"user_id":    event.UserID,
		},
		event.Data,                      // fields (not indexed, for values)
		event.Timestamp,
	)

	// Write point (non-blocking)
	c.writeAPI.WritePoint(p)

	return nil
}

// Flush ensures all pending writes are sent to InfluxDB
func (c *InfluxDBClient) Flush() {
	c.writeAPI.Flush()
}

// QueryEvents queries events from InfluxDB with filters
func (c *InfluxDBClient) QueryEvents(ctx context.Context, filters EventFilters) ([]EventData, error) {
	// Build Flux query
	query := c.buildFluxQuery(filters)

	result, err := c.queryAPI.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query InfluxDB: %w", err)
	}

	var eventsList []EventData
	for result.Next() {
		record := result.Record()

		// Extract event from record
		event := EventData{
			ID:        record.ValueByKey("event_id").(string),
			Type:      record.ValueByKey("event_type").(string),
			Timestamp: record.Time(),
			Source:    record.ValueByKey("source").(string),
			ServerID:  record.ValueByKey("server_id").(string),
			UserID:    record.ValueByKey("user_id").(string),
			Data:      make(map[string]interface{}),
		}

		// Extract all fields as data
		for k, v := range record.Values() {
			if k != "_time" && k != "_measurement" && k != "event_id" && k != "event_type" && k != "source" && k != "server_id" && k != "user_id" {
				event.Data[k] = v
			}
		}

		eventsList = append(eventsList, event)

		if filters.Limit > 0 && len(eventsList) >= filters.Limit {
			break
		}
	}

	if result.Err() != nil {
		return nil, fmt.Errorf("query parsing failed: %w", result.Err())
	}

	return eventsList, nil
}

// buildFluxQuery builds a Flux query from filters
func (c *InfluxDBClient) buildFluxQuery(filters EventFilters) string {
	query := fmt.Sprintf(`from(bucket: "%s")`, c.bucket)

	// Time range
	if !filters.StartTime.IsZero() {
		query += fmt.Sprintf(`
  |> range(start: %s`, filters.StartTime.Format(time.RFC3339))
		if !filters.EndTime.IsZero() {
			query += fmt.Sprintf(`, stop: %s`, filters.EndTime.Format(time.RFC3339))
		}
		query += ")"
	} else {
		// Default to last 24 hours
		query += `
  |> range(start: -24h)`
	}

	// Filter by measurement
	query += `
  |> filter(fn: (r) => r._measurement == "system_event")`

	// Filter by event types
	if len(filters.Types) > 0 {
		query += `
  |> filter(fn: (r) => `
		for i, eventType := range filters.Types {
			if i > 0 {
				query += " or "
			}
			query += fmt.Sprintf(`r.event_type == "%s"`, eventType)
		}
		query += ")"
	}

	// Filter by server ID
	if filters.ServerID != "" {
		query += fmt.Sprintf(`
  |> filter(fn: (r) => r.server_id == "%s")`, filters.ServerID)
	}

	// Filter by user ID
	if filters.UserID != "" {
		query += fmt.Sprintf(`
  |> filter(fn: (r) => r.user_id == "%s")`, filters.UserID)
	}

	// Sort by time descending
	query += `
  |> sort(columns: ["_time"], desc: true)`

	// Limit
	if filters.Limit > 0 {
		query += fmt.Sprintf(`
  |> limit(n: %d)`, filters.Limit)
	}

	return query
}

// Close closes the InfluxDB client and flushes pending writes
func (c *InfluxDBClient) Close() {
	c.writeAPI.Flush()
	c.client.Close()
	logger.Info("InfluxDB client closed", nil)
}
