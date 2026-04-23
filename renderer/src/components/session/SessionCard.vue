<template>
  <div
    class="card"
    :class="[statusTone, { 'drop-target': isDropTarget, installing: installingSkill }]"
    @mouseenter="hover = true"
    @mouseleave="hover = false"
    @click="emit('select')"
    @dragenter="onDragEnter"
    @dragover="onDragOver"
    @dragleave="onDragLeave"
    @drop="onDrop"
  >
    <!-- Top strip: name + menu -->
    <div class="top-strip">
      <div class="title-row">
        <div class="name" :title="session.meta.name">{{ session.meta.name }}</div>
        <div class="title-path" :title="session.meta.work_dir">
          <MaterialSymbolsFolderSharp class="path-icon" aria-hidden="true" />
          <span class="path-text">{{ shortDir }}</span>
        </div>
      </div>
      <PopupMenu
        :items="menuItems"
        :visible="hover"
        @select="onMenuSelect"
      />
    </div>

    <!-- Body -->
    <div class="body">
      <div v-if="pendingPermLine" class="warn-line">
        <CiOctagonWarning class="warn-icon" aria-hidden="true" />
        <span>{{ pendingPermLine }}</span>
      </div>

      <div v-if="installFlash" class="install-flash" :class="installFlash.tone">
        {{ installFlash.message }}
      </div>

      <div class="meta-row">
        <div class="status-tag" :class="statusTone">
          <span class="status-dot" />
          <span>{{ statusLabel }}</span>
        </div>
        <div v-if="session.meta.model_name" class="model-tag" :title="session.meta.model_name">
          {{ session.meta.model_name }}
        </div>
        <div class="updated">{{ updatedAt }}</div>
      </div>
    </div>

    <ConfirmModal
      v-if="conflict"
      :title="t('skillsBrowser.installConflictTitle')"
      :message="t('skillsBrowser.installConflictMessage', { name: conflict.skillName })"
      :confirm-label="t('skillsBrowser.installConflictOverwrite')"
      @cancel="onConflictCancel"
      @confirm="onConflictConfirm"
    />

    <!-- Footer: profile chips + permission chips -->
    <div class="footer">
      <div v-if="domainLabel" class="chip">{{ domainLabel }}</div>
      <div v-if="styleLabel" class="chip">{{ styleLabel }}</div>
      <div class="perm-chips">
        <div class="perm-chip" :class="{ on: session.meta.permissions.execute }">EXEC</div>
        <div class="perm-chip" :class="{ on: session.meta.permissions.write }">WRITE</div>
        <div class="perm-chip" :class="{ on: session.meta.permissions.send_to_agent }">SEND</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import CiOctagonWarning from '~icons/ci/octagon-warning'
import MaterialSymbolsFolderSharp from '~icons/material-symbols/folder-sharp'
import type { AgentSession } from '../../stores/sessions'
import { installSkillToSession } from '../../api/http'
import { t } from '../../i18n'
import { getSessionStatusLabel, getSessionStatusTone } from '../../utils/sessionStatus'
import { findDomainOption, findStyleOption } from '../../utils/profile'
import { getPermissionLabel } from '../../utils/permissions'
import { formatMessageTime } from '../../utils/messageTime'
import PopupMenu, { type PopupMenuEntry } from '../ui/PopupMenu.vue'
import ConfirmModal from '../ui/ConfirmModal.vue'

const SKILL_MIME = 'application/biene-skill'

const props = defineProps<{ session: AgentSession }>()
const emit = defineEmits<{
  (e: 'select'): void
  (e: 'open-folder'): void
  (e: 'settings'): void
  (e: 'delete'): void
}>()

const hover = ref(false)

const menuItems = computed<PopupMenuEntry[]>(() => [
  { key: 'open-folder', label: t('grid.openFolderMenu') },
  { key: 'settings', label: t('common.settings') },
  { separator: true },
  { key: 'delete', label: t('common.delete'), danger: true },
])

const statusTone = computed(() => getSessionStatusTone(props.session))
const statusLabel = computed(() => getSessionStatusLabel(statusTone.value))

const shortDir = computed(() => {
  const trimmed = props.session.meta.work_dir.replace(/[\\/]+$/, '')
  if (!trimmed) return props.session.meta.work_dir
  const parts = trimmed.split(/[\\/]/).filter(Boolean)
  if (parts.length === 0) return trimmed
  return parts[parts.length - 1]
})

const pendingPermLine = computed(() => {
  const perm = props.session.pendingPermission
  if (!perm) return ''
  const label = getPermissionLabel(perm.permission)
  const tool = perm.tool_name ?? ''
  return tool ? `${label} · ${tool}` : label
})

const updatedAt = computed(() => formatMessageTime(props.session.meta.last_active))

const domainLabel = computed(() =>
  findDomainOption(props.session.meta.profile.domain)?.label ?? ''
)
const styleLabel = computed(() =>
  findStyleOption(props.session.meta.profile.style)?.label ?? ''
)

function onMenuSelect(key: string) {
  if (key === 'open-folder') emit('open-folder')
  else if (key === 'settings') emit('settings')
  else if (key === 'delete') emit('delete')
}

const dragDepth = ref(0)
const installingSkill = ref(false)
const installFlash = ref<{ tone: 'ok' | 'err'; message: string } | null>(null)
const conflict = ref<{ skillId: string; skillName: string } | null>(null)
let flashTimer: number | null = null

const isDropTarget = computed(() => dragDepth.value > 0)

function hasSkillPayload(event: DragEvent) {
  const types = event.dataTransfer?.types
  if (!types) return false
  for (let i = 0; i < types.length; i += 1) {
    if (types[i] === SKILL_MIME) return true
  }
  return false
}

function onDragEnter(event: DragEvent) {
  if (!hasSkillPayload(event)) return
  event.preventDefault()
  dragDepth.value += 1
}

function onDragOver(event: DragEvent) {
  if (!hasSkillPayload(event)) return
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
}

function onDragLeave(event: DragEvent) {
  if (!hasSkillPayload(event)) return
  dragDepth.value = Math.max(0, dragDepth.value - 1)
}

function scheduleFlashClear() {
  if (flashTimer != null) window.clearTimeout(flashTimer)
  flashTimer = window.setTimeout(() => {
    installFlash.value = null
    flashTimer = null
  }, 2400)
}

async function performInstall(skillId: string) {
  installingSkill.value = true
  installFlash.value = null
  try {
    const result = await installSkillToSession(props.session.meta.id, skillId)
    installFlash.value = {
      tone: 'ok',
      message: t('skillsBrowser.installSuccess', { name: result.skill_name }),
    }
    scheduleFlashClear()
  } catch (error) {
    installFlash.value = {
      tone: 'err',
      message: error instanceof Error ? error.message : String(error),
    }
    scheduleFlashClear()
  } finally {
    installingSkill.value = false
  }
}

async function onDrop(event: DragEvent) {
  if (!hasSkillPayload(event)) return
  event.preventDefault()
  dragDepth.value = 0
  const skillId = event.dataTransfer?.getData(SKILL_MIME)?.trim() ?? ''
  if (!skillId || installingSkill.value) return
  const installed = props.session.meta.installed_skill_ids ?? []
  if (installed.includes(skillId)) {
    const label = event.dataTransfer?.getData('text/plain')?.trim() || skillId
    conflict.value = { skillId, skillName: label }
    return
  }
  await performInstall(skillId)
}

function onConflictCancel() {
  conflict.value = null
}

async function onConflictConfirm() {
  const pending = conflict.value
  conflict.value = null
  if (!pending) return
  await performInstall(pending.skillId)
}
</script>

<style scoped>
.card {
  position: relative;
  background: var(--panel-2);
  border: 1px solid var(--rule);
  display: flex;
  flex-direction: column;
  min-height: 180px;
  cursor: pointer;
  user-select: none;
  transition: transform 180ms cubic-bezier(.2,.7,.2,1),
              box-shadow 180ms cubic-bezier(.2,.7,.2,1);
}

.card:hover {
  transform: translate(-2px, -2px);
  box-shadow: 4px 4px 0 0 var(--rule);
}

.card.approval { border-color: var(--warn); }
.card.error    { border-color: var(--err); }

.card.drop-target {
  border-color: transparent;
  transform: translate(-2px, -2px);
}

.card.drop-target::before {
  content: '';
  position: absolute;
  top: 4px;
  left: 4px;
  right: -4px;
  bottom: -4px;
  pointer-events: none;
  background: linear-gradient(135deg,
    #d7b7b2, #c7a7c4, #ada4d3, #72c0d0, #8bc59a, #d7b37d, #d7b7b2);
  clip-path: polygon(
    100% 0,
    100% 100%,
    0 100%,
    0 calc(100% - 4px),
    calc(100% - 4px) calc(100% - 4px),
    calc(100% - 4px) 0
  );
  animation: bieneSkillDropShimmer 1s linear infinite;
}

.card.drop-target::after {
  content: '';
  position: absolute;
  inset: -1px;
  pointer-events: none;
  background: linear-gradient(135deg,
    #d7b7b2, #c7a7c4, #ada4d3, #72c0d0, #8bc59a, #d7b37d, #d7b7b2);
  padding: 1px;
  -webkit-mask:
    linear-gradient(#000 0 0) content-box,
    linear-gradient(#000 0 0);
  -webkit-mask-composite: xor;
          mask-composite: exclude;
  animation: bieneSkillDropShimmer 1s linear infinite;
}

@keyframes bieneSkillDropShimmer {
  from { filter: hue-rotate(0deg); }
  to   { filter: hue-rotate(360deg); }
}

.card.installing {
  cursor: progress;
}

.install-flash {
  font-family: var(--mono);
  font-size: 10.5px;
  letter-spacing: 0.08em;
  padding: 4px 8px;
  border: 1px solid currentColor;
}

.install-flash.ok {
  color: var(--ok);
  background: color-mix(in srgb, var(--ok) 10%, transparent);
}

.install-flash.err {
  color: var(--err);
  background: color-mix(in srgb, var(--err) 10%, transparent);
}

/* Top strip */
.top-strip {
  display: grid;
  grid-template-columns: 1fr auto;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border-bottom: 1px dashed var(--rule-soft);
}

.title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.name {
  font-family: var(--sans);
  font-size: 15px;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: var(--ink);
  line-height: 1.15;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
  flex: 0 1 auto;
  max-width: 52%;
}

/* Body */
.body {
  padding: 14px 14px 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  flex: 1;
}

.title-path {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
  min-width: 0;
  flex: 1 1 auto;
  font-family: var(--mono);
  font-size: 11px;
  color: var(--ink-4);
}

.path-icon {
  width: 13px;
  height: 13px;
  flex: 0 0 auto;
}

.path-text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
  text-align: right;
}

.warn-line {
  display: flex;
  align-items: center;
  gap: 6px;
  font-family: var(--mono);
  font-size: 11.5px;
  color: var(--warn);
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.warn-icon {
  width: 13px;
  height: 13px;
  flex: 0 0 auto;
}

.meta-row {
  margin-top: auto;
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 2px 7px;
  border: 1px solid currentColor;
  font-family: var(--mono);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  white-space: nowrap;
  color: var(--ink-4);
}

.status-tag.running  { color: var(--ok); }
.status-tag.approval { color: var(--warn); }
.status-tag.error    { color: var(--err); }

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.status-tag.running .status-dot,
.status-tag.approval .status-dot {
  animation: bienePulse 1.6s ease-in-out infinite;
}

.model-tag {
  min-width: 0;
  max-width: 150px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--mono);
  font-size: 10px;
  letter-spacing: 0.08em;
  color: var(--ink-2);
  border: 1px solid var(--rule-soft);
  background: var(--panel);
  padding: 2px 6px;
}

.updated {
  margin-left: auto;
  font-family: var(--mono);
  font-size: 10px;
  letter-spacing: 0.06em;
  color: var(--ink-4);
  white-space: nowrap;
}

/* Footer */
.footer {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  border-top: 1px solid var(--rule);
  background: var(--panel);
  flex-wrap: wrap;
}

.chip {
  font-family: var(--mono);
  font-size: 10px;
  letter-spacing: 0.08em;
  padding: 2px 6px;
  color: var(--ink-3);
  border: 1px solid var(--rule-softer);
  background: var(--panel-2);
}

.perm-chips {
  margin-left: auto;
  display: flex;
  gap: 4px;
}

.perm-chip {
  font-family: var(--mono);
  font-size: 9.5px;
  font-weight: 600;
  letter-spacing: 0.1em;
  padding: 2px 5px;
  color: var(--ink-4);
  background: transparent;
  border: 1px solid var(--rule-softer);
  text-decoration: line-through;
  opacity: 0.6;
}

.perm-chip.on {
  color: var(--ink-2);
  background: var(--bg-2);
  border-color: var(--rule-soft);
  text-decoration: none;
  opacity: 1;
}
</style>
