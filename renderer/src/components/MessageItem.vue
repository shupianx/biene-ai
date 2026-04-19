<template>
  <div class="message" :class="msg.role">
    <div class="bubble">
      <div v-if="msg.role === 'assistant'" class="assistant-head">
        <span class="role-tag">AGENT</span>
        <span v-if="usedSkillLabel" class="skill-note">⚡ {{ usedSkillLabel }}</span>
      </div>
      <div v-if="reasoningText" class="reasoning-block" :class="{ complete: reasoningComplete }">
        <button
          class="reasoning-head"
          :class="{ clickable: reasoningComplete }"
          type="button"
          :disabled="!reasoningComplete"
          :aria-expanded="reasoningComplete ? !reasoningCollapsed : undefined"
          @click="toggleReasoning"
        >
          <span class="reasoning-title">{{ reasoningLabel }}</span>
          <MaterialSymbolsArrowForwardIosRounded
            v-if="reasoningComplete"
            class="reasoning-chevron"
            :class="{ expanded: !reasoningCollapsed }"
            aria-hidden="true"
          />
        </button>
        <div
          class="reasoning-content"
          :class="{
            complete: reasoningComplete,
            collapsed: reasoningCollapsed && reasoningComplete,
            streaming: !reasoningComplete,
          }"
          :style="reasoningContentStyle"
        >
          <div ref="reasoningBodyRef" class="reasoning-body" dir="auto">
            {{ reasoningText }}
          </div>
        </div>
      </div>
      <div v-if="assistantHasText" class="markdown" v-html="renderedText" />
      <div
        v-else-if="msg.role !== 'assistant'"
        class="user-text"
        :class="{
          'agent-source-text': msg.author_type === 'agent',
          'system-note-text': msg.author_type === 'system',
        }"
        dir="auto"
      >
        <div class="user-head">
          <span class="role-tag">{{ userRoleTag }}</span>
        </div>
        <div class="user-body">{{ msg.text }}</div>
      </div>
      <ToolCallCard
        v-for="(tc, i) in (msg.tool_calls ?? [])"
        :key="i"
        :tc="tc"
      />
      <div v-if="metaLinesVisible" class="message-meta">
        <div v-if="formattedMessageTime" class="message-time">{{ formattedMessageTime }}</div>
        <div v-if="sourceAgentLabel" class="message-source">
          <span>{{ t('message.from') }} </span>
          <button
            v-if="canOpenSourceAgent"
            class="message-source-link"
            type="button"
            @click="openSourceAgent"
          >
            {{ sourceAgentLabel }}
          </button>
          <span v-else>{{ sourceAgentLabel }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import MaterialSymbolsArrowForwardIosRounded from '~icons/material-symbols/arrow-forward-ios-rounded'
import { useAgentNavigation } from '../composables/useAgentNavigation'
import type { DisplayMessage } from '../api/http'
import { t } from '../i18n'
import { useSessionsStore } from '../stores/sessions'
import ToolCallCard from './ToolCallCard.vue'
import { renderMarkdown } from '../utils/markdown'
import { formatMessageTime } from '../utils/messageTime'

const props = defineProps<{ msg: DisplayMessage }>()
const { openAgent } = useAgentNavigation()
const store = useSessionsStore()

const renderedText = computed(() =>
  renderMarkdown(props.msg.text)
)

const assistantHasText = computed(() =>
  props.msg.role === 'assistant' && Boolean(props.msg.text.trim())
)

const usedSkillLabel = computed(() => {
  if (props.msg.role !== 'assistant' || !props.msg.used_skill_name) return ''
  return t('message.usedSkill', { name: props.msg.used_skill_name })
})
const reasoningCollapsed = ref(false)
const reasoningBodyRef = ref<HTMLElement | null>(null)
const reasoningExpandedHeight = ref(0)

const reasoningText = computed(() =>
  props.msg.role === 'assistant' ? props.msg.reasoning?.text ?? '' : ''
)

const reasoningComplete = computed(() =>
  props.msg.role === 'assistant' && Boolean(props.msg.reasoning?.duration_ms)
)

const reasoningDuration = computed(() => {
  const durationMs = props.msg.reasoning?.duration_ms ?? 0
  if (!durationMs) return ''
  const seconds = durationMs / 1000
  const value = seconds < 10 ? seconds.toFixed(1) : Math.round(seconds).toString()
  return t('agent.durationSeconds', { n: value })
})

const reasoningLabel = computed(() => {
  if (!reasoningText.value) return ''
  if (!reasoningComplete.value) return t('agent.thinkingLive')
  return t('agent.thoughtFor', { duration: reasoningDuration.value })
})

const reasoningContentStyle = computed(() => {
  if (!reasoningText.value) return {}
  if (!reasoningComplete.value) {
    return { maxHeight: '100px' }
  }
  return { maxHeight: reasoningCollapsed.value ? '0px' : `${reasoningExpandedHeight.value}px` }
})

const userRoleTag = computed(() => {
  if (props.msg.author_type === 'agent') return 'AGENT'
  if (props.msg.author_type === 'system') return 'SYSTEM'
  return 'USER'
})

const sourceAgentLabel = computed(() => {
  if (props.msg.role !== 'user' || props.msg.author_type !== 'agent') return ''
  return (
    (props.msg.author_id ? store.sessions[props.msg.author_id]?.meta.name : '') ||
    props.msg.author_name ||
    props.msg.author_id ||
    t('agent.anotherAgent')
  )
})

const formattedMessageTime = computed(() => {
  if (props.msg.role !== 'user') return ''
  return formatMessageTime(props.msg.created_at)
})

const metaLinesVisible = computed(() =>
  props.msg.role === 'user' && Boolean(sourceAgentLabel.value || formattedMessageTime.value)
)

const canOpenSourceAgent = computed(() =>
  Boolean(props.msg.author_id)
)

watch(
  () => reasoningComplete.value,
  (complete, previous) => {
    if (complete && !previous) {
      reasoningCollapsed.value = true
      return
    }
    if (!complete) {
      reasoningCollapsed.value = false
    }
  },
  { immediate: true },
)

watch(
  [reasoningText, reasoningCollapsed, reasoningComplete],
  () => {
    void nextTick(() => {
      reasoningExpandedHeight.value = reasoningBodyRef.value?.scrollHeight ?? 0
    })
  },
  { immediate: true },
)

async function openSourceAgent() {
  if (!props.msg.author_id) return
  await openAgent(props.msg.author_id)
}

function toggleReasoning() {
  if (!reasoningComplete.value) return
  reasoningCollapsed.value = !reasoningCollapsed.value
}
</script>

<style scoped>
.message {
  display: flex;
  padding: 10px 0;
}

.bubble {
  max-width: 78%;
  min-width: 0;
}

.message.assistant .bubble {
  width: 100%;
  max-width: none;
}

.message.user .bubble {
  margin-left: auto;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
}

.assistant-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}

.user-head {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 4px;
}

.role-tag {
  font-family: var(--mono);
  font-size: 9.5px;
  font-weight: 600;
  letter-spacing: 0.18em;
  color: var(--ink-4);
  padding: 1px 6px;
  border: 1px solid var(--rule-softer);
}

.skill-note {
  font-family: var(--mono);
  font-size: 10px;
  letter-spacing: 0.1em;
  color: var(--accent);
}

.reasoning-block {
  margin: 0 0 10px;
}

.reasoning-head {
  width: auto;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 4px 0 6px;
  border: none;
  background: transparent;
  color: var(--ink-3);
  text-align: left;
  cursor: default;
}

.reasoning-head.clickable {
  cursor: pointer;
}

.reasoning-title {
  font-family: var(--mono);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.reasoning-chevron {
  flex: 0 0 auto;
  color: var(--ink-4);
  font-size: 11px;
  transition: transform .22s cubic-bezier(.2,.7,.2,1), color .12s ease;
}

.reasoning-head.clickable:hover .reasoning-chevron,
.reasoning-head.clickable:hover .reasoning-title {
  color: var(--ink-2);
}

.reasoning-content {
  overflow: hidden;
  opacity: 1;
  transition: max-height .26s cubic-bezier(.2,.7,.2,1), opacity .18s ease;
}

.reasoning-content.streaming {
  overflow-y: auto;
  overscroll-behavior: contain;
}

.reasoning-content.collapsed {
  opacity: 0;
}

.reasoning-body {
  padding: 0 0 6px;
  font-size: 13px;
  line-height: 1.55;
  color: var(--ink-3);
  white-space: pre-wrap;
  word-break: break-word;
}

.reasoning-chevron.expanded {
  transform: rotate(90deg);
}

@media (prefers-reduced-motion: reduce) {
  .reasoning-chevron,
  .reasoning-content {
    transition: none;
  }
}

.user-text {
  display: inline-block;
  background: var(--panel-2);
  border: 1px solid var(--rule-soft);
  color: var(--ink);
  padding: 8px 12px;
  max-width: 100%;
}

.user-body {
  text-align: start;
  font-size: 14px;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-word;
}

.user-text.agent-source-text {
  background: var(--panel-2);
  border-color: var(--accent);
}

.user-text.agent-source-text .role-tag {
  color: var(--accent);
  border-color: var(--accent);
}

.user-text.system-note-text {
  background: var(--bg-2);
  border-color: var(--rule-soft);
  color: var(--ink-3);
}

.message-meta {
  margin-top: 6px;
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  user-select: none;
  -webkit-user-select: none;
}

.message-source,
.message-time {
  font-family: var(--mono);
  font-size: 10px;
  letter-spacing: 0.08em;
  line-height: 1.3;
  color: var(--ink-4);
  text-align: right;
}

.message-source-link {
  border: none;
  padding: 0;
  margin: 0;
  background: transparent;
  color: var(--accent);
  font: inherit;
  cursor: pointer;
  text-decoration: underline;
  text-underline-offset: 2px;
}

.message-source-link:hover {
  color: var(--ink);
}

.markdown {
  font-size: 14px;
  line-height: 1.65;
  color: var(--ink);
  overflow-wrap: anywhere;
}

.markdown :deep(h1),
.markdown :deep(h2),
.markdown :deep(h3),
.markdown :deep(h4),
.markdown :deep(h5),
.markdown :deep(h6) {
  margin: 0 0 10px;
  line-height: 1.25;
  color: var(--ink);
  font-family: var(--sans);
  font-weight: 600;
  letter-spacing: -0.01em;
}

.markdown :deep(h1) { font-size: 1.4rem; }
.markdown :deep(h2) { font-size: 1.2rem; }
.markdown :deep(h3) { font-size: 1.05rem; }
.markdown :deep(h4),
.markdown :deep(h5),
.markdown :deep(h6) { font-size: 0.98rem; }

.markdown :deep(p) { margin: 0 0 10px; }
.markdown :deep(p:last-child) { margin-bottom: 0; }

.markdown :deep(blockquote) {
  margin: 0 0 12px;
  padding: 8px 14px;
  border-left: 3px solid var(--rule);
  background: var(--panel);
  color: var(--ink-3);
}

.markdown :deep(hr) {
  border: 0;
  border-top: 1px dashed var(--rule-soft);
  margin: 16px 0;
}

.markdown :deep(pre) {
  margin: 0 0 12px;
  background: var(--panel);
  border: 1px solid var(--rule-soft);
  padding: 10px 12px;
  overflow-x: auto;
  font-size: 12.5px;
  font-family: var(--mono);
}

.markdown :deep(code) {
  background: var(--bg-2);
  border: 1px solid var(--rule-softer);
  padding: 0 4px;
  font-size: 12.5px;
  font-family: var(--mono);
}

.markdown :deep(pre code) {
  background: none;
  border: none;
  padding: 0;
  white-space: pre;
  word-break: normal;
}

.markdown :deep(pre code.hljs) { background: transparent; }

.markdown :deep(ul),
.markdown :deep(ol) {
  margin: 0 0 12px;
  padding-left: 20px;
  list-style-position: outside;
}

.markdown :deep(ul) { list-style-type: disc; }
.markdown :deep(ol) { list-style-type: decimal; }
.markdown :deep(ul ul) { list-style-type: circle; }
.markdown :deep(ul ul ul) { list-style-type: square; }
.markdown :deep(ol ol) { list-style-type: lower-alpha; }
.markdown :deep(ol ol ol) { list-style-type: lower-roman; }

.markdown :deep(li) { margin: 4px 0; }
.markdown :deep(li::marker) { color: var(--ink-4); }

.markdown :deep(.task-list) {
  padding-left: 0;
  list-style: none;
}

.markdown :deep(.task-list-item) {
  list-style: none;
  display: flex;
  align-items: flex-start;
  gap: 8px;
}

.markdown :deep(input[type="checkbox"]) {
  margin-top: 0.25rem;
  accent-color: var(--accent);
}

.markdown :deep(a) {
  color: var(--accent);
  text-decoration: underline;
  text-underline-offset: 2px;
}

.markdown :deep(a:hover) { color: var(--ink); }

.markdown :deep(table) {
  display: block;
  width: max-content;
  max-width: 100%;
  margin: 0 0 12px;
  overflow-x: auto;
  border-collapse: collapse;
  border: 1px solid var(--rule-soft);
}

.markdown :deep(th),
.markdown :deep(td) {
  padding: 6px 10px;
  border: 1px solid var(--rule-softer);
  vertical-align: top;
}

.markdown :deep(th) {
  background: var(--panel);
  font-family: var(--mono);
  font-size: 11px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  font-weight: 600;
}

.markdown :deep(img) {
  display: block;
  max-width: 100%;
  height: auto;
  margin: 0 0 12px;
  border: 1px solid var(--rule-softer);
}
</style>
