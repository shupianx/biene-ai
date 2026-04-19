export type SessionEventType =
  | 'message_added'
  | 'skill_activated'
  | 'status'
  | 'reasoning_delta'
  | 'text_delta'
  | 'tool_compose'
  | 'tool_start'
  | 'tool_result'
  | 'tool_denied'
  | 'permission_request'
  | 'permission_cleared'
  | 'error'
  | 'done'

export type SessionListEventType =
  | 'session_created'
  | 'session_updated'
  | 'session_deleted'

export interface MessageAddedData {
  message: import('../api/http').DisplayMessage
}

export interface StatusData {
  status: import('../api/http').SessionMeta['status']
}

export interface SkillActivatedData {
  skill_name: string
}

export interface TextDeltaData {
  text: string
}

export interface ReasoningDeltaData {
  text: string
}

export interface ToolStartData {
  tool_id?: string
  tool_name: string
  tool_summary: string
  tool_input: unknown
}

export interface ToolResultData {
  tool_id?: string
  tool_name: string
  text: string
  is_error: boolean
}

export interface ToolDeniedData {
  tool_id?: string
  tool_name: string
}

export interface PermissionRequestData {
  request_id: string
  permission: string
  tool_name: string
  tool_summary: string
  tool_input: unknown
  expired?: boolean
}

export interface PermissionClearedData {
  request_id?: string
}

export interface ErrorData {
  message: string
}

export interface SessionMetaEventData {
  session: import('../api/http').SessionMeta
}

export interface SessionDeletedData {
  id: string
}
