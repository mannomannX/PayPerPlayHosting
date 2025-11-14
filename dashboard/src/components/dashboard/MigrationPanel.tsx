import { motion } from 'framer-motion';
import { useState } from 'react';
import { useDashboardStore } from '../../stores/dashboardStore';

interface Server {
  id: string;
  name: string;
  currentNodeId: string;
  currentNodeName: string;
  ramMB: number;
  status: string;
}

interface MigrationPanelProps {
  inline?: boolean;
}

export const MigrationPanel = ({ inline = false }: MigrationPanelProps) => {
  const { nodes } = useDashboardStore();
  const [selectedServer, setSelectedServer] = useState<Server | null>(null);
  const [targetNodeId, setTargetNodeId] = useState<string>('');
  const [autoApprove, setAutoApprove] = useState(true);
  const [reason, setReason] = useState('');
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  // Extract all servers from nodes
  const allServers: Server[] = nodes.flatMap(node =>
    node.data.containers
      .filter(c => c.status === 'running') // Only allow migration of running servers
      .map(c => ({
        id: c.server_id,
        name: c.server_name,
        currentNodeId: node.id,
        currentNodeName: node.data.label,
        ramMB: c.ram_mb,
        status: c.status,
      }))
  );

  // Available target nodes (excluding current node and system nodes)
  const availableNodes = nodes.filter(n =>
    !n.data.isSystemNode &&
    n.id !== selectedServer?.currentNodeId &&
    n.data.status === 'healthy'
  );

  const handleTriggerMigration = async () => {
    if (!selectedServer || !targetNodeId) {
      setMessage({ type: 'error', text: 'Please select a server and target node' });
      return;
    }

    setLoading(true);
    setMessage(null);

    try {
      const response = await fetch('/admin/migrations', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          server_id: selectedServer.id,
          to_node_id: targetNodeId,
          reason: reason || 'Manual migration from dashboard',
          auto_approve: autoApprove,
        }),
      });

      const data = await response.json();

      if (response.ok) {
        setMessage({
          type: 'success',
          text: `Migration ${autoApprove ? 'started' : 'created'} successfully!`
        });
        // Reset form
        setSelectedServer(null);
        setTargetNodeId('');
        setReason('');
      } else {
        setMessage({
          type: 'error',
          text: data.error || 'Failed to create migration'
        });
      }
    } catch (error) {
      setMessage({
        type: 'error',
        text: `Network error: ${error}`
      });
    } finally {
      setLoading(false);
    }
  };

  const containerStyle = inline ? {
    background: 'rgba(59, 130, 246, 0.1)',
    border: '2px solid #3b82f6',
    borderRadius: '8px',
    padding: '16px',
    color: 'white',
  } : {
    position: 'fixed' as const,
    top: '100px',
    left: '20px',
    zIndex: 15,
    background: 'rgba(15, 23, 42, 0.95)',
    border: '2px solid #667eea',
    borderRadius: '12px',
    padding: '20px',
    width: '320px',
    maxHeight: 'calc(100vh - 120px)',
    overflowY: 'auto' as const,
    color: 'white',
    backdropFilter: 'blur(10px)',
  };

  return (
    <motion.div
      initial={inline ? {} : { x: -300, opacity: 0 }}
      animate={inline ? {} : { x: 0, opacity: 1 }}
      style={containerStyle}
    >
      <h3 style={{
        margin: '0 0 16px 0',
        fontSize: inline ? '14px' : '18px',
        fontWeight: 'bold',
        color: inline ? '#3b82f6' : '#667eea',
      }}>
        Create Migration
      </h3>

      {allServers.length === 0 ? (
        <div style={{
          padding: '20px',
          textAlign: 'center',
          opacity: 0.6,
          fontSize: '14px',
        }}>
          No running servers available for migration
        </div>
      ) : (
        <>
          {/* Server Selection */}
          <div style={{ marginBottom: '16px' }}>
            <label style={{
              display: 'block',
              marginBottom: '8px',
              fontSize: '13px',
              fontWeight: '500',
            }}>
              Select Server
            </label>
            <select
              value={selectedServer?.id || ''}
              onChange={(e) => {
                const server = allServers.find(s => s.id === e.target.value);
                setSelectedServer(server || null);
                setTargetNodeId(''); // Reset target node
              }}
              style={{
                width: '100%',
                padding: '10px',
                background: '#1e293b',
                border: '1px solid #334155',
                borderRadius: '6px',
                color: 'white',
                fontSize: '13px',
              }}
            >
              <option value="">-- Select a server --</option>
              {allServers.map(server => (
                <option key={server.id} value={server.id}>
                  {server.name} ({server.ramMB} MB @ {server.currentNodeName})
                </option>
              ))}
            </select>
          </div>

          {/* Target Node Selection */}
          {selectedServer && (
            <div style={{ marginBottom: '16px' }}>
              <label style={{
                display: 'block',
                marginBottom: '8px',
                fontSize: '13px',
                fontWeight: '500',
              }}>
                Target Node
              </label>
              <select
                value={targetNodeId}
                onChange={(e) => setTargetNodeId(e.target.value)}
                style={{
                  width: '100%',
                  padding: '10px',
                  background: '#1e293b',
                  border: '1px solid #334155',
                  borderRadius: '6px',
                  color: 'white',
                  fontSize: '13px',
                }}
              >
                <option value="">-- Select target node --</option>
                {availableNodes.map(node => (
                  <option key={node.id} value={node.id}>
                    {node.data.label} ({node.data.freeRAM} MB free)
                  </option>
                ))}
              </select>
              {availableNodes.length === 0 && (
                <div style={{
                  marginTop: '8px',
                  fontSize: '12px',
                  color: '#f59e0b',
                }}>
                  No available target nodes
                </div>
              )}
            </div>
          )}

          {/* Reason */}
          {selectedServer && targetNodeId && (
            <div style={{ marginBottom: '16px' }}>
              <label style={{
                display: 'block',
                marginBottom: '8px',
                fontSize: '13px',
                fontWeight: '500',
              }}>
                Reason (optional)
              </label>
              <input
                type="text"
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder="e.g., Testing migration system"
                style={{
                  width: '100%',
                  padding: '10px',
                  background: '#1e293b',
                  border: '1px solid #334155',
                  borderRadius: '6px',
                  color: 'white',
                  fontSize: '13px',
                }}
              />
            </div>
          )}

          {/* Auto-Approve Checkbox */}
          {selectedServer && targetNodeId && (
            <div style={{ marginBottom: '16px' }}>
              <label style={{
                display: 'flex',
                alignItems: 'center',
                gap: '8px',
                fontSize: '13px',
                cursor: 'pointer',
              }}>
                <input
                  type="checkbox"
                  checked={autoApprove}
                  onChange={(e) => setAutoApprove(e.target.checked)}
                  style={{ cursor: 'pointer' }}
                />
                <span>Auto-approve and execute immediately</span>
              </label>
              <div style={{
                marginTop: '4px',
                fontSize: '11px',
                opacity: 0.6,
                marginLeft: '24px',
              }}>
                {autoApprove
                  ? 'Migration will start immediately'
                  : 'Migration will be created as suggestion'}
              </div>
            </div>
          )}

          {/* Trigger Button */}
          {selectedServer && targetNodeId && (
            <button
              onClick={handleTriggerMigration}
              disabled={loading}
              style={{
                width: '100%',
                padding: '12px',
                background: loading ? '#334155' : '#667eea',
                border: 'none',
                borderRadius: '8px',
                color: 'white',
                fontSize: '14px',
                fontWeight: 'bold',
                cursor: loading ? 'not-allowed' : 'pointer',
                transition: 'all 0.2s',
              }}
            >
              {loading ? 'Creating...' : (autoApprove ? 'Start Migration' : 'Create Suggestion')}
            </button>
          )}

          {/* Message */}
          {message && (
            <motion.div
              initial={{ opacity: 0, y: -10 }}
              animate={{ opacity: 1, y: 0 }}
              style={{
                marginTop: '16px',
                padding: '12px',
                background: message.type === 'success' ? 'rgba(16, 185, 129, 0.1)' : 'rgba(239, 68, 68, 0.1)',
                border: `1px solid ${message.type === 'success' ? '#10b981' : '#ef4444'}`,
                borderRadius: '8px',
                fontSize: '13px',
                color: message.type === 'success' ? '#10b981' : '#ef4444',
              }}
            >
              {message.text}
            </motion.div>
          )}

          {/* Migration Info */}
          {selectedServer && targetNodeId && (
            <div style={{
              marginTop: '16px',
              padding: '12px',
              background: 'rgba(103, 126, 234, 0.1)',
              border: '1px solid #667eea',
              borderRadius: '8px',
              fontSize: '12px',
            }}>
              <div style={{ fontWeight: 'bold', marginBottom: '8px', color: '#667eea' }}>
                Migration Summary:
              </div>
              <div style={{ opacity: 0.9, lineHeight: '1.6' }}>
                <div>Server: <strong>{selectedServer.name}</strong></div>
                <div>RAM: <strong>{selectedServer.ramMB} MB</strong></div>
                <div>From: <strong>{selectedServer.currentNodeName}</strong></div>
                <div>To: <strong>{availableNodes.find(n => n.id === targetNodeId)?.data.label}</strong></div>
              </div>
            </div>
          )}
        </>
      )}
    </motion.div>
  );
};
