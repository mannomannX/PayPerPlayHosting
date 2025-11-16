import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import { PageNavigation } from '../components/navigation/PageNavigation';

interface ArchivedServer {
  ID: string;
  Name: string;
  MinecraftVersion: string;
  ServerType: string;
  RAMMb: number;
  Status: string;
  LastStoppedAt: string;
  ArchivedAt: string;
  ArchiveLocation: string;
  ArchiveSize: number; // Size in bytes
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
      // Use admin endpoint (no auth required, like other dashboard endpoints)
      const response = await fetch('/admin/servers/archived');

      if (response.ok) {
        const data = await response.json();
        // Filter out corrupted archives (0 bytes)
        const validServers = (data.servers || []).filter((s: ArchivedServer) => s.ArchiveSize > 0);
        setServers(validServers);
      } else {
        console.error('Failed to fetch archived servers:', response.status, response.statusText);
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
      // TODO: This needs auth - for now, show error message
      alert('Unarchive functionality requires authentication. Please use the API directly for now.');
      setUnarchiving(null);

      // const response = await fetch(`/api/servers/${serverId}/start`, {
      //   method: 'POST',
      // });
      //
      // if (response.ok) {
      //   setTimeout(() => {
      //     fetchArchivedServers();
      //     setUnarchiving(null);
      //   }, 2000);
      // }
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

  const formatFileSize = (bytes: number): string => {
    if (!bytes || bytes === 0) return 'N/A';
    const mb = bytes / 1024 / 1024;
    if (mb < 1024) return `${mb.toFixed(1)} MB`;
    return `${(mb / 1024).toFixed(2)} GB`;
  };

  const getStorageType = (location: string): string => {
    if (!location) return 'Unknown';
    if (location.includes('minecraft-archives/')) return 'Hetzner Storage Box (SFTP)';
    if (location.includes('.archives/')) return 'Local Fallback';
    return 'Unknown';
  };

  const getCompressionInfo = (archiveSize: number, ramMb: number): { ratio: string; savings: string } => {
    if (!archiveSize || !ramMb) return { ratio: 'N/A', savings: 'N/A' };

    // Estimate original size as roughly RAM size (world data typically ~70-80% of RAM usage)
    const estimatedOriginalMB = ramMb * 0.75;
    const archiveMB = archiveSize / 1024 / 1024;
    const compressionRatio = estimatedOriginalMB / archiveMB;
    const savingsPercent = ((estimatedOriginalMB - archiveMB) / estimatedOriginalMB) * 100;

    return {
      ratio: `${compressionRatio.toFixed(1)}:1`,
      savings: `${savingsPercent.toFixed(0)}%`,
    };
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
    }}>
      {/* Header with Navigation */}
      <motion.div
        initial={{ y: -100, opacity: 0 }}
        animate={{ y: 0, opacity: 1 }}
        style={{
          position: 'fixed',
          top: 0,
          left: 0,
          right: 0,
          zIndex: 20,
          background: 'rgba(15, 23, 42, 0.95)',
          backdropFilter: 'blur(10px)',
          borderBottom: '1px solid #334155',
          padding: '16px 30px',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '24px' }}>
          <div>
            <h1 style={{ color: 'white', margin: 0, fontSize: '24px', fontWeight: 'bold' }}>
              📦 Archived Servers
            </h1>
            <p style={{ color: '#94a3b8', margin: '4px 0 0 0', fontSize: '12px' }}>
              FREE storage • ~30s restore time
            </p>
          </div>

          {/* Page Navigation */}
          <PageNavigation />
        </div>
      </motion.div>

      {/* Content */}
      <div style={{ paddingTop: '100px', padding: '100px 20px 40px 20px' }}>
        <div style={{ maxWidth: '1400px', margin: '0 auto' }}>

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
                Total Size
              </div>
              <div style={{ fontSize: '28px', fontWeight: 'bold', color: '#3b82f6' }}>
                {formatFileSize(servers.reduce((sum, s) => sum + (s.ArchiveSize || 0), 0))}
              </div>
              <div style={{ fontSize: '10px', color: 'rgba(255,255,255,0.3)', marginTop: '2px' }}>
                tar.gz compressed
              </div>
            </div>
            <div>
              <div style={{ fontSize: '12px', color: 'rgba(255,255,255,0.5)', marginBottom: '4px' }}>
                Avg Compression
              </div>
              <div style={{ fontSize: '28px', fontWeight: 'bold', color: '#10b981' }}>
                {(() => {
                  if (servers.length === 0) return 'N/A';
                  const avgRatio = servers
                    .filter(s => s.ArchiveSize && s.RAMMb)
                    .map(s => {
                      const estimatedOriginalMB = s.RAMMb * 0.75;
                      const archiveMB = s.ArchiveSize / 1024 / 1024;
                      return estimatedOriginalMB / archiveMB;
                    })
                    .reduce((sum, ratio, _, arr) => sum + ratio / arr.length, 0);
                  return `${avgRatio.toFixed(1)}:1`;
                })()}
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
                    Archive Size
                  </th>
                  <th style={{
                    textAlign: 'left',
                    padding: '16px',
                    fontSize: '12px',
                    fontWeight: '600',
                    color: 'rgba(255,255,255,0.6)',
                    textTransform: 'uppercase',
                  }}>
                    Compression
                  </th>
                  <th style={{
                    textAlign: 'left',
                    padding: '16px',
                    fontSize: '12px',
                    fontWeight: '600',
                    color: 'rgba(255,255,255,0.6)',
                    textTransform: 'uppercase',
                  }}>
                    Storage
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
                      <div style={{ fontSize: '14px', color: 'white', fontWeight: '600' }}>
                        {formatFileSize(server.ArchiveSize)}
                      </div>
                      <div style={{ fontSize: '11px', color: 'rgba(255,255,255,0.4)', marginTop: '2px' }}>
                        tar.gz compressed
                      </div>
                    </td>
                    <td style={{ padding: '16px' }}>
                      {(() => {
                        const compression = getCompressionInfo(server.ArchiveSize, server.RAMMb);
                        return (
                          <>
                            <div style={{ fontSize: '14px', color: '#10b981', fontWeight: '600' }}>
                              {compression.ratio}
                            </div>
                            <div style={{ fontSize: '11px', color: 'rgba(255,255,255,0.4)', marginTop: '2px' }}>
                              {compression.savings} saved
                            </div>
                          </>
                        );
                      })()}
                    </td>
                    <td style={{ padding: '16px' }}>
                      <div style={{
                        display: 'inline-block',
                        fontSize: '11px',
                        padding: '4px 8px',
                        borderRadius: '4px',
                        background: getStorageType(server.ArchiveLocation).includes('Hetzner')
                          ? 'rgba(59, 130, 246, 0.2)'
                          : 'rgba(168, 85, 247, 0.2)',
                        border: getStorageType(server.ArchiveLocation).includes('Hetzner')
                          ? '1px solid #3b82f6'
                          : '1px solid #a855f7',
                        color: getStorageType(server.ArchiveLocation).includes('Hetzner')
                          ? '#60a5fa'
                          : '#c084fc',
                      }}>
                        {getStorageType(server.ArchiveLocation).includes('Hetzner') ? '☁️ Remote' : '💾 Local'}
                      </div>
                      <div style={{ fontSize: '10px', color: 'rgba(255,255,255,0.3)', marginTop: '4px' }}>
                        {getStorageType(server.ArchiveLocation)}
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
    </div>
  );
};
