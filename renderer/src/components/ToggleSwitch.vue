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
    <span class="switch-track">
      <span class="switch-knob" />
    </span>
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
  width: 40px;
  height: 22px;
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
  width: 40px;
  height: 22px;
  background: var(--toggle-track-off, var(--switch-track-off));
  border: 1px solid var(--rule-soft);
  transition: background .15s ease, border-color .15s ease;
}

.switch-knob {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 16px;
  height: 16px;
  background: var(--toggle-knob-off, var(--switch-knob-off));
  transition: transform .15s ease, background .15s ease;
}

.switch-input:checked + .switch-track {
  background: var(--toggle-track-on, var(--switch-track-on));
  border-color: var(--toggle-track-on-border, var(--toggle-track-on, var(--switch-track-on)));
}

.switch-input:checked + .switch-track .switch-knob {
  background: var(--toggle-knob-on, var(--switch-knob-on));
  transform: translateX(18px);
}

.switch-input:focus-visible + .switch-track {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.switch-input:disabled + .switch-track {
  opacity: .5;
}

.switch-input:disabled,
.switch-input:disabled + .switch-track {
  cursor: not-allowed;
}
</style>
