import React, { useEffect, useRef, useState } from 'react';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';
import { TraceHop } from '../services/api';

interface LiveMapProps {
  hops: TraceHop[];
  isLive?: boolean;
  destination?: string;
  onStop?: () => void;
}

const LiveMap: React.FC<LiveMapProps> = ({ hops, isLive = false, destination, onStop }) => {
  const mapRef = useRef<L.Map | null>(null);
  const mapContainerRef = useRef<HTMLDivElement>(null);
  const markersRef = useRef<L.Marker[]>([]);
  const [mapReady, setMapReady] = useState(false);
  const tileLayerRef = useRef<L.TileLayer | null>(null);
  const [currentTheme, setCurrentTheme] = useState<string>(() => {
    return document.documentElement.getAttribute('data-theme') || 'light';
  });

  // Initialize map once
  useEffect(() => {
    if (!mapContainerRef.current || mapRef.current) return;

    console.log('Initializing Leaflet map');
    
    // Create map
    const map = L.map(mapContainerRef.current, {
      center: [50.0, 10.0],
      zoom: 4,
      zoomControl: true,
    });

    // Determine initial theme
    const theme = document.documentElement.getAttribute('data-theme') || 'light';
    
    // Add tile layer based on theme
    const tileUrl = theme === 'dark'
      ? 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png'
      : 'https://{s}.basemaps.cartocdn.com/rastertiles/voyager/{z}/{x}/{y}{r}.png';
    
    const tileLayer = L.tileLayer(tileUrl, {
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>',
      maxZoom: 18,
    }).addTo(map);

    tileLayerRef.current = tileLayer;
    mapRef.current = map;
    setMapReady(true);

    // Force map to recalculate size
    setTimeout(() => {
      map.invalidateSize();
    }, 100);

    return () => {
      if (mapRef.current) {
        mapRef.current.remove();
        mapRef.current = null;
      }
    };
  }, []);

  // Observer for theme changes
  useEffect(() => {
    const observer = new MutationObserver(() => {
      const theme = document.documentElement.getAttribute('data-theme') || 'light';
      setCurrentTheme(theme);
    });

    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['data-theme']
    });

    return () => {
      observer.disconnect();
    };
  }, []);

  // Update tile layer when theme changes
  useEffect(() => {
    if (!mapRef.current || !tileLayerRef.current) return;

    const tileUrl = currentTheme === 'dark'
      ? 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png'
      : 'https://{s}.basemaps.cartocdn.com/rastertiles/voyager/{z}/{x}/{y}{r}.png';

    // Remove old tile layer
    tileLayerRef.current.remove();

    // Add new tile layer
    const newTileLayer = L.tileLayer(tileUrl, {
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>',
      maxZoom: 18,
    }).addTo(mapRef.current);

    tileLayerRef.current = newTileLayer;
  }, [currentTheme]);

  // Update markers when hops change
  useEffect(() => {
    if (!mapRef.current || !mapReady) return;

    console.log('Updating map with', hops.length, 'hops');

    // Clear existing markers
    markersRef.current.forEach(marker => marker.remove());
    markersRef.current = [];

    // Filter valid hops
    const validHops = hops.filter(
      hop => hop.latitude !== null && 
             hop.longitude !== null && 
             !isNaN(hop.latitude) && 
             !isNaN(hop.longitude)
    );

    console.log('Valid hops with coordinates:', validHops.length);

    if (validHops.length === 0) return;

    // Create custom icons
    const createIcon = (color: string, label: string) => {
      return L.divIcon({
        html: `<div style="
          background: ${color};
          width: 24px;
          height: 24px;
          border-radius: 50%;
          border: 2px solid white;
          display: flex;
          align-items: center;
          justify-content: center;
          font-size: 12px;
          color: white;
          font-weight: bold;
          box-shadow: 0 2px 8px rgba(0,0,0,0.3);
        ">${label}</div>`,
        className: 'custom-map-marker',
        iconSize: [24, 24],
        iconAnchor: [12, 12],
      });
    };

    // Add markers for each hop
    validHops.forEach((hop, index) => {
      const isFirst = index === 0;
      const isLast = index === validHops.length - 1;
      const isInterpolated = hop.geoip_interpolated === true;
      
      let icon: L.DivIcon;
      if (isFirst) {
        icon = createIcon('#10b981', '🚀');
      } else if (isLast) {
        icon = createIcon('#ef4444', '🎯');
      } else if (isInterpolated) {
        icon = createIcon('#f59e0b', '?');
      } else {
        icon = createIcon('#3b82f6', String(hop.hop_number));
      }

      const marker = L.marker([hop.latitude, hop.longitude], { icon })
        .bindPopup(`
          <div style="font-family: system-ui; min-width: 200px;">
            <strong style="font-size: 14px;">Hop #${hop.hop_number}</strong>${isInterpolated ? ' <span style="color: #f59e0b;">⚠️ Estimated</span>' : ''}<br/>
            ${hop.city ? `📍 ${hop.city}, ${hop.country}<br/>` : ''}
            ${hop.ip_address ? `IP: ${hop.ip_address}<br/>` : ''}
            ${hop.hostname ? `Host: ${hop.hostname}<br/>` : ''}
            ${hop.rtt_ms ? `RTT: ${hop.rtt_ms.toFixed(1)}ms<br/>` : ''}
            ${hop.asn_organization ? `ISP: ${hop.asn_organization}` : ''}
            ${isInterpolated ? `<br/><span style="color: #f59e0b; font-size: 12px; font-style: italic;">Location estimated from nearby hops</span>` : ''}
          </div>
        `)
        .addTo(mapRef.current);

      markersRef.current.push(marker);
    });

    // Draw path
    if (validHops.length > 1) {
      const pathCoords: L.LatLngExpression[] = validHops.map(hop => [hop.latitude, hop.longitude]);
      L.polyline(pathCoords, {
        color: '#2196F3',
        weight: 3,
        opacity: 0.7,
        dashArray: '10, 5',
      }).addTo(mapRef.current);
    }

    // Fit bounds to show all markers
    const bounds = L.latLngBounds(validHops.map(hop => [hop.latitude, hop.longitude]));
    mapRef.current.fitBounds(bounds, { padding: [50, 50], maxZoom: 10 });

  }, [hops, mapReady]);

  return (
    <div style={{ position: 'relative', width: '100%', height: '100%' }}>
      <div 
        ref={mapContainerRef} 
        style={{ 
          width: '100%', 
          height: '100%', 
          minHeight: '600px',
          borderRadius: '12px',
          overflow: 'hidden',
        }} 
      />
      {isLive && hops.length > 0 && (
        <div style={{
          position: 'absolute',
          top: '10px',
          left: '60px',
          background: 'var(--card-bg)',
          color: 'var(--text-primary)',
          padding: '12px 16px',
          borderRadius: '8px',
          fontSize: '14px',
          fontWeight: 500,
          zIndex: 1000,
          display: 'flex',
          alignItems: 'center',
          gap: '12px',
          border: '1px solid var(--border-color)',
          backdropFilter: 'blur(10px)',
        }}>
          <div style={{
            width: '8px',
            height: '8px',
            borderRadius: '50%',
            background: '#10b981',
            animation: 'pulse 1.5s ease-in-out infinite',
            boxShadow: '0 0 8px rgba(16, 185, 129, 0.6)',
          }} />
          <span style={{ textTransform: 'uppercase', letterSpacing: '0.5px', fontSize: '11px', fontWeight: 600 }}>
            Live Tracing
          </span>
          {destination && (
            <>
              <span style={{ color: 'var(--text-secondary)' }}>-</span>
              <span style={{ fontSize: '13px' }}>{destination}</span>
            </>
          )}
          <span style={{ color: 'var(--text-secondary)' }}>|</span>
          <span style={{ fontSize: '13px' }}>
            Hops: <strong>{hops.length}</strong>
          </span>
          <span style={{ color: 'var(--text-secondary)' }}>|</span>
          <span style={{ fontSize: '13px' }}>
            Located: <strong>{hops.filter(h => h.latitude && h.longitude).length}</strong>
          </span>
          {onStop && (
            <button 
              onClick={onStop}
              style={{
                marginLeft: '8px',
                background: '#ef4444',
                color: 'white',
                border: 'none',
                padding: '4px 12px',
                borderRadius: '6px',
                fontSize: '12px',
                fontWeight: 600,
                cursor: 'pointer',
                transition: 'background 0.2s',
              }}
              onMouseOver={(e) => e.currentTarget.style.background = '#dc2626'}
              onMouseOut={(e) => e.currentTarget.style.background = '#ef4444'}
            >
              Stop
            </button>
          )}
        </div>
      )}
    </div>
  );
};

export default LiveMap;
