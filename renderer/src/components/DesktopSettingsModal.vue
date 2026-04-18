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
  gap: 10px;
}

.setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 14px;
  border: 1px solid var(--rule-softer);
  background: var(--panel);
}

.setting-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.setting-label {
  font-family: var(--mono);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--ink-2);
}

.setting-hint {
  font-size: 12px;
  line-height: 1.45;
  color: var(--ink-4);
}

.btn-close {
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

.btn-close:hover {
  transform: translate(-1px, -1px);
  box-shadow: 2px 2px 0 0 var(--rule);
}

.btn-close:active {
  transform: translate(0, 0);
  box-shadow: none;
}
</style>
