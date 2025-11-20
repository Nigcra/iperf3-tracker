import React, { useState, useEffect } from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter as Router, Routes, Route, Link, useLocation, Navigate } from 'react-router-dom';
// @ts-ignore - FiShare2 exists at runtime but not in type definitions
import { FiHome, FiActivity, FiServer, FiMenu, FiX, FiUsers, FiShare2 } from 'react-icons/fi';
import './index.css';
import Dashboard from './components/Dashboard';
import ServerManager from './components/ServerManager';
import TestRunner from './components/TestRunner';
import Login from './components/Login';
import AdminPanel from './components/AdminPanel';
import PeeringMap from './components/PeeringMap';
import Logo from './components/Logo';
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
            {sidebarOpen ? (
              <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                <div style={{ flexShrink: 0 }}>
                  <Logo size={40} showText={false} variant="icon" />
                </div>
                <div style={{ 
                  display: 'flex', 
                  flexDirection: 'column',
                  gap: '0px'
                }}>
                  <div style={{
                    fontSize: '20px',
                    fontWeight: 600,
                    color: 'white',
                    letterSpacing: '1px',
                    lineHeight: 1.2,
                    whiteSpace: 'nowrap',
                    fontFamily: '"Inter", "SF Pro Display", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
                    textTransform: 'uppercase'
                  }}>
                    iPerf3 Tracker
                  </div>
                </div>
              </div>
            ) : (
              <Logo size={36} showText={false} variant="icon" />
            )}
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
          <Link to="/peering" className={`nav-item ${isActive('/peering')}`}>
            <FiShare2 className="nav-icon" size={20} />
            {sidebarOpen && <span>Peering Map</span>}
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
          <Route path="/peering" element={<PeeringMap />} />
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
