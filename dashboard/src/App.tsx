import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import { Dashboard } from './components/dashboard/Dashboard';
import { ArchivedServersPage } from './pages/ArchivedServersPage';
import { BackupsPage } from './pages/BackupsPage';

function App() {
  return (
    <Router>
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/archived" element={<ArchivedServersPage />} />
        <Route path="/backups" element={<BackupsPage />} />
      </Routes>
    </Router>
  );
}

export default App;
