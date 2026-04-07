<template>
  <BaseModal title="New Agent" @close="emit('close')">
    <label class="field">
      <span class="label">Agent name</span>
      <input
        ref="nameInput"
        v-model="name"
        class="input"
        :class="{ invalid: nameConflict }"
        :placeholder="defaultName"
        @keydown.enter="submit"
      />
      <span v-if="nameConflict" class="error-text">Agent name already exists.</span>
    </label>

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
      <p class="hint">Closed means the agent will ask before using that permission group.</p>
    </div>

    <section class="advanced-section">
      <button
        type="button"
        class="advanced-toggle"
        :aria-expanded="advancedOpen"
        @click="advancedOpen = !advancedOpen"
      >
        <span class="label">Advanced settings</span>
        <span class="advanced-arrow" :class="{ open: advancedOpen }" aria-hidden="true">⌄</span>
      </button>

      <div v-if="advancedOpen" class="advanced-panel">
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
      </div>
    </section>

    <template #footer>
      <button class="btn-cancel" @click="emit('close')">Cancel</button>
      <button class="btn-create" :disabled="nameConflict" @click="submit">Create</button>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import type { AgentProfile, SessionPermissions } from '../api/http'
import AutoGrowTextarea from './AutoGrowTextarea.vue'
import BaseModal from './BaseModal.vue'
import ToggleSwitch from './ToggleSwitch.vue'
import { isAgentNameTaken } from '../utils/agentNames'
import { defaultPermissions, permissionDefinitions } from '../utils/permissions'
import { defaultProfile, domainOptions, styleOptions } from '../utils/profile'

const props = defineProps<{
  defaultName: string
  existingNames: string[]
}>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'create', name: string, permissions: SessionPermissions, profile: AgentProfile): void
}>()

const name = ref('')
const permissions = ref<SessionPermissions>(defaultPermissions())
const profile = ref<AgentProfile>(defaultProfile())
const advancedOpen = ref(false)
const nameInput = ref<HTMLInputElement | null>(null)

onMounted(() => nameInput.value?.focus())

const selectedDomainDescription = computed(() =>
  domainOptions.find((option) => option.value === profile.value.domain)?.description ?? ''
)

const selectedStyleDescription = computed(() =>
  styleOptions.find((option) => option.value === profile.value.style)?.description ?? ''
)

const effectiveName = computed(() =>
  name.value.trim() || props.defaultName
)

const nameConflict = computed(() =>
  isAgentNameTaken(effectiveName.value, props.existingNames)
)

function submit() {
  if (nameConflict.value) return
  emit(
    'create',
    effectiveName.value,
    { ...permissions.value },
    { ...profile.value, custom_instructions: (profile.value.custom_instructions ?? '').trim() },
  )
}
</script>

<style scoped>
.field { display: flex; flex-direction: column; gap: 6px; }
.label { font-size: 13px; font-weight: 600; color: #374151; }

.input {
  height: 38px; padding: 0 12px;
  border: 1.5px solid #e5e7eb; border-radius: 8px;
  font-size: 14px; color: #111827; outline: none;
  transition: border-color .15s;
}
.input:focus { border-color: #6366f1; }
.input.invalid { border-color: #ef4444; }
.sub-label {
  font-size: 12px;
  font-weight: 600;
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
  transition: border-color .15s;
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
  border-color: #6366f1;
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

.advanced-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.advanced-toggle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border: none;
  background: transparent;
  padding: 0;
  text-align: left;
  cursor: pointer;
}
.advanced-toggle:hover {
  color: #111827;
}
.advanced-arrow {
  font-size: 16px;
  line-height: 1;
  color: #6b7280;
  transform: rotate(0deg);
  transition: transform .15s ease, color .15s ease;
  user-select: none;
}
.advanced-arrow.open {
  transform: rotate(180deg);
  color: #374151;
}
.advanced-panel {
  padding-top: 2px;
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
  font-weight: 600;
  color: #111827;
}
.permission-desc {
  font-size: 12px;
  color: #6b7280;
}
.hint { font-size: 12px; color: #9ca3af; margin: 0; }
.btn-cancel {
  padding: 8px 16px; border-radius: 8px; border: 1.5px solid #e5e7eb;
  background: #fff; color: #374151; font-size: 13px; font-weight: 600;
  cursor: pointer; transition: background .15s;
}
.btn-cancel:hover { background: #f9fafb; }
.btn-create {
  padding: 8px 20px; border-radius: 8px; border: none;
  background: #6366f1; color: #fff; font-size: 13px; font-weight: 600;
  cursor: pointer; transition: background .15s;
}
.btn-create:hover:not(:disabled) { background: #4f46e5; }
.btn-create:disabled { opacity: .5; cursor: not-allowed; }
</style>
