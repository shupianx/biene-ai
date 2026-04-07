<template>
  <BaseModal :title="title" max-width="400px" :z-index="230" @close="emit('cancel')">
    <p class="message">{{ message }}</p>

    <template #footer>
      <button class="btn-cancel" @click="emit('cancel')">Cancel</button>
      <button class="btn-confirm" @click="emit('confirm')">{{ confirmLabel || 'Confirm' }}</button>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import BaseModal from './BaseModal.vue'

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
  color: #4b5563;
  font-size: 14px;
  line-height: 1.5;
}
.btn-cancel {
  padding: 8px 16px; border-radius: 8px; border: 1.5px solid #e5e7eb;
  background: #fff; color: #374151; font-size: 13px; font-weight: 600;
  cursor: pointer;
}
.btn-cancel:hover { background: #f9fafb; }
.btn-confirm {
  padding: 8px 18px; border-radius: 8px; border: none;
  background: #ef4444; color: #fff; font-size: 13px; font-weight: 600;
  cursor: pointer;
}
.btn-confirm:hover { background: #dc2626; }
</style>
