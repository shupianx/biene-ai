<template>
  <div v-if="visible" class="capsule-wrap">
    <button
      class="capsule"
      :class="statusClass"
      type="button"
      :title="fullCommand"
      @click="onCapsuleClick"
    >
      <span class="dot" />
      <span class="command">{{ commandLabel }}</span>
      <span
        v-if="postExitBanner"
        class="post-exit-banner"
        :class="{
          canceled: postExit?.kind === 'canceled',
          failed: postExit?.kind === 'failed',
        }"
      >
        {{ postExitBanner }}
      </span>
      <span v-else-if="cwdDisplay" class="cwd" :title="cwd">{{ cwdDisplay }}</span>
      <span class="elapsed">{{ elapsedLabel }}</span>
    </button>
    <button
      class="capsule-action icon-only"
      type="button"
      :aria-label="logsOpen ? t('process.collapse') : t('process.expand')"
      :title="logsOpen ? t('process.collapse') : t('process.expand')"
      @click="emit('toggle-logs')"
    >
      <MaterialSymbolsMinimize v-if="logsOpen" class="action-icon" aria-hidden="true" />
      <GgMaximize v-else class="action-icon" aria-hidden="true" />
    </button>
    <button
      v-if="state?.active"
      class="capsule-action icon-only danger"
      type="button"
      :disabled="stopping"
      :aria-label="stopping ? t('process.stopping') : t('process.stop')"
      :title="stopping ? t('process.stopping') : t('process.stop')"
      @click="onStop"
    >
      <MaterialSymbolsStopCircleOutline class="action-icon" aria-hidden="true" />
    </button>
    <button
      v-else
      class="capsule-action icon-only"
      type="button"
      :aria-label="t('process.close')"
      :title="t('process.close')"
      @click="onClose"
    >
      <MaterialSymbolsClose class="action-icon" aria-hidden="true" />
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref } from 'vue'
import GgMaximize from '~icons/gg/maximize'
import MaterialSymbolsMinimize from '~icons/material-symbols/minimize'
import MaterialSymbolsStopCircleOutline from '~icons/material-symbols/stop-circle-outline'
import MaterialSymbolsClose from '~icons/material-symbols/close'
import type { ProcessStateData } from '../../types/events'
import { t } from '../../i18n'

// Post-exit UI state handed down from AgentChatView after a process ends
// cleanly. The capsule stays visible during the countdown and shows a
// banner in place of the cwd; any click on the capsule itself cancels
// the countdown and freezes the banner into its "cancelled" form.
export type PostExitState =
  | { kind: 'idle' }
  | { kind: 'counting'; secondsLeft: number }
  | { kind: 'canceled' }
  | { kind: 'failed'; exitCode: number }

const props = defineProps<{
  state: ProcessStateData | null
  logsOpen: boolean
  stopping?: boolean
  postExit?: PostExitState
}>()

const emit = defineEmits<{
  (e: 'toggle-logs'): void
  (e: 'stop'): void
  (e: 'cancel-post-exit'): void
  (e: 'close'): void
}>()

const visible = computed(() => {
  if (props.state?.active) return true
  return props.postExit && props.postExit.kind !== 'idle'
})

const postExitBanner = computed(() => {
  const pe = props.postExit
  if (!pe) return ''
  if (pe.kind === 'counting') return `执行完毕，${pe.secondsLeft} 秒后关闭窗口`
  if (pe.kind === 'canceled') return '您终止了窗口关闭'
  if (pe.kind === 'failed') return `进程非正常退出（退出码 ${pe.exitCode}）`
  return ''
})

function onCapsuleClick() {
  if (props.postExit?.kind === 'counting') {
    emit('cancel-post-exit')
    return
  }
  emit('toggle-logs')
}

const fullCommand = computed(() => {
  const s = props.state
  if (!s?.command) return ''
  const parts = [s.command, ...(s.args ?? [])]
  return parts.join(' ')
})

const commandLabel = computed(() => fullCommand.value || '')

const cwd = computed(() => props.state?.cwd || '')

// Trim from the front so the trailing segments (the actual project / file
// name) stay visible when the path is long.
const CWD_MAX_CHARS = 28
const cwdDisplay = computed(() => {
  const p = cwd.value
  if (!p) return ''
  if (p.length <= CWD_MAX_CHARS) return p
  return '…' + p.slice(p.length - CWD_MAX_CHARS + 1)
})

const statusClass = computed(() => {
  const status = props.state?.status ?? 'idle'
  return `status-${status}`
})

// Elapsed timer
const now = ref(Date.now())
let timer: number | null = null

onMounted(() => {
  timer = window.setInterval(() => {
    now.value = Date.now()
  }, 1000)
})

onBeforeUnmount(() => {
  if (timer !== null) {
    window.clearInterval(timer)
    timer = null
  }
})

const elapsedLabel = computed(() => {
  const s = props.state
  if (!s?.started_at) return '00:00'
  const startedMs = Date.parse(s.started_at)
  if (!Number.isFinite(startedMs)) return '00:00'
  // Freeze the counter at exit time once the process stops running, so
  // the capsule's final resting label reflects the actual runtime rather
  // than ticking on forever through the post-exit countdown.
  let endMs = now.value
  if (!s.active && s.exited_at) {
    const exitedMs = Date.parse(s.exited_at)
    if (Number.isFinite(exitedMs)) endMs = exitedMs
  }
  const totalSec = Math.max(0, Math.floor((endMs - startedMs) / 1000))
  const hours = Math.floor(totalSec / 3600)
  const minutes = Math.floor((totalSec % 3600) / 60)
  const seconds = totalSec % 60
  const pad = (n: number) => n.toString().padStart(2, '0')
  if (hours > 0) return `${pad(hours)}:${pad(minutes)}:${pad(seconds)}`
  return `${pad(minutes)}:${pad(seconds)}`
})

function onStop() {
  emit('stop')
}

function onClose() {
  emit('close')
}
</script>

<style scoped>
.capsule-wrap {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 6px 5px 6px 10px;
  background: transparent;
  max-width: 100%;
  pointer-events: auto;
  font-family: var(--mono);
  font-size: 11px;
  color: var(--ink-2);
}

.capsule {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  flex: 1;
  padding: 0 4px;
  background: transparent;
  border: 0;
  color: inherit;
  cursor: pointer;
  font: inherit;
  text-align: left;
}

.dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--ink-4);
  flex-shrink: 0;
}

.capsule.status-running .dot {
  background: var(--ok);
  animation: bienePulse 1.6s ease-in-out infinite;
}

.capsule.status-exited .dot  { background: var(--ink-4); }
.capsule.status-killed .dot  { background: var(--warn); }
.capsule.status-failed .dot  { background: var(--err); }

.command {
  font-weight: 600;
  color: var(--ink);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 40ch;
}

.cwd,
.elapsed {
  color: var(--ink-3);
  white-space: nowrap;
}

.cwd {
  margin-left: 10px;
}

.post-exit-banner {
  margin-left: 10px;
  padding: 2px 8px;
  background: var(--info, #3b82f6);
  color: #fff;
  letter-spacing: 0.02em;
  white-space: nowrap;
  border-radius: 2px;
  transition: background .15s;
}

.post-exit-banner.canceled {
  background: var(--ink-3);
}

.post-exit-banner.failed {
  background: var(--err);
}

.elapsed {
  margin-left: auto;
  padding-left: 8px;
  flex-shrink: 0;
}

.capsule-action {
  flex-shrink: 0;
  padding: 2px 10px;
  background: transparent;
  border: none;
  color: var(--ink-2);
  font: inherit;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  cursor: pointer;
  transition: background .12s, color .12s;
}

.capsule-action.icon-only {
  width: 24px;
  height: 22px;
  padding: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  letter-spacing: 0;
}

.action-icon {
  width: 14px;
  height: 14px;
  flex-shrink: 0;
}

.capsule-action:hover:not(:disabled) {
  background: var(--bg-2);
  color: var(--ink);
}

.capsule-action:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.capsule-action.danger {
  color: var(--err);
}

.capsule-action.danger:hover:not(:disabled) {
  background: color-mix(in srgb, var(--err) 10%, transparent);
  color: var(--err);
}
</style>
