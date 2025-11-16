import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';

interface ArchivedServer {
  ID: string;
  Name: string;
  MinecraftVersion: string;
  ServerType: string;
  RAMMb: number;
  Status: string;
  LastStoppedAt: string;
}

export const ArchivedServersPage = () => {
  const [servers, setServers] = useState<ArchivedServer[]>([]);
  const [loading, setLoading] = useState(true);
  const [unarchiving, setUnarchiving] = useState<string | null>(null);

  useEffect(() => {
    fetchArchivedServers();
  }, []);

  const fetchArchivedServers = async () => {
    try {
      const response = await fetch('http://91.98.202.235:8000/api/servers/archived', {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });

      if (response.ok) {
        const data = await response.json();
        setServers(data.servers || []);
      }
    } catch (error) {
      console.error('Failed to fetch archived servers:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleUnarchive = async (serverId: string) => {
    setUnarchiving(serverId);

    try {
      const response = await fetch(`http://91.98.202.235:8000/api/servers/${serverId}/start`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });

      if (response.ok) {
        // Refresh list after unarchive
        setTimeout(() => {
          fetchArchivedServers();
          setUnarchiving(null);
        }, 2000);
      }
    } catch (error) {
      console.error('Failed to unarchive server:', error);
      setUnarchiving(null);
    }
  };

  const formatDate = (dateString: string) => {
    if (!dateString) return 'Unknown';
    const date = new Date(dateString);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

    if (diffDays === 0) return 'Today';
    if (diffDays === 1) return 'Yesterday';
    if (diffDays < 7) return `${diffDays} days ago`;
    if (diffDays < 30) return `${Math.floor(diffDays / 7)} weeks ago`;
    return `${Math.floor(diffDays / 30)} months ago`;
  };

  if (loading) {
    return (
      <div style={{
        minHeight: '100vh',
        background: 'linear-gradient(135deg, #0f172a 0%, #1e293b 100%)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        color: 'white',
      }}>
        <div style={{ fontSize: '18px', opacity: 0.7 }}>Loading archived servers...</div>
      </div>
    );
  }

  return (
    <div style={{
      minHeight: '100vh',
      background: 'linear-gradient(135deg, #0f172a 0%, #1e293b 100%)',
      padding: '40px 20px',
    }}>
      <div style={{ maxWidth: '1400px', margin: '0 auto' }}>
        {/* Header */}
        <motion.div
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          style={{ marginBottom: '32px' }}
        >
          <h1 style={{
            fontSize: '36px',
            fontWeight: 'bold',
            color: 'white',
            marginBottom: '12px',
            display: 'flex',
            alignItems: 'center',
            gap: '16px',
          }}>
            📦 Archived Servers
          </h1>
          <p style={{ fontSize: '16px', color: 'rgba(255,255,255,0.6)', margin: 0 }}>
            Servers archived for cost savings (FREE storage). Unarchive to restore in ~30 seconds.
          </p>
        </motion.div>

        {/* Stats Card */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.1 }}
          style={{
            background: 'rgba(255,255,255,0.05)',
            borderRadius: '12px',
            padding: '20px',
            marginBottom: '24px',
            border: '1px solid rgba(255,255,255,0.1)',
          }}
        >
          <div style={{ display: 'flex', gap: '32px', flexWrap: 'wrap' }}>
            <div>
              <div style={{ fontSize: '12px', color: 'rgba(255,255,255,0.5)', marginBottom: '4px' }}>
                Total Archived
              </div>
              <div style={{ fontSize: '28px', fontWeight: 'bold', color: '#10b981' }}>
                {servers.length}
              </div>
            </div>
            <div>
              <div style={{ fontSize: '12px', color: 'rgba(255,255,255,0.5)', marginBottom: '4px' }}>
                Storage Cost
              </div>
              <div style={{ fontSize: '28px', fontWeight: 'bold', color: '#10b981' }}>
                FREE
              </div>
            </div>
            <div>
              <div style={{ fontSize: '12px', color: 'rgba(255,255,255,0.5)', marginBottom: '4px' }}>
                Restore Time
              </div>
              <div style={{ fontSize: '28px', fontWeight: 'bold', color: '#3b82f6' }}>
                ~30s
              </div>
            </div>
          </div>
        </motion.div>

        {/* Servers Table */}
        {servers.length === 0 ? (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ delay: 0.2 }}
            style={{
              background: 'rgba(255,255,255,0.05)',
              borderRadius: '12px',
              padding: '60px 20px',
              textAlign: 'center',
              border: '1px solid rgba(255,255,255,0.1)',
            }}
          >
            <div style={{ fontSize: '48px', marginBottom: '16px' }}>📭</div>
            <div style={{ fontSize: '18px', color: 'rgba(255,255,255,0.6)' }}>
              No archived servers yet
            </div>
            <div style={{ fontSize: '14px', color: 'rgba(255,255,255,0.4)', marginTop: '8px' }}>
              Servers are automatically archived after 48 hours of inactivity
            </div>
          </motion.div>
        ) : (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ delay: 0.2 }}
            style={{
              background: 'rgba(255,255,255,0.05)',
              borderRadius: '12px',
              overflow: 'hidden',
              border: '1px solid rgba(255,255,255,0.1)',
            }}
          >
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr style={{ borderBottom: '1px solid rgba(255,255,255,0.1)' }}>
                  <th style={{
                    textAlign: 'left',
                    padding: '16px',
                    fontSize: '12px',
                    fontWeight: '600',
                    color: 'rgba(255,255,255,0.6)',
                    textTransform: 'uppercase',
                  }}>
                    Server Name
                  </th>
                  <th style={{
                    textAlign: 'left',
                    padding: '16px',
                    fontSize: '12px',
                    fontWeight: '600',
                    color: 'rgba(255,255,255,0.6)',
                    textTransform: 'uppercase',
                  }}>
                    Version & Type
                  </th>
                  <th style={{
                    textAlign: 'left',
                    padding: '16px',
                    fontSize: '12px',
                    fontWeight: '600',
                    color: 'rgba(255,255,255,0.6)',
                    textTransform: 'uppercase',
                  }}>
                    RAM
                  </th>
                  <th style={{
                    textAlign: 'left',
                    padding: '16px',
                    fontSize: '12px',
                    fontWeight: '600',
                    color: 'rgba(255,255,255,0.6)',
                    textTransform: 'uppercase',
                  }}>
                    Archived
                  </th>
                  <th style={{
                    textAlign: 'right',
                    padding: '16px',
                    fontSize: '12px',
                    fontWeight: '600',
                    color: 'rgba(255,255,255,0.6)',
                    textTransform: 'uppercase',
                  }}>
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody>
                {servers.map((server, index) => (
                  <motion.tr
                    key={server.ID}
                    initial={{ opacity: 0, x: -20 }}
                    animate={{ opacity: 1, x: 0 }}
                    transition={{ delay: 0.3 + index * 0.05 }}
                    style={{
                      borderBottom: '1px solid rgba(255,255,255,0.05)',
                      cursor: 'default',
                    }}
                  >
                    <td style={{ padding: '16px' }}>
                      <div style={{
                        fontSize: '14px',
                        fontWeight: '600',
                        color: 'white',
                        marginBottom: '4px',
                      }}>
                        {server.Name}
                      </div>
                      <div style={{
                        fontSize: '12px',
                        color: 'rgba(255,255,255,0.4)',
                        fontFamily: 'monospace',
                      }}>
                        {server.ID}
                      </div>
                    </td>
                    <td style={{ padding: '16px' }}>
                      <div style={{
                        fontSize: '14px',
                        color: 'white',
                        marginBottom: '4px',
                      }}>
                        {server.MinecraftVersion || 'N/A'}
                      </div>
                      {server.ServerType && (
                        <div style={{
                          display: 'inline-block',
                          fontSize: '11px',
                          padding: '2px 8px',
                          borderRadius: '4px',
                          background: 'rgba(16, 185, 129, 0.2)',
                          border: '1px solid #10b981',
                          color: '#10b981',
                          textTransform: 'uppercase',
                        }}>
                          {server.ServerType}
                        </div>
                      )}
                    </td>
                    <td style={{ padding: '16px' }}>
                      <div style={{ fontSize: '14px', color: 'white' }}>
                        {server.RAMMb} MB
                      </div>
                    </td>
                    <td style={{ padding: '16px' }}>
                      <div style={{ fontSize: '14px', color: 'rgba(255,255,255,0.7)' }}>
                        {formatDate(server.LastStoppedAt)}
                      </div>
                    </td>
                    <td style={{ padding: '16px', textAlign: 'right' }}>
                      <button
                        onClick={() => handleUnarchive(server.ID)}
                        disabled={unarchiving === server.ID}
                        style={{
                          background: unarchiving === server.ID
                            ? 'rgba(107, 114, 128, 0.2)'
                            : 'rgba(59, 130, 246, 0.2)',
                          border: `1px solid ${unarchiving === server.ID ? '#6b7280' : '#3b82f6'}`,
                          borderRadius: '8px',
                          padding: '8px 16px',
                          color: unarchiving === server.ID ? '#9ca3af' : '#3b82f6',
                          cursor: unarchiving === server.ID ? 'not-allowed' : 'pointer',
                          fontSize: '13px',
                          fontWeight: '600',
                          transition: 'all 0.2s',
                          opacity: unarchiving === server.ID ? 0.6 : 1,
                        }}
                      >
                        {unarchiving === server.ID ? '🔄 Unarchiving...' : '▶️ Unarchive & Start'}
                      </button>
                    </td>
                  </motion.tr>
                ))}
              </tbody>
            </table>
          </motion.div>
        )}
      </div>
    </div>
  );
};
