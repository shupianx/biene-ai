<template>
  <BaseModal :title="t('modal.desktopSettingsTitle')" max-width="860px" :z-index="220" @close="emit('close')">
    <div class="setting-list">
      <div class="setting-row">
        <div class="setting-copy">
          <span class="setting-label">{{ t('modal.language') }}</span>
          <span class="setting-hint">{{ t('modal.languageHint') }}</span>
        </div>
        <div class="language-group" :aria-label="t('modal.language')">
          <button
            v-for="option in localeOptions"
            :key="option.value"
            class="language-btn"
            :class="{ active: locale === option.value }"
            type="button"
            @click="onLocaleChange(option.value)"
          >
            {{ option.label }}
          </button>
        </div>
      </div>

      <div class="setting-row">
        <div class="setting-copy">
          <span class="setting-label">{{ t('titleBar.darkMode') }}</span>
          <span class="setting-hint">{{ t('modal.darkModeHint') }}</span>
        </div>
        <ToggleSwitch v-model="darkMode" :label="t('titleBar.darkMode')" />
      </div>

      <div v-if="desktopSettingsSupported" class="setting-row">
        <div class="setting-copy">
          <span class="setting-label">{{ t('titleBar.keepCoreRunningOnExit') }}</span>
          <span class="setting-hint">{{ t('modal.keepCoreRunningOnExitHint') }}</span>
        </div>
        <ToggleSwitch
          :model-value="keepCoreRunningOnExit"
          :label="t('titleBar.keepCoreRunningOnExit')"
          @update:model-value="onKeepCoreRunningOnExitChange"
        />
      </div>

      <section class="providers-section">
        <div class="section-head">
          <div class="setting-copy">
            <span class="setting-label">{{ t('modal.modelProviders') }}</span>
            <span class="setting-hint">{{ t('modal.modelProvidersHint') }}</span>
          </div>
          <button class="section-action" type="button" :disabled="configSaving" @click="openAddProvider">
            {{ t('modal.addProvider') }}
          </button>
        </div>

        <p v-if="configError" class="config-error">{{ configError }}</p>
        <p v-if="configLoading" class="config-state">{{ t('modal.providerLoading') }}</p>

        <div v-else-if="coreConfig" class="providers-grid">
          <article
            v-for="entry in coreConfig.model_list"
            :key="entry.id"
            class="provider-card"
            :class="{ default: coreConfig.default_model === entry.id }"
          >
            <div class="provider-card-head">
              <div class="provider-card-copy">
                <div class="provider-title-row">
                  <h3 class="provider-name">{{ entry.name }}</h3>
                  <span v-if="coreConfig.default_model === entry.id" class="provider-default">
                    {{ t('modal.defaultProvider') }}
                  </span>
                </div>
                <p class="provider-model">{{ entry.model }}</p>
                <p v-if="providerUsageCount(entry.id) > 0" class="provider-usage">
                  {{ t('modal.providerInUse', { count: providerUsageCount(entry.id) }) }}
                </p>
              </div>
              <div class="provider-actions">
                <button
                  v-if="coreConfig.default_model !== entry.id"
                  class="card-btn"
                  type="button"
                  :disabled="configSaving"
                  @click="setDefaultProvider(entry.id)"
                >
                  {{ t('modal.makeDefaultProvider') }}
                </button>
                <button class="card-btn" type="button" :disabled="configSaving" @click="openEditProvider(entry)">
                  {{ t('common.edit') }}
                </button>
                <span
                  class="tooltip-anchor"
                  :class="{ 'has-tooltip': !!providerDeleteDisabledReason(entry.id) }"
                  :data-tooltip="providerDeleteDisabledReason(entry.id) || null"
                >
                  <button
                    class="card-btn danger"
                    type="button"
                    :disabled="configSaving || coreConfig.model_list.length <= 1 || providerUsageCount(entry.id) > 0"
                    @click="deleteProvider(entry.id)"
                  >
                    {{ t('common.delete') }}
                  </button>
                </span>
              </div>
            </div>

            <dl class="provider-meta">
              <div class="provider-meta-item">
                <dt>{{ t('modal.providerType') }}</dt>
                <dd>{{ providerLabel(entry.provider) }}</dd>
              </div>
              <div class="provider-meta-item">
                <dt>{{ t('modal.providerBaseUrl') }}</dt>
                <dd>{{ entry.base_url || '—' }}</dd>
              </div>
            </dl>
          </article>
        </div>

        <div v-if="editorMode" class="provider-editor">
          <div class="provider-editor-head">
            <div class="setting-copy">
              <span class="setting-label">
                {{ editorMode === 'add' ? t('modal.addProvider') : t('modal.editProvider') }}
              </span>
              <span class="setting-hint">{{ t('modal.providerEditorHint') }}</span>
            </div>
          </div>

          <p v-if="editorError" class="config-error">{{ editorError }}</p>

          <div class="provider-form-grid">
            <label class="provider-field">
              <span class="provider-field-label">{{ t('modal.quickAddTemplate') }}</span>
              <select v-model="providerTemplate" class="provider-input" @change="applyProviderTemplate(providerTemplate)">
                <option v-for="option in providerTemplateOptions" :key="option.value" :value="option.value">
                  {{ option.label }}
                </option>
              </select>
            </label>

            <label class="provider-field">
              <span class="provider-field-label">{{ t('modal.providerName') }}</span>
              <input v-model="providerDraft.name" class="provider-input" type="text" autocomplete="off" />
            </label>

            <label class="provider-field">
              <span class="provider-field-label">{{ t('modal.providerType') }}</span>
              <select v-model="providerDraft.provider" class="provider-input" :disabled="isProviderTemplateLocked">
                <option v-for="option in providerTypeOptions" :key="option.value" :value="option.value">
                  {{ option.label }}
                </option>
              </select>
            </label>

            <label class="provider-field">
              <span class="provider-field-label">{{ t('modal.providerModel') }}</span>
              <input
                v-model="providerDraft.model"
                class="provider-input mono"
                type="text"
                autocomplete="off"
                :disabled="isProviderTemplateLocked"
              />
            </label>

            <label class="provider-field provider-field-wide">
              <span class="provider-field-label">{{ t('modal.providerApiKey') }}</span>
              <input v-model="providerDraft.api_key" class="provider-input mono" type="password" autocomplete="off" />
            </label>

            <label class="provider-field provider-field-wide">
              <span class="provider-field-label">{{ t('modal.providerBaseUrl') }}</span>
              <input
                v-model="providerDraft.base_url"
                class="provider-input mono"
                type="text"
                autocomplete="off"
                :disabled="isProviderTemplateLocked"
              />
            </label>
          </div>

          <div class="provider-editor-actions">
            <button class="card-btn" type="button" :disabled="configSaving" @click="cancelProviderEditor">
              {{ t('common.cancel') }}
            </button>
            <button class="primary-btn" type="button" :disabled="configSaving" @click="saveProviderDraft">
              {{ t('common.save') }}
            </button>
          </div>
        </div>
      </section>
    </div>

    <template #footer>
      <button class="btn-close" type="button" @click="emit('close')">{{ t('common.close') }}</button>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { fetchConfig, listSessions, saveConfig, type ConfigModelEntry, type CoreConfig, type SessionMeta } from '../api/http'
import { providerTemplateList, providerTemplates, type ProviderTemplateKey } from '../constants/providerTemplates'
import BaseModal from './BaseModal.vue'
import ToggleSwitch from './ToggleSwitch.vue'
import { useTheme } from '../composables/useTheme'
import { useDesktopSettings } from '../composables/useDesktopSettings'
import { t } from '../i18n'
import type { AppLocale } from '../i18n/messages'

type ProviderEditorMode = 'add' | 'edit' | null
type ProviderTemplateID = 'none' | ProviderTemplateKey

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { isDark, setTheme } = useTheme()
const {
  desktopSettingsSupported,
  keepCoreRunningOnExit,
  locale,
  setKeepCoreRunningOnExit,
  setLocalePreference,
} = useDesktopSettings()

const coreConfig = ref<CoreConfig | null>(null)
const configLoading = ref(false)
const configSaving = ref(false)
const configError = ref('')
const editorError = ref('')
const editorMode = ref<ProviderEditorMode>(null)
const editingProviderID = ref('')
const providerTemplate = ref<ProviderTemplateID>('none')
const providerDraft = reactive<ConfigModelEntry>(emptyProviderDraft())
const sessionMetas = ref<SessionMeta[]>([])

const localeOptions = computed<{ value: AppLocale; label: string }[]>(() => [
  { value: 'en', label: t('language.english') },
  { value: 'zh-CN', label: t('language.simplifiedChinese') },
])

const providerTypeOptions = computed(() => [
  { value: 'anthropic', label: t('modal.providerTypes.anthropic') },
  { value: 'openai_compatible', label: t('modal.providerTypes.openaiCompatible') },
])
const providerTemplateOptions = computed<Array<{ value: ProviderTemplateID; label: string }>>(() => [
  { value: 'none', label: t('common.none') },
  ...providerTemplateList.map((template) => ({ value: template.id, label: template.name })),
])
const isProviderTemplateLocked = computed(() => providerTemplate.value !== 'none')

const darkMode = computed({
  get: () => isDark.value,
  set: (value: boolean) => setTheme(value ? 'dark' : 'light'),
})

function emptyProviderDraft(): ConfigModelEntry {
  return {
    id: '',
    name: '',
    provider: 'openai_compatible',
    api_key: '',
    model: '',
    base_url: '',
    thinking_available: false,
  }
}

function cloneProvider(entry: ConfigModelEntry): ConfigModelEntry {
  return {
    id: entry.id,
    name: entry.name,
    provider: entry.provider,
    api_key: entry.api_key,
    model: entry.model,
    base_url: entry.base_url,
    thinking_available: Boolean(entry.thinking_available),
  }
}

function cloneConfig(config: CoreConfig): CoreConfig {
  return {
    default_model: config.default_model,
    model_list: config.model_list.map(cloneProvider),
    max_tokens: config.max_tokens,
  }
}

function normalizeProviderID(value: string) {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
}

function nextProviderIDFromName(name: string, excludeID = '') {
  const existing = new Set(
    (coreConfig.value?.model_list ?? [])
      .map((entry) => entry.id)
      .filter((id) => id !== excludeID)
  )
  const base = normalizeProviderID(name) || 'provider'
  for (let i = 1; ; i += 1) {
    const candidate = i === 1 ? base : `${base}-${i}`
    if (!existing.has(candidate)) return candidate
  }
}

function providerLabel(provider: string) {
  return provider === 'openai_compatible'
    ? t('modal.providerTypes.openaiCompatible')
    : t('modal.providerTypes.anthropic')
}

function providerUsageCount(id: string) {
  return sessionMetas.value.filter((session) => session.model_id === id).length
}

function providerDeleteDisabledReason(id: string) {
  const usage = providerUsageCount(id)
  if (usage > 0) {
    return t('modal.providerDeleteInUseTooltip', { count: usage })
  }
  if ((coreConfig.value?.model_list.length ?? 0) <= 1) {
    return t('modal.providerDeleteLastTooltip')
  }
  return ''
}

function detectProviderTemplate(entry: ConfigModelEntry): ProviderTemplateID {
  for (const [id, template] of Object.entries(providerTemplates)) {
    if (
      entry.provider === template.provider &&
      entry.model === template.model &&
      entry.base_url === template.base_url &&
      Boolean(entry.thinking_available) === Boolean(template.thinking_available)
    ) {
      return id as ProviderTemplateID
    }
  }
  return 'none'
}

function applyProviderTemplate(templateID: ProviderTemplateID) {
  providerTemplate.value = templateID
  if (templateID === 'none') return

  const template = providerTemplates[templateID]
  providerDraft.name = template.name
  providerDraft.provider = template.provider
  providerDraft.model = template.model
  providerDraft.base_url = template.base_url
  providerDraft.thinking_available = Boolean(template.thinking_available)
}

async function loadCoreConfig() {
  configLoading.value = true
  configError.value = ''
  try {
    const [config, sessions] = await Promise.all([fetchConfig(), listSessions()])
    coreConfig.value = config
    sessionMetas.value = sessions
  } catch (error) {
    configError.value = error instanceof Error ? error.message : String(error)
  } finally {
    configLoading.value = false
  }
}

async function persistCoreConfig(next: CoreConfig) {
  configSaving.value = true
  configError.value = ''
  try {
    coreConfig.value = await saveConfig(next)
    sessionMetas.value = await listSessions()
    cancelProviderEditor()
  } catch (error) {
    configError.value = error instanceof Error ? error.message : String(error)
  } finally {
    configSaving.value = false
  }
}

function onKeepCoreRunningOnExitChange(value: boolean) {
  void setKeepCoreRunningOnExit(value)
}

function onLocaleChange(value: AppLocale) {
  void setLocalePreference(value)
}

function openAddProvider() {
  editorMode.value = 'add'
  editingProviderID.value = ''
  editorError.value = ''
  providerTemplate.value = 'none'
  Object.assign(providerDraft, emptyProviderDraft())
}

function openEditProvider(entry: ConfigModelEntry) {
  editorMode.value = 'edit'
  editingProviderID.value = entry.id
  editorError.value = ''
  providerTemplate.value = detectProviderTemplate(entry)
  Object.assign(providerDraft, cloneProvider(entry))
}

function cancelProviderEditor() {
  editorMode.value = null
  editingProviderID.value = ''
  editorError.value = ''
  providerTemplate.value = 'none'
  Object.assign(providerDraft, emptyProviderDraft())
}

async function saveProviderDraft() {
  if (!coreConfig.value) return
  const nextID = editorMode.value === 'edit'
    ? editingProviderID.value
    : nextProviderIDFromName(providerDraft.name)

  const nextEntry: ConfigModelEntry = {
    id: nextID,
    name: providerDraft.name.trim(),
    provider: providerDraft.provider === 'openai_compatible' ? 'openai_compatible' : 'anthropic',
    api_key: providerDraft.api_key.trim(),
    model: providerDraft.model.trim(),
    base_url: providerDraft.base_url.trim(),
    thinking_available: Boolean(providerDraft.thinking_available),
  }

  if (!nextEntry.name) {
    editorError.value = t('modal.providerNameRequired')
    return
  }
  if (!nextEntry.model) {
    editorError.value = t('modal.providerModelRequired')
    return
  }

  const nextConfig = cloneConfig(coreConfig.value)
  const duplicate = nextConfig.model_list.find((entry) =>
    entry.id === nextEntry.id && entry.id !== editingProviderID.value
  )
  if (duplicate) {
    editorError.value = t('modal.providerIdExists')
    return
  }

  if (editorMode.value === 'edit') {
    const index = nextConfig.model_list.findIndex((entry) => entry.id === editingProviderID.value)
    if (index < 0) return
    nextConfig.model_list.splice(index, 1, nextEntry)
    if (nextConfig.default_model === editingProviderID.value) {
      nextConfig.default_model = nextEntry.id
    }
  } else {
    nextConfig.model_list.push(nextEntry)
    if (!nextConfig.default_model) {
      nextConfig.default_model = nextEntry.id
    }
  }

  await persistCoreConfig(nextConfig)
}

async function setDefaultProvider(id: string) {
  if (!coreConfig.value || coreConfig.value.default_model === id) return
  const nextConfig = cloneConfig(coreConfig.value)
  nextConfig.default_model = id
  await persistCoreConfig(nextConfig)
}

async function deleteProvider(id: string) {
  if (!coreConfig.value) return
  if (providerUsageCount(id) > 0) {
    configError.value = t('modal.providerDeleteInUseError')
    return
  }
  if (coreConfig.value.model_list.length <= 1) {
    configError.value = t('modal.providerDeleteLastError')
    return
  }

  const nextConfig = cloneConfig(coreConfig.value)
  nextConfig.model_list = nextConfig.model_list.filter((entry) => entry.id !== id)
  if (nextConfig.default_model === id) {
    nextConfig.default_model = nextConfig.model_list[0]?.id ?? ''
  }
  if (editingProviderID.value === id) {
    cancelProviderEditor()
  }
  await persistCoreConfig(nextConfig)
}

onMounted(() => {
  void loadCoreConfig()
})
</script>

<style scoped>
.setting-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.setting-row,
.providers-section {
  padding: 12px 14px;
  border: 1px solid var(--rule-softer);
  background: var(--panel);
}

.setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.setting-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.setting-label {
  font-family: var(--mono);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--ink-2);
}

.setting-hint {
  font-size: 12px;
  line-height: 1.45;
  color: var(--ink-4);
}

.language-group {
  display: inline-flex;
  align-items: center;
  border: 1px solid var(--rule-soft);
  background: var(--panel-2);
  flex-shrink: 0;
}

.language-btn {
  min-width: 92px;
  height: 30px;
  padding: 0 12px;
  border: none;
  background: transparent;
  color: var(--ink-3);
  cursor: pointer;
  font-family: var(--mono);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  transition: background .12s, color .12s;
}

.language-btn + .language-btn {
  border-left: 1px solid var(--rule-soft);
}

.language-btn:hover {
  background: var(--hover-soft);
  color: var(--ink);
}

.language-btn.active {
  background: var(--ink);
  color: var(--bg);
}

.providers-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.section-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.tooltip-anchor {
  position: relative;
  display: inline-flex;
}

.tooltip-anchor.has-tooltip > .card-btn {
  pointer-events: none;
}

.tooltip-anchor.has-tooltip:hover::after,
.tooltip-anchor.has-tooltip:focus-within::after {
  content: attr(data-tooltip);
  position: absolute;
  right: 0;
  bottom: calc(100% + 8px);
  z-index: 8;
  width: max-content;
  max-width: 240px;
  padding: 7px 9px;
  border: 1px solid var(--rule);
  background: color-mix(in srgb, var(--panel) 92%, var(--bg));
  color: var(--ink-2);
  box-shadow: 0 10px 24px color-mix(in srgb, var(--ink) 12%, transparent);
  font-size: 11px;
  line-height: 1.45;
  letter-spacing: 0.01em;
  text-transform: none;
  white-space: normal;
}

.section-action,
.card-btn,
.btn-close,
.primary-btn {
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
  transition: transform .12s, box-shadow .12s, background .12s, color .12s, border-color .12s;
}

.section-action:hover,
.card-btn:hover,
.btn-close:hover,
.primary-btn:hover {
  transform: translate(-1px, -1px);
  box-shadow: 2px 2px 0 0 var(--rule);
}

.section-action:active,
.card-btn:active,
.btn-close:active,
.primary-btn:active {
  transform: translate(0, 0);
  box-shadow: none;
}

.section-action:disabled,
.card-btn:disabled,
.primary-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
  transform: none;
  box-shadow: none;
}

.config-error,
.config-state {
  margin: 0;
  padding: 10px 12px;
  font-size: 12px;
  line-height: 1.5;
}

.config-error {
  border: 1px solid color-mix(in srgb, var(--err) 30%, transparent);
  background: color-mix(in srgb, var(--err) 8%, var(--panel-2));
  color: var(--err);
}

.config-state {
  border: 1px dashed var(--rule-soft);
  background: var(--panel-2);
  color: var(--ink-4);
}

.providers-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 12px;
}

.provider-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 12px;
  border: 1px solid var(--rule-soft);
  background: var(--panel-2);
}

.provider-card.default {
  border-color: oklch(74% 0.11 248);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, oklch(74% 0.11 248) 48%, transparent);
}

.provider-card-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.provider-card-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.provider-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.provider-name {
  margin: 0;
  font-size: 15px;
  line-height: 1.2;
  color: var(--ink);
}

.provider-default {
  display: inline-flex;
  align-items: center;
  padding: 2px 6px;
  border: 1px solid color-mix(in srgb, oklch(74% 0.11 248) 42%, transparent);
  font-family: var(--mono);
  font-size: 10px;
  letter-spacing: 0.08em;
  color: oklch(64% 0.09 246);
  background: color-mix(in srgb, oklch(74% 0.11 248) 10%, var(--panel));
}

.provider-model {
  margin: 0;
  font-family: var(--mono);
  font-size: 11px;
  color: var(--ink-3);
  word-break: break-word;
}

.provider-usage {
  margin: 0;
  font-size: 11px;
  line-height: 1.4;
  color: var(--ink-4);
}

.provider-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  flex-wrap: wrap;
}

.card-btn.danger {
  color: var(--err);
  border-color: color-mix(in srgb, var(--err) 40%, var(--rule));
}

.provider-meta {
  margin: 0;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.provider-meta-item {
  min-width: 0;
}

.provider-meta-item dt {
  margin: 0 0 4px;
  font-family: var(--mono);
  font-size: 10px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--ink-4);
}

.provider-meta-item dd {
  margin: 0;
  font-size: 12px;
  line-height: 1.5;
  color: var(--ink-2);
  word-break: break-word;
}

.provider-editor {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 12px;
  border: 1px solid var(--rule-soft);
  background: color-mix(in srgb, var(--panel-2) 76%, var(--panel));
}

.provider-form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.provider-field {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.provider-field-wide {
  grid-column: 1 / -1;
}

.provider-field-label {
  font-family: var(--mono);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--ink-3);
}

.provider-input {
  width: 100%;
  min-width: 0;
  height: 34px;
  padding: 0 10px;
  border: 1px solid var(--rule-soft);
  background: var(--panel-2);
  color: var(--ink);
  outline: none;
  font-size: 12px;
}

.provider-input.mono {
  font-family: var(--mono);
}

.provider-input:focus {
  border-color: var(--accent);
}

.provider-input:disabled {
  cursor: not-allowed;
  color: var(--ink-4);
  background: color-mix(in srgb, var(--panel-2) 72%, var(--bg));
}

.provider-editor-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.primary-btn {
  background: var(--ink);
  border-color: var(--ink);
  color: var(--bg);
}

@media (max-width: 760px) {
  .setting-row,
  .section-head,
  .provider-card-head {
    flex-direction: column;
    align-items: stretch;
  }

  .language-group,
  .section-action {
    width: 100%;
  }

  .language-btn {
    flex: 1 1 0;
    min-width: 0;
  }

  .providers-grid,
  .provider-form-grid,
  .provider-meta {
    grid-template-columns: 1fr;
  }
}
</style>
