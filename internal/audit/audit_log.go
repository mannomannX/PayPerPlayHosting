package audit

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/payperplay/hosting/pkg/logger"
)

// ActionType represents the type of action being audited
type ActionType string

const (
	ActionNodeDecommission ActionType = "node_decommission"
	ActionNodeProvision    ActionType = "node_provision"
	ActionContainerMigrate ActionType = "container_migrate"
	ActionScaleUp          ActionType = "scale_up"
	ActionScaleDown        ActionType = "scale_down"
)

// AuditEntry represents a single audit log entry
type AuditEntry struct {
	Timestamp     time.Time              `json:"timestamp"`
	Action        ActionType             `json:"action"`
	NodeID        string                 `json:"node_id,omitempty"`
	ContainerID   string                 `json:"container_id,omitempty"`
	Reason        string                 `json:"reason"`
	StateSnapshot map[string]interface{} `json:"state_snapshot"`
	DecisionBy    string                 `json:"decision_by"` // "reactive_policy", "consolidation_policy", "manual"
	Result        string                 `json:"result"`      // "success", "rejected", "failed"
	Error         string                 `json:"error,omitempty"`
}

// AuditLogger logs all destructive actions for accountability
type AuditLogger struct {
	entries []AuditEntry
	mu      sync.RWMutex
	maxSize int // Maximum entries to keep in memory
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(maxSize int) *AuditLogger {
	if maxSize <= 0 {
		maxSize = 1000 // Default
	}

	return &AuditLogger{
		entries: make([]AuditEntry, 0, maxSize),
		maxSize: maxSize,
	}
}

// Record adds an entry to the audit log
func (a *AuditLogger) Record(entry AuditEntry) {
	a.mu.Lock()
	defer a.mu.Unlock()

	entry.Timestamp = time.Now()

	// Add to in-memory log
	a.entries = append(a.entries, entry)

	// Trim if exceeded max size (keep most recent)
	if len(a.entries) > a.maxSize {
		a.entries = a.entries[len(a.entries)-a.maxSize:]
	}

	// Log to structured logger
	fields := map[string]interface{}{
		"action":      entry.Action,
		"node_id":     entry.NodeID,
		"container_id": entry.ContainerID,
		"reason":      entry.Reason,
		"decision_by": entry.DecisionBy,
		"result":      entry.Result,
	}

	// Add state snapshot (but don't log entire snapshot, too verbose)
	if len(entry.StateSnapshot) > 0 {
		snapshotJSON, _ := json.Marshal(entry.StateSnapshot)
		fields["state_snapshot"] = string(snapshotJSON)
	}

	if entry.Error != "" {
		fields["error"] = entry.Error
	}

	switch entry.Result {
	case "success":
		logger.Info("AUDIT: "+string(entry.Action), fields)
	case "rejected":
		logger.Warn("AUDIT: "+string(entry.Action)+" REJECTED", fields)
	case "failed":
		logger.Error("AUDIT: "+string(entry.Action)+" FAILED", nil, fields)
	default:
		logger.Info("AUDIT: "+string(entry.Action), fields)
	}
}

// RecordNodeDecommission records a node decommission attempt
func (a *AuditLogger) RecordNodeDecommission(nodeID string, reason string, decisionBy string, state map[string]interface{}, result string, err error) {
	entry := AuditEntry{
		Action:        ActionNodeDecommission,
		NodeID:        nodeID,
		Reason:        reason,
		StateSnapshot: state,
		DecisionBy:    decisionBy,
		Result:        result,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	a.Record(entry)
}

// RecordNodeProvision records a node provisioning
func (a *AuditLogger) RecordNodeProvision(nodeID string, reason string, decisionBy string, state map[string]interface{}, result string, err error) {
	entry := AuditEntry{
		Action:        ActionNodeProvision,
		NodeID:        nodeID,
		Reason:        reason,
		StateSnapshot: state,
		DecisionBy:    decisionBy,
		Result:        result,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	a.Record(entry)
}

// GetRecent returns the N most recent audit entries
func (a *AuditLogger) GetRecent(n int) []AuditEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if n <= 0 || n > len(a.entries) {
		n = len(a.entries)
	}

	// Return most recent N entries (from end of slice)
	start := len(a.entries) - n
	result := make([]AuditEntry, n)
	copy(result, a.entries[start:])

	return result
}

// GetByNodeID returns all audit entries for a specific node
func (a *AuditLogger) GetByNodeID(nodeID string) []AuditEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []AuditEntry
	for _, entry := range a.entries {
		if entry.NodeID == nodeID {
			result = append(result, entry)
		}
	}

	return result
}

// GetByAction returns all audit entries for a specific action type
func (a *AuditLogger) GetByAction(action ActionType) []AuditEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []AuditEntry
	for _, entry := range a.entries {
		if entry.Action == action {
			result = append(result, entry)
		}
	}

	return result
}

// Stats returns audit statistics
func (a *AuditLogger) Stats() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats := map[string]interface{}{
		"total_entries": len(a.entries),
		"max_size":      a.maxSize,
	}

	// Count by action type
	actionCounts := make(map[ActionType]int)
	resultCounts := make(map[string]int)

	for _, entry := range a.entries {
		actionCounts[entry.Action]++
		resultCounts[entry.Result]++
	}

	stats["by_action"] = actionCounts
	stats["by_result"] = resultCounts

	// Most recent entry
	if len(a.entries) > 0 {
		lastEntry := a.entries[len(a.entries)-1]
		stats["last_action"] = lastEntry.Action
		stats["last_timestamp"] = lastEntry.Timestamp
	}

	return stats
}

// String returns a human-readable audit log summary
func (a *AuditLogger) String() string {
	stats := a.Stats()
	statsJSON, _ := json.MarshalIndent(stats, "", "  ")
	return fmt.Sprintf("Audit Log Stats:\n%s", string(statsJSON))
}
