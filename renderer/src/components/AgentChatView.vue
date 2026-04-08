<template>
  <div class="chat-shell">
    <header class="chat-header">
      <div class="chat-title">
        <span class="chat-name">{{ session.meta.name }}</span>
        <span class="chat-dir">{{ session.meta.work_dir }}</span>
      </div>
      <div class="chat-header-right">
        <span class="badge" :class="statusTone">{{ statusLabel }}</span>
        <button class="close-btn" @click="emit('close')">✕</button>
      </div>
    </header>

    <div ref="listRef" class="message-list" @scroll="onListScroll">
      <div v-if="session.messages.length === 0" class="empty-chat">
        <div class="empty-icon">⚡</div>
        <p>Agent ready. Send a message to start.</p>
        <p class="empty-dir">Working directory: <code>{{ session.meta.work_dir }}</code></p>
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
        <span /><span /><span />
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
import { useSessionsStore } from '../stores/sessions'
import MessageItem from './MessageItem.vue'
import InputBar from './InputBar.vue'
import PermissionDialog from './PermissionDialog.vue'
import { getSessionStatusLabel, getSessionStatusTone } from '../utils/sessionStatus'

const props = defineProps<{ session: AgentSession }>()
const emit = defineEmits<{
  (e: 'close'): void
}>()

const store = useSessionsStore()
const listRef = ref<HTMLElement | null>(null)
const stickToBottom = ref(true)

const statusTone = computed(() => getSessionStatusTone(props.session))
const statusLabel = computed(() => getSessionStatusLabel(statusTone.value))
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
  background: var(--app-bg);
}

.chat-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  height: 56px;
  flex-shrink: 0;
  border-bottom: 1px solid #e5e7eb;
  background: var(--app-bg);
  -webkit-app-region: drag;
  user-select: none;
}

.chat-title {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.chat-name {
  font-size: 15px;
  font-weight: bold;
  color: #111827;
}

.chat-dir {
  font-size: 11px;
  color: #9ca3af;
  font-family: monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.chat-header-right {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
  -webkit-app-region: no-drag;
}

.badge {
  font-size: 11px;
  font-weight: bold;
  padding: 3px 10px;
  border-radius: 999px;
  text-transform: uppercase;
  background: #f3f4f6;
  color: #6b7280;
}

.badge.approval { background: #fef3c7; color: #92400e; }
.badge.running { background: #d1fae5; color: #065f46; }
.badge.error   { background: #fee2e2; color: #991b1b; }

.close-btn {
  background: none;
  border: none;
  cursor: pointer;
  color: #9ca3af;
  font-size: 16px;
  padding: 4px 6px;
  border-radius: 6px;
  line-height: 1;
  transition: background .15s, color .15s;
}

.close-btn:hover {
  background: #fff2e8;
  color: #374151;
}

.message-list {
  flex: 1;
  overflow-y: auto;
  padding: 20px 28px;
  display: flex;
  flex-direction: column;
  scrollbar-width: thin;
  scrollbar-color: #d6d3d1 #fffaf5;
}

.message-list::-webkit-scrollbar {
  width: 10px;
}

.message-list::-webkit-scrollbar-track {
  background: #fffaf5;
  border-radius: 999px;
}

.message-list::-webkit-scrollbar-thumb {
  background: #d6d3d1;
  border: 2px solid #fffaf5;
  border-radius: 999px;
}

.message-list::-webkit-scrollbar-thumb:hover {
  background: #a8a29e;
}

.empty-chat {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: #9ca3af;
  gap: 8px;
  text-align: center;
}

.empty-icon { font-size: 40px; }
.empty-chat p { font-size: 15px; margin: 0; }
.empty-dir { font-size: 12px !important; }
.empty-dir code {
  font-size: 12px;
  background: #fff;
  padding: 2px 6px;
  border-radius: 4px;
  color: #374151;
}

.typing {
  display: flex;
  gap: 4px;
  padding: 12px 0;
  align-items: center;
}

.typing span {
  width: 7px;
  height: 7px;
  background: #9ca3af;
  border-radius: 50%;
  animation: bounce .9s ease-in-out infinite;
}

.typing span:nth-child(2) { animation-delay: .15s; }
.typing span:nth-child(3) { animation-delay: .3s; }

@keyframes bounce {
  0%, 60%, 100% { transform: translateY(0); }
  30% { transform: translateY(-6px); }
}
</style>
