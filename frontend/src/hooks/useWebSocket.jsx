/* eslint-disable react-refresh/only-export-components -- context/hook file intentionally co-exports its Provider and hook; splitting would ripple imports across the app with no runtime benefit */
import { createContext, useContext, useEffect, useRef, useCallback, useState } from 'react';
import { useAuth } from '../contexts/AuthContext';

const WebSocketContext = createContext(null);

/**
 * WebSocketProvider manages a single persistent WebSocket connection to /api/ws.
 * The connection is gated on the authenticated user from AuthContext: it opens
 * only when a user is present and is torn down on logout/session expiry, so
 * unauthenticated tabs don't loop on 401 handshakes.
 */
export function WebSocketProvider({ children }) {
  const { user } = useAuth();
  const wsRef = useRef(null);
  const [status, setStatus] = useState('disconnected'); // connected | reconnecting | degraded | disconnected
  const statusRef = useRef('disconnected');
  const updateStatus = useCallback((newStatus) => {
    statusRef.current = newStatus;
    setStatus(newStatus);
  }, []);
  const listenersRef = useRef(new Set());
  const topicsRef = useRef(new Map()); // topic -> refCount
  const refreshCallbacksRef = useRef(new Map()); // key -> callback
  const retryTimeoutRef = useRef(null);
  const retryCountRef = useRef(0);
  const mountedRef = useRef(true);
  const enabledRef = useRef(false);
  const degradedIntervalRef = useRef(null);

  // Reconnect parameters: exponential backoff (1s initial, 2x multiplier, 30s max, 25% jitter)
  const INITIAL_RETRY_DELAY = 1000;
  const MAX_RETRY_DELAY = 30000;
  const BACKOFF_MULTIPLIER = 2;
  const JITTER_FACTOR = 0.25;
  const DEGRADED_THRESHOLD = 5; // retries before entering degraded mode
  const DEGRADED_POLL_INTERVAL = 30000; // 30s polling fallback in degraded mode

  const sendJSON = useCallback((obj) => {
    const ws = wsRef.current;
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(obj));
    }
  }, []);

  const getRetryDelay = useCallback(() => {
    const base = Math.min(INITIAL_RETRY_DELAY * Math.pow(BACKOFF_MULTIPLIER, retryCountRef.current), MAX_RETRY_DELAY);
    const jitter = base * JITTER_FACTOR * (Math.random() * 2 - 1); // ±25% jitter
    return Math.max(INITIAL_RETRY_DELAY, Math.round(base + jitter));
  }, []);

  const startDegradedPolling = useCallback(() => {
    // Stop any existing degraded polling
    if (degradedIntervalRef.current) {
      clearInterval(degradedIntervalRef.current);
    }
    // Start polling all refresh callbacks as a fallback
    degradedIntervalRef.current = setInterval(() => {
      for (const cb of refreshCallbacksRef.current.values()) {
        try { cb(); } catch (e) { console.error('Degraded polling refresh error:', e); }
      }
    }, DEGRADED_POLL_INTERVAL);
  }, []);

  const stopDegradedPolling = useCallback(() => {
    if (degradedIntervalRef.current) {
      clearInterval(degradedIntervalRef.current);
      degradedIntervalRef.current = null;
    }
  }, []);

  const connect = useCallback(function connect() {
    if (!mountedRef.current || !enabledRef.current) return;

    // Build ws:// or wss:// URL from current location
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${proto}//${window.location.host}/api/ws`;

    try {
      const ws = new WebSocket(url);
      wsRef.current = ws;

      ws.onopen = () => {
        if (!mountedRef.current) return;
        const wasReconnecting = statusRef.current === 'reconnecting' || statusRef.current === 'degraded';
        updateStatus('connected');
        retryCountRef.current = 0;
        stopDegradedPolling();

        // Re-subscribe all active topics
        for (const [topic] of topicsRef.current) {
          sendJSON({ action: 'subscribe', topic });
        }

        // If reconnecting, call all refresh callbacks to catch up on missed events
        if (wasReconnecting) {
          for (const cb of refreshCallbacksRef.current.values()) {
            try { cb(); } catch (e) { console.error('Refresh callback error:', e); }
          }
        }
      };

      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          // Dispatch to all registered listeners
          for (const listener of listenersRef.current) {
            try { listener(data); } catch (e) { console.error('WS listener error:', e); }
          }
        } catch (e) {
          console.error('WS message parse error:', e);
        }
      };

      ws.onclose = () => {
        if (!mountedRef.current) return;
        wsRef.current = null;

        // Auth was revoked while connected: don't reconnect
        if (!enabledRef.current) {
          updateStatus('disconnected');
          return;
        }

        retryCountRef.current += 1;

        // After threshold retries, enter degraded mode with polling fallback
        if (retryCountRef.current >= DEGRADED_THRESHOLD) {
          updateStatus('degraded');
          startDegradedPolling();
        } else {
          updateStatus('reconnecting');
        }

        // Schedule reconnect with exponential backoff
        const delay = getRetryDelay();
        retryTimeoutRef.current = setTimeout(connect, delay);
      };

      ws.onerror = () => {
        // onclose will fire after onerror, which handles reconnection
      };
    } catch (e) {
      console.error('WebSocket connection error:', e);
      if (!enabledRef.current) return;
      retryCountRef.current += 1;

      if (retryCountRef.current >= DEGRADED_THRESHOLD) {
        updateStatus('degraded');
        startDegradedPolling();
      } else {
        updateStatus('reconnecting');
      }

      const delay = getRetryDelay();
      retryTimeoutRef.current = setTimeout(connect, delay);
    }
  }, [sendJSON, getRetryDelay, startDegradedPolling, stopDegradedPolling, updateStatus]);

  // Track provider mount/unmount for final cleanup.
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      if (retryTimeoutRef.current) {
        clearTimeout(retryTimeoutRef.current);
        retryTimeoutRef.current = null;
      }
      if (degradedIntervalRef.current) {
        clearInterval(degradedIntervalRef.current);
        degradedIntervalRef.current = null;
      }
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, []);

  // Open the connection only when authenticated; tear it down on logout.
  useEffect(() => {
    if (!user) {
      enabledRef.current = false;
      if (retryTimeoutRef.current) {
        clearTimeout(retryTimeoutRef.current);
        retryTimeoutRef.current = null;
      }
      stopDegradedPolling();
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
      retryCountRef.current = 0;
      updateStatus('disconnected');
      return;
    }
    enabledRef.current = true;
    retryCountRef.current = 0;
    connect();
  }, [user, connect, stopDegradedPolling, updateStatus]);

  const subscribe = useCallback((topic) => {
    const count = topicsRef.current.get(topic) || 0;
    topicsRef.current.set(topic, count + 1);
    if (count === 0) {
      sendJSON({ action: 'subscribe', topic });
    }
  }, [sendJSON]);

  const unsubscribe = useCallback((topic) => {
    const count = topicsRef.current.get(topic) || 0;
    if (count <= 1) {
      topicsRef.current.delete(topic);
      sendJSON({ action: 'unsubscribe', topic });
    } else {
      topicsRef.current.set(topic, count - 1);
    }
  }, [sendJSON]);

  const addListener = useCallback((fn) => {
    listenersRef.current.add(fn);
  }, []);

  const removeListener = useCallback((fn) => {
    listenersRef.current.delete(fn);
  }, []);

  const registerRefresh = useCallback((key, callback) => {
    refreshCallbacksRef.current.set(key, callback);
  }, []);

  const unregisterRefresh = useCallback((key) => {
    refreshCallbacksRef.current.delete(key);
  }, []);

  const value = {
    status,
    subscribe,
    unsubscribe,
    addListener,
    removeListener,
    registerRefresh,
    unregisterRefresh,
  };

  return (
    <WebSocketContext.Provider value={value}>
      {children}
    </WebSocketContext.Provider>
  );
}

/**
 * useWebSocket returns the WebSocket context value.
 * Must be used within a WebSocketProvider.
 */
export function useWebSocket() {
  const ctx = useContext(WebSocketContext);
  if (!ctx) {
    // Silently return a no-op object if used outside provider (e.g., login page)
    return {
      status: 'disconnected',
      subscribe: () => {},
      unsubscribe: () => {},
      addListener: () => {},
      removeListener: () => {},
      registerRefresh: () => {},
      unregisterRefresh: () => {},
    };
  }
  return ctx;
}
