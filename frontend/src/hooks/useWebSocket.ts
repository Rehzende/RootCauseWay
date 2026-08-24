import { useEffect, useRef, useState, useCallback } from 'react';

export interface WSEvent {
  type: string;
  incident_id?: string;
  data: unknown;
  timestamp: string;
}

interface UseWebSocketReturn {
  lastEvent: WSEvent | null;
  isConnected: boolean;
  subscribe: (topics: string[]) => void;
  unsubscribe: (topics: string[]) => void;
}

export function useWebSocket(topics?: string[]): UseWebSocketReturn {
  const [lastEvent, setLastEvent] = useState<WSEvent | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout>>();
  const reconnectDelayRef = useRef(1000);
  const mountedRef = useRef(true);
  const topicsRef = useRef<string[]>(topics ?? []);

  const sendMessage = useCallback((msg: Record<string, unknown>) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(msg));
    }
  }, []);

  const subscribe = useCallback((newTopics: string[]) => {
    topicsRef.current = [...new Set([...topicsRef.current, ...newTopics])];
    sendMessage({ action: 'subscribe', topics: newTopics });
  }, [sendMessage]);

  const unsubscribe = useCallback((removeTopics: string[]) => {
    topicsRef.current = topicsRef.current.filter((t) => !removeTopics.includes(t));
    sendMessage({ action: 'unsubscribe', topics: removeTopics });
  }, [sendMessage]);

  useEffect(() => {
    mountedRef.current = true;

    function connect() {
      const token = localStorage.getItem('rootcauseway_token');
      if (!token) return;

      // Same-origin, relative to whatever host actually served this page --
      // a hardcoded "localhost:8080" here resolves against the *browser's*
      // own machine, not the server, so it silently never connects outside
      // local dev. nginx (frontend/nginx.conf) proxies /ws to the backend in
      // prod; vite.config.ts's dev server proxy does the same in dev.
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = `${protocol}//${window.location.host}/ws?token=${encodeURIComponent(token)}`;
      const ws = new WebSocket(wsUrl);
      wsRef.current = ws;

      ws.onopen = () => {
        if (!mountedRef.current) return;
        setIsConnected(true);
        reconnectDelayRef.current = 1000;
        // Subscribe to initial topics
        if (topicsRef.current.length > 0) {
          sendMessage({ action: 'subscribe', topics: topicsRef.current });
        }
      };

      ws.onmessage = (event) => {
        if (!mountedRef.current) return;
        try {
          const parsed = JSON.parse(event.data) as WSEvent;
          setLastEvent(parsed);
        } catch {
          // Ignore non-JSON messages
        }
      };

      ws.onclose = () => {
        if (!mountedRef.current) return;
        setIsConnected(false);
        wsRef.current = null;
        // Exponential backoff reconnect
        const delay = Math.min(reconnectDelayRef.current, 30000);
        reconnectTimeoutRef.current = setTimeout(() => {
          if (mountedRef.current) {
            reconnectDelayRef.current = delay * 2;
            connect();
          }
        }, delay);
      };

      ws.onerror = () => {
        ws.close();
      };
    }

    connect();

    return () => {
      mountedRef.current = false;
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // Update subscriptions when topics prop changes
  useEffect(() => {
    if (topics) {
      topicsRef.current = topics;
      if (wsRef.current?.readyState === WebSocket.OPEN) {
        sendMessage({ action: 'subscribe', topics });
      }
    }
  }, [topics?.join(',')]); // eslint-disable-line react-hooks/exhaustive-deps

  return { lastEvent, isConnected, subscribe, unsubscribe };
}
