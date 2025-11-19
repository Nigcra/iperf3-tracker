import React, { useEffect, useState } from 'react';
import { getServers, createServer, updateServer, deleteServer, Server, ServerCreate, ProtocolType, TestDirection } from '../services/api';
import './ServerManager.css';

const ServerManager: React.FC = () => {
  const [servers, setServers] = useState<Server[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [editingServer, setEditingServer] = useState<Server | null>(null);
  const [loading, setLoading] = useState(true);

  const defaultFormData: ServerCreate = {
    name: '',
    host: '',
    port: 5201,
    description: '',
    enabled: true,
    default_duration: 10,
    default_parallel: 1,
    default_protocol: ProtocolType.TCP,
    default_direction: TestDirection.DOWNLOAD,
    schedule_enabled: false,
    schedule_interval_minutes: 30,
  };

  const [formData, setFormData] = useState<ServerCreate>(defaultFormData);

  useEffect(() => {
    loadServers();
  }, []);

  const loadServers = async () => {
    try {
      setLoading(true);
      const data = await getServers();
      setServers(data);
    } catch (error) {
      console.error('Error loading servers:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      if (editingServer) {
        await updateServer(editingServer.id, formData);
      } else {
        await createServer(formData);
      }
      setShowForm(false);
      setEditingServer(null);
      setFormData(defaultFormData);
      await loadServers();
    } catch (error) {
      console.error('Error saving server:', error);
      alert('Error saving server. Please check the console for details.');
    }
  };

  const handleEdit = (server: Server) => {
    setEditingServer(server);
    setFormData({
      name: server.name,
      host: server.host,
      port: server.port,
      description: server.description || '',
      enabled: server.enabled,
      default_duration: server.default_duration,
      default_parallel: server.default_parallel,
      default_protocol: server.default_protocol,
      default_direction: server.default_direction,
      schedule_enabled: server.schedule_enabled,
      schedule_interval_minutes: server.schedule_interval_minutes,
    });
    setShowForm(true);
  };

  const handleDelete = async (id: number) => {
    if (window.confirm('Are you sure you want to delete this server?')) {
      try {
        await deleteServer(id);
        await loadServers();
      } catch (error) {
        console.error('Error deleting server:', error);
      }
    }
  };

  const handleCancel = () => {
    setShowForm(false);
    setEditingServer(null);
    setFormData(defaultFormData);
  };

  return (
    <div className="server-manager">
      <div className="header">
        <h1>Server Management</h1>
        {!showForm && (
          <button className="btn-primary" onClick={() => setShowForm(true)}>
            Add Server
          </button>
        )}
      </div>

      {showForm && (
        <div className="form-container">
          <h2>{editingServer ? 'Edit Server' : 'Add New Server'}</h2>
          <form onSubmit={handleSubmit}>
            <div className="form-grid">
              <div className="form-group">
                <label>Name *</label>
                <input
                  type="text"
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  required
                />
              </div>

              <div className="form-group">
                <label>Host *</label>
                <input
                  type="text"
                  value={formData.host}
                  onChange={(e) => setFormData({ ...formData, host: e.target.value })}
                  placeholder="192.168.1.100 or server.example.com"
                  required
                />
              </div>

              <div className="form-group">
                <label>Port</label>
                <input
                  type="number"
                  value={formData.port}
                  onChange={(e) => setFormData({ ...formData, port: parseInt(e.target.value) })}
                  min="1"
                  max="65535"
                />
              </div>

              <div className="form-group">
                <label>Duration (seconds)</label>
                <input
                  type="number"
                  value={formData.default_duration}
                  onChange={(e) => setFormData({ ...formData, default_duration: parseInt(e.target.value) })}
                  min="1"
                  max="300"
                />
              </div>

              <div className="form-group">
                <label>Parallel Streams</label>
                <input
                  type="number"
                  value={formData.default_parallel}
                  onChange={(e) => setFormData({ ...formData, default_parallel: parseInt(e.target.value) })}
                  min="1"
                  max="128"
                />
              </div>

              <div className="form-group">
                <label>Protocol</label>
                <select
                  value={formData.default_protocol}
                  onChange={(e) => setFormData({ ...formData, default_protocol: e.target.value as ProtocolType })}
                >
                  <option value={ProtocolType.TCP}>TCP</option>
                  <option value={ProtocolType.UDP}>UDP</option>
                </select>
              </div>

              <div className="form-group">
                <label>Direction</label>
                <select
                  value={formData.default_direction}
                  onChange={(e) => setFormData({ ...formData, default_direction: e.target.value as TestDirection })}
                >
                  <option value={TestDirection.DOWNLOAD}>Download</option>
                  <option value={TestDirection.UPLOAD}>Upload</option>
                  <option value={TestDirection.BIDIRECTIONAL}>Bidirectional</option>
                </select>
              </div>

              <div className="form-group">
                <label>Schedule Interval (minutes)</label>
                <input
                  type="number"
                  value={formData.schedule_interval_minutes}
                  onChange={(e) => setFormData({ ...formData, schedule_interval_minutes: parseInt(e.target.value) })}
                  min="1"
                />
              </div>

              <div className="form-group full-width">
                <label>Description</label>
                <textarea
                  value={formData.description}
                  onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                  rows={3}
                />
              </div>

              <div className="form-group checkbox">
                <label>
                  <input
                    type="checkbox"
                    checked={formData.enabled}
                    onChange={(e) => setFormData({ ...formData, enabled: e.target.checked })}
                  />
                  Enabled
                </label>
              </div>

              <div className="form-group checkbox">
                <label>
                  <input
                    type="checkbox"
                    checked={formData.schedule_enabled}
                    onChange={(e) => setFormData({ ...formData, schedule_enabled: e.target.checked })}
                  />
                  Scheduled Tests
                </label>
              </div>
            </div>

            <div className="form-actions">
              <button type="submit" className="btn btn-primary">
                {editingServer ? 'Update' : 'Create'}
              </button>
              <button type="button" className="btn btn-secondary" onClick={handleCancel}>
                Cancel
              </button>
            </div>
          </form>
        </div>
      )}

      {loading ? (
        <div className="loading">Loading servers...</div>
      ) : (
        <div className="servers-list">
          {servers.length === 0 ? (
            <div className="empty-state">
              No servers configured. Click "Add Server" to get started.
            </div>
          ) : (
            <div className="servers-grid">
              {servers.map((server) => (
                <div key={server.id} className={`server-item ${!server.enabled ? 'disabled' : ''}`}>
                  <div className="server-header">
                    <h3>{server.name}</h3>
                    <div className="server-badges">
                      {server.schedule_enabled && <span className="badge">Scheduled</span>}
                      {!server.enabled && <span className="badge disabled-badge">Disabled</span>}
                    </div>
                  </div>
                  <div className="server-details">
                    <p><strong>Host:</strong> {server.host}:{server.port}</p>
                    {server.description && <p><strong>Description:</strong> {server.description}</p>}
                    <p><strong>Protocol:</strong> {server.default_protocol.toUpperCase()}</p>
                    <p><strong>Direction:</strong> {server.default_direction}</p>
                    <p><strong>Duration:</strong> {server.default_duration}s</p>
                    <p><strong>Parallel Streams:</strong> {server.default_parallel}</p>
                    {server.schedule_enabled && (
                      <p><strong>Schedule:</strong> Every {server.schedule_interval_minutes} minutes</p>
                    )}
                  </div>
                  <div className="server-actions">
                    <button className="btn btn-primary" onClick={() => handleEdit(server)}>
                      Edit
                    </button>
                    <button className="btn btn-danger" onClick={() => handleDelete(server.id)}>
                      Delete
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
};

export default ServerManager;
