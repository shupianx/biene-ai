import type {
  MessageAddedData,
  StatusData,
  SessionDeletedData,
  SessionListEventType,
  SessionMetaEventData,
  ReasoningDeltaData,
  TextDeltaData,
  ToolStartData,
  ToolResultData,
  ToolDeniedData,
  PermissionRequestData,
  PermissionClearedData,
  SkillActivatedData,
  ProcessStateData,
  SessionProcessStateData,
  ToolComposeProgressData,
  CompactionStartData,
  CompactionDoneData,
  CompactionFailedData,
  ErrorData,
  SessionEventType,
} from '../types/events'
import { buildCoreWebSocketUrl } from '../runtime'

export interface WSHandlers {
  onMessageAdded: (data: MessageAddedData) => void
  onStatus: (data: StatusData) => void
  onReasoningDelta: (data: ReasoningDeltaData) => void
  onTextDelta: (data: TextDeltaData) => void
  onToolCompose: (data: ToolStartData) => void
  onToolComposeProgress: (data: ToolComposeProgressData) => void
  onToolStart: (data: ToolStartData) => void
  onToolResult: (data: ToolResultData) => void
  onToolDenied: (data: ToolDeniedData) => void
  onPermissionRequest: (data: PermissionRequestData) => void
  onPermissionCleared: (data: PermissionClearedData) => void
  onSkillActivated: (data: SkillActivatedData) => void
  onProcessState: (data: ProcessStateData) => void
  onCompactionStart?: (data: CompactionStartData) => void
  onCompactionDone?: (data: CompactionDoneData) => void
  onCompactionFailed?: (data: CompactionFailedData) => void
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
      case 'status':
        handlers.onStatus(message.data as StatusData)
        break
      case 'reasoning_delta':
        handlers.onReasoningDelta(message.data as ReasoningDeltaData)
        break
      case 'text_delta':
        handlers.onTextDelta(message.data as TextDeltaData)
        break
      case 'tool_compose':
        handlers.onToolCompose(message.data as ToolStartData)
        break
      case 'tool_compose_progress':
        handlers.onToolComposeProgress(message.data as ToolComposeProgressData)
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
      case 'skill_activated':
        handlers.onSkillActivated(message.data as SkillActivatedData)
        break
      case 'process_state':
        handlers.onProcessState(message.data as ProcessStateData)
        break
      case 'compaction_start':
        handlers.onCompactionStart?.(message.data as CompactionStartData)
        break
      case 'compaction_done':
        handlers.onCompactionDone?.(message.data as CompactionDoneData)
        break
      case 'compaction_failed':
        handlers.onCompactionFailed?.(message.data as CompactionFailedData)
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

export interface ProcessTerminalHandlers {
  onOutput: (chunk: Uint8Array) => void
  onState: (state: ProcessStateData) => void
  onError?: (event: Event) => void
}

export interface ProcessTerminalHandle {
  close: () => void
  writeInput: (data: string | Uint8Array) => void
  resize: (cols: number, rows: number) => void
}

interface ProcessLogFrame {
  output?: string
  state?: ProcessStateData
}

function encodeBase64(input: string | Uint8Array): string {
  const bytes = typeof input === 'string' ? new TextEncoder().encode(input) : input
  let bin = ''
  for (let i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i])
  return btoa(bin)
}

function decodeBase64(b64: string): Uint8Array {
  const bin = atob(b64)
  const bytes = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i)
  return bytes
}

/** Opens a bidirectional WebSocket to the session's background-process PTY.
 *  Output chunks arrive as Uint8Array (raw PTY bytes, potentially with ANSI
 *  escape sequences); the caller can push keystrokes via writeInput and the
 *  terminal's visible dimensions via resize. */
export function connectProcessLogsWS(
  sessionId: string,
  handlers: ProcessTerminalHandlers,
): ProcessTerminalHandle {
  const url = buildCoreWebSocketUrl(`/api/sessions/${sessionId}/process/logs/ws`)
  let closed = false
  let ready = false
  const pendingSends: string[] = []
  const ws = new WebSocket(url)

  const send = (frame: unknown) => {
    const payload = JSON.stringify(frame)
    if (!ready) {
      pendingSends.push(payload)
      return
    }
    try {
      ws.send(payload)
    } catch (err) {
      console.warn(`[process-terminal WS] send failed for ${sessionId}:`, err)
    }
  }

  ws.onopen = () => {
    ready = true
    while (pendingSends.length > 0) {
      const payload = pendingSends.shift()!
      try {
        ws.send(payload)
      } catch (err) {
        console.warn(`[process-terminal WS] flush failed for ${sessionId}:`, err)
        break
      }
    }
  }

  ws.onmessage = (event) => {
    try {
      const frame = JSON.parse(event.data) as ProcessLogFrame
      if (frame.output) {
        handlers.onOutput(decodeBase64(frame.output))
      }
      if (frame.state) {
        handlers.onState(frame.state)
      }
    } catch (err) {
      console.warn(`[process-terminal WS] invalid message for ${sessionId}:`, err)
    }
  }

  ws.onerror = (event) => {
    if (!closed) handlers.onError?.(event)
  }

  return {
    close() {
      closed = true
      ready = false
      ws.close()
    },
    writeInput(data) {
      if (!data) return
      send({ input: encodeBase64(data) })
    },
    resize(cols, rows) {
      if (!cols || !rows) return
      send({ resize: { cols, rows } })
    },
  }
}

export interface SessionListWSHandlers {
  onSessionCreated: (data: SessionMetaEventData) => void
  onSessionUpdated: (data: SessionMetaEventData) => void
  onSessionDeleted: (data: SessionDeletedData) => void
  onSessionProcessState: (data: SessionProcessStateData) => void
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
      case 'session_process_state':
        handlers.onSessionProcessState(message.data as SessionProcessStateData)
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
