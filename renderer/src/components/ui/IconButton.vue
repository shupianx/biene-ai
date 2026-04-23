<template>
  <button
    class="icon-btn"
    :class="[`icon-btn--${size}`, `icon-btn--${variant}`]"
    :type="type"
    :disabled="disabled"
  >
    <slot />
  </button>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  size?: 'default' | 'small'
  variant?: 'neutral' | 'danger'
  disabled?: boolean
  type?: 'button' | 'submit' | 'reset'
}>(), {
  size: 'default',
  variant: 'neutral',
  type: 'button',
})
</script>

<style scoped>
.icon-btn {
  -webkit-app-region: no-drag;
  width: 26px;
  height: 26px;
  padding: 0;
  border: none;
  background: transparent;
  color: var(--ink-3);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: background .12s, color .12s;
}

.icon-btn:hover:not(:disabled) {
  background: var(--bg-2);
  color: var(--ink);
}

.icon-btn:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.icon-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.icon-btn--small {
  width: 22px;
  height: 22px;
}

.icon-btn--danger {
  color: var(--err);
}

.icon-btn--danger:hover:not(:disabled) {
  color: var(--err);
  background: color-mix(in srgb, var(--err) 10%, transparent);
}
</style>
