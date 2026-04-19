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

    <template #footer>
      <button class="btn-cancel" @click="emit('close')">{{ t('common.cancel') }}</button>
      <button class="btn-save" :disabled="nameConflict" @click="submit">{{ t('common.save') }}</button>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { AgentProfile, SessionPermissions } from '../api/http'
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

onMounted(() => {
  nameInput.value?.focus()
  nameInput.value?.select()
})

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
