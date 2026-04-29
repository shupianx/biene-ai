import { buildCoreHeaders, buildCoreUrl } from '../runtime'

export class HttpError extends Error {
  status: number
  body: string

  constructor(status: number, message: string, body: string) {
    super(message)
    this.name = 'HttpError'
    this.status = status
    this.body = body
  }
}

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
    throw new HttpError(res.status, text || res.statusText, text)
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
  send_message_to_agent: boolean
  cowork: boolean
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
  // Sprite index ("0".."19") into renderer/public/avatar_sprite.png. The
  // server picks one at session creation and backfills any legacy session
  // missing this field, so it should always be present on a fresh load —
  // but the type stays optional for safety against transient mismatches.
  avatar?: string
  model_id: string
  model_name: string
  thinking_available?: boolean
  thinking_enabled?: boolean
  // Defaults to true when unset. Only `false` disables the composer's
  // image attachment control.
  images_available?: boolean
  // Reports whether the session's pinned model is currently usable.
  // Today only chatgpt_official models can flip this to false (when
  // OAuth has been revoked). When false the renderer should:
  //   • render the agent's grid card semi-transparent
  //   • allow opening the chat (history is still readable)
  //   • disable the composer with a "ChatGPT 无授权" banner
  // Always emitted by the backend so renderer code can rely on
  // `=== false` checks; older payloads that omit it are treated as
  // available by default.
  model_available?: boolean
  permissions: SessionPermissions
  profile: AgentProfile
  pending_permission?: import('../types/events').PermissionRequestData
  active_skills?: string[]
  installed_skill_ids?: string[]
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
  // Sprite index ("0".."19") chosen by the user in the new-agent modal.
  // Optional — when omitted, the server picks a random one.
  avatar?: string
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

// HEAD-fork an agent: server clones its workspace, copies history,
// generates a fresh ID, and returns the new agent's metadata. The new
// agent appears in the list immediately (server also broadcasts a
// session_created event, but the direct response is the authoritative
// snapshot the caller should attach to).
export function forkSession(id: string, name: string) {
  return post<SessionMeta>(`/api/sessions/${id}/fork`, { name })
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

export interface DisplayAttachment {
  name: string
  path: string
  size: number
  kind?: 'image' | ''
  media_type?: string
}

export interface DisplayCompaction {
  summary: string
  tokens_before: number
  tokens_after: number
  replaced: number
  manual?: boolean
}

/**
 * Marker for a help card pushed by `/help`. The card always renders the
 * current command catalogue dynamically, so this carries no payload —
 * it just discriminates "render as HelpCard" from regular messages in
 * AgentChatView's iteration. Renderer-only field; never persisted to
 * the backend (the help message itself is transient state in
 * `sess.messages`).
 */
export interface DisplayHelp {
  shown: true
}

export interface DisplayMessage {
  id: string
  role: 'user' | 'assistant' | 'system'
  author_type?: 'user' | 'agent' | 'system'
  author_id?: string
  author_name?: string
  agent_meta?: AgentMessageMeta
  text: string
  created_at: string
  streaming?: boolean
  tool_calls?: DisplayTool[]
  reasoning?: DisplayReasoning
  attachments?: DisplayAttachment[]
  compaction?: DisplayCompaction
  help?: DisplayHelp
}

export interface HistoryPage {
  messages: DisplayMessage[]
  has_more: boolean
}

export function getSessionHistory(
  sessionId: string,
  options?: { before?: string; limit?: number },
) {
  const params = new URLSearchParams()
  if (options?.before) params.set('before', options.before)
  if (options?.limit != null) params.set('limit', String(options.limit))
  const qs = params.toString()
  const path = `/api/sessions/${sessionId}/history${qs ? `?${qs}` : ''}`
  return get<HistoryPage>(path)
}

export function sendMessage(
  sessionId: string,
  text: string,
  options?: {
    clientMessageId?: string
    thinkingEnabled?: boolean
    files?: File[]
  },
) {
  const files = options?.files ?? []
  if (files.length === 0) {
    return post<{ ok: boolean }>(`/api/sessions/${sessionId}/send`, {
      text,
      client_message_id: options?.clientMessageId,
      thinking_enabled: options?.thinkingEnabled,
    })
  }
  const form = new FormData()
  form.append('text', text)
  if (options?.clientMessageId) form.append('client_message_id', options.clientMessageId)
  if (options?.thinkingEnabled !== undefined) form.append('thinking_enabled', String(options.thinkingEnabled))
  for (const file of files) {
    form.append('files', file, file.name)
  }
  return post<{ ok: boolean }>(`/api/sessions/${sessionId}/send`, form)
}

// Build a URL that serves a chat-level asset (e.g. a user-uploaded image)
// from the session's .biene/assets/user/ directory. Asset routes are
// exempted from the auth middleware on the server side (see isPublicAssetPath
// in server.go) so the URL stays clean for "open in external browser" —
// path entropy plus CORS are the actual safeguard.
export function sessionAssetURL(sessionId: string, attachmentPath: string) {
  const basename = attachmentPath.split('/').pop() ?? ''
  return buildCoreUrl(`/api/sessions/${sessionId}/assets/${encodeURIComponent(basename)}`)
}

export function setThinkingEnabled(sessionId: string, enabled: boolean) {
  return post<SessionMeta>(`/api/sessions/${sessionId}/thinking`, {
    enabled,
  })
}

export function interruptSession(id: string) {
  return post<{ ok: boolean }>(`/api/sessions/${id}/interrupt`)
}

// Triggers an explicit compaction round on the session. The session
// must be idle (no in-flight turn). Optional `instructions` are
// appended to the summarizer prompt for steering.
//
// Two non-error outcomes:
//   - status="compacted": the marker is in `message`, render it in chat
//   - status="no_op":     conversation was already short enough; the
//                         renderer surfaces a friendly inline notice
export interface CompactResponse {
  ok: boolean
  status: 'compacted' | 'no_op'
  message?: DisplayMessage
  reason?: string
}

export function compactSession(id: string, instructions?: string) {
  return post<CompactResponse>(
    `/api/sessions/${id}/compact`,
    instructions ? { instructions } : {},
  )
}

export function resolvePermission(
  sessionId: string,
  requestId: string,
  decision: 'allow' | 'always' | 'deny',
  resolution?: Record<string, unknown>,
) {
  return post<SessionMeta>(`/api/sessions/${sessionId}/permission`, {
    request_id: requestId,
    decision,
    ...(resolution ? { resolution } : {}),
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
  thinking_on?: Record<string, unknown>
  thinking_off?: Record<string, unknown>
  images_available?: boolean
  context_window?: number
}

export interface CompactionConfig {
  enabled: boolean
  reserve_tokens: number
  keep_recent_tokens: number
}

export interface CoreConfig {
  default_model: string
  model_list: ConfigModelEntry[]
  compaction?: CompactionConfig
}

export function fetchConfig() {
  return get<CoreConfig>('/api/config')
}

export function saveConfig(config: CoreConfig) {
  return post<CoreConfig>('/api/config', config)
}

// ── Provider templates ────────────────────────────────────────────────────

// ServerProviderTemplate / ServerVendor mirror the JSON shapes returned
// by GET /api/provider-templates. The renderer enriches each entry with
// vendor metadata + an icon component before exposing them to UI code
// (see providerTemplates store / constants/providerTemplateIcons.ts).
export interface ServerProviderTemplate {
  id: string
  name: string
  model: string
  context_window?: number
  thinking_available?: boolean
  thinking_on?: Record<string, unknown>
  thinking_off?: Record<string, unknown>
  images_available?: boolean
}

export interface ServerVendor {
  id: string
  name: string
  provider: string
  base_url: string
  models: ServerProviderTemplate[]
}

export interface ProviderTemplatesResponse {
  vendors: ServerVendor[]
}

export function fetchProviderTemplates() {
  return get<ProviderTemplatesResponse>('/api/provider-templates')
}

// ── Available models (config + synthetic OAuth entries) ──────────────────

// AvailableModelEntry is a single picker option. The id is what gets
// persisted on a session as `model_id`; for OAuth-backed entries it is
// the synthetic form `chatgpt_official:<openai-model-name>`.
export interface AvailableModelEntry {
  id: string
  label: string
  provider: string
  model: string
  summary?: string
}

// AvailableModelGroup buckets entries under a header in the picker.
// `kind === 'chatgpt_official'` marks the OAuth virtual provider so the
// renderer can render its sub-selector differently from user configs.
export interface AvailableModelGroup {
  id: string
  label: string
  kind: 'user' | 'chatgpt_official'
  description?: string
  models: AvailableModelEntry[]
}

export interface AvailableModelsResponse {
  default_model_id: string
  groups: AvailableModelGroup[]
}

export function fetchAvailableModels() {
  return get<AvailableModelsResponse>('/api/models/available')
}

// ── ChatGPT OAuth ─────────────────────────────────────────────────────────

export interface ChatGPTAuthStatus {
  authenticated: boolean
  email?: string
  account_id?: string
  expires_at?: number
  // Populated when the most recent OAuth attempt failed (e.g. token
  // exchange returned 4xx). The renderer's poll loop watches this so
  // it can stop spinning the moment the backend gives up.
  last_error?: string
}

export interface ChatGPTStartResponse {
  auth_url: string
  state: string
  flow_id: string
  port: number
  expires_in_seconds: number
  // Set when localhost:1455 couldn't be bound (typically because the
  // Codex CLI is running in parallel and already holds the port). The
  // browser will still complete the redirect, but it lands on
  // *something other than us* — the user has to copy the redirected
  // URL back into Biene and POST it through finishChatGPTOAuthManually.
  manual_paste_required?: boolean
  // Underlying bind error so the UI can explain why the manual
  // fallback kicked in. Empty when the listener bound normally.
  port_bind_error?: string
}

export function fetchChatGPTAuthStatus() {
  return get<ChatGPTAuthStatus>('/api/auth/chatgpt/status')
}

export function startChatGPTOAuth() {
  return post<ChatGPTStartResponse>('/api/auth/chatgpt/start')
}

export function cancelChatGPTOAuth(state: string) {
  return post<{ ok: boolean }>('/api/auth/chatgpt/cancel', { state })
}

// ── ChatGPT usage / quota ────────────────────────────────────────────────
//
// Surfaced from the `x-codex-*` rate-limit headers on /responses
// turns rather than from a separate active fetch (the upstream's
// /api/codex/usage sits behind Cloudflare and reliably 403s any
// non-browser client). Consequence: the panel is empty until the
// user has sent at least one Codex turn — the `available` flag on
// the response distinguishes that empty state from a populated one.

export interface RateLimitWindow {
  used_percent: number
  // Length of the rolling window in minutes; absent on snapshots
  // that don't expose it.
  window_minutes?: number
  // Unix seconds. Clients render as a relative "resets in X min"
  // rather than show the raw timestamp.
  reset_at?: number
}

export interface RateLimitSnapshot {
  // Unix seconds when the snapshot was captured. Used by the UI to
  // hint "data is X minutes old" if the user hasn't sent anything
  // recently.
  updated_at: number
  // Bucket id ("codex" for the default chat budget). Omitted when
  // the upstream doesn't echo a name back.
  limit_name?: string
  primary?: RateLimitWindow
  secondary?: RateLimitWindow
}

export interface ChatGPTUsageResponse {
  // false = no Codex turn has happened since core started, so we
  // have no data to show. Renderer should display a "send a message
  // first" empty state in this case.
  available: boolean
  snapshot?: RateLimitSnapshot
}

// fetchChatGPTUsage returns the cached rate-limit snapshot, or an
// `available: false` payload when no snapshot has been captured yet.
// Cheap (no upstream RTT), so safe to call on every Settings open.
export function fetchChatGPTUsage() {
  return get<ChatGPTUsageResponse>('/api/auth/chatgpt/usage')
}

// finishChatGPTOAuthManually completes a flow that started with
// `manual_paste_required: true`. The `code` field accepts the full
// pasted redirect URL ("http://localhost:1455/auth/callback?code=…"),
// the bare query fragment, or just the code value; the server parses
// all three. Pass the `state` value returned from startChatGPTOAuth so
// the server can match the pasted code back to the right pending flow.
export function finishChatGPTOAuthManually(state: string, code: string) {
  return post<{ ok: boolean }>('/api/auth/chatgpt/manual-callback', { state, code })
}

export function logoutChatGPT() {
  return post<{ ok: boolean }>('/api/auth/chatgpt/logout')
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

export function installSkillToSession(sessionId: string, skillId: string) {
  return post<{ skill_name: string }>(
    `/api/sessions/${encodeURIComponent(sessionId)}/skills/install`,
    { skill_id: skillId },
  )
}

export function uninstallSkillFromSession(sessionId: string, skillId: string) {
  return del<{ skill_name: string }>(
    `/api/sessions/${encodeURIComponent(sessionId)}/skills/${encodeURIComponent(skillId)}`,
  )
}

// ── Background process (one per session) ──────────────────────────────────

import type { ProcessStateData } from '../types/events'

export function getProcessState(sessionId: string) {
  return get<ProcessStateData>(`/api/sessions/${encodeURIComponent(sessionId)}/process`)
}

export function stopSessionProcess(sessionId: string) {
  return post<ProcessStateData>(`/api/sessions/${encodeURIComponent(sessionId)}/process/stop`)
}

export interface ActiveBackgroundProcess {
  session_id: string
  session_name: string
  command: string
  args?: string[]
  pid?: number
}

export function listActiveProcesses() {
  return get<{ processes: ActiveBackgroundProcess[] }>('/api/processes/active')
}
