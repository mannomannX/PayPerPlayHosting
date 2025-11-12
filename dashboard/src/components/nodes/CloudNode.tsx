import { Handle, Position } from 'reactflow';
import { motion } from 'framer-motion';

interface CloudNodeProps {
  data: {
    label: string;
    provider?: string;
    location?: string;
    totalRAM: number;
    usableRAM: number;
    allocatedRAM: number;
    freeRAM: number;
    containerCount: number;
    capacityPercent: number;
    status: string;
    ipAddress: string;
    containers: Array<{
      server_id: string;
      server_name: string;
      ram_mb: number;
      status: string;
    }>;
  };
}

export const CloudNode = ({ data }: CloudNodeProps) => {
  const getCapacityColor = (percent: number) => {
    if (percent < 50) return '#10b981'; // green
    if (percent < 70) return '#f59e0b'; // yellow
    if (percent < 85) return '#f97316'; // orange
    return '#ef4444'; // red
  };

  const capacityColor = getCapacityColor(data.capacityPercent);

  return (
    <motion.div
      initial={{ scale: 0, opacity: 0 }}
      animate={{ scale: 1, opacity: 1 }}
      exit={{ scale: 0, opacity: 0 }}
      className="cloud-node"
      style={{
        background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
        border: `2px solid ${capacityColor}`,
        borderRadius: '12px',
        padding: '16px',
        minWidth: '280px',
        boxShadow: '0 4px 6px rgba(0, 0, 0, 0.1)',
        color: 'white',
      }}
    >
      <Handle type="target" position={Position.Top} />

      <div style={{ marginBottom: '8px' }}>
        <div style={{ fontSize: '16px', fontWeight: 'bold', marginBottom: '4px' }}>
          ☁️ {data.label}
        </div>
        <div style={{ fontSize: '11px', opacity: 0.8 }}>
          {data.provider} • {data.location}
        </div>
        <div style={{ fontSize: '11px', opacity: 0.7 }}>
          {data.ipAddress}
        </div>
      </div>

      {/* Capacity Bar */}
      <div style={{ marginBottom: '12px' }}>
        <div style={{ fontSize: '12px', marginBottom: '4px', display: 'flex', justifyContent: 'space-between' }}>
          <span>Capacity</span>
          <span style={{ fontWeight: 'bold' }}>{data.capacityPercent.toFixed(1)}%</span>
        </div>
        <div style={{ height: '8px', background: 'rgba(255,255,255,0.2)', borderRadius: '4px', overflow: 'hidden' }}>
          <motion.div
            initial={{ width: 0 }}
            animate={{ width: `${data.capacityPercent}%` }}
            style={{
              height: '100%',
              background: capacityColor,
              transition: 'width 0.5s ease',
            }}
          />
        </div>
      </div>

      {/* Stats */}
      <div style={{ fontSize: '12px', marginBottom: '8px' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '2px' }}>
          <span>RAM:</span>
          <span>{data.allocatedRAM} / {data.usableRAM} MB</span>
        </div>
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '2px' }}>
          <span>Free:</span>
          <span>{data.freeRAM} MB</span>
        </div>
        <div style={{ display: 'flex', justifyContent: 'space-between' }}>
          <span>Containers:</span>
          <span>{data.containerCount}</span>
        </div>
      </div>

      {/* Container List */}
      {data.containers.length > 0 && (
        <div style={{ marginTop: '8px', borderTop: '1px solid rgba(255,255,255,0.2)', paddingTop: '8px' }}>
          <div style={{ fontSize: '11px', fontWeight: 'bold', marginBottom: '4px' }}>Active Servers:</div>
          <div style={{ maxHeight: '100px', overflowY: 'auto' }}>
            {data.containers.map((container) => (
              <div
                key={container.server_id}
                style={{
                  fontSize: '10px',
                  padding: '4px 6px',
                  background: 'rgba(255,255,255,0.1)',
                  borderRadius: '4px',
                  marginBottom: '2px',
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                }}
              >
                <span>{container.server_name}</span>
                <span style={{ opacity: 0.7 }}>{container.ram_mb}MB</span>
              </div>
            ))}
          </div>
        </div>
      )}

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

      <Handle type="source" position={Position.Bottom} />
    </motion.div>
  );
};
