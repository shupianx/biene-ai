<template>
  <div class="input-bar">
    <div class="composer" :class="{ disabled }">
      <textarea
        ref="taRef"
        v-model="text"
        :disabled="disabled"
        placeholder="Message this agent…"
        rows="1"
        @keydown.enter.exact.prevent="onEnter"
        @input="resize"
        @compositionstart="onCompositionStart"
        @compositionend="onCompositionEnd"
      />
      <div class="composer-actions">
        <button
          class="action-btn"
          :class="{ interrupt: interruptible }"
          :disabled="buttonDisabled"
          :title="buttonTitle"
          @click="handleAction"
        >
          <svg
            v-if="!interruptible"
            class="send-icon"
            viewBox="0 0 24 24"
            aria-hidden="true"
            v-html="sendIconBody"
          />
          <span v-else-if="interrupting" class="interrupt-spinner" aria-hidden="true" />
          <svg
            v-else
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="currentColor"
            aria-hidden="true"
          >
            <rect x="6" y="6" width="12" height="12" rx="2" />
          </svg>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, nextTick } from 'vue'
import { icons as mynauiIcons } from '@iconify-json/mynaui'

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
const sendIconBody = mynauiIcons.icons.send.body
let compositionLockedUntil = 0

const buttonDisabled = computed(() => {
  if (props.interruptible) {
    return Boolean(props.interrupting)
  }
  return Boolean(props.disabled || !text.value.trim())
})

const buttonTitle = computed(() =>
  props.interruptible
    ? (props.interrupting ? 'Interrupting' : 'Interrupt')
    : 'Send message'
)

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
  // Some IMEs emit an immediate Enter right after composition ends.
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
  const t = text.value.trim()
  if (!t || props.disabled) return
  text.value = ''
  await nextTick()
  if (taRef.value) taRef.value.style.height = 'auto'
  emit('send', t)
}
</script>

<style scoped>
.input-bar {
  padding: 0 15px 15px;
  background: var(--app-bg);
}

.composer {
  display: flex;
  flex-direction: column;
  gap: 14px;
  width: 100%;
  min-height: 102px;
  padding: 18px 18px 14px 20px;
  border: 1px solid #ddd6cf;
  border-radius: 20px;
  background: #fff;
  box-shadow: 0 1px 2px rgba(15, 23, 42, .03);
  transition: border-color .2s, box-shadow .2s, background .2s;
}

.composer:focus-within {
  border-color: #d6d3d1;
  box-shadow: 0 4px 12px rgba(15, 23, 42, .06);
}

.composer.disabled {
  background: #fffcf8;
}

textarea {
  width: 100%;
  min-height: 42px;
  resize: none;
  border: none;
  padding: 0;
  font-size: 15px;
  font-family: inherit;
  line-height: 1.55;
  outline: none;
  transition: background .2s;
  background: transparent;
  color: #0f172a;
  max-height: 160px; overflow-y: auto;
}
textarea::placeholder {
  color: #78716c;
}
textarea:focus {
  background: transparent;
}
textarea:disabled { color: #9ca3af; }

.composer-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: auto;
}

.action-btn {
  flex-shrink: 0; width: 40px; height: 40px; border-radius: 10px;
  border: 1px solid #ea580c;
  background: #ea580c;
  color: #fff;
  display: flex; align-items: center; justify-content: center;
  cursor: pointer; transition: background .2s, border-color .2s, color .2s, opacity .2s;
}
.action-btn:hover:not(:disabled) {
  background: #c2410c;
  border-color: #c2410c;
}
.action-btn:active:not(:disabled) {
  background: #9a3412;
  border-color: #9a3412;
}
.action-btn:disabled { opacity: .4; cursor: not-allowed; }
.action-btn:focus-visible {
  outline: 2px solid var(--accent-warm-ring);
  outline-offset: 2px;
}
.send-icon {
  width: 18px;
  height: 18px;
  overflow: visible;
  flex-shrink: 0;
}
.action-btn.interrupt {
  border-color: #e11d48;
  background: #e11d48;
  color: #fff;
}
.action-btn.interrupt:hover:not(:disabled) {
  border-color: #be123c;
  background: #be123c;
}

.interrupt-spinner {
  width: 16px;
  height: 16px;
  border-radius: 999px;
  border: 2px solid rgba(255, 255, 255, .35);
  border-top-color: #fff;
  animation: spin .8s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
