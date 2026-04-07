<template>
  <Teleport to="body">
    <div v-if="active" class="modal-backdrop" @click.self="store.setActive(null)">
      <div class="modal">
        <header class="modal-header">
          <div class="modal-title">
            <span class="modal-name">{{ active.meta.name }}</span>
            <span class="modal-dir">{{ active.meta.work_dir }}</span>
          </div>
          <div class="modal-header-right">
            <span class="badge" :class="activeStatusTone">{{ activeStatusLabel }}</span>
            <button class="close-btn" @click="store.setActive(null)">✕</button>
          </div>
        </header>

        <div ref="listRef" class="message-list" @scroll="onListScroll">
          <div v-if="active.messages.length === 0" class="empty-chat">
            <div class="empty-icon">⚡</div>
            <p>Agent ready. Send a message to start.</p>
            <p class="empty-dir">Working directory: <code>{{ active.meta.work_dir }}</code></p>
          </div>
          <MessageItem
            v-for="msg in active.messages"
            :key="msg.id"
            :msg="msg"
          />
          <PermissionDialog
            v-if="active.pendingPermission"
            :req="active.pendingPermission"
            @resolve="onResolve"
          />
          <div v-if="active.isStreaming && lastIsUser && !active.pendingPermission" class="typing">
            <span /><span /><span />
          </div>
        </div>

        <InputBar
          :disabled="active.isStreaming"
          :interruptible="active.isStreaming"
          :interrupting="active.isInterrupting"
          @send="onSend"
          @interrupt="onInterrupt"
        />
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useSessionsStore } from '../stores/sessions'
import MessageItem from './MessageItem.vue'
import InputBar from './InputBar.vue'
import PermissionDialog from './PermissionDialog.vue'
import { getSessionStatusLabel, getSessionStatusTone } from '../utils/sessionStatus'

const store = useSessionsStore()
const listRef = ref<HTMLElement | null>(null)
const stickToBottom = ref(true)

const active = computed(() => store.activeSession)
const activeStatusTone = computed(() =>
  active.value ? getSessionStatusTone(active.value) : 'idle'
)
const activeStatusLabel = computed(() =>
  getSessionStatusLabel(activeStatusTone.value)
)

const lastIsUser = computed(() => {
  const msgs = active.value?.messages ?? []
  return msgs.length > 0 && msgs[msgs.length - 1].role === 'user'
})

// Snap to bottom when modal first opens.
watch(() => store.activeId, (id) => {
  if (id) {
    stickToBottom.value = true
    nextTick(() => {
      if (listRef.value) listRef.value.scrollTop = listRef.value.scrollHeight
    })
  }
})

watch(
  () => active.value?.messages,
  () => {
    if (!stickToBottom.value) return
    nextTick(() => {
      if (listRef.value) listRef.value.scrollTop = listRef.value.scrollHeight
    })
  },
  { deep: true }
)

watch(
  () => active.value?.pendingPermission?.request_id,
  () => {
    if (!stickToBottom.value) return
    nextTick(() => {
      if (listRef.value) listRef.value.scrollTop = listRef.value.scrollHeight
    })
  }
)

function onListScroll() {
  const el = listRef.value
  if (!el) return
  stickToBottom.value = el.scrollHeight - el.scrollTop - el.clientHeight < 30
}

function onSend(text: string) {
  if (!active.value) return
  store.sendMessage(active.value.meta.id, text)
  stickToBottom.value = true
  nextTick(() => {
    if (listRef.value) listRef.value.scrollTop = listRef.value.scrollHeight
  })
}

function onResolve(decision: 'allow' | 'always' | 'deny') {
  if (active.value) store.resolvePermission(active.value.meta.id, decision)
}

function onInterrupt() {
  if (active.value) store.interrupt(active.value.meta.id)
}
</script>

<style scoped>
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, .45);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
  padding: 24px;
}

.modal {
  background: #fff;
  border-radius: 16px;
  width: 100%;
  max-width: 780px;
  height: 80vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 24px 64px rgba(0, 0, 0, .2);
}

.modal-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 0 20px; height: 56px; flex-shrink: 0;
  border-bottom: 1px solid #e5e7eb;
}
.modal-title { display: flex; flex-direction: column; min-width: 0; }
.modal-name  { font-size: 15px; font-weight: 700; color: #111827; }
.modal-dir   { font-size: 11px; color: #9ca3af; font-family: monospace;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.modal-header-right { display: flex; align-items: center; gap: 10px; flex-shrink: 0; }

.badge {
  font-size: 11px; font-weight: 600; padding: 3px 10px; border-radius: 999px;
  text-transform: uppercase; background: #f3f4f6; color: #6b7280;
}
.badge.approval { background: #fef3c7; color: #92400e; }
.badge.running { background: #d1fae5; color: #065f46; }
.badge.error   { background: #fee2e2; color: #991b1b; }

.close-btn {
  background: none; border: none; cursor: pointer; color: #9ca3af;
  font-size: 16px; padding: 4px 6px; border-radius: 6px; line-height: 1;
  transition: background .15s, color .15s;
}
.close-btn:hover { background: #f3f4f6; color: #374151; }

.message-list {
  flex: 1; overflow-y: auto; padding: 20px;
  display: flex; flex-direction: column;
}

.empty-chat {
  flex: 1; display: flex; flex-direction: column; align-items: center;
  justify-content: center; color: #9ca3af; gap: 8px; text-align: center;
}
.empty-icon { font-size: 40px; }
.empty-chat p { font-size: 15px; margin: 0; }
.empty-dir { font-size: 12px !important; }
.empty-dir code { font-size: 12px; background: #f3f4f6; padding: 2px 6px; border-radius: 4px; color: #374151; }

.typing { display: flex; gap: 4px; padding: 12px 0; align-items: center; }
.typing span {
  width: 7px; height: 7px; background: #9ca3af; border-radius: 50%;
  animation: bounce .9s ease-in-out infinite;
}
.typing span:nth-child(2) { animation-delay: .15s; }
.typing span:nth-child(3) { animation-delay: .3s; }
@keyframes bounce { 0%,60%,100% { transform: translateY(0); } 30% { transform: translateY(-6px); } }
</style>
