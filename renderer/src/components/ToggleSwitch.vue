<template>
  <label class="switch" :aria-label="label">
    <input
      :checked="modelValue"
      :aria-label="label"
      class="switch-input"
      type="checkbox"
      :disabled="disabled"
      @change="onChange"
    />
    <span class="switch-track" />
  </label>
</template>

<script setup lang="ts">
defineProps<{
  modelValue: boolean
  label?: string
  disabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()

function onChange(event: Event) {
  emit('update:modelValue', (event.target as HTMLInputElement).checked)
}
</script>

<style scoped>
.switch {
  position: relative;
  width: 46px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  flex-shrink: 0;
  cursor: pointer;
}

.switch-input {
  position: absolute;
  inset: 0;
  opacity: 0;
  margin: 0;
  cursor: pointer;
}

.switch-track {
  position: relative;
  width: 46px;
  height: 28px;
  border-radius: 999px;
  background: #d1d5db;
  transition: background .18s ease, box-shadow .18s ease;
  box-shadow: inset 0 0 0 1px rgba(17, 24, 39, .06);
}

.switch-track::after {
  content: '';
  position: absolute;
  top: 3px;
  left: 3px;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: #fff;
  box-shadow: 0 2px 6px rgba(15, 23, 42, .18);
  transition: transform .18s ease;
}

.switch-input:checked + .switch-track {
  background: var(--accent-warm-bg-active);
  box-shadow: inset 0 0 0 1px rgba(251, 146, 60, .28);
}

.switch-input:checked + .switch-track::after {
  transform: translateX(18px);
}

.switch-input:focus-visible + .switch-track {
  outline: 2px solid var(--accent-warm-ring);
  outline-offset: 2px;
}

.switch-input:disabled + .switch-track {
  opacity: .55;
}

.switch-input:disabled,
.switch-input:disabled + .switch-track {
  cursor: not-allowed;
}
</style>
