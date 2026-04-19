<template>
  <div class="input-bar">
    <div class="composer" :class="{ disabled, focused: focused }">
      <textarea
        ref="taRef"
        v-model="text"
        :disabled="disabled"
        :placeholder="t('input.placeholder')"
        rows="1"
        @keydown.enter.exact.prevent="onEnter"
        @input="resize"
        @focus="focused = true"
        @blur="focused = false"
        @compositionstart="onCompositionStart"
        @compositionend="onCompositionEnd"
      />
      <div class="composer-actions">
        <button
          class="action-btn"
          :class="{ interrupt: interruptible }"
          :aria-label="buttonTitle"
          :disabled="buttonDisabled"
          :title="buttonTitle"
          @click="handleAction"
        >
          <MynauiSend
            v-if="!interruptible"
            class="send-icon"
            aria-hidden="true"
          />
          <span v-else-if="interrupting" class="interrupt-spinner" aria-hidden="true" />
          <svg
            v-else
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="currentColor"
            aria-hidden="true"
          >
            <rect x="6" y="6" width="12" height="12" />
          </svg>
          <span class="action-label">{{ actionLabel }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, nextTick } from 'vue'
import MynauiSend from '~icons/mynaui/send'
import { t } from '../i18n'

const props = defineProps<{
  disabled?: boolean
  interruptible?: boolean
  interrupting?: boolean
}>()
const emit  = defineEmits<{
  (e: 'send', text: string): void
  (e: 'interrupt'): void
}>()

const text  = ref('')
const taRef = ref<HTMLTextAreaElement | null>(null)
const isComposing = ref(false)
const focused = ref(false)
let compositionLockedUntil = 0

const buttonDisabled = computed(() => {
  if (props.interruptible) {
    return Boolean(props.interrupting)
  }
  return Boolean(props.disabled || !text.value.trim())
})

const buttonTitle = computed(() =>
  props.interruptible
    ? (props.interrupting ? t('input.interrupting') : t('input.interrupt'))
    : t('input.send')
)

const actionLabel = computed(() => {
  if (props.interruptible) {
    return props.interrupting ? t('input.interrupting') : t('input.interrupt')
  }
  return t('input.send')
})

function resize() {
  const el = taRef.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = Math.min(el.scrollHeight, 160) + 'px'
}

function onCompositionStart() {
  isComposing.value = true
}

function onCompositionEnd() {
  isComposing.value = false
  compositionLockedUntil = Date.now() + 30
}

function onEnter(event: KeyboardEvent) {
  if (isComposing.value || event.isComposing) return
  if (Date.now() < compositionLockedUntil) return
  if (props.interruptible) return
  submit()
}

function handleAction() {
  if (props.interruptible) {
    if (!props.interrupting) emit('interrupt')
    return
  }
  submit()
}

async function submit() {
  const value = text.value.trim()
  if (!value || props.disabled) return
  text.value = ''
  await nextTick()
  if (taRef.value) taRef.value.style.height = 'auto'
  emit('send', value)
}
</script>

<style scoped>
.input-bar {
  pointer-events: auto;
}

.composer {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
  padding: 10px 12px 10px;
  border: 1px solid var(--rule-soft);
  background: var(--panel-2);
  box-shadow: 0 10px 30px rgba(20, 18, 15, 0.10);
  transition: border-color .15s, box-shadow .15s;
}

.composer.focused {
  border-color: var(--rule);
  box-shadow: 2px 2px 0 0 var(--rule);
}

.composer.disabled {
  background: var(--bg-2);
}

textarea {
  width: 100%;
  min-height: 40px;
  resize: none;
  border: none;
  padding: 0;
  font-size: 14px;
  font-family: var(--sans);
  line-height: 1.55;
  outline: none;
  background: transparent;
  color: var(--ink);
  max-height: 160px;
  overflow-y: auto;
}

textarea::placeholder {
  color: var(--ink-4);
}

textarea:disabled {
  color: var(--ink-4);
}

.composer-actions {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  padding-top: 6px;
  border-top: 1px dashed var(--rule-softer);
}

.action-btn {
  height: 28px;
  padding: 0 12px;
  border: 1px solid var(--ink);
  background: var(--ink);
  color: var(--panel-2);
  display: inline-flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  font-family: var(--mono);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  transition: transform .12s, box-shadow .12s, opacity .15s;
}

.action-btn:hover:not(:disabled) {
  transform: translate(-1px, -1px);
  box-shadow: 2px 2px 0 0 var(--rule);
}

.action-btn:active:not(:disabled) {
  transform: translate(0, 0);
  box-shadow: none;
}

.action-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.action-btn:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.send-icon {
  width: 13px;
  height: 13px;
  flex-shrink: 0;
}

.action-btn.interrupt {
  border-color: color-mix(in srgb, var(--err) 42%, var(--rule-soft));
  background: color-mix(in srgb, var(--err) 14%, var(--panel-2));
  color: var(--err);
}

.interrupt-spinner {
  width: 12px;
  height: 12px;
  border: 2px solid color-mix(in srgb, var(--err) 20%, transparent);
  border-top-color: var(--err);
  animation: bieneSpin .8s linear infinite;
}

.action-label {
  line-height: 1;
}
</style>
