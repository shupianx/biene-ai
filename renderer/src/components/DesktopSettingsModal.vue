<template>
  <BaseModal :title="t('modal.desktopSettingsTitle')" max-width="420px" :z-index="220" @close="emit('close')">
    <div class="setting-list">
      <div class="setting-row">
        <div class="setting-copy">
          <span class="setting-label">{{ t('titleBar.darkMode') }}</span>
          <span class="setting-hint">{{ t('modal.darkModeHint') }}</span>
        </div>
        <ToggleSwitch v-model="darkMode" :label="t('titleBar.darkMode')" />
      </div>

      <div v-if="desktopSettingsSupported" class="setting-row">
        <div class="setting-copy">
          <span class="setting-label">{{ t('titleBar.keepCoreRunningOnExit') }}</span>
          <span class="setting-hint">{{ t('modal.keepCoreRunningOnExitHint') }}</span>
        </div>
        <ToggleSwitch
          :model-value="keepCoreRunningOnExit"
          :label="t('titleBar.keepCoreRunningOnExit')"
          @update:model-value="onKeepCoreRunningOnExitChange"
        />
      </div>
    </div>

    <template #footer>
      <button class="btn-close" type="button" @click="emit('close')">{{ t('common.close') }}</button>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import BaseModal from './BaseModal.vue'
import ToggleSwitch from './ToggleSwitch.vue'
import { useTheme } from '../composables/useTheme'
import { useDesktopSettings } from '../composables/useDesktopSettings'
import { t } from '../i18n'

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { isDark, setTheme } = useTheme()
const {
  desktopSettingsSupported,
  keepCoreRunningOnExit,
  setKeepCoreRunningOnExit,
} = useDesktopSettings()

const darkMode = computed({
  get: () => isDark.value,
  set: (value: boolean) => setTheme(value ? 'dark' : 'light'),
})

function onKeepCoreRunningOnExitChange(value: boolean) {
  void setKeepCoreRunningOnExit(value)
}
</script>

<style scoped>
.setting-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 16px;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  background: #fff;
}

.setting-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.setting-label {
  font-size: 13px;
  font-weight: 700;
  color: #111827;
}

.setting-hint {
  font-size: 12px;
  line-height: 1.45;
  color: #6b7280;
}

.btn-close {
  min-width: 88px;
  height: 36px;
  padding: 0 14px;
  border: 1px solid #d1d5db;
  border-radius: 10px;
  background: #fff;
  color: #374151;
  font-size: 13px;
  font-weight: 700;
  cursor: pointer;
  transition: background .15s, border-color .15s, color .15s;
}

.btn-close:hover {
  background: #f9fafb;
  border-color: #cbd5e1;
}
</style>
