<template>
  <div class="input-bar">
    <div class="composer" :class="{ disabled, focused: focused }">
      <div v-if="stagedImages.length" class="attachment-tray">
        <div
          v-for="img in stagedImages"
          :key="img.id"
          class="attachment-chip"
        >
          <img :src="img.previewUrl" :alt="img.file.name" />
          <button
            class="attachment-remove"
            type="button"
            :aria-label="t('input.removeImage')"
            :title="t('input.removeImage')"
            @click="removeImage(img.id)"
          >
            <span aria-hidden="true">×</span>
          </button>
        </div>
      </div>
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
        @paste="onPaste"
        @compositionstart="onCompositionStart"
        @compositionend="onCompositionEnd"
      />
      <div class="composer-actions">
        <button
          class="attach-btn"
          type="button"
          :disabled="disabled"
          :aria-label="t('input.attachImage')"
          :title="t('input.attachImage')"
          @click="openFilePicker"
        >
          <MaterialSymbolsImageOutline class="attach-icon" aria-hidden="true" />
        </button>
        <input
          ref="fileInputRef"
          class="file-input"
          type="file"
          accept="image/*"
          multiple
          @change="onFileInputChange"
        />
        <div v-if="thinkingAvailable" class="thinking-control">
          <span class="thinking-label">{{ t('input.thinkingToggle') }}</span>
          <ToggleSwitch
            :model-value="thinkingEnabled"
            :label="t('input.thinkingToggle')"
            @update:model-value="emit('update:thinkingEnabled', $event)"
          />
        </div>
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
import { computed, ref, nextTick, onBeforeUnmount } from 'vue'
import MynauiSend from '~icons/mynaui/send'
import MaterialSymbolsImageOutline from '~icons/material-symbols/image-outline'
import ToggleSwitch from './ToggleSwitch.vue'
import { t } from '../i18n'

interface StagedImage {
  id: string
  file: File
  previewUrl: string
}

const props = defineProps<{
  disabled?: boolean
  interruptible?: boolean
  interrupting?: boolean
  thinkingAvailable?: boolean
  thinkingEnabled?: boolean
}>()
const emit = defineEmits<{
  (e: 'send', payload: { text: string; files: File[] }): void
  (e: 'update:thinkingEnabled', value: boolean): void
  (e: 'interrupt'): void
}>()

const text  = ref('')
const taRef = ref<HTMLTextAreaElement | null>(null)
const fileInputRef = ref<HTMLInputElement | null>(null)
const stagedImages = ref<StagedImage[]>([])
const isComposing = ref(false)
const focused = ref(false)
let compositionLockedUntil = 0

const buttonDisabled = computed(() => {
  if (props.interruptible) {
    return Boolean(props.interrupting)
  }
  if (props.disabled) return true
  return !text.value.trim() && stagedImages.value.length === 0
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

function openFilePicker() {
  fileInputRef.value?.click()
}

function onFileInputChange(event: Event) {
  const input = event.target as HTMLInputElement
  if (!input.files) return
  const files = Array.from(input.files)
  stageImages(files)
  input.value = ''
}

function onPaste(event: ClipboardEvent) {
  if (!event.clipboardData) return
  const images: File[] = []
  for (const item of Array.from(event.clipboardData.items)) {
    if (item.kind !== 'file') continue
    if (!item.type.startsWith('image/')) continue
    const file = item.getAsFile()
    if (file) images.push(file)
  }
  if (images.length === 0) return
  event.preventDefault()
  stageImages(images)
}

function stageImages(files: File[]) {
  for (const file of files) {
    if (!file.type.startsWith('image/')) continue
    stagedImages.value.push({
      id: crypto.randomUUID(),
      file,
      previewUrl: URL.createObjectURL(file),
    })
  }
}

function removeImage(id: string) {
  const idx = stagedImages.value.findIndex(img => img.id === id)
  if (idx < 0) return
  URL.revokeObjectURL(stagedImages.value[idx].previewUrl)
  stagedImages.value.splice(idx, 1)
}

function clearStagedImages() {
  for (const img of stagedImages.value) {
    URL.revokeObjectURL(img.previewUrl)
  }
  stagedImages.value = []
}

async function submit() {
  const value = text.value.trim()
  if (props.disabled) return
  if (!value && stagedImages.value.length === 0) return
  const files = stagedImages.value.map(img => img.file)
  text.value = ''
  clearStagedImages()
  await nextTick()
  if (taRef.value) taRef.value.style.height = 'auto'
  emit('send', { text: value, files })
}

onBeforeUnmount(() => {
  for (const img of stagedImages.value) {
    URL.revokeObjectURL(img.previewUrl)
  }
})
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

.attachment-tray {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 2px 0 6px;
  border-bottom: 1px dashed var(--rule-softer);
}

.attachment-chip {
  position: relative;
  width: 56px;
  height: 56px;
  border: 1px solid var(--rule-softer);
  background: var(--bg-2);
  overflow: hidden;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.attachment-chip img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.attachment-remove {
  position: absolute;
  top: 2px;
  right: 2px;
  width: 16px;
  height: 16px;
  border: none;
  background: rgba(20, 18, 15, 0.7);
  color: #fff;
  font-size: 13px;
  line-height: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  padding: 0;
}

.attachment-remove:hover {
  background: rgba(20, 18, 15, 0.88);
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
  flex-wrap: wrap;
  gap: 12px;
  padding-top: 6px;
  border-top: 1px dashed var(--rule-softer);
}

.attach-btn {
  margin-right: auto;
  height: 26px;
  width: 26px;
  border: none;
  background: transparent;
  color: var(--ink-3);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  padding: 0;
  transition: color .12s, background-color .12s;
}

.attach-btn:hover:not(:disabled) {
  color: var(--ink);
  background-color: var(--bg-2);
}

.attach-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.attach-icon {
  width: 15px;
  height: 15px;
}

.file-input {
  display: none;
}

.thinking-control {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  --toggle-track-on: color-mix(in srgb, #84befe 72%, var(--panel-2));
  --toggle-track-on-border: color-mix(in srgb, #67a8f4 68%, var(--rule-soft));
  --toggle-knob-on: #f7fbff;
}

.thinking-label {
  font-family: var(--sans);
  font-size: 12px;
  font-weight: 500;
  letter-spacing: 0.02em;
  color: color-mix(in srgb, var(--ink-4) 78%, var(--panel-2));
  white-space: nowrap;
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
