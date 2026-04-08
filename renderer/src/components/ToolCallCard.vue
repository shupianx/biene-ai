<template>
  <div class="tool-card" :class="statusClass">
    <div class="tool-header" @click="expanded = !expanded">
      <span class="tool-icon">{{ icon }}</span>
      <span class="tool-name">{{ tc.tool_name }}</span>
      <span class="tool-summary">{{ tc.tool_summary }}</span>
      <span class="expand-icon">{{ expanded ? '▲' : '▼' }}</span>
    </div>
    <div v-if="expanded" class="tool-body">
      <div v-if="tc.tool_input" class="tool-section">
        <div class="section-label">Input</div>
        <pre class="code-block">{{ fmt(tc.tool_input) }}</pre>
      </div>
      <div v-if="tc.result" class="tool-section">
        <div class="section-label">Output</div>
        <pre class="code-block">{{ tc.result }}</pre>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { DisplayTool } from '../api/http'

const props = defineProps<{ tc: DisplayTool }>()
const expanded = ref(false)

const statusClass = computed(() => ({
  'status-composing': props.tc.status === 'composing',
  'status-pending': props.tc.status === 'pending',
  'status-done':    props.tc.status === 'done',
  'status-error':   props.tc.status === 'error',
  'status-denied':  props.tc.status === 'denied',
  'status-cancelled': props.tc.status === 'cancelled',
}))

const icon = computed(() => (
  { composing: '…', pending: '⟳', done: '✓', error: '✗', denied: '⊘', cancelled: '■' }[props.tc.status] ?? '?'
))

function fmt(v: unknown) {
  try { return JSON.stringify(v, null, 2) } catch { return String(v) }
}
</script>

<style scoped>
.tool-card { border-radius: 8px; border: 1px solid #e5e7eb; margin: 6px 0; overflow: hidden; font-size: 13px; }
.tool-header {
  display: flex; align-items: center; gap: 8px; padding: 8px 12px;
  cursor: pointer; background: #f9fafb; user-select: none;
}
.tool-header:hover { background: #f3f4f6; }
.tool-icon   { font-size: 14px; width: 16px; }
.tool-name   { font-weight: bold; color: #374151; }
.tool-summary { flex: 1; color: #6b7280; font-family: monospace; font-size: 12px;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.expand-icon { color: #9ca3af; font-size: 10px; }
.tool-body   { padding: 0 12px 10px; background: #fff; }
.tool-section { margin-top: 8px; }
.section-label { font-size: 11px; font-weight: bold; color: #9ca3af; text-transform: uppercase; margin-bottom: 4px; }
.code-block {
  background: #f3f4f6; border-radius: 6px; padding: 8px; font-size: 12px;
  white-space: pre-wrap; word-break: break-all; overflow-x: auto; margin: 0; max-height: 200px;
}
.status-composing .tool-icon { color: #6b7280; }
.status-pending .tool-icon { color: #f59e0b; animation: spin 1s linear infinite; }
.status-done    .tool-icon { color: #10b981; }
.status-error   .tool-icon { color: #ef4444; }
.status-denied  .tool-icon { color: #6b7280; }
.status-cancelled .tool-icon { color: #475569; }
@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
</style>
