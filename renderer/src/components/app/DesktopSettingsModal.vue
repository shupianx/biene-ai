<template>
  <BaseModal :title="t('modal.desktopSettingsTitle')" max-width="860px" :z-index="220" @close="emit('close')">
    <div class="setting-list">
      <div class="setting-row">
        <div class="setting-copy">
          <span class="setting-label">{{ t('modal.language') }}</span>
          <span class="setting-hint">{{ t('modal.languageHint') }}</span>
        </div>
        <SelectField
          class="language-select"
          :model-value="locale"
          :options="localeOptions"
          :aria-label="t('modal.language')"
          @update:model-value="onLocaleChange($event)"
        />
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

      <section class="chatgpt-auth-section">
        <div class="section-head">
          <div class="setting-copy">
            <span class="setting-label chatgpt-auth-label">
              <span class="chatgpt-auth-label-text">{{ t('chatgptAuth.title') }}</span>
              <!--
                Star sits right after the title text (not at the row's
                far edge) — matches the New Agent dropdown's accent
                marker so OAuth-derived options read as the same kind
                of "premium / managed" affordance everywhere it appears.
              -->
              <StarShineIcon class="chatgpt-auth-accent-icon" aria-hidden="true" />
            </span>
            <span class="setting-hint">{{ t('chatgptAuth.description') }}</span>
            <span v-if="chatgptAuthError" class="config-error">{{ chatgptAuthError }}</span>
          </div>
          <div class="chatgpt-auth-trailing">
            <span class="chatgpt-auth-status" :class="{ muted: !chatgptAuth.authenticated }">
              {{ chatgptStatusLabel }}
            </span>
            <AppButton
              v-if="!chatgptAuth.authenticated"
              variant="neutral"
              :disabled="chatgptLoginPending"
              @click="onChatGPTLogin"
            >
              {{ t('chatgptAuth.loginButton') }}
            </AppButton>
            <AppButton
              v-else
              variant="neutral"
              :disabled="chatgptLogoutPending"
              @click="onChatGPTLogout"
            >
              {{ t('chatgptAuth.logoutButton') }}
            </AppButton>
          </div>
        </div>

        <!--
          Quota / rate-limit panel. Shown only when authenticated; we
          surface both the primary (short window, soaks bursts) and
          secondary (long window, plan total) buckets the upstream
          returns. On error we keep the prior values visible so a
          flaky network doesn't make the panel disappear.
        -->
        <div v-if="chatgptAuth.authenticated" class="chatgpt-usage">
          <div class="chatgpt-usage-head">
            <span class="setting-hint">{{ t('chatgptAuth.usageTitle') }}</span>
            <button
              type="button"
              class="chatgpt-usage-refresh"
              :disabled="chatgptUsageLoading"
              @click="loadChatGPTUsage"
            >
              {{ chatgptUsageLoading ? t('chatgptAuth.usageLoading') : t('chatgptAuth.usageRefresh') }}
            </button>
          </div>

          <p v-if="chatgptUsageError && !chatgptUsage" class="config-error">
            {{ chatgptUsageError }}
          </p>

          <p v-else-if="!hasUsageSnapshot && !chatgptUsageLoading" class="setting-hint">
            {{ t('chatgptAuth.usageNotAvailable') }}
          </p>

          <template v-if="hasUsageSnapshot">
            <div
              v-for="row in usageWindows"
              :key="row.label"
              class="chatgpt-usage-row"
            >
              <div class="chatgpt-usage-row-head">
                <span class="chatgpt-usage-label">{{ row.label }}</span>
                <span class="chatgpt-usage-numeric">
                  {{ row.win.used_percent.toFixed(0) }}%
                  <template v-if="row.win.reset_at">
                    · {{ formatResetDelta(row.win.reset_at) }}
                  </template>
                </span>
              </div>
              <div class="chatgpt-usage-bar" :class="{ exhausted: row.win.used_percent >= 100 }">
                <div
                  class="chatgpt-usage-bar-fill"
                  :style="{ width: usageBarFill(row.win.used_percent) }"
                />
              </div>
            </div>
          </template>
        </div>

        <!--
          Manual-paste fallback. Shown only when /start signalled that
          the local 1455 listener couldn't bind (typically because the
          Codex CLI is already running on this machine). The browser
          still finishes the redirect, but the user has to copy the
          resulting URL back into Biene.
        -->
        <div v-if="chatgptManualPaste.required" class="chatgpt-manual-paste">
          <p class="setting-hint">{{ t('chatgptAuth.manualPasteHint') }}</p>
          <p v-if="chatgptManualPaste.bindError" class="config-error">
            {{ t('chatgptAuth.manualPasteBindError', { err: chatgptManualPaste.bindError }) }}
          </p>
          <textarea
            v-model="chatgptManualPaste.pasted"
            class="chatgpt-manual-paste-input"
            rows="3"
            :placeholder="t('chatgptAuth.manualPastePlaceholder')"
            :disabled="chatgptManualPaste.submitting"
          ></textarea>
          <div class="chatgpt-manual-paste-actions">
            <AppButton
              variant="neutral"
              :disabled="chatgptManualPaste.submitting"
              @click="onChatGPTManualPasteCancel"
            >
              {{ t('common.cancel') }}
            </AppButton>
            <AppButton
              variant="primary"
              :disabled="chatgptManualPaste.submitting || !chatgptManualPaste.pasted.trim()"
              @click="onChatGPTManualPasteSubmit"
            >
              {{ t('chatgptAuth.manualPasteSubmit') }}
            </AppButton>
          </div>
        </div>
      </section>

      <section class="providers-section">
        <div class="section-head">
          <div class="setting-copy">
            <span class="setting-label">{{ t('modal.modelProviders') }}</span>
            <span class="setting-hint">{{ t('modal.modelProvidersHint') }}</span>
          </div>
          <AppButton variant="neutral" :disabled="configSaving" @click="openAddProvider">
            {{ t('modal.addProvider') }}
          </AppButton>
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
                  class="card-btn accent"
                  type="button"
                  :disabled="configSaving"
                  @click="setDefaultProvider(entry.id)"
                >
                  {{ t('modal.makeDefaultProvider') }}
                </button>
                <AppButton variant="neutral" size="compact" :disabled="configSaving" @click="openEditProvider(entry)">
                  {{ t('common.edit') }}
                </AppButton>
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

        <div v-if="editorMode" ref="editorRef" class="provider-editor">
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
            <div class="provider-field">
              <span class="provider-field-label">{{ t('modal.addModel') }}</span>
              <ProviderTemplateMenu
                :selected-id="providerTemplate"
                :custom-label="t('modal.customProvider')"
                @select="onTemplateSelect"
              >
                <template #trigger="{ open, toggle }">
                  <button
                    type="button"
                    class="provider-input provider-select-trigger"
                    :class="{ open }"
                    @click="toggle"
                  >
                    <span class="select-label">{{ currentTemplateLabel }}</span>
                    <ArrowDropDownIcon class="chevron" aria-hidden="true" />
                  </button>
                </template>
              </ProviderTemplateMenu>
            </div>

            <label class="provider-field">
              <span class="provider-field-label">{{ t('modal.providerName') }}</span>
              <input v-model="providerDraft.name" class="provider-input" type="text" autocomplete="off" />
            </label>

            <div v-if="!isProviderTemplateLocked" class="provider-field">
              <span class="provider-field-label">{{ t('modal.providerType') }}</span>
              <PopupMenu :items="providerTypeMenuItems" @select="onProviderTypeSelect">
                <template #trigger="{ open, toggle }">
                  <button
                    type="button"
                    class="provider-input provider-select-trigger"
                    :class="{ open }"
                    @click="toggle"
                  >
                    <span class="select-label">{{ currentProviderTypeLabel }}</span>
                    <ArrowDropDownIcon class="chevron" aria-hidden="true" />
                  </button>
                </template>
              </PopupMenu>
            </div>

            <label v-if="!isProviderTemplateLocked" class="provider-field">
              <span class="provider-field-label">{{ t('modal.providerModel') }}</span>
              <input
                v-model="providerDraft.model"
                class="provider-input mono"
                type="text"
                autocomplete="off"
              />
            </label>

            <label class="provider-field provider-field-wide">
              <span class="provider-field-label">{{ t('modal.providerApiKey') }}</span>
              <input v-model="providerDraft.api_key" class="provider-input mono" type="password" autocomplete="off" />
            </label>

            <label v-if="!isProviderTemplateLocked" class="provider-field provider-field-wide">
              <span class="provider-field-label">{{ t('modal.providerBaseUrl') }}</span>
              <input
                v-model="providerDraft.base_url"
                class="provider-input mono"
                type="text"
                autocomplete="off"
              />
            </label>

            <label class="provider-field provider-field-wide">
              <span class="provider-field-label">Context window (tokens)</span>
              <input
                v-model.number="providerDraft.context_window"
                class="provider-input mono"
                type="number"
                min="0"
                step="1000"
                placeholder="leave blank for default (32000)"
                autocomplete="off"
              />
            </label>
          </div>

          <div class="provider-editor-actions">
            <AppButton variant="neutral" :disabled="configSaving" @click="cancelProviderEditor">
              {{ t('common.cancel') }}
            </AppButton>
            <AppButton variant="primary" :disabled="configSaving" @click="saveProviderDraft">
              {{ t('common.save') }}
            </AppButton>
          </div>
        </div>
      </section>
    </div>

    <template #footer>
      <AppButton variant="neutral" @click="emit('close')">{{ t('common.close') }}</AppButton>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import {
  cancelChatGPTOAuth,
  fetchChatGPTAuthStatus,
  fetchChatGPTUsage,
  fetchConfig,
  finishChatGPTOAuthManually,
  listSessions,
  logoutChatGPT,
  saveConfig,
  startChatGPTOAuth,
  type ChatGPTAuthStatus,
  type ChatGPTUsageResponse,
  type RateLimitWindow,
  type ConfigModelEntry,
  type CoreConfig,
  type SessionMeta,
} from '../../api/http'
import {
  customTemplate,
  defaultTemplateID,
  type ProviderVendor,
} from '../../constants/providerTemplates'
import { useProviderTemplatesStore } from '../../stores/providerTemplates'
import ArrowDropDownIcon from '~icons/material-symbols/arrow-drop-down'
import StarShineIcon from '~icons/material-symbols/star-shine'
import AppButton from '../ui/AppButton.vue'
import BaseModal from '../ui/BaseModal.vue'
import PopupMenu, { type PopupMenuEntry } from '../ui/PopupMenu.vue'
import ProviderTemplateMenu from '../ui/ProviderTemplateMenu.vue'
import SelectField from '../ui/SelectField.vue'
import ToggleSwitch from '../ui/ToggleSwitch.vue'
import { useTheme } from '../../composables/useTheme'
import { useDesktopSettings } from '../../composables/useDesktopSettings'
import { t } from '../../i18n'
import type { AppLocale } from '../../i18n/messages'

type ProviderEditorMode = 'add' | 'edit' | null
// One of: 'custom', 'vendor:<vendorId>' (no preset model selected,
// vendor's provider/base_url were applied), or a model template id.
type ProviderTemplateID = string

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

// ChatGPT OAuth state — independent from coreConfig because the tokens
// live in a separate file (~/.biene/chatgpt_tokens.json) and the
// "logged in" status is what gates the synthetic provider in the New
// Agent picker.
const chatgptAuth = ref<ChatGPTAuthStatus>({ authenticated: false })
const chatgptLoginPending = ref(false)
const chatgptLogoutPending = ref(false)
const chatgptAuthError = ref('')

// Manual-paste fallback state. Populated when the /start response sets
// `manual_paste_required` (port 1455 already in use). The textarea
// accepts the full redirect URL the browser ends up on, just the query
// fragment, or the bare `code` value — the server parses all three.
const chatgptManualPaste = reactive({
  required: false,
  state: '',
  authUrl: '',
  pasted: '',
  submitting: false,
  bindError: '',
})

function resetChatGPTManualPaste() {
  chatgptManualPaste.required = false
  chatgptManualPaste.state = ''
  chatgptManualPaste.authUrl = ''
  chatgptManualPaste.pasted = ''
  chatgptManualPaste.submitting = false
  chatgptManualPaste.bindError = ''
}

// Quota / rate-limit panel state. The snapshot is populated as a
// side-effect of normal Codex turns (read off `x-codex-*` response
// headers in the backend), not by an active fetch — we just pull
// the cached value on demand. Empty until the user has sent at
// least one Codex message; the empty-state copy explains that.
const chatgptUsage = ref<ChatGPTUsageResponse | null>(null)
const chatgptUsageLoading = ref(false)
const chatgptUsageError = ref('')

async function loadChatGPTUsage() {
  if (!chatgptAuth.value.authenticated) {
    chatgptUsage.value = null
    chatgptUsageError.value = ''
    return
  }
  chatgptUsageLoading.value = true
  chatgptUsageError.value = ''
  try {
    chatgptUsage.value = await fetchChatGPTUsage()
  } catch (err) {
    chatgptUsageError.value = err instanceof Error ? err.message : String(err)
    // Keep the prior snapshot rather than wiping it on transient
    // errors — a flaky network shouldn't make the user think their
    // quota disappeared.
  } finally {
    chatgptUsageLoading.value = false
  }
}

// formatResetDelta turns the unix-seconds reset_at into a "resets in
// X min" / "X h" / "X d" string. Negative deltas (clock skew, stale
// snapshot crossing the boundary) clamp to 0 — the user shouldn't
// see "resets in -3 min".
function formatResetDelta(resetAt: number): string {
  const delta = Math.max(0, resetAt - Math.floor(Date.now() / 1000))
  if (delta < 60) return t('chatgptAuth.usageResetSec', { sec: delta })
  if (delta < 3600) return t('chatgptAuth.usageResetMin', { min: Math.round(delta / 60) })
  if (delta < 86400) return t('chatgptAuth.usageResetHr', { hr: Math.round(delta / 3600) })
  return t('chatgptAuth.usageResetDay', { day: Math.round(delta / 86400) })
}

// usageWindows assembles the (label, window) pairs the template
// renders as progress bars. Filters out empty windows so an upstream
// that only reports one tier doesn't show a phantom "Long-term" row.
const usageWindows = computed<Array<{ label: string; win: RateLimitWindow }>>(() => {
  const snap = chatgptUsage.value?.snapshot
  if (!snap) return []
  const out: Array<{ label: string; win: RateLimitWindow }> = []
  if (snap.primary) {
    out.push({ label: t('chatgptAuth.usagePrimary'), win: snap.primary })
  }
  if (snap.secondary) {
    out.push({ label: t('chatgptAuth.usageSecondary'), win: snap.secondary })
  }
  return out
})

// hasUsageSnapshot is the gate the template uses to decide between
// the empty-state copy and the populated rows. Pulled into a
// computed so both Vue blocks reference the same predicate.
const hasUsageSnapshot = computed(
  () => chatgptUsage.value?.available === true && !!chatgptUsage.value.snapshot,
)

// usageBarFill clamps used_percent into [0, 100] for the CSS width —
// the upstream occasionally reports >100 right after a burst because
// of rounding, and a 105% bar fill is visually broken.
function usageBarFill(percent: number): string {
  return Math.max(0, Math.min(100, percent)).toFixed(0) + '%'
}

// Inline status label rendered next to the action button. Pending state
// takes precedence so the user sees "等待浏览器…" while the OAuth flow
// is in flight; otherwise it reflects the persisted authenticated state.
const chatgptStatusLabel = computed(() => {
  if (chatgptLoginPending.value) return t('chatgptAuth.loginPending')
  if (!chatgptAuth.value.authenticated) return t('chatgptAuth.statusSignedOut')
  if (chatgptAuth.value.email) {
    return t('chatgptAuth.statusSignedInAs', { email: chatgptAuth.value.email })
  }
  return t('chatgptAuth.statusSignedIn')
})
const editorError = ref('')
const editorMode = ref<ProviderEditorMode>(null)
const editingProviderID = ref('')
const providerTemplate = ref<ProviderTemplateID>(defaultTemplateID)
const providerDraft = reactive<ConfigModelEntry>(emptyProviderDraft())
const sessionMetas = ref<SessionMeta[]>([])
const editorRef = ref<HTMLElement | null>(null)
const tplStore = useProviderTemplatesStore()
void tplStore.ensureLoaded()

const localeOptions = computed<{ value: AppLocale; label: string }[]>(() => [
  { value: 'en', label: t('language.english') },
  { value: 'zh-CN', label: t('language.simplifiedChinese') },
  { value: 'de', label: t('language.german') },
])

const providerTypeOptions = computed(() => [
  { value: 'anthropic', label: t('modal.providerTypes.anthropic') },
  { value: 'openai_compatible', label: t('modal.providerTypes.openaiCompatible') },
])
// Locked = a concrete model preset is selected; the user can still edit
// `name` and `api_key`, but provider type / model / base_url come from
// the template and are read-only. Both "custom" and "vendor:<id>" leave
// the form fully editable.
const isProviderTemplateLocked = computed(
  () =>
    providerTemplate.value !== customTemplate.id &&
    !providerTemplate.value.startsWith('vendor:') &&
    Boolean(tplStore.byId[providerTemplate.value]),
)

const currentTemplateLabel = computed(() => {
  const id = providerTemplate.value
  if (id === customTemplate.id) return t('modal.customProvider')
  if (id.startsWith('vendor:')) {
    const vendorId = id.slice('vendor:'.length)
    const vendor = tplStore.vendors.find((v: ProviderVendor) => v.id === vendorId)
    return vendor?.name ?? t('modal.customProvider')
  }
  const template = tplStore.byId[id]
  if (!template) return t('modal.customProvider')
  return `${template.vendorName} · ${template.name}`
})

const providerTypeMenuItems = computed<PopupMenuEntry[]>(() =>
  providerTypeOptions.value.map((option) => ({
    key: option.value,
    label: option.label,
    selected: option.value === providerDraft.provider,
  }))
)
const currentProviderTypeLabel = computed(
  () =>
    providerTypeOptions.value.find((o) => o.value === providerDraft.provider)?.label ?? ''
)

function onTemplateSelect(key: string) {
  providerTemplate.value = key as ProviderTemplateID
  applyProviderTemplate(providerTemplate.value)
}

function onProviderTypeSelect(key: string) {
  if (isProviderTemplateLocked.value) return
  providerDraft.provider = key as ConfigModelEntry['provider']
}

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
    thinking_on: undefined,
    thinking_off: undefined,
    images_available: true,
    context_window: undefined,
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
    thinking_on: entry.thinking_on,
    thinking_off: entry.thinking_off,
    images_available: entry.images_available !== false,
    context_window: entry.context_window,
  }
}

function cloneConfig(config: CoreConfig): CoreConfig {
  return {
    default_model: config.default_model,
    model_list: config.model_list.map(cloneProvider),
    compaction: config.compaction,
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
  for (const [id, template] of Object.entries(tplStore.byId)) {
    if (
      entry.provider === template.provider &&
      entry.model === template.model &&
      entry.base_url === template.base_url &&
      Boolean(entry.thinking_available) === Boolean(template.thinking_available) &&
      (entry.images_available !== false) === (template.images_available !== false)
    ) {
      return id
    }
  }
  return customTemplate.id
}

function applyProviderTemplate(templateID: ProviderTemplateID) {
  providerTemplate.value = templateID

  if (templateID === customTemplate.id) {
    providerDraft.thinking_on = undefined
    providerDraft.thinking_off = undefined
    providerDraft.images_available = true
    providerDraft.context_window = undefined
    return
  }

  if (templateID.startsWith('vendor:')) {
    // Vendor-only selection: pre-fill provider type + base_url, leave name
    // and model blank for the user to enter. Useful for vendors we know
    // about but haven't shipped a concrete model preset for.
    const vendorId = templateID.slice('vendor:'.length)
    const vendor = tplStore.vendors.find((v: ProviderVendor) => v.id === vendorId)
    if (!vendor) return
    providerDraft.name = ''
    providerDraft.provider = vendor.provider
    providerDraft.model = ''
    providerDraft.base_url = vendor.base_url
    providerDraft.thinking_available = false
    providerDraft.thinking_on = undefined
    providerDraft.thinking_off = undefined
    providerDraft.images_available = true
    providerDraft.context_window = undefined
    return
  }

  const template = tplStore.byId[templateID]
  if (!template) return
  providerDraft.name = `${template.vendorName} ${template.name}`
  providerDraft.provider = template.provider
  providerDraft.model = template.model
  providerDraft.base_url = template.base_url
  providerDraft.thinking_available = Boolean(template.thinking_available)
  providerDraft.thinking_on = template.thinking_on
  providerDraft.thinking_off = template.thinking_off
  providerDraft.images_available = template.images_available !== false
  providerDraft.context_window = template.context_window
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
  Object.assign(providerDraft, emptyProviderDraft())
  // Pre-apply the configured default model so the editor opens with a
  // working baseline (provider type, model name, base url, thinking
  // toggles). User still has to fill in api_key.
  applyProviderTemplate(defaultTemplateID)
  nextTick(() => {
    editorRef.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  })
}

function openEditProvider(entry: ConfigModelEntry) {
  editorMode.value = 'edit'
  editingProviderID.value = entry.id
  editorError.value = ''
  providerTemplate.value = detectProviderTemplate(entry)
  Object.assign(providerDraft, cloneProvider(entry))
  nextTick(() => {
    editorRef.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  })
}

function cancelProviderEditor() {
  editorMode.value = null
  editingProviderID.value = ''
  editorError.value = ''
  providerTemplate.value = customTemplate.id
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
    thinking_on: providerDraft.thinking_on,
    thinking_off: providerDraft.thinking_off,
    images_available: providerDraft.images_available !== false,
    context_window:
      providerDraft.context_window && providerDraft.context_window > 0
        ? providerDraft.context_window
        : undefined,
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
  void refreshChatGPTAuth()
})

async function refreshChatGPTAuth() {
  try {
    chatgptAuth.value = await fetchChatGPTAuthStatus()
  } catch (err) {
    // Status reads are best-effort — a transient core blip shouldn't
    // block the rest of the settings UI.
    chatgptAuth.value = { authenticated: false }
  }
  // Whenever auth flips (initial load, login completion, logout), the
  // usage panel needs to follow: re-fetch when authenticated, clear
  // otherwise. loadChatGPTUsage handles both cases.
  void loadChatGPTUsage()
}

// Login flow: kick the server's start endpoint, open the auth URL in
// the user's browser, then poll for status changes until the listener
// completes the token exchange. The server-side listener resolves on
// the OAuth callback, so this loop just watches for the persisted
// state to flip to authenticated.
async function onChatGPTLogin() {
  if (chatgptLoginPending.value) return
  chatgptLoginPending.value = true
  chatgptAuthError.value = ''
  resetChatGPTManualPaste()
  let started: Awaited<ReturnType<typeof startChatGPTOAuth>> | null = null
  try {
    started = await startChatGPTOAuth()
    const desktop = (window as unknown as { bieneDesktop?: { openExternal: (url: string) => void } }).bieneDesktop
    if (desktop?.openExternal) {
      desktop.openExternal(started.auth_url)
    } else {
      window.open(started.auth_url, '_blank', 'noopener,noreferrer')
    }
    if (started.manual_paste_required) {
      // The local listener couldn't bind (typically Codex CLI
      // running on the same machine). Show the paste box; the
      // login pending flag stays true so the action button is
      // disabled until the user finishes (or the auth section
      // resets after success/failure).
      chatgptManualPaste.required = true
      chatgptManualPaste.state = started.state
      chatgptManualPaste.authUrl = started.auth_url
      chatgptManualPaste.bindError = started.port_bind_error ?? ''
      // We deliberately do NOT call waitForChatGPTAuth here: nothing
      // is going to flip status to authenticated until the user
      // submits via onChatGPTManualPasteSubmit. The login button
      // re-enables only after that submit (or cancel).
      return
    }
    await waitForChatGPTAuth(started.expires_in_seconds)
  } catch (err) {
    chatgptAuthError.value = parseChatGPTAuthError(err)
    if (started?.state) {
      try { await cancelChatGPTOAuth(started.state) } catch { /* noop */ }
    }
  } finally {
    // Manual-paste flow keeps the pending flag on so the button
    // stays disabled while the user is in the paste step.
    if (!chatgptManualPaste.required) {
      chatgptLoginPending.value = false
    }
  }
}

async function onChatGPTManualPasteSubmit() {
  if (chatgptManualPaste.submitting) return
  const code = chatgptManualPaste.pasted.trim()
  if (!code) return
  chatgptManualPaste.submitting = true
  chatgptAuthError.value = ''
  try {
    await finishChatGPTOAuthManually(chatgptManualPaste.state, code)
    // Re-fetch status rather than trusting the {ok:true} response —
    // matches the listener path's behavior, where waitForChatGPTAuth
    // pulls a fresh snapshot before flipping the UI to authenticated.
    chatgptAuth.value = await fetchChatGPTAuthStatus()
    resetChatGPTManualPaste()
    chatgptLoginPending.value = false
    // Authenticated for the first time — pull the quota panel in.
    void loadChatGPTUsage()
  } catch (err) {
    chatgptAuthError.value = parseChatGPTAuthError(err)
  } finally {
    chatgptManualPaste.submitting = false
  }
}

async function onChatGPTManualPasteCancel() {
  if (chatgptManualPaste.state) {
    try { await cancelChatGPTOAuth(chatgptManualPaste.state) } catch { /* noop */ }
  }
  resetChatGPTManualPaste()
  chatgptLoginPending.value = false
}

async function waitForChatGPTAuth(expiresInSeconds: number) {
  const deadline = Date.now() + expiresInSeconds * 1000
  while (Date.now() < deadline) {
    await new Promise((r) => setTimeout(r, 1500))
    try {
      const status = await fetchChatGPTAuthStatus()
      if (status.authenticated) {
        chatgptAuth.value = status
        // First successful auth — pull the quota panel in. Same hook
        // the manual-paste path uses; keeps both login routes
        // visually consistent on completion.
        void loadChatGPTUsage()
        return
      }
      // Backend recorded a token-exchange / state-mismatch failure —
      // bail out immediately instead of timing out, and pass the
      // server-side message through verbatim so the user sees what
      // actually broke.
      if (status.last_error) {
        throw new Error(status.last_error)
      }
    } catch (err) {
      // Network blips are fine — keep polling. But propagate errors
      // we threw ourselves above (carrying the server's last_error).
      if (err instanceof Error && err.message && !err.message.includes('fetch')) {
        throw err
      }
    }
  }
  // Time-out: surface a friendly error and let the user retry. The
  // server's pending-flow TTL (5 min) matches expiresInSeconds, so the
  // server will have already expired the codeVerifier on its side.
  throw new Error(t('chatgptAuth.cancelled'))
}

function parseChatGPTAuthError(err: unknown): string {
  if (err instanceof Error) {
    if (err.message.includes('callback port unavailable')) {
      return t('chatgptAuth.portInUse')
    }
    if (err.message === t('chatgptAuth.cancelled')) {
      return t('chatgptAuth.cancelled')
    }
    // Server-side last_error already carries a useful message
    // (state-mismatch text, OAuth provider error description, etc.).
    if (err.message) {
      return err.message
    }
  }
  return t('chatgptAuth.failed')
}

async function onChatGPTLogout() {
  if (chatgptLogoutPending.value) return
  chatgptLogoutPending.value = true
  chatgptAuthError.value = ''
  try {
    await logoutChatGPT()
    await refreshChatGPTAuth()
  } catch (err) {
    chatgptAuthError.value = err instanceof Error ? err.message : String(err)
  } finally {
    chatgptLogoutPending.value = false
  }
}
</script>

<style scoped>
.setting-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  /* Breathing room below the last section so the last row doesn't sit
   * flush against the modal's bottom edge / footer rule. */
  padding-bottom: 32px;
}

.setting-row,
.providers-section,
.chatgpt-auth-section {
  padding: 12px 14px;
  border: 1px solid var(--rule-softer);
  background: var(--panel);
}

.chatgpt-auth-trailing {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.chatgpt-auth-status {
  font-size: 12px;
  color: var(--ink-3);
  white-space: nowrap;
}

.chatgpt-auth-status.muted {
  color: var(--ink-4);
}

/*
 * Title + accent star, paired side-by-side. The label container is
 * inline-flex so the star tracks the text's right edge regardless of
 * the surrounding row's available width — same visual contract as the
 * New Agent dropdown's `.menu-item-accent-icon`.
 */
.chatgpt-auth-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.chatgpt-auth-accent-icon {
  flex: 0 0 auto;
  width: 14px;
  height: 14px;
  /* Tone matches PopupMenu's hover/selected accent depth — section
   * headings sit at higher visual hierarchy than dropdown rows, so
   * the star tracks the dropdown's *active* state rather than its
   * resting one to read consistently strong here. */
  color: var(--ink-3);
  opacity: 0.8;
}

.chatgpt-usage {
  margin-top: 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.chatgpt-usage-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
}

.chatgpt-usage-refresh {
  border: none;
  background: transparent;
  color: var(--ink-3);
  font-size: 11px;
  cursor: pointer;
  padding: 0;
}

.chatgpt-usage-refresh:hover:not(:disabled) {
  color: var(--ink);
  text-decoration: underline;
}

.chatgpt-usage-refresh:disabled {
  opacity: 0.6;
  cursor: default;
}

.chatgpt-usage-row {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.chatgpt-usage-row-head {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: var(--ink-2);
}

.chatgpt-usage-numeric {
  font-variant-numeric: tabular-nums;
  color: var(--ink-3);
}

.chatgpt-usage-bar {
  height: 6px;
  border-radius: 3px;
  background: var(--bg-2);
  overflow: hidden;
}

.chatgpt-usage-bar-fill {
  height: 100%;
  background: var(--accent, var(--ink-3));
  transition: width 200ms ease;
}

/* Exhausted bucket gets a warning-tinted fill so the user notices
 * the cause without having to read the percentage. */
.chatgpt-usage-bar.exhausted .chatgpt-usage-bar-fill {
  background: var(--err, #d94343);
}

.chatgpt-manual-paste {
  margin-top: 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.chatgpt-manual-paste-input {
  width: 100%;
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: 12px;
  padding: 8px 10px;
  border: 1px solid var(--border-1);
  border-radius: 6px;
  background: var(--bg-1);
  color: var(--ink-1);
  resize: vertical;
}

.chatgpt-manual-paste-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
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

.language-select {
  flex-shrink: 0;
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

.card-btn {
  /* Variant styles below extend the shared AppButton visual language. */
  height: 28px;
  padding: 0 8px;
  border: 1px solid var(--rule);
  background: var(--panel-2);
  color: var(--ink-2);
  cursor: pointer;
  font-family: var(--mono);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  transition: transform .12s, box-shadow .12s, background .12s, color .12s, border-color .12s;
}

.card-btn:hover:not(:disabled) {
  transform: translate(-1px, -1px);
  box-shadow: 2px 2px 0 0 var(--rule);
}

.card-btn:active:not(:disabled) {
  transform: translate(0, 0);
  box-shadow: none;
}

.card-btn:disabled {
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
  color: var(--ok);
  font-weight: 500;
}

.provider-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  flex-wrap: wrap;
}

.provider-actions :deep(.app-btn--compact) {
  padding: 0 8px;
}

.card-btn.danger {
  color: var(--err);
  border-color: color-mix(in srgb, var(--err) 40%, var(--rule));
}

.card-btn.danger:not(:disabled) {
  background: color-mix(in srgb, var(--err) 10%, transparent);
}

.card-btn.danger:hover:not(:disabled) {
  background: color-mix(in srgb, var(--err) 18%, transparent);
}

.card-btn.accent {
  background: transparent;
  border-color: var(--info);
  color: var(--info);
}

.card-btn.accent:hover:not(:disabled) {
  background: color-mix(in srgb, var(--info) 12%, transparent);
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
  padding: 14px;
  /* Bold ink border to signal "this is the active editing surface".
   * Slightly darker background lifts it off the surrounding section. */
  border: 2px solid var(--ink);
  background: color-mix(in srgb, var(--panel-2) 60%, var(--panel));
  animation: provider-editor-enter 220ms cubic-bezier(.2,.7,.2,1);
}

@keyframes provider-editor-enter {
  from { opacity: 0.6; }
  to { opacity: 1; }
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

.provider-select-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  cursor: pointer;
  text-align: left;
  font-family: inherit;
}

.provider-select-trigger.open,
.provider-select-trigger:focus-visible {
  border-color: var(--accent);
}

.provider-select-trigger .select-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.provider-select-trigger .chevron {
  flex: 0 0 auto;
  width: 18px;
  height: 18px;
  color: var(--ink-4);
  transition: transform 150ms ease;
}

.provider-select-trigger.open .chevron {
  transform: rotate(180deg);
}

.provider-editor-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

@media (max-width: 760px) {
  .setting-row,
  .section-head,
  .provider-card-head {
    flex-direction: column;
    align-items: stretch;
  }

  .language-select {
    width: 100%;
  }

  .providers-grid,
  .provider-form-grid,
  .provider-meta {
    grid-template-columns: 1fr;
  }
}
</style>
