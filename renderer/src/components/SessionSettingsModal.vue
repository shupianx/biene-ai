<template>
  <BaseModal title="Agent Settings" max-width="460px" :z-index="220" @close="emit('close')">
    <label class="field">
      <span class="label">Agent name</span>
      <input
        ref="nameInput"
        v-model="name"
        class="input"
        :class="{ invalid: nameConflict }"
        @keydown.enter="submit"
      />
      <span v-if="nameConflict" class="error-text">Agent name already exists.</span>
    </label>

    <div class="field">
      <span class="label">Profile</span>
      <div class="profile-grid">
        <label class="field">
          <span class="sub-label">Domain</span>
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
          <span class="sub-label">Style</span>
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
        <span class="sub-label">Custom instructions</span>
        <AutoGrowTextarea
          v-model="profile.custom_instructions"
          class="textarea"
          placeholder="Optional agent-specific instructions"
        />
      </label>
    </div>

    <div class="field">
      <span class="label">Tool permissions</span>
      <div class="permission-list">
        <div
          v-for="permission in permissionDefinitions"
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
      <button class="btn-cancel" @click="emit('close')">Cancel</button>
      <button class="btn-save" :disabled="nameConflict" @click="submit">Save</button>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { AgentProfile, SessionPermissions } from '../api/http'
import AutoGrowTextarea from './AutoGrowTextarea.vue'
import BaseModal from './BaseModal.vue'
import ToggleSwitch from './ToggleSwitch.vue'
import { isAgentNameTaken } from '../utils/agentNames'
import { clonePermissions, permissionDefinitions } from '../utils/permissions'
import { cloneProfile, domainOptions, styleOptions } from '../utils/profile'

const props = defineProps<{
  name: string
  existingNames: string[]
  permissions: SessionPermissions
  profile: AgentProfile
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'save', name: string, permissions: SessionPermissions, profile: AgentProfile): void
}>()

const name = ref(props.name)
const permissions = ref<SessionPermissions>(clonePermissions(props.permissions))
const profile = ref<AgentProfile>(cloneProfile(props.profile))
const nameInput = ref<HTMLInputElement | null>(null)

onMounted(() => {
  nameInput.value?.focus()
  nameInput.value?.select()
})

const selectedDomainDescription = computed(() =>
  domainOptions.find((option) => option.value === profile.value.domain)?.description ?? ''
)

const selectedStyleDescription = computed(() =>
  styleOptions.find((option) => option.value === profile.value.style)?.description ?? ''
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
.field { display: flex; flex-direction: column; gap: 6px; }
.label { font-size: 13px; font-weight: bold; color: #374151; }

.input {
  height: 38px; padding: 0 12px;
  border: 1.5px solid #e5e7eb; border-radius: 8px;
  font-size: 14px; color: #111827; outline: none;
  transition: border-color .15s, box-shadow .15s;
}
.input:focus {
  border-color: var(--accent-soft-bg-active);
  box-shadow: 0 0 0 3px var(--accent-soft-focus);
}
.input.invalid { border-color: #ef4444; }
.sub-label {
  font-size: 12px;
  font-weight: bold;
  color: #6b7280;
}
.profile-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}
.select,
.textarea {
  width: 100%;
  border: 1.5px solid #e5e7eb;
  border-radius: 8px;
  font-size: 14px;
  color: #111827;
  outline: none;
  background: #fff;
  transition: border-color .15s, box-shadow .15s;
}
.select {
  height: 38px;
  padding: 0 12px;
}
.textarea {
  padding: 10px 12px;
  font-family: inherit;
  line-height: 1.45;
}
.select:focus,
.textarea:focus {
  border-color: var(--accent-soft-bg-active);
  box-shadow: 0 0 0 3px var(--accent-soft-focus);
}
.field-hint {
  font-size: 12px;
  color: #9ca3af;
  line-height: 1.4;
}
.error-text {
  font-size: 12px;
  color: #dc2626;
}

.permission-list {
  display: flex;
  flex-direction: column;
}
.permission-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 0;
}
.permission-item + .permission-item {
  border-top: 1px solid #e5e7eb;
}
.permission-copy {
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.permission-name {
  font-size: 13px;
  font-weight: bold;
  color: #111827;
}
.permission-desc {
  font-size: 12px;
  color: #6b7280;
}
.btn-cancel {
  padding: 8px 16px; border-radius: 8px; border: 1.5px solid #e5e7eb;
  background: #fff; color: #374151; font-size: 13px; font-weight: bold;
  cursor: pointer; transition: background .15s;
}
.btn-cancel:hover { background: #f9fafb; }
.btn-save {
  padding: 8px 20px; border-radius: 8px; border: 1px solid var(--accent-soft-border);
  background: var(--accent-soft-bg); color: var(--accent-soft-text); font-size: 13px; font-weight: bold;
  cursor: pointer; transition: background .15s, border-color .15s, color .15s;
}
.btn-save:disabled { opacity: .5; cursor: not-allowed; }
.btn-save:hover:not(:disabled) {
  background: var(--accent-soft-bg-hover);
  border-color: var(--accent-soft-bg-active);
}
.btn-save:active:not(:disabled) {
  background: var(--accent-soft-bg-active);
  border-color: var(--accent-soft-border-strong);
}
.btn-save:focus-visible {
  outline: 2px solid var(--accent-soft-ring);
  outline-offset: 2px;
}
</style>
