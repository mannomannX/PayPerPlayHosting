import { motion } from 'framer-motion';
import { useState } from 'react';
import { ContainerDetailsModal } from '../dashboard/ContainerDetailsModal';

interface Container {
  server_id: string;
  server_name: string;
  ram_mb: number;
  status: string;
  port: number;
  join_address: string;
  minecraft_version?: string;
  server_type?: string;
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
      isSystemNode: boolean;
      containers: Container[];
    };
  };
  getStatusColor: (status: string) => string;
}

export const GridNode = ({ node, getStatusColor }: GridNodeProps) => {
  const { data } = node;
  const [selectedContainer, setSelectedContainer] = useState<Container | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);

  const getCapacityColor = (percent: number) => {
    if (percent < 50) return '#10b981'; // green
    if (percent < 70) return '#f59e0b'; // yellow
    if (percent < 85) return '#f97316'; // orange
    return '#ef4444'; // red
  };

  const handleContainerClick = (container: Container) => {
    setSelectedContainer(container);
    setIsModalOpen(true);
  };

  const capacityColor = getCapacityColor(data.capacityPercent);

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
          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px', alignItems: 'flex-end' }}>
            {data.isSystemNode && (
              <div
                style={{
                  fontSize: '9px',
                  padding: '2px 6px',
                  borderRadius: '4px',
                  background: '#6366f1',
                  fontWeight: 'bold',
                  color: 'white',
                }}
              >
                SYSTEM
              </div>
            )}
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
          MC-Containers ({data.containers.length}):
        </div>
        <div style={{
          display: 'flex',
          flexDirection: 'column',
          gap: '4px',
          maxHeight: '200px',
          overflowY: 'auto',
        }}>
          {data.containers.length === 0 ? (
            <div style={{
              height: '32px',
              borderRadius: '4px',
              background: 'rgba(255,255,255,0.05)',
              border: '1px dashed rgba(255,255,255,0.2)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: '10px',
              color: 'rgba(255,255,255,0.5)',
            }}>
              No containers
            </div>
          ) : (
            data.containers.map((container, idx) => {
              // Calculate height based on RAM (min 24px, max 60px, proportional to RAM)
              const ramGB = container.ram_mb / 1024;
              const baseHeight = 24;
              const heightPerGB = 8;
              const containerHeight = Math.min(60, Math.max(baseHeight, baseHeight + (ramGB * heightPerGB)));

              return (
                <motion.div
                  key={container.server_id}
                  initial={{ opacity: 0, x: -10 }}
                  animate={{ opacity: 1, x: 0 }}
                  transition={{ delay: idx * 0.05 }}
                  onClick={() => handleContainerClick(container)}
                  title={`${container.server_name}\n${(container.ram_mb / 1024).toFixed(1)} GB RAM\nStatus: ${container.status}\nClick for details`}
                  style={{
                    height: `${containerHeight}px`,
                    borderRadius: '6px',
                    background: getStatusColor(container.status),
                    border: 'none',
                    display: 'flex',
                    flexDirection: 'column',
                    justifyContent: 'center',
                    padding: '0 10px',
                    fontSize: '9px',
                    fontWeight: 'bold',
                    cursor: 'pointer',
                    transition: 'all 0.2s',
                    boxShadow: '0 2px 4px rgba(0,0,0,0.3)',
                  }}
                  whileHover={{
                    scale: 1.02,
                    boxShadow: '0 4px 8px rgba(0,0,0,0.4)',
                  }}
                >
                  <div style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    width: '100%',
                  }}>
                    <span style={{
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                      flex: 1,
                      fontSize: '10px',
                    }}>
                      {container.server_name}
                    </span>
                    <span style={{
                      opacity: 0.9,
                      fontSize: '9px',
                      marginLeft: '6px',
                      background: 'rgba(0,0,0,0.2)',
                      padding: '2px 5px',
                      borderRadius: '3px',
                      whiteSpace: 'nowrap',
                    }}>
                      {(container.ram_mb / 1024).toFixed(1)}GB
                    </span>
                  </div>
                  {containerHeight > 35 && (
                    <div style={{
                      fontSize: '7px',
                      opacity: 0.7,
                      marginTop: '2px',
                    }}>
                      {container.status.toUpperCase()}
                    </div>
                  )}
                </motion.div>
              );
            })
          )}
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

      {/* Container Details Modal */}
      <ContainerDetailsModal
        isOpen={isModalOpen}
        onClose={() => {
          setIsModalOpen(false);
          setSelectedContainer(null);
        }}
        container={selectedContainer}
        nodeId={node.id}
        nodeIp={data.ipAddress}
      />
    </motion.div>
  );
};
