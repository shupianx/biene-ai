<template>
  <div v-if="loading" class="agent-page state-page">
    <p>Loading agent…</p>
  </div>
  <div v-else-if="!session" class="agent-page state-page">
    <p>Agent not found.</p>
    <button class="back-btn" @click="closeAgentView">Back</button>
  </div>
  <AgentChatView
    v-else
    :session="session"
    @close="closeAgentView"
  />
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import AgentChatView from '../components/AgentChatView.vue'
import { useAgentNavigation } from '../composables/useAgentNavigation'
import { useSessionsStore } from '../stores/sessions'

const route = useRoute()
const store = useSessionsStore()
const { closeAgentView } = useAgentNavigation()
const loading = ref(true)

const sessionId = computed(() =>
  typeof route.params.id === 'string' ? route.params.id : ''
)

const session = computed(() =>
  sessionId.value ? store.sessions[sessionId.value] ?? null : null
)

function syncSessionMeta() {
  void store.refresh(false)
}

function onVisibilityChange() {
  if (document.visibilityState !== 'visible') return
  syncSessionMeta()
}

watch(
  sessionId,
  async (id) => {
    loading.value = true

    if (!id) {
      loading.value = false
      return
    }

    await store.refresh(false)
    await store.ensureSession(id, true)
    loading.value = false
  },
  { immediate: true },
)

watch(
  () => session.value?.meta.name,
  (name) => {
    document.title = name ? `${name} · Biene` : 'Biene'
  },
  { immediate: true },
)

onMounted(() => {
  window.addEventListener('focus', syncSessionMeta)
  document.addEventListener('visibilitychange', onVisibilityChange)
})

onBeforeUnmount(() => {
  window.removeEventListener('focus', syncSessionMeta)
  document.removeEventListener('visibilitychange', onVisibilityChange)
})
</script>

<style scoped>
.agent-page {
  height: 100%;
  background: var(--app-bg);
}

.state-page {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: #6b7280;
}

.state-page p {
  margin: 0;
  font-size: 14px;
}

.back-btn {
  padding: 8px 16px;
  border-radius: 8px;
  border: 1.5px solid #e5e7eb;
  background: #fff;
  color: #374151;
  font-size: 13px;
  font-weight: bold;
  cursor: pointer;
}

.back-btn:hover {
  background: #f9fafb;
}
</style>
