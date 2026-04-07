<template>
  <div class="app-shell">
    <AppTitleBar v-if="showTitleBar" />
    <main class="app-content">
      <RouterView />
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import AppTitleBar from './components/AppTitleBar.vue'
import { getDesktopBridge } from './runtime'

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
  background: #f8fafc;
}

.app-content {
  flex: 1;
  min-height: 0;
}
</style>
