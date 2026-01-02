'use client'

import { createContext, useContext, useEffect, useState, useRef, useCallback } from 'react'
import { useJobStore } from '@/stores/useJobStore'
import { useQueryClient } from '@tanstack/react-query'
import { Job, JobStatus, PlaylistUpdatePayload } from '@/lib/types'
import { getConfig, getWsUrl as getWsUrlFromConfig } from '@/lib/config'

interface WebSocketContextType {
  isConnected: boolean
}

interface WebSocketPayload {
  job_id?: string
  id?: string
  progress?: number
  status?: string
  song_status?: string
  lyrics_status?: string
  cover_status?: string
  error_message?: string
  [key: string]: unknown
}

interface WebSocketMessage {
  type: 'job_update' | 'batch_update' | 'playlist_update' | 'error'
  payload: WebSocketPayload | PlaylistUpdatePayload
}

const WebSocketContext = createContext<WebSocketContextType>({ isConnected: false })

// Exponential backoff configuration
const INITIAL_RECONNECT_DELAY = 1000 // 1 second
const MAX_RECONNECT_DELAY = 30000 // 30 seconds
const RECONNECT_MULTIPLIER = 2

export function WebSocketProvider({ children }: { children: React.ReactNode }) {
  const [isConnected, setIsConnected] = useState(false)
  const { updateJob } = useJobStore()
  const queryClient = useQueryClient()
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null)
  const reconnectDelayRef = useRef(INITIAL_RECONNECT_DELAY)
  const isUnmountedRef = useRef(false)
  const currentWsUrlRef = useRef<string | null>(null)
  const wsBaseUrlRef = useRef<string | null>(null)

  const getToken = useCallback(() => {
    if (typeof window === 'undefined') return null
    return localStorage.getItem('token')
  }, [])

  const clearReconnectTimeout = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current)
      reconnectTimeoutRef.current = null
    }
  }, [])

  const scheduleReconnect = useCallback(() => {
    if (isUnmountedRef.current) return

    clearReconnectTimeout()
    
    const delay = reconnectDelayRef.current
    console.log(`[WebSocket] Scheduling reconnect in ${delay}ms`)
    
    reconnectTimeoutRef.current = setTimeout(() => {
      if (!isUnmountedRef.current) {
        connectWithConfig()
      }
    }, delay)

    // Increase delay for next attempt (exponential backoff)
    reconnectDelayRef.current = Math.min(
      reconnectDelayRef.current * RECONNECT_MULTIPLIER,
      MAX_RECONNECT_DELAY
    )
  }, [clearReconnectTimeout])

  const connect = useCallback((wsBaseUrl: string) => {
    // Store the base URL for future connections
    wsBaseUrlRef.current = wsBaseUrl

    // Clean up existing connection
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }

    const token = getToken()
    if (!token) {
      console.log('[WebSocket] No token available, skipping connection')
      currentWsUrlRef.current = null
      return
    }

    const url = `${wsBaseUrl}/api/v1/ws?token=${encodeURIComponent(token)}`

    // Store the URL being used for this connection attempt
    currentWsUrlRef.current = url

    console.log('[WebSocket] Connecting...')
    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onopen = () => {
      console.log('[WebSocket] Connected')
      setIsConnected(true)
      // Reset reconnect delay on successful connection
      reconnectDelayRef.current = INITIAL_RECONNECT_DELAY
    }

    ws.onmessage = (event) => {
      try {
        const message: WebSocketMessage = JSON.parse(event.data)
        
        if (message.type === 'job_update' && message.payload) {
          const payload = message.payload as WebSocketPayload
          const jobId = payload.job_id || payload.id
          if (jobId) {
            updateJob(jobId, {
              id: jobId,
              progress: payload.progress,
              status: payload.status as JobStatus | undefined,
              song_status: payload.song_status as Job['song_status'],
              lyrics_status: payload.lyrics_status as Job['lyrics_status'],
              cover_status: payload.cover_status as Job['cover_status'],
            })
            // Invalidate React Query cache to trigger UI update
            if (payload.batch_id) {
              queryClient.invalidateQueries({ queryKey: ['batch', payload.batch_id] })
            }
          }
        } else if (message.type === 'batch_update') {
          console.log('[WebSocket] Batch update:', message.payload)
          // Invalidate batch queries to update the UI
          if (message.payload.batch_id) {
            queryClient.invalidateQueries({ queryKey: ['batch', message.payload.batch_id] })
          }
          queryClient.invalidateQueries({ queryKey: ['batches'] })
        } else if (message.type === 'playlist_update') {
          console.log('[WebSocket] Playlist update:', message.payload)
          const payload = message.payload as PlaylistUpdatePayload
          // Invalidate batch queries to update the UI with new playlist status
          if (payload.batch_id) {
            queryClient.invalidateQueries({ queryKey: ['batch', payload.batch_id] })
          }
          // Also invalidate batches list to show playlist status changes
          queryClient.invalidateQueries({ queryKey: ['batches'] })
        } else if (message.type === 'error') {
          console.error('[WebSocket] Server error:', message.payload)
        }
      } catch (error) {
        console.error('[WebSocket] Failed to parse message:', error)
      }
    }

    ws.onclose = (event) => {
      console.log(`[WebSocket] Disconnected (code: ${event.code}, reason: ${event.reason})`)
      setIsConnected(false)
      wsRef.current = null

      // Only reconnect if not unmounted and not a clean close (1000) or auth failure (4001/4003)
      const authFailureCodes = [4001, 4003]
      if (!isUnmountedRef.current && event.code !== 1000 && !authFailureCodes.includes(event.code)) {
        scheduleReconnect()
      }
    }

    ws.onerror = () => {
      // Browser WebSocket errors are intentionally opaque for security
      // Use the stored URL from connection attempt, not a recalculated one
      // (token may have changed since connection was initiated)
      const attemptedUrl = currentWsUrlRef.current
      console.error('[WebSocket] Connection error')
      console.error('[WebSocket] Attempted URL:', attemptedUrl?.replace(/token=[^&]+/, 'token=***') ?? 'unknown')
      console.error('[WebSocket] Possible causes: backend not running, CORS blocked, or network issue')
      setIsConnected(false)
    }
  }, [getToken, updateJob, scheduleReconnect, queryClient])

  // Wrapper that fetches config before connecting
  const connectWithConfig = useCallback(async () => {
    try {
      const config = await getConfig()
      const wsBaseUrl = config.apiUrl.replace(/^http/, 'ws')
      connect(wsBaseUrl)
    } catch (error) {
      console.error('[WebSocket] Failed to get config:', error)
      // Fall back to default
      connect(getWsUrlFromConfig())
    }
  }, [connect])

  // Listen for storage events to detect token changes (login/logout in other tabs)
  useEffect(() => {
    const handleStorageChange = (event: StorageEvent) => {
      if (event.key === 'token') {
        if (event.newValue) {
          // Token was added/changed, reconnect
          console.log('[WebSocket] Token changed, reconnecting...')
          reconnectDelayRef.current = INITIAL_RECONNECT_DELAY
          connectWithConfig()
        } else {
          // Token was removed, disconnect
          console.log('[WebSocket] Token removed, disconnecting...')
          clearReconnectTimeout()
          if (wsRef.current) {
            wsRef.current.close(1000, 'Logged out')
            wsRef.current = null
          }
          setIsConnected(false)
        }
      }
    }

    window.addEventListener('storage', handleStorageChange)
    return () => window.removeEventListener('storage', handleStorageChange)
  }, [connectWithConfig, clearReconnectTimeout])

  // Main connection effect - wait for runtime config before connecting
  useEffect(() => {
    isUnmountedRef.current = false
    
    // Only connect if we have a token
    if (getToken()) {
      // Fetch runtime config then connect
      connectWithConfig()
    }

    return () => {
      isUnmountedRef.current = true
      clearReconnectTimeout()
      if (wsRef.current) {
        wsRef.current.close(1000, 'Component unmounted')
        wsRef.current = null
      }
    }
  }, [connectWithConfig, clearReconnectTimeout, getToken])

  return (
    <WebSocketContext.Provider value={{ isConnected }}>
      {children}
    </WebSocketContext.Provider>
  )
}

export function useWebSocketContext() {
  const context = useContext(WebSocketContext)
  if (!context) {
    throw new Error('useWebSocketContext must be used within a WebSocketProvider')
  }
  return context
}
