<template>
  <BaseModal :title="title" max-width="400px" :z-index="230" @close="emit('cancel')">
    <p class="message">{{ message }}</p>

    <template #footer>
      <button class="btn-cancel" @click="emit('cancel')">{{ t('common.cancel') }}</button>
      <button class="btn-confirm" @click="emit('confirm')">{{ confirmLabel || t('common.confirm') }}</button>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import BaseModal from './BaseModal.vue'
import { t } from '../i18n'

defineProps<{
  title: string
  message: string
  confirmLabel?: string
}>()

const emit = defineEmits<{
  (e: 'cancel'): void
  (e: 'confirm'): void
}>()
</script>

<style scoped>
.message {
  margin: 0;
  color: var(--ink-2);
  font-size: 13.5px;
  line-height: 1.55;
}

.btn-cancel,
.btn-confirm {
  height: 30px;
  padding: 0 14px;
  border: 1px solid var(--rule);
  background: var(--panel-2);
  color: var(--ink-2);
  cursor: pointer;
  font-family: var(--mono);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  transition: transform .12s, box-shadow .12s;
}

.btn-cancel:hover,
.btn-confirm:hover {
  transform: translate(-1px, -1px);
  box-shadow: 2px 2px 0 0 var(--rule);
}

.btn-cancel:active,
.btn-confirm:active {
  transform: translate(0, 0);
  box-shadow: none;
}

.btn-confirm {
  background: var(--err);
  border-color: var(--err);
  color: var(--panel-2);
}
</style>
