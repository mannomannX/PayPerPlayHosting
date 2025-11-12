import { motion } from 'framer-motion';
import { useState } from 'react';

interface ManhattanArrowProps {
  fromNode: { id: string; x: number; y: number; width: number; height: number };
  toNode: { id: string; x: number; y: number; width: number; height: number };
  migration: {
    operation_id: string;
    server_name: string;
    progress: number;
    status: 'started' | 'progress' | 'completed' | 'failed';
    error?: string;
  };
  isSelected: boolean;
  onClick: () => void;
}

export const ManhattanArrow = ({
  fromNode,
  toNode,
  migration,
  isSelected,
  onClick,
}: ManhattanArrowProps) => {
  const [isHovered, setIsHovered] = useState(false);

  // Calculate start and end points (center-right of fromNode, center-left of toNode)
  const startX = fromNode.x + fromNode.width;
  const startY = fromNode.y + fromNode.height / 2;
  const endX = toNode.x;
  const endY = toNode.y + toNode.height / 2;

  // Manhattan routing: horizontal -> vertical -> horizontal
  const midX = (startX + endX) / 2;

  // Create path
  const pathData = `
    M ${startX} ${startY}
    L ${midX} ${startY}
    L ${midX} ${endY}
    L ${endX} ${endY}
  `;

  // Arrow color based on status
  const getColor = () => {
    switch (migration.status) {
      case 'completed': return '#10b981'; // green
      case 'failed': return '#ef4444'; // red
      case 'progress': return '#3b82f6'; // blue
      default: return '#f59e0b'; // yellow
    }
  };

  const color = getColor();
  const opacity = isSelected ? 1 : isHovered ? 0.9 : 0.7;

  return (
    <g
      onClick={onClick}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
      style={{ cursor: 'pointer' }}
    >
      {/* Invisible wider path for easier clicking */}
      <path
        d={pathData}
        stroke="transparent"
        strokeWidth="20"
        fill="none"
      />

      {/* Main path */}
      <motion.path
        d={pathData}
        stroke={color}
        strokeWidth={isSelected ? 4 : isHovered ? 3 : 2}
        fill="none"
        strokeDasharray={migration.status === 'completed' ? '0' : '8 4'}
        initial={{ pathLength: 0, opacity: 0 }}
        animate={{
          pathLength: 1,
          opacity: opacity,
        }}
        transition={{ duration: 0.5 }}
      />

      {/* Animated dot traveling along path */}
      {migration.status === 'progress' && (
        <motion.circle
          r="4"
          fill={color}
          initial={{ offsetDistance: '0%', opacity: 0 }}
          animate={{
            offsetDistance: '100%',
            opacity: [0, 1, 1, 0],
          }}
          transition={{
            duration: 2,
            repeat: Infinity,
            ease: 'linear',
          }}
        >
          <animateMotion dur="2s" repeatCount="indefinite">
            <mpath href={`#path-${migration.operation_id}`} />
          </animateMotion>
        </motion.circle>
      )}

      {/* Arrow head */}
      <polygon
        points={`${endX},${endY} ${endX - 10},${endY - 6} ${endX - 10},${endY + 6}`}
        fill={color}
        opacity={opacity}
      />

      {/* Label */}
      {(isHovered || isSelected) && (
        <g>
          {/* Background */}
          <rect
            x={midX - 60}
            y={Math.min(startY, endY) + Math.abs(startY - endY) / 2 - 25}
            width="120"
            height="50"
            fill="rgba(15, 23, 42, 0.95)"
            stroke={color}
            strokeWidth="2"
            rx="6"
          />
          {/* Text */}
          <text
            x={midX}
            y={Math.min(startY, endY) + Math.abs(startY - endY) / 2 - 10}
            textAnchor="middle"
            fill="white"
            fontSize="11"
            fontWeight="bold"
          >
            {migration.server_name}
          </text>
          <text
            x={midX}
            y={Math.min(startY, endY) + Math.abs(startY - endY) / 2 + 5}
            textAnchor="middle"
            fill={color}
            fontSize="10"
          >
            {migration.status === 'progress'
              ? `${migration.progress}%`
              : migration.status.toUpperCase()}
          </text>
          {migration.error && (
            <text
              x={midX}
              y={Math.min(startY, endY) + Math.abs(startY - endY) / 2 + 18}
              textAnchor="middle"
              fill="#ef4444"
              fontSize="9"
            >
              Error
            </text>
          )}
        </g>
      )}

      {/* Hidden path for animateMotion reference */}
      <path
        id={`path-${migration.operation_id}`}
        d={pathData}
        fill="none"
        stroke="none"
      />
    </g>
  );
};
