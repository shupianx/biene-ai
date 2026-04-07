<template>
  <div class="input-bar">
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
    <button
      class="action-btn"
      :class="{ interrupt: interruptible }"
      :disabled="buttonDisabled"
      :title="buttonTitle"
      @click="handleAction"
    >
      <svg
        v-if="!interruptible"
        width="18"
        height="18"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2.5"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <line x1="22" y1="2" x2="11" y2="13" /><polygon points="22 2 15 22 11 13 2 9 22 2" />
      </svg>
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
</template>

<script setup lang="ts">
import { computed, ref, nextTick } from 'vue'

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
  display: flex; align-items: flex-end; gap: 10px;
  padding: 14px 16px;
  background: #fff; border-top: 1px solid #e5e7eb;
}
textarea {
  flex: 1; resize: none;
  border: 1.5px solid #e5e7eb; border-radius: 12px;
  padding: 10px 14px; font-size: 14px; font-family: inherit;
  line-height: 1.5; outline: none; transition: border-color .2s;
  max-height: 160px; overflow-y: auto;
}
textarea:focus   { border-color: #6366f1; }
textarea:disabled { background: #f9fafb; color: #9ca3af; }
.action-btn {
  flex-shrink: 0; width: 40px; height: 40px; border-radius: 10px;
  border: none; background: #6366f1; color: #fff;
  display: flex; align-items: center; justify-content: center;
  cursor: pointer; transition: background .2s, opacity .2s;
}
.action-btn:hover:not(:disabled) { background: #4f46e5; }
.action-btn:disabled { opacity: .4; cursor: not-allowed; }
.action-btn.interrupt { background: #e11d48; }
.action-btn.interrupt:hover:not(:disabled) { background: #be123c; }

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
