import { useEffect, useRef, useState, useCallback } from 'react'

export interface WsEvent {
  type: string
  payload: Record<string, unknown>
}

interface UseWebSocketOptions {
  onEvent?: (event: WsEvent) => void
  reconnectInterval?: number
}

export function useWebSocket(options: UseWebSocketOptions = {}) {
  const { onEvent, reconnectInterval = 5000 } = options
  const [connected, setConnected] = useState(false)
  const [events, setEvents] = useState<WsEvent[]>([])
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const onEventRef = useRef(onEvent)

  onEventRef.current = onEvent

  const connect = useCallback(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    const url = `${protocol}//${host}/ws`

    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onopen = () => {
      setConnected(true)
    }

    ws.onmessage = (e) => {
      try {
        const event = JSON.parse(e.data) as WsEvent
        setEvents((prev) => [event, ...prev].slice(0, 50))
        onEventRef.current?.(event)
      } catch {
        // ignore non-JSON messages
      }
    }

    ws.onclose = () => {
      setConnected(false)
      wsRef.current = null
      reconnectTimerRef.current = setTimeout(connect, reconnectInterval)
    }

    ws.onerror = () => {
      ws.close()
    }
  }, [reconnectInterval])

  const disconnect = useCallback(() => {
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current)
      reconnectTimerRef.current = null
    }
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }
    setConnected(false)
  }, [])

  useEffect(() => {
    connect()
    return () => {
      disconnect()
    }
  }, [connect, disconnect])

  return { connected, events, disconnect, reconnect: connect }
}
