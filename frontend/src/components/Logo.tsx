import React from 'react';

interface LogoProps {
  size?: number;
  showText?: boolean;
  variant?: 'full' | 'icon';
}

const Logo: React.FC<LogoProps> = ({ size = 120, showText = true, variant = 'full' }) => {
  const iconSize = variant === 'icon' ? size : size;
  
  return (
    <div style={{ 
      display: 'flex', 
      flexDirection: 'column', 
      alignItems: 'center', 
      gap: showText ? 12 : 0
    }}>
      {/* Speedometer ring */}
      <div style={{
        width: iconSize,
        height: iconSize,
        position: 'relative',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center'
      }}>
        {/* Background ring - gray */}
        <div style={{
          width: '100%',
          height: '100%',
          borderRadius: '50%',
          border: `${iconSize * 0.1}px solid #4B5563`,
          position: 'absolute',
          boxSizing: 'border-box'
        }} />
        
        {/* Orange segment - top left arc */}
        <div style={{
          width: '100%',
          height: '100%',
          borderRadius: '50%',
          border: `${iconSize * 0.1}px solid transparent`,
          borderTopColor: '#F97316',
          borderLeftColor: '#FB923C',
          position: 'absolute',
          transform: 'rotate(-45deg)',
          boxSizing: 'border-box'
        }} />
        
        {/* Needle - pointing to top right */}
        <div style={{
          width: iconSize * 0.45,
          height: 2.5,
          background: 'linear-gradient(90deg, rgba(249, 115, 22, 0.3), #F97316)',
          position: 'absolute',
          transformOrigin: 'left center',
          transform: 'rotate(-55deg)',
          left: '50%',
          top: '50%',
          borderRadius: 2
        }} />
        
        {/* Center dot - orange */}
        <div style={{
          width: iconSize * 0.16,
          height: iconSize * 0.16,
          background: '#F97316',
          borderRadius: '50%',
          position: 'absolute',
          border: `${iconSize * 0.02}px solid #1e293b`,
          boxSizing: 'border-box',
          zIndex: 10
        }} />
      </div>
      
      {/* Text */}
      {showText && (
        <div style={{ 
          textAlign: 'center',
          fontFamily: '"Inter", "SF Pro Display", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif'
        }}>
          <div style={{ 
            fontSize: size * 0.22,
            fontWeight: 700,
            color: 'white',
            letterSpacing: '0.5px',
            lineHeight: 1.1,
            textShadow: '0 2px 4px rgba(0,0,0,0.3)',
            textTransform: 'uppercase'
          }}>
            iPerf3
          </div>
          <div style={{ 
            fontSize: size * 0.16,
            fontWeight: 600,
            color: 'rgba(255,255,255,0.9)',
            letterSpacing: '1px',
            marginTop: '2px',
            textShadow: '0 2px 4px rgba(0,0,0,0.3)',
            textTransform: 'uppercase'
          }}>
            Tracker
          </div>
        </div>
      )}
    </div>
  );
};

export default Logo;
