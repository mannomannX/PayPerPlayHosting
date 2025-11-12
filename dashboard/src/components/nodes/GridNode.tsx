import { motion } from 'framer-motion';

interface Container {
  server_id: string;
  server_name: string;
  ram_mb: number;
  status: string;
}

interface GridNodeProps {
  node: {
    id: string;
    data: {
      label: string;
      type: 'dedicated' | 'cloud' | 'velocity';
      totalRAM: number;
      usableRAM: number;
      allocatedRAM: number;
      freeRAM: number;
      containerCount: number;
      capacityPercent: number;
      cpuUsagePercent?: number;
      status: string;
      ipAddress: string;
      containers: Container[];
    };
  };
  getStatusColor: (status: string) => string;
}

export const GridNode = ({ node, getStatusColor }: GridNodeProps) => {
  const { data } = node;

  const getCapacityColor = (percent: number) => {
    if (percent < 50) return '#10b981'; // green
    if (percent < 70) return '#f59e0b'; // yellow
    if (percent < 85) return '#f97316'; // orange
    return '#ef4444'; // red
  };

  const capacityColor = getCapacityColor(data.capacityPercent);

  // Calculate max container slots (usableRAM / 1GB minimum)
  const maxSlots = Math.floor(data.usableRAM / 1024);
  const containerSlots = Array(maxSlots).fill(null).map((_, i) => data.containers[i] || null);

  return (
    <motion.div
      initial={{ scale: 0, opacity: 0 }}
      animate={{ scale: 1, opacity: 1 }}
      style={{
        background: 'linear-gradient(135deg, #1e293b 0%, #0f172a 100%)',
        border: `2px solid ${capacityColor}`,
        borderRadius: '12px',
        padding: '16px',
        width: '300px',
        minHeight: '200px',
        boxShadow: '0 4px 6px rgba(0, 0, 0, 0.3)',
        color: 'white',
        position: 'relative',
      }}
    >
      {/* Header */}
      <div style={{ marginBottom: '12px' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div>
            <div style={{ fontSize: '14px', fontWeight: 'bold' }}>
              {data.type === 'velocity' ? '🚀' : data.type === 'cloud' ? '☁️' : '🏢'} {node.id}
            </div>
            <div style={{ fontSize: '10px', opacity: 0.8 }}>
              {data.ipAddress}
            </div>
          </div>
          <div
            style={{
              fontSize: '9px',
              padding: '2px 6px',
              borderRadius: '4px',
              background: data.status === 'healthy' ? '#10b981' : '#ef4444',
              fontWeight: 'bold',
            }}
          >
            {data.status.toUpperCase()}
          </div>
        </div>
      </div>

      {/* Stats */}
      <div style={{ fontSize: '11px', marginBottom: '12px' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '4px' }}>
          <span>Capacity:</span>
          <span style={{ fontWeight: 'bold', color: capacityColor }}>{data.capacityPercent.toFixed(0)}%</span>
        </div>
        <div style={{ height: '6px', background: 'rgba(255,255,255,0.2)', borderRadius: '3px', overflow: 'hidden', marginBottom: '6px' }}>
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
        <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '10px' }}>
          <span>{data.allocatedRAM} MB used</span>
          <span>{data.freeRAM} MB free</span>
        </div>
        {data.cpuUsagePercent !== undefined && (
          <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: '4px' }}>
            <span>CPU:</span>
            <span style={{
              color: data.cpuUsagePercent > 80 ? '#ef4444' : data.cpuUsagePercent > 60 ? '#f59e0b' : '#10b981',
              fontWeight: 'bold'
            }}>
              {data.cpuUsagePercent.toFixed(1)}%
            </span>
          </div>
        )}
      </div>

      {/* Container Slots */}
      <div style={{ marginTop: '12px', borderTop: '1px solid rgba(255,255,255,0.2)', paddingTop: '12px' }}>
        <div style={{ fontSize: '11px', fontWeight: 'bold', marginBottom: '6px' }}>
          Assigned MC-Servers ({data.containerCount}/{maxSlots}):
        </div>
        <div style={{
          display: 'flex',
          flexDirection: 'column',
          gap: '4px',
          maxHeight: '200px',
          overflowY: 'auto',
        }}>
          {containerSlots.map((container, idx) => (
            <div
              key={idx}
              title={container ? `${container.server_name} (${container.ram_mb}MB) - ${container.status}` : 'Empty slot'}
              style={{
                height: '28px',
                borderRadius: '4px',
                background: container
                  ? getStatusColor(container.status)
                  : 'rgba(255,255,255,0.1)',
                border: container ? 'none' : '1px dashed rgba(255,255,255,0.3)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                padding: '0 8px',
                fontSize: '9px',
                fontWeight: 'bold',
                cursor: container ? 'pointer' : 'default',
                transition: 'all 0.2s',
              }}
            >
              {container ? (
                <>
                  <span style={{
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                    flex: 1,
                  }}>
                    {container.server_name}
                  </span>
                  <span style={{ opacity: 0.7, fontSize: '8px', marginLeft: '4px' }}>
                    {container.ram_mb}MB
                  </span>
                </>
              ) : (
                <span style={{ opacity: 0.5, textAlign: 'center', width: '100%' }}>—</span>
              )}
            </div>
          ))}
        </div>
      </div>

      {/* Type Badge */}
      <div
        style={{
          position: 'absolute',
          bottom: '8px',
          right: '8px',
          fontSize: '8px',
          padding: '2px 4px',
          borderRadius: '3px',
          background: 'rgba(255,255,255,0.2)',
          fontWeight: 'bold',
        }}
      >
        {data.type.toUpperCase()}
      </div>
    </motion.div>
  );
};
