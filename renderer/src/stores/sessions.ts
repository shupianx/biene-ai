import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  listSessions, createSession, updateSession, deleteSession,
  getSessionHistory, sendMessage as apiSend,
  interruptSession as apiInterrupt,
  resolvePermission as apiResolve,
  setThinkingEnabled as apiSetThinking,
  getProcessState as apiGetProcess,
  stopSessionProcess as apiStopProcess,
  type SessionMeta, type SessionPermissions, type AgentProfile, type DisplayMessage, type DisplayTool,
} from '../api/http'
import { connectWS } from '../api/ws'
import { t } from '../i18n'
import { getCoreBaseUrl, getDesktopBridge } from '../runtime'
import type { PermissionRequestData, ProcessStateData } from '../types/events'

// ── Per-session state ──────────────────────────────────────────────────────

export interface PermissionRequest extends PermissionRequestData {}

export interface AgentSession {
  meta: SessionMeta
  messages: DisplayMessage[]
  isStreaming: boolean
  isInterrupting: boolean
  pendingPermission: PermissionRequest | null
  activeSkills: string[]
  processState: ProcessStateData | null
  _historyLoaded: boolean
  hasMoreHistory: boolean
  isLoadingMoreHistory: boolean
  // cleanup fn returned by connectWS
  _disconnect: (() => void) | null
}

const HISTORY_PAGE_SIZE = 50

interface AttachOptions {
  loadHistory?: boolean
  subscribe?: boolean
}

// ── Store ──────────────────────────────────────────────────────────────────

export const useSessionsStore = defineStore('sessions', () => {
  const sessions = ref<Record<string, AgentSession>>({})
  let initialized = false

  // Startup diagnostic
  const bridge = getDesktopBridge()
  console.log(`[sessions] coreBaseUrl=${getCoreBaseUrl()}, isElectron=${bridge?.isElectron ?? false}, windowKind=${bridge?.windowKind ?? 'browser'}`)

  const sessionList = computed(() =>
    Object.values(sessions.value).sort(
      (a, b) => new Date(a.meta.created_at).getTime() - new Date(b.meta.created_at).getTime()
    )
  )

  // ── Bootstrap ────────────────────────────────────────────────────────────

  async function init(loadHistory = false, subscribe = true) {
    if (initialized) {
      await Promise.all(
        Object.values(sessions.value).map((sess) =>
          _attach(sess.meta, { loadHistory, subscribe })
        )
      )
      return
    }
    initialized = true
    const list = await listSessions()
    await Promise.all(
      list.map((meta) => _attach(meta, { loadHistory, subscribe }))
    )
  }

  async function refresh(loadHistory = false, subscribe = true) {
    const list = await listSessions()
    const liveIDs = new Set(list.map((meta) => meta.id))

    for (const id of Object.keys(sessions.value)) {
      if (liveIDs.has(id)) continue
      const sess = sessions.value[id]
      if (sess?._disconnect) sess._disconnect()
      delete sessions.value[id]
    }

    await Promise.all(
      list.map((meta) => _attach(meta, { loadHistory, subscribe }))
    )
  }

  async function syncSession(id: string, loadHistory = false, subscribe = true) {
    const list = await listSessions()
    const meta = list.find((item) => item.id === id)
    if (!meta) {
      const sess = sessions.value[id]
      if (sess?._disconnect) sess._disconnect()
      delete sessions.value[id]
      return null
    }

    await _attach(meta, { loadHistory, subscribe })
    return sessions.value[id] ?? null
  }

  async function upsertSessionMeta(meta: SessionMeta, subscribe = false) {
    await _attach(meta, { subscribe })
    return sessions.value[meta.id] ?? null
  }

  function removeSessionLocal(id: string) {
    const sess = sessions.value[id]
    if (sess?._disconnect) sess._disconnect()
    delete sessions.value[id]
  }

  async function ensureSession(id: string, loadHistory = false, subscribe = true) {
    const existing = sessions.value[id]
    if (existing) {
      _setSubscription(existing, id, subscribe)
      if (loadHistory) await ensureHistory(id)
      return existing
    }
    return syncSession(id, loadHistory, subscribe)
  }

  async function ensureHistory(id: string) {
    const sess = sessions.value[id]
    if (!sess || sess._historyLoaded) return

    const page = await getSessionHistory(id, { limit: HISTORY_PAGE_SIZE })
    const liveMessages = sess.messages
    const knownIDs = new Set(page.messages.map((msg) => msg.id))

    sess.messages = page.messages
    for (const msg of liveMessages) {
      if (!knownIDs.has(msg.id)) {
        sess.messages.push(msg)
      }
    }
    sess._historyLoaded = true
    sess.hasMoreHistory = page.has_more
  }

  async function loadMoreHistory(id: string) {
    const sess = sessions.value[id]
    if (!sess || !sess._historyLoaded) return
    if (!sess.hasMoreHistory || sess.isLoadingMoreHistory) return
    const oldest = sess.messages[0]
    if (!oldest) return

    sess.isLoadingMoreHistory = true
    try {
      const page = await getSessionHistory(id, {
        before: oldest.id,
        limit: HISTORY_PAGE_SIZE,
      })
      const existingIDs = new Set(sess.messages.map((m) => m.id))
      const older = page.messages.filter((m) => !existingIDs.has(m.id))
      if (older.length > 0) {
        sess.messages = [...older, ...sess.messages]
      }
      sess.hasMoreHistory = page.has_more
    } catch (err) {
      console.error(`[loadMoreHistory] ${id}:`, err)
    } finally {
      sess.isLoadingMoreHistory = false
    }
  }

  // ── Create / delete ───────────────────────────────────────────────────────

  async function create(
    name: string,
    modelID: string,
    permissions: SessionPermissions,
    profile: AgentProfile,
    avatar: string,
    options: AttachOptions = {},
  ) {
    const meta = await createSession({ name, model_id: modelID, permissions, profile, avatar })
    await _attach(meta, options)
    return meta
  }

  async function update(id: string, payload: { name?: string; permissions?: SessionPermissions; profile?: AgentProfile }) {
    const meta = await updateSession(id, payload)
    const sess = sessions.value[id]
    if (sess) {
      sess.meta = meta
    }
    return meta
  }

  async function remove(id: string) {
    removeSessionLocal(id)
    await deleteSession(id)
  }

  // ── Messaging ─────────────────────────────────────────────────────────────

  async function sendMessage(sessionId: string, text: string, files: File[] = []) {
    const sess = sessions.value[sessionId]
    if (!sess || sess.isStreaming) {
      console.warn(`[sendMessage] blocked: sess=${!!sess}, isStreaming=${sess?.isStreaming}`)
      return
    }
    const messageId = crypto.randomUUID()
    const createdAt = new Date().toISOString()
    sess.isStreaming = true
    sess.messages.push({ id: messageId, role: 'user', text, created_at: createdAt, tool_calls: [] })
    try {
      await apiSend(sessionId, text, {
        clientMessageId: messageId,
        thinkingEnabled: sess.meta.thinking_available ? Boolean(sess.meta.thinking_enabled) : undefined,
        files,
      })
      console.log(`[sendMessage] POST ok for ${sessionId}`)
    } catch (err) {
      console.error(`[sendMessage] POST failed for ${sessionId}:`, err)
      sess.isStreaming = false
      sess.meta.status = 'error'
      sess.messages = sess.messages.filter(m => m.id !== messageId)

      const msg = _ensureAssistantTextSegment(sess)
      msg.text += `\n\n**${t('common.errorLabel')}:** ${err instanceof Error ? err.message : String(err)}`
      _finishAssistantTurn(sess)
    }
  }

  async function setThinkingEnabled(sessionId: string, enabled: boolean) {
    const sess = sessions.value[sessionId]
    if (!sess?.meta.thinking_available) return

    const previous = Boolean(sess.meta.thinking_enabled)
    sess.meta.thinking_enabled = enabled
    try {
      sess.meta = await apiSetThinking(sessionId, enabled)
    } catch (err) {
      sess.meta.thinking_enabled = previous
      console.error(`[setThinkingEnabled] POST failed for ${sessionId}:`, err)
    }
  }

  async function interrupt(sessionId: string) {
    const sess = sessions.value[sessionId]
    if (!sess || !sess.isStreaming || sess.isInterrupting) return
    sess.isInterrupting = true
    try {
      await apiInterrupt(sessionId)
      // Give the realtime channel a moment to deliver the status event, then
      // sync state from the server as a fallback in case it was missed.
      setTimeout(() => {
        const s = sessions.value[sessionId]
        if (s?.isInterrupting || s?.isStreaming) {
          void syncSession(sessionId, false, true)
        }
      }, 2000)
    } catch {
      sess.isInterrupting = false
    }
  }

  async function _refreshProcessState(sessionId: string) {
    try {
      const state = await apiGetProcess(sessionId)
      const sess = sessions.value[sessionId]
      if (sess) sess.processState = state
    } catch (err) {
      console.warn(`[processState] fetch failed for ${sessionId}:`, err)
    }
  }

  async function stopProcess(sessionId: string) {
    const state = await apiStopProcess(sessionId)
    const sess = sessions.value[sessionId]
    if (sess) sess.processState = state
    return state
  }

  // Merge a runtime-only process snapshot pushed on the session-list WS.
  // The list event carries only {active, command} — the rich per-session
  // process state (PID, started_at, log_file) keeps coming through the
  // per-session WS and overwrites this when AgentChatView attaches.
  function applyListProcessState(data: {
    session_id: string
    active: boolean
    command?: string
    args?: string[]
  }) {
    const sess = sessions.value[data.session_id]
    if (!sess) return
    if (data.active) {
      sess.processState = {
        ...(sess.processState ?? { status: 'running' }),
        active: true,
        status: 'running',
        command: data.command ?? sess.processState?.command,
        args: data.args ?? sess.processState?.args,
      }
    } else if (sess.processState) {
      sess.processState = { ...sess.processState, active: false }
    } else {
      sess.processState = { active: false, status: 'idle' }
    }
  }

  async function resolvePermission(
    sessionId: string,
    decision: 'allow' | 'always' | 'deny',
    resolution?: Record<string, unknown>,
  ) {
    const sess = sessions.value[sessionId]
    if (!sess?.pendingPermission) return
    const requestId = sess.pendingPermission.request_id
    sess.pendingPermission = null
    sess.meta.pending_permission = undefined
    sess.meta = await apiResolve(sessionId, requestId, decision, resolution)
  }

  // ── Internal: attach a session to realtime updates and load its history ───

  async function _attach(meta: SessionMeta, options: AttachOptions = {}) {
    const loadHistory = Boolean(options.loadHistory)
    const subscribe = options.subscribe ?? true

    let sess = sessions.value[meta.id]
    if (!sess) {
      sess = {
        meta,
        messages: [],
        isStreaming: meta.status === 'running',
        isInterrupting: false,
        pendingPermission: meta.pending_permission ?? null,
        activeSkills: [...(meta.active_skills ?? [])],
        processState: null,
        _historyLoaded: false,
        hasMoreHistory: false,
        isLoadingMoreHistory: false,
        _disconnect: null,
      }
      sessions.value[meta.id] = sess
      void _refreshProcessState(meta.id)
    } else {
      sess.meta = meta
      sess.isStreaming = meta.status === 'running'
      if (meta.status !== 'running') {
        sess.isInterrupting = false
      }
      sess.pendingPermission = meta.pending_permission ?? null
      sess.activeSkills = [...(meta.active_skills ?? [])]
    }

    if (loadHistory) {
      await ensureHistory(meta.id)
    }

    _setSubscription(sess, meta.id, subscribe)
  }

  // Subscribe means "ensure this session has its realtime channel open" —
  // it never closes one. `subscribe=false` is "I don't need the channel
  // for my use case, leave whatever's there alone", NOT "force-disconnect".
  // The latter caused a nasty regression where a list-WS event handler in
  // an agent window would silently kill the per-session feed it was
  // depending on for tool_compose / tool_result. Drop the channel
  // explicitly via removeSessionLocal when a session truly goes away.
  function _setSubscription(sess: AgentSession, id: string, subscribe: boolean) {
    if (!subscribe) return
    if (sess._disconnect) return

    sess._disconnect = connectWS(id, {
      onMessageAdded({ message }) {
        const s = sessions.value[id]
        if (!s) return
        // The sender may have optimistically appended the message locally with
        // just the text. Merge the server's authoritative version (which
        // includes attachments, author metadata, etc.) into that slot.
        const existingIdx = s.messages.findIndex(m => m.id === message.id)
        if (existingIdx >= 0) {
          s.messages[existingIdx] = { ...s.messages[existingIdx], ...message }
        } else {
          s.messages.push(message)
        }
        s.isStreaming = true
      },
      onStatus({ status }) {
        const s = sessions.value[id]
        if (!s) return
        s.meta.status = status
        s.isStreaming = status === 'running'
        if (status !== 'running') {
          s.isInterrupting = false
          s.pendingPermission = null
          s.meta.pending_permission = undefined
        }
      },
      onReasoningDelta({ text }) {
        const s = sessions.value[id]
        if (!s) return
        const msg = _ensureAssistantReasoningSegment(s)
        if (!msg.reasoning) {
          msg.reasoning = {
            text: '',
            started_at: new Date().toISOString(),
          }
        }
        msg.reasoning.text += text
      },
      onTextDelta({ text }) {
        const s = sessions.value[id]
        if (!s) return
        _finalizeLatestAssistantReasoning(s)
        const msg = _ensureAssistantTextSegment(s)
        msg.text += text
      },
      onToolCompose({ tool_id, tool_name, tool_summary, tool_input }) {
        const s = sessions.value[id]
        if (!s) return
        _finalizeLatestAssistantReasoning(s)
        const tc: DisplayTool = {
          tool_id,
          tool_name,
          tool_summary,
          tool_input,
          status: 'composing',
        }
        const msg = _ensureAssistantToolSegment(s)
        msg.tool_calls!.push(tc)
      },
      onToolComposeProgress({ tool_id, file_path, file_text_bytes }) {
        const s = sessions.value[id]
        if (!s) return
        // Mirror the progress onto the pending permission card (if it
        // belongs to this tool call) so the blind "preparing…" prompt
        // fills in path and byte count as the input streams in.
        if (s.pendingPermission?.tool_id === tool_id) {
          const updated = { ...s.pendingPermission, progress: { file_path, file_text_bytes } } as PermissionRequest
          s.pendingPermission = updated
          s.meta.pending_permission = updated
        }
      },
      onToolStart({ tool_id, tool_name, tool_summary, tool_input }) {
        const s = sessions.value[id]
        if (!s) return
        _finalizeLatestAssistantReasoning(s)
        const existing = _findLatestActiveTool(s, tool_id, tool_name, ['composing'])
        if (existing) {
          existing.tool_summary = tool_summary
          existing.tool_input = tool_input
          existing.status = 'pending'
          return
        }
        const tc: DisplayTool = {
          tool_id,
          tool_name,
          tool_summary,
          tool_input,
          status: 'pending',
        }
        const msg = _ensureAssistantToolSegment(s)
        msg.tool_calls!.push(tc)
      },
      onToolResult({ tool_id, tool_name, text, is_error }) {
        const s = sessions.value[id]
        if (!s) return
        _finalizeLatestAssistantReasoning(s)
        const tc = _findLatestActiveTool(s, tool_id, tool_name, ['pending', 'composing'])
        if (tc) { tc.status = is_error ? 'error' : 'done'; tc.result = text }
      },
      onToolDenied({ tool_id, tool_name }) {
        const s = sessions.value[id]
        if (!s) return
        _finalizeLatestAssistantReasoning(s)
        const tc = _findLatestActiveTool(s, tool_id, tool_name, ['pending', 'composing'])
        if (tc) tc.status = 'denied'
      },
      onPermissionRequest(data) {
        const s = sessions.value[id]
        if (!s) return
        s.pendingPermission = data
        s.meta.pending_permission = data
      },
      onPermissionCleared() {
        const s = sessions.value[id]
        if (!s) return
        s.pendingPermission = null
        s.meta.pending_permission = undefined
      },
      onProcessState(state) {
        const s = sessions.value[id]
        if (!s) return
        s.processState = state
      },
      onSkillActivated({ skill_name }) {
        const s = sessions.value[id]
        if (!s) return
        if (!s.activeSkills.includes(skill_name)) {
          s.activeSkills.push(skill_name)
        }
        const metaSkills = s.meta.active_skills ?? []
        if (!metaSkills.includes(skill_name)) {
          s.meta.active_skills = [...metaSkills, skill_name]
        }
      },
      onError({ message }) {
        const s = sessions.value[id]
        if (!s) return
        _finalizeLatestAssistantReasoning(s)
        const msg = _ensureAssistantTextSegment(s)
        msg.text += `\n\n**${t('common.errorLabel')}:** ${message}`
      },
      onDone() {
        const s = sessions.value[id]
        if (!s) return
        if (s.isInterrupting) {
          _interruptAssistantTurn(s)
          return
        }
        _finishAssistantTurn(s)
      },
      onReconnect() {
        const s = sessions.value[id]
        if (!s) return
        s._historyLoaded = false
        void syncSession(id, true, true)
      },
    })
  }

  return {
    sessions, sessionList,
    init, refresh, syncSession, upsertSessionMeta, removeSessionLocal, ensureSession, ensureHistory, loadMoreHistory, create, update, remove,
    sendMessage, setThinkingEnabled, interrupt, resolvePermission,
    stopProcess, applyListProcessState,
  }
})

// ── Helpers ───────────────────────────────────────────────────────────────

function _newAssistantSegment(sess: AgentSession): DisplayMessage {
  const msg: DisplayMessage = {
    id: crypto.randomUUID(),
    role: 'assistant',
    text: '',
    created_at: new Date().toISOString(),
    streaming: true,
    tool_calls: [],
  }
  sess.messages.push(msg)
  return msg
}

function _latestStreamingAssistant(sess: AgentSession): DisplayMessage | null {
  for (let i = sess.messages.length - 1; i >= 0; i -= 1) {
    const msg = sess.messages[i]
    if (msg.role !== 'assistant') break
    if (msg.streaming !== false) return msg
  }
  return null
}

function _ensureAssistantTextSegment(sess: AgentSession): DisplayMessage {
  const last = _latestStreamingAssistant(sess)
  if (last && (last.tool_calls?.length ?? 0) === 0) return last
  return _newAssistantSegment(sess)
}

function _ensureAssistantReasoningSegment(sess: AgentSession): DisplayMessage {
  const last = _latestStreamingAssistant(sess)
  if (last && (last.tool_calls?.length ?? 0) === 0 && !last.text) return last
  return _newAssistantSegment(sess)
}

function _ensureAssistantToolSegment(sess: AgentSession): DisplayMessage {
  const last = _latestStreamingAssistant(sess)
  if (last && !last.text) return last
  return _newAssistantSegment(sess)
}

function _finalizeAssistantReasoning(msg: DisplayMessage | null) {
  if (!msg?.reasoning || msg.reasoning.duration_ms) return
  const startedAt = new Date(msg.reasoning.started_at).getTime()
  const duration = Number.isFinite(startedAt) ? Math.max(1, Date.now() - startedAt) : 1
  msg.reasoning.duration_ms = duration
}

function _finalizeLatestAssistantReasoning(sess: AgentSession) {
  _finalizeAssistantReasoning(_latestStreamingAssistant(sess))
}

function _findLatestActiveTool(
  sess: AgentSession,
  toolId: string | undefined,
  toolName: string,
  statuses: DisplayTool['status'][],
): DisplayTool | undefined {
  for (let i = sess.messages.length - 1; i >= 0; i -= 1) {
    const msg = sess.messages[i]
    if (msg.role !== 'assistant' || msg.streaming === false) break
    const tools = [...(msg.tool_calls ?? [])].reverse()
    if (toolId) {
      const byId = tools.find((tool) => tool.tool_id === toolId && statuses.includes(tool.status))
      if (byId) return byId
    }
    const byName = tools.find((tool) => tool.tool_name === toolName && statuses.includes(tool.status))
    if (byName) return byName
  }
}

function _finishAssistantTurn(sess: AgentSession) {
  for (let i = sess.messages.length - 1; i >= 0; i -= 1) {
    const msg = sess.messages[i]
    if (msg.role !== 'assistant' || msg.streaming === false) break
    _finalizeAssistantReasoning(msg)
    msg.streaming = false
  }
}

function _interruptAssistantTurn(sess: AgentSession) {
  for (let i = sess.messages.length - 1; i >= 0; i -= 1) {
    const msg = sess.messages[i]
    if (msg.role !== 'assistant' || msg.streaming === false) break
    _finalizeAssistantReasoning(msg)
    for (const tool of msg.tool_calls ?? []) {
      if (tool.status !== 'pending' && tool.status !== 'composing') continue
      tool.status = 'cancelled'
      if (!tool.result) {
        tool.result = t('input.interrupted')
      }
    }
    msg.streaming = false
  }
}
