import { Link, useLocation } from 'react-router-dom';

export const PageNavigation = () => {
  const location = useLocation();

  return (
    <div style={{
      display: 'flex',
      gap: '8px',
      alignItems: 'center',
    }}>
      <Link
        to="/"
        style={{
          padding: '8px 16px',
          borderRadius: '6px',
          background: location.pathname === '/'
            ? 'rgba(59, 130, 246, 0.2)'
            : 'rgba(255,255,255,0.05)',
          border: location.pathname === '/'
            ? '1px solid #3b82f6'
            : '1px solid rgba(255,255,255,0.1)',
          color: location.pathname === '/' ? '#60a5fa' : '#94a3b8',
          textDecoration: 'none',
          fontSize: '13px',
          fontWeight: '600',
          transition: 'all 0.2s',
          display: 'flex',
          alignItems: 'center',
          gap: '6px',
        }}
      >
        <span>🏠</span>
        <span>Live Fleet</span>
      </Link>
      <Link
        to="/archived"
        style={{
          padding: '8px 16px',
          borderRadius: '6px',
          background: location.pathname === '/archived'
            ? 'rgba(59, 130, 246, 0.2)'
            : 'rgba(255,255,255,0.05)',
          border: location.pathname === '/archived'
            ? '1px solid #3b82f6'
            : '1px solid rgba(255,255,255,0.1)',
          color: location.pathname === '/archived' ? '#60a5fa' : '#94a3b8',
          textDecoration: 'none',
          fontSize: '13px',
          fontWeight: '600',
          transition: 'all 0.2s',
          display: 'flex',
          alignItems: 'center',
          gap: '6px',
        }}
      >
        <span>📦</span>
        <span>Archived</span>
      </Link>
      <Link
        to="/backups"
        style={{
          padding: '8px 16px',
          borderRadius: '6px',
          background: location.pathname === '/backups'
            ? 'rgba(59, 130, 246, 0.2)'
            : 'rgba(255,255,255,0.05)',
          border: location.pathname === '/backups'
            ? '1px solid #3b82f6'
            : '1px solid rgba(255,255,255,0.1)',
          color: location.pathname === '/backups' ? '#60a5fa' : '#94a3b8',
          textDecoration: 'none',
          fontSize: '13px',
          fontWeight: '600',
          transition: 'all 0.2s',
          display: 'flex',
          alignItems: 'center',
          gap: '6px',
        }}
      >
        <span>💾</span>
        <span>Backups</span>
      </Link>
    </div>
  );
};
