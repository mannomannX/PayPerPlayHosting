import { useCallback, useMemo } from 'react';
import ReactFlow, {
  Background,
  Controls,
  MiniMap,
  BackgroundVariant,
} from 'reactflow';
import type { NodeTypes, EdgeTypes } from 'reactflow';
import 'reactflow/dist/style.css';
import { motion } from 'framer-motion';

import { useDashboardStore } from '../../stores/dashboardStore';
import { useWebSocket } from '../../hooks/useWebSocket';
import { CloudNode } from '../nodes/CloudNode';
import { DedicatedNode } from '../nodes/DedicatedNode';
import { VelocityNode } from '../nodes/VelocityNode';
import { MigrationEdge } from '../edges/MigrationEdge';

// WebSocket URL - uses nginx proxy (no port needed, nginx forwards /api/ to backend)
const WS_URL = `ws://${window.location.hostname}/api/admin/dashboard/stream`;

export const Dashboard = () => {
  const {
    nodes,
    edges,
    fleetStats,
    connected,
    setConnected,
    handleEvent,
  } = useDashboardStore();

  // WebSocket connection
  useWebSocket({
    url: WS_URL,
    onMessage: handleEvent,
    onConnect: () => setConnected(true),
    onDisconnect: () => setConnected(false),
    onError: (error) => console.error('[Dashboard] WebSocket error:', error),
  });

  // Define custom node types
  const nodeTypes: NodeTypes = useMemo(
    () => ({
      cloudNode: CloudNode,
      dedicatedNode: DedicatedNode,
      velocityNode: VelocityNode,
    }),
    []
  );

  // Define custom edge types
  const edgeTypes: EdgeTypes = useMemo(
    () => ({
      migrationEdge: MigrationEdge,
    }),
    []
  );

  // Mini map node colors
  const nodeColor = useCallback((node: any) => {
    switch (node.type) {
      case 'cloudNode':
        return '#667eea';
      case 'dedicatedNode':
        return '#134e4a';
      case 'velocityNode':
        return '#0ea5e9';
      default:
        return '#6b7280';
    }
  }, []);

  return (
    <div style={{ width: '100vw', height: '100vh', background: '#0f172a' }}>
      {/* Header */}
      <motion.div
        initial={{ y: -100, opacity: 0 }}
        animate={{ y: 0, opacity: 1 }}
        style={{
          position: 'absolute',
          top: 0,
          left: 0,
          right: 0,
          zIndex: 10,
          background: 'linear-gradient(to bottom, rgba(15, 23, 42, 0.95), transparent)',
          padding: '20px 30px',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
        }}
      >
        <div>
          <h1 style={{ color: 'white', margin: 0, fontSize: '28px', fontWeight: 'bold' }}>
            PayPerPlay Dashboard
          </h1>
          <p style={{ color: '#94a3b8', margin: '4px 0 0 0', fontSize: '14px' }}>
            Live Fleet Monitoring & Auto-Scaling Visualization
          </p>
        </div>

        {/* Connection Status */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '20px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <motion.div
              animate={{
                scale: [1, 1.2, 1],
                opacity: [0.5, 1, 0.5],
              }}
              transition={{
                duration: 2,
                repeat: Infinity,
                ease: 'easeInOut',
              }}
              style={{
                width: '10px',
                height: '10px',
                borderRadius: '50%',
                background: connected ? '#10b981' : '#ef4444',
              }}
            />
            <span style={{ color: 'white', fontSize: '14px', fontWeight: '500' }}>
              {connected ? 'Connected' : 'Disconnected'}
            </span>
          </div>
        </div>
      </motion.div>

      {/* Fleet Stats Panel */}
      {fleetStats && (
        <motion.div
          initial={{ x: -300, opacity: 0 }}
          animate={{ x: 0, opacity: 1 }}
          style={{
            position: 'absolute',
            top: '100px',
            left: '20px',
            zIndex: 10,
            background: 'rgba(15, 23, 42, 0.9)',
            border: '2px solid #334155',
            borderRadius: '12px',
            padding: '16px',
            minWidth: '250px',
            color: 'white',
            backdropFilter: 'blur(10px)',
          }}
        >
          <h3 style={{ margin: '0 0 12px 0', fontSize: '16px', fontWeight: 'bold' }}>
            Fleet Statistics
          </h3>

          <div style={{ fontSize: '13px', lineHeight: '1.8' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '4px' }}>
              <span style={{ opacity: 0.8 }}>Total Nodes:</span>
              <span style={{ fontWeight: 'bold' }}>{fleetStats.total_nodes}</span>
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '4px' }}>
              <span style={{ opacity: 0.8 }}>Cloud Nodes:</span>
              <span style={{ fontWeight: 'bold', color: '#667eea' }}>{fleetStats.cloud_nodes}</span>
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '4px' }}>
              <span style={{ opacity: 0.8 }}>Dedicated:</span>
              <span style={{ fontWeight: 'bold', color: '#134e4a' }}>{fleetStats.dedicated_nodes}</span>
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '4px' }}>
              <span style={{ opacity: 0.8 }}>Total Servers:</span>
              <span style={{ fontWeight: 'bold' }}>{fleetStats.total_servers}</span>
            </div>

            <div style={{ margin: '12px 0 8px 0', borderTop: '1px solid #334155', paddingTop: '12px' }}>
              <div style={{ fontSize: '12px', marginBottom: '6px', display: 'flex', justifyContent: 'space-between' }}>
                <span>Fleet Capacity</span>
                <span style={{ fontWeight: 'bold' }}>{fleetStats.capacity_percent.toFixed(1)}%</span>
              </div>
              <div style={{ height: '8px', background: '#1e293b', borderRadius: '4px', overflow: 'hidden' }}>
                <motion.div
                  initial={{ width: 0 }}
                  animate={{ width: `${fleetStats.capacity_percent}%` }}
                  style={{
                    height: '100%',
                    background:
                      fleetStats.capacity_percent < 50
                        ? '#10b981'
                        : fleetStats.capacity_percent < 70
                        ? '#f59e0b'
                        : fleetStats.capacity_percent < 85
                        ? '#f97316'
                        : '#ef4444',
                    transition: 'width 0.5s ease',
                  }}
                />
              </div>
            </div>

            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '4px' }}>
              <span style={{ opacity: 0.8 }}>Total RAM:</span>
              <span style={{ fontWeight: 'bold' }}>{(fleetStats.total_ram_mb / 1024).toFixed(1)} GB</span>
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '4px' }}>
              <span style={{ opacity: 0.8 }}>Allocated:</span>
              <span style={{ fontWeight: 'bold' }}>{(fleetStats.allocated_ram_mb / 1024).toFixed(1)} GB</span>
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between' }}>
              <span style={{ opacity: 0.8 }}>Free:</span>
              <span style={{ fontWeight: 'bold', color: '#10b981' }}>{(fleetStats.free_ram_mb / 1024).toFixed(1)} GB</span>
            </div>
          </div>
        </motion.div>
      )}

      {/* React Flow Canvas */}
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        fitView
        attributionPosition="bottom-left"
      >
        <Background variant={BackgroundVariant.Dots} gap={20} size={1} color="#1e293b" />
        <Controls />
        <MiniMap nodeColor={nodeColor} pannable zoomable />
      </ReactFlow>

      {/* Legend */}
      <motion.div
        initial={{ x: 300, opacity: 0 }}
        animate={{ x: 0, opacity: 1 }}
        style={{
          position: 'absolute',
          bottom: '20px',
          right: '20px',
          zIndex: 10,
          background: 'rgba(15, 23, 42, 0.9)',
          border: '2px solid #334155',
          borderRadius: '12px',
          padding: '16px',
          color: 'white',
          backdropFilter: 'blur(10px)',
        }}
      >
        <h3 style={{ margin: '0 0 12px 0', fontSize: '14px', fontWeight: 'bold' }}>
          Legend
        </h3>
        <div style={{ fontSize: '12px', display: 'flex', flexDirection: 'column', gap: '8px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <div style={{ width: '20px', height: '20px', borderRadius: '4px', background: '#667eea' }} />
            <span>Cloud Node</span>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <div style={{ width: '20px', height: '20px', borderRadius: '4px', background: '#134e4a' }} />
            <span>Dedicated Node</span>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <div style={{ width: '20px', height: '20px', borderRadius: '4px', background: '#0ea5e9' }} />
            <span>Velocity Proxy</span>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginTop: '4px', paddingTop: '8px', borderTop: '1px solid #334155' }}>
            <div style={{ width: '20px', height: '2px', background: '#3b82f6' }} />
            <span>Active Migration</span>
          </div>
        </div>
      </motion.div>
    </div>
  );
};
