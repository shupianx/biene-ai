<template>
  <footer class="app-footer">
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
  </footer>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useCoreHeartbeat } from '../composables/useCoreHeartbeat'
import { t } from '../i18n'
import { getDesktopBridge } from '../runtime'

const bridge = getDesktopBridge()
const isElectron = bridge?.isElectron ?? false
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
</script>

<style scoped>
.app-footer {
  position: relative;
  height: 28px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  padding: 0 16px;
  background: var(--titlebar-bg);
  color: var(--titlebar-text);
}

.app-footer::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 1px;
  background: var(--titlebar-border);
  pointer-events: none;
  z-index: 2;
}

.core-status-button {
  position: relative;
  z-index: 1;
  padding: 0 8px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  border: none;
  border-radius: 0;
  background: transparent;
  color: var(--titlebar-text-muted);
  cursor: pointer;
  transition: background .15s, color .15s;
}

.core-status-button:hover {
  background: var(--footer-action-hover-bg);
  color: var(--footer-action-hover-text);
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
</style>
