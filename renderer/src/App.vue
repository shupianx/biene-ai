<template>
  <div class="app-shell">
    <AppTitleBar v-if="showDesktopChrome" />
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
    <AppFooter v-if="showDesktopChrome" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import AppTitleBar from './components/AppTitleBar.vue'
import AppFooter from './components/AppFooter.vue'
import { useCoreHeartbeat } from './composables/useCoreHeartbeat'
import { t } from './i18n'
import { getDesktopBridge } from './runtime'

const { isCoreHealthy } = useCoreHeartbeat()
const showDesktopChrome = computed(() => {
  const bridge = getDesktopBridge()
  return Boolean(bridge?.isElectron && bridge.windowKind === 'main')
})
</script>

<style scoped>
.app-shell {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--bg);
  color: var(--ink);
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
  inset: 0;
  z-index: 30;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: var(--overlay);
  backdrop-filter: blur(2px);
}

.core-offline-message {
  min-width: 240px;
  max-width: 340px;
  padding: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  border: 1px solid var(--rule);
  background: var(--panel-2);
  box-shadow: 3px 3px 0 0 var(--rule);
  text-align: center;
  color: var(--ink-2);
}

.core-offline-message strong {
  font-family: var(--mono);
  font-size: 11px;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--ink);
}

.core-offline-message span {
  font-size: 12px;
  line-height: 1.55;
  color: var(--ink-3);
}
</style>
