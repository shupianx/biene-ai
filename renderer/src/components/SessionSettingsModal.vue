<template>
  <BaseModal :title="t('modal.agentSettingsTitle')" max-width="460px" :z-index="220" @close="emit('close')">
    <label class="field">
      <span class="label">{{ t('agentName.label') }}</span>
      <input
        ref="nameInput"
        v-model="name"
        class="input"
        :class="{ invalid: nameConflict }"
        @keydown.enter="submit"
      />
      <span v-if="nameConflict" class="error-text">{{ t('agentName.exists') }}</span>
    </label>

    <label class="field">
      <span class="label">{{ t('modal.agentModel') }}</span>
      <input :value="modelName" class="input" disabled />
      <span class="field-hint">{{ t('modal.agentModelLockedHint') }}</span>
    </label>

    <div class="field">
      <span class="label">{{ t('modal.profile') }}</span>
      <div class="profile-grid">
        <label class="field">
          <span class="sub-label">{{ t('modal.domain') }}</span>
          <select v-model="profile.domain" class="select">
            <option
              v-for="option in domainOptions"
              :key="option.value"
              :value="option.value"
            >
              {{ option.label }}
            </option>
          </select>
          <span class="field-hint">{{ selectedDomainDescription }}</span>
        </label>

        <label class="field">
          <span class="sub-label">{{ t('modal.style') }}</span>
          <select v-model="profile.style" class="select">
            <option
              v-for="option in styleOptions"
              :key="option.value"
              :value="option.value"
            >
              {{ option.label }}
            </option>
          </select>
          <span class="field-hint">{{ selectedStyleDescription }}</span>
        </label>
      </div>

      <label class="field">
        <span class="sub-label">{{ t('modal.customInstructions') }}</span>
        <AutoGrowTextarea
          v-model="profile.custom_instructions"
          class="textarea"
          :placeholder="t('modal.customInstructionsPlaceholder')"
        />
      </label>
    </div>

    <div class="field">
      <span class="label">{{ t('modal.toolPermissions') }}</span>
      <div class="permission-list">
        <div
          v-for="permission in permissionOptions"
          :key="permission.key"
          class="permission-item"
        >
          <div class="permission-copy">
            <span class="permission-name">{{ permission.label }}</span>
            <span class="permission-desc">{{ permission.description }}</span>
          </div>
          <ToggleSwitch
            v-model="permissions[permission.key]"
            :label="permission.label"
          />
        </div>
      </div>
    </div>

    <div class="field">
      <span class="label">{{ t('modal.installedSkills') }}</span>
      <div v-if="installedSkillRows.length === 0" class="installed-skills-empty">
        {{ t('modal.installedSkillsEmpty') }}
      </div>
      <div v-else class="installed-skills-list" aria-label="Installed skills">
        <div
          v-for="row in installedSkillRows"
          :key="row.id"
          class="skill-list-item"
          :class="{ 'skill-missing': row.missing }"
          :title="row.missing
            ? t('modal.skillSourceMissing')
            : row.description || (catalogLoading ? t('modal.installedSkillsLoading') : '')"
        >
          <span class="skill-list-name">{{ row.name }}</span>
          <span v-if="row.active" class="skill-active-badge">
            {{ t('modal.skillActiveBadge') }}
          </span>
          <div class="skill-list-actions">
            <div class="skill-delete-shell">
              <button
                class="skill-action skill-action-delete"
                type="button"
                :title="t('modal.skillRemove')"
                :aria-label="t('modal.skillRemove')"
                :disabled="removingSkillId === row.id"
                @click="onToggleRemovePopover(row.id)"
              >
                <DeleteIcon class="skill-action-icon" />
              </button>
              <div v-if="removeConfirmId === row.id" class="skill-delete-popover">
                <button
                  class="skill-delete-popover-btn is-danger"
                  type="button"
                  :disabled="removingSkillId === row.id"
                  @click="onConfirmRemove(row.id)"
                >
                  {{ t('skillsBrowser.confirmDeleteLabel') }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <button class="btn-cancel" @click="emit('close')">{{ t('common.cancel') }}</button>
      <button class="btn-save" :disabled="nameConflict" @click="submit">{{ t('common.save') }}</button>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import DeleteIcon from '~icons/material-symbols/delete-forever-outline-sharp'
import type { AgentProfile, SessionPermissions, SkillCatalogEntry } from '../api/http'
import { listSkills, uninstallSkillFromSession } from '../api/http'
import AutoGrowTextarea from './AutoGrowTextarea.vue'
import BaseModal from './BaseModal.vue'
import ToggleSwitch from './ToggleSwitch.vue'
import { t } from '../i18n'
import { isAgentNameTaken } from '../utils/agentNames'
import { clonePermissions, listPermissionDefinitions } from '../utils/permissions'
import {
  cloneProfile,
  findDomainOption,
  findStyleOption,
  listDomainOptions,
  listStyleOptions,
} from '../utils/profile'

const props = defineProps<{
  name: string
  modelName: string
  existingNames: string[]
  permissions: SessionPermissions
  profile: AgentProfile
  sessionId: string
  installedSkillIds: string[]
  activeSkillNames: string[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'save', name: string, permissions: SessionPermissions, profile: AgentProfile): void
}>()

const name = ref(props.name)
const modelName = computed(() => props.modelName)
const permissions = ref<SessionPermissions>(clonePermissions(props.permissions))
const profile = ref<AgentProfile>(cloneProfile(props.profile))
const nameInput = ref<HTMLInputElement | null>(null)

const catalogById = ref<Record<string, SkillCatalogEntry>>({})
const catalogLoading = ref(true)
const removingSkillId = ref<string | null>(null)
const removeConfirmId = ref<string>('')

onMounted(async () => {
  nameInput.value?.focus()
  nameInput.value?.select()
  document.addEventListener('pointerdown', onDocumentPointerDown)
  try {
    const catalog = await listSkills()
    const map: Record<string, SkillCatalogEntry> = {}
    for (const entry of catalog.skills) {
      map[entry.id] = entry
    }
    catalogById.value = map
  } catch {
    // Catalog lookup is best-effort; rows fall back to raw ids.
  } finally {
    catalogLoading.value = false
  }
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onDocumentPointerDown)
})

interface InstalledSkillRow {
  id: string
  name: string
  description: string
  missing: boolean
  active: boolean
}

const installedSkillRows = computed<InstalledSkillRow[]>(() =>
  props.installedSkillIds.map((id) => {
    const entry = catalogById.value[id]
    const displayName = entry?.name ?? id
    return {
      id,
      name: displayName,
      description: entry?.description ?? '',
      missing: !entry,
      active: props.activeSkillNames.includes(displayName),
    }
  })
)

function onToggleRemovePopover(id: string) {
  removeConfirmId.value = removeConfirmId.value === id ? '' : id
}

async function onConfirmRemove(id: string) {
  if (removingSkillId.value) return
  removingSkillId.value = id
  try {
    await uninstallSkillFromSession(props.sessionId, id)
    removeConfirmId.value = ''
  } catch {
    // Swallow errors silently; the SSE meta update will refresh state regardless.
  } finally {
    removingSkillId.value = null
  }
}

function onDocumentPointerDown(event: PointerEvent) {
  const target = event.target
  if (!(target instanceof Element)) {
    removeConfirmId.value = ''
    return
  }
  if (target.closest('.skill-delete-shell')) return
  removeConfirmId.value = ''
}

const domainOptions = computed(() =>
  listDomainOptions(profile.value.domain)
)

const styleOptions = computed(() =>
  listStyleOptions(profile.value.style)
)

const permissionOptions = computed(() =>
  listPermissionDefinitions()
)

const selectedDomainDescription = computed(() =>
  findDomainOption(profile.value.domain)?.description ?? ''
)

const selectedStyleDescription = computed(() =>
  findStyleOption(profile.value.style)?.description ?? ''
)

const effectiveName = computed(() =>
  name.value.trim() || props.name
)

const nameConflict = computed(() =>
  isAgentNameTaken(effectiveName.value, props.existingNames)
)

function submit() {
  if (nameConflict.value) return
  emit(
    'save',
    effectiveName.value,
    { ...permissions.value },
    { ...profile.value, custom_instructions: (profile.value.custom_instructions ?? '').trim() },
  )
}
</script>

<style scoped>
.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.label {
  font-family: var(--mono);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--ink-3);
}

.sub-label {
  font-family: var(--mono);
  font-size: 9.5px;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--ink-4);
}

.input,
.select,
.textarea {
  width: 100%;
  border: 1px solid var(--rule-soft);
  background: var(--panel-2);
  color: var(--ink);
  font-size: 13px;
  font-family: var(--sans);
  outline: none;
  transition: border-color .12s, box-shadow .12s;
}

.input {
  height: 34px;
  padding: 0 10px;
}

.select {
  height: 34px;
  padding: 0 10px;
}

.textarea {
  padding: 8px 10px;
  line-height: 1.45;
}

.input:focus,
.select:focus,
.textarea:focus {
  border-color: var(--ink);
  box-shadow: 2px 2px 0 0 var(--rule);
}

.input:disabled,
.select:disabled,
.textarea:disabled {
  color: var(--ink-3);
  background: color-mix(in srgb, var(--panel-2) 76%, var(--bg));
  cursor: not-allowed;
  opacity: 1;
}

.input.invalid {
  border-color: var(--err);
}

.profile-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.field-hint {
  font-size: 11.5px;
  color: var(--ink-4);
  line-height: 1.4;
}

.error-text {
  font-family: var(--mono);
  font-size: 11px;
  color: var(--err);
  letter-spacing: 0.04em;
}

.permission-list {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--rule-softer);
  background: var(--panel);
}

.permission-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 10px 12px;
}

.permission-item + .permission-item {
  border-top: 1px dashed var(--rule-softer);
}

.permission-copy {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.permission-name {
  font-family: var(--mono);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  color: var(--ink-2);
}

.permission-desc {
  font-size: 11.5px;
  color: var(--ink-4);
  line-height: 1.4;
}

.installed-skills-empty {
  padding: 10px 12px;
  border: 1px dashed var(--rule-softer);
  background: var(--panel);
  font-size: 12px;
  color: var(--ink-4);
  line-height: 1.5;
}

.installed-skills-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px;
  max-height: 220px;
  overflow: auto;
  border: 1px solid var(--rule-softer);
  background: var(--panel);
}

.skill-list-item {
  min-height: 42px;
  padding: 6px 8px 6px 10px;
  display: flex;
  align-items: center;
  gap: 8px;
  border: 1px solid var(--rule-softer);
  background: color-mix(in srgb, var(--panel) 90%, var(--bg));
  transition: background .14s, border-color .14s;
}

.skill-list-item:hover {
  background: color-mix(in srgb, var(--panel-2) 92%, var(--bg));
  border-color: var(--rule-soft);
}

.skill-list-item.skill-missing {
  border-style: dashed;
  opacity: 0.72;
}

.skill-active-badge {
  flex-shrink: 0;
  padding: 2px 6px;
  border: 1px solid color-mix(in srgb, var(--ok) 60%, var(--rule-soft));
  color: color-mix(in srgb, var(--ok) 80%, var(--ink));
  background: color-mix(in srgb, var(--ok) 12%, transparent);
  font-family: var(--mono);
  font-size: 9.5px;
  font-weight: 600;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  line-height: 1.4;
}

.skill-list-name {
  min-width: 0;
  font-family: var(--mono);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.05em;
  line-height: 1.45;
  color: var(--ink-2);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.skill-missing .skill-list-name {
  color: var(--ink-4);
  font-style: italic;
}

.skill-list-actions {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
  margin-left: auto;
}

.skill-delete-shell {
  position: relative;
  display: inline-flex;
}

.skill-action {
  width: 24px;
  height: 24px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: none;
  background: transparent;
  color: var(--ink-4);
  cursor: pointer;
  transition: background .14s, color .14s, opacity .14s;
}

.skill-action:hover:not(:disabled) {
  color: var(--ink);
}

.skill-action:disabled {
  opacity: 0.55;
  cursor: progress;
}

.skill-action-icon {
  width: 16px;
  height: 16px;
}

.skill-action-delete:hover:not(:disabled) {
  background: color-mix(in srgb, var(--err) 14%, transparent);
  color: color-mix(in srgb, var(--err) 82%, var(--ink));
}

.skill-delete-popover {
  position: absolute;
  top: 50%;
  right: calc(100% + 6px);
  transform: translateY(-50%);
  z-index: 2;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px;
  min-width: max-content;
  border: 1px solid var(--rule-soft);
  background: var(--panel);
  box-shadow: 0 8px 18px rgba(0, 0, 0, 0.08);
}

.skill-delete-popover-btn {
  height: 22px;
  min-width: max-content;
  padding: 0 10px;
  border: none;
  background: transparent;
  color: var(--ink-3);
  cursor: pointer;
  font-family: var(--mono);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  transition: background .14s, color .14s, opacity .14s;
}

.skill-delete-popover-btn.is-danger:hover:not(:disabled) {
  background: color-mix(in srgb, var(--err) 14%, transparent);
  color: color-mix(in srgb, var(--err) 82%, var(--ink));
}

.skill-delete-popover-btn:disabled {
  opacity: 0.55;
  cursor: progress;
}

.btn-cancel,
.btn-save {
  height: 30px;
  padding: 0 14px;
  border: 1px solid var(--rule);
  background: var(--panel-2);
  color: var(--ink-2);
  cursor: pointer;
  font-family: var(--mono);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  transition: transform .12s, box-shadow .12s;
}

.btn-cancel:hover,
.btn-save:hover:not(:disabled) {
  transform: translate(-1px, -1px);
  box-shadow: 2px 2px 0 0 var(--rule);
}

.btn-cancel:active,
.btn-save:active:not(:disabled) {
  transform: translate(0, 0);
  box-shadow: none;
}

.btn-save {
  background: var(--ink);
  border-color: var(--ink);
  color: var(--panel-2);
}

.btn-save:disabled {
  opacity: .4;
  cursor: not-allowed;
}

.btn-save:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
</style>
