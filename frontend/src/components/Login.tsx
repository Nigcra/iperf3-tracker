import React, { useState } from 'react';
import { login, initAdmin } from '../services/api';
import Logo from './Logo';
import './Login.css';

interface LoginProps {
  onLoginSuccess: () => void;
}

const Login: React.FC<LoginProps> = ({ onLoginSuccess }) => {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [showInitAdmin, setShowInitAdmin] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    try {
      const response = await login({ username, password });
      localStorage.setItem('token', response.access_token);
      localStorage.setItem('user', JSON.stringify(response.user));
      onLoginSuccess();
    } catch (err: any) {
      if (err.response?.status === 401) {
        setError('Invalid username or password');
      } else if (err.response?.status === 400) {
        setError(err.response.data.detail || 'Login failed');
      } else {
        setError('An error occurred. Please try again.');
        // Show init admin option if no users exist
        if (err.response?.status === 404 || err.message.includes('Network Error')) {
          setShowInitAdmin(true);
        }
      }
    } finally {
      setLoading(false);
    }
  };

  const handleInitAdmin = async () => {
    if (!window.confirm('This will create a default admin user (admin/admin123). Continue?')) {
      return;
    }

    try {
      setLoading(true);
      const result = await initAdmin();
      alert(`${result.message}\nUsername: ${result.username}\nPassword: ${result.password}\n\n${result.warning}`);
      setUsername('admin');
      setPassword('admin123');
      setShowInitAdmin(false);
    } catch (err: any) {
      setError(err.response?.data?.detail || 'Failed to initialize admin user');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="login-container">
      <div className="login-card">
        <div className="login-header">
          <Logo size={126} showText={true} />
          <p style={{ marginTop: '20px', fontSize: '16px', color: 'rgba(255,255,255,0.8)' }}>
            Network Performance Monitoring
          </p>
        </div>

        <form onSubmit={handleSubmit} className="login-form">
          {error && <div className="error-message">{error}</div>}

          <div className="form-group">
            <label htmlFor="username">Username</label>
            <input
              id="username"
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="Enter your username"
              required
              autoFocus
            />
          </div>

          <div className="form-group">
            <label htmlFor="password">Password</label>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Enter your password"
              required
            />
          </div>

          <button
            type="submit"
            className="btn btn-primary btn-block"
            disabled={loading}
          >
            {loading ? 'Logging in...' : 'Login'}
          </button>

          {showInitAdmin && (
            <button
              type="button"
              className="btn btn-secondary btn-block"
              onClick={handleInitAdmin}
              disabled={loading}
            >
              Initialize Admin User
            </button>
          )}
        </form>
      </div>
    </div>
  );
};

export default Login;
