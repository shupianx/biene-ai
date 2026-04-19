<template>
  <footer class="status-bar">
    <div v-if="runningCount > 0" class="status-item">
      <span class="dot ok" />
      <span>{{ t('statusBar.running', { n: runningCount }) }}</span>
    </div>
    <div v-if="pendingCount > 0" class="status-item warn">
      <span class="dot warn" />
      <span>{{ t('statusBar.pending', { n: pendingCount }) }}</span>
    </div>
    <div v-if="errorCount > 0" class="status-item err">
      <span class="dot err" />
      <span>{{ t('statusBar.errors', { n: errorCount }) }}</span>
    </div>

    <div class="status-item api" :title="apiHost">
      <span>{{ t('statusBar.apiPrefix') }} · {{ apiHost }}</span>
    </div>

    <button
      v-if="isElectron"
      class="status-item core-button"
      type="button"
      :title="coreLabel"
      :aria-label="coreLabel"
      @click="onCoreMenu"
    >
      <span class="dot" :class="isCoreHealthy ? 'ok' : 'err'" />
      <span>{{ coreLabel }}</span>
    </button>
    <div v-else class="status-item">
      <span class="dot" :class="isCoreHealthy ? 'ok' : 'err'" />
      <span>{{ coreLabel }}</span>
    </div>
  </footer>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useCoreHeartbeat } from '../composables/useCoreHeartbeat'
import { t } from '../i18n'
import { getCoreBaseUrl, getDesktopBridge } from '../runtime'
import { useSessionsStore } from '../stores/sessions'

const bridge = getDesktopBridge()
const isElectron = bridge?.isElectron ?? false
const { isCoreHealthy } = useCoreHeartbeat()
const store = useSessionsStore()

const runningCount = computed(() =>
  store.sessionList.filter((s) => s.meta.status === 'running').length
)
const pendingCount = computed(() =>
  store.sessionList.filter((s) => Boolean(s.pendingPermission)).length
)
const errorCount = computed(() =>
  store.sessionList.filter((s) => s.meta.status === 'error').length
)

const apiHost = computed(() => {
  try {
    const url = new URL(getCoreBaseUrl())
    return url.host || url.origin
  } catch {
    return getCoreBaseUrl()
  }
})

const coreLabel = computed(() => (
  isCoreHealthy.value ? t('statusBar.coreOnline') : t('statusBar.coreOffline')
))

function onCoreMenu() {
  if (!bridge?.showCoreMenu) return
  void bridge.showCoreMenu({
    killCore: t('titleBar.killCore'),
    runCore: t('titleBar.runCore'),
  })
}
</script>

<style scoped>
.status-bar {
  position: relative;
  flex-shrink: 0;
  height: 28px;
  display: flex;
  align-items: center;
  gap: 18px;
  padding: 0 14px;
  background: var(--panel);
  border-top: 1px solid var(--rule-soft);
  font-family: var(--mono);
  font-size: 10.5px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--ink-4);
}

.status-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}

.status-item.warn { color: var(--warn); }
.status-item.err  { color: var(--err); }

.status-item.api {
  margin-left: auto;
}

.core-button {
  border: none;
  background: transparent;
  color: inherit;
  font: inherit;
  letter-spacing: inherit;
  text-transform: inherit;
  padding: 2px 4px;
  cursor: pointer;
  transition: background .15s, color .15s;
}

.core-button:hover {
  background: var(--bg-2);
  color: var(--ink);
}

.dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--ink-4);
  display: inline-block;
  flex-shrink: 0;
}

.dot.ok   { background: var(--ok); }
.dot.warn { background: var(--warn); }
.dot.err  { background: var(--err); }
</style>
