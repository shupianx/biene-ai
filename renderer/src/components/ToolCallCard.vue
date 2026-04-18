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
        <div class="section-label">{{ t('tool.input') }}</div>
        <pre class="code-block">{{ fmt(tc.tool_input) }}</pre>
      </div>
      <div v-if="tc.result" class="tool-section">
        <div class="section-label">{{ t('tool.output') }}</div>
        <pre class="code-block">{{ tc.result }}</pre>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { DisplayTool } from '../api/http'
import { t } from '../i18n'

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
.tool-card {
  border: 1px solid var(--rule-soft);
  background: var(--panel-2);
  margin: 6px 0;
  overflow: hidden;
  font-size: 12.5px;
}

.tool-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 10px;
  cursor: pointer;
  background: var(--panel);
  border-bottom: 1px dashed transparent;
  user-select: none;
  transition: background .12s;
}

.tool-header:hover {
  background: var(--bg-2);
}

.tool-icon {
  font-family: var(--mono);
  font-size: 12px;
  width: 14px;
  text-align: center;
  color: var(--ink-3);
}

.tool-name {
  font-family: var(--mono);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--ink-2);
}

.tool-summary {
  flex: 1;
  color: var(--ink-4);
  font-family: var(--mono);
  font-size: 11.5px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.expand-icon {
  color: var(--ink-4);
  font-size: 9px;
  font-family: var(--mono);
}

.tool-body {
  padding: 0 10px 8px;
  background: var(--panel-2);
  border-top: 1px dashed var(--rule-softer);
}

.tool-section { margin-top: 8px; }

.section-label {
  font-family: var(--mono);
  font-size: 9.5px;
  font-weight: 600;
  color: var(--ink-4);
  letter-spacing: 0.14em;
  text-transform: uppercase;
  margin-bottom: 4px;
}

.code-block {
  background: var(--panel);
  border: 1px solid var(--rule-softer);
  padding: 8px 10px;
  font-size: 11.5px;
  font-family: var(--mono);
  white-space: pre-wrap;
  word-break: break-all;
  overflow-x: auto;
  margin: 0;
  max-height: 220px;
  color: var(--ink-2);
}

.status-composing .tool-icon { color: var(--ink-4); }
.status-pending   .tool-icon {
  color: var(--warn);
  animation: bieneSpin 1s linear infinite;
}
.status-done      .tool-icon { color: var(--ok); }
.status-error     .tool-icon { color: var(--err); }
.status-denied    .tool-icon { color: var(--ink-4); }
.status-cancelled .tool-icon { color: var(--ink-3); }

.status-error .tool-header {
  border-left: 2px solid var(--err);
}

.status-pending .tool-header {
  border-left: 2px solid var(--warn);
}
</style>
