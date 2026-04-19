<template>
  <section class="skills-browser" :class="{ embedded }">
    <header class="skills-topbar">
      <div class="skills-topbar-actions">
        <button class="skills-action skills-action-import" type="button" :disabled="importing" @click="onImport">
          {{ importing ? t('skillsBrowser.importing') : t('skillsBrowser.import') }}
        </button>
        <button v-if="closable" class="skills-action skills-action-close" type="button" @click="emit('close')">
          {{ t('common.close') }}
        </button>
      </div>
      <div class="skills-topbar-path-row">
        <code class="skills-topbar-path">{{ catalog?.root || rootFallback }}</code>
        <button
          class="skills-folder-btn"
          type="button"
          :title="t('grid.openFolderMenu')"
          :aria-label="t('grid.openFolderMenu')"
          @click="onOpenFolder"
        >
          <FolderOpenIcon class="skills-folder-icon" />
        </button>
      </div>
    </header>

    <p v-if="error" class="skills-error">{{ error }}</p>
    <p v-else-if="loading" class="skills-state">{{ t('skillsBrowser.loading') }}</p>

    <div v-else-if="!catalog || catalog.skills.length === 0" class="skills-empty">
      <span class="skills-empty-icon" aria-hidden="true"></span>
      <strong>{{ t('skillsBrowser.emptyTitle') }}</strong>
      <span>{{ t('skillsBrowser.emptyHint') }}</span>
    </div>

    <div v-else class="skills-layout">
      <div class="skills-list" aria-label="Skills">
        <div
          v-for="skill in catalog.skills"
          :key="skill.file_path"
          class="skill-list-item"
        >
          <span class="skill-list-name">{{ skill.name }}</span>
          <div class="skill-list-actions">
            <button
              class="skill-action skill-action-default"
              :class="{ 'is-active': isDefaultEnabledSkill(skill) }"
              type="button"
              :title="t('skillsBrowser.defaultEnabledLabel')"
              :aria-label="t('skillsBrowser.defaultEnabledLabel')"
              :disabled="busySkillDir === skill.dir"
              @click="onToggleDefaultEnabled(skill)"
            >
              <DefaultOnIcon v-if="isDefaultEnabledSkill(skill)" class="skill-action-icon" />
              <DefaultOffIcon v-else class="skill-action-icon" />
            </button>
            <div class="skill-delete-shell">
              <button
                class="skill-action skill-action-delete"
                type="button"
                :title="t('skillsBrowser.deleteLabel')"
                :aria-label="t('skillsBrowser.deleteLabel')"
                :disabled="busySkillDir === skill.dir"
                @click="onToggleDeletePopover(skill)"
              >
                <DeleteIcon class="skill-action-icon" />
              </button>
              <div
                v-if="deleteConfirmSkillDir === skill.dir"
                class="skill-delete-popover"
              >
                <button
                  class="skill-delete-popover-btn is-danger"
                  type="button"
                  :disabled="busySkillDir === skill.dir"
                  @click="onDeleteSkill(skill)"
                >
                  {{ t('skillsBrowser.confirmDeleteLabel') }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import FolderOpenIcon from '~icons/material-symbols/folder-open-outline'
import DeleteIcon from '~icons/material-symbols/delete-forever-outline-sharp'
import DefaultOffIcon from '~icons/tabler/file'
import DefaultOnIcon from '~icons/tabler/file-power'
import { listSkills, type SkillCatalogEntry, type SkillsCatalog } from '../api/http'
import { t } from '../i18n'
import { getDesktopBridge, type SkillConfig } from '../runtime'

const props = withDefaults(defineProps<{
  embedded?: boolean
  closable?: boolean
}>(), {
  embedded: false,
  closable: false,
})

const emit = defineEmits<{
  (e: 'close'): void
}>()

const catalog = ref<SkillsCatalog | null>(null)
const loading = ref(false)
const importing = ref(false)
const error = ref('')
const busySkillDir = ref('')
const deleteConfirmSkillDir = ref('')
const rootFallback = '~/.biene/skills'

function defaultSkillConfig(): SkillConfig {
  return {
    defaultEnabledSkillDirs: [],
  }
}

const skillConfig = ref<SkillConfig>(defaultSkillConfig())

function isDefaultEnabledSkill(skill: SkillCatalogEntry) {
  return skillConfig.value.defaultEnabledSkillDirs.includes(skill.dir)
}

async function loadSkillConfig() {
  const bridge = getDesktopBridge()
  if (!bridge?.getSkillConfig) {
    skillConfig.value = defaultSkillConfig()
    return
  }

  skillConfig.value = await bridge.getSkillConfig()
}

async function updateSkillConfig(patch: Partial<SkillConfig>) {
  const bridge = getDesktopBridge()
  if (!bridge?.updateSkillConfig) {
    skillConfig.value = {
      ...skillConfig.value,
      ...patch,
    }
    return
  }

  skillConfig.value = await bridge.updateSkillConfig(patch)
}

async function loadSkills() {
  loading.value = true
  error.value = ''
  try {
    catalog.value = await listSkills()
  } catch (nextError) {
    error.value = nextError instanceof Error ? nextError.message : String(nextError)
  } finally {
    loading.value = false
  }
}

async function onImport() {
  const bridge = getDesktopBridge()
  if (!bridge?.importSkillFolder) return

  importing.value = true
  error.value = ''
  try {
    const importedCount = await bridge.importSkillFolder()
    if (importedCount > 0) {
      await loadSkills()
    }
  } catch (nextError) {
    error.value = nextError instanceof Error ? nextError.message : String(nextError)
  } finally {
    importing.value = false
  }
}

async function onOpenFolder() {
  const bridge = getDesktopBridge()
  if (!bridge?.openPath) return

  try {
    await bridge.openPath(catalog.value?.root || rootFallback)
  } catch (nextError) {
    error.value = nextError instanceof Error ? nextError.message : String(nextError)
  }
}

async function onToggleDefaultEnabled(skill: SkillCatalogEntry) {
  busySkillDir.value = skill.dir
  error.value = ''
  try {
    const next = new Set(skillConfig.value.defaultEnabledSkillDirs)
    if (next.has(skill.dir)) {
      next.delete(skill.dir)
    } else {
      next.add(skill.dir)
    }

    await updateSkillConfig({
      defaultEnabledSkillDirs: [...next],
    })
  } catch (nextError) {
    error.value = nextError instanceof Error ? nextError.message : String(nextError)
  } finally {
    busySkillDir.value = ''
  }
}

async function onDeleteSkill(skill: SkillCatalogEntry) {
  const bridge = getDesktopBridge()
  if (!bridge?.deleteSkill) return

  busySkillDir.value = skill.dir
  error.value = ''
  try {
    skillConfig.value = await bridge.deleteSkill(skill.dir)
    deleteConfirmSkillDir.value = ''
    await loadSkills()
  } catch (nextError) {
    error.value = nextError instanceof Error ? nextError.message : String(nextError)
  } finally {
    busySkillDir.value = ''
  }
}

function onToggleDeletePopover(skill: SkillCatalogEntry) {
  deleteConfirmSkillDir.value = deleteConfirmSkillDir.value === skill.dir ? '' : skill.dir
}

function onDocumentPointerDown(event: PointerEvent) {
  const target = event.target
  if (!(target instanceof Element)) {
    deleteConfirmSkillDir.value = ''
    return
  }
  if (target.closest('.skill-delete-shell')) return
  deleteConfirmSkillDir.value = ''
}

onMounted(() => {
  document.addEventListener('pointerdown', onDocumentPointerDown)
  void Promise.all([loadSkills(), loadSkillConfig()])
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onDocumentPointerDown)
})
</script>

<style scoped>
.skills-browser {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px;
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--panel) 88%, transparent), transparent 42%),
    var(--bg);
  color: var(--ink);
}

.skills-browser.embedded {
  padding: 12px;
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--panel) 84%, transparent), transparent 46%),
    var(--panel);
}

.skills-topbar {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
}

.skills-topbar-path-row {
  display: flex;
  align-items: center;
  gap: 8px;
  width: min(100%, 28rem);
  padding: 8px 10px;
  border: 1px solid var(--rule-soft);
  background: var(--panel);
}

.skills-topbar-path {
  min-width: 0;
  flex: 1;
  font-family: var(--mono);
  font-size: 12px;
  line-height: 1.45;
  color: var(--ink-2);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.skills-topbar-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  justify-content: flex-end;
}

.skills-folder-btn {
  width: 24px;
  height: 24px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border: 1px solid var(--rule-soft);
  background: var(--panel-2);
  color: var(--ink-3);
  cursor: pointer;
  transition: background .14s, color .14s, border-color .14s;
}

.skills-folder-btn:hover {
  background: var(--bg-2);
  border-color: var(--rule);
  color: var(--ink);
}

.skills-folder-icon {
  width: 15px;
  height: 15px;
}

.skills-action {
  height: 24px;
  padding: 0 10px;
  border: 1px solid var(--rule-soft);
  background: var(--panel-2);
  color: var(--ink-3);
  cursor: pointer;
  font-family: var(--mono);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  transition: background .14s, color .14s, border-color .14s, opacity .14s;
}

.skills-action:hover:not(:disabled) {
  background: var(--bg-2);
  border-color: var(--rule);
  color: var(--ink);
}

.skills-action-import {
  border-color: color-mix(in srgb, #68a8b4 34%, var(--rule-soft));
  background: color-mix(in srgb, #68a8b4 14%, var(--panel-2));
  color: color-mix(in srgb, #2f6172 72%, var(--ink));
}

.skills-action-import:hover:not(:disabled) {
  border-color: color-mix(in srgb, #68a8b4 48%, var(--rule));
  background: color-mix(in srgb, #68a8b4 22%, var(--bg-2));
  color: color-mix(in srgb, #1f4c5d 82%, var(--ink));
}

.skills-action-close {
  border-color: color-mix(in srgb, #b79a82 30%, var(--rule-soft));
  background: color-mix(in srgb, #b79a82 10%, var(--panel-2));
  color: color-mix(in srgb, #7e644d 66%, var(--ink));
}

.skills-action-close:hover:not(:disabled) {
  border-color: color-mix(in srgb, #b79a82 42%, var(--rule));
  background: color-mix(in srgb, #b79a82 16%, var(--bg-2));
  color: color-mix(in srgb, #614634 76%, var(--ink));
}

.skills-action:disabled {
  opacity: 0.6;
  cursor: progress;
}

.skills-state,
.skills-error {
  padding: 10px 12px;
  border: 1px solid var(--rule-softer);
  background: var(--panel);
}

.skills-state,
.skills-error {
  margin: 0;
  font-size: 12px;
  line-height: 1.5;
}

.skills-state {
  color: var(--ink-4);
  border-style: dashed;
}

.skills-error {
  color: var(--err);
  border-color: color-mix(in srgb, var(--err) 28%, transparent);
  background: color-mix(in srgb, var(--err) 7%, var(--panel));
}

.skills-empty {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 24px;
  border: 1px dashed var(--rule-soft);
  background: color-mix(in srgb, var(--panel) 90%, transparent);
  text-align: center;
}

.skills-empty-icon {
  --skills-book-mask: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24'%3E%3Cpath fill='black' d='M5 16.175q.25-.075.488-.125T6 16h1V4H6q-.425 0-.712.288T5 5zM6 22q-1.25 0-2.125-.875T3 19V5q0-1.25.875-2.125T6 2h7v2H9v12h6v-3h2v5H6q-.425 0-.712.288T5 19t.288.713T6 20h13v-8h2v10zm-1-5.825V4zM17.5 12q0-2.3 1.6-3.9T23 6.5q-2.3 0-3.9-1.6T17.5 1q0 2.3-1.6 3.9T12 6.5q2.3 0 3.9 1.6t1.6 3.9'/%3E%3C/svg%3E");
  position: relative;
  width: 26px;
  height: 26px;
  display: block;
  background:
    radial-gradient(circle at 18% 16%, rgba(240, 229, 207, 0.56) 0%, rgba(240, 229, 207, 0.1) 22%, transparent 46%),
    radial-gradient(circle at 80% 18%, rgba(145, 223, 207, 0.2) 0%, rgba(145, 223, 207, 0.08) 26%, transparent 54%),
    linear-gradient(136deg, #d7b7b2 0%, #c7a7c4 18%, #ada4d3 34%, #72c0d0 52%, #8bc59a 70%, #d7b37d 84%, #d1aeb7 100%);
  background-size: 100% 100%, 100% 100%, 172% 172%;
  background-position: center, center, -8% -8%;
  -webkit-mask: var(--skills-book-mask) center / contain no-repeat;
  mask: var(--skills-book-mask) center / contain no-repeat;
  filter:
    drop-shadow(0 0 7px rgba(128, 116, 184, 0.14))
    drop-shadow(0 0 14px rgba(91, 176, 187, 0.08));
  animation: enchantedBookFlow 8.8s linear infinite alternate;
}

.skills-empty-icon::before,
.skills-empty-icon::after {
  content: '';
  position: absolute;
  inset: 0;
  -webkit-mask: var(--skills-book-mask) center / contain no-repeat;
  mask: var(--skills-book-mask) center / contain no-repeat;
  pointer-events: none;
}

.skills-empty-icon::before {
  inset: -12px;
  border-radius: 999px;
  background:
    radial-gradient(circle at 50% 50%, rgba(166, 145, 212, 0.14) 0%, rgba(117, 195, 188, 0.08) 34%, transparent 72%);
  filter: blur(7px);
  animation: enchantedBookPulse 4.2s ease-in-out infinite alternate;
}

.skills-empty-icon::after {
  background:
    linear-gradient(135deg, transparent 18%, rgba(255, 250, 243, 0.03) 34%, rgba(234, 205, 220, 0.1) 42%, rgba(255, 255, 255, 0.34) 50%, rgba(188, 226, 217, 0.12) 58%, rgba(231, 216, 181, 0.06) 68%, transparent 84%);
  background-size: 220% 220%;
  background-position: -42% -42%;
  mix-blend-mode: screen;
  opacity: 0.16;
  animation: enchantedBookShimmer 4.8s ease-in-out infinite;
}

.skills-empty strong {
  font-family: var(--mono);
  font-size: 12px;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.skills-empty span {
  max-width: 28rem;
  font-size: 12px;
  line-height: 1.55;
  color: var(--ink-3);
}

@keyframes enchantedBookFlow {
  0% {
    background-position: center, -8% -8%;
  }

  100% {
    background-position: center, 108% 108%;
  }
}

@keyframes enchantedBookPulse {
  0%, 100% {
    opacity: 0.72;
    transform: scale(0.96);
  }

  50% {
    opacity: 1;
    transform: scale(1.08);
  }
}

@keyframes enchantedBookShimmer {
  0% {
    background-position: -42% -42%;
    opacity: 0.14;
  }

  50% {
    opacity: 0.34;
  }

  100% {
    background-position: 128% 128%;
    opacity: 0.16;
  }
}

.skills-layout {
  flex: 1;
  min-height: 0;
  display: flex;
}

.skills-list {
  min-height: 0;
  border: 1px solid var(--rule-softer);
  background: var(--panel);
}

.skills-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px;
  flex: 1;
  overflow: auto;
}

.skill-list-item {
  min-height: 42px;
  padding: 6px 8px 6px 10px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  border: 1px solid var(--rule-softer);
  background: color-mix(in srgb, var(--panel) 90%, var(--bg));
  color: inherit;
  text-align: left;
  transition: background .14s, border-color .14s;
}

.skill-list-item:hover {
  background: color-mix(in srgb, var(--panel-2) 92%, var(--bg));
  border-color: var(--rule-soft);
}

.skill-list-name {
  min-width: 0;
  flex: 1;
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

.skill-list-actions {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
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

.skill-action-default {
  color: var(--ink-4);
}

.skill-action-default:hover:not(:disabled),
.skill-action-default.is-active {
  background: color-mix(in srgb, #76a9d8 18%, transparent);
  color: color-mix(in srgb, #2a6aa3 82%, var(--ink));
}

.skill-action-delete {
  color: var(--ink-4);
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

.skill-delete-popover-btn:hover:not(:disabled) {
  background: var(--bg-2);
  color: var(--ink);
}

.skill-delete-popover-btn.is-danger:hover:not(:disabled) {
  background: color-mix(in srgb, var(--err) 14%, transparent);
  color: color-mix(in srgb, var(--err) 82%, var(--ink));
}

.skill-delete-popover-btn:disabled {
  opacity: 0.55;
  cursor: progress;
}
</style>
