import { useWebSocket } from '../hooks/useWebSocket';

/**
 * ConnectionStatus displays a small indicator when the WebSocket connection
 * is not in the "connected" state. Hidden when everything is normal.
 * States: connected (hidden), disconnected (hidden), reconnecting (warning), degraded (danger).
 */
export default function ConnectionStatus() {
  const { status } = useWebSocket();

  if (status === 'connected' || status === 'disconnected') {
    return null;
  }

  const isReconnecting = status === 'reconnecting';
  const isDegraded = status === 'degraded';

  return (
    <div style={{
      position: 'fixed',
      bottom: 16,
      right: 16,
      zIndex: 9999,
      display: 'flex',
      alignItems: 'center',
      gap: 8,
      padding: '8px 14px',
      borderRadius: 8,
      fontSize: '0.8rem',
      fontWeight: 500,
      color: isDegraded ? 'var(--text-danger, #d9534f)' : 'var(--text-warning, #f0ad4e)',
      background: 'var(--bg-elevated, #1e1e2e)',
      border: `1px solid ${isDegraded ? 'var(--border-danger, #d9534f33)' : 'var(--border-warning, #f0ad4e33)'}`,
      boxShadow: '0 2px 12px rgba(0,0,0,0.3)',
      transition: 'all 0.3s ease',
      opacity: 0,
      animation: 'fadeInStatus 0.3s ease forwards',
    }}>
      {isReconnecting && (
        <span style={{
          width: 12,
          height: 12,
          border: '2px solid currentColor',
          borderTopColor: 'transparent',
          borderRadius: '50%',
          animation: 'spin 0.8s linear infinite',
        }} />
      )}
      {isDegraded && (
        <span style={{ fontSize: '1rem', lineHeight: 1 }}>!</span>
      )}
      {isReconnecting && 'Reconnecting…'}
      {isDegraded && 'Live updates unavailable — using periodic refresh'}
      <style>{`
        @keyframes spin { to { transform: rotate(360deg); } }
        @keyframes fadeInStatus { from { opacity: 0; transform: translateY(8px); } to { opacity: 1; transform: translateY(0); } }
      `}</style>
    </div>
  );
}
