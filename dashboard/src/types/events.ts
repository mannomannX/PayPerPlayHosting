// Dashboard Event Types matching backend events

export interface DashboardEvent {
  type: string;
  timestamp: string;
  data: any;
}

// Node Events
export interface NodeCreatedEvent {
  node_id: string;
  node_type: 'dedicated' | 'cloud' | 'velocity';
  provider: string;
  location: string;
  total_ram_mb: number;
  usable_ram_mb: number;
  status: string;
  ip_address: string;
  is_system_node: boolean;
  created_at: string;
}

export interface NodeStatsEvent {
  node_id: string;
  allocated_ram_mb: number;
  free_ram_mb: number;
  container_count: number;
  capacity_percent: number;
  cpu_usage_percent?: number;
}

export interface NodeRemovedEvent {
  node_id: string;
  reason: string;
}

// Container Events
export interface ContainerCreatedEvent {
  server_id: string;
  server_name: string;
  node_id: string;
  ram_mb: number;
  status: string;
  port: number;
  join_address: string;
}

export interface ContainerStatusChangedEvent {
  server_id: string;
  server_name: string;
  node_id: string;
  status: string;
  port: number;
  join_address: string;
  timestamp: string;
}

export interface ContainerRemovedEvent {
  server_id: string;
  server_name: string;
  node_id: string;
  reason: string;
}

// Migration Events
export interface MigrationStartedEvent {
  operation_id: string;
  server_id: string;
  server_name: string;
  from_node: string;
  to_node: string;
  ram_mb: number;
  player_count: number;
  status: 'started';
}

export interface MigrationProgressEvent {
  operation_id: string;
  server_id: string;
  status: 'progress';
  progress: number;
  message: string;
}

export interface MigrationCompletedEvent {
  operation_id: string;
  server_id: string;
  status: 'completed';
  progress: 100;
}

export interface MigrationFailedEvent {
  operation_id: string;
  server_id: string;
  status: 'failed';
  error: string;
}

// Union type for all migration events
export type MigrationEvent =
  | MigrationStartedEvent
  | MigrationProgressEvent
  | MigrationCompletedEvent
  | MigrationFailedEvent;

// Scaling Events
export interface ScalingDecisionEvent {
  policy_name: string;
  action: 'scale_up' | 'scale_down' | 'consolidate' | 'none';
  server_type?: string;
  count?: number;
  reason: string;
  urgency: 'low' | 'medium' | 'high' | 'critical';
  capacity_percent: number;
  decision_tree?: Record<string, any>;
}

export interface ScalingActionEvent {
  action: string;
  details: Record<string, any>;
}

// Consolidation Events
export interface ConsolidationStartedEvent {
  migration_count: number;
  nodes_before: number;
  nodes_after: number;
  node_savings: number;
  estimated_cost_savings: number;
  reason: string;
  nodes_to_remove: string[];
}

export interface ConsolidationCompletedEvent {
  status: 'completed';
  migrations_completed: number;
  migrations_failed: number;
}

// Fleet Stats
export interface FleetStatsEvent {
  total_nodes: number;
  dedicated_nodes: number;
  cloud_nodes: number;
  total_ram_mb: number;
  usable_ram_mb: number;
  allocated_ram_mb: number;
  free_ram_mb: number;
  capacity_percent: number;
  total_servers: number;
}

// Velocity Stats
export interface VelocityStatsEvent {
  total_players: number;
  total_servers: number;
  server_stats: Array<{
    name: string;
    players: number;
  }>;
}
