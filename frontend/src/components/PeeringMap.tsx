import React, { useEffect, useState } from 'react';
import { MapContainer, TileLayer, Marker, Polyline, Popup, CircleMarker } from 'react-leaflet';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';
import './PeeringMap.css';
import * as api from '../services/api';
import { Trace, TraceHop } from '../services/api';

// Fix for default marker icons in react-leaflet
delete (L.Icon.Default.prototype as any)._getIconUrl;
L.Icon.Default.mergeOptions({
  iconRetinaUrl: require('leaflet/dist/images/marker-icon-2x.png'),
  iconUrl: require('leaflet/dist/images/marker-icon.png'),
  shadowUrl: require('leaflet/dist/images/marker-shadow.png'),
});

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
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadData();
  }, [testId]);

  const loadData = async () => {
    try {
      setLoading(true);
      setError(null);

      // Load servers
      const serverList = await api.getServers();
      setServers(serverList);

      if (testId) {
        // Load specific trace
        const trace = await api.getTrace(testId);
        setTraces([trace]);
        setSelectedTrace(trace);
      } else {
        // Load recent traces
        const recentTraces = await api.getRecentTraces(10);
        setTraces(recentTraces);
        if (recentTraces.length > 0) {
          setSelectedTrace(recentTraces[0]);
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
      setError(null);

      // Create new trace
      const trace = await api.createTrace({
        destination: server.host,
        max_hops: 30,
        timeout: 2,
        count: 3
      });

      // Reload traces
      await loadData();
      
      // Select the new trace
      setSelectedTrace(trace);

      // Poll for updates (trace runs async on backend)
      const pollInterval = setInterval(async () => {
        try {
          const updatedTrace = await api.getTrace(trace.id);
          setSelectedTrace(updatedTrace);
          
          if (updatedTrace.completed) {
            clearInterval(pollInterval);
            setRunningTrace(false);
            await loadData(); // Reload all traces
          }
        } catch (err) {
          console.error('Error polling trace:', err);
          clearInterval(pollInterval);
          setRunningTrace(false);
        }
      }, 3000); // Poll every 3 seconds

      // Stop polling after 3 minutes (increased for longer traceroutes)
      setTimeout(() => {
        clearInterval(pollInterval);
        setRunningTrace(false);
      }, 180000);

    } catch (err: any) {
      console.error('Error starting traceroute:', err);
      setError(err.message || 'Failed to start traceroute');
      setRunningTrace(false);
    }
  };

  const getHopColor = (rtt: number | null): string => {
    if (!rtt) return '#999';
    if (rtt < 20) return '#4CAF50'; // Green - fast
    if (rtt < 50) return '#FFC107'; // Yellow - medium
    if (rtt < 100) return '#FF9800'; // Orange - slow
    return '#F44336'; // Red - very slow
  };

  const getHopRadius = (hop: TraceHop, maxRtt: number): number => {
    if (!hop.rtt_ms) return 6;
    // Scale radius based on RTT (6-16px)
    const normalized = hop.rtt_ms / maxRtt;
    return 6 + normalized * 10;
  };

  const createStartIcon = () => {
    return L.divIcon({
      html: `<div style="
        background: linear-gradient(135deg, #4CAF50 0%, #45a049 100%);
        border: 3px solid white;
        border-radius: 50%;
        width: 24px;
        height: 24px;
        box-shadow: 0 4px 12px rgba(76, 175, 80, 0.4);
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 14px;
      ">🏠</div>`,
      className: 'custom-marker',
      iconSize: [24, 24],
      iconAnchor: [12, 12]
    });
  };

  const createEndIcon = () => {
    return L.divIcon({
      html: `<div style="
        background: linear-gradient(135deg, #2196F3 0%, #1976D2 100%);
        border: 3px solid white;
        border-radius: 50%;
        width: 24px;
        height: 24px;
        box-shadow: 0 4px 12px rgba(33, 150, 243, 0.4);
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 14px;
      ">🎯</div>`,
      className: 'custom-marker',
      iconSize: [24, 24],
      iconAnchor: [12, 12]
    });
  };

  const renderMap = () => {
    if (!selectedTrace || !selectedTrace.hops || selectedTrace.hops.length === 0) {
      return (
        <div className="no-data">
          <p>No trace data available</p>
          {!selectedTrace?.completed && selectedTrace && (
            <p>Trace is still running...</p>
          )}
        </div>
      );
    }

    // Filter hops with valid coordinates
    const validHops = selectedTrace.hops.filter(
      hop => hop.latitude !== null && hop.longitude !== null && hop.responded
    );

    if (validHops.length === 0) {
      return (
        <div className="no-data">
          <p>No geolocation data available for this trace</p>
        </div>
      );
    }

    // Calculate map center and bounds
    const lats = validHops.map(h => h.latitude!);
    const lons = validHops.map(h => h.longitude!);
    const center: [number, number] = [
      (Math.min(...lats) + Math.max(...lats)) / 2,
      (Math.min(...lons) + Math.max(...lons)) / 2
    ];

    // Create path for polyline (including start and end if we have their coordinates)
    const pathPositions: [number, number][] = validHops.map(hop => [hop.latitude!, hop.longitude!]);

    // Find max RTT for scaling
    const maxRtt = Math.max(...validHops.map(h => h.rtt_ms || 0));

    return (
      <MapContainer
        center={center}
        zoom={4}
        style={{ height: '100%', width: '100%' }}
        className="peering-map-container"
      >
        <TileLayer
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
        />

        {/* Draw path line */}
        <Polyline
          positions={pathPositions}
          color="#2196F3"
          weight={3}
          opacity={0.7}
          dashArray="10, 5"
          className="trace-path"
        />

        {/* Start marker (first hop with location) */}
        {validHops.length > 0 && (
          <Marker
            position={[validHops[0].latitude!, validHops[0].longitude!]}
            icon={createStartIcon()}
          >
            <Popup>
              <div className="hop-popup">
                <strong>Start Location</strong>
                <br />
                {validHops[0].city && <div>{validHops[0].city}, {validHops[0].country}</div>}
                {validHops[0].ip_address && <div className="mono">{validHops[0].ip_address}</div>}
                {validHops[0].hostname && <div className="mono small">{validHops[0].hostname}</div>}
              </div>
            </Popup>
          </Marker>
        )}

        {/* End marker (last hop with location) */}
        {validHops.length > 1 && (
          <Marker
            position={[validHops[validHops.length - 1].latitude!, validHops[validHops.length - 1].longitude!]}
            icon={createEndIcon()}
          >
            <Popup>
              <div className="hop-popup">
                <strong>🎯 Destination</strong>
                <br />
                <div className="mono">{selectedTrace.destination_host}</div>
                {selectedTrace.destination_ip && (
                  <div className="mono small">{selectedTrace.destination_ip}</div>
                )}
                {validHops[validHops.length - 1].city && (
                  <div>{validHops[validHops.length - 1].city}, {validHops[validHops.length - 1].country}</div>
                )}
              </div>
            </Popup>
          </Marker>
        )}

        {/* Hop markers */}
        {validHops.map((hop, index) => {
          // Skip first and last (already shown as start/end markers)
          if (index === 0 || index === validHops.length - 1) return null;

          return (
            <CircleMarker
              key={hop.id}
              center={[hop.latitude!, hop.longitude!]}
              radius={getHopRadius(hop, maxRtt)}
              fillColor={getHopColor(hop.rtt_ms)}
              color="white"
              weight={2}
              opacity={1}
              fillOpacity={0.8}
            >
              <Popup>
                <div className="hop-popup">
                  <strong>Hop #{hop.hop_number}</strong>
                  <br />
                  {hop.city && <div>📍 {hop.city}, {hop.country}</div>}
                  {hop.ip_address && <div className="mono">{hop.ip_address}</div>}
                  {hop.hostname && <div className="mono small">{hop.hostname}</div>}
                  {hop.asn_organization && <div className="small">🏢 {hop.asn_organization}</div>}
                  {hop.rtt_ms !== null && (
                    <div className="metric">
                      ⏱️ <strong>{hop.rtt_ms.toFixed(1)} ms</strong>
                    </div>
                  )}
                  {hop.packet_loss !== null && hop.packet_loss > 0 && (
                    <div className="metric warning">
                      📉 Packet Loss: {hop.packet_loss.toFixed(1)}%
                    </div>
                  )}
                </div>
              </Popup>
            </CircleMarker>
          );
        })}
      </MapContainer>
    );
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

      {/* Server Selection and Traceroute */}
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

      {!testId && traces.length > 0 && (
        <div className="trace-selector">
          <label>View Previous Trace:</label>
          <select
            value={selectedTrace?.id || ''}
            onChange={(e) => {
              const trace = traces.find(t => t.id === Number(e.target.value));
              setSelectedTrace(trace || null);
            }}
          >
            <option value="">-- Select a trace --</option>
            {traces.map(trace => (
              <option key={trace.id} value={trace.id}>
                {trace.destination_host} - {new Date(trace.created_at).toLocaleString()} ({trace.total_hops} hops)
              </option>
            ))}
          </select>
          <button onClick={loadData} className="btn btn-secondary">
            Refresh
          </button>
        </div>
      )}

      {selectedTrace && (
        <div className="trace-info">
          <div className="info-grid">
            <div className="info-item">
              <label>Destination:</label>
              <span className="mono">{selectedTrace.destination_host}</span>
            </div>
            <div className="info-item">
              <label>Total Hops:</label>
              <span>{selectedTrace.total_hops}</span>
            </div>
            <div className="info-item">
              <label>Total RTT:</label>
              <span>{selectedTrace.total_rtt_ms?.toFixed(1)} ms</span>
            </div>
            <div className="info-item">
              <label>Status:</label>
              <span className={selectedTrace.completed ? 'status-completed' : 'status-running'}>
                {selectedTrace.completed ? 'Completed' : 'Running'}
              </span>
            </div>
          </div>
        </div>
      )}

      <div className="map-container">
        {renderMap()}
      </div>

      {selectedTrace && selectedTrace.hops && selectedTrace.hops.length > 0 && (
        <div className="hops-table">
          <h2>Network Hops</h2>
          <table>
            <thead>
              <tr>
                <th>#</th>
                <th>IP Address</th>
                <th>Hostname</th>
                <th>Location</th>
                <th>ISP</th>
                <th>RTT</th>
                <th>Loss</th>
              </tr>
            </thead>
            <tbody>
              {selectedTrace.hops.map(hop => (
                <tr key={hop.id} className={!hop.responded ? 'no-response' : ''}>
                  <td>{hop.hop_number}</td>
                  <td className="mono">{hop.ip_address || '* * *'}</td>
                  <td className="mono small">{hop.hostname || '-'}</td>
                  <td>
                    {hop.city && hop.country ? (
                      <>
                        {hop.city}, {hop.country_code}
                      </>
                    ) : '-'}
                  </td>
                  <td className="small">{hop.asn_organization || '-'}</td>
                  <td className={hop.rtt_ms ? `rtt-${hop.rtt_ms < 20 ? 'good' : hop.rtt_ms < 50 ? 'medium' : 'bad'}` : ''}>
                    {hop.rtt_ms !== null ? `${hop.rtt_ms.toFixed(1)} ms` : '-'}
                  </td>
                  <td>
                    {hop.packet_loss !== null && hop.packet_loss > 0
                      ? `${hop.packet_loss.toFixed(1)}%`
                      : '-'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <div className="legend">
        <h3>Performance Legend</h3>
        <div className="legend-items">
          <div className="legend-item">
            <div className="legend-color" style={{ background: '#4CAF50' }}></div>
            <span>&lt; 20ms - Excellent</span>
          </div>
          <div className="legend-item">
            <div className="legend-color" style={{ background: '#FFC107' }}></div>
            <span>20-50ms - Good</span>
          </div>
          <div className="legend-item">
            <div className="legend-color" style={{ background: '#FF9800' }}></div>
            <span>50-100ms - Fair</span>
          </div>
          <div className="legend-item">
            <div className="legend-color" style={{ background: '#F44336' }}></div>
            <span>&gt; 100ms - Poor</span>
          </div>
        </div>
      </div>
    </div>
  );
};

export default PeeringMap;
