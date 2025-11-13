package conductor

import (
	"sync"
	"time"
)

// DebugLogEntry represents a single log entry for the debug console
type DebugLogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"` // INFO, WARN, ERROR
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// DebugLogBuffer stores recent important log events for the dashboard debug console
type DebugLogBuffer struct {
	entries []DebugLogEntry
	maxSize int
	mutex   sync.RWMutex
}

// NewDebugLogBuffer creates a new debug log buffer
func NewDebugLogBuffer(maxSize int) *DebugLogBuffer {
	return &DebugLogBuffer{
		entries: make([]DebugLogEntry, 0, maxSize),
		maxSize: maxSize,
	}
}

// Add adds a new log entry (circular buffer)
func (b *DebugLogBuffer) Add(level, message string, fields map[string]interface{}) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	entry := DebugLogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Fields:    fields,
	}

	// Circular buffer: remove oldest if at capacity
	if len(b.entries) >= b.maxSize {
		b.entries = b.entries[1:]
	}

	b.entries = append(b.entries, entry)
}

// GetAll returns all log entries (newest first)
func (b *DebugLogBuffer) GetAll() []DebugLogEntry {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	// Return copy in reverse order (newest first)
	result := make([]DebugLogEntry, len(b.entries))
	for i, j := 0, len(b.entries)-1; j >= 0; i, j = i+1, j-1 {
		result[i] = b.entries[j]
	}

	return result
}

// Clear removes all log entries
func (b *DebugLogBuffer) Clear() {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.entries = make([]DebugLogEntry, 0, b.maxSize)
}
