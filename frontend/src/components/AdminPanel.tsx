import React, { useEffect, useState } from 'react';
import { FiUsers, FiTrash2, FiDatabase, FiAlertCircle, FiServer } from 'react-icons/fi';
import {
  getUsers,
  deleteUser,
  register,
  getDatabaseStats,
  cleanupTests,
  cleanupTraces,
  getPublicServers,
  User,
  UserCreate,
  PublicServer
} from '../services/api';
import './AdminPanel.css';

const AdminPanel: React.FC = () => {
  const [users, setUsers] = useState<User[]>([]);
  const [publicServers, setPublicServers] = useState<PublicServer[]>([]);
  const [dbStats, setDbStats] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'users' | 'cleanup' | 'public-servers'>('users');
  
  // User management state
  const [showUserForm, setShowUserForm] = useState(false);
  const [newUser, setNewUser] = useState<UserCreate>({
    username: '',
    email: '',
    password: '',
    is_admin: false
  });

  // Cleanup state
  const [cleanupDays, setCleanupDays] = useState<number>(30);
  const [cleanupTraceDays, setCleanupTraceDays] = useState<number>(30);
  const [cleanupServerId, setCleanupServerId] = useState<string>('');

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    try {
      setLoading(true);
      const [usersData, statsData, serversData] = await Promise.all([
        getUsers(),
        getDatabaseStats(),
        getPublicServers()
      ]);
      setUsers(usersData);
      setDbStats(statsData);
      setPublicServers(serversData);
    } catch (error) {
      console.error('Error loading admin data:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleCreateUser = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await register(newUser);
      alert('User created successfully!');
      setShowUserForm(false);
      setNewUser({ username: '', email: '', password: '', is_admin: false });
      loadData();
    } catch (error: any) {
      alert(error.response?.data?.detail || 'Failed to create user');
    }
  };

  const handleDeleteUser = async (userId: number, username: string) => {
    if (!window.confirm(`Delete user "${username}"?`)) return;
    
    try {
      await deleteUser(userId);
      alert('User deleted successfully!');
      loadData();
    } catch (error: any) {
      alert(error.response?.data?.detail || 'Failed to delete user');
    }
  };

  const handleCleanup = async (all: boolean = false) => {
    const message = all 
      ? 'Delete ALL test data? This cannot be undone!'
      : `Delete tests older than ${cleanupDays} days?`;
    
    if (!window.confirm(message)) return;

    try {
      const params: any = { all };
      if (!all && cleanupDays) params.days = cleanupDays;
      if (cleanupServerId) params.server_id = parseInt(cleanupServerId);
      
      const result = await cleanupTests(params);
      alert(`Successfully deleted ${result.deleted_count} test(s)`);
      loadData();
    } catch (error: any) {
      alert(error.response?.data?.detail || 'Failed to cleanup tests');
    }
  };

  const handleCleanupTraces = async (all: boolean = false) => {
    const message = all 
      ? 'Delete ALL trace data? This cannot be undone!'
      : `Delete traces older than ${cleanupTraceDays} days?`;
    
    if (!window.confirm(message)) return;

    try {
      const params: any = { all };
      if (!all && cleanupTraceDays) params.days = cleanupTraceDays;
      
      const result = await cleanupTraces(params);
      alert(`Successfully deleted ${result.deleted_traces} trace(s) and ${result.deleted_hops} hop(s)`);
      loadData();
    } catch (error: any) {
      alert(error.response?.data?.detail || 'Failed to cleanup traces');
    }
  };

  const copyServerInfo = (server: PublicServer) => {
    const text = `${server.host}:${server.port}`;
    navigator.clipboard.writeText(text);
    alert(`Copied: ${text}`);
  };

  if (loading) {
    return <div className="loading">Loading admin panel...</div>;
  }

  return (
    <div className="admin-panel">
      <div className="admin-header">
        <h1>Admin Panel</h1>
        <p className="subtitle">Manage users, data, and system settings</p>
      </div>

      {/* Tabs */}
      <div className="admin-tabs">
        <button
          className={`tab ${activeTab === 'users' ? 'active' : ''}`}
          onClick={() => setActiveTab('users')}
        >
          <FiUsers /> Users
        </button>
        <button
          className={`tab ${activeTab === 'cleanup' ? 'active' : ''}`}
          onClick={() => setActiveTab('cleanup')}
        >
          <FiDatabase /> Data Cleanup
        </button>
        <button
          className={`tab ${activeTab === 'public-servers' ? 'active' : ''}`}
          onClick={() => setActiveTab('public-servers')}
        >
          <FiServer /> Public Servers
        </button>
      </div>

      {/* Database Stats */}
      {dbStats && (
        <div className="stats-grid">
          <div className="stat-card blue">
            <FiUsers size={24} />
            <div className="stat-value">{dbStats.total_users}</div>
            <div className="stat-label">Total Users</div>
          </div>
          <div className="stat-card purple">
            <FiServer size={24} />
            <div className="stat-value">{dbStats.total_servers}</div>
            <div className="stat-label">Total Servers</div>
          </div>
          <div className="stat-card green">
            <FiDatabase size={24} />
            <div className="stat-value">{dbStats.total_tests}</div>
            <div className="stat-label">Total Tests</div>
          </div>
          <div className="stat-card orange">
            <FiDatabase size={24} />
            <div className="stat-value">{dbStats.total_traces}</div>
            <div className="stat-label">Total Traces</div>
          </div>
        </div>
      )}

      {/* User Management Tab */}
      {activeTab === 'users' && (
        <div className="tab-content">
          <div className="section-header">
            <h2>User Management</h2>
            <button className="btn btn-primary" onClick={() => setShowUserForm(!showUserForm)}>
              {showUserForm ? 'Cancel' : 'Add User'}
            </button>
          </div>

          {showUserForm && (
            <div className="card user-form">
              <h3>Create New User</h3>
              <form onSubmit={handleCreateUser}>
                <div className="form-row">
                  <div className="form-group">
                    <label>Username</label>
                    <input
                      type="text"
                      value={newUser.username}
                      onChange={(e) => setNewUser({ ...newUser, username: e.target.value })}
                      required
                      minLength={3}
                    />
                  </div>
                  <div className="form-group">
                    <label>Email</label>
                    <input
                      type="email"
                      value={newUser.email}
                      onChange={(e) => setNewUser({ ...newUser, email: e.target.value })}
                      required
                    />
                  </div>
                </div>
                <div className="form-row">
                  <div className="form-group">
                    <label>Password</label>
                    <input
                      type="password"
                      value={newUser.password}
                      onChange={(e) => setNewUser({ ...newUser, password: e.target.value })}
                      required
                      minLength={6}
                    />
                  </div>
                  <div className="form-group checkbox">
                    <label>
                      <input
                        type="checkbox"
                        checked={newUser.is_admin}
                        onChange={(e) => setNewUser({ ...newUser, is_admin: e.target.checked })}
                      />
                      Admin privileges
                    </label>
                  </div>
                </div>
                <button type="submit" className="btn btn-primary">Create User</button>
              </form>
            </div>
          )}

          <div className="card">
            <table className="table">
              <thead>
                <tr>
                  <th>Username</th>
                  <th>Email</th>
                  <th>Role</th>
                  <th>Status</th>
                  <th>Last Login</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {users.map((user) => (
                  <tr key={user.id}>
                    <td>{user.username}</td>
                    <td>{user.email}</td>
                    <td>
                      <span className={`badge ${user.is_admin ? 'badge-warning' : 'badge-info'}`}>
                        {user.is_admin ? 'Admin' : 'User'}
                      </span>
                    </td>
                    <td>
                      <span className={`badge ${user.is_active ? 'badge-success' : 'badge-danger'}`}>
                        {user.is_active ? 'Active' : 'Inactive'}
                      </span>
                    </td>
                    <td>{user.last_login ? new Date(user.last_login).toLocaleString() : 'Never'}</td>
                    <td>
                      <button
                        className="btn-icon btn-danger"
                        onClick={() => handleDeleteUser(user.id, user.username)}
                        title="Delete user"
                      >
                        <FiTrash2 />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Cleanup Tab */}
      {activeTab === 'cleanup' && (
        <div className="tab-content">
          <div className="section-header">
            <h2>Data Cleanup</h2>
          </div>

          <div className="card cleanup-section">
            <div className="warning-box">
              <FiAlertCircle size={24} />
              <div>
                <strong>Warning:</strong> Deleted data cannot be recovered.
                Please make sure you have backups if needed.
              </div>
            </div>

            <div className="cleanup-options">
              <h3>Delete Old Tests</h3>
              <div className="form-row">
                <div className="form-group">
                  <label>Delete tests older than (days)</label>
                  <input
                    type="number"
                    value={cleanupDays}
                    onChange={(e) => setCleanupDays(parseInt(e.target.value))}
                    min="1"
                  />
                </div>
                <div className="form-group">
                  <label>Server ID (optional, leave empty for all servers)</label>
                  <input
                    type="text"
                    value={cleanupServerId}
                    onChange={(e) => setCleanupServerId(e.target.value)}
                    placeholder="Leave empty for all servers"
                  />
                </div>
              </div>
              <button className="btn btn-warning" onClick={() => handleCleanup(false)}>
                Delete Old Tests
              </button>
            </div>

            <hr />

            <div className="cleanup-options">
              <h3>Delete Old Traces</h3>
              <p className="info-text">
                Traces: {dbStats?.total_traces || 0} | Hops: {dbStats?.total_hops || 0}
              </p>
              <div className="form-row">
                <div className="form-group">
                  <label>Delete traces older than (days)</label>
                  <input
                    type="number"
                    value={cleanupTraceDays}
                    onChange={(e) => setCleanupTraceDays(parseInt(e.target.value))}
                    min="1"
                  />
                </div>
              </div>
              <button className="btn btn-warning" onClick={() => handleCleanupTraces(false)}>
                Delete Old Traces
              </button>
            </div>

            <hr />

            <div className="cleanup-options">
              <h3>Delete All Tests</h3>
              <p>This will permanently delete ALL test data from the database.</p>
              <button className="btn btn-danger" onClick={() => handleCleanup(true)}>
                <FiTrash2 /> Delete All Tests
              </button>
            </div>

            <hr />

            <div className="cleanup-options">
              <h3>Delete All Traces</h3>
              <p>This will permanently delete ALL trace data from the database.</p>
              <button className="btn btn-danger" onClick={() => handleCleanupTraces(true)}>
                <FiTrash2 /> Delete All Traces
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Public Servers Tab */}
      {activeTab === 'public-servers' && (
        <div className="tab-content">
          <div className="section-header">
            <h2>Public iperf3 Servers</h2>
            <p>Use these public servers to test your connection</p>
          </div>

          <div className="public-servers-grid">
            {publicServers.map((server, index) => (
              <div key={index} className="card public-server-card">
                <div className="server-name">{server.name}</div>
                <div className="server-details">
                  <div className="detail-row">
                    <span className="label">Host:</span>
                    <code>{server.host}</code>
                  </div>
                  <div className="detail-row">
                    <span className="label">Port:</span>
                    <code>{server.port}</code>
                  </div>
                  <div className="detail-row">
                    <span className="label">Location:</span>
                    <span>{server.location}</span>
                  </div>
                  <div className="detail-row">
                    <span className="label">Provider:</span>
                    <span>{server.provider}</span>
                  </div>
                </div>
                <button
                  className="btn btn-sm btn-primary"
                  onClick={() => copyServerInfo(server)}
                >
                  Copy Host:Port
                </button>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};

export default AdminPanel;
