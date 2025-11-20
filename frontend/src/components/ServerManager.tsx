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
    auto_trace_enabled: false,
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
      auto_trace_enabled: server.auto_trace_enabled,
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
          <button className="btn btn-primary" onClick={() => setShowForm(true)}>
            Add Server
          </button>
        )}
      </div>

      {/* Modal Overlay for Add/Edit Form */}
      {showForm && (
        <div className="modal-overlay" onClick={handleCancel}>
          <div className="modal-content large-modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h2>{editingServer ? 'Edit Server' : 'Add New Server'}</h2>
              <button className="modal-close" onClick={handleCancel}>×</button>
            </div>
            <div className="modal-body">
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

                  <div className="form-group checkbox">
                    <label>
                      <input
                        type="checkbox"
                        checked={formData.auto_trace_enabled}
                        onChange={(e) => setFormData({ ...formData, auto_trace_enabled: e.target.checked })}
                        disabled={!formData.schedule_enabled}
                      />
                      Auto Traceroute (when scheduled)
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
          </div>
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
            <div className="servers-table-container">
              <table className="servers-table">
                <thead>
                  <tr>
                    <th>Status</th>
                    <th>Name</th>
                    <th>Host</th>
                    <th>Protocol</th>
                    <th>Direction</th>
                    <th>Duration</th>
                    <th>Streams</th>
                    <th>Schedule</th>
                    <th>Auto Trace</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {servers.map((server) => (
                    <tr key={server.id} className={!server.enabled ? 'disabled-row' : ''}>
                      <td>
                        <div className="status-cell">
                          <span className={`status-indicator ${server.enabled ? 'enabled' : 'disabled'}`}></span>
                          {server.enabled ? 'Enabled' : 'Disabled'}
                        </div>
                      </td>
                      <td>
                        <strong className="server-name">{server.name}</strong>
                        {server.description && (
                          <div className="server-description">{server.description}</div>
                        )}
                      </td>
                      <td>
                        <code className="host-code">{server.host}:{server.port}</code>
                      </td>
                      <td>
                        <span className="badge protocol-badge">
                          {server.default_protocol.toUpperCase()}
                        </span>
                      </td>
                      <td>
                        <span className="direction-text">{server.default_direction}</span>
                      </td>
                      <td className="text-center">{server.default_duration}s</td>
                      <td className="text-center">{server.default_parallel}</td>
                      <td>
                        <div className="schedule-cell">
                          {server.schedule_enabled ? (
                            <>
                              <span className="checkmark">✓</span>
                              <span className="schedule-interval">Every {server.schedule_interval_minutes}m</span>
                            </>
                          ) : (
                            <span className="disabled-text">—</span>
                          )}
                        </div>
                      </td>
                      <td className="text-center">
                        {server.auto_trace_enabled ? (
                          <span className="checkmark">✓</span>
                        ) : (
                          <span className="disabled-text">—</span>
                        )}
                      </td>
                      <td>
                        <div className="table-actions">
                          <button 
                            className="btn-icon btn-edit" 
                            onClick={() => handleEdit(server)}
                            title="Edit"
                          >
                            ✏️
                          </button>
                          <button 
                            className="btn-icon btn-delete" 
                            onClick={() => handleDelete(server.id)}
                            title="Delete"
                          >
                            🗑️
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </div>
  );
};

export default ServerManager;
