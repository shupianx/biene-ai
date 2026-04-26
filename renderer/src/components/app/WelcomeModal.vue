<template>
  <BaseModal
    :title="t('welcome.title')"
    max-width="520px"
    :z-index="240"
    :dismissable="false"
  >
    <p class="lead">{{ t('welcome.lead') }}</p>

    <p v-if="errorMessage" class="error-box">{{ errorMessage }}</p>

    <div class="form-grid">
      <div class="field">
        <span class="field-label">{{ t('modal.addModel') }}</span>
        <ProviderTemplateMenu
          :selected-id="template"
          :custom-label="t('welcome.customTemplate')"
          @select="onTemplateSelect"
        >
          <template #trigger="{ open, toggle }">
            <button
              type="button"
              class="input select-trigger"
              :class="{ open }"
              @click="toggle"
            >
              <span class="select-label">{{ currentTemplateLabel }}</span>
              <ArrowDropDownIcon class="chevron" aria-hidden="true" />
            </button>
          </template>
        </ProviderTemplateMenu>
      </div>

      <label class="field">
        <span class="field-label">{{ t('modal.providerName') }}</span>
        <input
          v-model="draft.name"
          class="input"
          type="text"
          autocomplete="off"
        />
      </label>

      <div v-if="!isTemplateLocked" class="field">
        <span class="field-label">{{ t('modal.providerType') }}</span>
        <PopupMenu :items="providerTypeMenuItems" @select="onProviderTypeSelect">
          <template #trigger="{ open, toggle }">
            <button
              type="button"
              class="input select-trigger"
              :class="{ open }"
              @click="toggle"
            >
              <span class="select-label">{{ currentProviderTypeLabel }}</span>
              <ArrowDropDownIcon class="chevron" aria-hidden="true" />
            </button>
          </template>
        </PopupMenu>
      </div>

      <label v-if="!isTemplateLocked" class="field">
        <span class="field-label">{{ t('modal.providerModel') }}</span>
        <input
          v-model="draft.model"
          class="input mono"
          type="text"
          autocomplete="off"
        />
      </label>

      <label class="field field-wide">
        <span class="field-label">{{ t('modal.providerApiKey') }}</span>
        <input
          v-model="draft.api_key"
          class="input mono"
          type="password"
          autocomplete="off"
        />
        <span class="field-hint">{{ t('welcome.apiKeyHint') }}</span>
      </label>

      <label v-if="!isTemplateLocked" class="field field-wide">
        <span class="field-label">{{ t('modal.providerBaseUrl') }}</span>
        <input
          v-model="draft.base_url"
          class="input mono"
          type="text"
          autocomplete="off"
        />
      </label>
    </div>

    <template #footer>
      <AppButton variant="primary" :disabled="saveDisabled" @click="submit">
        {{ saving ? t('welcome.saving') : t('welcome.saveButton') }}
      </AppButton>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import ArrowDropDownIcon from '~icons/material-symbols/arrow-drop-down'
import { saveConfig, type ConfigModelEntry, type CoreConfig } from '../../api/http'
import {
  customTemplate,
  defaultTemplateID,
  providerTemplates,
  providerVendors,
} from '../../constants/providerTemplates'
import { t } from '../../i18n'
import AppButton from '../ui/AppButton.vue'
import BaseModal from '../ui/BaseModal.vue'
import PopupMenu, { type PopupMenuEntry } from '../ui/PopupMenu.vue'
import ProviderTemplateMenu from '../ui/ProviderTemplateMenu.vue'

// Same id shape as DesktopSettingsModal: 'custom' | 'vendor:<id>' | <modelID>.
type TemplateID = string

const emit = defineEmits<{
  (e: 'done', config: CoreConfig): void
}>()

const template = ref<TemplateID>(defaultTemplateID)
const saving = ref(false)
const errorMessage = ref('')

const draft = reactive<ConfigModelEntry>({
  id: '',
  name: '',
  provider: 'openai_compatible',
  api_key: '',
  model: '',
  base_url: '',
  thinking_available: false,
  thinking_on: undefined,
  thinking_off: undefined,
})

// Apply the default template once at mount so the form opens with a
// concrete preset (provider, model, base_url). User still has to fill
// api_key.
applyTemplate(defaultTemplateID)

const currentTemplateLabel = computed(() => {
  const id = template.value
  if (id === customTemplate.id) return t('welcome.customTemplate')
  if (id.startsWith('vendor:')) {
    const vendor = providerVendors.find((v) => `vendor:${v.id}` === id)
    return vendor?.name ?? t('welcome.customTemplate')
  }
  const preset = providerTemplates[id]
  return preset ? `${preset.vendorName} · ${preset.name}` : t('welcome.customTemplate')
})

const providerTypeOptions = computed(() => [
  { value: 'anthropic', label: t('modal.providerTypes.anthropic') },
  { value: 'openai_compatible', label: t('modal.providerTypes.openaiCompatible') },
])

const providerTypeMenuItems = computed<PopupMenuEntry[]>(() =>
  providerTypeOptions.value.map((option) => ({
    key: option.value,
    label: option.label,
    selected: option.value === draft.provider,
  }))
)

const currentProviderTypeLabel = computed(
  () => providerTypeOptions.value.find((o) => o.value === draft.provider)?.label ?? ''
)

const isTemplateLocked = computed(
  () =>
    template.value !== customTemplate.id &&
    !template.value.startsWith('vendor:') &&
    Boolean(providerTemplates[template.value]),
)

const saveDisabled = computed(
  () =>
    saving.value ||
    !draft.name.trim() ||
    !draft.model.trim() ||
    !draft.api_key.trim()
)

function onTemplateSelect(key: string) {
  applyTemplate(key as TemplateID)
}

function applyTemplate(id: TemplateID) {
  template.value = id

  if (id === customTemplate.id) {
    draft.name = ''
    draft.thinking_on = undefined
    draft.thinking_off = undefined
    return
  }

  if (id.startsWith('vendor:')) {
    const vendorId = id.slice('vendor:'.length)
    const vendor = providerVendors.find((v) => v.id === vendorId)
    if (!vendor) return
    draft.name = ''
    draft.provider = vendor.provider
    draft.model = ''
    draft.base_url = vendor.base_url
    draft.thinking_available = false
    draft.thinking_on = undefined
    draft.thinking_off = undefined
    return
  }

  const preset = providerTemplates[id]
  if (!preset) return
  draft.name = `${preset.vendorName} ${preset.name}`
  draft.provider = preset.provider
  draft.model = preset.model
  draft.base_url = preset.base_url
  draft.thinking_available = Boolean(preset.thinking_available)
  draft.thinking_on = preset.thinking_on
  draft.thinking_off = preset.thinking_off
}

function onProviderTypeSelect(key: string) {
  if (isTemplateLocked.value) return
  draft.provider = key as ConfigModelEntry['provider']
}

function normalizeID(value: string) {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
}

async function submit() {
  if (saveDisabled.value) return
  saving.value = true
  errorMessage.value = ''
  try {
    const id = normalizeID(draft.name) || 'main'
    const entry: ConfigModelEntry = {
      id,
      name: draft.name.trim(),
      provider: draft.provider === 'openai_compatible' ? 'openai_compatible' : 'anthropic',
      api_key: draft.api_key.trim(),
      model: draft.model.trim(),
      base_url: draft.base_url.trim(),
      thinking_available: Boolean(draft.thinking_available),
      thinking_on: draft.thinking_on,
      thinking_off: draft.thinking_off,
    }
    const nextConfig: CoreConfig = {
      default_model: id,
      model_list: [entry],
    }
    const saved = await saveConfig(nextConfig)
    emit('done', saved)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : String(error)
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.lead {
  margin: 0;
  font-size: 13px;
  line-height: 1.5;
  color: var(--ink-3);
}

.error-box {
  margin: 0;
  padding: 10px 12px;
  border: 1px solid color-mix(in srgb, var(--err) 30%, transparent);
  background: color-mix(in srgb, var(--err) 8%, var(--panel-2));
  color: var(--err);
  font-size: 12px;
  line-height: 1.5;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.field {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.field-wide {
  grid-column: 1 / -1;
}

.field-label {
  font-family: var(--mono);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--ink-3);
}

.field-hint {
  font-size: 11.5px;
  color: var(--ink-4);
  line-height: 1.4;
}

.input {
  width: 100%;
  min-width: 0;
  height: 34px;
  padding: 0 10px;
  border: 1px solid var(--rule-soft);
  background: var(--panel-2);
  color: var(--ink);
  outline: none;
  font-size: 13px;
  font-family: var(--sans);
  transition: border-color .12s, box-shadow .12s;
}

.input.mono {
  font-family: var(--mono);
}

.input:focus {
  border-color: var(--ink);
  box-shadow: 2px 2px 0 0 var(--rule);
}

.input:disabled {
  cursor: not-allowed;
  color: var(--ink-3);
  background: color-mix(in srgb, var(--panel-2) 76%, var(--bg));
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

@media (max-width: 560px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
