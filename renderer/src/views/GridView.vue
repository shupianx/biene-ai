<template>
  <div class="grid-view">
    <div class="grid-body">
      <div v-if="store.sessionList.length > 0" class="grid-actions">
        <button
          class="icon-btn"
          type="button"
          :title="t('grid.refreshStatus')"
          :aria-label="t('grid.refreshStatus')"
          @click="onRefresh"
        >
          <svg class="icon-btn-icon" viewBox="0 0 24 24" aria-hidden="true" v-html="refreshIconBody" />
        </button>
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
import { connectSessionListWS } from '../api/ws'
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
const refreshIconBody = '<path fill="currentColor" d="M12 20q-3.35 0-5.675-2.325T4 12q0-3.35 2.325-5.675T12 4q1.725 0 3.275.7t2.7 1.95V4h2v7h-7v-2h4.2q-.85-1.175-2.175-1.837T12 6Q9.5 6 7.75 7.75T6 12t1.75 4.25T12 18q1.925 0 3.475-1.1T17.6 14h2.1q-.7 2.7-2.85 4.35T12 20"/>' // sourced from @iconify-json/material-symbols
const showNewModal = ref(false)
const editingSessionId = ref<string | null>(null)
const deletingSessionId = ref<string | null>(null)
let disconnectListWS: (() => void) | null = null

function syncSessions() {
  void store.refresh(false, false)
}

function onVisibilityChange() {
  if (document.visibilityState !== 'visible') return
  syncSessions()
}

onMounted(() => {
  void store.init(false, false)
  disconnectListWS = connectSessionListWS({
    onOpen() {
      void store.refresh(false, false)
    },
    onSessionCreated({ session }) {
      void store.upsertSessionMeta(session, false)
    },
    onSessionUpdated({ session }) {
      void store.upsertSessionMeta(session, false)
    },
    onSessionDeleted({ id }) {
      store.removeSessionLocal(id)
    },
    onReconnect() {
      void store.refresh(false, false)
    },
  })
  window.addEventListener('focus', syncSessions)
  document.addEventListener('visibilitychange', onVisibilityChange)
})

onBeforeUnmount(() => {
  disconnectListWS?.()
  disconnectListWS = null
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

async function onRefresh() {
  await store.refresh(false, false)
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

.icon-btn {
  width: 36px;
  height: 36px;
  padding: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  flex-shrink: 0;
  border: 1px solid rgba(226, 232, 240, 0.96);
  background: rgba(255, 255, 255, 0.96);
  color: #64748b;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.05);
  cursor: pointer;
  transition: background .15s, border-color .15s, color .15s, box-shadow .15s;
}

.icon-btn:hover {
  background: #ffffff;
  border-color: #cbd5e1;
  color: #334155;
}

.icon-btn:active {
  background: #f8fafc;
  border-color: #94a3b8;
}

.icon-btn-icon {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
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
  align-items: center;
  gap: 8px;
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

.icon-btn:focus-visible {
  outline: 2px solid rgba(148, 163, 184, 0.3);
  outline-offset: 2px;
}

.new-btn:focus-visible,
.new-agent-btn:focus-visible {
  outline: 2px solid var(--accent-warm-ring);
  outline-offset: 2px;
}
</style>
