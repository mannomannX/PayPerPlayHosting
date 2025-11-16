import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import { Dashboard } from './components/dashboard/Dashboard';
import { ArchivedServersPage } from './pages/ArchivedServersPage';

function App() {
  return (
    <Router>
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/archived" element={<ArchivedServersPage />} />
      </Routes>
    </Router>
  );
}

export default App;
