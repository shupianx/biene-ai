import { buildCoreHeaders, buildCoreUrl } from '../runtime'

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers = buildCoreHeaders(body ? { 'Content-Type': 'application/json' } : undefined)
  const res = await fetch(buildCoreUrl(path), {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || res.statusText)
  }
  return res.json()
}

const get  = <T>(path: string)                   => request<T>('GET', path)
const post = <T>(path: string, body?: unknown)   => request<T>('POST', path, body)
const del  = <T>(path: string)                   => request<T>('DELETE', path)

// ── Sessions ──────────────────────────────────────────────────────────────

export interface SessionPermissions {
  execute: boolean
  write: boolean
  send_to_agent: boolean
}

export type AgentDomain = string
export type AgentStyle = string

export interface AgentProfile {
  domain: AgentDomain
  style: AgentStyle
  custom_instructions: string
}

export interface SessionMeta {
  id: string
  name: string
  work_dir: string
  status: 'idle' | 'running' | 'error'
  permissions: SessionPermissions
  profile: AgentProfile
  pending_permission?: import('../types/events').PermissionRequestData
  created_at: string
  last_active: string
}

export function listSessions() {
  return get<SessionMeta[]>('/api/sessions')
}

export interface CreateSessionOptions {
  name: string
  permissions?: SessionPermissions
  profile?: AgentProfile
}

export function createSession(opts: CreateSessionOptions) {
  return post<SessionMeta>('/api/sessions', opts)
}

export interface UpdateSessionOptions {
  name?: string
  permissions?: SessionPermissions
  profile?: AgentProfile
}

export function updateSession(id: string, opts: UpdateSessionOptions) {
  return post<SessionMeta>(`/api/sessions/${id}/settings`, opts)
}

export function deleteSession(id: string) {
  return del<{ ok: boolean }>(`/api/sessions/${id}`)
}

// ── Chat ──────────────────────────────────────────────────────────────────

export interface DisplayTool {
  tool_id?: string
  tool_name: string
  tool_summary: string
  tool_input?: unknown
  status: 'composing' | 'pending' | 'done' | 'error' | 'denied' | 'cancelled'
  result?: string
}

export interface AgentMessageMeta {
  thread_id: string
  message_id: string
  in_reply_to?: string
}

export interface DisplayMessage {
  id: string
  role: 'user' | 'assistant'
  author_type?: 'user' | 'agent' | 'system'
  author_id?: string
  author_name?: string
  agent_meta?: AgentMessageMeta
  used_skill_name?: string
  text: string
  created_at: string
  streaming?: boolean
  tool_calls?: DisplayTool[]
}

export function getSessionHistory(sessionId: string) {
  return get<DisplayMessage[]>(`/api/sessions/${sessionId}/history`)
}

export function sendMessage(sessionId: string, text: string, clientMessageId?: string) {
  return post<{ ok: boolean }>(`/api/sessions/${sessionId}/send`, {
    text,
    client_message_id: clientMessageId,
  })
}

export function interruptSession(id: string) {
  return post<{ ok: boolean }>(`/api/sessions/${id}/interrupt`)
}

export function resolvePermission(sessionId: string, requestId: string, decision: 'allow' | 'always' | 'deny') {
  return post<SessionMeta>(`/api/sessions/${sessionId}/permission`, {
    request_id: requestId,
    decision,
  })
}

// ── Config ────────────────────────────────────────────────────────────────

export function fetchConfig() {
  return get<{
    default_model: string
    model_list: { name: string; provider: string; model: string; base_url: string }[]
    max_tokens: number
  }>('/api/config')
}
