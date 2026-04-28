<template>
  <div class="compaction-marker" :class="{ expanded: !collapsed }">
    <button
      class="compaction-rule"
      type="button"
      :aria-expanded="!collapsed"
      @click="collapsed = !collapsed"
    >
      <span class="rule-line" aria-hidden="true" />
      <span class="rule-cluster">
        <span class="tag">{{ tag }}</span>
        <span class="meta">{{ metaLine }}</span>
        <MaterialSymbolsArrowForwardIosRounded
          class="chevron"
          :class="{ expanded: !collapsed }"
          aria-hidden="true"
        />
      </span>
      <span class="rule-line" aria-hidden="true" />
    </button>

    <div v-if="!collapsed" class="compaction-body">
      <div class="body-rail" aria-hidden="true" />
      <div class="body-content">
        <div class="body-eyebrow">
          <span class="eyebrow-tag">{{ t('compaction.summaryEyebrow') }}</span>
          <span class="eyebrow-time">{{ formattedTime }}</span>
        </div>
        <div class="body-markdown markdown" v-html="renderedSummary" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import MaterialSymbolsArrowForwardIosRounded from '~icons/material-symbols/arrow-forward-ios-rounded'
import { renderMarkdown } from '../../utils/markdown'
import { t } from '../../i18n'
import type { DisplayCompaction } from '../../api/http'

const props = defineProps<{
  compaction: DisplayCompaction
  createdAt: string
}>()

const collapsed = ref(true)

const tag = computed(() =>
  props.compaction.manual
    ? t('compaction.tagManual')
    : t('compaction.tagAuto'),
)

const metaLine = computed(() =>
  t('compaction.meta', {
    before: formatTokens(props.compaction.tokens_before),
    after: formatTokens(props.compaction.tokens_after),
    n: props.compaction.replaced,
  }),
)

const formattedTime = computed(() => formatTimestamp(props.createdAt))

const renderedSummary = computed(() => renderMarkdown(props.compaction.summary))

function formatTokens(n: number): string {
  if (n <= 0) return '0'
  if (n >= 10000) return `${(n / 1000).toFixed(1)}k`
  return n.toLocaleString()
}

function formatTimestamp(iso: string): string {
  if (!iso) return ''
  try {
    const d = new Date(iso)
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  } catch {
    return ''
  }
}
</script>

<style scoped>
.compaction-marker {
  margin: 14px 0;
  display: flex;
  flex-direction: column;
  gap: 0;
}

.compaction-rule {
  appearance: none;
  border: none;
  background: transparent;
  padding: 4px 0;
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  color: var(--ink-4);
  transition: color .12s ease;
}

.compaction-rule:hover {
  color: var(--ink-3);
}

.compaction-rule:focus-visible {
  outline: none;
}

.compaction-rule:focus-visible .rule-cluster {
  border-color: var(--ink-3);
}

.rule-line {
  flex: 1 1 auto;
  height: 0;
  border-top: 1px dashed var(--rule-soft);
}

.rule-cluster {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 3px 10px;
  background: var(--panel);
  border: 1px solid var(--rule-softer);
  border-radius: 2px;
  transition: border-color .12s ease, background .12s ease;
}

.compaction-rule:hover .rule-cluster {
  border-color: var(--rule-soft);
  background: var(--panel-2);
}

.tag {
  font-family: var(--mono);
  font-size: 9.5px;
  font-weight: 600;
  letter-spacing: 0.18em;
  color: var(--ink-3);
}

.compaction-marker.expanded .tag,
.compaction-rule:hover .tag {
  color: var(--ink-2);
}

.meta {
  font-family: var(--mono);
  font-size: 10px;
  letter-spacing: 0.04em;
  color: var(--ink-4);
  white-space: nowrap;
}

.chevron {
  width: 10px;
  height: 10px;
  color: var(--ink-4);
  transition: transform .22s cubic-bezier(.2,.7,.2,1), color .12s ease;
}

.chevron.expanded {
  transform: rotate(90deg);
  color: var(--ink-2);
}

.compaction-body {
  margin-top: 8px;
  display: flex;
  gap: 12px;
  animation: bieneFadeIn .18s ease both;
}

.body-rail {
  flex: 0 0 2px;
  align-self: stretch;
  background: var(--rule-soft);
}

.body-content {
  flex: 1 1 auto;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 2px 0 6px;
}

.body-eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: var(--ink-4);
}

.eyebrow-tag {
  font-family: var(--mono);
  font-size: 9px;
  font-weight: 600;
  letter-spacing: 0.18em;
}

.eyebrow-time {
  font-family: var(--mono);
  font-size: 10px;
  letter-spacing: 0.04em;
}

.body-markdown {
  color: var(--ink-2);
  font-size: 13px;
  line-height: 1.6;
}

:deep(.body-markdown h1),
:deep(.body-markdown h2),
:deep(.body-markdown h3) {
  font-size: 13px;
  letter-spacing: 0.02em;
  color: var(--ink);
  margin: 10px 0 4px;
}

:deep(.body-markdown h1:first-child),
:deep(.body-markdown h2:first-child),
:deep(.body-markdown h3:first-child) {
  margin-top: 0;
}

:deep(.body-markdown p) {
  margin: 4px 0;
}

:deep(.body-markdown ul),
:deep(.body-markdown ol) {
  margin: 4px 0;
  padding-left: 20px;
}

:deep(.body-markdown code) {
  font-family: var(--mono);
  font-size: 12px;
  background: var(--bg-2);
  border: 1px solid var(--rule-softer);
  padding: 0 4px;
  border-radius: 2px;
}

@media (prefers-reduced-motion: reduce) {
  .chevron,
  .compaction-body {
    animation: none;
    transition: none;
  }
}
</style>
