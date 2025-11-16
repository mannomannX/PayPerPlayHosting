import { motion, AnimatePresence } from 'framer-motion';

interface ContainerDetailsModalProps {
  isOpen: boolean;
  onClose: () => void;
  container: {
    server_id: string;
    server_name: string;
    ram_mb: number;
    status: string;
    port: number;
    join_address: string;
    minecraft_version?: string;
    server_type?: string;
  } | null;
  nodeId: string;
  nodeIp: string;
}

const getStatusColor = (status: string): string => {
  switch (status.toLowerCase()) {
    case 'running':
      return '#10b981'; // Green
    case 'starting':
    case 'provisioning':
      return '#3b82f6'; // Blue
    case 'stopping':
      return '#f59e0b'; // Yellow
    case 'stopped':
    case 'sleeping':
      return '#6b7280'; // Gray
    case 'crashed':
    case 'failed':
      return '#ef4444'; // Red
    default:
      return '#8b5cf6'; // Purple
  }
};

const getStatusEmoji = (status: string): string => {
  switch (status.toLowerCase()) {
    case 'running':
      return '✅';
    case 'starting':
    case 'provisioning':
      return '🔵';
    case 'stopping':
      return '🟡';
    case 'stopped':
    case 'sleeping':
      return '⚫';
    case 'crashed':
    case 'failed':
      return '🔴';
    default:
      return '🟣';
  }
};

export const ContainerDetailsModal = ({
  isOpen,
  onClose,
  container,
  nodeId,
  nodeIp,
}: ContainerDetailsModalProps) => {
  if (!container) return null;

  const statusColor = getStatusColor(container.status);
  const statusEmoji = getStatusEmoji(container.status);

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
  };

  return (
    <AnimatePresence>
      {isOpen && (
        <>
          {/* Backdrop */}
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={onClose}
            style={{
              position: 'fixed',
              top: 0,
              left: 0,
              right: 0,
              bottom: 0,
              backgroundColor: 'rgba(0, 0, 0, 0.5)',
              zIndex: 1000,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            {/* Modal */}
            <motion.div
              initial={{ scale: 0.9, opacity: 0 }}
              animate={{ scale: 1, opacity: 1 }}
              exit={{ scale: 0.9, opacity: 0 }}
              onClick={(e) => e.stopPropagation()}
              style={{
                background: 'linear-gradient(135deg, #1e293b 0%, #0f172a 100%)',
                borderRadius: '16px',
                padding: '24px',
                minWidth: '500px',
                maxWidth: '600px',
                boxShadow: '0 20px 25px -5px rgba(0, 0, 0, 0.3)',
                border: '1px solid rgba(255, 255, 255, 0.1)',
                color: 'white',
              }}
            >
              {/* Header */}
              <div style={{ marginBottom: '24px', borderBottom: '1px solid rgba(255,255,255,0.1)', paddingBottom: '16px' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                  <div>
                    <h2 style={{ margin: 0, fontSize: '24px', fontWeight: 'bold', marginBottom: '8px' }}>
                      {container.server_name}
                    </h2>
                    <div style={{ fontSize: '12px', opacity: 0.6, fontFamily: 'monospace' }}>
                      {container.server_id}
                    </div>
                  </div>
                  <button
                    onClick={onClose}
                    style={{
                      background: 'rgba(255,255,255,0.1)',
                      border: 'none',
                      borderRadius: '8px',
                      padding: '8px 12px',
                      color: 'white',
                      cursor: 'pointer',
                      fontSize: '16px',
                      fontWeight: 'bold',
                    }}
                  >
                    ✕
                  </button>
                </div>
              </div>

              {/* Status Badge */}
              <div style={{ marginBottom: '20px' }}>
                <div
                  style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: '8px',
                    padding: '8px 16px',
                    borderRadius: '8px',
                    background: `${statusColor}20`,
                    border: `2px solid ${statusColor}`,
                  }}
                >
                  <span style={{ fontSize: '20px' }}>{statusEmoji}</span>
                  <span style={{ fontSize: '14px', fontWeight: 'bold', color: statusColor }}>
                    {container.status.toUpperCase()}
                  </span>
                </div>
              </div>

              {/* Details Grid */}
              <div style={{ display: 'grid', gap: '16px' }}>
                {/* Join Address */}
                <div
                  style={{
                    background: 'rgba(255,255,255,0.05)',
                    borderRadius: '8px',
                    padding: '16px',
                    border: '1px solid rgba(255,255,255,0.1)',
                  }}
                >
                  <div style={{ fontSize: '12px', opacity: 0.6, marginBottom: '4px' }}>Join Address</div>
                  <div
                    style={{
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'center',
                    }}
                  >
                    <div style={{ fontSize: '18px', fontWeight: 'bold', fontFamily: 'monospace' }}>
                      {container.join_address}
                    </div>
                    <button
                      onClick={() => copyToClipboard(container.join_address)}
                      style={{
                        background: 'rgba(59, 130, 246, 0.2)',
                        border: '1px solid #3b82f6',
                        borderRadius: '6px',
                        padding: '6px 12px',
                        color: '#3b82f6',
                        cursor: 'pointer',
                        fontSize: '12px',
                        fontWeight: 'bold',
                      }}
                    >
                      📋 Copy
                    </button>
                  </div>
                </div>

                {/* Minecraft Version */}
                {container.minecraft_version && (
                  <div
                    style={{
                      background: 'rgba(255,255,255,0.05)',
                      borderRadius: '8px',
                      padding: '16px',
                      border: '1px solid rgba(255,255,255,0.1)',
                    }}
                  >
                    <div style={{ fontSize: '12px', opacity: 0.6, marginBottom: '4px' }}>Minecraft Version</div>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <div style={{ fontSize: '18px', fontWeight: 'bold' }}>
                        {container.minecraft_version}
                      </div>
                      {container.server_type && (
                        <div
                          style={{
                            fontSize: '11px',
                            padding: '4px 8px',
                            borderRadius: '4px',
                            background: 'rgba(16, 185, 129, 0.2)',
                            border: '1px solid #10b981',
                            color: '#10b981',
                            fontWeight: 'bold',
                            textTransform: 'uppercase',
                          }}
                        >
                          {container.server_type}
                        </div>
                      )}
                    </div>
                  </div>
                )}

                {/* Port */}
                <div
                  style={{
                    background: 'rgba(255,255,255,0.05)',
                    borderRadius: '8px',
                    padding: '16px',
                    border: '1px solid rgba(255,255,255,0.1)',
                  }}
                >
                  <div style={{ fontSize: '12px', opacity: 0.6, marginBottom: '4px' }}>Minecraft Port</div>
                  <div style={{ fontSize: '18px', fontWeight: 'bold', fontFamily: 'monospace' }}>
                    {container.port}
                  </div>
                </div>

                {/* RAM Allocation */}
                <div
                  style={{
                    background: 'rgba(255,255,255,0.05)',
                    borderRadius: '8px',
                    padding: '16px',
                    border: '1px solid rgba(255,255,255,0.1)',
                  }}
                >
                  <div style={{ fontSize: '12px', opacity: 0.6, marginBottom: '4px' }}>RAM Allocation</div>
                  <div style={{ fontSize: '18px', fontWeight: 'bold' }}>
                    {container.ram_mb} MB
                  </div>
                </div>

                {/* Node Assignment */}
                <div
                  style={{
                    background: 'rgba(255,255,255,0.05)',
                    borderRadius: '8px',
                    padding: '16px',
                    border: '1px solid rgba(255,255,255,0.1)',
                  }}
                >
                  <div style={{ fontSize: '12px', opacity: 0.6, marginBottom: '8px' }}>Node Assignment</div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                    <div style={{ fontSize: '14px', fontWeight: 'bold' }}>
                      {nodeId}
                    </div>
                    <div style={{ fontSize: '13px', opacity: 0.8, fontFamily: 'monospace' }}>
                      {nodeIp}
                    </div>
                  </div>
                </div>
              </div>

              {/* Footer */}
              <div style={{ marginTop: '20px', paddingTop: '16px', borderTop: '1px solid rgba(255,255,255,0.1)' }}>
                <div style={{ fontSize: '11px', opacity: 0.5, textAlign: 'center' }}>
                  Use the join address to connect from your Minecraft client
                </div>
              </div>
            </motion.div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
};
