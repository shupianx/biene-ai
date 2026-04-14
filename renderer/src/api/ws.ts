import type {
  MessageAddedData,
  SkillActivatedData,
  StatusData,
  SessionDeletedData,
  SessionListEventType,
  SessionMetaEventData,
  TextDeltaData,
  ToolStartData,
  ToolResultData,
  ToolDeniedData,
  PermissionRequestData,
  PermissionClearedData,
  ErrorData,
  SessionEventType,
} from '../types/events'
import { buildCoreWebSocketUrl } from '../runtime'

export interface WSHandlers {
  onMessageAdded: (data: MessageAddedData) => void
  onSkillActivated: (data: SkillActivatedData) => void
  onStatus: (data: StatusData) => void
  onTextDelta: (data: TextDeltaData) => void
  onToolCompose: (data: ToolStartData) => void
  onToolStart: (data: ToolStartData) => void
  onToolResult: (data: ToolResultData) => void
  onToolDenied: (data: ToolDeniedData) => void
  onPermissionRequest: (data: PermissionRequestData) => void
  onPermissionCleared: (data: PermissionClearedData) => void
  onError: (data: ErrorData) => void
  onDone: () => void
  onReconnect?: () => void
}

interface WSMessage {
  type: SessionEventType
  data: unknown
}

interface SessionListWSMessage {
  type: SessionListEventType
  data: unknown
}

/** Opens a persistent WebSocket connection for a session. Returns a cleanup function. */
export function connectWS(sessionId: string, handlers: WSHandlers): () => void {
  const url = buildCoreWebSocketUrl(`/api/sessions/${sessionId}/ws`)
  let ws: WebSocket | null = null
  let reconnectTimer: number | null = null
  let reconnectDelayMs = 500
  let closed = false
  let hasOpened = false

  const dispatch = (message: WSMessage) => {
    switch (message.type) {
      case 'message_added':
        handlers.onMessageAdded(message.data as MessageAddedData)
        break
      case 'skill_activated':
        handlers.onSkillActivated(message.data as SkillActivatedData)
        break
      case 'status':
        handlers.onStatus(message.data as StatusData)
        break
      case 'text_delta':
        handlers.onTextDelta(message.data as TextDeltaData)
        break
      case 'tool_compose':
        handlers.onToolCompose(message.data as ToolStartData)
        break
      case 'tool_start':
        handlers.onToolStart(message.data as ToolStartData)
        break
      case 'tool_result':
        handlers.onToolResult(message.data as ToolResultData)
        break
      case 'tool_denied':
        handlers.onToolDenied(message.data as ToolDeniedData)
        break
      case 'permission_request':
        handlers.onPermissionRequest(message.data as PermissionRequestData)
        break
      case 'permission_cleared':
        handlers.onPermissionCleared(message.data as PermissionClearedData)
        break
      case 'error':
        handlers.onError(message.data as ErrorData)
        break
      case 'done':
        handlers.onDone()
        break
    }
  }

  const scheduleReconnect = () => {
    if (closed || reconnectTimer !== null) return
    reconnectTimer = window.setTimeout(() => {
      reconnectTimer = null
      open()
    }, reconnectDelayMs)
    reconnectDelayMs = Math.min(reconnectDelayMs * 2, 3000)
  }

  const open = () => {
    console.log(`[WS] connecting: ${url}`)
    ws = new WebSocket(url)

    ws.onopen = () => {
      console.log(`[WS] connected: ${sessionId}`)
      reconnectDelayMs = 500
      if (hasOpened) {
        handlers.onReconnect?.()
      }
      hasOpened = true
    }

    ws.onmessage = (event) => {
      try {
        dispatch(JSON.parse(event.data) as WSMessage)
      } catch (err) {
        console.warn(`[WS] invalid message for ${sessionId}:`, err)
      }
    }

    ws.onerror = (event) => {
      console.warn(`[WS] error for ${sessionId}`, event)
    }

    ws.onclose = (event) => {
      ws = null
      if (closed) return
      console.warn(`[WS] closed for ${sessionId}, code=${event.code}, reason=${event.reason || 'n/a'}`)
      scheduleReconnect()
    }
  }

  open()

  return () => {
    closed = true
    if (reconnectTimer !== null) {
      window.clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    ws?.close()
    ws = null
  }
}

export interface SessionListWSHandlers {
  onSessionCreated: (data: SessionMetaEventData) => void
  onSessionUpdated: (data: SessionMetaEventData) => void
  onSessionDeleted: (data: SessionDeletedData) => void
  onOpen?: () => void
  onReconnect?: () => void
}

export function connectSessionListWS(handlers: SessionListWSHandlers): () => void {
  const url = buildCoreWebSocketUrl('/api/sessions/ws')
  let ws: WebSocket | null = null
  let reconnectTimer: number | null = null
  let reconnectDelayMs = 500
  let closed = false
  let hasOpened = false

  const dispatch = (message: SessionListWSMessage) => {
    switch (message.type) {
      case 'session_created':
        handlers.onSessionCreated(message.data as SessionMetaEventData)
        break
      case 'session_updated':
        handlers.onSessionUpdated(message.data as SessionMetaEventData)
        break
      case 'session_deleted':
        handlers.onSessionDeleted(message.data as SessionDeletedData)
        break
    }
  }

  const scheduleReconnect = () => {
    if (closed || reconnectTimer !== null) return
    reconnectTimer = window.setTimeout(() => {
      reconnectTimer = null
      open()
    }, reconnectDelayMs)
    reconnectDelayMs = Math.min(reconnectDelayMs * 2, 3000)
  }

  const open = () => {
    console.log(`[WS] connecting session list: ${url}`)
    ws = new WebSocket(url)

    ws.onopen = () => {
      console.log('[WS] connected session list')
      reconnectDelayMs = 500
      handlers.onOpen?.()
      if (hasOpened) {
        handlers.onReconnect?.()
      }
      hasOpened = true
    }

    ws.onmessage = (event) => {
      try {
        dispatch(JSON.parse(event.data) as SessionListWSMessage)
      } catch (err) {
        console.warn('[WS] invalid session list message:', err)
      }
    }

    ws.onerror = (event) => {
      console.warn('[WS] session list error', event)
    }

    ws.onclose = (event) => {
      ws = null
      if (closed) return
      console.warn(`[WS] session list closed, code=${event.code}, reason=${event.reason || 'n/a'}`)
      scheduleReconnect()
    }
  }

  open()

  return () => {
    closed = true
    if (reconnectTimer !== null) {
      window.clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    ws?.close()
    ws = null
  }
}
