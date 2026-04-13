<template>
  <div class="app-shell">
    <AppTitleBar v-if="showTitleBar" />
    <div class="app-main-shell" :class="{ offline: !isCoreHealthy }">
      <main class="app-content">
        <RouterView />
      </main>
      <div
        v-if="!isCoreHealthy"
        class="core-offline-overlay"
        aria-live="polite"
      >
        <div class="core-offline-message">
          <strong>{{ t('titleBar.coreStopped') }}</strong>
          <span>{{ t('app.coreUnavailable') }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import AppTitleBar from './components/AppTitleBar.vue'
import { useCoreHeartbeat } from './composables/useCoreHeartbeat'
import { t } from './i18n'
import { getDesktopBridge } from './runtime'

const { isCoreHealthy } = useCoreHeartbeat()
const showTitleBar = computed(() => {
  const bridge = getDesktopBridge()
  return Boolean(bridge?.isElectron && bridge.windowKind === 'main')
})
</script>

<style scoped>
.app-shell {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--app-bg);
}

.app-main-shell {
  position: relative;
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  transition: opacity .18s ease;
}

.app-main-shell.offline {
  opacity: 0.52;
}

.app-content {
  flex: 1;
  min-height: 0;
}

.core-offline-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 30;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: rgba(248, 250, 252, 0.24);
  backdrop-filter: blur(2px);
}

.core-offline-message {
  min-width: 220px;
  max-width: 320px;
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  border: 1px solid rgba(226, 232, 240, 0.92);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.94);
  box-shadow: 0 16px 32px rgba(15, 23, 42, 0.08);
  text-align: center;
  color: #334155;
}

.core-offline-message strong {
  font-size: 13px;
  color: #0f172a;
}

.core-offline-message span {
  font-size: 12px;
  line-height: 1.5;
}
</style>
