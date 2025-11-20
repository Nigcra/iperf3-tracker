import React, { useEffect, useState, useRef } from 'react';
import './PeeringMap.css';
import * as api from '../services/api';
import { Trace, TraceHop } from '../services/api';
import LiveMap from './LiveMap';

interface PeeringMapProps {
  testId?: number;
}

const PeeringMap: React.FC<PeeringMapProps> = ({ testId }) => {
  const [traces, setTraces] = useState<Trace[]>([]);
  const [selectedTrace, setSelectedTrace] = useState<Trace | null>(null);
  const [servers, setServers] = useState<any[]>([]);
  const [selectedServer, setSelectedServer] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [runningTrace, setRunningTrace] = useState(false);
  const [liveHops, setLiveHops] = useState<TraceHop[]>([]);
  const [isLiveMode, setIsLiveMode] = useState(false);
  const [liveDestination, setLiveDestination] = useState<string>('');
  const [error, setError] = useState<string | null>(null);
  const eventSourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    loadData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [testId]);

  const loadData = async () => {
    try {
      setLoading(true);
      setError(null);

      const serverList = await api.getServers();
      setServers(serverList);

      if (testId) {
        const trace = await api.getTrace(testId);
        setSelectedTrace(trace);
        setTraces([trace]);
      } else {
        const recentTraces = await api.getRecentTraces(10);
        setTraces(recentTraces);
        console.log('Loaded traces:', recentTraces.length);
        
        if (recentTraces.length > 0) {
          // Select the most recent trace (first in list should be newest)
          const newest = recentTraces[0];
          console.log('Auto-selecting trace:', newest.id, 'hops:', newest.total_hops);
          setSelectedTrace(newest);
        }
      }
    } catch (err: any) {
      console.error('Error loading data:', err);
      setError(err.message || 'Failed to load data');
    } finally {
      setLoading(false);
    }
  };

  const startTraceroute = async () => {
    if (!selectedServer) {
      setError('Please select a server first');
      return;
    }

    const server = servers.find(s => s.id === selectedServer);
    if (!server) return;

    try {
      setRunningTrace(true);
      setIsLiveMode(true);
      setLiveHops([]);
      setLiveDestination(server.host);
      setError(null);

      const token = localStorage.getItem('token');
      if (!token) {
        setError('Not authenticated');
        setRunningTrace(false);
        setIsLiveMode(false);
        return;
      }
      
      const eventSource = new EventSource(
        `http://localhost:8000/api/live-trace/stream/${encodeURIComponent(server.host)}?token=${encodeURIComponent(token)}`
      );
      
      eventSourceRef.current = eventSource;

      eventSource.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          
          if (data.type === 'start') {
            console.log('Traceroute started:', data.destination);
          } else if (data.type === 'hop') {
            setLiveHops(prev => [...prev, data.data]);
          } else if (data.type === 'complete') {
            console.log('Traceroute complete');
            eventSource.close();
            setRunningTrace(false);
            
            // Wait longer before loading data (give backend time to save)
            setTimeout(async () => {
              console.log('Loading historical traces...');
              await loadData();
              
              // Keep live mode active for a moment to show smooth transition
              setTimeout(() => {
                console.log('Switching to historical view');
                setIsLiveMode(false);
                setLiveHops([]);
              }, 1000);
            }, 2000); // Increased from 1000ms to 2000ms
          } else if (data.type === 'error') {
            setError(data.message);
            eventSource.close();
            setRunningTrace(false);
            setIsLiveMode(false);
          }
        } catch (err) {
          console.error('Error parsing SSE event:', err);
        }
      };

      eventSource.onerror = (err) => {
        console.error('SSE error:', err);
        setError('Connection lost');
        eventSource.close();
        setRunningTrace(false);
        setIsLiveMode(false);
      };

      setTimeout(() => {
        if (runningTrace) {
          eventSource.close();
          setRunningTrace(false);
          setIsLiveMode(false);
          setError('Traceroute timeout');
        }
      }, 180000);

    } catch (err: any) {
      console.error('Error starting traceroute:', err);
      setError(err.message || 'Failed to start traceroute');
      setRunningTrace(false);
      setIsLiveMode(false);
    }
  };

  const stopTraceroute = () => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
    setRunningTrace(false);
    setIsLiveMode(false);
    setLiveHops([]);
    console.log('Traceroute stopped by user');
  };

  const renderMap = () => {
    const hopsToDisplay = isLiveMode ? liveHops : (selectedTrace?.hops || []);
    
    console.log('renderMap:', { 
      isLiveMode, 
      liveHopsLength: liveHops.length, 
      selectedTraceHops: selectedTrace?.hops?.length,
      hopsToDisplay: hopsToDisplay.length 
    });
    
    if (hopsToDisplay.length === 0) {
      return (
        <div className="no-data">
          <p>{isLiveMode ? '🔄 Waiting for trace data...' : 'No trace data available'}</p>
          {isLiveMode && <div className="spinner"></div>}
        </div>
      );
    }

    return <LiveMap hops={hopsToDisplay} isLive={isLiveMode} />;
  };

  if (loading) {
    return (
      <div className="peering-map-page">
        <div className="loading">Loading traces...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="peering-map-page">
        <div className="error-message">
          <h3>Error</h3>
          <p>{error}</p>
          <button onClick={loadData} className="btn btn-primary">Retry</button>
        </div>
      </div>
    );
  }

  return (
    <div className="peering-map-page">
      <div className="page-header">
        <h1>Peering Map</h1>
        <p>Visualize network paths and hop performance</p>
      </div>

      {/* Server Selection */}
      <div className="trace-controls">
        <div className="control-group">
          <label>Select Server:</label>
          <select
            value={selectedServer || ''}
            onChange={(e) => setSelectedServer(Number(e.target.value))}
            disabled={runningTrace}
          >
            <option value="">-- Choose a server --</option>
            {servers.map(server => (
              <option key={server.id} value={server.id}>
                {server.name} ({server.host})
              </option>
            ))}
          </select>
        </div>
        <button
          onClick={startTraceroute}
          disabled={!selectedServer || runningTrace}
          className="btn btn-primary"
        >
          {runningTrace ? '🔄 Running Traceroute...' : '🚀 Start Traceroute'}
        </button>
      </div>

      {/* Live Status */}
      {isLiveMode && (
        <div className="live-status-compact">
          <div className="live-indicator">
            <span className="live-dot"></span>
            <strong>LIVE TRACING</strong> - {liveDestination}
          </div>
          <div className="live-stats">
            Hops: <strong>{liveHops.length}</strong> | 
            Located: <strong>{liveHops.filter(h => h.latitude && h.longitude).length}</strong>
          </div>
          <button onClick={stopTraceroute} className="btn btn-danger btn-sm">
            Stop
          </button>
        </div>
      )}

      {/* Previous Traces */}
      {!testId && (
        <div className="trace-selector">
          <label>View Previous Trace:</label>
          <select
            value={selectedTrace?.id || ''}
            onChange={(e) => {
              const trace = traces.find(t => t.id === Number(e.target.value));
              setSelectedTrace(trace || null);
              // Exit live mode when selecting historical trace
              if (isLiveMode) {
                stopTraceroute();
              }
            }}
            disabled={isLiveMode || traces.length === 0}
          >
            <option value="">
              {traces.length === 0 ? '-- No traces available --' : '-- Select a trace --'}
            </option>
            {traces.map(trace => (
              <option key={trace.id} value={trace.id}>
                {trace.destination_host} - {new Date(trace.created_at).toLocaleString()} ({trace.total_hops} hops)
              </option>
            ))}
          </select>
          <button onClick={loadData} className="btn btn-secondary" disabled={isLiveMode}>
            Refresh
          </button>
        </div>
      )}

      {/* Map */}
      <div className="map-container">
        {renderMap()}
      </div>
    </div>
  );
};

export default PeeringMap;
