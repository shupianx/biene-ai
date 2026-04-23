<template>
  <div
    class="fp-root"
    :style="rootStyle"
    role="dialog"
    aria-modal="false"
  >
    <header
      class="fp-chrome"
      @pointerdown="onHeaderPointerDown"
    >
      <div class="fp-brand">
        <span class="fp-brand-text">{{ title }}</span>
      </div>
      <div class="fp-chrome-slot">
        <slot name="chrome" />
      </div>
      <button
        class="fp-close"
        type="button"
        :aria-label="closeLabel"
        :title="closeLabel"
        @click="emit('close')"
        @pointerdown.stop
      >
        <svg viewBox="0 0 24 24" aria-hidden="true" v-html="closeIconBody" />
      </button>
    </header>
    <div class="fp-body">
      <slot />
    </div>
    <div
      class="fp-resize"
      @pointerdown="onResizePointerDown"
      aria-hidden="true"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

const props = withDefaults(defineProps<{
  title: string
  closeLabel?: string
  initialWidth?: number
  initialHeight?: number
  minWidth?: number
  minHeight?: number
  storageKey?: string
}>(), {
  closeLabel: 'Close',
  initialWidth: 640,
  initialHeight: 560,
  minWidth: 360,
  minHeight: 280,
  storageKey: '',
})

const emit = defineEmits<{ (e: 'close'): void }>()

const closeIconBody =
  '<path fill="currentColor" d="m12 13.4l-4.9 4.9q-.275.275-.7.275t-.7-.275t-.275-.7t.275-.7l4.9-4.9l-4.9-4.9q-.275-.275-.275-.7t.275-.7t.7-.275t.7.275l4.9 4.9l4.9-4.9q.275-.275.7-.275t.7.275t.275.7t-.275.7L13.4 12l4.9 4.9q.275.275.275.7t-.275.7t-.7.275t-.7-.275z"/>'

type PanelState = { x: number; y: number; w: number; h: number }

function readPersisted(): PanelState | null {
  if (!props.storageKey) return null
  try {
    const raw = localStorage.getItem(props.storageKey)
    if (!raw) return null
    const parsed = JSON.parse(raw)
    if (
      typeof parsed?.x === 'number' &&
      typeof parsed?.y === 'number' &&
      typeof parsed?.w === 'number' &&
      typeof parsed?.h === 'number'
    ) {
      return parsed as PanelState
    }
  } catch {
    // ignore
  }
  return null
}

function writePersisted(state: PanelState) {
  if (!props.storageKey) return
  try {
    localStorage.setItem(props.storageKey, JSON.stringify(state))
  } catch {
    // ignore
  }
}

const x = ref(0)
const y = ref(0)
const w = ref(props.initialWidth)
const h = ref(props.initialHeight)

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max)
}

function clampToViewport() {
  const vw = window.innerWidth
  const vh = window.innerHeight
  w.value = clamp(w.value, props.minWidth, Math.max(props.minWidth, vw - 16))
  h.value = clamp(h.value, props.minHeight, Math.max(props.minHeight, vh - 16))
  x.value = clamp(x.value, 8, Math.max(8, vw - w.value - 8))
  y.value = clamp(y.value, 8, Math.max(8, vh - h.value - 8))
}

function centerPanel() {
  x.value = Math.round((window.innerWidth - w.value) / 2)
  y.value = Math.round((window.innerHeight - h.value) / 2)
}

const rootStyle = computed(() => ({
  transform: `translate(${x.value}px, ${y.value}px)`,
  width: `${w.value}px`,
  height: `${h.value}px`,
}))

let dragOffsetX = 0
let dragOffsetY = 0
let resizeStartX = 0
let resizeStartY = 0
let resizeStartW = 0
let resizeStartH = 0
let activeDragPointerId: number | null = null
let activeResizePointerId: number | null = null

function onHeaderPointerDown(event: PointerEvent) {
  if (event.button !== 0) return
  const target = event.target
  if (target instanceof Element && target.closest('button, input, a, [data-no-drag]')) return
  activeDragPointerId = event.pointerId
  dragOffsetX = event.clientX - x.value
  dragOffsetY = event.clientY - y.value
  event.preventDefault()
  window.addEventListener('pointermove', onDragMove)
  window.addEventListener('pointerup', onDragEnd)
  window.addEventListener('pointercancel', onDragEnd)
}

function onDragMove(event: PointerEvent) {
  if (event.pointerId !== activeDragPointerId) return
  x.value = event.clientX - dragOffsetX
  y.value = event.clientY - dragOffsetY
  clampToViewport()
}

function onDragEnd(event: PointerEvent) {
  if (event.pointerId !== activeDragPointerId) return
  activeDragPointerId = null
  window.removeEventListener('pointermove', onDragMove)
  window.removeEventListener('pointerup', onDragEnd)
  window.removeEventListener('pointercancel', onDragEnd)
  writePersisted({ x: x.value, y: y.value, w: w.value, h: h.value })
}

function onResizePointerDown(event: PointerEvent) {
  if (event.button !== 0) return
  activeResizePointerId = event.pointerId
  resizeStartX = event.clientX
  resizeStartY = event.clientY
  resizeStartW = w.value
  resizeStartH = h.value
  event.preventDefault()
  window.addEventListener('pointermove', onResizeMove)
  window.addEventListener('pointerup', onResizeEnd)
  window.addEventListener('pointercancel', onResizeEnd)
}

function onResizeMove(event: PointerEvent) {
  if (event.pointerId !== activeResizePointerId) return
  w.value = resizeStartW + (event.clientX - resizeStartX)
  h.value = resizeStartH + (event.clientY - resizeStartY)
  clampToViewport()
}

function onResizeEnd(event: PointerEvent) {
  if (event.pointerId !== activeResizePointerId) return
  activeResizePointerId = null
  window.removeEventListener('pointermove', onResizeMove)
  window.removeEventListener('pointerup', onResizeEnd)
  window.removeEventListener('pointercancel', onResizeEnd)
  writePersisted({ x: x.value, y: y.value, w: w.value, h: h.value })
}

function onViewportResize() {
  clampToViewport()
}

onMounted(() => {
  const persisted = readPersisted()
  if (persisted) {
    x.value = persisted.x
    y.value = persisted.y
    w.value = persisted.w
    h.value = persisted.h
    clampToViewport()
  } else {
    centerPanel()
  }
  window.addEventListener('resize', onViewportResize)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', onViewportResize)
  window.removeEventListener('pointermove', onDragMove)
  window.removeEventListener('pointerup', onDragEnd)
  window.removeEventListener('pointercancel', onDragEnd)
  window.removeEventListener('pointermove', onResizeMove)
  window.removeEventListener('pointerup', onResizeEnd)
  window.removeEventListener('pointercancel', onResizeEnd)
})
</script>

<style scoped>
.fp-root {
  position: fixed;
  top: 0;
  left: 0;
  z-index: 40;
  display: flex;
  flex-direction: column;
  background: var(--panel);
  border: 1px solid var(--rule);
  box-shadow: 6px 6px 0 0 var(--rule);
  overflow: hidden;
}

.fp-chrome {
  height: 36px;
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 8px 0 14px;
  background: var(--panel);
  border-bottom: 1px solid var(--rule-soft);
  cursor: grab;
  user-select: none;
}

.fp-chrome:active {
  cursor: grabbing;
}

.fp-brand {
  display: inline-flex;
  align-items: center;
}

.fp-brand-text {
  font-family: var(--mono);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.14em;
  color: var(--ink);
}

.fp-chrome-slot {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
}

.fp-close {
  width: 26px;
  height: 26px;
  display: grid;
  place-items: center;
  background: transparent;
  border: 1px solid transparent;
  color: var(--ink-3);
  cursor: pointer;
  transition: background .12s, color .12s, border-color .12s;
}

.fp-close:hover {
  background: var(--bg-2);
  border-color: var(--rule-softer);
  color: var(--ink);
}

.fp-close svg {
  width: 14px;
  height: 14px;
}

.fp-body {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.fp-resize {
  position: absolute;
  right: 0;
  bottom: 0;
  width: 14px;
  height: 14px;
  cursor: nwse-resize;
  background:
    linear-gradient(135deg, transparent 50%, var(--rule-soft) 50%, var(--rule-soft) 60%, transparent 60%, transparent 70%, var(--rule-soft) 70%, var(--rule-soft) 80%, transparent 80%);
}
</style>
