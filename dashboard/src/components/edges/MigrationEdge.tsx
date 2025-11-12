import type { EdgeProps } from 'reactflow';
import { getSmoothStepPath } from 'reactflow';
import { motion } from 'framer-motion';

export const MigrationEdge = ({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  data,
}: EdgeProps) => {
  const [edgePath, labelX, labelY] = getSmoothStepPath({
    sourceX,
    sourceY,
    sourcePosition,
    targetX,
    targetY,
    targetPosition,
  });

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'started':
      case 'progress':
        return '#3b82f6'; // blue
      case 'completed':
        return '#10b981'; // green
      case 'failed':
        return '#ef4444'; // red
      default:
        return '#6b7280'; // gray
    }
  };

  const color = getStatusColor(data?.status || 'started');
  const progress = data?.progress || 0;

  return (
    <>
      {/* Base Path */}
      <path
        id={id}
        style={{
          stroke: color,
          strokeWidth: 3,
          fill: 'none',
          strokeDasharray: '5, 5',
          opacity: 0.3,
        }}
        d={edgePath}
      />

      {/* Animated Path */}
      <motion.path
        initial={{ pathLength: 0 }}
        animate={{ pathLength: progress / 100 }}
        transition={{ duration: 0.5, ease: 'easeInOut' }}
        style={{
          stroke: color,
          strokeWidth: 4,
          fill: 'none',
          filter: 'drop-shadow(0 0 4px rgba(59, 130, 246, 0.6))',
        }}
        d={edgePath}
      />

      {/* Progress Label */}
      <foreignObject
        width={120}
        height={50}
        x={labelX - 60}
        y={labelY - 25}
        style={{ overflow: 'visible' }}
      >
        <motion.div
          initial={{ scale: 0, opacity: 0 }}
          animate={{ scale: 1, opacity: 1 }}
          exit={{ scale: 0, opacity: 0 }}
          style={{
            background: color,
            color: 'white',
            padding: '6px 10px',
            borderRadius: '8px',
            fontSize: '11px',
            fontWeight: 'bold',
            textAlign: 'center',
            boxShadow: '0 2px 8px rgba(0, 0, 0, 0.2)',
            border: '2px solid white',
          }}
        >
          <div style={{ marginBottom: '2px' }}>🔄 MIGRATION</div>
          <div style={{ fontSize: '10px', opacity: 0.9 }}>
            {data?.server_name || 'Server'}
          </div>
          <div style={{ marginTop: '4px', fontSize: '14px' }}>
            {progress}%
          </div>
        </motion.div>
      </foreignObject>

      {/* Animated Particles */}
      {(data?.status === 'started' || data?.status === 'progress') && (
        <>
          {[0, 0.33, 0.66].map((offset) => (
            <motion.circle
              key={offset}
              r="3"
              fill={color}
              initial={{ offsetDistance: '0%' }}
              animate={{ offsetDistance: '100%' }}
              transition={{
                duration: 2,
                repeat: Infinity,
                delay: offset * 0.66,
                ease: 'linear',
              }}
              style={{
                offsetPath: `path('${edgePath}')`,
                filter: 'drop-shadow(0 0 4px rgba(59, 130, 246, 0.8))',
              }}
            />
          ))}
        </>
      )}
    </>
  );
};
