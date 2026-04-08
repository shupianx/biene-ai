import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  listSessions, createSession, updateSession, deleteSession,
  getSessionHistory, sendMessage as apiSend,
  interruptSession as apiInterrupt,
  resolvePermission as apiResolve,
  type SessionMeta, type SessionPermissions, type AgentProfile, type DisplayMessage, type DisplayTool,
} from '../api/http'
import { connectSSE } from '../api/sse'
import type { PermissionRequestData } from '../types/events'

// ── Per-session state ──────────────────────────────────────────────────────

export interface PermissionRequest extends PermissionRequestData {}

export interface AgentSession {
  meta: SessionMeta
  messages: DisplayMessage[]
  isStreaming: boolean
  isInterrupting: boolean
  pendingPermission: PermissionRequest | null
  _historyLoaded: boolean
  // cleanup fn returned by connectSSE
  _disconnect: (() => void) | null
}

// ── Store ──────────────────────────────────────────────────────────────────

export const useSessionsStore = defineStore('sessions', () => {
  const sessions = ref<Record<string, AgentSession>>({})
  let initialized = false

  const sessionList = computed(() =>
    Object.values(sessions.value).sort(
      (a, b) => new Date(a.meta.created_at).getTime() - new Date(b.meta.created_at).getTime()
    )
  )

  // ── Bootstrap ────────────────────────────────────────────────────────────

  async function init(loadHistory = false) {
    if (initialized) {
      if (loadHistory) {
        await Promise.all(Object.keys(sessions.value).map((id) => ensureHistory(id)))
      }
      return
    }
    initialized = true
    const list = await listSessions()
    await Promise.all(list.map(meta => _attach(meta, loadHistory)))
  }

  async function refresh(loadHistory = false) {
    const list = await listSessions()
    const liveIDs = new Set(list.map((meta) => meta.id))

    for (const id of Object.keys(sessions.value)) {
      if (liveIDs.has(id)) continue
      const sess = sessions.value[id]
      if (sess?._disconnect) sess._disconnect()
      delete sessions.value[id]
    }

    await Promise.all(list.map(async (meta) => {
      const existing = sessions.value[meta.id]
      if (existing) {
        existing.meta = meta
        existing.isStreaming = meta.status === 'running'
        if (meta.status !== 'running') {
          existing.isInterrupting = false
        }
        existing.pendingPermission = meta.pending_permission ?? null
        if (loadHistory) {
          await ensureHistory(meta.id)
        }
        return
      }
      await _attach(meta, loadHistory)
    }))
  }

  async function ensureSession(id: string, loadHistory = false) {
    const existing = sessions.value[id]
    if (existing) {
      if (loadHistory) await ensureHistory(id)
      return existing
    }

    const list = await listSessions()
    const meta = list.find((item) => item.id === id)
    if (!meta) return null

    await _attach(meta, loadHistory)
    return sessions.value[id] ?? null
  }

  async function ensureHistory(id: string) {
    const sess = sessions.value[id]
    if (!sess || sess._historyLoaded) return

    const history = await getSessionHistory(id)
    const liveMessages = sess.messages
    const knownIDs = new Set(history.map((msg) => msg.id))

    sess.messages = history
    for (const msg of liveMessages) {
      if (!knownIDs.has(msg.id)) {
        sess.messages.push(msg)
      }
    }
    sess._historyLoaded = true
  }

  // ── Create / delete ───────────────────────────────────────────────────────

  async function create(name: string, permissions: SessionPermissions, profile: AgentProfile) {
    const meta = await createSession({ name, permissions, profile })
    await _attach(meta, true)
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
    const sess = sessions.value[id]
    if (sess?._disconnect) sess._disconnect()
    delete sessions.value[id]
    await deleteSession(id)
  }

  // ── Messaging ─────────────────────────────────────────────────────────────

  async function sendMessage(sessionId: string, text: string) {
    const sess = sessions.value[sessionId]
    if (!sess || sess.isStreaming) return
    const messageId = crypto.randomUUID()
    const createdAt = new Date().toISOString()
    sess.isStreaming = true
    sess.messages.push({ id: messageId, role: 'user', text, created_at: createdAt, tool_calls: [] })
    try {
      await apiSend(sessionId, text, messageId)
    } catch (err) {
      sess.isStreaming = false
      sess.meta.status = 'error'
      sess.messages = sess.messages.filter(m => m.id !== messageId)

      const msg = _ensureAssistantTextSegment(sess)
      msg.text += `\n\n**Error:** ${err instanceof Error ? err.message : String(err)}`
      _finishAssistantTurn(sess)
    }
  }

  async function interrupt(sessionId: string) {
    const sess = sessions.value[sessionId]
    if (!sess || !sess.isStreaming || sess.isInterrupting) return
    sess.isInterrupting = true
    try {
      await apiInterrupt(sessionId)
    } catch {
      sess.isInterrupting = false
    }
  }

  async function resolvePermission(sessionId: string, decision: 'allow' | 'always' | 'deny') {
    const sess = sessions.value[sessionId]
    if (!sess?.pendingPermission) return
    const requestId = sess.pendingPermission.request_id
    sess.pendingPermission = null
    sess.meta.pending_permission = undefined
    sess.meta = await apiResolve(sessionId, requestId, decision)
  }

  // ── Internal: attach a session to SSE and load its history ────────────────

  async function _attach(meta: SessionMeta, loadHistory: boolean) {
    const existing = sessions.value[meta.id]
    if (existing?._disconnect) existing._disconnect()

    const sess: AgentSession = {
      meta,
      messages: [],
      isStreaming: meta.status === 'running',
      isInterrupting: false,
      pendingPermission: meta.pending_permission ?? null,
      _historyLoaded: false,
      _disconnect: null,
    }
    sessions.value[meta.id] = sess

    if (loadHistory) {
      const history = await getSessionHistory(meta.id)
      sessions.value[meta.id].messages = history
      sessions.value[meta.id]._historyLoaded = true
    }

    const id = meta.id
    sess._disconnect = connectSSE(meta.id, {
      onMessageAdded({ message }) {
        const s = sessions.value[id]
        // Avoid duplicates (the sender already appended the message optimistically).
        if (!s.messages.find(m => m.id === message.id)) {
          s.messages.push(message)
        }
        s.isStreaming = true
      },
      onStatus({ status }) {
        const s = sessions.value[id]
        s.meta.status = status
        s.isStreaming = status === 'running'
        if (status !== 'running') {
          s.isInterrupting = false
          s.pendingPermission = null
          s.meta.pending_permission = undefined
        }
      },
      onTextDelta({ text }) {
        _ensureAssistantTextSegment(sessions.value[id]).text += text
      },
      onToolCompose({ tool_id, tool_name, tool_summary, tool_input }) {
        const tc: DisplayTool = {
          tool_id,
          tool_name,
          tool_summary,
          tool_input,
          status: 'composing',
        }
        _ensureAssistantToolSegment(sessions.value[id]).tool_calls!.push(tc)
      },
      onToolStart({ tool_id, tool_name, tool_summary, tool_input }) {
        const existing = _findLatestActiveTool(sessions.value[id], tool_id, tool_name, ['composing'])
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
        _ensureAssistantToolSegment(sessions.value[id]).tool_calls!.push(tc)
      },
      onToolResult({ tool_id, tool_name, text, is_error }) {
        const tc = _findLatestActiveTool(sessions.value[id], tool_id, tool_name, ['pending', 'composing'])
        if (tc) { tc.status = is_error ? 'error' : 'done'; tc.result = text }
      },
      onToolDenied({ tool_id, tool_name }) {
        const tc = _findLatestActiveTool(sessions.value[id], tool_id, tool_name, ['pending', 'composing'])
        if (tc) tc.status = 'denied'
      },
      onPermissionRequest(data) {
        sessions.value[id].pendingPermission = data
        sessions.value[id].meta.pending_permission = data
      },
      onPermissionCleared() {
        const s = sessions.value[id]
        s.pendingPermission = null
        s.meta.pending_permission = undefined
      },
      onError({ message }) {
        _ensureAssistantTextSegment(sessions.value[id]).text += `\n\n**Error:** ${message}`
      },
      onDone() {
        const s = sessions.value[id]
        if (s.isInterrupting) {
          _interruptAssistantTurn(s)
          return
        }
        _finishAssistantTurn(s)
      },
    })
  }

  return {
    sessions, sessionList,
    init, refresh, ensureSession, ensureHistory, create, update, remove,
    sendMessage, interrupt, resolvePermission,
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

function _ensureAssistantToolSegment(sess: AgentSession): DisplayMessage {
  const last = _latestStreamingAssistant(sess)
  if (last && !last.text) return last
  return _newAssistantSegment(sess)
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
    msg.streaming = false
  }
}

function _interruptAssistantTurn(sess: AgentSession) {
  for (let i = sess.messages.length - 1; i >= 0; i -= 1) {
    const msg = sess.messages[i]
    if (msg.role !== 'assistant' || msg.streaming === false) break
    for (const tool of msg.tool_calls ?? []) {
      if (tool.status !== 'pending' && tool.status !== 'composing') continue
      tool.status = 'cancelled'
      if (!tool.result) {
        tool.result = 'Interrupted.'
      }
    }
    msg.streaming = false
  }
}
