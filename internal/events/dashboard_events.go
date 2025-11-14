package events

import (
	"time"

	"github.com/payperplay/hosting/pkg/logger"
)

// DashboardEventPublisher is the global dashboard event publisher
// This is set by the API server when initializing the WebSocket
var DashboardEventPublisher DashboardPublisher

// DashboardPublisher interface for publishing dashboard events
type DashboardPublisher interface {
	PublishEvent(eventType string, data interface{})
}

// PublishNodeCreated publishes a node creation event
func PublishNodeCreated(nodeID, nodeType, provider, location, status, ipAddress string, totalRAMMB, usableRAMMB int, isSystemNode bool, createdAt time.Time) {
	if DashboardEventPublisher == nil {
		return
	}

	data := map[string]interface{}{
		"node_id":        nodeID,
		"node_type":      nodeType,
		"provider":       provider,
		"location":       location,
		"total_ram_mb":   totalRAMMB,
		"usable_ram_mb":  usableRAMMB,
		"status":         status,
		"ip_address":     ipAddress,
		"is_system_node": isSystemNode,
		"created_at":     createdAt,
	}

	DashboardEventPublisher.PublishEvent("node.created", data)
	logger.Debug("Dashboard event published: node.created", map[string]interface{}{
		"node_id":        nodeID,
		"is_system_node": isSystemNode,
	})
}

// PublishNodeStatsUpdate publishes node statistics update
func PublishNodeStatsUpdate(nodeID string, usableRAMMB, allocatedRAMMB, freeRAMMB, containerCount int) {
	if DashboardEventPublisher == nil {
		return
	}

	capacityPercent := 0.0
	if usableRAMMB > 0 {
		capacityPercent = (float64(allocatedRAMMB) / float64(usableRAMMB)) * 100
	}

	data := map[string]interface{}{
		"node_id":          nodeID,
		"allocated_ram_mb": allocatedRAMMB,
		"free_ram_mb":      freeRAMMB,
		"container_count":  containerCount,
		"capacity_percent": capacityPercent,
	}

	DashboardEventPublisher.PublishEvent("node.stats", data)
}

// PublishNodeStats publishes comprehensive node statistics including CPU metrics
func PublishNodeStats(nodeID string, allocatedRAMMB, freeRAMMB, containerCount int, capacityPercent, cpuUsagePercent float64) {
	if DashboardEventPublisher == nil {
		return
	}

	data := map[string]interface{}{
		"node_id":           nodeID,
		"allocated_ram_mb":  allocatedRAMMB,
		"free_ram_mb":       freeRAMMB,
		"container_count":   containerCount,
		"capacity_percent":  capacityPercent,
		"cpu_usage_percent": cpuUsagePercent,
	}

	DashboardEventPublisher.PublishEvent("node.stats", data)
}

// PublishDashboardNodeRemoved publishes a node removal event to dashboard
func PublishDashboardNodeRemoved(nodeID string, reason string) {
	if DashboardEventPublisher == nil {
		return
	}

	data := map[string]interface{}{
		"node_id": nodeID,
		"reason":  reason,
	}

	DashboardEventPublisher.PublishEvent("node.removed", data)
	logger.Debug("Dashboard event published: node.removed", map[string]interface{}{
		"node_id": nodeID,
	})
}

// PublishContainerCreated publishes a container creation event
func PublishContainerCreated(serverID, serverName, nodeID string, ramMb, port int, status, joinAddress string) {
	if DashboardEventPublisher == nil {
		return
	}

	data := map[string]interface{}{
		"server_id":    serverID,
		"server_name":  serverName,
		"node_id":      nodeID,
		"ram_mb":       ramMb,
		"status":       status,
		"port":         port,
		"join_address": joinAddress,
	}

	DashboardEventPublisher.PublishEvent("container.created", data)
	logger.Debug("Dashboard event published: container.created", map[string]interface{}{
		"server_id": serverID,
		"node_id":   nodeID,
		"port":      port,
	})
}

// PublishContainerStatusChanged publishes a container status change event
func PublishContainerStatusChanged(serverID, serverName, nodeID, status string, port int, joinAddress string) {
	if DashboardEventPublisher == nil {
		return
	}

	data := map[string]interface{}{
		"server_id":    serverID,
		"server_name":  serverName,
		"node_id":      nodeID,
		"status":       status,
		"port":         port,
		"join_address": joinAddress,
		"timestamp":    time.Now(),
	}

	DashboardEventPublisher.PublishEvent("container.status_changed", data)
	logger.Debug("Dashboard event published: container.status_changed", map[string]interface{}{
		"server_id": serverID,
		"status":    status,
		"port":      port,
	})
}

// PublishContainerRemoved publishes a container removal event
func PublishContainerRemoved(serverID, serverName, nodeID, reason string) {
	if DashboardEventPublisher == nil {
		return
	}

	data := map[string]interface{}{
		"server_id":   serverID,
		"server_name": serverName,
		"node_id":     nodeID,
		"reason":      reason,
	}

	DashboardEventPublisher.PublishEvent("container.removed", data)
	logger.Debug("Dashboard event published: container.removed", map[string]interface{}{
		"server_id": serverID,
		"node_id":   nodeID,
	})
}

// PublishMigrationStarted publishes a migration start event
func PublishMigrationStarted(operationID, serverID, serverName, fromNode, toNode string, ramMb, playerCount int) {
	if DashboardEventPublisher == nil {
		return
	}

	data := map[string]interface{}{
		"operation_id": operationID,
		"server_id":    serverID,
		"server_name":  serverName,
		"from_node":    fromNode,
		"to_node":      toNode,
		"ram_mb":       ramMb,
		"player_count": playerCount,
		"status":       "started",
	}

	DashboardEventPublisher.PublishEvent("operation.migration.started", data)
	logger.Info("Dashboard event published: operation.migration.started", map[string]interface{}{
		"operation_id": operationID,
		"server_id":    serverID,
	})
}

// PublishMigrationProgress publishes a migration progress event
func PublishMigrationProgress(operationID, serverID string, progress int, message string) {
	if DashboardEventPublisher == nil {
		return
	}

	data := map[string]interface{}{
		"operation_id": operationID,
		"server_id":    serverID,
		"status":       "progress",
		"progress":     progress,
		"message":      message,
	}

	DashboardEventPublisher.PublishEvent("operation.migration.progress", data)
}

// PublishMigrationCompleted publishes a migration completion event
func PublishMigrationCompleted(operationID, serverID string) {
	if DashboardEventPublisher == nil {
		return
	}

	data := map[string]interface{}{
		"operation_id": operationID,
		"server_id":    serverID,
		"status":       "completed",
		"progress":     100,
	}

	DashboardEventPublisher.PublishEvent("operation.migration.completed", data)
	logger.Info("Dashboard event published: operation.migration.completed", map[string]interface{}{
		"operation_id": operationID,
		"server_id":    serverID,
	})
}

// PublishMigrationFailed publishes a migration failure event
func PublishMigrationFailed(operationID, serverID, errorMsg string) {
	if DashboardEventPublisher == nil {
		return
	}

	data := map[string]interface{}{
		"operation_id": operationID,
		"server_id":    serverID,
		"status":       "failed",
		"error":        errorMsg,
	}

	DashboardEventPublisher.PublishEvent("operation.migration.failed", data)
	logger.Info("Dashboard event published: operation.migration.failed", map[string]interface{}{
		"operation_id": operationID,
		"server_id":    serverID,
		"error":        errorMsg,
	})
}

// PublishScalingDecision publishes a scaling decision event
func PublishScalingDecision(policyName, action, serverType, reason, urgency string, count int, capacityPercent float64, decisionTree map[string]interface{}) {
	if DashboardEventPublisher == nil {
		return
	}

	data := map[string]interface{}{
		"policy_name":      policyName,
		"action":           action,
		"server_type":      serverType,
		"count":            count,
		"reason":           reason,
		"urgency":          urgency,
		"capacity_percent": capacityPercent,
		"decision_tree":    decisionTree,
	}

	DashboardEventPublisher.PublishEvent("scaling.decision", data)
	logger.Info("Dashboard event published: scaling.decision", map[string]interface{}{
		"policy": policyName,
		"action": action,
	})
}

// PublishScalingAction publishes a scaling action execution event
func PublishScalingAction(action string, details map[string]interface{}) {
	if DashboardEventPublisher == nil {
		return
	}

	data := map[string]interface{}{
		"action":  action,
		"details": details,
	}

	DashboardEventPublisher.PublishEvent("scaling.action", data)
	logger.Info("Dashboard event published: scaling.action", map[string]interface{}{
		"action": action,
	})
}

// PublishConsolidationStarted publishes a consolidation operation start event
func PublishConsolidationStarted(migrationCount, nodesBefore, nodesAfter, nodeSavings int, estimatedCostSavings float64, reason string, nodesToRemove []string) {
	if DashboardEventPublisher == nil {
		return
	}

	data := map[string]interface{}{
		"migration_count":        migrationCount,
		"nodes_before":           nodesBefore,
		"nodes_after":            nodesAfter,
		"node_savings":           nodeSavings,
		"estimated_cost_savings": estimatedCostSavings,
		"reason":                 reason,
		"nodes_to_remove":        nodesToRemove,
	}

	DashboardEventPublisher.PublishEvent("operation.consolidation.started", data)
	logger.Info("Dashboard event published: operation.consolidation.started", map[string]interface{}{
		"migrations":   migrationCount,
		"node_savings": nodeSavings,
	})
}

// PublishConsolidationCompleted publishes a consolidation completion event
func PublishConsolidationCompleted(migrationsCompleted, migrationsFailed int) {
	if DashboardEventPublisher == nil {
		return
	}

	data := map[string]interface{}{
		"status":               "completed",
		"migrations_completed": migrationsCompleted,
		"migrations_failed":    migrationsFailed,
	}

	DashboardEventPublisher.PublishEvent("operation.consolidation.completed", data)
	logger.Info("Dashboard event published: operation.consolidation.completed", map[string]interface{}{
		"completed": migrationsCompleted,
		"failed":    migrationsFailed,
	})
}

// PublishVelocityStats publishes Velocity proxy statistics
func PublishVelocityStats(totalPlayers, totalServers int, serverStats []map[string]interface{}) {
	if DashboardEventPublisher == nil {
		return
	}

	data := map[string]interface{}{
		"total_players": totalPlayers,
		"total_servers": totalServers,
		"server_stats":  serverStats,
	}

	DashboardEventPublisher.PublishEvent("stats.velocity", data)
}

// PublishQueueUpdated publishes a deployment queue update event
func PublishQueueUpdated(queueSize int, servers interface{}) {
	if DashboardEventPublisher == nil {
		return
	}

	data := map[string]interface{}{
		"queue_size": queueSize,
		"servers":    servers,
	}

	DashboardEventPublisher.PublishEvent("queue.updated", data)
	logger.Debug("Dashboard event published: queue.updated", map[string]interface{}{
		"queue_size": queueSize,
	})
}

// PublishServerQueued publishes a server queued event
func PublishServerQueued(serverID, serverName string, ramMb, position int) {
	if DashboardEventPublisher == nil {
		return
	}

	data := map[string]interface{}{
		"server_id":   serverID,
		"server_name": serverName,
		"ram_mb":      ramMb,
		"position":    position,
	}

	DashboardEventPublisher.PublishEvent("queue.server_added", data)
	logger.Info("Dashboard event published: queue.server_added", map[string]interface{}{
		"server_id": serverID,
		"position":  position,
	})
}

// PublishServerDequeued publishes a server dequeued event
func PublishServerDequeued(serverID, serverName string) {
	if DashboardEventPublisher == nil {
		return
	}

	data := map[string]interface{}{
		"server_id":   serverID,
		"server_name": serverName,
	}

	DashboardEventPublisher.PublishEvent("queue.server_removed", data)
	logger.Info("Dashboard event published: queue.server_removed", map[string]interface{}{
		"server_id": serverID,
	})
}
