import { BrowserRouter as Router, Routes, Route, Link, useLocation } from 'react-router-dom';
import { Dashboard } from './components/dashboard/Dashboard';
import { ArchivedServersPage } from './pages/ArchivedServersPage';

const Navigation = () => {
  const location = useLocation();

  return (
    <div style={{
      position: 'fixed',
      top: 20,
      right: 20,
      zIndex: 1000,
      display: 'flex',
      gap: '12px',
    }}>
      <Link
        to="/"
        style={{
          padding: '10px 20px',
          borderRadius: '8px',
          background: location.pathname === '/' ? 'rgba(59, 130, 246, 0.3)' : 'rgba(255,255,255,0.1)',
          border: location.pathname === '/' ? '1px solid #3b82f6' : '1px solid rgba(255,255,255,0.2)',
          color: 'white',
          textDecoration: 'none',
          fontSize: '14px',
          fontWeight: '600',
          transition: 'all 0.2s',
        }}
      >
        🏠 Live Fleet
      </Link>
      <Link
        to="/archived"
        style={{
          padding: '10px 20px',
          borderRadius: '8px',
          background: location.pathname === '/archived' ? 'rgba(59, 130, 246, 0.3)' : 'rgba(255,255,255,0.1)',
          border: location.pathname === '/archived' ? '1px solid #3b82f6' : '1px solid rgba(255,255,255,0.2)',
          color: 'white',
          textDecoration: 'none',
          fontSize: '14px',
          fontWeight: '600',
          transition: 'all 0.2s',
        }}
      >
        📦 Archived Servers
      </Link>
    </div>
  );
};

function App() {
  return (
    <Router>
      <Navigation />
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/archived" element={<ArchivedServersPage />} />
      </Routes>
    </Router>
  );
}

export default App;
