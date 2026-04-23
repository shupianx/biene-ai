<template>
  <header
    class="titlebar"
    :class="[
      `platform-${platform}`,
      `window-${windowKind}`,
      { electron: isElectron },
    ]"
  >
    <div class="titlebar-brand">
      <span class="brand-name">BIENE</span>
      <span class="brand-divider" aria-hidden="true" />
      <span class="brand-context">{{ contextLabel }}</span>
    </div>
    <div class="titlebar-actions" @click.stop>
      <IconButton
        :aria-label="t('titleBar.openSettingsMenu')"
        :title="t('titleBar.openSettingsMenu')"
        @click="onSettingsMenu"
      >
        <RiSettings3Line class="titlebar-icon" aria-hidden="true" />
      </IconButton>
    </div>
    <DesktopSettingsModal v-if="settingsModalOpen" @close="settingsModalOpen = false" />
  </header>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import RiSettings3Line from '~icons/ri/settings-3-line'
import { getDesktopBridge } from '../../runtime'
import DesktopSettingsModal from './DesktopSettingsModal.vue'
import IconButton from '../ui/IconButton.vue'
import { t } from '../../i18n'

const bridge = getDesktopBridge()
const platform = bridge?.platform ?? 'web'
const isElectron = bridge?.isElectron ?? false
const windowKind = bridge?.windowKind ?? 'main'
const settingsModalOpen = ref(false)
const contextLabel = computed(() => t('titleBar.context'))

function onSettingsMenu() {
  if (!bridge?.showSettingsMenu) {
    settingsModalOpen.value = true
    return
  }

  void bridge.showSettingsMenu({
    settings: t('common.settings'),
    about: t('titleBar.about'),
  })
}

function onSettingsMenuAction(event: Event) {
  const detail = (event as CustomEvent<{ action?: string }>).detail
  if (detail?.action === 'settings') {
    settingsModalOpen.value = true
  }
}

onMounted(() => window.addEventListener('biene:settings-menu-action', onSettingsMenuAction as EventListener))
onBeforeUnmount(() => window.removeEventListener('biene:settings-menu-action', onSettingsMenuAction as EventListener))
</script>

<style scoped>
.titlebar {
  position: relative;
  z-index: 40;
  flex-shrink: 0;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 0 14px 0 12px;
  background: var(--panel);
  color: var(--ink);
  border-bottom: 1px solid var(--rule-soft);
  -webkit-app-region: drag;
  user-select: none;
}

.titlebar.electron.platform-darwin {
  padding-left: 86px;
}

.titlebar.electron:not(.platform-darwin) {
  padding-right: 144px;
}

.titlebar.window-agent {
  padding-left: 12px;
  padding-right: 14px;
}

.titlebar-brand {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.brand-name {
  font-family: var(--mono);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.14em;
  color: var(--ink);
}

.brand-divider {
  width: 1px;
  height: 16px;
  background: var(--rule-soft);
}

.brand-context {
  font-family: var(--sans);
  font-size: 12px;
  color: var(--ink-3);
  letter-spacing: 0.02em;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.titlebar-actions {
  position: relative;
  display: flex;
  align-items: center;
  gap: 6px;
  -webkit-app-region: no-drag;
}

.titlebar-icon {
  width: 15px;
  height: 15px;
}
</style>
