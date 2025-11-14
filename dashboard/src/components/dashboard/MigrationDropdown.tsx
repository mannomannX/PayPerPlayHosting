import { motion, AnimatePresence } from 'framer-motion';
import { useState } from 'react';
import { useDashboardStore } from '../../stores/dashboardStore';
import { MigrationPanel } from './MigrationPanel';

export const MigrationDropdown = () => {
  const { migrations } = useDashboardStore();
  const [isOpen, setIsOpen] = useState(false);
  const [showCreatePanel, setShowCreatePanel] = useState(false);

  const migrationArray = Array.from(migrations.values());
  const activeMigrations = migrationArray.filter(m =>
    m.status !== 'completed' && m.status !== 'failed'
  );
  const hasActive = activeMigrations.length > 0;

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'started':
      case 'progress':
      case 'preparing':
      case 'transferring':
      case 'completing':
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
      case 'preparing':
      case 'transferring':
      case 'completing':
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
    <div style={{ position: 'relative' }}>
      {/* Toggle Button */}
      <motion.button
        whileHover={{ scale: 1.05 }}
        whileTap={{ scale: 0.95 }}
        onClick={() => setIsOpen(!isOpen)}
        style={{
          background: hasActive ? 'rgba(59, 130, 246, 0.2)' : 'rgba(255, 255, 255, 0.1)',
          border: `2px solid ${hasActive ? '#3b82f6' : '#334155'}`,
          borderRadius: '10px',
          padding: '8px 16px',
          color: 'white',
          fontSize: '14px',
          fontWeight: '600',
          cursor: 'pointer',
          display: 'flex',
          alignItems: 'center',
          gap: '8px',
          position: 'relative',
        }}
      >
        <span>🔄</span>
        <span>Migrations</span>
        {hasActive && (
          <motion.div
            initial={{ scale: 0 }}
            animate={{ scale: 1 }}
            style={{
              background: '#3b82f6',
              color: 'white',
              borderRadius: '12px',
              width: '20px',
              height: '20px',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: '11px',
              fontWeight: 'bold',
            }}
          >
            {activeMigrations.length}
          </motion.div>
        )}
        <motion.span
          animate={{ rotate: isOpen ? 180 : 0 }}
          transition={{ duration: 0.2 }}
        >
          ▼
        </motion.span>
      </motion.button>

      {/* Dropdown Panel */}
      <AnimatePresence>
        {isOpen && (
          <motion.div
            initial={{ opacity: 0, y: -10, scale: 0.95 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: -10, scale: 0.95 }}
            transition={{ duration: 0.2 }}
            style={{
              position: 'absolute',
              top: '50px',
              right: 0,
              width: '400px',
              maxHeight: '500px',
              overflowY: 'auto',
              background: 'rgba(15, 23, 42, 0.98)',
              border: '2px solid #3b82f6',
              borderRadius: '12px',
              padding: '16px',
              boxShadow: '0 8px 32px rgba(0, 0, 0, 0.4)',
              backdropFilter: 'blur(10px)',
              zIndex: 100,
            }}
          >
            <div style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              marginBottom: '16px',
            }}>
              <h3 style={{
                margin: 0,
                fontSize: '16px',
                fontWeight: 'bold',
                color: '#3b82f6',
              }}>
                Server Migrations
              </h3>
              <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                {/* Plus Button to Create Migration */}
                <motion.button
                  whileHover={{ scale: 1.1 }}
                  whileTap={{ scale: 0.9 }}
                  onClick={() => setShowCreatePanel(!showCreatePanel)}
                  style={{
                    background: showCreatePanel ? '#3b82f6' : 'transparent',
                    border: '2px solid #3b82f6',
                    borderRadius: '6px',
                    color: showCreatePanel ? 'white' : '#3b82f6',
                    fontSize: '18px',
                    cursor: 'pointer',
                    padding: '4px',
                    width: '28px',
                    height: '28px',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontWeight: 'bold',
                  }}
                  title="Create Manual Migration"
                >
                  {showCreatePanel ? '−' : '+'}
                </motion.button>
                {/* Close Button */}
                <button
                  onClick={() => setIsOpen(false)}
                  style={{
                    background: 'transparent',
                    border: 'none',
                    color: '#94a3b8',
                    fontSize: '20px',
                    cursor: 'pointer',
                    padding: '0',
                    width: '24px',
                    height: '24px',
                  }}
                >
                  ×
                </button>
              </div>
            </div>

            {/* Create Migration Panel */}
            <AnimatePresence>
              {showCreatePanel && (
                <motion.div
                  initial={{ height: 0, opacity: 0 }}
                  animate={{ height: 'auto', opacity: 1 }}
                  exit={{ height: 0, opacity: 0 }}
                  transition={{ duration: 0.2 }}
                  style={{ overflow: 'hidden', marginBottom: '16px' }}
                >
                  <MigrationPanel inline={true} />
                </motion.div>
              )}
            </AnimatePresence>

            {migrationArray.length === 0 && !showCreatePanel ? (
              <div style={{
                padding: '40px 20px',
                textAlign: 'center',
                color: '#64748b',
                fontSize: '14px',
              }}>
                No migrations running or recent
              </div>
            ) : (
              <>
                {/* Active Migrations */}
                {activeMigrations.length > 0 && (
                  <div style={{ marginBottom: '16px' }}>
                    <div style={{
                      fontSize: '12px',
                      fontWeight: 'bold',
                      marginBottom: '8px',
                      color: '#94a3b8',
                      textTransform: 'uppercase',
                      letterSpacing: '0.5px',
                    }}>
                      Active ({activeMigrations.length})
                    </div>
                    {activeMigrations.map((migration) => (
                      <motion.div
                        key={migration.operation_id}
                        initial={{ opacity: 0, x: -10 }}
                        animate={{ opacity: 1, x: 0 }}
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
                              color: 'white',
                              marginBottom: '2px',
                            }}>
                              {migration.server_name}
                            </div>
                            <div style={{
                              fontSize: '11px',
                              color: '#94a3b8',
                            }}>
                              {migration.from_node} → {migration.to_node}
                            </div>
                          </div>
                          <motion.div
                            animate={{ rotate: 360 }}
                            transition={{ duration: 2, repeat: Infinity, ease: 'linear' }}
                            style={{ fontSize: '20px' }}
                          >
                            ⏳
                          </motion.div>
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
                          color: '#94a3b8',
                        }}>
                          Progress: {migration.progress}% • {migration.status}
                        </div>
                      </motion.div>
                    ))}
                  </div>
                )}

                {/* Recent Completed/Failed */}
                {migrationArray.filter(m => m.status === 'completed' || m.status === 'failed').length > 0 && (
                  <div>
                    <div style={{
                      fontSize: '12px',
                      fontWeight: 'bold',
                      marginBottom: '8px',
                      color: '#94a3b8',
                      textTransform: 'uppercase',
                      letterSpacing: '0.5px',
                    }}>
                      Recent
                    </div>
                    {migrationArray
                      .filter(m => m.status === 'completed' || m.status === 'failed')
                      .slice(0, 3)
                      .map((migration) => (
                        <motion.div
                          key={migration.operation_id}
                          initial={{ opacity: 0, x: -10 }}
                          animate={{ opacity: 1, x: 0 }}
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
                                color: 'white',
                                marginBottom: '2px',
                              }}>
                                {migration.server_name}
                              </div>
                              <div style={{
                                fontSize: '10px',
                                color: '#94a3b8',
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
                  </div>
                )}
              </>
            )}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
};
