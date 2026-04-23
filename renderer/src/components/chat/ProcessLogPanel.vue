<template>
  <div class="log-panel" :style="{ '--panel-height': `${height}px` }">
    <div ref="bodyRef" class="log-body">
      <pre v-if="lines.length > 0" class="log-text">{{ lines.join('\n') }}</pre>
      <div v-else class="log-empty">{{ t('process.noLogs') }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { ProcessStateData } from '../../types/events'
import { connectProcessLogsWS } from '../../api/ws'
import { t } from '../../i18n'

const props = defineProps<{
  sessionId: string
  state: ProcessStateData | null
  height?: number
}>()

const height = computed(() => props.height ?? 220)

const MAX_LINES = 1000

const lines = ref<string[]>([])
const bodyRef = ref<HTMLElement | null>(null)
let disconnect: (() => void) | null = null

function appendLine(line: string) {
  lines.value.push(line)
  if (lines.value.length > MAX_LINES) {
    lines.value.splice(0, lines.value.length - MAX_LINES)
  }
  nextTick(() => {
    const el = bodyRef.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

function openConnection(id: string) {
  lines.value = []
  disconnect = connectProcessLogsWS(id, {
    onLine: (line) => appendLine(line),
    onState: () => {
      // State updates arrive on the main session WS as well; ignored here.
    },
  })
}

function closeConnection() {
  if (disconnect) {
    disconnect()
    disconnect = null
  }
}

onMounted(() => {
  openConnection(props.sessionId)
})

onBeforeUnmount(() => {
  closeConnection()
})

watch(
  () => props.sessionId,
  (id, oldId) => {
    if (id === oldId) return
    closeConnection()
    openConnection(id)
  },
)

</script>

<style scoped>
.log-panel {
  --panel-height: 220px;
  display: flex;
  flex-direction: column;
  height: var(--panel-height);
  background: var(--panel);
  overflow: hidden;
  pointer-events: auto;
}

.log-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 10px 12px;
  scrollbar-width: thin;
  scrollbar-color: var(--rule-soft) transparent;
}

.log-body::-webkit-scrollbar {
  width: 10px;
}

.log-body::-webkit-scrollbar-thumb {
  background: var(--rule-soft);
  border: 2px solid var(--panel);
}

.log-text {
  margin: 0;
  font-family: var(--mono);
  font-size: 11.5px;
  line-height: 1.5;
  color: var(--ink-2);
  white-space: pre-wrap;
  word-break: break-all;
}

.log-empty {
  font-family: var(--mono);
  font-size: 11px;
  color: var(--ink-4);
  text-align: center;
  padding: 20px 0;
}
</style>
