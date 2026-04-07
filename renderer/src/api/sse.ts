import type {
  MessageAddedData,
  StatusData,
  TextDeltaData,
  ToolStartData,
  ToolResultData,
  ToolDeniedData,
  PermissionRequestData,
  PermissionClearedData,
  ErrorData,
} from '../types/events'
import { buildCoreUrl } from '../runtime'

export interface SSEHandlers {
  onMessageAdded: (data: MessageAddedData) => void
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
}

/** Opens a persistent SSE connection for a session. Returns a cleanup function. */
export function connectSSE(sessionId: string, handlers: SSEHandlers): () => void {
  const es = new EventSource(buildCoreUrl(`/api/sessions/${sessionId}/events`))

  const on = (type: string, fn: (data: any) => void) => {
    es.addEventListener(type, (e: MessageEvent) => {
      try { fn(JSON.parse(e.data)) } catch (_) {}
    })
  }

  on('message_added',      handlers.onMessageAdded)
  on('status',             handlers.onStatus)
  on('text_delta',         handlers.onTextDelta)
  on('tool_compose',       handlers.onToolCompose)
  on('tool_start',         handlers.onToolStart)
  on('tool_result',        handlers.onToolResult)
  on('tool_denied',        handlers.onToolDenied)
  on('permission_request', handlers.onPermissionRequest)
  on('permission_cleared', handlers.onPermissionCleared)
  on('error',              handlers.onError)
  on('done',               () => handlers.onDone())

  return () => es.close()
}
