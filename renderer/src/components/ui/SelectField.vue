<template>
  <div
    ref="wrapRef"
    class="select-field"
    :class="{ open, disabled }"
  >
    <button
      ref="triggerRef"
      class="select-trigger"
      type="button"
      :disabled="disabled"
      :aria-label="ariaLabel"
      :aria-haspopup="'listbox'"
      :aria-expanded="open"
      @click="toggle"
      @keydown="onTriggerKeydown"
    >
      <span class="select-value">{{ displayLabel }}</span>
      <svg
        class="select-caret"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <polyline points="6 9 12 15 18 9" />
      </svg>
    </button>
    <ul
      v-if="open"
      class="select-menu"
      role="listbox"
      :aria-activedescendant="activeId"
    >
      <li
        v-for="(option, index) in options"
        :id="optionId(index)"
        :key="option.value"
        class="select-option"
        :class="{
          active: option.value === modelValue,
          highlight: index === highlightedIndex,
        }"
        role="option"
        :aria-selected="option.value === modelValue"
        @mousedown.prevent="onPick(option.value)"
        @mouseenter="highlightedIndex = index"
      >
        {{ option.label }}
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts" generic="T extends string">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

interface SelectOption {
  value: T
  label: string
}

const props = defineProps<{
  modelValue: T
  options: SelectOption[]
  ariaLabel?: string
  disabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: T): void
}>()

const wrapRef = ref<HTMLElement | null>(null)
const triggerRef = ref<HTMLButtonElement | null>(null)
const open = ref(false)
const highlightedIndex = ref(-1)

const displayLabel = computed(() => {
  const match = props.options.find(o => o.value === props.modelValue)
  return match?.label ?? ''
})

const activeId = computed(() =>
  highlightedIndex.value >= 0 ? optionId(highlightedIndex.value) : undefined,
)

function optionId(index: number) {
  return `select-opt-${index}`
}

function setOpen(value: boolean) {
  if (open.value === value) return
  open.value = value
  if (value) {
    const idx = props.options.findIndex(o => o.value === props.modelValue)
    highlightedIndex.value = idx >= 0 ? idx : 0
  }
}

function toggle() {
  if (props.disabled) return
  setOpen(!open.value)
}

function onPick(value: T) {
  emit('update:modelValue', value)
  setOpen(false)
  // Return focus to the trigger so keyboard nav continues naturally.
  void nextTick(() => triggerRef.value?.focus())
}

function onTriggerKeydown(event: KeyboardEvent) {
  if (props.disabled) return
  if (!open.value) {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp' || event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      setOpen(true)
    }
    return
  }
  if (event.key === 'Escape') {
    event.preventDefault()
    setOpen(false)
    return
  }
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    moveHighlight(1)
    return
  }
  if (event.key === 'ArrowUp') {
    event.preventDefault()
    moveHighlight(-1)
    return
  }
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    const picked = props.options[highlightedIndex.value]
    if (picked) onPick(picked.value)
    return
  }
  if (event.key === 'Tab') {
    setOpen(false)
  }
}

function moveHighlight(delta: number) {
  if (!props.options.length) return
  const next = (highlightedIndex.value + delta + props.options.length) % props.options.length
  highlightedIndex.value = next
}

function handlePointerDown(event: MouseEvent) {
  if (!open.value) return
  if (wrapRef.value?.contains(event.target as Node)) return
  setOpen(false)
}

watch(
  () => props.modelValue,
  () => {
    if (open.value) setOpen(false)
  },
)

onMounted(() => document.addEventListener('pointerdown', handlePointerDown))
onBeforeUnmount(() => document.removeEventListener('pointerdown', handlePointerDown))
</script>

<style scoped>
.select-field {
  position: relative;
  display: inline-block;
  min-width: 140px;
}

.select-trigger {
  width: 100%;
  height: 30px;
  padding: 0 8px 0 10px;
  display: inline-flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  border: 1px solid var(--rule-soft);
  background: var(--panel-2);
  color: var(--ink);
  font-family: var(--mono);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.06em;
  cursor: pointer;
  transition: border-color .12s;
}

.select-trigger:hover:not(:disabled) {
  border-color: var(--rule);
}

.select-trigger:focus-visible {
  outline: none;
  border-color: var(--ink);
}

.select-field.open .select-trigger {
  border-color: var(--ink);
}

.select-field.disabled .select-trigger {
  opacity: 0.55;
  cursor: not-allowed;
}

.select-value {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: left;
}

.select-caret {
  width: 12px;
  height: 12px;
  flex: 0 0 auto;
  color: var(--ink-3);
  transition: transform .12s;
}

.select-field.open .select-caret {
  transform: rotate(180deg);
}

.select-menu {
  position: absolute;
  top: calc(100% + 2px);
  left: 0;
  right: 0;
  margin: 0;
  padding: 4px;
  list-style: none;
  background: var(--panel-2);
  border: 1px solid var(--rule);
  box-shadow: 3px 3px 0 0 var(--rule);
  z-index: 240;
  max-height: 240px;
  overflow-y: auto;
}

.select-option {
  padding: 6px 10px;
  font-family: var(--sans);
  font-size: 12px;
  color: var(--ink-2);
  cursor: pointer;
  user-select: none;
}

.select-option.highlight {
  background: var(--bg-2);
  color: var(--ink);
}

.select-option.active {
  color: var(--ink);
  font-weight: 600;
}

.select-option.active::after {
  content: '·';
  margin-left: 6px;
  color: var(--ink-3);
}
</style>
