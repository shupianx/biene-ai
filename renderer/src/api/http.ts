import { buildCoreHeaders, buildCoreUrl } from '../runtime'

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const isFormData = typeof FormData !== 'undefined' && body instanceof FormData
  const headers = buildCoreHeaders(body && !isFormData ? { 'Content-Type': 'application/json' } : undefined)
  const res = await fetch(buildCoreUrl(path), {
    method,
    headers,
    body: body
      ? (isFormData ? body : JSON.stringify(body))
      : undefined,
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
  model_id: string
  model_name: string
  thinking_available?: boolean
  thinking_enabled?: boolean
  permissions: SessionPermissions
  profile: AgentProfile
  pending_permission?: import('../types/events').PermissionRequestData
  active_skills?: string[]
  created_at: string
  last_active: string
}

export function listSessions() {
  return get<SessionMeta[]>('/api/sessions')
}

export interface CreateSessionOptions {
  name: string
  model_id?: string
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

export interface DisplayReasoning {
  text: string
  started_at: string
  duration_ms?: number
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
  text: string
  created_at: string
  streaming?: boolean
  tool_calls?: DisplayTool[]
  reasoning?: DisplayReasoning
}

export function getSessionHistory(sessionId: string) {
  return get<DisplayMessage[]>(`/api/sessions/${sessionId}/history`)
}

export function sendMessage(sessionId: string, text: string, clientMessageId?: string, thinkingEnabled?: boolean) {
  return post<{ ok: boolean }>(`/api/sessions/${sessionId}/send`, {
    text,
    client_message_id: clientMessageId,
    thinking_enabled: thinkingEnabled,
  })
}

export function setThinkingEnabled(sessionId: string, enabled: boolean) {
  return post<SessionMeta>(`/api/sessions/${sessionId}/thinking`, {
    enabled,
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

export interface ConfigModelEntry {
  id: string
  name: string
  provider: string
  api_key: string
  model: string
  base_url: string
  thinking_available?: boolean
}

export interface CoreConfig {
  default_model: string
  model_list: ConfigModelEntry[]
  max_tokens: number
}

export function fetchConfig() {
  return get<CoreConfig>('/api/config')
}

export function saveConfig(config: CoreConfig) {
  return post<CoreConfig>('/api/config', config)
}

// ── Skills ────────────────────────────────────────────────────────────────

export interface SkillCatalogEntry {
  id: string
  name: string
  description: string
  instructions: string
}

export interface SkillsCatalog {
  root: string
  skills: SkillCatalogEntry[]
  default_enabled_skill_ids: string[]
}

export function listSkills() {
  return get<SkillsCatalog>('/api/skills')
}

export function saveSkillsConfig(defaultEnabledSkillIDs: string[]) {
  return post<SkillsCatalog>('/api/skills/config', {
    default_enabled_skill_ids: defaultEnabledSkillIDs,
  })
}

export function deleteSkill(id: string) {
  return del<SkillsCatalog>(`/api/skills/${encodeURIComponent(id)}`)
}

export function importSkillFolder(files: File[]) {
  const form = new FormData()
  for (const file of files) {
    const relativePath = file.webkitRelativePath || file.name
    form.append('files', file, relativePath)
  }
  return post<SkillsCatalog>('/api/skills/import', form)
}
