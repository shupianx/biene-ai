<template>
  <div class="grid-view">
    <!-- Toolbar: title + counter + search + refresh + new -->
    <div class="grid-toolbar">
      <h1 class="grid-title">{{ t('grid.title') }}</h1>
      <div class="counter">
        {{ pad(filtered.length) }} / {{ pad(store.sessionList.length) }}
      </div>

      <div class="search-box">
        <svg class="search-icon" viewBox="0 0 24 24" aria-hidden="true" v-html="searchIconBody" />
        <input
          v-model="search"
          class="search-input"
          :placeholder="t('grid.searchPlaceholder')"
        />
      </div>

      <button
        class="icon-btn"
        type="button"
        :title="t('grid.refreshStatus')"
        :aria-label="t('grid.refreshStatus')"
        @click="onRefresh"
      >
        <svg class="icon-btn-icon" viewBox="0 0 24 24" aria-hidden="true" :class="{ spinning: refreshing }" v-html="refreshIconBody" />
      </button>

      <button class="skills-btn" type="button" @click="onToggleSkills">
        <span>{{ t('grid.skills') }}</span>
      </button>

      <button class="new-btn" @click="showNewModal = true">
        <svg class="new-btn-icon" viewBox="0 0 24 24" aria-hidden="true" v-html="addIconBody" />
        <span>{{ t('grid.newAgent') }}</span>
      </button>
    </div>

    <!-- Body -->
    <div class="grid-body">
      <div v-if="store.sessionList.length === 0" class="empty-grid">
        <MaterialSymbolsRobot2Outline class="empty-icon empty-icon-muted" aria-hidden="true" />
        <p class="empty-title">{{ t('grid.noAgentsYet') }}</p>
        <button class="new-agent-btn" @click="onNew">
          <svg class="new-btn-icon" viewBox="0 0 24 24" aria-hidden="true" v-html="addIconBody" />
          <span>{{ t('grid.createOne') }}</span>
        </button>
      </div>
      <div v-else-if="filtered.length === 0" class="empty-grid">
        <MaterialSymbolsRobot2Outline class="empty-icon empty-icon-muted" aria-hidden="true" />
        <p class="empty-title">{{ t('grid.noAgentsYet') }}</p>
        <button class="new-agent-btn" @click="onNew">
          <svg class="new-btn-icon" viewBox="0 0 24 24" aria-hidden="true" v-html="addIconBody" />
          <span>{{ t('grid.newAgent') }}</span>
        </button>
      </div>
      <div v-else class="agent-grid">
        <SessionCard
          v-for="s in filtered"
          :key="s.meta.id"
          :session="s"
          @select="onOpenSession(s.meta.id)"
          @open-folder="onOpenFolder(s.meta.work_dir)"
          @settings="onOpenSettings(s.meta.id)"
          @delete="deletingSessionId = s.meta.id"
        />
      </div>
    </div>

    <WelcomeModal
      v-if="showWelcome"
      @done="onWelcomeDone"
    />
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
      :model-name="editingSession.meta.model_name"
      :existing-names="editableOtherNames"
      :permissions="editingSession.meta.permissions"
      :profile="editingSession.meta.profile"
      :session-id="editingSession.meta.id"
      :installed-skill-ids="editingSession.meta.installed_skill_ids ?? []"
      :active-skill-names="editingSession.meta.active_skills ?? []"
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
    <FloatingPanel
      v-if="skillsPanelOpen"
      :title="t('skillsBrowser.title')"
      :close-label="t('common.close')"
      :initial-width="640"
      :initial-height="560"
      :min-width="360"
      :min-height="320"
      storage-key="biene.skillsPanel"
      @close="skillsPanelOpen = false"
    >
      <template #chrome>
        <code class="skills-panel-path" :title="skillsRootPath">{{ skillsRootPath }}</code>
        <button
          class="skills-panel-folder-btn"
          type="button"
          :title="t('grid.openFolderMenu')"
          :aria-label="t('grid.openFolderMenu')"
          @click="onOpenSkillsFolder"
        >
          <FolderOpenIcon class="skills-panel-folder-icon" />
        </button>
      </template>
      <SkillsBrowser embedded @root-change="onSkillsRootChange" />
    </FloatingPanel>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onBeforeUnmount, onMounted } from 'vue'
import FolderOpenIcon from '~icons/material-symbols/folder-open-outline'
import MaterialSymbolsRobot2Outline from '~icons/material-symbols/robot-2-outline'
import type { AgentProfile, SessionPermissions } from '../api/http'
import { fetchConfig } from '../api/http'
import { connectSessionListWS } from '../api/ws'
import { t } from '../i18n'
import { getDesktopBridge } from '../runtime'
import { useSessionsStore } from '../stores/sessions'
import { useAgentNavigation } from '../composables/useAgentNavigation'
import { nextDefaultAgentName } from '../utils/agentNames'
import SessionCard from '../components/session/SessionCard.vue'
import NewAgentModal from '../components/session/NewAgentModal.vue'
import SessionSettingsModal from '../components/session/SessionSettingsModal.vue'
import ConfirmModal from '../components/ui/ConfirmModal.vue'
import FloatingPanel from '../components/ui/FloatingPanel.vue'
import SkillsBrowser from '../components/skills/SkillsBrowser.vue'
import WelcomeModal from '../components/app/WelcomeModal.vue'

const store = useSessionsStore()
const { openAgent } = useAgentNavigation()
const skillsPanelOpen = ref(false)
const skillsRootPath = ref('~/.biene/skills')

const addIconBody = '<path fill="currentColor" d="M11 13H6q-.425 0-.712-.288T5 12t.288-.712T6 11h5V6q0-.425.288-.712T12 5t.713.288T13 6v5h5q.425 0 .713.288T19 12t-.288.713T18 13h-5v5q0 .425-.288.713T12 19t-.712-.288T11 18z"/>'
const refreshIconBody = '<path fill="currentColor" d="M12 20q-3.35 0-5.675-2.325T4 12q0-3.35 2.325-5.675T12 4q1.725 0 3.275.7t2.7 1.95V4h2v7h-7v-2h4.2q-.85-1.175-2.175-1.837T12 6Q9.5 6 7.75 7.75T6 12t1.75 4.25T12 18q1.925 0 3.475-1.1T17.6 14h2.1q-.7 2.7-2.85 4.35T12 20"/>'
const searchIconBody = '<path fill="currentColor" d="M9.5 3A6.5 6.5 0 0 1 16 9.5c0 1.61-.59 3.09-1.57 4.23l.27.28h.8l5 5l-1.5 1.5l-5-5v-.79l-.28-.27A6.52 6.52 0 0 1 9.5 16A6.5 6.5 0 0 1 3 9.5A6.5 6.5 0 0 1 9.5 3m0 2C7 5 5 7 5 9.5S7 14 9.5 14S14 12 14 9.5S12 5 9.5 5"/>'

const showNewModal = ref(false)
const showWelcome = ref(false)
const editingSessionId = ref<string | null>(null)
const deletingSessionId = ref<string | null>(null)
const search = ref('')
const refreshing = ref(false)
let disconnectListWS: (() => void) | null = null

async function checkWelcome() {
  try {
    const cfg = await fetchConfig()
    if (!cfg.model_list || cfg.model_list.length === 0) {
      showWelcome.value = true
    }
  } catch (error) {
    console.error('Failed to check config for welcome flow:', error)
  }
}

function onWelcomeDone() {
  showWelcome.value = false
}

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  return store.sessionList.filter((s) => {
    if (!q) return true
    const hay = `${s.meta.name} ${s.meta.model_name} ${s.meta.id} ${s.meta.work_dir}`.toLowerCase()
    return hay.includes(q)
  })
})

function pad(n: number) {
  return n.toString().padStart(2, '0')
}

function syncSessions() {
  void store.refresh(false, false)
}

function onVisibilityChange() {
  if (document.visibilityState !== 'visible') return
  syncSessions()
}

onMounted(() => {
  void store.init(false, false)
  void checkWelcome()
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
    onSessionProcessState(data) {
      store.applyListProcessState(data)
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

function onToggleSkills() {
  skillsPanelOpen.value = !skillsPanelOpen.value
}

function onSkillsRootChange(next: string) {
  if (next) skillsRootPath.value = next
}

async function onOpenSkillsFolder() {
  const bridge = getDesktopBridge()
  if (!bridge?.openPath) return
  try {
    await bridge.openPath(skillsRootPath.value)
  } catch (error) {
    console.error('Failed to open skills folder:', error)
  }
}

async function onRefresh() {
  refreshing.value = true
  try {
    await store.refresh(false, false)
  } finally {
    setTimeout(() => { refreshing.value = false }, 350)
  }
}

async function onCreateAgent(name: string, modelID: string, permissions: SessionPermissions, profile: AgentProfile) {
  const meta = await store.create(name, modelID, permissions, profile, { subscribe: false })
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

async function onOpenFolder(workDir: string) {
  const bridge = getDesktopBridge()
  if (!bridge?.openPath) return

  try {
    await bridge.openPath(workDir)
  } catch (error) {
    console.error('Failed to open agent folder:', error)
  }
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
  background: var(--bg);
  color: var(--ink);
}

/* ── Toolbar ──────────────────────────────────────────────────────── */
.grid-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 20px;
  border-bottom: 1px solid var(--rule-soft);
  flex: 0 0 auto;
  background: var(--bg);
}

.grid-title {
  margin: 0;
  font-family: var(--sans);
  font-size: 20px;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: var(--ink);
  white-space: nowrap;
}

.counter {
  font-family: var(--mono);
  font-size: 11px;
  color: var(--ink-4);
  letter-spacing: 0.08em;
  padding: 2px 6px;
  border: 1px solid var(--rule-soft);
  white-space: nowrap;
}

/* Search */
.search-box {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 10px;
  width: 200px;
  min-width: 120px;
  flex: 0 1 200px;
  border: 1px solid var(--rule-soft);
  background: var(--panel-2);
}

.search-icon {
  width: 12px;
  height: 12px;
  flex: 0 0 auto;
  color: var(--ink-4);
}

.search-input {
  border: none;
  outline: none;
  background: transparent;
  flex: 1;
  min-width: 0;
  width: 100%;
  font-size: 12px;
  font-family: var(--mono);
  color: var(--ink-2);
}

.search-input::placeholder {
  color: var(--ink-4);
}

/* Icon button (refresh) */
.icon-btn {
  width: 32px;
  height: 32px;
  display: grid;
  place-items: center;
  background: var(--panel-2);
  color: var(--ink-2);
  border: 1px solid var(--rule-soft);
  cursor: pointer;
  flex: 0 0 auto;
  transition: background .12s, border-color .12s, color .12s;
}

.icon-btn:hover {
  background: var(--panel);
  border-color: var(--rule);
  color: var(--ink);
}

.icon-btn-icon {
  width: 14px;
  height: 14px;
}

.icon-btn-icon.spinning {
  animation: bieneSpin 700ms linear;
}

/* New button (primary) */
.new-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 7px 14px;
  font-family: var(--mono);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.08em;
  background: var(--ink);
  color: var(--bg);
  border: 1px solid var(--ink);
  cursor: pointer;
  white-space: nowrap;
  flex: 0 0 auto;
  transition: background .12s, color .12s;
}

.skills-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 7px 14px;
  font-family: var(--mono);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.08em;
  background: var(--panel-2);
  color: var(--ink-2);
  border: 1px solid var(--rule-soft);
  cursor: pointer;
  white-space: nowrap;
  flex: 0 0 auto;
  transition: background .12s, color .12s, border-color .12s;
}

.skills-btn:hover {
  background: var(--panel);
  border-color: var(--rule);
  color: var(--ink);
}

.new-btn:hover {
  background: var(--ink-2);
  border-color: var(--ink-2);
}

.new-btn-icon {
  width: 13px;
  height: 13px;
}

/* ── Body ────────────────────────────────────────────────────────── */
.grid-body {
  flex: 1;
  overflow: auto;
  padding: 20px;
  position: relative;
  scrollbar-width: thin;
  scrollbar-color: var(--rule-soft) transparent;
}

.grid-body::-webkit-scrollbar {
  width: 10px;
}

.grid-body::-webkit-scrollbar-track {
  background: transparent;
}

.grid-body::-webkit-scrollbar-thumb {
  background: var(--rule-soft);
  border: 2px solid var(--bg);
}

.grid-body::-webkit-scrollbar-thumb:hover {
  background: var(--rule);
}

.agent-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 16px;
}

.empty-grid {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
}

.empty-grid > * {
  grid-row: 1;
  grid-column: 1;
}

.empty-grid {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  text-align: center;
  color: var(--ink-4);
}

.empty-icon {
  font-size: 40px;
  color: var(--ink-3);
}

.empty-icon-muted {
  color: color-mix(in srgb, var(--ink-4) 72%, transparent);
}

.empty-frame {
  width: 48px;
  height: 48px;
  display: grid;
  place-items: center;
  border: 1px solid var(--rule);
  color: var(--ink-3);
  font-size: 22px;
  margin-bottom: 4px;
}

.empty-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--ink-2);
}

.empty-hint {
  margin: 0;
  font-family: var(--mono);
  font-size: 11px;
  color: var(--ink-4);
  letter-spacing: 0.04em;
}

.new-agent-btn {
  margin-top: 8px;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  font-family: var(--mono);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.08em;
  background: var(--ink);
  color: var(--bg);
  border: 1px solid var(--ink);
  cursor: pointer;
  transition: background .12s;
}

.new-agent-btn:hover {
  background: var(--ink-2);
  border-color: var(--ink-2);
}

.skills-panel-path {
  min-width: 0;
  flex: 1;
  font-family: var(--mono);
  font-size: 11px;
  line-height: 1.4;
  color: var(--ink-3);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  user-select: none;
}

.skills-panel-folder-btn {
  width: 22px;
  height: 22px;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: none;
  background: transparent;
  color: var(--ink-3);
  cursor: pointer;
  transition: background .14s, color .14s;
}

.skills-panel-folder-btn:hover {
  background: var(--bg-2);
  color: var(--ink);
}

.skills-panel-folder-icon {
  width: 14px;
  height: 14px;
}
</style>
