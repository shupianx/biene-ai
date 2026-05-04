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
      <img :src="bieneLogo" class="brand-logo" alt="" aria-hidden="true" />
      <span class="brand-name">BIENE</span>
      <span class="brand-divider" aria-hidden="true" />
      <span class="brand-context">{{ contextLabel }}</span>
    </div>
    <div class="titlebar-right" @click.stop>
      <div class="titlebar-actions">
        <IconButton
          :aria-label="t('titleBar.openSettingsMenu')"
          :title="t('titleBar.openSettingsMenu')"
          @click="onSettingsMenu"
        >
          <RiSettings3Line
            class="titlebar-icon"
            :style="{ transform: `rotate(${settingsIconRotation}deg)` }"
            aria-hidden="true"
          />
        </IconButton>
      </div>
      <div v-if="showCaptionButtons" class="caption-buttons">
        <button
          type="button"
          class="caption-btn"
          :aria-label="t('titleBar.minimize')"
          :title="t('titleBar.minimize')"
          @click="onMinimize"
        >
          <svg width="10" height="10" viewBox="0 0 10 10" aria-hidden="true">
            <path d="M0 5h10" stroke="currentColor" stroke-width="1" />
          </svg>
        </button>
        <button
          type="button"
          class="caption-btn"
          :aria-label="isMaximized ? t('titleBar.restore') : t('titleBar.maximize')"
          :title="isMaximized ? t('titleBar.restore') : t('titleBar.maximize')"
          @click="onToggleMaximize"
        >
          <svg v-if="!isMaximized" width="10" height="10" viewBox="0 0 10 10" aria-hidden="true">
            <rect x="0.5" y="0.5" width="9" height="9" fill="none" stroke="currentColor" stroke-width="1" />
          </svg>
          <svg v-else width="10" height="10" viewBox="0 0 10 10" aria-hidden="true">
            <path d="M2.5 0.5h7v7h-2M0.5 2.5h7v7h-7z" fill="none" stroke="currentColor" stroke-width="1" />
          </svg>
        </button>
        <button
          type="button"
          class="caption-btn caption-close"
          :aria-label="t('titleBar.close')"
          :title="t('titleBar.close')"
          @click="onClose"
        >
          <svg width="10" height="10" viewBox="0 0 10 10" aria-hidden="true">
            <path d="M0 0l10 10M10 0L0 10" stroke="currentColor" stroke-width="1" />
          </svg>
        </button>
      </div>
    </div>
    <DesktopSettingsModal v-if="settingsModalOpen" @close="settingsModalOpen = false" />
    <AboutModal v-if="aboutModalOpen" @close="aboutModalOpen = false" />
  </header>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import RiSettings3Line from '~icons/ri/settings-3-line'
import bieneLogo from '../../assets/biene-logo.png'
import { getDesktopBridge } from '../../runtime'
import DesktopSettingsModal from './DesktopSettingsModal.vue'
import AboutModal from './AboutModal.vue'
import IconButton from '../ui/IconButton.vue'
import { t } from '../../i18n'

const bridge = getDesktopBridge()
const platform = bridge?.platform ?? 'web'
const isElectron = bridge?.isElectron ?? false
const windowKind = bridge?.windowKind ?? 'main'
const settingsModalOpen = ref(false)
const aboutModalOpen = ref(false)
const settingsIconRotation = ref(0)
const contextLabel = computed(() => t('titleBar.context'))

// macOS keeps the native traffic lights (titleBarStyle 'hiddenInset' in
// the main process). On Windows/Linux Electron we render the caption
// buttons ourselves since titleBarStyle 'hidden' removes the native chrome.
const showCaptionButtons = computed(() => isElectron && platform !== 'darwin')
const isMaximized = ref(false)

function onMinimize() {
  void bridge?.windowMinimize?.()
}

async function onToggleMaximize() {
  if (!bridge?.windowToggleMaximize) return
  isMaximized.value = await bridge.windowToggleMaximize()
}

function onClose() {
  void bridge?.windowClose?.()
}

function onMaximizedChange(event: Event) {
  isMaximized.value = Boolean((event as CustomEvent<boolean>).detail)
}

function onSettingsMenu() {
  // Cumulative rotation: each click adds a half turn so the gear snaps to
  // a new orientation. Value grows forever in principle but only matters
  // after millions of clicks.
  settingsIconRotation.value += 180
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
  } else if (detail?.action === 'about') {
    aboutModalOpen.value = true
  }
}

onMounted(async () => {
  window.addEventListener('biene:settings-menu-action', onSettingsMenuAction as EventListener)
  window.addEventListener('biene:window-maximized-change', onMaximizedChange as EventListener)
  if (showCaptionButtons.value && bridge?.windowIsMaximized) {
    isMaximized.value = await bridge.windowIsMaximized()
  }
})
onBeforeUnmount(() => {
  window.removeEventListener('biene:settings-menu-action', onSettingsMenuAction as EventListener)
  window.removeEventListener('biene:window-maximized-change', onMaximizedChange as EventListener)
})
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

/* Custom caption buttons live inside .titlebar-right and reach the right
   edge themselves, so the titlebar zeroes its right padding on Windows.
   The settings-icon group keeps its own gap via .titlebar-actions. */
.titlebar.electron:not(.platform-darwin) {
  padding-right: 0;
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

.brand-logo {
  width: 20px;
  height: 18px;
  flex-shrink: 0;
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

.titlebar-right {
  display: flex;
  align-items: stretch;
  height: 100%;
  -webkit-app-region: no-drag;
}

.titlebar-actions {
  position: relative;
  display: flex;
  align-items: center;
  gap: 6px;
  padding-right: 4px;
}

.titlebar-icon {
  width: 15px;
  height: 15px;
  transition: transform .5s cubic-bezier(.4, .0, .2, 1);
}

/* Win11-style caption buttons: 46×40 hit targets, full title-bar height,
   subtle hover, close turns red on hover. The 1px-stroke SVG glyphs
   match Segoe Fluent Icons proportions. */
.caption-buttons {
  display: flex;
  align-items: stretch;
  height: 100%;
}

.caption-btn {
  width: 46px;
  height: 100%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: 0;
  padding: 0;
  margin: 0;
  color: var(--ink);
  cursor: pointer;
  transition: background-color .12s ease, color .12s ease;
}

.caption-btn:hover {
  background: var(--rule-soft);
}

.caption-btn:active {
  background: var(--rule);
}

.caption-close:hover {
  background: #c42b1c;
  color: #ffffff;
}

.caption-close:active {
  background: #b4271a;
  color: #ffffff;
}
</style>
