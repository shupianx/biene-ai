<template>
  <div ref="wrapRef" class="menu-wrap" :class="{ 'trigger-block': hasTriggerSlot }" @click.stop>
    <slot name="trigger" :open="open" :toggle="toggle">
      <button
        class="menu-btn"
        :class="{ visible: visible || open, open }"
        :title="title || t('common.more')"
        :aria-label="title || t('common.more')"
        @click="toggle"
      >
        <svg viewBox="0 0 24 24" aria-hidden="true" v-html="icon || defaultIcon" />
      </button>
    </slot>
    <div v-if="open" class="menu" :class="[placementClass, { 'menu-block': hasTriggerSlot }]">
      <template v-for="(item, index) in items" :key="'separator' in item ? `sep-${index}` : item.key">
        <div v-if="'separator' in item" class="menu-sep" aria-hidden="true" />
        <button
          v-else
          class="menu-item"
          :class="{ danger: item.danger, selected: item.selected, accent: item.accent }"
          :disabled="item.disabled"
          @click="onSelect(item)"
        >
          <span class="menu-item-label">{{ item.label }}</span>
          <StarShineIcon v-if="item.accent" class="menu-item-accent-icon" aria-hidden="true" />
        </button>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, useSlots } from 'vue'
import StarShineIcon from '~icons/material-symbols/star-shine'
import { t } from '../../i18n'

export type PopupMenuItem = {
  key: string
  label: string
  danger?: boolean
  disabled?: boolean
  selected?: boolean
  // accent appends a star-shine icon after the label so "premium" /
  // OAuth-derived options read as a distinct class without disturbing
  // the dropdown's typography. Currently used for ChatGPT-OAuth
  // models in the New Agent picker.
  accent?: boolean
}

export type PopupMenuSeparator = { separator: true }

export type PopupMenuEntry = PopupMenuItem | PopupMenuSeparator

const props = withDefaults(
  defineProps<{
    items: PopupMenuEntry[]
    visible?: boolean
    title?: string
    icon?: string
    placement?: 'bottom-right' | 'bottom-left'
  }>(),
  {
    visible: false,
    placement: 'bottom-right',
  }
)

const emit = defineEmits<{
  (e: 'select', key: string): void
  (e: 'open-change', open: boolean): void
}>()

const defaultIcon =
  '<path fill="currentColor" d="M6 14q-.825 0-1.412-.587T4 12t.588-1.412T6 10t1.413.588T8 12t-.587 1.413T6 14m6 0q-.825 0-1.412-.587T10 12t.588-1.412T12 10t1.413.588T14 12t-.587 1.413T12 14m6 0q-.825 0-1.412-.587T16 12t.588-1.412T18 10t1.413.588T20 12t-.587 1.413T18 14"/>'

const slots = useSlots()
const hasTriggerSlot = computed(() => !!slots.trigger)

const wrapRef = ref<HTMLElement | null>(null)
const open = ref(false)

const placementClass = computed(() =>
  props.placement === 'bottom-left' ? 'placement-bl' : 'placement-br'
)

function setOpen(value: boolean) {
  if (open.value === value) return
  open.value = value
  emit('open-change', value)
}

function toggle() {
  setOpen(!open.value)
}

function onSelect(item: PopupMenuItem) {
  if (item.disabled) return
  setOpen(false)
  emit('select', item.key)
}

function handlePointerDown(event: MouseEvent) {
  if (!open.value) return
  if (wrapRef.value?.contains(event.target as Node)) return
  setOpen(false)
}

onMounted(() => document.addEventListener('pointerdown', handlePointerDown))
onBeforeUnmount(() => document.removeEventListener('pointerdown', handlePointerDown))

defineExpose({ close: () => setOpen(false), toggle })
</script>

<style scoped>
.menu-wrap {
  position: relative;
  display: inline-block;
}

.menu-wrap.trigger-block {
  display: block;
}

.menu-btn {
  width: 24px;
  height: 24px;
  border: none;
  background: transparent;
  color: var(--ink-4);
  cursor: pointer;
  display: grid;
  place-items: center;
  opacity: 0;
  transition: opacity .12s, background .12s, color .12s;
}

.menu-btn.visible,
.menu-btn:focus-visible {
  opacity: 1;
}

.menu-btn.open {
  background: var(--bg-2);
}

.menu-btn:hover {
  color: var(--ink-2);
  background: var(--bg-2);
}

.menu-btn svg {
  width: 14px;
  height: 14px;
}

.menu {
  position: absolute;
  top: calc(100% + 4px);
  min-width: 144px;
  padding: 4px;
  background: var(--panel-2);
  border: 1px solid var(--rule);
  box-shadow: 3px 3px 0 0 var(--rule);
  z-index: 10;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.menu.menu-block {
  left: 0;
  right: 0;
  width: auto;
}

.menu.placement-br {
  right: 0;
}

.menu.placement-bl {
  left: 0;
}

.menu-item {
  display: flex;
  align-items: center;
  gap: 6px;
  border: none;
  background: transparent;
  text-align: left;
  padding: 6px 10px;
  font-family: var(--sans);
  font-size: 12px;
  color: var(--ink-2);
  cursor: pointer;
}

.menu-item-label {
  /*
   * `flex: 0 1 auto` keeps the label at its natural width but lets it
   * shrink (with ellipsis) when the menu hits its max width. Crucially
   * it does NOT grow — without this the label would eat all available
   * row width and push trailing decorations (the .accent star) to the
   * far right edge, far away from the text it's meant to annotate.
   */
  flex: 0 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.menu-item:hover:not(:disabled) {
  background: var(--bg-2);
}

.menu-item.selected {
  color: var(--ink);
  font-weight: 600;
  background: var(--bg-2);
}

.menu-item:disabled {
  opacity: 0.5;
  cursor: default;
}

.menu-item.danger {
  color: var(--err);
}

.menu-item.danger:hover:not(:disabled) {
  background: var(--err);
  color: var(--panel-2);
}

/*
 * Accent items (currently: ChatGPT-OAuth-derived models) keep the
 * regular text color and hover/selected backgrounds — the only visual
 * cue is a small star-shine icon sitting right after the label. This
 * separates "premium" / OAuth-derived options from regular configs
 * without fighting the theme (the previous gradient was unreadable in
 * dark mode and against hovered backgrounds).
 *
 * Tone is intentionally muted: the star is a *secondary* signal
 * trailing the label, not a primary affordance. Bright accent color
 * pulled the eye away from the model name itself.
 */
.menu-item-accent-icon {
  flex: 0 0 auto;
  width: 14px;
  height: 14px;
  color: var(--ink-4);
  opacity: 0.55;
}

.menu-item.accent:hover:not(:disabled) .menu-item-accent-icon,
.menu-item.accent.selected .menu-item-accent-icon {
  /* Lift slightly on hover/selected so the user gets feedback that
   * the row is interactive, but still well short of the label's
   * full ink contrast. */
  color: var(--ink-3);
  opacity: 0.8;
}

.menu-sep {
  height: 1px;
  background: var(--rule-softer);
  margin: 4px 2px;
}
</style>
