import { useEffect, useRef, useCallback } from 'react';
import type { DashboardEvent } from '../types/events';

interface UseWebSocketOptions {
  url: string;
  onMessage: (event: DashboardEvent) => void;
  onConnect?: () => void;
  onDisconnect?: () => void;
  onError?: (error: Event) => void;
  reconnectInterval?: number;
  reconnectAttempts?: number;
}

export const useWebSocket = ({
  url,
  onMessage,
  onConnect,
  onDisconnect,
  onError,
  reconnectInterval = 5000,
  reconnectAttempts = 3,
}: UseWebSocketOptions) => {
  const ws = useRef<WebSocket | null>(null);
  const reconnectCount = useRef(0);
  const reconnectTimeout = useRef<number | undefined>(undefined);
  const shouldReconnect = useRef(true);
  const mounted = useRef(true);

  const connect = useCallback(() => {
    if (!mounted.current || !shouldReconnect.current) {
      return;
    }

    try {
      console.log('[WebSocket] Connecting to', url);
      ws.current = new WebSocket(url);

      ws.current.onopen = () => {
        console.log('[WebSocket] Connected');
        reconnectCount.current = 0;
        onConnect?.();
      };

      ws.current.onmessage = (event) => {
        try {
          const data: DashboardEvent = JSON.parse(event.data);
          console.log('[WebSocket] Message received:', data.type);
          onMessage(data);
        } catch (error) {
          console.error('[WebSocket] Failed to parse message:', error);
        }
      };

      ws.current.onerror = (error) => {
        console.error('[WebSocket] Error:', error);
        onError?.(error);
      };

      ws.current.onclose = () => {
        console.log('[WebSocket] Disconnected');
        onDisconnect?.();

        // Attempt to reconnect only if mounted and should reconnect
        if (mounted.current && shouldReconnect.current && reconnectCount.current < reconnectAttempts) {
          reconnectCount.current++;
          console.log(`[WebSocket] Reconnecting in ${reconnectInterval}ms... (attempt ${reconnectCount.current}/${reconnectAttempts})`);

          reconnectTimeout.current = setTimeout(() => {
            if (mounted.current) {
              connect();
            }
          }, reconnectInterval);
        } else if (reconnectCount.current >= reconnectAttempts) {
          console.error('[WebSocket] Max reconnection attempts reached. Backend might be offline.');
        }
      };
    } catch (error) {
      console.error('[WebSocket] Failed to create connection:', error);
    }
  }, [url, onMessage, onConnect, onDisconnect, onError, reconnectInterval, reconnectAttempts]);

  const disconnect = useCallback(() => {
    console.log('[WebSocket] Manually disconnecting');
    shouldReconnect.current = false;

    if (reconnectTimeout.current) {
      clearTimeout(reconnectTimeout.current);
    }

    if (ws.current) {
      ws.current.close();
      ws.current = null;
    }
  }, []);

  useEffect(() => {
    mounted.current = true;
    shouldReconnect.current = true;
    connect();

    return () => {
      mounted.current = false;
      disconnect();
    };
  }, [url]);

  return {
    disconnect,
    reconnect: () => {
      disconnect();
      shouldReconnect.current = true;
      connect();
    },
  };
};
