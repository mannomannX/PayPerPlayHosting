import { motion } from 'framer-motion';
import { useState, useRef, useEffect } from 'react';

import { useDashboardStore } from '../../stores/dashboardStore';
import { useWebSocket } from '../../hooks/useWebSocket';
import { GridNode } from '../nodes/GridNode';
import { ManhattanArrow } from '../arrows/ManhattanArrow';
import { DebugConsole } from './DebugConsole';
import { MigrationDropdown } from './MigrationDropdown';
import { PageNavigation } from '../navigation/PageNavigation';

// WebSocket URL - uses nginx proxy (no port needed, nginx forwards /api/ to backend)
const WS_URL = `ws://${window.location.hostname}/api/admin/dashboard/stream`;

export const Dashboard = () => {
  const {
    nodes,
    migrations,
    fleetStats,
    queueInfo,
    connected,
    setConnected,
    handleEvent,
  } = useDashboardStore();

  const [selectedMigration, setSelectedMigration] = useState<string | null>(null);
  const [nodePositions, setNodePositions] = useState<Map<string, { x: number; y: number; width: number; height: number }>>(new Map());
  const containerRef = useRef<HTMLDivElement>(null);

  // WebSocket connection
  useWebSocket({
    url: WS_URL,
    onMessage: handleEvent,
    onConnect: () => setConnected(true),
    onDisconnect: () => setConnected(false),
    onError: (error) => console.error('[Dashboard] WebSocket error:', error),
  });

  // Calculate node positions for arrow routing
  useEffect(() => {
    if (!containerRef.current) return;

    const positions = new Map<string, { x: number; y: number; width: number; height: number }>();
    const nodeElements = containerRef.current.querySelectorAll('[data-node-id]');

    nodeElements.forEach((element) => {
      const nodeId = element.getAttribute('data-node-id');
      if (!nodeId) return;

      const rect = element.getBoundingClientRect();
      const containerRect = containerRef.current!.getBoundingClientRect();

      positions.set(nodeId, {
        x: rect.left - containerRect.left,
        y: rect.top - containerRect.top,
        width: rect.width,
        height: rect.height,
      });
    });

    setNodePositions(positions);
  }, [nodes, migrations]);

  // Separate nodes by tier for grid layout
  const controlNodes = nodes.filter(n => n.id.includes('local') || n.id.includes('control'));
  const proxyNodes = nodes.filter(n => n.id.includes('proxy'));
  const workloadNodes = nodes.filter(n => !n.id.includes('local') && !n.id.includes('control') && !n.id.includes('proxy'));

  // Get container status color
  const getStatusColor = (status: string) => {
    switch (status.toLowerCase()) {
      case 'running': return '#10b981'; // green
      case 'starting': return '#3b82f6'; // blue
      case 'stopping': return '#f59e0b'; // yellow
      case 'stopped': return '#6b7280'; // gray
      case 'error':
      case 'crashed': return '#ef4444'; // red
      default: return '#6b7280'; // gray
    }
  };

  return (
    <div style={{ width: '100vw', height: '100vh', background: '#0f172a', overflow: 'hidden' }}>
      {/* Header */}
      <motion.div
        initial={{ y: -100, opacity: 0 }}
        animate={{ y: 0, opacity: 1 }}
        style={{
          position: 'fixed',
          top: 0,
          left: 0,
          right: 0,
          zIndex: 20,
          background: 'rgba(15, 23, 42, 0.95)',
          backdropFilter: 'blur(10px)',
          borderBottom: '1px solid #334155',
          padding: '16px 30px',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '24px' }}>
          <div>
            <h1 style={{ color: 'white', margin: 0, fontSize: '24px', fontWeight: 'bold' }}>
              PayPerPlay Dashboard
            </h1>
            <p style={{ color: '#94a3b8', margin: '4px 0 0 0', fontSize: '12px' }}>
              3-Tier Architecture Fleet Monitoring
            </p>
          </div>

          {/* Page Navigation */}
          <PageNavigation />
        </div>

        {/* Connection Status & Migration Dropdown */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '20px' }}>
          {/* Migration Dropdown */}
          <MigrationDropdown />

          {/* Connection Status */}
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
              <span style={{ opacity: 0.8 }} title="Total physical RAM across all nodes">Total RAM:</span>
              <span style={{ fontWeight: 'bold' }}>{(fleetStats.total_ram_mb / 1024).toFixed(1)} GB</span>
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '4px' }}>
              <span style={{ opacity: 0.8 }} title="RAM available for containers (after system reserve)">Usable RAM:</span>
              <span style={{ fontWeight: 'bold', color: '#3b82f6' }}>{(fleetStats.usable_ram_mb / 1024).toFixed(1)} GB</span>
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '4px' }}>
              <span style={{ opacity: 0.8 }} title="RAM currently allocated to running containers">Allocated:</span>
              <span style={{ fontWeight: 'bold', color: '#f59e0b' }}>{(fleetStats.allocated_ram_mb / 1024).toFixed(1)} GB</span>
            </div>
            <div style={{ display: 'flex', justifyContent: 'space-between' }}>
              <span style={{ opacity: 0.8 }} title="RAM available for new containers">Available:</span>
              <span style={{ fontWeight: 'bold', color: '#10b981' }}>{(fleetStats.free_ram_mb / 1024).toFixed(1)} GB</span>
            </div>
          </div>
        </motion.div>
      )}

      {/* Deployment Queue Panel */}
      {queueInfo && queueInfo.queue_size > 0 && (
        <motion.div
          initial={{ x: 300, opacity: 0 }}
          animate={{ x: 0, opacity: 1 }}
          style={{
            position: 'fixed',
            top: '100px',
            right: '20px',
            zIndex: 10,
            background: 'rgba(15, 23, 42, 0.9)',
            border: '2px solid #f59e0b',
            borderRadius: '12px',
            padding: '16px',
            minWidth: '280px',
            maxWidth: '320px',
            color: 'white',
            backdropFilter: 'blur(10px)',
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
            <h3 style={{ margin: 0, fontSize: '16px', fontWeight: 'bold' }}>
              Deployment Queue
            </h3>
            <div style={{
              background: '#f59e0b',
              color: '#000',
              borderRadius: '12px',
              padding: '2px 8px',
              fontSize: '12px',
              fontWeight: 'bold',
            }}>
              {queueInfo.queue_size}
            </div>
          </div>

          <div style={{ fontSize: '12px', maxHeight: '400px', overflowY: 'auto' }}>
            {queueInfo.servers.map((server, idx) => (
              <motion.div
                key={server.ServerID}
                initial={{ x: 20, opacity: 0 }}
                animate={{ x: 0, opacity: 1 }}
                transition={{ delay: idx * 0.1 }}
                style={{
                  background: 'rgba(245, 158, 11, 0.1)',
                  border: '1px solid #f59e0b',
                  borderRadius: '8px',
                  padding: '10px',
                  marginBottom: '8px',
                }}
              >
                <div style={{ fontWeight: 'bold', marginBottom: '4px' }}>
                  #{idx + 1} {server.ServerName || server.ServerID.substring(0, 8)}
                </div>
                <div style={{ opacity: 0.8, fontSize: '11px' }}>
                  RAM: {server.RequiredRAMMB} MB
                </div>
                <div style={{ opacity: 0.7, fontSize: '10px', marginTop: '4px' }}>
                  Waiting for capacity...
                </div>
              </motion.div>
            ))}
          </div>
        </motion.div>
      )}

      {/* Grid Layout for Nodes */}
      <div
        ref={containerRef}
        style={{
          position: 'absolute',
          top: '80px',
          left: '300px',
          right: queueInfo && queueInfo.queue_size > 0 ? '360px' : '20px',
          bottom: '20px',
          padding: '20px',
          overflowY: 'auto',
        }}>
        {/* Tier 1: Control Plane & Proxy */}
        <div style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, 320px)',
          gap: '60px',
          marginBottom: '80px',
          justifyContent: 'start',
        }}>
          {controlNodes.map(node => (
            <div key={node.id} data-node-id={node.id}>
              <GridNode node={node} getStatusColor={getStatusColor} />
            </div>
          ))}
          {proxyNodes.map(node => (
            <div key={node.id} data-node-id={node.id}>
              <GridNode node={node} getStatusColor={getStatusColor} />
            </div>
          ))}
        </div>

        {/* Separator between System Nodes and Worker Nodes */}
        {workloadNodes.length > 0 && (
          <div style={{
            height: '2px',
            background: 'linear-gradient(to right, transparent, #6366f1, transparent)',
            marginBottom: '60px',
            position: 'relative',
          }}>
            <div style={{
              position: 'absolute',
              top: '-10px',
              left: '50%',
              transform: 'translateX(-50%)',
              background: '#0f172a',
              padding: '0 16px',
              color: '#a5b4fc',
              fontSize: '12px',
              fontWeight: 'bold',
            }}>
              WORKER NODES (MC Containers Only)
            </div>
          </div>
        )}

        {/* Tier 3: Workload Nodes */}
        <div style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, 320px)',
          gap: '60px',
          justifyContent: 'start',
        }}>
          {workloadNodes.map(node => (
            <div key={node.id} data-node-id={node.id}>
              <GridNode node={node} getStatusColor={getStatusColor} />
            </div>
          ))}
        </div>

        {/* Empty State */}
        {nodes.length === 0 && (
          <div style={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            height: '100%',
            color: '#64748b',
          }}>
            <div style={{ fontSize: '48px', marginBottom: '16px' }}>🏗️</div>
            <div style={{ fontSize: '20px', fontWeight: 'bold', marginBottom: '8px' }}>
              No Nodes Detected
            </div>
            <div style={{ fontSize: '14px', opacity: 0.7 }}>
              Waiting for nodes to register...
            </div>
          </div>
        )}

        {/* SVG Overlay for Migration Arrows */}
        {migrations.size > 0 && containerRef.current && (
          <svg
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              width: '100%',
              height: '100%',
              pointerEvents: 'none',
              zIndex: 5,
            }}
          >
            {Array.from(migrations.values()).map((migration) => {
              const fromNode = nodePositions.get(migration.from_node);
              const toNode = nodePositions.get(migration.to_node);

              if (!fromNode || !toNode) return null;

              return (
                <g key={migration.operation_id} style={{ pointerEvents: 'all' }}>
                  <ManhattanArrow
                    fromNode={{ id: migration.from_node, ...fromNode }}
                    toNode={{ id: migration.to_node, ...toNode }}
                    migration={migration}
                    isSelected={selectedMigration === migration.operation_id}
                    onClick={() => {
                      if (selectedMigration === migration.operation_id) {
                        setSelectedMigration(null);
                      } else {
                        setSelectedMigration(migration.operation_id);
                      }
                    }}
                  />
                </g>
              );
            })}
          </svg>
        )}
      </div>

      {/* Status Legend */}
      <motion.div
        initial={{ y: 50, opacity: 0 }}
        animate={{ y: 0, opacity: 1 }}
        style={{
          position: 'fixed',
          bottom: '20px',
          left: '300px',
          zIndex: 10,
          background: 'rgba(15, 23, 42, 0.9)',
          border: '2px solid #334155',
          borderRadius: '12px',
          padding: '12px 16px',
          color: 'white',
          backdropFilter: 'blur(10px)',
        }}
      >
        <div style={{ fontSize: '11px', fontWeight: 'bold', marginBottom: '8px' }}>
          Container Status:
        </div>
        <div style={{ display: 'flex', gap: '16px', fontSize: '10px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
            <div style={{ width: '12px', height: '12px', borderRadius: '3px', background: '#10b981' }} />
            <span>Running</span>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
            <div style={{ width: '12px', height: '12px', borderRadius: '3px', background: '#3b82f6' }} />
            <span>Starting</span>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
            <div style={{ width: '12px', height: '12px', borderRadius: '3px', background: '#f59e0b' }} />
            <span>Stopping</span>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
            <div style={{ width: '12px', height: '12px', borderRadius: '3px', background: '#6b7280' }} />
            <span>Stopped</span>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
            <div style={{ width: '12px', height: '12px', borderRadius: '3px', background: '#ef4444' }} />
            <span>Error</span>
          </div>
        </div>
      </motion.div>

      {/* Debug Console */}
      <DebugConsole />
    </div>
  );
};
