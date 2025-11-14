import { motion, AnimatePresence } from 'framer-motion';
import { useDashboardStore } from '../../stores/dashboardStore';

export const MigrationsList = () => {
  const { migrations } = useDashboardStore();

  const migrationArray = Array.from(migrations.values());
  const activeMigrations = migrationArray.filter(m =>
    m.status === 'started' || m.status === 'progress'
  );
  const completedMigrations = migrationArray.filter(m =>
    m.status === 'completed' || m.status === 'failed'
  );

  if (migrationArray.length === 0) {
    return null;
  }

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

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'started':
      case 'progress':
        return '⏳';
      case 'completed':
        return '✓';
      case 'failed':
        return '✗';
      default:
        return '•';
    }
  };

  return (
    <motion.div
      initial={{ x: 300, opacity: 0 }}
      animate={{ x: 0, opacity: 1 }}
      style={{
        position: 'fixed',
        bottom: '20px',
        right: '20px',
        zIndex: 15,
        background: 'rgba(15, 23, 42, 0.95)',
        border: '2px solid #3b82f6',
        borderRadius: '12px',
        padding: '16px',
        width: '350px',
        maxHeight: '400px',
        overflowY: 'auto',
        color: 'white',
        backdropFilter: 'blur(10px)',
      }}
    >
      <h3 style={{
        margin: '0 0 12px 0',
        fontSize: '16px',
        fontWeight: 'bold',
        color: '#3b82f6',
      }}>
        Server Migrations
      </h3>

      {/* Active Migrations */}
      {activeMigrations.length > 0 && (
        <div style={{ marginBottom: '16px' }}>
          <div style={{
            fontSize: '12px',
            fontWeight: 'bold',
            marginBottom: '8px',
            opacity: 0.7,
          }}>
            Active ({activeMigrations.length})
          </div>
          <AnimatePresence>
            {activeMigrations.map((migration) => (
              <motion.div
                key={migration.operation_id}
                initial={{ opacity: 0, y: -10 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, x: 20 }}
                style={{
                  background: 'rgba(59, 130, 246, 0.1)',
                  border: '1px solid #3b82f6',
                  borderRadius: '8px',
                  padding: '12px',
                  marginBottom: '8px',
                }}
              >
                <div style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'flex-start',
                  marginBottom: '8px',
                }}>
                  <div>
                    <div style={{
                      fontWeight: 'bold',
                      fontSize: '13px',
                      marginBottom: '2px',
                    }}>
                      {migration.server_name}
                    </div>
                    <div style={{
                      fontSize: '11px',
                      opacity: 0.7,
                    }}>
                      {migration.from_node} → {migration.to_node}
                    </div>
                  </div>
                  <div style={{
                    fontSize: '20px',
                  }}>
                    <motion.div
                      animate={{ rotate: 360 }}
                      transition={{ duration: 2, repeat: Infinity, ease: 'linear' }}
                    >
                      ⏳
                    </motion.div>
                  </div>
                </div>

                {/* Progress Bar */}
                <div style={{
                  background: '#1e293b',
                  borderRadius: '4px',
                  height: '6px',
                  overflow: 'hidden',
                  marginBottom: '6px',
                }}>
                  <motion.div
                    initial={{ width: 0 }}
                    animate={{ width: `${migration.progress}%` }}
                    style={{
                      height: '100%',
                      background: '#3b82f6',
                      borderRadius: '4px',
                    }}
                  />
                </div>

                <div style={{
                  fontSize: '11px',
                  opacity: 0.8,
                }}>
                  Progress: {migration.progress}%
                </div>
              </motion.div>
            ))}
          </AnimatePresence>
        </div>
      )}

      {/* Completed/Failed Migrations */}
      {completedMigrations.length > 0 && (
        <div>
          <div style={{
            fontSize: '12px',
            fontWeight: 'bold',
            marginBottom: '8px',
            opacity: 0.7,
          }}>
            Recent ({completedMigrations.length})
          </div>
          <AnimatePresence>
            {completedMigrations.slice(0, 3).map((migration) => (
              <motion.div
                key={migration.operation_id}
                initial={{ opacity: 0, y: -10 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, scale: 0.8 }}
                style={{
                  background: migration.status === 'completed'
                    ? 'rgba(16, 185, 129, 0.1)'
                    : 'rgba(239, 68, 68, 0.1)',
                  border: `1px solid ${getStatusColor(migration.status)}`,
                  borderRadius: '8px',
                  padding: '10px',
                  marginBottom: '6px',
                }}
              >
                <div style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                }}>
                  <div style={{ flex: 1 }}>
                    <div style={{
                      fontWeight: 'bold',
                      fontSize: '12px',
                      marginBottom: '2px',
                    }}>
                      {migration.server_name}
                    </div>
                    <div style={{
                      fontSize: '10px',
                      opacity: 0.7,
                    }}>
                      {migration.from_node} → {migration.to_node}
                    </div>
                    {migration.error && (
                      <div style={{
                        fontSize: '10px',
                        color: '#ef4444',
                        marginTop: '4px',
                      }}>
                        Error: {migration.error}
                      </div>
                    )}
                  </div>
                  <div style={{
                    fontSize: '16px',
                    color: getStatusColor(migration.status),
                  }}>
                    {getStatusIcon(migration.status)}
                  </div>
                </div>
              </motion.div>
            ))}
          </AnimatePresence>
        </div>
      )}
    </motion.div>
  );
};
