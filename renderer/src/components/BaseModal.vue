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
  background: rgba(0, 0, 0, 0.45);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}

.modal {
  width: 100%;
  max-height: calc(100dvh - 48px);
  background: #fff;
  border-radius: 16px;
  display: flex;
  flex-direction: column;
  box-shadow: 0 24px 64px rgba(0, 0, 0, 0.2);
  overflow: hidden;
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 20px 0;
  flex: 0 0 auto;
}

.modal-title {
  font-size: 16px;
  font-weight: bold;
  color: #111827;
}

.close-btn {
  background: none;
  border: none;
  cursor: pointer;
  color: #9ca3af;
  font-size: 16px;
  padding: 4px 6px;
  border-radius: 6px;
  line-height: 1;
  transition: background 0.15s, color 0.15s;
}

.close-btn:hover {
  background: #f3f4f6;
  color: #374151;
}

.modal-body {
  padding: 20px;
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
  flex-direction: column;
  gap: 20px;
  overflow-y: auto;
  overscroll-behavior: contain;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 0 20px 20px;
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
    padding: 16px 16px 0;
  }

  .modal-body {
    padding: 16px;
  }

  .modal-footer {
    padding: 0 16px 16px;
  }
}
</style>
