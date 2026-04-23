<template>
  <div class="term-panel" :style="{ '--panel-height': `${height}px` }">
    <div ref="containerRef" class="term-host" @click="focusTerminal" />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Terminal, type ITheme } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import type { ProcessStateData } from '../../types/events'
import { connectProcessLogsWS, type ProcessTerminalHandle } from '../../api/ws'
import { useTheme } from '../../composables/useTheme'

const props = defineProps<{
  sessionId: string
  state: ProcessStateData | null
  height?: number
}>()

const height = computed(() => props.height ?? 220)

const containerRef = ref<HTMLElement | null>(null)
let terminal: Terminal | null = null
let fitAddon: FitAddon | null = null
let handle: ProcessTerminalHandle | null = null
let resizeObserver: ResizeObserver | null = null

const { theme } = useTheme()

// buildTerminalTheme reads the current CSS custom properties from :root,
// so it reflects whichever data-theme (light / dark) is active. Invoked
// both at terminal creation and whenever the app-level theme flips.
function buildTerminalTheme(): ITheme {
  const styles = getComputedStyle(document.documentElement)
  const readVar = (name: string, fallback: string) =>
    styles.getPropertyValue(name).trim() || fallback
  return {
    background: readVar('--panel', '#f6f2ea'),
    foreground: readVar('--ink', '#14120f'),
    cursor: readVar('--ink', '#14120f'),
    selectionBackground: readVar('--rule-soft', '#cfc7b8'),
  }
}

function focusTerminal() {
  terminal?.focus()
}

function applyFit() {
  if (!terminal || !fitAddon || !handle) return
  try {
    fitAddon.fit()
    const dims = fitAddon.proposeDimensions()
    if (dims && dims.cols > 0 && dims.rows > 0) {
      handle.resize(dims.cols, dims.rows)
    }
  } catch {
    /* container not measurable yet; retry on next ResizeObserver fire */
  }
}

function openTerminal() {
  const host = containerRef.value
  if (!host) return

  const styles = getComputedStyle(document.documentElement)
  const readVar = (name: string, fallback: string) =>
    styles.getPropertyValue(name).trim() || fallback

  terminal = new Terminal({
    cursorBlink: true,
    fontFamily: readVar('--mono', 'ui-monospace, monospace'),
    fontSize: 12,
    scrollback: 2000,
    theme: buildTerminalTheme(),
    allowProposedApi: false,
    convertEol: false,
  })
  fitAddon = new FitAddon()
  terminal.loadAddon(fitAddon)
  terminal.open(host)

  // Bidirectional PTY bridge. Writes from xterm.js (onData) go straight
  // to the server as input; output chunks from the server get written
  // back into the terminal buffer.
  handle = connectProcessLogsWS(props.sessionId, {
    onOutput: (bytes) => {
      terminal?.write(bytes)
    },
    onState: () => {
      // State transitions also flow on the main session WS; nothing to
      // do locally other than letting the capsule shell update.
    },
  })

  terminal.onData((chunk) => {
    handle?.writeInput(chunk)
  })

  nextTick(() => {
    applyFit()
    focusTerminal()
  })

  if (typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver(() => applyFit())
    resizeObserver.observe(host)
  }
}

function closeTerminal() {
  resizeObserver?.disconnect()
  resizeObserver = null
  handle?.close()
  handle = null
  terminal?.dispose()
  terminal = null
  fitAddon = null
}

onMounted(openTerminal)

// App theme toggled (light ↔ dark): xterm caches its theme colors at
// construction, so we have to hand it a fresh ITheme each time the CSS
// variables rebind. options.theme triggers a full redraw inside xterm.
watch(theme, () => {
  if (!terminal) return
  // CSS variable updates land in the same tick as the data-theme flip
  // on :root; next-tick read guarantees the new values are in effect.
  nextTick(() => {
    if (!terminal) return
    terminal.options.theme = buildTerminalTheme()
  })
})
onBeforeUnmount(closeTerminal)

watch(
  () => props.sessionId,
  (id, oldId) => {
    if (id === oldId) return
    closeTerminal()
    openTerminal()
  },
)
</script>

<style scoped>
.term-panel {
  --panel-height: 220px;
  display: flex;
  height: var(--panel-height);
  background: var(--panel);
  overflow: hidden;
  pointer-events: auto;
}

.term-host {
  flex: 1;
  min-width: 0;
  min-height: 0;
  padding: 8px 6px 6px 10px;
}

/* xterm injects its own .xterm elements; fix their sizing to fill our host */
.term-host :deep(.xterm),
.term-host :deep(.xterm-viewport),
.term-host :deep(.xterm-screen) {
  width: 100% !important;
  height: 100% !important;
}
</style>
