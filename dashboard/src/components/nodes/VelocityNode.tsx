import { Handle, Position } from 'reactflow';
import { motion } from 'framer-motion';

interface VelocityNodeProps {
  data: {
    label: string;
    totalPlayers?: number;
    totalServers?: number;
    status: string;
    ipAddress: string;
  };
}

export const VelocityNode = ({ data }: VelocityNodeProps) => {
  return (
    <motion.div
      initial={{ scale: 0, opacity: 0 }}
      animate={{ scale: 1, opacity: 1 }}
      exit={{ scale: 0, opacity: 0 }}
      className="velocity-node"
      style={{
        background: 'linear-gradient(135deg, #0ea5e9 0%, #06b6d4 100%)',
        border: '3px solid #22d3ee',
        borderRadius: '12px',
        padding: '16px',
        minWidth: '240px',
        boxShadow: '0 8px 16px rgba(0, 0, 0, 0.2)',
        color: 'white',
      }}
    >
      <Handle type="target" position={Position.Top} />

      <div style={{ marginBottom: '12px', textAlign: 'center' }}>
        <div style={{ fontSize: '20px', fontWeight: 'bold', marginBottom: '4px' }}>
          ⚡ VELOCITY PROXY
        </div>
        <div style={{ fontSize: '11px', opacity: 0.8 }}>
          {data.ipAddress}
        </div>
      </div>

      {/* Stats */}
      <div style={{ fontSize: '14px', marginBottom: '8px', textAlign: 'center' }}>
        <div style={{ marginBottom: '6px', padding: '8px', background: 'rgba(255,255,255,0.1)', borderRadius: '6px' }}>
          <div style={{ fontSize: '24px', fontWeight: 'bold' }}>
            {data.totalPlayers ?? 0}
          </div>
          <div style={{ fontSize: '11px', opacity: 0.8 }}>
            Online Players
          </div>
        </div>

        <div style={{ padding: '8px', background: 'rgba(255,255,255,0.1)', borderRadius: '6px' }}>
          <div style={{ fontSize: '20px', fontWeight: 'bold' }}>
            {data.totalServers ?? 0}
          </div>
          <div style={{ fontSize: '11px', opacity: 0.8 }}>
            Registered Servers
          </div>
        </div>
      </div>

      {/* Status Badge */}
      <div
        style={{
          position: 'absolute',
          top: '8px',
          right: '8px',
          fontSize: '10px',
          padding: '2px 6px',
          borderRadius: '4px',
          background: data.status === 'healthy' ? '#10b981' : '#ef4444',
          fontWeight: 'bold',
        }}
      >
        {data.status.toUpperCase()}
      </div>

      {/* Pulse Animation */}
      <motion.div
        animate={{
          scale: [1, 1.2, 1],
          opacity: [0.5, 0.8, 0.5],
        }}
        transition={{
          duration: 2,
          repeat: Infinity,
          ease: "easeInOut",
        }}
        style={{
          position: 'absolute',
          top: '-4px',
          right: '-4px',
          width: '12px',
          height: '12px',
          borderRadius: '50%',
          background: '#22d3ee',
        }}
      />

      <Handle type="source" position={Position.Bottom} />
    </motion.div>
  );
};
