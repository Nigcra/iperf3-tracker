import React, { useEffect, useState } from 'react';
import { getTestLiveStatus, TestLiveStatus } from '../services/api';
import './LiveTestDisplay.css';

interface LiveTestDisplayProps {
  testId: number;
  onComplete?: () => void;
}

const LiveTestDisplay: React.FC<LiveTestDisplayProps> = ({ testId, onComplete }) => {
  const [liveData, setLiveData] = useState<TestLiveStatus | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [completionScheduled, setCompletionScheduled] = useState(false);

  useEffect(() => {
    let intervalId: NodeJS.Timeout;

    const fetchLiveStatus = async () => {
      try {
        const status = await getTestLiveStatus(testId);
        setLiveData(status);

        // If test is no longer running, stop polling and call onComplete after showing final results
        if (!status.is_running && onComplete && !completionScheduled) {
          setCompletionScheduled(true);
          clearInterval(intervalId);
          // Keep final results visible for 5 seconds before calling onComplete
          setTimeout(onComplete, 5000);
        }
      } catch (err) {
        setError('Failed to fetch live status');
        clearInterval(intervalId);
      }
    };

    // Initial fetch
    fetchLiveStatus();

    // Poll every 500ms for live updates
    intervalId = setInterval(fetchLiveStatus, 500);

    return () => {
      if (intervalId) {
        clearInterval(intervalId);
      }
    };
  }, [testId, onComplete, completionScheduled]);

  if (error) {
    return <div className="live-test-error">{error}</div>;
  }

  if (!liveData) {
    return <div className="live-test-loading">Loading test status...</div>;
  }

  const progressPercent = liveData.progress;
  const timeRemaining = liveData.total_seconds - liveData.elapsed_seconds;

  return (
    <div className="live-test-container">
      <div className="live-test-header">
        <h3>
          {liveData.is_running ? '🔴 ' : '✅ '}
          Testing against {liveData.server_name}...
        </h3>
        <div className="live-indicator">
          {liveData.is_running ? (
            <>
              <span className="pulse-dot"></span>
              <span>LIVE</span>
            </>
          ) : (
            <span>FINISHED</span>
          )}
        </div>
      </div>

      <div className="progress-section">
        <div className="progress-bar-container">
          <div 
            className="progress-bar-fill" 
            style={{ width: `${progressPercent}%` }}
          >
            <span className="progress-text">{progressPercent}%</span>
          </div>
        </div>
        <div className="time-info">
          <span>{liveData.elapsed_seconds}s elapsed</span>
          <span>{timeRemaining}s remaining</span>
        </div>
      </div>

      <div className="live-metrics">
        <div className="live-metric download">
          <div className="metric-icon">⬇️</div>
          <div className="metric-content">
            <div className="metric-label">Download</div>
            <div className="metric-value">
              {liveData.current_download_mbps.toFixed(2)}
              <span className="metric-unit">Mbps</span>
            </div>
          </div>
        </div>

        <div className="live-metric upload">
          <div className="metric-icon">⬆️</div>
          <div className="metric-content">
            <div className="metric-label">Upload</div>
            <div className="metric-value">
              {liveData.current_upload_mbps.toFixed(2)}
              <span className="metric-unit">Mbps</span>
            </div>
          </div>
        </div>
      </div>

      <div className="live-status">
        Status: <strong>{liveData.status}</strong>
      </div>
    </div>
  );
};

export default LiveTestDisplay;
