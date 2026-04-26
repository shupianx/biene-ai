<template>
  <div class="notice-board">
    <Transition :name="transitionName" appear>
      <div
        v-if="top"
        :key="top.id"
        class="notice"
        :class="'tone-' + top.tone"
      >
        <component
          v-if="top.icon"
          :is="top.icon"
          class="notice-icon"
          aria-hidden="true"
        />
        <component
          v-if="top.body"
          :is="top.body"
          v-bind="top.bodyProps ?? {}"
          class="notice-body"
        />
        <span v-else class="notice-text">{{ top.text }}</span>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import type { Notice } from './notice'

const props = defineProps<{ stack: Notice[] }>()
const emit = defineEmits<{ (e: 'expire', id: string): void }>()

const top = computed<Notice | null>(() =>
  props.stack.length ? props.stack[props.stack.length - 1] : null,
)

// Direction of the top change decides which transition classes run.
// push — a brand-new notice landed on top (slides in from above, shoves the
// previous one downward out of view).
// pop — the previous top was removed and an older notice resurfaced
// (rises up from below; the removed one exits upward).
const transitionName = ref<'push' | 'pop'>('push')
let lastStackIds: string[] = []

watch(
  () => props.stack,
  (newStack) => {
    const newTopId = newStack.length ? newStack[newStack.length - 1].id : null
    const oldTopId = lastStackIds.length
      ? lastStackIds[lastStackIds.length - 1]
      : null
    if (newTopId !== oldTopId) {
      if (!newTopId || lastStackIds.includes(newTopId)) {
        transitionName.value = 'pop'
      } else {
        transitionName.value = 'push'
      }
    }
    lastStackIds = newStack.map(n => n.id)
  },
  { immediate: true },
)

let timer: number | null = null

function clearTimer() {
  if (timer !== null) {
    window.clearTimeout(timer)
    timer = null
  }
}

watch(
  () => top.value?.id,
  () => {
    clearTimer()
    const current = top.value
    if (!current || !current.ttlMs) return
    const id = current.id
    timer = window.setTimeout(() => {
      timer = null
      emit('expire', id)
    }, current.ttlMs)
  },
  { immediate: true },
)

onBeforeUnmount(clearTimer)
</script>

<style scoped>
/* Grid overlap: entering and leaving notices share one cell, so the board's
 * height stays stable at one notice tall during the swap and neither element
 * perturbs the other's layout. overflow: hidden clips the translate-out. */
.notice-board {
  display: grid;
  grid-template-columns: 1fr;
  overflow: hidden;
}

.notice {
  grid-column: 1;
  grid-row: 1;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  font-family: var(--mono);
  font-size: 11px;
  line-height: 1.4;
  min-width: 0;
}

.notice-icon {
  width: 13px;
  height: 13px;
  flex: 0 0 auto;
  opacity: 0.7;
}

.notice-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.tone-warn {
  color: var(--warn);
  background: color-mix(in srgb, var(--warn) 10%, transparent);
}
.tone-ok {
  color: var(--ok);
  background: color-mix(in srgb, var(--ok) 12%, transparent);
}
.tone-err {
  color: var(--err);
  background: color-mix(in srgb, var(--err) 10%, transparent);
}
.tone-info {
  color: var(--ink-2);
  /* Pale sage that drifts toward the cream panel — derived from the theme's
   * --ok green so light/dark modes stay coherent. The 12% blend keeps it
   * whisper-soft against the cream card so it reads as "running" rather
   * than "success" (which is the warn/ok tones' job). */
  background: color-mix(in srgb, var(--ok) 12%, var(--panel) 88%);
}

/* Metallic gleam — scoped to info-tone leading icons (currently only the
 * running-process notice). The icon's base sits at 0.7 opacity (the dim
 * default further up); during the peak we override BOTH opacity and color
 * so the icon visibly snaps from "dim grey" to "bright white with halo"
 * for ~150ms each cycle. Brightness alone wasn't enough — it was fighting
 * the dim opacity floor. */
.tone-info .notice-icon {
  animation: notice-icon-metal-shine 2.4s ease-in-out infinite;
  will-change: opacity, filter, color;
}

@keyframes notice-icon-metal-shine {
  0%, 100% {
    opacity: 0.7;
    color: var(--ink-2);
    filter: drop-shadow(0 0 0 transparent);
  }
  50% {
    opacity: 1;
    color: var(--ink-1);
    filter:
      drop-shadow(0 0 4px color-mix(in srgb, var(--ink-1) 70%, transparent))
      drop-shadow(0 0 8px color-mix(in srgb, var(--ink-1) 35%, transparent));
  }
}

.push-enter-active,
.pop-enter-active,
.push-leave-active,
.pop-leave-active {
  transition:
    transform .28s cubic-bezier(0.22, 1, 0.36, 1),
    opacity .22s ease;
}

/* Push: new drops in from above, old is shoved down and out. */
.push-enter-from {
  opacity: 0;
  transform: translateY(-100%);
}
.push-leave-to {
  opacity: 0;
  transform: translateY(100%);
}

/* Pop: old slides up and out, the one below rises into view from below. */
.pop-enter-from {
  opacity: 0;
  transform: translateY(100%);
}
.pop-leave-to {
  opacity: 0;
  transform: translateY(-100%);
}
</style>
