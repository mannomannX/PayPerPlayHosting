import { create } from 'zustand';
import type { Node, Edge } from 'reactflow';
import type {
  NodeCreatedEvent,
  NodeStatsEvent,
  ContainerCreatedEvent,
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
    containers: ContainerInfo[];
  };
}

interface ContainerInfo {
  server_id: string;
  server_name: string;
  ram_mb: number;
  status: string;
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
}

interface DashboardState {
  nodes: DashboardNode[];
  edges: Edge[];
  migrations: Map<string, MigrationOperation>;
  fleetStats: FleetStats | null;
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
  startMigration: (event: MigrationStartedEvent) => void;
  updateMigration: (event: MigrationEvent) => void;
  updateFleetStats: (event: FleetStatsEvent) => void;
}

export const useDashboardStore = create<DashboardState>((set, get) => ({
  nodes: [],
  edges: [],
  migrations: new Map(),
  fleetStats: null,
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
      default:
        console.log('[Store] Unhandled event type:', type);
    }

    set({ lastUpdate: new Date() });
  },

  addNode: (event: NodeCreatedEvent) => {
    const nodeExists = get().nodes.some((n) => n.id === event.node_id);
    if (nodeExists) return;

    const newNode: DashboardNode = {
      id: event.node_id,
      type: event.node_type === 'velocity' ? 'velocityNode' : event.node_type === 'dedicated' ? 'dedicatedNode' : 'cloudNode',
      position: {
        x: Math.random() * 500 + 100,
        y: Math.random() * 300 + 100,
      },
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
}));
