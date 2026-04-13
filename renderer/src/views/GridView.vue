<template>
  <div class="grid-view">
    <div class="grid-body">
      <div v-if="store.sessionList.length > 0" class="grid-actions">
        <button class="new-btn" @click="showNewModal = true">
          <svg class="new-btn-icon" viewBox="0 0 24 24" aria-hidden="true" v-html="addRoundedIconBody" />
          <span>{{ t('grid.newAgent') }}</span>
        </button>
      </div>
      <div v-if="store.sessionList.length === 0" class="empty-grid">
        <div class="empty-icon">⚡</div>
        <p>{{ t('grid.noAgentsYet') }}</p>
        <button class="new-agent-btn" @click="onNew">{{ t('grid.createOne') }}</button>
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
      :title="t('grid.deleteAgentTitle')"
      :message="t('grid.deleteAgentMessage', { name: deletingSession.meta.name })"
      :confirm-label="t('common.delete')"
      @cancel="deletingSessionId = null"
      @confirm="onConfirmDelete"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onBeforeUnmount, onMounted } from 'vue'
import type { AgentProfile, SessionPermissions } from '../api/http'
import { t } from '../i18n'
import { useSessionsStore } from '../stores/sessions'
import { useAgentNavigation } from '../composables/useAgentNavigation'
import { nextDefaultAgentName } from '../utils/agentNames'
import SessionCard from '../components/SessionCard.vue'
import NewAgentModal from '../components/NewAgentModal.vue'
import SessionSettingsModal from '../components/SessionSettingsModal.vue'
import ConfirmModal from '../components/ConfirmModal.vue'

const store = useSessionsStore()
const { openAgent } = useAgentNavigation()
const addRoundedIconBody = '<path fill="currentColor" d="M11 13H6q-.425 0-.712-.288T5 12t.288-.712T6 11h5V6q0-.425.288-.712T12 5t.713.288T13 6v5h5q.425 0 .713.288T19 12t-.288.713T18 13h-5v5q0 .425-.288.713T12 19t-.712-.288T11 18z"/>' // sourced from @iconify-json/material-symbols
const showNewModal = ref(false)
const editingSessionId = ref<string | null>(null)
const deletingSessionId = ref<string | null>(null)
let refreshTimer: number | null = null

function syncSessions() {
  void store.refresh(false, false)
}

function onVisibilityChange() {
  if (document.visibilityState !== 'visible') return
  syncSessions()
}

onMounted(() => {
  void store.init(false, false)
  refreshTimer = window.setInterval(() => {
    if (document.visibilityState !== 'visible') return
    syncSessions()
  }, 2000)
  window.addEventListener('focus', syncSessions)
  document.addEventListener('visibilitychange', onVisibilityChange)
})

onBeforeUnmount(() => {
  if (refreshTimer !== null) {
    window.clearInterval(refreshTimer)
    refreshTimer = null
  }
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
  const meta = await store.create(name, permissions, profile, { subscribe: false })
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
  await store.refresh(false, false)
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
  background: var(--app-bg);
}

.new-btn,
.new-agent-btn {
  border: 1px solid var(--accent-warm-border);
  background: var(--accent-warm-bg);
  color: var(--accent-warm-text);
  cursor: pointer;
  transition: background .15s, border-color .15s, color .15s;
}

.new-btn:hover,
.new-agent-btn:hover {
  background: var(--accent-warm-bg-hover);
  border-color: var(--accent-warm-bg-active);
}

.new-btn:active,
.new-agent-btn:active {
  background: var(--accent-warm-bg-active);
  border-color: var(--accent-warm-border-strong);
}

.new-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 14px 8px 10px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: bold;
}

.new-btn-icon {
  width: 17px;
  height: 17px;
  flex-shrink: 0;
}

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
  margin-top: 4px;
  padding: 10px 24px;
  border-radius: 8px;
  font-size: 14px;
  font-weight: bold;
}

.new-btn:focus-visible,
.new-agent-btn:focus-visible {
  outline: 2px solid var(--accent-warm-ring);
  outline-offset: 2px;
}
</style>
