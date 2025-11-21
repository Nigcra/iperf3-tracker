import React, { useEffect, useState } from 'react';
import { getServers, runTest, getTests, Server, Test, ProtocolType, TestDirection, TestStatus } from '../services/api';
import LiveTestDisplay from './LiveTestDisplay';
import './TestRunner.css';

const TestRunner: React.FC = () => {
  const [servers, setServers] = useState<Server[]>([]);
  const [selectedServerId, setSelectedServerId] = useState<number | null>(null);
  const [duration, setDuration] = useState<number>(10);
  const [parallelStreams, setParallelStreams] = useState<number>(1);
  const [protocol, setProtocol] = useState<ProtocolType>(ProtocolType.TCP);
  const [direction, setDirection] = useState<TestDirection>(TestDirection.DOWNLOAD);
  const [isRunning, setIsRunning] = useState(false);
  const [runningTestId, setRunningTestId] = useState<number | null>(null);
  const [recentTests, setRecentTests] = useState<Test[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedTest, setSelectedTest] = useState<Test | null>(null);

  const loadData = async () => {
    try {
      setLoading(true);
      const [serversData, testsData] = await Promise.all([
        getServers(),
        getTests({ limit: 10 })
      ]);
      setServers(serversData.filter(s => s.enabled));
      setRecentTests(testsData);
      
      if (serversData.length > 0 && !selectedServerId) {
        setSelectedServerId(serversData[0].id);
      }
    } catch (error) {
      console.error('Error loading data:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleRunTest = async () => {
    if (!selectedServerId) {
      alert('Please select a server');
      return;
    }

    try {
      setIsRunning(true);
      
      // Start the test and get the test ID immediately
      const result = await runTest({
        server_id: selectedServerId,
        protocol,
        direction,
        duration,
        parallel_streams: parallelStreams
      });

      // Show live test display with the test ID
      // The test will run in the background on the server
      setRunningTestId(result.id);
      
    } catch (error: any) {
      console.error('Error running test:', error);
      alert(`Error running test: ${error.response?.data?.detail || error.message}`);
      setIsRunning(false);
    }
  };

  const handleTestComplete = async () => {
    setIsRunning(false);
    setRunningTestId(null);
    await loadData(); // Refresh test list
  };

  const handleServerChange = (serverId: number) => {
    setSelectedServerId(serverId);
    const server = servers.find(s => s.id === serverId);
    if (server) {
      setDuration(server.default_duration);
      setParallelStreams(server.default_parallel);
      setProtocol(server.default_protocol);
      setDirection(server.default_direction);
    }
  };

  const getStatusBadge = (status: TestStatus) => {
    const statusColors: Record<TestStatus, string> = {
      [TestStatus.PENDING]: 'status-pending',
      [TestStatus.RUNNING]: 'status-running',
      [TestStatus.COMPLETED]: 'status-completed',
      [TestStatus.FAILED]: 'status-failed'
    };
    return statusColors[status] || '';
  };

  if (loading) {
    return <div className="loading">Loading...</div>;
  }

  if (servers.length === 0) {
    return (
      <div className="test-runner">
        <h1>Run Test</h1>
        <div className="empty-state">
          No servers available. Please add a server first.
        </div>
      </div>
    );
  }

  return (
    <div className="test-runner">
      <h1>Run iperf3 Test</h1>

      {/* Live Test Display */}
      {isRunning && runningTestId && (
        <LiveTestDisplay 
          testId={runningTestId} 
          onComplete={handleTestComplete}
        />
      )}

      <div className="card">
        <h2>Test Configuration</h2>
        <div className="test-form">
          <div className="form-group">
            <label>Server *</label>
            <select
              value={selectedServerId || ''}
              onChange={(e) => handleServerChange(parseInt(e.target.value))}
            >
              {servers.map((server) => (
                <option key={server.id} value={server.id}>
                  {server.name} ({server.host}:{server.port})
                </option>
              ))}
            </select>
          </div>

          <div className="form-row">
            <div className="form-group">
              <label>Duration (seconds)</label>
              <input
                type="number"
                value={duration}
                onChange={(e) => setDuration(parseInt(e.target.value))}
                min="1"
                max="300"
              />
            </div>

            <div className="form-group">
              <label>Parallel Streams (-P)</label>
              <input
                type="number"
                value={parallelStreams}
                onChange={(e) => setParallelStreams(parseInt(e.target.value))}
                min="1"
                max="128"
              />
              <small>Number of simultaneous connections</small>
              {parallelStreams > 4 && (
                <div className="warning-message" style={{ 
                  marginTop: '8px', 
                  padding: '8px', 
                  backgroundColor: '#fff3cd', 
                  border: '1px solid #ffc107', 
                  borderRadius: '4px',
                  color: '#856404',
                  fontSize: '13px'
                }}>
                  ⚠️ Most public servers only support up to 4 parallel streams
                </div>
              )}
            </div>
          </div>

          <div className="form-row">
            <div className="form-group">
              <label>Protocol</label>
              <select
                value={protocol}
                onChange={(e) => setProtocol(e.target.value as ProtocolType)}
              >
                <option value={ProtocolType.TCP}>TCP</option>
                <option value={ProtocolType.UDP}>UDP</option>
              </select>
            </div>

            <div className="form-group">
              <label>Direction</label>
              <select
                value={direction}
                onChange={(e) => setDirection(e.target.value as TestDirection)}
              >
                <option value={TestDirection.DOWNLOAD}>Download (Server → Client)</option>
                <option value={TestDirection.UPLOAD}>Upload (Client → Server)</option>
                <option value={TestDirection.BIDIRECTIONAL}>Bidirectional</option>
              </select>
            </div>
          </div>

          <div className="form-actions">
            <button
              className="btn btn-primary"
              onClick={handleRunTest}
              disabled={isRunning || !selectedServerId}
            >
              {isRunning ? '⏳ Running Test...' : '▶️ Run Test'}
            </button>
            <button
              className="btn btn-success"
              onClick={loadData}
              disabled={isRunning}
            >
              🔄 Refresh
            </button>
          </div>
        </div>
      </div>

      <div className="card">
        <h2>Recent Tests</h2>
        {recentTests.length === 0 ? (
          <div className="empty-state">No tests yet. Run your first test!</div>
        ) : (
          <div className="tests-table">
            <table>
              <thead>
                <tr>
                  <th>Time</th>
                  <th>Server</th>
                  <th>Protocol</th>
                  <th>Direction</th>
                  <th>Parallel</th>
                  <th>Download</th>
                  <th>Upload</th>
                  <th>Status</th>
                </tr>
              </thead>
              <tbody>
                {recentTests.map((test) => {
                  const server = servers.find(s => s.id === test.server_id);
                  return (
                    <tr 
                      key={test.id} 
                      onClick={() => setSelectedTest(test)}
                      style={{ cursor: 'pointer' }}
                      title="Click for details"
                    >
                      <td>{new Date(test.created_at.endsWith('Z') ? test.created_at : test.created_at + 'Z').toLocaleString()}</td>
                      <td>{server?.name || `Server ${test.server_id}`}</td>
                      <td>{test.protocol.toUpperCase()}</td>
                      <td>{test.direction}</td>
                      <td>{test.parallel_streams}</td>
                      <td>
                        {test.download_bandwidth_mbps !== null && test.download_bandwidth_mbps !== undefined
                          ? `${test.download_bandwidth_mbps.toFixed(2)} Mbps`
                          : '-'}
                      </td>
                      <td>
                        {test.upload_bandwidth_mbps !== null && test.upload_bandwidth_mbps !== undefined
                          ? `${test.upload_bandwidth_mbps.toFixed(2)} Mbps`
                          : '-'}
                      </td>
                      <td>
                        <span className={`status-badge ${getStatusBadge(test.status)}`}>
                          {test.status}
                        </span>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Test Details Modal */}
      {selectedTest && (
        <div className="modal-overlay" onClick={() => setSelectedTest(null)}>
          <div className="modal-content" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h2>Test Details</h2>
              <button className="modal-close" onClick={() => setSelectedTest(null)}>×</button>
            </div>
            <div className="modal-body">
              <div className="detail-section">
                <h3>Test Information</h3>
                <div className="detail-grid">
                  <div className="detail-item">
                    <span className="detail-label">Status:</span>
                    <span className={`status-badge ${getStatusBadge(selectedTest.status)}`}>
                      {selectedTest.status}
                    </span>
                  </div>
                  <div className="detail-item">
                    <span className="detail-label">Created:</span>
                    <span>{new Date(selectedTest.created_at.endsWith('Z') ? selectedTest.created_at : selectedTest.created_at + 'Z').toLocaleString()}</span>
                  </div>
                  {selectedTest.started_at && (
                    <div className="detail-item">
                      <span className="detail-label">Started:</span>
                      <span>{new Date(selectedTest.started_at.endsWith('Z') ? selectedTest.started_at : selectedTest.started_at + 'Z').toLocaleString()}</span>
                    </div>
                  )}
                  {selectedTest.completed_at && (
                    <div className="detail-item">
                      <span className="detail-label">Completed:</span>
                      <span>{new Date(selectedTest.completed_at.endsWith('Z') ? selectedTest.completed_at : selectedTest.completed_at + 'Z').toLocaleString()}</span>
                    </div>
                  )}
                </div>
              </div>

              <div className="detail-section">
                <h3>Configuration</h3>
                <div className="detail-grid">
                  <div className="detail-item">
                    <span className="detail-label">Protocol:</span>
                    <span>{selectedTest.protocol.toUpperCase()}</span>
                  </div>
                  <div className="detail-item">
                    <span className="detail-label">Direction:</span>
                    <span>{selectedTest.direction}</span>
                  </div>
                  <div className="detail-item">
                    <span className="detail-label">Duration:</span>
                    <span>{selectedTest.duration}s</span>
                  </div>
                  <div className="detail-item">
                    <span className="detail-label">Parallel Streams:</span>
                    <span>{selectedTest.parallel_streams}</span>
                  </div>
                </div>
              </div>

              {selectedTest.status === TestStatus.COMPLETED && (
                <div className="detail-section">
                  <h3>Results</h3>
                  <div className="detail-grid">
                    {selectedTest.download_bandwidth_mbps !== null && selectedTest.download_bandwidth_mbps !== undefined && (
                      <>
                        <div className="detail-item">
                          <span className="detail-label">Download Bandwidth:</span>
                          <span className="highlight">{selectedTest.download_bandwidth_mbps.toFixed(2)} Mbps</span>
                        </div>
                        <div className="detail-item">
                          <span className="detail-label">Download Bytes:</span>
                          <span>{selectedTest.download_bytes?.toLocaleString() || 'N/A'}</span>
                        </div>
                      </>
                    )}
                    {selectedTest.upload_bandwidth_mbps !== null && selectedTest.upload_bandwidth_mbps !== undefined && (
                      <>
                        <div className="detail-item">
                          <span className="detail-label">Upload Bandwidth:</span>
                          <span className="highlight">{selectedTest.upload_bandwidth_mbps.toFixed(2)} Mbps</span>
                        </div>
                        <div className="detail-item">
                          <span className="detail-label">Upload Bytes:</span>
                          <span>{selectedTest.upload_bytes?.toLocaleString() || 'N/A'}</span>
                        </div>
                      </>
                    )}
                    {selectedTest.download_jitter_ms !== null && selectedTest.download_jitter_ms !== undefined && (
                      <div className="detail-item">
                        <span className="detail-label">Jitter:</span>
                        <span>{selectedTest.download_jitter_ms.toFixed(2)} ms</span>
                      </div>
                    )}
                    {selectedTest.download_packet_loss_percent !== null && selectedTest.download_packet_loss_percent !== undefined && (
                      <div className="detail-item">
                        <span className="detail-label">Packet Loss:</span>
                        <span>{selectedTest.download_packet_loss_percent.toFixed(2)}%</span>
                      </div>
                    )}
                    {selectedTest.retransmits !== null && selectedTest.retransmits !== undefined && (
                      <div className="detail-item">
                        <span className="detail-label">Retransmits:</span>
                        <span>{selectedTest.retransmits}</span>
                      </div>
                    )}
                    {selectedTest.cpu_percent !== null && selectedTest.cpu_percent !== undefined && (
                      <div className="detail-item">
                        <span className="detail-label">CPU Usage:</span>
                        <span>{selectedTest.cpu_percent.toFixed(2)}%</span>
                      </div>
                    )}
                  </div>
                </div>
              )}

              {selectedTest.error_message && (
                <div className="detail-section error-section">
                  <h3>Error Details</h3>
                  <div className="error-message">
                    {selectedTest.error_message}
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default TestRunner;
