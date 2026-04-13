<template>
  <div class="message" :class="msg.role">
    <div class="bubble">
      <div v-if="msg.role === 'assistant'" class="markdown" v-html="renderedText" />
      <div
        v-else
        class="user-text"
        :class="{ 'agent-source-text': msg.author_type === 'agent' }"
        dir="auto"
      >
        {{ msg.text }}
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
import { computed } from 'vue'
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

async function openSourceAgent() {
  if (!props.msg.author_id) return
  await openAgent(props.msg.author_id)
}
</script>

<style scoped>
.message { display: flex; gap: 0; padding: 12px 0; }

.bubble { max-width: 72%; min-width: 0; }
.message.assistant .bubble { width: 100%; max-width: none; }
.message.user .bubble {
  margin-left: auto;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
}

.user-text {
  display: inline-block; background: #eceef1; color: #111827;
  padding: 10px 14px; border-radius: 16px;
  text-align: start;
  font-size: 14px; line-height: 1.5; white-space: pre-wrap; word-break: break-word;
}

.user-text.agent-source-text {
  background: var(--accent-warm-bg);
  border: 1px solid var(--accent-warm-border);
}

.message-meta {
  margin-top: 6px;
  margin-right: 8px;
  display: flex;
  flex-direction: row;
  align-items: flex-end;
  justify-content: flex-end;
  gap: 8px;
  user-select: none;
  -webkit-user-select: none;
}

.message-source,
.message-time {
  font-size: 11px;
  line-height: 1.3;
  color: #9ca3af;
  text-align: right;
}

.message-source-link {
  border: none;
  padding: 0;
  margin: 0;
  background: transparent;
  color: #2563eb;
  font: inherit;
  font-weight: bold;
  cursor: pointer;
  text-decoration: none;
}

.message-source-link:hover {
  color: #1d4ed8;
}

.markdown { font-size: 14px; line-height: 1.6; color: #111827; overflow-wrap: anywhere; }
.markdown :deep(h1),
.markdown :deep(h2),
.markdown :deep(h3),
.markdown :deep(h4),
.markdown :deep(h5),
.markdown :deep(h6) {
  margin: 0 0 12px;
  line-height: 1.25;
  color: #0f172a;
}
.markdown :deep(h1) { font-size: 1.5rem; }
.markdown :deep(h2) { font-size: 1.3rem; }
.markdown :deep(h3) { font-size: 1.15rem; }
.markdown :deep(h4),
.markdown :deep(h5),
.markdown :deep(h6) { font-size: 1rem; }
.markdown :deep(p)            { margin: 0 0 10px; }
.markdown :deep(p:last-child) { margin-bottom: 0; }
.markdown :deep(blockquote) {
  margin: 0 0 12px;
  padding: 10px 14px;
  border-left: 3px solid #cbd5e1;
  background: #f8fafc;
  color: #475569;
  border-radius: 0 12px 12px 0;
}
.markdown :deep(hr) {
  border: 0;
  border-top: 1px solid #e5e7eb;
  margin: 16px 0;
}
.markdown :deep(pre) {
  margin: 0 0 12px;
  background: #f3f4f6; border-radius: 8px; padding: 12px;
  overflow-x: auto; font-size: 13px;
}
.markdown :deep(code) {
  background: #f3f4f6; border-radius: 4px; padding: 1px 5px; font-size: 13px;
}
.markdown :deep(pre code) { background: none; padding: 0; white-space: pre; word-break: normal; }
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
.markdown :deep(li::marker) { color: #475569; }
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
  accent-color: #6366f1;
}
.markdown :deep(a) {
  color: #2563eb;
  text-decoration: underline;
  text-underline-offset: 2px;
}
.markdown :deep(a:hover) { color: #1d4ed8; }
.markdown :deep(table) {
  display: block;
  width: max-content;
  max-width: 100%;
  margin: 0 0 12px;
  overflow-x: auto;
  border-collapse: collapse;
}
.markdown :deep(th),
.markdown :deep(td) {
  padding: 8px 10px;
  border: 1px solid #e5e7eb;
  vertical-align: top;
}
.markdown :deep(th) {
  background: #f8fafc;
  font-weight: bold;
}
.markdown :deep(img) {
  display: block;
  max-width: 100%;
  height: auto;
  margin: 0 0 12px;
  border-radius: 10px;
}
</style>
