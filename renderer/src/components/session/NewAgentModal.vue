<template>
  <BaseModal :title="t('modal.newAgentTitle')" @close="emit('close')">
    <div class="field">
      <span class="label">{{ t('agentName.label') }}</span>
      <div class="name-row">
        <button
          ref="avatarTriggerRef"
          type="button"
          class="avatar-trigger"
          :class="{ open: avatarPickerOpen }"
          :aria-label="t('modal.avatarPicker')"
          :title="t('modal.avatarPicker')"
          @click="toggleAvatarPicker"
        >
          <AgentAvatar :index="selectedAvatar" :size="32" />
        </button>
        <input
          ref="nameInput"
          v-model="name"
          class="input name-input"
          :class="{ invalid: nameConflict }"
          :placeholder="defaultName"
          @keydown.enter="onNameEnter"
          @compositionstart="onCompositionStart"
          @compositionend="onCompositionEnd"
        />
        <div
          v-if="avatarPickerOpen"
          ref="avatarPickerRef"
          class="avatar-picker"
          role="listbox"
          :aria-label="t('modal.avatarPicker')"
        >
          <button
            v-for="idx in avatarChoices"
            :key="idx"
            type="button"
            class="avatar-option"
            :class="{ selected: idx === selectedAvatar }"
            :aria-selected="idx === selectedAvatar"
            @click="pickAvatar(idx)"
          >
            <AgentAvatar :index="idx" :size="36" />
          </button>
        </div>
      </div>
      <span v-if="nameConflict" class="error-text">{{ t('agentName.exists') }}</span>
    </div>

    <div class="field">
      <span class="label">{{ t('modal.agentModel') }}</span>
      <PopupMenu :items="modelMenuItems" @select="onModelSelect">
        <template #trigger="{ open, toggle }">
          <button
            type="button"
            class="select select-trigger"
            :class="{ open }"
            :disabled="configLoading || modelOptions.length === 0"
            @click="toggle"
          >
            <span class="select-label">{{ currentModelLabel }}</span>
            <ArrowDropDownIcon class="chevron" aria-hidden="true" />
          </button>
        </template>
      </PopupMenu>
      <span v-if="configLoading" class="field-hint">{{ t('modal.modelLoading') }}</span>
      <span v-else-if="configError" class="error-text">{{ configError }}</span>
      <span v-else class="field-hint">{{ selectedModelSummary || t('modal.agentModelHint') }}</span>
    </div>

    <div class="field">
      <span class="label">{{ t('modal.profile') }}</span>
      <div class="profile-grid">
        <div class="field">
          <span class="sub-label">{{ t('modal.domain') }}</span>
          <PopupMenu :items="domainMenuItems" @select="onDomainSelect">
            <template #trigger="{ open, toggle }">
              <button
                type="button"
                class="select select-trigger"
                :class="{ open }"
                @click="toggle"
              >
                <span class="select-label">{{ currentDomainLabel }}</span>
                <ArrowDropDownIcon class="chevron" aria-hidden="true" />
              </button>
            </template>
          </PopupMenu>
          <span class="field-hint">{{ selectedDomainDescription }}</span>
        </div>

        <div class="field">
          <span class="sub-label">{{ t('modal.style') }}</span>
          <PopupMenu :items="styleMenuItems" @select="onStyleSelect">
            <template #trigger="{ open, toggle }">
              <button
                type="button"
                class="select select-trigger"
                :class="{ open }"
                @click="toggle"
              >
                <span class="select-label">{{ currentStyleLabel }}</span>
                <ArrowDropDownIcon class="chevron" aria-hidden="true" />
              </button>
            </template>
          </PopupMenu>
          <span class="field-hint">{{ selectedStyleDescription }}</span>
        </div>
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

    <section class="advanced-section">
      <button
        type="button"
        class="advanced-toggle"
        :aria-expanded="advancedOpen"
        @click="advancedOpen = !advancedOpen"
      >
        <span class="label">{{ t('modal.advancedSettings') }}</span>
        <span class="advanced-arrow" :class="{ open: advancedOpen }" aria-hidden="true">⌄</span>
      </button>

      <div v-if="advancedOpen" class="advanced-panel">
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
          <p class="hint">{{ t('modal.toolPermissionsHint') }}</p>
        </div>
      </div>
    </section>

    <template #footer>
      <AppButton variant="neutral" @click="emit('close')">{{ t('common.cancel') }}</AppButton>
      <AppButton variant="primary" :disabled="submitDisabled" @click="submit">{{ t('common.create') }}</AppButton>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import { computed, ref, onBeforeUnmount, onMounted } from 'vue'
import { fetchConfig, type AgentProfile, type ConfigModelEntry, type SessionPermissions } from '../../api/http'
import ArrowDropDownIcon from '~icons/material-symbols/arrow-drop-down'
import AgentAvatar from '../ui/AgentAvatar.vue'
import AppButton from '../ui/AppButton.vue'
import AutoGrowTextarea from '../ui/AutoGrowTextarea.vue'
import BaseModal from '../ui/BaseModal.vue'
import PopupMenu, { type PopupMenuEntry } from '../ui/PopupMenu.vue'
import ToggleSwitch from '../ui/ToggleSwitch.vue'
import { t } from '../../i18n'
import { isAgentNameTaken } from '../../utils/agentNames'
import { defaultPermissions, listPermissionDefinitions } from '../../utils/permissions'
import {
  defaultProfile,
  findDomainOption,
  findStyleOption,
  listDomainOptions,
  listStyleOptions,
} from '../../utils/profile'

const props = defineProps<{
  defaultName: string
  existingNames: string[]
}>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'create', name: string, modelID: string, permissions: SessionPermissions, profile: AgentProfile, avatar: string): void
}>()

// Sprite layout — keep in sync with AgentAvatar.vue. 5 cols × 4 rows = 20.
const AVATAR_TOTAL = 20
const avatarChoices = Array.from({ length: AVATAR_TOTAL }, (_, i) => i)

function randomAvatarIndex() {
  return Math.floor(Math.random() * AVATAR_TOTAL)
}

const name = ref('')
const selectedModelID = ref('')
const permissions = ref<SessionPermissions>(defaultPermissions())
const profile = ref<AgentProfile>(defaultProfile())
const advancedOpen = ref(false)
const nameInput = ref<HTMLInputElement | null>(null)
const configLoading = ref(false)
const configError = ref('')
const modelOptions = ref<ConfigModelEntry[]>([])
// Pre-roll a random avatar so the modal opens with a concrete face — the
// user only has to interact with the picker if they want to change it.
const selectedAvatar = ref(randomAvatarIndex())
const avatarPickerOpen = ref(false)
const avatarTriggerRef = ref<HTMLButtonElement | null>(null)
const avatarPickerRef = ref<HTMLDivElement | null>(null)

onMounted(() => {
  nameInput.value?.focus()
  void loadModelOptions()
  document.addEventListener('mousedown', onDocumentMouseDown)
  document.addEventListener('keydown', onDocumentKeyDown)
})

onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDocumentMouseDown)
  document.removeEventListener('keydown', onDocumentKeyDown)
})

function toggleAvatarPicker() {
  avatarPickerOpen.value = !avatarPickerOpen.value
}

function pickAvatar(idx: number) {
  selectedAvatar.value = idx
  avatarPickerOpen.value = false
}

// Close the picker when the user clicks outside it. Listening on
// `mousedown` rather than `click` lets us swallow the press before the
// trigger button toggles itself back open on the same gesture.
function onDocumentMouseDown(event: MouseEvent) {
  if (!avatarPickerOpen.value) return
  const target = event.target as Node | null
  if (!target) return
  if (avatarTriggerRef.value?.contains(target)) return
  if (avatarPickerRef.value?.contains(target)) return
  avatarPickerOpen.value = false
}

function onDocumentKeyDown(event: KeyboardEvent) {
  if (event.key === 'Escape' && avatarPickerOpen.value) {
    avatarPickerOpen.value = false
  }
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

const domainMenuItems = computed<PopupMenuEntry[]>(() =>
  domainOptions.value.map((option) => ({
    key: option.value,
    label: option.label,
    selected: option.value === profile.value.domain,
  }))
)

const styleMenuItems = computed<PopupMenuEntry[]>(() =>
  styleOptions.value.map((option) => ({
    key: option.value,
    label: option.label,
    selected: option.value === profile.value.style,
  }))
)

const currentDomainLabel = computed(
  () => findDomainOption(profile.value.domain)?.label ?? ''
)

const currentStyleLabel = computed(
  () => findStyleOption(profile.value.style)?.label ?? ''
)

function onDomainSelect(key: string) {
  profile.value.domain = key as AgentProfile['domain']
}

function onStyleSelect(key: string) {
  profile.value.style = key as AgentProfile['style']
}

const effectiveName = computed(() =>
  name.value.trim() || props.defaultName
)

const nameConflict = computed(() =>
  isAgentNameTaken(effectiveName.value, props.existingNames)
)

const selectedModel = computed(() =>
  modelOptions.value.find((entry) => entry.id === selectedModelID.value) ?? null
)

const modelMenuItems = computed<PopupMenuEntry[]>(() =>
  modelOptions.value.map((entry) => ({
    key: entry.id,
    label: entry.name,
    selected: entry.id === selectedModelID.value,
  }))
)

const currentModelLabel = computed(() => {
  if (configLoading.value) return t('modal.modelLoading')
  return selectedModel.value?.name ?? ''
})

function onModelSelect(key: string) {
  selectedModelID.value = key
}

const selectedModelSummary = computed(() => {
  if (!selectedModel.value) return ''
  const providerType = selectedModel.value.provider === 'openai_compatible'
    ? t('modal.providerTypes.openaiCompatible')
    : t('modal.providerTypes.anthropic')
  return `${selectedModel.value.model} · ${providerType}`
})

const submitDisabled = computed(() =>
  nameConflict.value || configLoading.value || !selectedModelID.value || Boolean(configError.value)
)

async function loadModelOptions() {
  configLoading.value = true
  configError.value = ''
  try {
    const config = await fetchConfig()
    modelOptions.value = config.model_list
    selectedModelID.value = config.default_model || config.model_list[0]?.id || ''
  } catch (error) {
    configError.value = error instanceof Error ? error.message : String(error)
  } finally {
    configLoading.value = false
  }
}

const isComposing = ref(false)
let compositionLockedUntil = 0

function onCompositionStart() {
  isComposing.value = true
}

function onCompositionEnd() {
  isComposing.value = false
  compositionLockedUntil = Date.now() + 30
}

function onNameEnter(event: KeyboardEvent) {
  if (isComposing.value || event.isComposing || event.keyCode === 229) return
  if (Date.now() < compositionLockedUntil) return
  submit()
}

function submit() {
  if (submitDisabled.value) return
  emit(
    'create',
    effectiveName.value,
    selectedModelID.value,
    { ...permissions.value },
    { ...profile.value, custom_instructions: (profile.value.custom_instructions ?? '').trim() },
    String(selectedAvatar.value),
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

.select-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  cursor: pointer;
  text-align: left;
  font-family: inherit;
}

.select-trigger:disabled {
  cursor: not-allowed;
  color: var(--ink-3);
  background: color-mix(in srgb, var(--panel-2) 76%, var(--bg));
}

.select-trigger.open,
.select-trigger:focus-visible {
  border-color: var(--ink);
  box-shadow: 2px 2px 0 0 var(--rule);
}

.select-trigger .select-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.select-trigger .chevron {
  flex: 0 0 auto;
  width: 18px;
  height: 18px;
  color: var(--ink-4);
  transition: transform 150ms ease;
}

.select-trigger.open .chevron {
  transform: rotate(180deg);
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

.input.invalid {
  border-color: var(--err);
}

.name-row {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
}

.name-input {
  flex: 1;
  min-width: 0;
}

.avatar-trigger {
  flex: 0 0 auto;
  width: 38px;
  height: 38px;
  padding: 2px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--rule-soft);
  background: var(--panel-2);
  cursor: pointer;
  transition: border-color .12s, box-shadow .12s;
}

.avatar-trigger:hover {
  border-color: var(--rule);
}

.avatar-trigger.open,
.avatar-trigger:focus-visible {
  border-color: var(--ink);
  box-shadow: 2px 2px 0 0 var(--rule);
  outline: none;
}

/* Float the picker just below the trigger so the modal layout doesn't
 * jump when it opens. The grid mirrors the sprite (5×4) and uses the
 * sprite's natural pixel-art style. */
.avatar-picker {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  z-index: 5;
  display: grid;
  grid-template-columns: repeat(5, 44px);
  gap: 4px;
  padding: 8px;
  background: var(--panel);
  border: 1px solid var(--rule);
  box-shadow: 4px 4px 0 0 var(--rule);
}

.avatar-option {
  width: 44px;
  height: 44px;
  padding: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: 1px solid transparent;
  cursor: pointer;
  transition: border-color .12s, background-color .12s;
}

.avatar-option:hover {
  border-color: var(--rule-soft);
  background: var(--bg-2);
}

.avatar-option.selected {
  border-color: var(--ink);
  background: var(--bg-2);
}

.avatar-option:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 1px;
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

.advanced-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding-top: 8px;
  border-top: 1px dashed var(--rule-softer);
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
  color: var(--ink-3);
}

.advanced-toggle:hover {
  color: var(--ink);
}

.advanced-arrow {
  font-size: 14px;
  line-height: 1;
  color: var(--ink-4);
  transform: rotate(0deg);
  transition: transform .15s ease, color .15s ease;
  user-select: none;
}

.advanced-arrow.open {
  transform: rotate(180deg);
  color: var(--ink-2);
}

.advanced-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
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

.hint {
  font-size: 11.5px;
  color: var(--ink-4);
  margin: 0;
}

</style>
