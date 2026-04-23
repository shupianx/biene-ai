<template>
  <BaseModal :title="title" max-width="400px" :z-index="230" @close="emit('cancel')">
    <p class="message">{{ message }}</p>

    <template #footer>
      <AppButton variant="neutral" @click="emit('cancel')">{{ t('common.cancel') }}</AppButton>
      <AppButton variant="danger" @click="emit('confirm')">
        {{ confirmLabel || t('common.confirm') }}
      </AppButton>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import BaseModal from './BaseModal.vue'
import AppButton from './AppButton.vue'
import { t } from '../../i18n'

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
</style>
