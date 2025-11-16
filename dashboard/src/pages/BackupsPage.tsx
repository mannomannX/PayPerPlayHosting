import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { PageNavigation } from '../components/navigation/PageNavigation';

interface Backup {
  ID: string;
  ServerID: string;
  ServerName: string;
  Type: string;
  Status: string;
  Description: string;
  CompressedSize: number;
  OriginalSize: number;
  MinecraftVersion: string;
  ServerType: string;
  RAMMb: number;
  CreatedAt: string;
  CompletedAt: string;
  RestoredCount: number;
  RetentionDays: number;
  ExpiresAt: string;
}

interface QuotaInfo {
  plan: string;
  backups_today: number;
  max_backups_day: number;
  backups_remaining: number;
  storage_used_gb: number;
  storage_quota_gb: number;
  storage_unlimited: boolean;
  total_backups: number;
  restores_this_month: number;
  max_restores_month: number;
  restores_unlimited: boolean;
  restores_remaining: number;
}

export const BackupsPage = () => {
  const [backups, setBackups] = useState<Backup[]>([]);
  const [quota, setQuota] = useState<QuotaInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [restoring, setRestoring] = useState<string | null>(null);

  // TODO: Get actual user ID from auth context
  const userID = '57fdf943-04cd-4c44-99d8-9e814377bf183'; // Placeholder

  useEffect(() => {
    fetchBackups();
    fetchQuota();
  }, []);

  const fetchBackups = async () => {
    try {
      const response = await fetch(`/api/users/${userID}/backups`, {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });

      if (response.ok) {
        const data = await response.json();
        setBackups(data.backups || []);
      } else {
        console.error('Failed to fetch backups:', response.status);
      }
    } catch (error) {
      console.error('Failed to fetch backups:', error);
    } finally {
      setLoading(false);
    }
  };

  const fetchQuota = async () => {
    try {
      const response = await fetch(`/api/users/${userID}/backups/quota`, {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });

      if (response.ok) {
        const data = await response.json();
        setQuota(data);
      }
    } catch (error) {
      console.error('Failed to fetch quota:', error);
    }
  };

  const handleRestore = async (backupID: string) => {
    if (!confirm('Are you sure you want to restore this backup? This will overwrite current server data.')) {
      return;
    }

    setRestoring(backupID);

    try {
      const response = await fetch(`/api/users/${userID}/backups/${backupID}/restore`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });

      if (response.ok) {
        alert('Backup restored successfully!');
        fetchBackups(); // Refresh list
        fetchQuota(); // Refresh quota
      } else {
        const data = await response.json();
        alert(`Failed to restore backup: ${data.error || 'Unknown error'}`);
      }
    } catch (error) {
      alert(`Failed to restore backup: ${error}`);
    } finally {
      setRestoring(null);
    }
  };

  const formatDate = (dateString: string) => {
    if (!dateString) return 'Unknown';
    return new Date(dateString).toLocaleString();
  };

  const formatFileSize = (bytes: number): string => {
    if (!bytes || bytes === 0) return 'N/A';
    const mb = bytes / 1024 / 1024;
    if (mb < 1024) return `${mb.toFixed(1)} MB`;
    return `${(mb / 1024).toFixed(2)} GB`;
  };

  const getCompressionRatio = (originalSize: number, compressedSize: number): string => {
    if (!originalSize || !compressedSize) return 'N/A';
    const ratio = originalSize / compressedSize;
    return `${ratio.toFixed(1)}:1`;
  };

  const getTypeLabel = (type: string): { label: string; color: string } => {
    switch (type) {
      case 'manual':
        return { label: 'Manual', color: '#3b82f6' };
      case 'scheduled':
        return { label: 'Scheduled', color: '#10b981' };
      case 'pre-migration':
        return { label: 'Pre-Migration', color: '#f59e0b' };
      case 'pre-deletion':
        return { label: 'Pre-Deletion', color: '#ef4444' };
      default:
        return { label: type, color: '#6b7280' };
    }
  };

  if (loading) {
    return (
      <div style={{
        minHeight: '100vh',
        background: 'linear-gradient(135deg, #0f172a 0%, #1e293b 100%)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}>
        <div style={{ color: 'white', fontSize: '18px' }}>Loading backups...</div>
      </div>
    );
  }

  return (
    <div style={{
      minHeight: '100vh',
      background: 'linear-gradient(135deg, #0f172a 0%, #1e293b 100%)',
    }}>
      {/* Fixed Header */}
      <div style={{
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        background: 'rgba(15, 23, 42, 0.95)',
        backdropFilter: 'blur(10px)',
        borderBottom: '1px solid rgba(255,255,255,0.1)',
        padding: '16px 20px',
        zIndex: 100,
      }}>
        <div style={{ maxWidth: '1400px', margin: '0 auto', display: 'flex', alignItems: 'center', gap: '24px' }}>
          <div>
            <h1 style={{ color: 'white', margin: 0, fontSize: '24px', fontWeight: 'bold' }}>
              💾 Backups
            </h1>
            <p style={{ color: '#94a3b8', margin: '4px 0 0 0', fontSize: '12px' }}>
              Manage server backups and restores
            </p>
          </div>

          {/* Page Navigation */}
          <PageNavigation />
        </div>
      </div>

      {/* Content */}
      <div style={{ paddingTop: '100px', padding: '100px 20px 40px 20px' }}>
        <div style={{ maxWidth: '1400px', margin: '0 auto' }}>

          {/* Quota Info Card */}
          {quota && (
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
                    Plan
                  </div>
                  <div style={{ fontSize: '28px', fontWeight: 'bold', color: '#60a5fa', textTransform: 'capitalize' }}>
                    {quota.plan}
                  </div>
                </div>

                <div>
                  <div style={{ fontSize: '12px', color: 'rgba(255,255,255,0.5)', marginBottom: '4px' }}>
                    Backups Today
                  </div>
                  <div style={{ fontSize: '28px', fontWeight: 'bold', color: quota.backups_remaining > 0 ? '#10b981' : '#ef4444' }}>
                    {quota.backups_today} / {quota.max_backups_day}
                  </div>
                  <div style={{ fontSize: '10px', color: 'rgba(255,255,255,0.3)', marginTop: '2px' }}>
                    {quota.backups_remaining} remaining
                  </div>
                </div>

                <div>
                  <div style={{ fontSize: '12px', color: 'rgba(255,255,255,0.5)', marginBottom: '4px' }}>
                    Storage Used
                  </div>
                  <div style={{ fontSize: '28px', fontWeight: 'bold', color: '#3b82f6' }}>
                    {quota.storage_used_gb.toFixed(2)} GB
                  </div>
                  <div style={{ fontSize: '10px', color: 'rgba(255,255,255,0.3)', marginTop: '2px' }}>
                    {quota.storage_unlimited ? 'Unlimited' : `of ${quota.storage_quota_gb} GB`}
                  </div>
                </div>

                <div>
                  <div style={{ fontSize: '12px', color: 'rgba(255,255,255,0.5)', marginBottom: '4px' }}>
                    Restores This Month
                  </div>
                  <div style={{ fontSize: '28px', fontWeight: 'bold', color: quota.restores_unlimited || quota.restores_remaining > 0 ? '#10b981' : '#ef4444' }}>
                    {quota.restores_this_month} {!quota.restores_unlimited && `/ ${quota.max_restores_month}`}
                  </div>
                  <div style={{ fontSize: '10px', color: 'rgba(255,255,255,0.3)', marginTop: '2px' }}>
                    {quota.restores_unlimited ? 'Unlimited' : `${quota.restores_remaining} remaining`}
                  </div>
                </div>

                <div>
                  <div style={{ fontSize: '12px', color: 'rgba(255,255,255,0.5)', marginBottom: '4px' }}>
                    Total Backups
                  </div>
                  <div style={{ fontSize: '28px', fontWeight: 'bold', color: '#a855f7' }}>
                    {quota.total_backups}
                  </div>
                </div>
              </div>
            </motion.div>
          )}

          {/* Backups Table */}
          {backups.length === 0 ? (
            <motion.div
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: 0.2 }}
              style={{
                background: 'rgba(255,255,255,0.05)',
                borderRadius: '12px',
                padding: '60px 20px',
                textAlign: 'center',
                border: '1px solid rgba(255,255,255,0.1)',
              }}
            >
              <div style={{ fontSize: '48px', marginBottom: '16px' }}>💾</div>
              <div style={{ fontSize: '18px', fontWeight: 'bold', color: 'white', marginBottom: '8px' }}>
                No backups found
              </div>
              <div style={{ fontSize: '14px', color: 'rgba(255,255,255,0.6)' }}>
                Create your first backup from the server management page
              </div>
            </motion.div>
          ) : (
            <motion.div
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
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
                    <th style={{ textAlign: 'left', padding: '16px', fontSize: '12px', fontWeight: '600', color: 'rgba(255,255,255,0.6)', textTransform: 'uppercase' }}>
                      Server
                    </th>
                    <th style={{ textAlign: 'left', padding: '16px', fontSize: '12px', fontWeight: '600', color: 'rgba(255,255,255,0.6)', textTransform: 'uppercase' }}>
                      Type
                    </th>
                    <th style={{ textAlign: 'left', padding: '16px', fontSize: '12px', fontWeight: '600', color: 'rgba(255,255,255,0.6)', textTransform: 'uppercase' }}>
                      Size
                    </th>
                    <th style={{ textAlign: 'left', padding: '16px', fontSize: '12px', fontWeight: '600', color: 'rgba(255,255,255,0.6)', textTransform: 'uppercase' }}>
                      Compression
                    </th>
                    <th style={{ textAlign: 'left', padding: '16px', fontSize: '12px', fontWeight: '600', color: 'rgba(255,255,255,0.6)', textTransform: 'uppercase' }}>
                      Created
                    </th>
                    <th style={{ textAlign: 'left', padding: '16px', fontSize: '12px', fontWeight: '600', color: 'rgba(255,255,255,0.6)', textTransform: 'uppercase' }}>
                      Restores
                    </th>
                    <th style={{ textAlign: 'right', padding: '16px', fontSize: '12px', fontWeight: '600', color: 'rgba(255,255,255,0.6)', textTransform: 'uppercase' }}>
                      Actions
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {backups.map((backup, index) => {
                    const typeInfo = getTypeLabel(backup.Type);
                    return (
                      <motion.tr
                        key={backup.ID}
                        initial={{ opacity: 0, x: -20 }}
                        animate={{ opacity: 1, x: 0 }}
                        transition={{ delay: index * 0.05 }}
                        style={{ borderBottom: '1px solid rgba(255,255,255,0.05)' }}
                      >
                        <td style={{ padding: '16px' }}>
                          <div style={{ fontSize: '14px', color: 'white', fontWeight: '600' }}>
                            {backup.ServerName}
                          </div>
                          <div style={{ fontSize: '11px', color: 'rgba(255,255,255,0.4)', marginTop: '2px' }}>
                            {backup.MinecraftVersion} • {backup.ServerType}
                          </div>
                        </td>
                        <td style={{ padding: '16px' }}>
                          <div style={{
                            display: 'inline-block',
                            fontSize: '11px',
                            padding: '4px 8px',
                            borderRadius: '4px',
                            background: `${typeInfo.color}20`,
                            border: `1px solid ${typeInfo.color}`,
                            color: typeInfo.color,
                          }}>
                            {typeInfo.label}
                          </div>
                        </td>
                        <td style={{ padding: '16px' }}>
                          <div style={{ fontSize: '14px', color: 'white', fontWeight: '600' }}>
                            {formatFileSize(backup.CompressedSize)}
                          </div>
                          <div style={{ fontSize: '11px', color: 'rgba(255,255,255,0.4)', marginTop: '2px' }}>
                            tar.gz
                          </div>
                        </td>
                        <td style={{ padding: '16px' }}>
                          <div style={{ fontSize: '14px', color: '#10b981', fontWeight: '600' }}>
                            {getCompressionRatio(backup.OriginalSize, backup.CompressedSize)}
                          </div>
                        </td>
                        <td style={{ padding: '16px' }}>
                          <div style={{ fontSize: '14px', color: 'rgba(255,255,255,0.7)' }}>
                            {formatDate(backup.CreatedAt)}
                          </div>
                        </td>
                        <td style={{ padding: '16px' }}>
                          <div style={{ fontSize: '14px', color: 'white' }}>
                            {backup.RestoredCount}×
                          </div>
                        </td>
                        <td style={{ padding: '16px', textAlign: 'right' }}>
                          <button
                            onClick={() => handleRestore(backup.ID)}
                            disabled={restoring === backup.ID || backup.Status !== 'completed'}
                            style={{
                              padding: '8px 16px',
                              borderRadius: '6px',
                              background: backup.Status === 'completed' ? '#3b82f6' : 'rgba(255,255,255,0.1)',
                              border: 'none',
                              color: 'white',
                              fontSize: '13px',
                              fontWeight: '600',
                              cursor: backup.Status === 'completed' ? 'pointer' : 'not-allowed',
                              opacity: restoring === backup.ID ? 0.5 : 1,
                            }}
                          >
                            {restoring === backup.ID ? 'Restoring...' : 'Restore'}
                          </button>
                        </td>
                      </motion.tr>
                    );
                  })}
                </tbody>
              </table>
            </motion.div>
          )}

        </div>
      </div>
    </div>
  );
};
