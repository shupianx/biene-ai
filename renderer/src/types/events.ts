export type SessionEventType =
  | 'message_added'
  | 'status'
  | 'reasoning_delta'
  | 'text_delta'
  | 'tool_compose'
  | 'tool_compose_progress'
  | 'tool_start'
  | 'tool_result'
  | 'tool_denied'
  | 'permission_request'
  | 'permission_cleared'
  | 'skill_activated'
  | 'process_state'
  | 'error'
  | 'done'

export type SessionListEventType =
  | 'session_created'
  | 'session_updated'
  | 'session_deleted'
  | 'session_process_state'

export interface SessionProcessStateData {
  session_id: string
  active: boolean
  command?: string
  args?: string[]
}

export interface MessageAddedData {
  message: import('../api/http').DisplayMessage
}

export interface StatusData {
  status: import('../api/http').SessionMeta['status']
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

export interface FileCollision {
  requested_path: string
  target_path: string
}

export interface PermissionContextData {
  collisions?: FileCollision[]
}

export interface ToolComposeProgressSnapshot {
  file_path?: string
  file_text_bytes?: number
}

export interface PermissionRequestData {
  request_id: string
  permission: string
  tool_name: string
  tool_summary: string
  tool_input: unknown
  tool_id?: string
  context?: PermissionContextData
  progress?: ToolComposeProgressSnapshot
  expired?: boolean
}

export interface ToolComposeProgressData {
  tool_id: string
  tool_name?: string
  file_path?: string
  file_text_bytes?: number
}

export type CollisionStrategy = 'rename' | 'overwrite' | 'skip'

export interface PermissionResolution {
  collision?: CollisionStrategy
}

export interface PermissionClearedData {
  request_id?: string
}

export interface SkillActivatedData {
  skill_name: string
}

export type ProcessStatus = 'idle' | 'running' | 'exited' | 'killed' | 'failed'

export interface ProcessStateData {
  active: boolean
  status: ProcessStatus
  command?: string
  args?: string[]
  cwd?: string
  pid?: number
  started_at?: string
  exited_at?: string
  exit_code?: number
  log_file?: string
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
