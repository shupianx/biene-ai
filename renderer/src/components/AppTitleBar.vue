<template>
  <header
    class="titlebar"
    :class="[
      `platform-${platform}`,
      `window-${windowKind}`,
      { electron: isElectron, compact: true },
    ]"
  >
    <div class="titlebar-brand">
      <span class="brand-mark">Biene</span>
      <span class="brand-context">{{ contextLabel }}</span>
    </div>
    <div class="titlebar-actions" @click.stop>
      <button
        v-if="isElectron"
        class="core-status-button"
        type="button"
        :title="coreHeartbeatLabel"
        :aria-label="coreHeartbeatLabel"
        @click="onCoreMenu"
      >
        <span
          class="core-heartbeat"
          :class="isCoreHealthy ? 'online' : 'offline'"
        />
        <span class="core-status-text">{{ coreStatusText }}</span>
      </button>
      <button
        class="settings-button"
        type="button"
        :aria-label="t('titleBar.openSettingsMenu')"
        @click="onSettingsMenu"
      >
        <svg class="settings-icon" viewBox="0 0 24 24" aria-hidden="true" v-html="settingsHeartIconBody" />
      </button>
    </div>
    <DesktopSettingsModal v-if="settingsModalOpen" @close="settingsModalOpen = false" />
  </header>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { getDesktopBridge } from '../runtime'
import { useCoreHeartbeat } from '../composables/useCoreHeartbeat'
import DesktopSettingsModal from './DesktopSettingsModal.vue'
import { t } from '../i18n'

const bridge = getDesktopBridge()
const platform = bridge?.platform ?? 'web'
const isElectron = bridge?.isElectron ?? false
const windowKind = bridge?.windowKind ?? 'main'
const settingsModalOpen = ref(false)
const contextLabel = computed(() => t('titleBar.context'))
const settingsHeartIconBody = '<path fill="currentColor" d="M12.425 15.513q.175-.063.325-.213l2.8-2.8q.425-.425.55-1.037t-.125-1.188t-.75-.925T14.1 9t-1.125.388t-.925.812q-.45-.425-.937-.812T10 9t-1.137.338t-.763.912t-.112 1.188t.562 1.062l2.775 2.8q.15.15.338.213t.387.062t.375-.062M10.825 22q-.675 0-1.162-.45t-.588-1.1L8.85 18.8q-.325-.125-.612-.3t-.563-.375l-1.55.65q-.625.275-1.25.05t-.975-.8l-1.175-2.05q-.35-.575-.2-1.225t.675-1.075l1.325-1Q4.5 12.5 4.5 12.337v-.675q0-.162.025-.337l-1.325-1Q2.675 9.9 2.525 9.25t.2-1.225L3.9 5.975q.35-.575.975-.8t1.25.05l1.55.65q.275-.2.575-.375t.6-.3l.225-1.65q.1-.65.588-1.1T10.825 2h2.35q.675 0 1.163.45t.587 1.1l.225 1.65q.325.125.613.3t.562.375l1.55-.65q.625-.275 1.25-.05t.975.8l1.175 2.05q.35.575.2 1.225t-.675 1.075l-1.325 1q.025.175.025.338v.674q0 .163-.05.338l1.325 1q.525.425.675 1.075t-.2 1.225l-1.2 2.05q-.35.575-.975.8t-1.25-.05l-1.5-.65q-.275.2-.575.375t-.6.3l-.225 1.65q-.1.65-.587 1.1t-1.163.45zM11 20h1.975l.35-2.65q.775-.2 1.438-.587t1.212-.938l2.475 1.025l.975-1.7l-2.15-1.625q.125-.35.175-.737T17.5 12t-.05-.787t-.175-.738l2.15-1.625l-.975-1.7l-2.475 1.05q-.55-.575-1.212-.962t-1.438-.588L13 4h-1.975l-.35 2.65q-.775.2-1.437.588t-1.213.937L5.55 7.15l-.975 1.7l2.15 1.6q-.125.375-.175.75t-.05.8q0 .4.05.775t.175.75l-2.15 1.625l.975 1.7l2.475-1.05q.55.575 1.213.963t1.437.587zm1-8"/>' // sourced from @iconify-json/material-symbols
const { isCoreHealthy } = useCoreHeartbeat()
const coreStatusText = computed(() => (
  isCoreHealthy.value ? t('titleBar.coreRunning') : t('titleBar.coreStopped')
))
const coreHeartbeatLabel = computed(() => coreStatusText.value)

function onCoreMenu() {
  if (!bridge?.showCoreMenu) return
  void bridge.showCoreMenu({
    killCore: t('titleBar.killCore'),
    runCore: t('titleBar.runCore'),
  })
}

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
  height: 40px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 0 16px;
  border-bottom: 1px solid var(--titlebar-border);
  background: var(--titlebar-bg);
  color: var(--titlebar-text);
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
  padding-left: 16px;
  padding-right: 16px;
}

.titlebar.compact {
  justify-content: space-between;
}

.titlebar-brand {
  min-width: 0;
  display: inline-flex;
  align-items: baseline;
  gap: 8px;
}

.brand-mark {
  font-size: 13px;
  font-weight: bold;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--titlebar-text-strong);
}

.brand-context {
  font-size: 11px;
  color: var(--titlebar-text-muted);
}

.titlebar-actions {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  -webkit-app-region: no-drag;
}

.core-status-button {
  padding: 0 4px 0 2px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--titlebar-text-muted);
  cursor: pointer;
  transition: background .15s, color .15s;
}

.core-status-button:hover {
  background: var(--titlebar-hover);
  color: var(--titlebar-text);
}

.core-heartbeat {
  width: 9px;
  height: 9px;
  flex-shrink: 0;
  border-radius: 50%;
  box-shadow: 0 0 0 2px rgba(15, 23, 42, .05);
}

.core-heartbeat.online {
  background: #22c55e;
}

.core-heartbeat.offline {
  background: #ef4444;
}

.core-status-text {
  font-size: 11px;
  line-height: 1;
  white-space: nowrap;
}

.settings-button {
  width: 30px;
  height: 30px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: 9px;
  background: transparent;
  color: var(--titlebar-icon);
  cursor: pointer;
  transition: background .15s, color .15s;
}

.settings-button:hover {
  background: var(--titlebar-hover);
  color: var(--titlebar-text-strong);
}

.settings-icon {
  width: 20px;
  height: 20px;
}

</style>
