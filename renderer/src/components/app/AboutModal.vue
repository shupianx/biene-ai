<template>
  <BaseModal :title="t('about.title')" max-width="380px" :z-index="240" @close="emit('close')">
    <div class="about-body">
      <img :src="bieneLogo" class="about-logo" alt="" aria-hidden="true" />
      <div class="about-name">BIENE</div>
      <div class="about-version">v{{ version }}</div>
      <div class="about-rule" aria-hidden="true" />
      <div class="about-copyright">© {{ copyrightYear }} Yu</div>
    </div>

    <template #footer>
      <AppButton variant="neutral" @click="emit('close')">
        {{ t('common.close') }}
      </AppButton>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import BaseModal from '../ui/BaseModal.vue'
import AppButton from '../ui/AppButton.vue'
import bieneLogo from '../../assets/biene-logo.png'
import { t } from '../../i18n'

const emit = defineEmits<{
  (e: 'close'): void
}>()

// Build-time injection from vite.config.ts (root package.json version).
// Falls back gracefully if the global isn't defined (e.g., when this
// component is mounted in an unconfigured test runner).
const version =
  typeof __APP_VERSION__ === 'string' && __APP_VERSION__ ? __APP_VERSION__ : '0.0.0'

// Hardcoded — copyright is never localised; the year auto-rolls without
// a deploy each new year so the value stays accurate even if the
// release pipeline goes quiet.
const copyrightYear = new Date().getFullYear()
</script>

<style scoped>
.about-body {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 4px 0 8px;
}

.about-logo {
  width: 56px;
  height: 50px;
  margin-bottom: 4px;
}

.about-name {
  font-family: var(--mono);
  font-size: 16px;
  font-weight: 600;
  letter-spacing: 0.18em;
  color: var(--ink);
}

.about-version {
  font-family: var(--mono);
  font-size: 11px;
  letter-spacing: 0.08em;
  color: var(--ink-3);
}

.about-rule {
  width: 60%;
  margin: 8px 0 4px;
  border-top: 1px dashed var(--rule-soft);
}

.about-copyright {
  font-family: var(--mono);
  font-size: 10.5px;
  letter-spacing: 0.06em;
  color: var(--ink-4);
}
</style>
