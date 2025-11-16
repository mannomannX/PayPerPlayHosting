import { create } from 'zustand';
import type { Node, Edge } from 'reactflow';
import type {
  NodeCreatedEvent,
  NodeStatsEvent,
  ContainerCreatedEvent,
  ContainerStatusChangedEvent,
  MigrationStartedEvent,
  MigrationEvent,
  FleetStatsEvent,
  DashboardEvent,
} from '../types/events';

interface DashboardNode extends Node {
  data: {
    label: string;
    type: 'dedicated' | 'cloud' | 'velocity';
    provider?: string;
    location?: string;
    totalRAM: number;
    usableRAM: number;
    allocatedRAM: number;
    freeRAM: number;
    containerCount: number;
    capacityPercent: number;
    cpuUsagePercent?: number;
    status: string;
    ipAddress: string;
    isSystemNode: boolean;
    containers: ContainerInfo[];
  };
}

interface ContainerInfo {
  server_id: string;
  server_name: string;
  ram_mb: number;
  status: string;
  port: number;
  join_address: string;
  minecraft_version?: string;
  server_type?: string;
}

interface MigrationOperation {
  operation_id: string;
  server_id: string;
  server_name: string;
  from_node: string;
  to_node: string;
  progress: number;
  status: 'started' | 'progress' | 'completed' | 'failed';
  error?: string;
}

interface FleetStats {
  total_nodes: number;
  dedicated_nodes: number;
  cloud_nodes: number;
  total_ram_mb: number;
  usable_ram_mb: number;
  allocated_ram_mb: number;
  free_ram_mb: number;
  capacity_percent: number;
  total_servers: number;
  queue_size?: number;
}

interface QueuedServer {
  ServerID: string;
  ServerName: string;
  RequiredRAMMB: number;
  QueuedAt: string;
  UserID: string;
}

interface QueueInfo {
  queue_size: number;
  servers: QueuedServer[];
}

interface DashboardState {
  nodes: DashboardNode[];
  edges: Edge[];
  migrations: Map<string, MigrationOperation>;
  fleetStats: FleetStats | null;
  queueInfo: QueueInfo | null;
  connected: boolean;
  lastUpdate: Date | null;

  // Actions
  setConnected: (connected: boolean) => void;
  handleEvent: (event: DashboardEvent) => void;
  addNode: (event: NodeCreatedEvent) => void;
  updateNodeStats: (event: NodeStatsEvent) => void;
  removeNode: (nodeId: string) => void;
  addContainer: (event: ContainerCreatedEvent) => void;
  removeContainer: (serverId: string) => void;
  updateContainerStatus: (event: ContainerStatusChangedEvent) => void;
  startMigration: (event: MigrationStartedEvent) => void;
  updateMigration: (event: MigrationEvent) => void;
  updateFleetStats: (event: FleetStatsEvent) => void;
  updateQueue: (queueData: any) => void;
}

export const useDashboardStore = create<DashboardState>((set, get) => ({
  nodes: [],
  edges: [],
  migrations: new Map(),
  fleetStats: null,
  queueInfo: null,
  connected: false,
  lastUpdate: null,

  setConnected: (connected) => set({ connected }),

  handleEvent: (event: DashboardEvent) => {
    const { type, data } = event;

    switch (type) {
      case 'node.created':
        get().addNode(data as NodeCreatedEvent);
        break;
      case 'node.stats':
        get().updateNodeStats(data as NodeStatsEvent);
        break;
      case 'node.removed':
        get().removeNode(data.node_id);
        break;
      case 'container.created':
        get().addContainer(data as ContainerCreatedEvent);
        break;
      case 'container.status_changed':
        get().updateContainerStatus(data as ContainerStatusChangedEvent);
        break;
      case 'container.removed':
        get().removeContainer(data.server_id);
        break;
      case 'operation.migration.started':
        get().startMigration(data as MigrationStartedEvent);
        break;
      case 'operation.migration.progress':
      case 'operation.migration.completed':
      case 'operation.migration.failed':
        get().updateMigration(data as MigrationEvent);
        break;
      case 'stats.fleet':
        get().updateFleetStats(data as FleetStatsEvent);
        break;
      case 'queue.updated':
      case 'queue.server_added':
      case 'queue.server_removed':
        get().updateQueue(data);
        break;
      default:
        console.log('[Store] Unhandled event type:', type);
    }

    set({ lastUpdate: new Date() });
  },

  addNode: (event: NodeCreatedEvent) => {
    const nodeExists = get().nodes.some((n) => n.id === event.node_id);
    if (nodeExists) return;

    // Calculate position based on node type to avoid overlaps
    const currentNodes = get().nodes;
    const tier = event.node_id.includes('proxy') ? 'proxy' : event.node_id.includes('local') || event.node_id.includes('control') ? 'control' : 'workload';
    const typeCount = currentNodes.filter(n => {
      const nTier = n.id.includes('proxy') ? 'proxy' : n.id.includes('local') || n.id.includes('control') ? 'control' : 'workload';
      return nTier === tier;
    }).length;

    // Position nodes in tiers (left to right: control, proxy, workload)
    let x = 200; // Control plane
    if (tier === 'proxy') x = 600; // Proxy layer
    if (tier === 'workload') x = 1000; // Workload layer

    // Stack nodes vertically within the same tier
    const y = 200 + (typeCount * 250);

    const newNode: DashboardNode = {
      id: event.node_id,
      type: event.node_type === 'velocity' ? 'velocityNode' : event.node_type === 'dedicated' ? 'dedicatedNode' : 'cloudNode',
      position: { x, y },
      data: {
        label: event.node_type.toUpperCase(),
        type: event.node_type,
        provider: event.provider,
        location: event.location,
        totalRAM: event.total_ram_mb,
        usableRAM: event.usable_ram_mb,
        allocatedRAM: 0,
        freeRAM: event.usable_ram_mb,
        containerCount: 0,
        capacityPercent: 0,
        status: event.status,
        ipAddress: event.ip_address,
        isSystemNode: event.is_system_node,
        containers: [],
      },
    };

    set({ nodes: [...get().nodes, newNode] });
  },

  updateNodeStats: (event: NodeStatsEvent) => {
    set({
      nodes: get().nodes.map((node) =>
        node.id === event.node_id
          ? {
              ...node,
              data: {
                ...node.data,
                allocatedRAM: event.allocated_ram_mb,
                freeRAM: event.free_ram_mb,
                containerCount: event.container_count,
                capacityPercent: event.capacity_percent,
                cpuUsagePercent: event.cpu_usage_percent,
              },
            }
          : node
      ),
    });
  },

  removeNode: (nodeId: string) => {
    set({
      nodes: get().nodes.filter((n) => n.id !== nodeId),
      edges: get().edges.filter((e) => e.source !== nodeId && e.target !== nodeId),
    });
  },

  addContainer: (event: ContainerCreatedEvent) => {
    set({
      nodes: get().nodes.map((node) =>
        node.id === event.node_id
          ? {
              ...node,
              data: {
                ...node.data,
                containers: [
                  ...node.data.containers,
                  {
                    server_id: event.server_id,
                    server_name: event.server_name,
                    ram_mb: event.ram_mb,
                    status: event.status,
                    port: event.port,
                    join_address: event.join_address,
                    minecraft_version: (event as any).minecraft_version,
                    server_type: (event as any).server_type,
                  },
                ],
              },
            }
          : node
      ),
    });
  },

  removeContainer: (serverId: string) => {
    set({
      nodes: get().nodes.map((node) => ({
        ...node,
        data: {
          ...node.data,
          containers: node.data.containers.filter((c) => c.server_id !== serverId),
        },
      })),
    });
  },

  updateContainerStatus: (event: ContainerStatusChangedEvent) => {
    set({
      nodes: get().nodes.map((node) => ({
        ...node,
        data: {
          ...node.data,
          containers: node.data.containers.map((c) =>
            c.server_id === event.server_id
              ? {
                  ...c,
                  status: event.status,
                  port: event.port,
                  join_address: event.join_address,
                  minecraft_version: (event as any).minecraft_version || c.minecraft_version,
                  server_type: (event as any).server_type || c.server_type,
                }
              : c
          ),
        },
      })),
    });
  },

  startMigration: (event: MigrationStartedEvent) => {
    const migration: MigrationOperation = {
      operation_id: event.operation_id,
      server_id: event.server_id,
      server_name: event.server_name,
      from_node: event.from_node,
      to_node: event.to_node,
      progress: 0,
      status: 'started',
    };

    const newMigrations = new Map(get().migrations);
    newMigrations.set(event.operation_id, migration);

    // Create edge for migration
    const newEdge: Edge = {
      id: `migration-${event.operation_id}`,
      source: event.from_node,
      target: event.to_node,
      type: 'migrationEdge',
      animated: true,
      data: {
        operation_id: event.operation_id,
        server_name: event.server_name,
        progress: 0,
        status: 'started',
      },
    };

    set({
      migrations: newMigrations,
      edges: [...get().edges, newEdge],
    });
  },

  updateMigration: (event: MigrationEvent) => {
    const newMigrations = new Map(get().migrations);
    const migration = newMigrations.get(event.operation_id);

    if (migration) {
      migration.progress = 'progress' in event ? event.progress : migration.progress;
      migration.status = event.status;
      if ('error' in event) {
        migration.error = event.error as string;
      }
      newMigrations.set(event.operation_id, migration);
    }

    // Update edge
    set({
      migrations: newMigrations,
      edges: get().edges.map((edge) =>
        edge.id === `migration-${event.operation_id}`
          ? {
              ...edge,
              data: {
                ...edge.data,
                progress: 'progress' in event ? event.progress : edge.data?.progress || 0,
                status: event.status,
              },
              animated: event.status !== 'completed' && event.status !== 'failed',
            }
          : edge
      ),
    });

    // Remove completed/failed edges after 5 seconds
    if (event.status === 'completed' || event.status === 'failed') {
      setTimeout(() => {
        set({
          edges: get().edges.filter((e) => e.id !== `migration-${event.operation_id}`),
        });
        const newMigrations = new Map(get().migrations);
        newMigrations.delete(event.operation_id);
        set({ migrations: newMigrations });
      }, 5000);
    }
  },

  updateFleetStats: (event: FleetStatsEvent) => {
    set({ fleetStats: event });
  },

  updateQueue: (queueData: any) => {
    set({
      queueInfo: {
        queue_size: queueData.queue_size || 0,
        servers: queueData.servers || [],
      },
    });
  },
}));
