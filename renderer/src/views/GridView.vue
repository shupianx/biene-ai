<template>
  <div class="grid-view">
    <div class="grid-body">
      <div v-if="store.sessionList.length > 0" class="grid-actions">
        <button class="new-btn" @click="showNewModal = true">+ New Agent</button>
      </div>
      <div v-if="store.sessionList.length === 0" class="empty-grid">
        <div class="empty-icon">⚡</div>
        <p>No agents yet</p>
        <button class="new-agent-btn" @click="onNew">Create one</button>
      </div>
      <div v-else class="agent-grid">
        <SessionCard
          v-for="s in store.sessionList"
          :key="s.meta.id"
          :session="s"
          @select="onOpenSession(s.meta.id)"
          @settings="onOpenSettings(s.meta.id)"
          @delete="deletingSessionId = s.meta.id"
        />
      </div>
    </div>

    <NewAgentModal
      v-if="showNewModal"
      :default-name="nextAgentName"
      :existing-names="sessionNames"
      @close="showNewModal = false"
      @create="onCreateAgent"
    />
    <SessionSettingsModal
      v-if="editingSession"
      :key="editingSession.meta.id"
      :name="editingSession.meta.name"
      :existing-names="editableOtherNames"
      :permissions="editingSession.meta.permissions"
      :profile="editingSession.meta.profile"
      @close="editingSessionId = null"
      @save="onSaveSettings"
    />
    <ConfirmModal
      v-if="deletingSession"
      title="Delete Agent"
      :message="`Delete ${deletingSession.meta.name}? Its workspace and stored history will be removed from disk.`"
      confirm-label="Delete"
      @cancel="deletingSessionId = null"
      @confirm="onConfirmDelete"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onBeforeUnmount, onMounted } from 'vue'
import type { AgentProfile, SessionPermissions } from '../api/http'
import { useSessionsStore } from '../stores/sessions'
import { useAgentNavigation } from '../composables/useAgentNavigation'
import { nextDefaultAgentName } from '../utils/agentNames'
import SessionCard from '../components/SessionCard.vue'
import NewAgentModal from '../components/NewAgentModal.vue'
import SessionSettingsModal from '../components/SessionSettingsModal.vue'
import ConfirmModal from '../components/ConfirmModal.vue'

const store = useSessionsStore()
const { openAgent } = useAgentNavigation()
const showNewModal = ref(false)
const editingSessionId = ref<string | null>(null)
const deletingSessionId = ref<string | null>(null)

function syncSessions() {
  void store.refresh(false)
}

function onVisibilityChange() {
  if (document.visibilityState !== 'visible') return
  syncSessions()
}

onMounted(() => {
  void store.init(false)
  window.addEventListener('focus', syncSessions)
  document.addEventListener('visibilitychange', onVisibilityChange)
})

onBeforeUnmount(() => {
  window.removeEventListener('focus', syncSessions)
  document.removeEventListener('visibilitychange', onVisibilityChange)
})

const editingSession = computed(() =>
  editingSessionId.value ? store.sessions[editingSessionId.value] ?? null : null
)

const deletingSession = computed(() =>
  deletingSessionId.value ? store.sessions[deletingSessionId.value] ?? null : null
)

const sessionNames = computed(() =>
  store.sessionList.map((session) => session.meta.name)
)

const nextAgentName = computed(() =>
  nextDefaultAgentName(sessionNames.value)
)

const editableOtherNames = computed(() =>
  editingSession.value
    ? store.sessionList
        .filter((session) => session.meta.id !== editingSession.value?.meta.id)
        .map((session) => session.meta.name)
    : []
)

function onNew() {
  showNewModal.value = true
}

async function onCreateAgent(name: string, permissions: SessionPermissions, profile: AgentProfile) {
  const meta = await store.create(name, permissions, profile)
  showNewModal.value = false
  await openAgent(meta.id)
}

async function onSaveSettings(name: string, permissions: SessionPermissions, profile: AgentProfile) {
  if (!editingSession.value) return
  await store.update(editingSession.value.meta.id, { name, permissions, profile })
  editingSessionId.value = null
}

async function onOpenSession(id: string) {
  await openAgent(id)
}

async function onOpenSettings(id: string) {
  await store.refresh(false)
  if (!store.sessions[id]) return
  editingSessionId.value = id
}

async function onConfirmDelete() {
  if (!deletingSession.value) return
  const id = deletingSession.value.meta.id
  deletingSessionId.value = null
  await store.remove(id)
}
</script>

<style scoped>
.grid-view {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #f3f4f6;
}

.new-btn {
  padding: 7px 16px; border-radius: 8px; border: none;
  background: #6366f1; color: #fff; font-size: 13px; font-weight: 600;
  cursor: pointer; transition: background .15s;
}
.new-btn:hover { background: #4f46e5; }

.grid-body {
  flex: 1; overflow-y: auto; padding: 24px;
}

.grid-actions {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 16px;
}

.agent-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 16px;
}

.empty-grid {
  display: flex; flex-direction: column; align-items: center;
  justify-content: center; height: 100%; padding-top: 120px;
  color: #9ca3af; gap: 10px; text-align: center;
}
.empty-icon { font-size: 40px; }
.empty-grid p { font-size: 15px; margin: 0; }
.new-agent-btn {
  margin-top: 4px; padding: 10px 24px; border-radius: 10px; border: none;
  background: #6366f1; color: #fff; font-size: 14px; font-weight: 600;
  cursor: pointer; transition: background .15s;
}
.new-agent-btn:hover { background: #4f46e5; }
</style>
