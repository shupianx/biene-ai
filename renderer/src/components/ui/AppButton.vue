<template>
  <button
    class="app-btn"
    :class="[`app-btn--${variant}`, `app-btn--${size}`]"
    :type="type"
    :disabled="disabled"
  >
    <slot />
  </button>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  variant?: 'neutral' | 'primary' | 'danger'
  size?: 'default' | 'compact'
  disabled?: boolean
  type?: 'button' | 'submit' | 'reset'
}>(), {
  variant: 'neutral',
  size: 'default',
  type: 'button',
})
</script>

<style scoped>
.app-btn {
  height: 30px;
  padding: 0 14px;
  border: 1px solid var(--rule);
  background: var(--panel-2);
  color: var(--ink-2);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  font-family: var(--mono);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  transition: transform .12s, box-shadow .12s, background .12s, color .12s, border-color .12s, opacity .12s;
}

.app-btn:hover:not(:disabled) {
  transform: translate(-1px, -1px);
  box-shadow: 2px 2px 0 0 var(--rule);
}

.app-btn:active:not(:disabled) {
  transform: translate(0, 0);
  box-shadow: none;
}

.app-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.app-btn:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.app-btn--compact {
  height: 28px;
  padding: 0 12px;
  font-size: 10px;
  letter-spacing: 0.14em;
}

.app-btn--primary {
  background: var(--ink);
  border-color: var(--ink);
  color: var(--panel-2);
}

.app-btn--danger {
  background: var(--err);
  border-color: var(--err);
  color: var(--panel-2);
}
</style>
