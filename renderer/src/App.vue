<template>
  <div class="app-shell">
    <AppTitleBar v-if="showDesktopTitleBar" />
    <div class="app-main-shell" :class="{ offline: !isCoreHealthy }">
      <div class="app-workspace">
        <main class="app-content">
          <RouterView />
        </main>
        <aside
          v-if="showSkillsSidebar"
          class="skills-sidebar-shell"
          :style="{ width: `${skillsSidebarWidth}px` }"
        >
          <SkillsBrowser
            embedded
            closable
            class="skills-sidebar"
            @close="onCloseSkillsSidebar"
          />
        </aside>
      </div>
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
    <AppFooter v-if="showDesktopFooter" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import AppTitleBar from './components/AppTitleBar.vue'
import AppFooter from './components/AppFooter.vue'
import SkillsBrowser from './components/SkillsBrowser.vue'
import { useCoreHeartbeat } from './composables/useCoreHeartbeat'
import { useSkillsSidebar } from './composables/useSkillsSidebar'
import { t } from './i18n'
import { getDesktopBridge } from './runtime'

const { isCoreHealthy } = useCoreHeartbeat()
const { skillsSidebarOpen, skillsSidebarWidth, closeSkillsSidebar } = useSkillsSidebar()
const showDesktopTitleBar = computed(() => {
  const bridge = getDesktopBridge()
  return Boolean(bridge?.isElectron && bridge.windowKind === 'main')
})

const showDesktopFooter = computed(() => {
  const bridge = getDesktopBridge()
  return Boolean(bridge?.isElectron && bridge.windowKind === 'main')
})

const showSkillsSidebar = computed(() => {
  const bridge = getDesktopBridge()
  return Boolean(bridge?.isElectron && bridge.windowKind === 'main' && skillsSidebarOpen.value)
})

function onCloseSkillsSidebar() {
  void closeSkillsSidebar()
}
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
  min-width: 0;
}

.app-workspace {
  flex: 1;
  min-height: 0;
  display: flex;
}

.skills-sidebar-shell {
  flex-shrink: 0;
  border-left: 1px solid var(--rule-soft);
  background: var(--panel);
  overflow: hidden;
}

.skills-sidebar {
  width: 100%;
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
