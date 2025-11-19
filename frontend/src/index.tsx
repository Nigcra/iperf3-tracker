import React, { useState, useEffect } from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter as Router, Routes, Route, Link, useLocation, Navigate } from 'react-router-dom';
import { FiHome, FiActivity, FiServer, FiMenu, FiX, FiUsers } from 'react-icons/fi';
import './index.css';
import Dashboard from './components/Dashboard';
import ServerManager from './components/ServerManager';
import TestRunner from './components/TestRunner';
import Login from './components/Login';
import AdminPanel from './components/AdminPanel';
import { User } from './services/api';
import { ThemeProvider, useTheme } from './context/ThemeContext';

function AppContent() {
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [currentUser, setCurrentUser] = useState<User | null>(null);
  const location = useLocation();
  const { theme, toggleTheme } = useTheme();

  useEffect(() => {
    // Check if user is logged in
    const token = localStorage.getItem('token');
    const userStr = localStorage.getItem('user');
    
    if (token && userStr) {
      try {
        const user = JSON.parse(userStr);
        setCurrentUser(user);
        setIsAuthenticated(true);
      } catch (error) {
        console.error('Invalid user data in localStorage');
        handleLogout();
      }
    }
  }, []);

  const handleLoginSuccess = () => {
    const userStr = localStorage.getItem('user');
    if (userStr) {
      setCurrentUser(JSON.parse(userStr));
      setIsAuthenticated(true);
    }
  };

  const handleLogout = () => {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    setCurrentUser(null);
    setIsAuthenticated(false);
  };

  const isActive = (path: string) => {
    return location.pathname === path ? 'active' : '';
  };

  // Show login if not authenticated
  if (!isAuthenticated) {
    return <Login onLoginSuccess={handleLoginSuccess} />;
  }

  return (
    <div className="app">
      {/* Sidebar */}
      <aside className={`sidebar ${sidebarOpen ? 'open' : 'closed'}`}>
        <div className="sidebar-header">
          <div className="logo">
            {sidebarOpen && <span className="logo-text">iperf3 Tracker</span>}
            {!sidebarOpen && <span className="logo-icon">i3</span>}
          </div>
          <button 
            className="toggle-btn" 
            onClick={() => setSidebarOpen(!sidebarOpen)}
            title={sidebarOpen ? 'Collapse sidebar' : 'Expand sidebar'}
          >
            {sidebarOpen ? <FiX size={20} /> : <FiMenu size={20} />}
          </button>
        </div>

        <nav className="sidebar-nav">
          <Link to="/" className={`nav-item ${isActive('/')}`}>
            <FiHome className="nav-icon" size={20} />
            {sidebarOpen && <span>Dashboard</span>}
          </Link>
          <Link to="/tests" className={`nav-item ${isActive('/tests')}`}>
            <FiActivity className="nav-icon" size={20} />
            {sidebarOpen && <span>Run Test</span>}
          </Link>
          <Link to="/servers" className={`nav-item ${isActive('/servers')}`}>
            <FiServer className="nav-icon" size={20} />
            {sidebarOpen && <span>Servers</span>}
          </Link>
          {currentUser?.is_admin && (
            <Link to="/admin" className={`nav-item ${isActive('/admin')}`}>
              <FiUsers className="nav-icon" size={20} />
              {sidebarOpen && <span>Admin</span>}
            </Link>
          )}
        </nav>

        {sidebarOpen && (
          <div className="sidebar-footer">
            <div className="user-info">
              <div className="user-name">{currentUser?.username}</div>
              {currentUser?.is_admin && <div className="user-badge">Admin</div>}
            </div>
            <button 
              className="btn btn-theme-toggle" 
              onClick={toggleTheme}
              title={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
            >
              <span style={{ fontSize: '16px' }}>{theme === 'dark' ? '☀️' : '🌙'}</span>
            </button>
            <button className="btn btn-logout" onClick={handleLogout}>
              Logout
            </button>
            <div className="version">v1.0.0</div>
          </div>
        )}
      </aside>

      {/* Main Content */}
      <main className={`main-content ${sidebarOpen ? '' : 'expanded'}`}>
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/tests" element={<TestRunner />} />
          <Route path="/servers" element={<ServerManager />} />
          <Route 
            path="/admin" 
            element={currentUser?.is_admin ? <AdminPanel /> : <Navigate to="/" />} 
          />
          <Route path="*" element={<Navigate to="/" />} />
        </Routes>
      </main>
    </div>
  );
}

const root = ReactDOM.createRoot(
  document.getElementById('root') as HTMLElement
);

root.render(
  <React.StrictMode>
    <ThemeProvider>
      <Router>
        <AppContent />
      </Router>
    </ThemeProvider>
  </React.StrictMode>
);
