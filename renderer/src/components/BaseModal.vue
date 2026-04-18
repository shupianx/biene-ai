<template>
  <Teleport to="body">
    <div class="backdrop" :style="{ zIndex: String(zIndex) }" @click.self="emit('close')">
      <div class="modal" :style="{ maxWidth }">
        <header class="modal-header">
          <span class="modal-title">{{ title }}</span>
          <button class="close-btn" :aria-label="t('common.close')" @click="emit('close')">✕</button>
        </header>

        <div class="modal-body">
          <slot />
        </div>

        <footer v-if="$slots.footer" class="modal-footer">
          <slot name="footer" />
        </footer>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { t } from '../i18n'

withDefaults(defineProps<{
  title: string
  maxWidth?: string
  zIndex?: number
}>(), {
  maxWidth: '440px',
  zIndex: 200,
})

const emit = defineEmits<{
  (e: 'close'): void
}>()
</script>

<style scoped>
.backdrop {
  position: fixed;
  inset: 0;
  background: var(--overlay);
  backdrop-filter: blur(2px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  animation: bieneFadeIn .15s ease-out;
}

.modal {
  width: 100%;
  max-height: calc(100dvh - 48px);
  background: var(--panel-2);
  border: 1px solid var(--rule);
  box-shadow: 4px 4px 0 0 var(--rule);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  border-bottom: 1px solid var(--rule);
  background: var(--panel);
  flex: 0 0 auto;
}

.modal-title {
  font-family: var(--mono);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--ink);
}

.close-btn {
  background: none;
  border: none;
  cursor: pointer;
  color: var(--ink-4);
  font-size: 14px;
  width: 22px;
  height: 22px;
  display: grid;
  place-items: center;
  line-height: 1;
  transition: background 0.12s, color 0.12s;
}

.close-btn:hover {
  background: var(--bg-2);
  color: var(--ink);
}

.modal-body {
  padding: 18px;
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
  flex-direction: column;
  gap: 16px;
  overflow-y: auto;
  overscroll-behavior: contain;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 12px 18px;
  border-top: 1px solid var(--rule);
  background: var(--panel);
  flex: 0 0 auto;
}

@media (max-width: 640px) {
  .backdrop {
    padding: 12px;
  }

  .modal {
    max-height: calc(100dvh - 24px);
  }

  .modal-header {
    padding: 12px 14px;
  }

  .modal-body {
    padding: 14px;
  }

  .modal-footer {
    padding: 10px 14px;
  }
}
</style>
