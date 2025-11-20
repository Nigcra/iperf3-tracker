import React, { useEffect, useState } from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, ReferenceLine } from 'recharts';
import { FiServer, FiActivity, FiTrendingUp, FiTrendingDown, FiAlertTriangle } from 'react-icons/fi';
import { getDashboardStats, getServerStats, getTests, getTestLiveStatus, getServers, runTest, DashboardStats, ServerStats, Test, TestLiveStatus, Server, TestDirection } from '../services/api';
import './Dashboard.css';

type TimeRange = '1h' | '24h' | '7d' | '30d';

const Dashboard: React.FC = () => {
  const [dashStats, setDashStats] = useState<DashboardStats | null>(null);
  const [serverStats, setServerStats] = useState<ServerStats[]>([]);
  const [servers, setServers] = useState<Server[]>([]);
  const [recentTests, setRecentTests] = useState<Test[]>([]);
  const [loading, setLoading] = useState(true);
  const [timeRange, setTimeRange] = useState<TimeRange>(() => {
    return (localStorage.getItem('dashboard_timeRange') as TimeRange) || '24h';
  });
  const [downloadThreshold, setDownloadThreshold] = useState<number>(() => {
    const saved = localStorage.getItem('dashboard_downloadThreshold');
    return saved ? Number(saved) : 100;
  });
  const [uploadThreshold, setUploadThreshold] = useState<number>(() => {
    const saved = localStorage.getItem('dashboard_uploadThreshold');
    return saved ? Number(saved) : 50;
  });
  const [runningTests, setRunningTests] = useState<Map<number, TestLiveStatus>>(new Map());

  // Get axis color based on current theme
  const getAxisColor = () => {
    const theme = document.documentElement.getAttribute('data-theme');
    return theme === 'dark' ? '#6b7280' : '#9ca3af';
  };

  const handleRunTest = async (serverId: number, serverInfo: Server | undefined) => {
    if (!serverInfo) return;
    
    try {
      await runTest({
        server_id: serverId,
        protocol: serverInfo.default_protocol,
        direction: serverInfo.default_direction,
        duration: serverInfo.default_duration,
        parallel_streams: serverInfo.default_num_streams
      });
      
      // Refresh stats immediately to show the running test
      checkRunningTests();
    } catch (error) {
      console.error('Error starting test:', error);
      alert('Failed to start test');
    }
  };

  // Helper functions to save settings to localStorage
  const handleTimeRangeChange = (newRange: TimeRange) => {
    setTimeRange(newRange);
    localStorage.setItem('dashboard_timeRange', newRange);
  };

  const handleDownloadThresholdChange = (value: number) => {
    setDownloadThreshold(value);
    localStorage.setItem('dashboard_downloadThreshold', value.toString());
  };

  const handleUploadThresholdChange = (value: number) => {
    setUploadThreshold(value);
    localStorage.setItem('dashboard_uploadThreshold', value.toString());
  };

  useEffect(() => {
    loadStats();
    checkRunningTests(); // Check for running tests immediately
    const statsInterval = setInterval(loadStats, 30000); // Refresh stats every 30 seconds
    const liveInterval = setInterval(checkRunningTests, 3000); // Check running tests every 3 seconds (reduced from 1s)
    
    return () => {
      clearInterval(statsInterval);
      clearInterval(liveInterval);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []); // Only load once on mount, timeRange changes don't trigger reload

  const checkRunningTests = async () => {
    try {
      // Get recent tests to find running and recently failed ones
      const tests = await getTests({ limit: 50 });
      
      const newRunningTests = new Map<number, TestLiveStatus>();
      
      // Fetch live status for each test (will show running or failed if still in active_tests)
      for (const test of tests) {
        try {
          const liveStatus = await getTestLiveStatus(test.id);
          // Show if running OR if failed and still in active tracking (within 10 seconds)
          if (liveStatus.is_running) {
            newRunningTests.set(test.server_id, liveStatus);
          }
        } catch (error) {
          // Silently fail for individual tests
        }
      }
      
      setRunningTests(newRunningTests);
    } catch (error) {
      // Silently fail - no running tests
      setRunningTests(new Map());
    }
  };

  const loadStats = async () => {
    try {
      setLoading(true);
      
      const [dash, serverStatsData, serversData, tests] = await Promise.all([
        getDashboardStats(),
        getServerStats(),
        getServers(),
        getTests({ limit: 200 }) // Get more tests to have enough data for all time ranges
      ]);
      
      // Include completed and failed tests (not pending or running)
      const finishedTests = tests.filter(t => t.status === 'completed' || t.status === 'failed');
      
      setDashStats(dash);
      setServerStats(serverStatsData);
      setServers(serversData);
      setRecentTests(finishedTests);
    } catch (error) {
      console.error('Error loading stats:', error);
    } finally {
      setLoading(false);
    }
  };

  // Calculate time domain to ensure full scope is shown
  const now = Date.now();
  const hoursMap: Record<TimeRange, number> = {
    '1h': 1,
    '24h': 24,
    '7d': 168,
    '30d': 720
  };
  const scopeMs = hoursMap[timeRange] * 60 * 60 * 1000;
  const minTime = now - scopeMs;

  // Prepare chart data - filter by time range AFTER loading all data
  const filteredRecentTests = recentTests.filter(test => {
    // Parse as UTC by appending 'Z' if not present
    const dateStr = test.created_at.endsWith('Z') ? test.created_at : test.created_at + 'Z';
    const testTime = new Date(dateStr).getTime();
    return testTime >= minTime;
  });
  
  // Deduplicate tests by ID (in case same test appears multiple times)
  const uniqueTests = Array.from(
    new Map(filteredRecentTests.map(test => [test.id, test])).values()
  );

  // Build chart data - each test gets its own point, grouped by server
  const chartData = uniqueTests.map((test) => {
    // Parse as UTC by appending 'Z' if not present
    const dateStr = test.created_at.endsWith('Z') ? test.created_at : test.created_at + 'Z';
    const date = new Date(dateStr);
    const timeKey = date.getTime();
    
    // Find server name
    const server = servers.find(s => s.id === test.server_id);
    const serverName = server?.name || `Server ${test.server_id}`;
    
    let name = '';
    if (timeRange === '1h' || timeRange === '24h') {
      name = date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    } else if (timeRange === '7d') {
      name = date.toLocaleDateString([], { month: 'short', day: 'numeric', hour: '2-digit' });
    } else {
      name = date.toLocaleDateString([], { month: 'short', day: 'numeric' });
    }
    
    const downloadSpeed = test.download_bandwidth_mbps || 0;
    const uploadSpeed = test.upload_bandwidth_mbps || 0;
    
    // Check if download/upload was actually tested based on direction
    const downloadTested = test.direction === TestDirection.DOWNLOAD || test.direction === TestDirection.BIDIRECTIONAL;
    const uploadTested = test.direction === TestDirection.UPLOAD || test.direction === TestDirection.BIDIRECTIONAL;
    
    // Create data point with all possible server keys set to null
    const dataPoint: any = {
      name,
      time: timeKey,
      serverName,
      downloadThreshold,
      uploadThreshold,
      downloadBelowThreshold: downloadTested && downloadSpeed > 0 && downloadSpeed < downloadThreshold,
      uploadBelowThreshold: uploadTested && uploadSpeed > 0 && uploadSpeed < uploadThreshold,
    };
    
    // Set this server's data
    dataPoint[`${serverName}_download`] = downloadTested ? downloadSpeed : null;
    dataPoint[`${serverName}_upload`] = uploadTested ? uploadSpeed : null;
    
    return dataPoint;
  }).sort((a, b) => a.time - b.time);
  
  // Always use the full scope for domain
  const actualMinTime = minTime;
  const actualMaxTime = now;
  
  // Generate color palette for servers
  const serverColors = [
    '#3b82f6', // blue
    '#10b981', // green
    '#f59e0b', // amber
    '#8b5cf6', // violet
    '#ef4444', // red
    '#06b6d4', // cyan
    '#ec4899', // pink
    '#f97316', // orange
  ];
  
  // Get unique servers from chart data
  const uniqueServers = Array.from(new Set(chartData.map(d => d.serverName)));
  const serverColorMap = new Map(
    uniqueServers.map((serverName, index) => [
      serverName,
      serverColors[index % serverColors.length]
    ])
  );
  
  // For LineChart, we don't need dummy data - just use empty array if no data
  const displayChartData = chartData;
  
  // Calculate Y-axis domain based on typical network speeds
  // Find the highest speed in the data
  const maxDataValue = chartData.length > 0 ? Math.max(
    ...chartData.flatMap(d => 
      uniqueServers.flatMap(server => [
        d[`${server}_download`] || 0,
        d[`${server}_upload`] || 0
      ])
    ),
    0
  ) : 0;
  
  // Determine appropriate scale based on common network speeds
  // 1000 Mbps (1 Gbit), 2500 Mbps (2.5 Gbit), 10000 Mbps (10 Gbit)
  let yAxisMax = 1000; // Default to 1 Gbit
  if (maxDataValue > 1000) {
    yAxisMax = 2500;
  }
  if (maxDataValue > 2500) {
    yAxisMax = 10000;
  }
  if (maxDataValue > 10000) {
    yAxisMax = Math.ceil(maxDataValue / 10000) * 10000; // Round up to next 10 Gbit
  }
  
  // Count tests below threshold
  const belowThresholdCount = {
    download: chartData.filter(d => d.downloadBelowThreshold).length,
    upload: chartData.filter(d => d.uploadBelowThreshold).length,
    total: chartData.filter(d => d.downloadBelowThreshold || d.uploadBelowThreshold).length,
  };
  
  const belowThresholdPercentage = chartData.length > 0 
    ? ((belowThresholdCount.total / chartData.length) * 100).toFixed(0)
    : 0;

  if (loading && !dashStats) {
    return <div className="loading">Loading dashboard...</div>;
  }

  return (
    <div className="dashboard">
      <div className="dashboard-header">
        <h1>Dashboard</h1>
        <p className="subtitle">Network Performance Overview</p>
      </div>

      {/* Stats Cards */}
      {dashStats && (
        <div className="stats-grid">
          <div className="stat-card blue">
            <div className="stat-icon">
              <FiServer size={28} />
            </div>
            <div className="stat-content">
              <div className="stat-label">Total Servers</div>
              <div className="stat-value">{dashStats.total_servers}</div>
              <div className="stat-detail">{dashStats.active_servers} active</div>
            </div>
          </div>

          <div className="stat-card green">
            <div className="stat-icon">
              <FiActivity size={28} />
            </div>
            <div className="stat-content">
              <div className="stat-label">Total Tests</div>
              <div className="stat-value">{dashStats.total_tests}</div>
              <div className="stat-detail">{dashStats.tests_today} today</div>
            </div>
          </div>

          <div className="stat-card purple">
            <div className="stat-icon">
              <FiTrendingUp size={28} />
            </div>
            <div className="stat-content">
              <div className="stat-label">Avg Download</div>
              <div className="stat-value">
                {dashStats.avg_download_mbps !== null && dashStats.avg_download_mbps !== undefined 
                  ? dashStats.avg_download_mbps.toFixed(1) 
                  : 'N/A'}
              </div>
              <div className="stat-detail">Mbps</div>
            </div>
          </div>

          <div className="stat-card orange">
            <div className="stat-icon">
              <FiTrendingDown size={28} />
            </div>
            <div className="stat-content">
              <div className="stat-label">Avg Upload</div>
              <div className="stat-value">
                {dashStats.avg_upload_mbps !== null && dashStats.avg_upload_mbps !== undefined
                  ? dashStats.avg_upload_mbps.toFixed(1)
                  : 'N/A'}
              </div>
              <div className="stat-detail">Mbps</div>
            </div>
          </div>
          
          {chartData.length > 0 && (
            <div className="stat-card red">
              <div className="stat-icon">
                <FiAlertTriangle size={28} />
              </div>
              <div className="stat-content">
                <div className="stat-label">Below Threshold</div>
                <div className="stat-value">{belowThresholdCount.total}</div>
                <div className="stat-detail">{belowThresholdPercentage}% of recent tests</div>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Charts Controls */}
      <div style={{ 
        display: 'flex', 
        justifyContent: 'space-between', 
        alignItems: 'center', 
        marginBottom: '20px',
        flexWrap: 'wrap',
        gap: '15px'
      }}>
        <div className="threshold-controls">
          <div className="threshold-input-group">
            <label htmlFor="download-threshold">Min Download (Mbps):</label>
            <input 
              id="download-threshold"
              type="number" 
              min="0"
              value={downloadThreshold}
              onChange={(e) => handleDownloadThresholdChange(Number(e.target.value))}
            />
          </div>
          <div className="threshold-input-group">
            <label htmlFor="upload-threshold">Min Upload (Mbps):</label>
            <input 
              id="upload-threshold"
              type="number" 
              min="0"
              value={uploadThreshold}
              onChange={(e) => handleUploadThresholdChange(Number(e.target.value))}
            />
          </div>
        </div>
        <div className="time-range-selector">
          <button 
            className={`time-btn ${timeRange === '1h' ? 'active' : ''}`}
            onClick={() => handleTimeRangeChange('1h')}
          >
            1h
          </button>
          <button 
            className={`time-btn ${timeRange === '24h' ? 'active' : ''}`}
            onClick={() => handleTimeRangeChange('24h')}
          >
            24h
          </button>
          <button 
            className={`time-btn ${timeRange === '7d' ? 'active' : ''}`}
            onClick={() => handleTimeRangeChange('7d')}
          >
            Last Week
          </button>
          <button 
            className={`time-btn ${timeRange === '30d' ? 'active' : ''}`}
            onClick={() => handleTimeRangeChange('30d')}
          >
            Last Month
          </button>
        </div>
      </div>

      {/* Charts */}
      <div className="charts-section">
        {/* Download Chart */}
        <div className="chart-card">
          <div className="chart-header">
            <h2>Download Bandwidth Over Time</h2>
            </div>
            <ResponsiveContainer width="100%" height={300}>
              <LineChart data={displayChartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#e5e7eb" />
                <XAxis 
                  dataKey="time" 
                  type="number"
                  domain={[actualMinTime, actualMaxTime]}
                  scale="time"
                  allowDataOverflow={true}
                  tickFormatter={(timestamp) => {
                    const date = new Date(timestamp);
                    if (timeRange === '1h' || timeRange === '24h') {
                      return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
                    } else if (timeRange === '7d') {
                      return date.toLocaleDateString([], { month: 'short', day: 'numeric' });
                    } else {
                      return date.toLocaleDateString([], { month: 'short', day: 'numeric' });
                    }
                  }}
                  stroke="#6b7280" 
                  fontSize={12} 
                />
                <YAxis 
                  stroke="#6b7280" 
                  fontSize={12}
                  domain={[0, yAxisMax]}
                />
                <Tooltip 
                  contentStyle={{ 
                    backgroundColor: 'white', 
                    border: '1px solid #e5e7eb',
                    borderRadius: '8px',
                    boxShadow: '0 4px 6px rgba(0,0,0,0.1)'
                  }}
                  labelFormatter={(timestamp, payload) => {
                    const date = new Date(timestamp);
                    const timeStr = date.toLocaleString([], { 
                      month: 'short', 
                      day: 'numeric', 
                      hour: '2-digit', 
                      minute: '2-digit' 
                    });
                    // Get server name from payload
                    const serverName = payload && payload[0]?.payload?.serverName;
                    return serverName ? `${timeStr} - ${serverName}` : timeStr;
                  }}
                  formatter={(value: any, name: string) => {
                    if (value === null || value === undefined) return ['N/A', ''];
                    // Extract server name from dataKey
                    const serverName = name.replace('_download', '');
                    return [typeof value === 'number' ? `${value.toFixed(1)} Mbps` : value, serverName];
                  }}
                />
                <ReferenceLine 
                  y={downloadThreshold} 
                  stroke="#ef4444" 
                  strokeDasharray="5 5" 
                  label={{ value: `Min: ${downloadThreshold} Mbps`, position: 'right', fill: '#ef4444', fontSize: 12 }}
                />
                {uniqueServers.map((serverName) => {
                  const color = serverColorMap.get(serverName);
                  // Filter data for this server only and remove null/zero values
                  const serverData = chartData
                    .filter(d => d.serverName === serverName && d[`${serverName}_download`] != null && d[`${serverName}_download`] > 0);
                  return (
                    <Line 
                      key={`${serverName}_download`}
                      data={serverData}
                      type="monotone" 
                      dataKey={`${serverName}_download`}
                      stroke={color}
                      strokeWidth={2}
                      dot={(props: any) => {
                        const { cx, cy, payload } = props;
                        const value = payload[`${serverName}_download`];
                        const isBelowThreshold = value < downloadThreshold;
                        return (
                          <circle
                            cx={cx}
                            cy={cy}
                            r={4}
                            fill={isBelowThreshold ? '#ef4444' : color}
                            stroke={isBelowThreshold ? '#dc2626' : color}
                            strokeWidth={isBelowThreshold ? 2 : 0}
                          />
                        );
                      }}
                      activeDot={{ r: 6 }}
                      name={serverName}
                      connectNulls={false}
                      isAnimationActive={false}
                    />
                  );
                })}
              </LineChart>
            </ResponsiveContainer>
        </div>

        {/* Upload Chart */}
        <div className="chart-card">
          <div className="chart-header">
            <h2>Upload Bandwidth Over Time</h2>
            </div>
            <ResponsiveContainer width="100%" height={300}>
              <LineChart data={displayChartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#e5e7eb" />
                <XAxis 
                  dataKey="time" 
                  type="number"
                  domain={[actualMinTime, actualMaxTime]}
                  scale="time"
                  allowDataOverflow={true}
                  tickFormatter={(timestamp) => {
                    const date = new Date(timestamp);
                    if (timeRange === '1h' || timeRange === '24h') {
                      return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
                    } else if (timeRange === '7d') {
                      return date.toLocaleDateString([], { month: 'short', day: 'numeric' });
                    } else {
                      return date.toLocaleDateString([], { month: 'short', day: 'numeric' });
                    }
                  }}
                  stroke="#6b7280" 
                  fontSize={12} 
                />
                <YAxis 
                  stroke="#6b7280" 
                  fontSize={12}
                  domain={[0, yAxisMax]}
                />
                <Tooltip 
                  contentStyle={{ 
                    backgroundColor: 'white', 
                    border: '1px solid #e5e7eb',
                    borderRadius: '8px',
                    boxShadow: '0 4px 6px rgba(0,0,0,0.1)'
                  }}
                  labelFormatter={(timestamp, payload) => {
                    const date = new Date(timestamp);
                    const timeStr = date.toLocaleString([], { 
                      month: 'short', 
                      day: 'numeric', 
                      hour: '2-digit', 
                      minute: '2-digit' 
                    });
                    // Get server name from payload
                    const serverName = payload && payload[0]?.payload?.serverName;
                    return serverName ? `${timeStr} - ${serverName}` : timeStr;
                  }}
                  formatter={(value: any, name: string) => {
                    if (value === null || value === undefined) return ['N/A', ''];
                    // Extract server name from dataKey
                    const serverName = name.replace('_upload', '');
                    return [typeof value === 'number' ? `${value.toFixed(1)} Mbps` : value, serverName];
                  }}
                />
                <ReferenceLine 
                  y={uploadThreshold} 
                  stroke="#f59e0b" 
                  strokeDasharray="5 5" 
                  label={{ value: `Min: ${uploadThreshold} Mbps`, position: 'right', fill: '#f59e0b', fontSize: 12 }}
                />
                {uniqueServers.map((serverName) => {
                  const color = serverColorMap.get(serverName);
                  // Filter data for this server only and remove null/zero values
                  const serverData = chartData
                    .filter(d => d.serverName === serverName && d[`${serverName}_upload`] != null && d[`${serverName}_upload`] > 0);
                  return (
                    <Line 
                      key={`${serverName}_upload`}
                      data={serverData}
                      type="monotone" 
                      dataKey={`${serverName}_upload`}
                      stroke={color}
                      strokeWidth={2}
                      dot={(props: any) => {
                        const { cx, cy, payload } = props;
                        const value = payload[`${serverName}_upload`];
                        const isBelowThreshold = value < uploadThreshold;
                        return (
                          <circle
                            cx={cx}
                            cy={cy}
                            r={4}
                            fill={isBelowThreshold ? '#ef4444' : color}
                            stroke={isBelowThreshold ? '#dc2626' : color}
                            strokeWidth={isBelowThreshold ? 2 : 0}
                          />
                        );
                      }}
                      activeDot={{ r: 6 }}
                      name={serverName}
                      connectNulls={false}
                      isAnimationActive={false}
                    />
                  );
                })}
              </LineChart>
            </ResponsiveContainer>
        </div>
      </div>

      {/* Shared Legend for both charts */}
      {uniqueServers.length > 0 && (
        <div style={{ 
          display: 'flex', 
          justifyContent: 'center', 
          gap: '20px', 
          flexWrap: 'wrap',
          marginBottom: '30px',
          padding: '15px',
          backgroundColor: 'var(--card-bg)',
          backdropFilter: 'blur(12px)',
          borderRadius: '12px',
          boxShadow: 'var(--shadow-sm)',
          border: '1px solid var(--border-color)'
        }}>
          {uniqueServers.map((serverName) => (
            <div key={serverName} style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <div style={{ 
                width: '16px', 
                height: '16px', 
                borderRadius: '50%', 
                backgroundColor: serverColorMap.get(serverName) 
              }} />
              <span style={{ fontSize: '14px', color: 'var(--text-primary)' }}>{serverName}</span>
            </div>
          ))}
        </div>
      )}

      {/* Server Statistics */}
      <div className="servers-section">
        <h2>Server Performance</h2>
        <div className="servers-grid">
          {serverStats.map((serverStat) => {
            const liveTest = runningTests.get(serverStat.server_id);
            const isRunning = liveTest && liveTest.is_running;
            const serverInfo = servers.find(s => s.id === serverStat.server_id);
            
            return (
              <div key={serverStat.server_id} className={`server-card ${isRunning ? 'running' : ''}`}>
                {/* Progress Bar - positioned absolutely to not affect layout */}
                {isRunning && liveTest.status !== 'failed' && liveTest.status !== 'pending' && (
                  <div 
                    className="progress-bar-overlay"
                    style={{ 
                      position: 'absolute',
                      top: '12px',
                      left: '50%',
                      transform: 'translateX(-50%)',
                      width: '66%',
                      height: '3px',
                      background: 'rgba(0, 0, 0, 0.05)',
                      borderRadius: '12px',
                      zIndex: 1000,
                      pointerEvents: 'none',
                      overflow: 'hidden'
                    }}
                  >
                    <div 
                      style={{ 
                        height: '100%',
                        width: `${liveTest.progress}%`,
                        background: 'linear-gradient(90deg, #3b82f6 0%, #2563eb 100%)',
                        boxShadow: '0 0 8px rgba(59, 130, 246, 0.6)',
                        transition: 'width 0.3s ease'
                      }}
                    />
                  </div>
                )}
                
                <div className="server-header">
                  <h3>{serverStat.server_name}</h3>
                  <div className="server-header-actions">
                    {!isRunning && serverInfo && (
                      <button 
                        className="quick-test-btn"
                        onClick={() => handleRunTest(serverStat.server_id, serverInfo)}
                        title="Run quick test"
                      >
                        ▶
                      </button>
                    )}
                    {isRunning && (
                      <div className={`testing-indicator ${
                        liveTest.status === 'failed' ? 'failed' : 
                        liveTest.status === 'pending' ? 'pending' : ''
                      }`}>
                        {liveTest.status === 'failed' ? (
                          <svg className="error-icon" viewBox="0 0 24 24" fill="none">
                            <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="3" />
                            <path d="M15 9L9 15M9 9l6 6" stroke="currentColor" strokeWidth="3" strokeLinecap="round" />
                          </svg>
                        ) : liveTest.status === 'pending' ? (
                          <svg className="pending-icon" viewBox="0 0 24 24" fill="none">
                            <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="3" opacity="0.3" />
                            <path d="M12 6v6l4 2" stroke="currentColor" strokeWidth="3" strokeLinecap="round" />
                          </svg>
                        ) : (
                          <svg className="spinner-icon" viewBox="0 0 24 24" fill="none">
                            <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeDasharray="32 32" />
                          </svg>
                        )}
                      </div>
                    )}
                  </div>
                </div>
                
                {/* Server Tags */}
                {serverInfo && (
                  <div className="server-tags">
                    <span className="tag tag-protocol">{serverInfo.default_protocol.toUpperCase()}</span>
                    <span className="tag tag-streams">{serverInfo.default_parallel} {serverInfo.default_parallel === 1 ? 'Stream' : 'Streams'}</span>
                    {serverInfo.schedule_enabled && (
                      <span className="tag tag-schedule">Every {serverInfo.schedule_interval_minutes} min</span>
                    )}
                    {serverStat.total_tests > 0 && (
                      <span className="tag tag-success">
                        {Math.round((serverStat.successful_tests / serverStat.total_tests) * 100)}% OK
                      </span>
                    )}
                  </div>
                )}
                
                <div className="server-stats">
                  <div className="stat-row">
                    <span className="label">Download:</span>
                    <span className={`value blue ${isRunning ? 'live-value' : ''}`}>
                      {isRunning 
                        ? (liveTest.status === 'failed' 
                          ? <span className="error-text">Test Failed</span>
                          : liveTest.status === 'pending'
                          ? <span className="pending-text">Pending</span>
                          : `${liveTest.current_download_mbps.toFixed(2)} Mbps`)
                        : (serverStat.avg_download_mbps !== null && serverStat.avg_download_mbps !== undefined 
                          ? `${serverStat.avg_download_mbps.toFixed(2)} Mbps` 
                          : 'N/A')
                      }
                    </span>
                  </div>
                  <div className="stat-row">
                    <span className="label">Upload:</span>
                    <span className={`value green ${isRunning ? 'live-value' : ''}`}>
                      {isRunning 
                        ? (liveTest.status === 'failed' 
                          ? <span className="error-text">Test Failed</span>
                          : liveTest.status === 'pending'
                          ? <span className="pending-text">Pending</span>
                          : `${liveTest.current_upload_mbps.toFixed(2)} Mbps`)
                        : (serverStat.avg_upload_mbps !== null && serverStat.avg_upload_mbps !== undefined 
                          ? `${serverStat.avg_upload_mbps.toFixed(2)} Mbps` 
                          : 'N/A')
                      }
                    </span>
                  </div>
                  {!isRunning && serverStat.avg_jitter_ms !== null && serverStat.avg_jitter_ms !== undefined && (
                    <div className="stat-row">
                      <span className="label">Avg Jitter:</span>
                      <span className="value">{serverStat.avg_jitter_ms.toFixed(2)} ms</span>
                    </div>
                  )}
                  {!isRunning && serverStat.avg_packet_loss_percent !== null && serverStat.avg_packet_loss_percent !== undefined && (
                    <div className="stat-row">
                      <span className="label">Packet Loss:</span>
                      <span className="value orange">{serverStat.avg_packet_loss_percent.toFixed(2)}%</span>
                    </div>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
};

export default Dashboard;
