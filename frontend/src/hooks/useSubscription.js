import { useEffect, useRef, useCallback } from 'react';
import { useWebSocket } from './useWebSocket';

/**
 * useSubscription subscribes to a WebSocket topic and calls the handler
 * when a matching event is received.
 *
 * @param {string} topic - The topic to subscribe to (e.g., "run:abc-123", "runs:*")
 * @param {Function} handler - Called with the event object when a matching event arrives
 * @param {Object} [options] - Optional configuration
 * @param {number} [options.debounceMs=0] - Debounce window in ms. If > 0, rapid events
 *   within the window are held until the window closes.
 * @param {boolean} [options.buffer=false] - With debounceMs: replay every held event
 *   at flush (in order) instead of collapsing to the latest. Use when events carry
 *   deltas that must all be applied; collapse is only safe for full-state payloads.
 */
export function useSubscription(topic, handler, options = {}) {
  const { debounceMs = 0, buffer = false } = options;
  const { subscribe, unsubscribe, addListener, removeListener } = useWebSocket();
  const handlerRef = useRef(handler);
  const debounceTimerRef = useRef(null);
  const pendingEventsRef = useRef([]);

  // Keep handler ref current without re-subscribing
  useEffect(() => {
    handlerRef.current = handler;
  }, [handler]);

  const listener = useCallback((event) => {
    // Only handle events matching our topic
    if (!event.topic) return;
    if (!topicMatches(topic, event.topic)) return;

    if (debounceMs > 0) {
      if (buffer) {
        pendingEventsRef.current.push(event);
      } else {
        pendingEventsRef.current = [event];
      }
      if (!debounceTimerRef.current) {
        debounceTimerRef.current = setTimeout(() => {
          debounceTimerRef.current = null;
          const events = pendingEventsRef.current;
          pendingEventsRef.current = [];
          // React 18 batches the state updates from one flush into one render.
          for (const e of events) {
            handlerRef.current(e);
          }
        }, debounceMs);
      }
    } else {
      handlerRef.current(event);
    }
  }, [topic, debounceMs, buffer]);

  useEffect(() => {
    if (!topic) return;

    subscribe(topic);
    addListener(listener);

    return () => {
      removeListener(listener);
      unsubscribe(topic);
      if (debounceTimerRef.current) {
        clearTimeout(debounceTimerRef.current);
        debounceTimerRef.current = null;
      }
      pendingEventsRef.current = [];
    };
  }, [topic, subscribe, unsubscribe, addListener, removeListener, listener]);
}

/**
 * Check if a subscription topic matches an event topic.
 * Mirrors the server-side topicMatches logic.
 */
function topicMatches(subscription, eventTopic) {
  if (subscription === eventTopic) return true;

  if (subscription.endsWith(':*')) {
    const prefix = subscription.slice(0, -2);
    const eventPrefix = eventTopic.split(':')[0];
    if (eventPrefix === prefix) return true;
    // "runs:*" also matches "run:{id}" topics
    if (prefix === 'runs' && eventPrefix === 'run') return true;
  }

  return false;
}
