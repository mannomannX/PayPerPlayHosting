package events

// PublishServerCreated publishes a server created event
func PublishServerCreated(serverID, userID, serverType string) {
	GetEventBus().Publish(Event{
		Type:     EventServerCreated,
		Source:   "minecraft_service",
		ServerID: serverID,
		UserID:   userID,
		Data: map[string]interface{}{
			"server_type": serverType,
		},
	})
}

// PublishServerStarted publishes a server started event
func PublishServerStarted(serverID, userID string) {
	GetEventBus().Publish(Event{
		Type:     EventServerStarted,
		Source:   "minecraft_service",
		ServerID: serverID,
		UserID:   userID,
		Data:     map[string]interface{}{},
	})
}

// PublishServerStopped publishes a server stopped event
func PublishServerStopped(serverID, reason string) {
	GetEventBus().Publish(Event{
		Type:     EventServerStopped,
		Source:   "minecraft_service",
		ServerID: serverID,
		Data: map[string]interface{}{
			"reason": reason,
		},
	})
}

// PublishServerDeleted publishes a server deleted event
func PublishServerDeleted(serverID, userID string) {
	GetEventBus().Publish(Event{
		Type:     EventServerDeleted,
		Source:   "minecraft_service",
		ServerID: serverID,
		UserID:   userID,
		Data:     map[string]interface{}{},
	})
}

// PublishServerCrashed publishes a server crashed event
func PublishServerCrashed(serverID string, exitCode int, errorMessage string) {
	GetEventBus().Publish(Event{
		Type:     EventServerCrashed,
		Source:   "recovery_service",
		ServerID: serverID,
		Data: map[string]interface{}{
			"exit_code":     exitCode,
			"error_message": errorMessage,
		},
	})
}

// PublishServerRestarted publishes a server restarted event
func PublishServerRestarted(serverID, reason string) {
	GetEventBus().Publish(Event{
		Type:     EventServerRestarted,
		Source:   "recovery_service",
		ServerID: serverID,
		Data: map[string]interface{}{
			"reason": reason,
		},
	})
}

// PublishPlayerJoined publishes a player joined event
func PublishPlayerJoined(serverID, playerName string, playerCount int) {
	GetEventBus().Publish(Event{
		Type:     EventPlayerJoined,
		Source:   "monitoring_service",
		ServerID: serverID,
		Data: map[string]interface{}{
			"player_name":  playerName,
			"player_count": playerCount,
		},
	})
}

// PublishPlayerLeft publishes a player left event
func PublishPlayerLeft(serverID, playerName string, playerCount int) {
	GetEventBus().Publish(Event{
		Type:     EventPlayerLeft,
		Source:   "monitoring_service",
		ServerID: serverID,
		Data: map[string]interface{}{
			"player_name":  playerName,
			"player_count": playerCount,
		},
	})
}

// PublishBackupCreated publishes a backup created event
func PublishBackupCreated(serverID, backupFile string, sizeBytes int64) {
	GetEventBus().Publish(Event{
		Type:     EventBackupCreated,
		Source:   "backup_service",
		ServerID: serverID,
		Data: map[string]interface{}{
			"backup_file": backupFile,
			"size_bytes":  sizeBytes,
		},
	})
}

// PublishBackupRestored publishes a backup restored event
func PublishBackupRestored(serverID, backupFile string) {
	GetEventBus().Publish(Event{
		Type:     EventBackupRestored,
		Source:   "backup_service",
		ServerID: serverID,
		Data: map[string]interface{}{
			"backup_file": backupFile,
		},
	})
}

// PublishBackupFailed publishes a backup failed event
func PublishBackupFailed(serverID, errorMessage string) {
	GetEventBus().Publish(Event{
		Type:     EventBackupFailed,
		Source:   "backup_service",
		ServerID: serverID,
		Data: map[string]interface{}{
			"error": errorMessage,
		},
	})
}

// PublishBillingPhaseChanged publishes a billing phase change event
func PublishBillingPhaseChanged(serverID, oldPhase, newPhase string) {
	GetEventBus().Publish(Event{
		Type:     EventBillingPhaseChanged,
		Source:   "lifecycle_service",
		ServerID: serverID,
		Data: map[string]interface{}{
			"old_phase": oldPhase,
			"new_phase": newPhase,
		},
	})
}

// PublishScalingTriggered publishes a scaling triggered event
func PublishScalingTriggered(reason string, nodeCount int, action string) {
	GetEventBus().Publish(Event{
		Type:   EventScalingTriggered,
		Source: "conductor",
		Data: map[string]interface{}{
			"reason":     reason,
			"node_count": nodeCount,
			"action":     action,
		},
	})
}

// PublishScalingEvent publishes a scaling event (scale_up, scale_down, provision_spare)
func PublishScalingEvent(action, status, details string) {
	GetEventBus().Publish(Event{
		Type:   EventScalingTriggered,
		Source: "scaling_engine",
		Data: map[string]interface{}{
			"action":  action,
			"status":  status,
			"details": details,
		},
	})
}

// PublishNodeAdded publishes a node added event
func PublishNodeAdded(nodeID, nodeType string) {
	GetEventBus().Publish(Event{
		Type:   EventNodeAdded,
		Source: "vm_provisioner",
		Data: map[string]interface{}{
			"node_id":   nodeID,
			"node_type": nodeType,
		},
	})
}

// PublishNodeRemoved publishes a node removed event
func PublishNodeRemoved(nodeID, reason string) {
	GetEventBus().Publish(Event{
		Type:   EventNodeRemoved,
		Source: "vm_provisioner",
		Data: map[string]interface{}{
			"node_id": nodeID,
			"reason":  reason,
		},
	})
}

// PublishNodeHealthChanged publishes a node health change event
func PublishNodeHealthChanged(nodeID, oldStatus, newStatus string) {
	GetEventBus().Publish(Event{
		Type:   EventNodeHealthChanged,
		Source: "health_checker",
		Data: map[string]interface{}{
			"node_id":    nodeID,
			"old_status": oldStatus,
			"new_status": newStatus,
		},
	})
}
