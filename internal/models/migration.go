package models

import (
	"time"

	"gorm.io/gorm"
)

// MigrationStatus represents the current state of a migration
type MigrationStatus string

const (
	MigrationStatusSuggested    MigrationStatus = "suggested"    // Suggested by cost optimization
	MigrationStatusApproved     MigrationStatus = "approved"     // Approved by admin
	MigrationStatusScheduled    MigrationStatus = "scheduled"    // Scheduled for execution
	MigrationStatusPreparing    MigrationStatus = "preparing"    // Preparing (starting new container)
	MigrationStatusTransferring MigrationStatus = "transferring" // Transferring players
	MigrationStatusCompleting   MigrationStatus = "completing"   // Finalizing (stopping old container)
	MigrationStatusCompleted    MigrationStatus = "completed"    // Successfully completed
	MigrationStatusFailed       MigrationStatus = "failed"       // Failed with error
	MigrationStatusCancelled    MigrationStatus = "cancelled"    // Manually cancelled
)

// MigrationReason represents why a migration was triggered
type MigrationReason string

const (
	MigrationReasonCostOptimization MigrationReason = "cost-optimization" // Cost optimization
	MigrationReasonManual           MigrationReason = "manual"            // Manual admin request
	MigrationReasonRebalancing      MigrationReason = "rebalancing"       // Load rebalancing
	MigrationReasonMaintenance      MigrationReason = "maintenance"       // Node maintenance
)

// Migration represents a server migration between nodes
type Migration struct {
	ID       string          `gorm:"type:varchar(36);primaryKey" json:"id"`
	ServerID string          `gorm:"type:varchar(36);not null;index:idx_migrations_server" json:"server_id"`
	Server   MinecraftServer `gorm:"foreignKey:ServerID;constraint:OnDelete:CASCADE" json:"-"`

	// Source and target nodes
	FromNodeID   string `gorm:"type:varchar(36);not null" json:"from_node_id"`
	FromNodeName string `gorm:"type:varchar(255)" json:"from_node_name"`
	ToNodeID     string `gorm:"type:varchar(36);not null" json:"to_node_id"`
	ToNodeName   string `gorm:"type:varchar(255)" json:"to_node_name"`

	// Status tracking
	Status MigrationStatus `gorm:"type:varchar(20);not null;index:idx_migrations_status" json:"status"`
	Reason MigrationReason `gorm:"type:varchar(50);not null" json:"reason"`

	// Cost information
	SavingsEURHour  float64 `gorm:"type:decimal(10,4)" json:"savings_eur_hour,omitempty"`
	SavingsEURMonth float64 `gorm:"type:decimal(10,2)" json:"savings_eur_month,omitempty"`

	// Timestamps
	CreatedAt   time.Time  `gorm:"not null;index:idx_migrations_created" json:"created_at"`
	ApprovedAt  *time.Time `json:"approved_at,omitempty"`
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Progress tracking
	PlayerCountAtStart int `gorm:"default:0" json:"player_count_at_start"`
	DataSyncProgress   int `gorm:"default:0" json:"data_sync_progress"` // 0-100%

	// Error handling
	ErrorMessage string `gorm:"type:text" json:"error_message,omitempty"`
	RetryCount   int    `gorm:"default:0" json:"retry_count"`
	MaxRetries   int    `gorm:"default:3" json:"max_retries"`

	// Metadata
	TriggeredBy string `gorm:"type:varchar(50)" json:"triggered_by"` // system, admin, user
	Notes       string `gorm:"type:text" json:"notes,omitempty"`

	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for GORM
func (Migration) TableName() string {
	return "migrations"
}

// IsActive returns true if migration is in an active state
func (m *Migration) IsActive() bool {
	return m.Status == MigrationStatusPreparing ||
		m.Status == MigrationStatusTransferring ||
		m.Status == MigrationStatusCompleting
}

// IsCompleted returns true if migration is in a final state
func (m *Migration) IsCompleted() bool {
	return m.Status == MigrationStatusCompleted ||
		m.Status == MigrationStatusFailed ||
		m.Status == MigrationStatusCancelled
}

// CanBeCancelled returns true if migration can be cancelled
func (m *Migration) CanBeCancelled() bool {
	return m.Status == MigrationStatusSuggested ||
		m.Status == MigrationStatusApproved ||
		m.Status == MigrationStatusScheduled
}

// DurationSeconds returns the duration of the migration in seconds
func (m *Migration) DurationSeconds() int {
	if m.StartedAt == nil || m.CompletedAt == nil {
		return 0
	}
	return int(m.CompletedAt.Sub(*m.StartedAt).Seconds())
}
