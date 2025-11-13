import { motion } from 'framer-motion';
import { useState, useEffect, useRef } from 'react';

interface DebugLog {
  timestamp: string;
  level: string;
  message: string;
  fields?: Record<string, any>;
}

export const DebugConsole = () => {
  const [logs, setLogs] = useState<DebugLog[]>([]);
  const [isPaused, setIsPaused] = useState(false);
  const [isExpanded, setIsExpanded] = useState(true);
  const logsEndRef = useRef<HTMLDivElement>(null);

  // Fetch debug logs
  const fetchLogs = async () => {
    if (isPaused) return;

    try {
      const response = await fetch('/conductor/debug-logs');
      const data = await response.json();
      if (data.status === 'ok' && data.data) {
        setLogs(data.data);
        // Auto-scroll to bottom
        setTimeout(() => {
          logsEndRef.current?.scrollIntoView({ behavior: 'smooth' });
        }, 100);
      }
    } catch (error) {
      console.error('Failed to fetch debug logs:', error);
    }
  };

  // Clear logs
  const clearLogs = async () => {
    try {
      await fetch('/conductor/debug-logs', { method: 'DELETE' });
      setLogs([]);
    } catch (error) {
      console.error('Failed to clear debug logs:', error);
    }
  };

  // Poll every 3 seconds
  useEffect(() => {
    fetchLogs(); // Initial fetch
    const interval = setInterval(fetchLogs, 3000);
    return () => clearInterval(interval);
  }, [isPaused]);

  // Get color for log level
  const getLevelColor = (level: string) => {
    switch (level) {
      case 'INFO': return '#3b82f6';
      case 'WARN': return '#f59e0b';
      case 'ERROR': return '#ef4444';
      default: return '#94a3b8';
    }
  };

  return (
    <motion.div
      initial={{ y: 50, opacity: 0 }}
      animate={{ y: 0, opacity: 1 }}
      style={{
        position: 'fixed',
        bottom: '20px',
        right: '20px',
        zIndex: 10,
        background: 'rgba(15, 23, 42, 0.95)',
        border: '2px solid #334155',
        borderRadius: '12px',
        width: isExpanded ? '600px' : '200px',
        maxHeight: isExpanded ? '400px' : '50px',
        color: 'white',
        backdropFilter: 'blur(10px)',
        overflow: 'hidden',
        transition: 'all 0.3s ease',
      }}
    >
      {/* Header */}
      <div
        style={{
          padding: '12px 16px',
          borderBottom: isExpanded ? '1px solid #334155' : 'none',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          cursor: 'pointer',
        }}
        onClick={() => setIsExpanded(!isExpanded)}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <span style={{ fontSize: '16px' }}>🐛</span>
          <h3 style={{ margin: 0, fontSize: '14px', fontWeight: 'bold' }}>
            Debug Console
          </h3>
          <span style={{
            fontSize: '10px',
            background: '#1e293b',
            padding: '2px 6px',
            borderRadius: '4px',
          }}>
            {logs.length}
          </span>
        </div>

        <div
          style={{ display: 'flex', gap: '8px', alignItems: 'center' }}
          onClick={(e) => e.stopPropagation()}
        >
          <button
            onClick={clearLogs}
            style={{
              background: '#1e293b',
              border: '1px solid #334155',
              color: 'white',
              padding: '4px 8px',
              borderRadius: '4px',
              fontSize: '11px',
              cursor: 'pointer',
              transition: 'background 0.2s',
            }}
            onMouseOver={(e) => e.currentTarget.style.background = '#334155'}
            onMouseOut={(e) => e.currentTarget.style.background = '#1e293b'}
          >
            Clear
          </button>
          <button
            onClick={() => setIsPaused(!isPaused)}
            style={{
              background: isPaused ? '#1e293b' : '#10b981',
              border: `1px solid ${isPaused ? '#334155' : '#10b981'}`,
              color: 'white',
              padding: '4px 8px',
              borderRadius: '4px',
              fontSize: '11px',
              cursor: 'pointer',
              transition: 'background 0.2s',
            }}
          >
            {isPaused ? '⏸ Paused' : '▶ Live'}
          </button>
          <button
            onClick={() => setIsExpanded(!isExpanded)}
            style={{
              background: 'transparent',
              border: 'none',
              color: '#94a3b8',
              fontSize: '14px',
              cursor: 'pointer',
              padding: '4px',
            }}
          >
            {isExpanded ? '▼' : '▲'}
          </button>
        </div>
      </div>

      {/* Logs */}
      {isExpanded && (
        <div
          style={{
            padding: '12px',
            maxHeight: '340px',
            overflowY: 'auto',
            fontSize: '11px',
            fontFamily: 'monospace',
          }}
        >
          {logs.length === 0 ? (
            <div style={{ color: '#64748b', textAlign: 'center', padding: '20px' }}>
              No debug logs yet. Scaling decisions will appear here.
            </div>
          ) : (
            logs.map((log, idx) => (
              <motion.div
                key={`${log.timestamp}-${idx}`}
                initial={{ opacity: 0, x: -10 }}
                animate={{ opacity: 1, x: 0 }}
                style={{
                  marginBottom: '8px',
                  padding: '6px 8px',
                  background: 'rgba(30, 41, 59, 0.5)',
                  borderLeft: `3px solid ${getLevelColor(log.level)}`,
                  borderRadius: '4px',
                }}
              >
                <div style={{ display: 'flex', gap: '8px', marginBottom: '2px' }}>
                  <span style={{ color: '#64748b' }}>
                    {new Date(log.timestamp).toLocaleTimeString()}
                  </span>
                  <span
                    style={{
                      color: getLevelColor(log.level),
                      fontWeight: 'bold',
                    }}
                  >
                    {log.level}
                  </span>
                </div>
                <div style={{ color: '#e2e8f0' }}>{log.message}</div>
              </motion.div>
            ))
          )}
          <div ref={logsEndRef} />
        </div>
      )}
    </motion.div>
  );
};
