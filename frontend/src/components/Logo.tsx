import React from 'react';

interface LogoProps {
  size?: number;
  showText?: boolean;
  variant?: 'full' | 'icon';
}

const Logo: React.FC<LogoProps> = ({ size = 120, showText = true, variant = 'full' }) => {
  const iconSize = variant === 'icon' ? size : size;
  
  // Keyframes for needle animation
  const needleAnimation = `
    @keyframes needleSweep {
      0% {
        transform: rotate(-135deg);
      }
      60% {
        transform: rotate(-55deg);
      }
      70% {
        transform: rotate(-60deg);
      }
      80% {
        transform: rotate(-55deg);
      }
      100% {
        transform: rotate(-55deg);
      }
    }
  `;
  
  return (
    <div style={{ 
      display: 'flex', 
      flexDirection: 'column', 
      alignItems: 'center', 
      gap: showText ? 12 : 0
    }}>
      <style>{needleAnimation}</style>
      {/* Speedometer ring */}
      <div style={{
        width: iconSize,
        height: iconSize,
        position: 'relative',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        filter: 'drop-shadow(0 4px 12px rgba(0, 0, 0, 0.15))'
      }}>
        {/* Background ring - gray with soft shadow */}
        <div style={{
          width: '100%',
          height: '100%',
          borderRadius: '50%',
          border: `${iconSize * 0.1}px solid #4B5563`,
          position: 'absolute',
          boxSizing: 'border-box',
          boxShadow: 'inset 0 2px 8px rgba(0, 0, 0, 0.1)'
        }} />
        
        {/* Orange segment - top left arc with glow */}
        <div style={{
          width: '100%',
          height: '100%',
          borderRadius: '50%',
          border: `${iconSize * 0.1}px solid transparent`,
          borderTopColor: '#F97316',
          borderLeftColor: '#FB923C',
          position: 'absolute',
          transform: 'rotate(-45deg)',
          boxSizing: 'border-box',
          filter: 'drop-shadow(0 0 8px rgba(249, 115, 22, 0.4))'
        }} />
        
        {/* Needle - pointing to top right with glow */}
        <div style={{
          width: iconSize * 0.45,
          height: 2.5,
          background: 'linear-gradient(90deg, rgba(249, 115, 22, 0.3), #F97316)',
          position: 'absolute',
          transformOrigin: 'left center',
          transform: 'rotate(-55deg)',
          left: '50%',
          top: '50%',
          borderRadius: 2,
          boxShadow: '0 2px 6px rgba(249, 115, 22, 0.3)',
          animation: 'needleSweep 2s ease-out forwards'
        }} />
        
        {/* Center dot - orange with soft shadow */}
        <div style={{
          width: iconSize * 0.16,
          height: iconSize * 0.16,
          background: 'linear-gradient(135deg, #FB923C 0%, #F97316 100%)',
          borderRadius: '50%',
          position: 'absolute',
          border: `${iconSize * 0.02}px solid #1e293b`,
          boxSizing: 'border-box',
          zIndex: 10,
          boxShadow: '0 2px 8px rgba(249, 115, 22, 0.4), inset 0 1px 2px rgba(255, 255, 255, 0.3)'
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
            fontWeight: 600,
            color: 'white',
            letterSpacing: '1px',
            lineHeight: 1.1,
            textShadow: '0 2px 8px rgba(0,0,0,0.3), 0 4px 16px rgba(0,0,0,0.2)',
            textTransform: 'uppercase'
          }}>
            iPerf3
          </div>
          <div style={{ 
            fontSize: size * 0.16,
            fontWeight: 500,
            color: 'rgba(255,255,255,0.85)',
            letterSpacing: '1.5px',
            marginTop: '2px',
            textShadow: '0 2px 8px rgba(0,0,0,0.3), 0 4px 16px rgba(0,0,0,0.2)',
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
