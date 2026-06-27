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
 *   within the window are collapsed and only the latest is delivered.
 */
export function useSubscription(topic, handler, options = {}) {
  const { debounceMs = 0 } = options;
  const { subscribe, unsubscribe, addListener, removeListener } = useWebSocket();
  const handlerRef = useRef(handler);
  const debounceTimerRef = useRef(null);
  const latestEventRef = useRef(null);

  // Keep handler ref current without re-subscribing
  useEffect(() => {
    handlerRef.current = handler;
  }, [handler]);

  const listener = useCallback((event) => {
    // Only handle events matching our topic
    if (!event.topic) return;
    if (!topicMatches(topic, event.topic)) return;

    if (debounceMs > 0) {
      latestEventRef.current = event;
      if (!debounceTimerRef.current) {
        debounceTimerRef.current = setTimeout(() => {
          debounceTimerRef.current = null;
          if (latestEventRef.current) {
            handlerRef.current(latestEventRef.current);
            latestEventRef.current = null;
          }
        }, debounceMs);
      }
    } else {
      handlerRef.current(event);
    }
  }, [topic, debounceMs]);

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
