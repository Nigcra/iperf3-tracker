import React, { useEffect, useRef } from 'react';
import { TraceHop } from '../services/api';
import './NetworkTopology.css';

interface NetworkTopologyProps {
  hops: TraceHop[];
  isLive?: boolean;
  destination?: string;
  onStop?: () => void;
}

const NetworkTopology: React.FC<NetworkTopologyProps> = ({ hops, isLive = false, destination, onStop }) => {
  const svgRef = useRef<SVGSVGElement>(null);

  // Check if destination is a local/private IP
  const isPrivateIP = (ip: string): boolean => {
    const parts = ip.split('.');
    if (parts.length !== 4) return false;
    
    const firstOctet = parseInt(parts[0]);
    const secondOctet = parseInt(parts[1]);
    
    // Class A: 10.0.0.0 - 10.255.255.255
    if (firstOctet === 10) return true;
    
    // Class B: 172.16.0.0 - 172.31.255.255
    if (firstOctet === 172 && secondOctet >= 16 && secondOctet <= 31) return true;
    
    // Class C: 192.168.0.0 - 192.168.255.255
    if (firstOctet === 192 && secondOctet === 168) return true;
    
    return false;
  };

  useEffect(() => {
    if (!svgRef.current || hops.length === 0) return;

    // Clear existing content
    while (svgRef.current.firstChild) {
      svgRef.current.removeChild(svgRef.current.firstChild);
    }

    const svg = svgRef.current;
    const svgNS = "http://www.w3.org/2000/svg";
    
    // Calculate layout
    const nodeWidth = 120;
    const nodeHeight = 80;
    const horizontalSpacing = 180;
    const verticalSpacing = 140;
    const startX = 100;
    const startY = 80;

    // Determine if we need multiple columns for better layout
    const maxNodesPerColumn = 8;
    const columns = Math.ceil(hops.length / maxNodesPerColumn);

    // Create nodes for each hop
    hops.forEach((hop, index) => {
      const column = Math.floor(index / maxNodesPerColumn);
      const row = index % maxNodesPerColumn;
      
      const x = startX + column * horizontalSpacing;
      const y = startY + row * verticalSpacing;

      // Draw connection line to next hop
      if (index < hops.length - 1) {
        const nextColumn = Math.floor((index + 1) / maxNodesPerColumn);
        const nextRow = (index + 1) % maxNodesPerColumn;
        const nextX = startX + nextColumn * horizontalSpacing;
        const nextY = startY + nextRow * verticalSpacing;

        const line = document.createElementNS(svgNS, "line");
        line.setAttribute("x1", String(x + nodeWidth / 2));
        line.setAttribute("y1", String(y + nodeHeight));
        line.setAttribute("x2", String(nextX + nodeWidth / 2));
        line.setAttribute("y2", String(nextY));
        line.setAttribute("class", "topology-connection");
        svg.appendChild(line);

        // Add arrow marker
        const arrowSize = 8;
        const polygon = document.createElementNS(svgNS, "polygon");
        polygon.setAttribute("points", `0,0 ${arrowSize},${arrowSize/2} 0,${arrowSize}`);
        polygon.setAttribute("class", "topology-arrow");
        
        // Calculate arrow position and rotation
        const angle = Math.atan2(nextY - (y + nodeHeight), nextX + nodeWidth/2 - (x + nodeWidth/2));
        const midX = (x + nodeWidth/2 + nextX + nodeWidth/2) / 2;
        const midY = (y + nodeHeight + nextY) / 2;
        
        polygon.setAttribute("transform", `translate(${midX},${midY}) rotate(${angle * 180 / Math.PI + 90})`);
        svg.appendChild(polygon);
      }

      // Create node group
      const group = document.createElementNS(svgNS, "g");
      group.setAttribute("class", "topology-node");
      group.setAttribute("transform", `translate(${x}, ${y})`);

      // Determine node type
      const isFirst = index === 0;
      const isLast = index === hops.length - 1;
      const isPrivate = hop.ip_address ? isPrivateIP(hop.ip_address) : false;

      // Node background
      const rect = document.createElementNS(svgNS, "rect");
      rect.setAttribute("width", String(nodeWidth));
      rect.setAttribute("height", String(nodeHeight));
      rect.setAttribute("rx", "8");
      rect.setAttribute("class", isFirst ? "topology-node-start" : isLast ? "topology-node-end" : "topology-node-middle");
      group.appendChild(rect);

      // Router icon (simplified)
      const iconY = 15;
      if (isFirst) {
        // Computer/Source icon
        const icon = document.createElementNS(svgNS, "text");
        icon.setAttribute("x", String(nodeWidth / 2));
        icon.setAttribute("y", String(iconY));
        icon.setAttribute("text-anchor", "middle");
        icon.setAttribute("class", "topology-icon");
        icon.textContent = "🖥️";
        group.appendChild(icon);
      } else if (isLast) {
        // Target icon
        const icon = document.createElementNS(svgNS, "text");
        icon.setAttribute("x", String(nodeWidth / 2));
        icon.setAttribute("y", String(iconY));
        icon.setAttribute("text-anchor", "middle");
        icon.setAttribute("class", "topology-icon");
        icon.textContent = "🎯";
        group.appendChild(icon);
      } else {
        // Router icon
        const icon = document.createElementNS(svgNS, "text");
        icon.setAttribute("x", String(nodeWidth / 2));
        icon.setAttribute("y", String(iconY));
        icon.setAttribute("text-anchor", "middle");
        icon.setAttribute("class", "topology-icon");
        icon.textContent = isPrivate ? "📡" : "🌐";
        group.appendChild(icon);
      }

      // Hop number
      const hopLabel = document.createElementNS(svgNS, "text");
      hopLabel.setAttribute("x", String(nodeWidth / 2));
      hopLabel.setAttribute("y", "35");
      hopLabel.setAttribute("text-anchor", "middle");
      hopLabel.setAttribute("class", "topology-hop-label");
      hopLabel.textContent = `Hop ${hop.hop_number}`;
      group.appendChild(hopLabel);

      // IP Address
      const ipText = document.createElementNS(svgNS, "text");
      ipText.setAttribute("x", String(nodeWidth / 2));
      ipText.setAttribute("y", "50");
      ipText.setAttribute("text-anchor", "middle");
      ipText.setAttribute("class", "topology-ip");
      ipText.textContent = hop.ip_address || 'Unknown';
      group.appendChild(ipText);

      // RTT
      if (hop.rtt_ms !== null && hop.rtt_ms !== undefined) {
        const rttText = document.createElementNS(svgNS, "text");
        rttText.setAttribute("x", String(nodeWidth / 2));
        rttText.setAttribute("y", "65");
        rttText.setAttribute("text-anchor", "middle");
        rttText.setAttribute("class", "topology-rtt");
        rttText.textContent = `${hop.rtt_ms.toFixed(1)}ms`;
        group.appendChild(rttText);
      }

      svg.appendChild(group);
    });

    // Update SVG viewBox
    const totalWidth = startX + columns * horizontalSpacing + 100;
    const totalHeight = startY + Math.min(hops.length, maxNodesPerColumn) * verticalSpacing + 100;
    svg.setAttribute("viewBox", `0 0 ${totalWidth} ${totalHeight}`);
    svg.setAttribute("width", "100%");
    svg.setAttribute("height", String(Math.min(totalHeight, 800)));

  }, [hops]);

  return (
    <div className="network-topology">
      {isLive && (
        <div className="topology-header">
          <div className="topology-status">
            <div className="pulse-dot" />
            <span className="status-label">LIVE TRACING</span>
            {destination && (
              <>
                <span className="separator">-</span>
                <span className="destination">{destination}</span>
              </>
            )}
            <span className="separator">|</span>
            <span className="hop-count">
              Hops: <strong>{hops.length}</strong>
            </span>
          </div>
          {onStop && (
            <button onClick={onStop} className="stop-button">
              Stop
            </button>
          )}
        </div>
      )}
      
      <div className="topology-content">
        {hops.length === 0 ? (
          <div className="no-data">
            <p>{isLive ? '🔄 Waiting for trace data...' : 'No trace data available'}</p>
            {isLive && <div className="spinner"></div>}
          </div>
        ) : (
          <svg ref={svgRef} className="topology-svg" />
        )}
      </div>
    </div>
  );
};

export default NetworkTopology;
