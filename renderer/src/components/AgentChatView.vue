<template>
  <div class="chat-shell">
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
      <button class="close-btn" type="button" :aria-label="t('common.close')" @click="emit('close')">
        <svg viewBox="0 0 24 24" aria-hidden="true" v-html="closeIconBody" />
      </button>
    </header>

    <!-- Session meta strip -->
    <div class="meta-strip">
      <div class="title-col">
        <div class="session-name">{{ session.meta.name }}</div>
        <div class="session-id">{{ session.meta.id }}</div>
      </div>
      <div class="workdir-col">
        <div class="workdir-label">{{ t('agent.workDirLabel') }}</div>
        <div class="workdir-path" :title="session.meta.work_dir">{{ session.meta.work_dir }}</div>
      </div>
      <div class="profile-col">
        <div v-if="domainLabel" class="chip">{{ domainLabel }}</div>
        <div v-if="styleLabel" class="chip">{{ styleLabel }}</div>
      </div>
      <div class="perm-col">
        <div class="perm-chip" :class="{ on: session.meta.permissions.execute }">EXEC</div>
        <div class="perm-chip" :class="{ on: session.meta.permissions.write }">WRITE</div>
        <div class="perm-chip" :class="{ on: session.meta.permissions.send_to_agent }">SEND</div>
      </div>
    </div>

    <!-- Message list -->
    <div ref="listRef" class="message-list" @scroll="onListScroll">
      <div v-if="session.messages.length === 0" class="empty-chat">
        <div class="empty-frame" aria-hidden="true">⚡</div>
        <p class="empty-title">{{ t('agent.ready') }}</p>
        <p class="empty-dir">
          <span class="empty-dir-label">{{ t('agent.workDirLabel') }}</span>
          <span class="empty-dir-path">{{ session.meta.work_dir }}</span>
        </p>
      </div>
      <MessageItem
        v-for="msg in session.messages"
        :key="msg.id"
        :msg="msg"
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
        <span v-if="activeSkillLabel" class="skill-indicator">{{ activeSkillLabel }}</span>
      </div>
    </div>

    <InputBar
      :disabled="session.isStreaming"
      :interruptible="session.isStreaming"
      :interrupting="session.isInterrupting"
      @send="onSend"
      @interrupt="onInterrupt"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import type { AgentSession } from '../stores/sessions'
import { t } from '../i18n'
import { useSessionsStore } from '../stores/sessions'
import MessageItem from './MessageItem.vue'
import InputBar from './InputBar.vue'
import PermissionDialog from './PermissionDialog.vue'
import { getSessionStatusLabel, getSessionStatusTone } from '../utils/sessionStatus'
import { findDomainOption, findStyleOption } from '../utils/profile'

const props = defineProps<{ session: AgentSession }>()
const emit = defineEmits<{
  (e: 'close'): void
}>()

const closeIconBody = '<path fill="currentColor" d="m12 13.4l-4.9 4.9q-.275.275-.7.275t-.7-.275t-.275-.7t.275-.7l4.9-4.9l-4.9-4.9q-.275-.275-.275-.7t.275-.7t.7-.275t.7.275l4.9 4.9l4.9-4.9q.275-.275.7-.275t.7.275t.275.7t-.275.7L13.4 12l4.9 4.9q.275.275.275.7t-.275.7t-.7.275t-.7-.275z"/>'

const store = useSessionsStore()
const listRef = ref<HTMLElement | null>(null)
const stickToBottom = ref(true)

const statusTone = computed(() => getSessionStatusTone(props.session))
const statusLabel = computed(() => getSessionStatusLabel(statusTone.value))

const domainLabel = computed(() =>
  findDomainOption(props.session.meta.profile.domain)?.label ?? ''
)
const styleLabel = computed(() =>
  findStyleOption(props.session.meta.profile.style)?.label ?? ''
)

const activeSkillLabel = computed(() =>
  props.session.activeSkillName ? t('agent.usingSkill', { name: props.session.activeSkillName }) : ''
)
const lastIsUser = computed(() => {
  const msgs = props.session.messages
  return msgs.length > 0 && msgs[msgs.length - 1].role === 'user'
})

watch(
  () => props.session.meta.id,
  () => {
    stickToBottom.value = true
    nextTick(() => {
      if (listRef.value) listRef.value.scrollTop = listRef.value.scrollHeight
    })
  },
  { immediate: true },
)

watch(
  () => props.session.messages,
  () => {
    if (!stickToBottom.value) return
    nextTick(() => {
      if (listRef.value) listRef.value.scrollTop = listRef.value.scrollHeight
    })
  },
  { deep: true },
)

watch(
  () => props.session.pendingPermission?.request_id,
  () => {
    if (!stickToBottom.value) return
    nextTick(() => {
      if (listRef.value) listRef.value.scrollTop = listRef.value.scrollHeight
    })
  },
)

function onListScroll() {
  const el = listRef.value
  if (!el) return
  stickToBottom.value = el.scrollHeight - el.scrollTop - el.clientHeight < 30
}

function onSend(text: string) {
  store.sendMessage(props.session.meta.id, text)
  stickToBottom.value = true
  nextTick(() => {
    if (listRef.value) listRef.value.scrollTop = listRef.value.scrollHeight
  })
}

function onResolve(decision: 'allow' | 'always' | 'deny') {
  store.resolvePermission(props.session.meta.id, decision)
}

function onInterrupt() {
  store.interrupt(props.session.meta.id)
}
</script>

<style scoped>
.chat-shell {
  height: 100%;
  display: flex;
  flex-direction: column;
  background: var(--bg);
  color: var(--ink);
  position: relative;
  overflow: hidden;
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
  border-bottom: 1px solid var(--rule);
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

.close-btn {
  -webkit-app-region: no-drag;
  width: 26px;
  height: 26px;
  display: grid;
  place-items: center;
  background: transparent;
  border: 1px solid transparent;
  color: var(--ink-3);
  cursor: pointer;
  transition: background .12s, color .12s, border-color .12s;
}

.close-btn:hover {
  background: var(--bg-2);
  border-color: var(--rule-softer);
  color: var(--ink);
}

.close-btn svg {
  width: 14px;
  height: 14px;
}

/* ── Meta strip ────────────────────────────────────────────────── */
.meta-strip {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 10px 20px;
  border-bottom: 1px solid var(--rule-soft);
  background: var(--panel-2);
  flex: 0 0 auto;
}

.title-col {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.session-name {
  font-family: var(--sans);
  font-size: 17px;
  font-weight: 700;
  letter-spacing: -0.01em;
  line-height: 1.15;
  color: var(--ink);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-id {
  margin-top: 3px;
  font-family: var(--mono);
  font-size: 11px;
  color: var(--ink-4);
  letter-spacing: 0.02em;
}

.workdir-col {
  flex: 1;
  min-width: 0;
}

.workdir-label {
  font-family: var(--mono);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.18em;
  color: var(--ink-4);
  text-transform: uppercase;
}

.workdir-path {
  margin-top: 2px;
  font-family: var(--mono);
  font-size: 12px;
  color: var(--ink-2);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.profile-col {
  display: flex;
  gap: 6px;
  flex-shrink: 0;
}

.chip {
  font-family: var(--mono);
  font-size: 10px;
  letter-spacing: 0.08em;
  padding: 3px 7px;
  color: var(--ink-3);
  border: 1px solid var(--rule-softer);
  background: var(--panel-2);
}

.perm-col {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}

.perm-chip {
  font-family: var(--mono);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.1em;
  padding: 3px 6px;
  color: var(--ink-4);
  background: transparent;
  border: 1px solid var(--rule-softer);
  text-decoration: line-through;
  opacity: 0.6;
}

.perm-chip.on {
  color: var(--ink-2);
  background: var(--bg-2);
  border-color: var(--rule-soft);
  text-decoration: none;
  opacity: 1;
}

/* ── Message list ──────────────────────────────────────────────── */
.message-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 4px 28px 20px;
  scrollbar-width: thin;
  scrollbar-color: var(--rule-soft) transparent;
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
  padding: 80px 20px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  text-align: center;
}

.empty-frame {
  width: 52px;
  height: 52px;
  display: grid;
  place-items: center;
  border: 1px solid var(--rule);
  color: var(--ink-3);
  margin-bottom: 4px;
  font-size: 22px;
}

.empty-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--ink-2);
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

.skill-indicator {
  font-family: var(--mono);
  font-size: 11px;
  color: var(--accent);
  letter-spacing: 0.08em;
}
</style>
