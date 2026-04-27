<template>
  <div
    class="chat-shell"
    :style="{ '--input-overlay-height': `${inputOverlayHeight}px` }"
    @pointerdown="onUserInteraction"
    @wheel.passive="onUserInteraction"
    @touchstart.passive="onUserInteraction"
    @touchmove.passive="onUserInteraction"
  >
    <!-- Compact window chrome (agent window is frameless) -->
    <header class="chat-chrome">
      <div class="brand">
        <span class="brand-text">{{ t('agent.brand') }}</span>
      </div>
      <div class="chrome-divider" aria-hidden="true" />
      <div class="chrome-name" :title="session.meta.name">{{ session.meta.name }}</div>
      <div class="chrome-spacer" />
      <div class="status-tag" :class="statusTone">
        <span class="status-dot" />
        <span>{{ statusLabel }}</span>
      </div>
      <IconButton :aria-label="t('common.close')" @click="emit('close')">
        <svg class="close-icon" viewBox="0 0 24 24" aria-hidden="true" v-html="closeIconBody" />
      </IconButton>
    </header>

    <!-- Message list -->
    <div ref="listRef" class="message-list" :class="{ empty: session.messages.length === 0 }" @scroll="onListScroll">
      <div v-if="session.messages.length === 0" class="empty-chat">
        <MynauiLightningSolid class="empty-icon" aria-hidden="true" />
        <p class="empty-title">{{ t('agent.ready') }}</p>
        <p class="empty-dir">
          <span class="empty-dir-label">{{ t('agent.workDirLabel') }}</span>
          <span class="empty-dir-path">{{ session.meta.work_dir }}</span>
        </p>
      </div>
      <div
        v-if="session.hasMoreHistory || session.isLoadingMoreHistory"
        ref="topSentinelRef"
        class="history-sentinel"
      >
        <span v-if="session.isLoadingMoreHistory" class="history-spinner" aria-hidden="true" />
      </div>
      <MessageItem
        v-for="msg in session.messages"
        :key="msg.id"
        :msg="msg"
        :session-id="session.meta.id"
      />
      <PermissionDialog
        v-if="session.pendingPermission"
        :req="session.pendingPermission"
        @resolve="onResolve"
      />
      <div v-if="session.isStreaming && lastIsUser && !session.pendingPermission" class="typing">
        <span class="typing-dot" />
        <span class="typing-dot" />
        <span class="typing-dot" />
      </div>
    </div>

    <div ref="inputOverlayRef" class="input-overlay">
      <div
        v-if="showProcessWindow"
        class="process-window"
        :class="{ expanded: logsOpen }"
      >
        <ProcessCapsule
          class="process-capsule"
          :state="session.processState"
          :logs-open="logsOpen"
          :stopping="isStopping"
          :post-exit="postExit"
          @toggle-logs="onToggleLogs"
          @stop="onStopProcess"
          @cancel-post-exit="cancelPostExitCountdown"
          @close="resetPostExit"
        />
        <Transition name="log-reveal">
          <ProcessLogPanel
            v-if="logsOpen"
            class="process-log-panel"
            :session-id="session.meta.id"
            :state="session.processState"
          />
        </Transition>
      </div>
      <InputBar
        :disabled="session.isStreaming"
        :interruptible="session.isStreaming"
        :interrupting="session.isInterrupting"
        :thinking-available="session.meta.thinking_available"
        :thinking-enabled="Boolean(session.meta.thinking_enabled)"
        :images-available="session.meta.images_available !== false"
        :mention-candidates="mentionCandidates"
        :skill-candidates="skillCandidates"
        @send="onSend"
        @update:thinking-enabled="onThinkingEnabledChange"
        @interrupt="onInterrupt"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { AgentSession } from '../../stores/sessions'
import MynauiLightningSolid from '~icons/mynaui/lightning-solid'
import { t } from '../../i18n'
import { useSessionsStore } from '../../stores/sessions'
import MessageItem from './MessageItem.vue'
import InputBar from './InputBar.vue'
import PermissionDialog from './PermissionDialog.vue'
import ProcessCapsule from './ProcessCapsule.vue'
import IconButton from '../ui/IconButton.vue'
import ProcessLogPanel from './ProcessLogPanel.vue'
import { getSessionStatusLabel, getSessionStatusTone } from '../../utils/sessionStatus'
import { listSkills, type SkillCatalogEntry } from '../../api/http'

const props = defineProps<{ session: AgentSession }>()
const emit = defineEmits<{
  (e: 'close'): void
}>()

const closeIconBody = '<path fill="currentColor" d="m12 13.4l-4.9 4.9q-.275.275-.7.275t-.7-.275t-.275-.7t.275-.7l4.9-4.9l-4.9-4.9q-.275-.275-.275-.7t.275-.7t.7-.275t.7.275l4.9 4.9l4.9-4.9q.275-.275.7-.275t.7.275t.275.7t-.275.7L13.4 12l4.9 4.9q.275.275.275.7t-.275.7t-.7.275t-.7-.275z"/>'
const AUTO_SCROLL_THRESHOLD = 50
const USER_IDLE_MS = 300

const store = useSessionsStore()
const listRef = ref<HTMLElement | null>(null)
const inputOverlayRef = ref<HTMLElement | null>(null)
const topSentinelRef = ref<HTMLElement | null>(null)
const inputOverlayHeight = ref(112)
const isUserInteracting = ref(false)
const pendingAutoScroll = ref(false)
const lastDistanceToBottom = ref(0)
let inputOverlayResizeObserver: ResizeObserver | null = null
let topSentinelObserver: IntersectionObserver | null = null
let interactionTimer: number | null = null

const statusTone = computed(() => getSessionStatusTone(props.session))
const statusLabel = computed(() => getSessionStatusLabel(statusTone.value))

const mentionCandidates = computed(() =>
  store.sessionList
    .filter(sess => sess.meta.id !== props.session.meta.id)
    .map(sess => ({ id: sess.meta.id, name: sess.meta.name }))
    .sort((a, b) => a.name.localeCompare(b.name))
)

// Skill catalog is fetched once per view and held as a ref; the candidate
// list is derived by joining it with the session's installed_skill_ids.
// The chip's "id" carries the skill NAME (what use_skill expects), since
// skill names — not IDs — are how the LLM references them.
const skillCatalog = ref<SkillCatalogEntry[]>([])

async function refreshSkillCatalog() {
  try {
    const catalog = await listSkills()
    skillCatalog.value = catalog.skills
  } catch (error) {
    console.warn('[agent-view] failed to load skill catalog', error)
  }
}

const skillCandidates = computed(() => {
  const installed = new Set(props.session.meta.installed_skill_ids ?? [])
  return skillCatalog.value
    .filter(skill => installed.has(skill.id))
    .map(skill => ({ id: skill.name, name: skill.name }))
    .sort((a, b) => a.name.localeCompare(b.name))
})

// If the session gains a skill we haven't seen yet (e.g. user drops a new
// skill onto the card), refetch the catalog so the picker shows it.
watch(
  () => props.session.meta.installed_skill_ids ?? [],
  (ids) => {
    const known = new Set(skillCatalog.value.map(s => s.id))
    if (ids.some(id => !known.has(id))) {
      void refreshSkillCatalog()
    }
  },
)

const lastIsUser = computed(() => {
  const msgs = props.session.messages
  return msgs.length > 0 && msgs[msgs.length - 1].role === 'user'
})

function syncInputOverlayHeight() {
  inputOverlayHeight.value = inputOverlayRef.value?.offsetHeight ?? 112
}

function getDistanceToBottom() {
  const el = listRef.value
  if (!el) return Number.POSITIVE_INFINITY
  return Math.max(0, el.scrollHeight - el.scrollTop - el.clientHeight)
}

function syncDistanceToBottom() {
  lastDistanceToBottom.value = getDistanceToBottom()
}

function scrollToBottom() {
  if (!listRef.value) return
  listRef.value.scrollTop = listRef.value.scrollHeight
  syncDistanceToBottom()
}

function clearInteractionTimer() {
  if (interactionTimer == null) return
  window.clearTimeout(interactionTimer)
  interactionTimer = null
}

function flushPendingAutoScroll() {
  if (!pendingAutoScroll.value) return
  pendingAutoScroll.value = false
  if (getDistanceToBottom() > AUTO_SCROLL_THRESHOLD) {
    syncDistanceToBottom()
    return
  }
  nextTick(() => {
    scrollToBottom()
  })
}

function onUserInteraction() {
  isUserInteracting.value = true
  clearInteractionTimer()
  interactionTimer = window.setTimeout(() => {
    interactionTimer = null
    isUserInteracting.value = false
    flushPendingAutoScroll()
  }, USER_IDLE_MS)
}

function onWindowKeydown(event: KeyboardEvent) {
  if (event.defaultPrevented) return
  const target = event.target
  if (target instanceof HTMLElement) {
    const tagName = target.tagName
    if (
      tagName === 'INPUT' ||
      tagName === 'TEXTAREA' ||
      tagName === 'SELECT' ||
      target.isContentEditable
    ) {
      return
    }
  }

  if (!['ArrowUp', 'ArrowDown', 'PageUp', 'PageDown', 'Home', 'End', ' ', 'Spacebar'].includes(event.key)) {
    return
  }
  onUserInteraction()
}

function requestAutoScroll({ force = false } = {}) {
  const wasNearBottom = force || lastDistanceToBottom.value <= AUTO_SCROLL_THRESHOLD

  nextTick(() => {
    if (!listRef.value) return
    if (isUserInteracting.value) {
      pendingAutoScroll.value = true
      syncDistanceToBottom()
      return
    }
    if (wasNearBottom) {
      scrollToBottom()
      return
    }
    syncDistanceToBottom()
  })
}

// Post-exit countdown: when a process ends with status="exited" (clean
// exit, code 0), keep the capsule visible for a few seconds with a blue
// banner so the user sees "it finished" rather than watching the panel
// disappear and wondering if it crashed. Any click on the capsule during
// the countdown freezes the banner ("you cancelled the close") and the
// window stays until a new process runs.
// This block sits above the session.meta.id watcher because that watcher
// runs with immediate: true; the sync initial call would reach into
// resetPostExit() and trip the Temporal Dead Zone if these let/const
// declarations lived below it.
const POST_EXIT_COUNTDOWN_SECONDS = 4
type PostExitState =
  | { kind: 'idle' }
  | { kind: 'counting'; secondsLeft: number }
  | { kind: 'canceled' }
  // Process exited with a non-zero code — something went wrong and the
  // user almost certainly wants to read the output. No auto-close; the
  // capsule sits with a red banner until the user clicks × or another
  // process starts.
  | { kind: 'failed'; exitCode: number }
const postExit = ref<PostExitState>({ kind: 'idle' })
let postExitTimer: number | null = null

const showProcessWindow = computed(() => {
  if (props.session.processState?.active) return true
  return postExit.value.kind !== 'idle'
})

function clearPostExitTimer() {
  if (postExitTimer != null) {
    window.clearInterval(postExitTimer)
    postExitTimer = null
  }
}

function resetPostExit() {
  clearPostExitTimer()
  postExit.value = { kind: 'idle' }
}

function startPostExitCountdown() {
  clearPostExitTimer()
  postExit.value = { kind: 'counting', secondsLeft: POST_EXIT_COUNTDOWN_SECONDS }
  postExitTimer = window.setInterval(() => {
    if (postExit.value.kind !== 'counting') {
      clearPostExitTimer()
      return
    }
    const next = postExit.value.secondsLeft - 1
    if (next <= 0) {
      resetPostExit()
    } else {
      postExit.value = { kind: 'counting', secondsLeft: next }
    }
  }, 1000)
}

function cancelPostExitCountdown() {
  if (postExit.value.kind !== 'counting') return
  clearPostExitTimer()
  postExit.value = { kind: 'canceled' }
}

function enterFailedPostExit(exitCode: number) {
  clearPostExitTimer()
  postExit.value = { kind: 'failed', exitCode }
}

watch(
  () => props.session.meta.id,
  () => {
    clearInteractionTimer()
    isUserInteracting.value = false
    pendingAutoScroll.value = false
    resetPostExit()
    nextTick(() => {
      scrollToBottom()
    })
  },
  { immediate: true },
)

watch(
  () => props.session.messages,
  () => {
    requestAutoScroll()
  },
  { deep: true },
)

watch(
  () => props.session.pendingPermission?.request_id,
  () => {
    requestAutoScroll()
  },
)

function onListScroll() {
  syncDistanceToBottom()
}

async function triggerLoadMoreHistory() {
  const sess = props.session
  if (!sess.hasMoreHistory || sess.isLoadingMoreHistory) return
  const el = listRef.value
  if (!el) return
  // Scroll preservation: anchor on the distance from the current scrollTop
  // to the bottom. After prepending older messages, restore that distance
  // so the viewport stays on the same content instead of snapping upward.
  const anchorFromBottom = el.scrollHeight - el.scrollTop
  await store.loadMoreHistory(sess.meta.id)
  await nextTick()
  if (!listRef.value) return
  listRef.value.scrollTop = listRef.value.scrollHeight - anchorFromBottom
  syncDistanceToBottom()
}

function setupTopSentinelObserver() {
  topSentinelObserver?.disconnect()
  topSentinelObserver = null
  const sentinel = topSentinelRef.value
  const root = listRef.value
  if (!sentinel || !root || typeof IntersectionObserver === 'undefined') return
  topSentinelObserver = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          void triggerLoadMoreHistory()
          break
        }
      }
    },
    { root, rootMargin: '160px 0px 0px 0px', threshold: 0 },
  )
  topSentinelObserver.observe(sentinel)
}

watch(
  [
    () => props.session.hasMoreHistory,
    () => props.session._historyLoaded,
    () => props.session.meta.id,
  ],
  () => {
    nextTick(() => {
      setupTopSentinelObserver()
    })
  },
  { immediate: true },
)

function onSend(payload: { text: string; files: File[] }) {
  store.sendMessage(props.session.meta.id, payload.text, payload.files)
  pendingAutoScroll.value = false
  nextTick(() => {
    scrollToBottom()
  })
}

function onThinkingEnabledChange(enabled: boolean) {
  store.setThinkingEnabled(props.session.meta.id, enabled)
}

function onResolve(decision: 'allow' | 'always' | 'deny', resolution?: Record<string, unknown>) {
  store.resolvePermission(props.session.meta.id, decision, resolution)
}

function onInterrupt() {
  store.interrupt(props.session.meta.id)
}

const logsOpen = ref(false)
const isStopping = ref(false)

// When a process starts, the capsule should appear in its closed state
// and then animate itself open after a short beat — letting the user
// see the capsule materialize before the log panel slides down.
let autoOpenLogsTimer: number | null = null
function clearAutoOpenLogsTimer() {
  if (autoOpenLogsTimer !== null) {
    window.clearTimeout(autoOpenLogsTimer)
    autoOpenLogsTimer = null
  }
}

function onToggleLogs() {
  // Manual toggle wins over the pending auto-open.
  clearAutoOpenLogsTimer()
  logsOpen.value = !logsOpen.value
}

async function onStopProcess() {
  if (isStopping.value) return
  isStopping.value = true
  try {
    await store.stopProcess(props.session.meta.id)
  } catch (err) {
    console.error('[stopProcess] failed:', err)
  } finally {
    isStopping.value = false
  }
}

watch(
  () => props.session.processState?.active,
  (active, wasActive) => {
    if (!active) {
      logsOpen.value = false
      clearAutoOpenLogsTimer()
    }
    if (active) {
      // A new process started — discard any lingering post-exit banner.
      resetPostExit()
      // Fresh start (was inactive → now active): show closed first, then
      // auto-expand so the user sees the capsule appear and slide open.
      // Skip if already open (defensive — covers consecutive replacements).
      if (!wasActive && !logsOpen.value) {
        clearAutoOpenLogsTimer()
        autoOpenLogsTimer = window.setTimeout(() => {
          logsOpen.value = true
          autoOpenLogsTimer = null
        }, 220)
      }
      return
    }
    // Post-exit routing:
    //   - status 'exited' + exit_code 0       → 4-second countdown banner
    //   - status 'exited' + exit_code non-zero → persistent red banner
    //   - status 'killed' (user or agent stop) → hide immediately
    //   - status 'failed' (exec init error)    → hide immediately
    // Note: the backend classifies non-zero exits under status='exited'
    // too, so we disambiguate via exit_code here.
    if (!wasActive) {
      resetPostExit()
      return
    }
    const st = props.session.processState
    if (st?.status === 'exited') {
      const code = st.exit_code ?? 0
      if (code === 0) {
        startPostExitCountdown()
      } else {
        enterFailedPostExit(code)
      }
    } else {
      resetPostExit()
    }
  },
)

onMounted(() => {
  nextTick(() => {
    syncInputOverlayHeight()
    syncDistanceToBottom()
    if (!inputOverlayRef.value || typeof ResizeObserver === 'undefined') return
    inputOverlayResizeObserver = new ResizeObserver(() => {
      syncInputOverlayHeight()
    })
    inputOverlayResizeObserver.observe(inputOverlayRef.value)
  })
  window.addEventListener('keydown', onWindowKeydown, true)
  void refreshSkillCatalog()
})

onBeforeUnmount(() => {
  clearAutoOpenLogsTimer()
  inputOverlayResizeObserver?.disconnect()
  inputOverlayResizeObserver = null
  topSentinelObserver?.disconnect()
  topSentinelObserver = null
  clearInteractionTimer()
  clearPostExitTimer()
  window.removeEventListener('keydown', onWindowKeydown, true)
})

watch(inputOverlayHeight, () => {
  requestAutoScroll()
})
</script>

<style scoped>
.chat-shell {
  --input-overlay-height: 112px;
  height: 100%;
  display: flex;
  flex-direction: column;
  background: var(--bg);
  color: var(--ink);
  position: relative;
  overflow: hidden;
}

.input-overlay {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 10;
  padding: 0 16px 16px;
  pointer-events: none;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.process-window {
  display: flex;
  flex-direction: column;
  background: var(--panel);
  border: 1px solid var(--rule-soft);
  pointer-events: auto;
  overflow: hidden;
  transition: border-color .15s, box-shadow .15s;
}

.process-window.expanded {
  border-color: var(--rule);
}

.process-window.expanded:focus-within {
  box-shadow: 2px 2px 0 0 var(--rule);
}

.process-capsule {
  align-self: stretch;
}

.process-log-panel {
  align-self: stretch;
  border-top: 1px solid var(--rule-soft);
}

.log-reveal-enter-active,
.log-reveal-leave-active {
  transition:
    max-height 260ms cubic-bezier(0.22, 0.61, 0.36, 1),
    opacity 180ms ease,
    border-top-color 160ms ease;
  overflow: hidden;
}

.log-reveal-enter-from,
.log-reveal-leave-to {
  max-height: 0;
  opacity: 0;
  border-top-color: transparent;
}

.log-reveal-enter-to,
.log-reveal-leave-from {
  max-height: 260px;
  opacity: 1;
  border-top-color: var(--rule-soft);
}

/* ── Chrome row ─────────────────────────────────────────────────── */
.chat-chrome {
  height: 36px;
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 8px 0 14px;
  background: var(--panel);
  border-bottom: 1px solid var(--rule-soft);
  user-select: none;
  -webkit-app-region: drag;
}

.brand {
  display: inline-flex;
  align-items: center;
}

.brand-text {
  font-family: var(--mono);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.14em;
  color: var(--ink);
}

.chrome-divider {
  width: 1px;
  height: 14px;
  background: var(--rule-soft);
}

.chrome-name {
  min-width: 0;
  font-size: 12px;
  font-weight: 500;
  color: var(--ink-2);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.chrome-spacer {
  flex: 1;
}

.status-tag {
  -webkit-app-region: no-drag;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 2px 7px;
  border: 1px solid currentColor;
  font-family: var(--mono);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  white-space: nowrap;
  color: var(--ink-4);
}

.status-tag.running  { color: var(--ok); }
.status-tag.approval { color: var(--warn); }
.status-tag.error    { color: var(--err); }

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.status-tag.running .status-dot,
.status-tag.approval .status-dot {
  animation: bienePulse 1.6s ease-in-out infinite;
}

.close-icon {
  width: 14px;
  height: 14px;
}

/* ── Message list ──────────────────────────────────────────────── */
.message-list {
  position: relative;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 4px 28px calc(var(--input-overlay-height) + 20px);
  scrollbar-width: thin;
  scrollbar-color: var(--rule-soft) transparent;
}

.history-sentinel {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 8px 0 4px;
  min-height: 20px;
}

.history-spinner {
  width: 12px;
  height: 12px;
  border: 2px solid color-mix(in srgb, var(--ink) 14%, transparent);
  border-top-color: var(--ink-3);
  animation: bieneSpin 0.8s linear infinite;
}

.message-list::-webkit-scrollbar {
  width: 10px;
}

.message-list::-webkit-scrollbar-track {
  background: transparent;
}

.message-list::-webkit-scrollbar-thumb {
  background: var(--rule-soft);
  border: 2px solid var(--bg);
}

.message-list::-webkit-scrollbar-thumb:hover {
  background: var(--rule);
}

.empty-chat {
  position: absolute;
  inset: 4px 28px calc(var(--input-overlay-height) + 20px) 28px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  text-align: center;
}

.empty-icon {
  width: 38px;
  height: 38px;
  color: var(--accent);
  flex-shrink: 0;
}

.empty-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--ink-2);
  max-width: 28ch;
}

.empty-dir {
  margin: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  font-family: var(--mono);
  font-size: 11px;
  color: var(--ink-4);
  letter-spacing: 0.04em;
  justify-content: center;
  flex-wrap: wrap;
  max-width: min(100%, 52ch);
}

.empty-dir-label {
  text-transform: uppercase;
  letter-spacing: 0.18em;
  color: var(--ink-4);
}

.empty-dir-path {
  color: var(--ink-3);
}

/* Typing indicator */
.typing {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 0 12px 2px;
}

.typing-dot {
  width: 6px;
  height: 6px;
  background: var(--ink-3);
  animation: bieneBlink 1.1s ease-in-out infinite;
}

.typing-dot:nth-child(2) { animation-delay: .15s; }
.typing-dot:nth-child(3) { animation-delay: .30s; }

</style>
