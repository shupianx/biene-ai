<template>
  <div
    ref="cardRef"
    class="card"
    :class="statusTone"
    @mouseenter="hover = true"
    @mouseleave="hover = false"
    @click="emit('select')"
  >
    <!-- Top strip: name + menu -->
    <div class="top-strip">
      <div class="title-row">
        <div class="name" :title="session.meta.name">{{ session.meta.name }}</div>
        <div class="title-path" :title="session.meta.work_dir">
          <svg class="path-icon" viewBox="0 0 24 24" aria-hidden="true" v-html="folderIconBody" />
          <span class="path-text">{{ shortDir }}</span>
        </div>
      </div>
      <div class="menu-wrap" @click.stop>
        <button
          class="menu-btn"
          :class="{ visible: hover || menuOpen, open: menuOpen }"
          :title="t('common.more')"
          :aria-label="t('common.more')"
          @click="menuOpen = !menuOpen"
        >
          <svg viewBox="0 0 24 24" aria-hidden="true" v-html="moreIconBody" />
        </button>
        <div v-if="menuOpen" class="menu">
          <button class="menu-item" @click="onOpen">{{ t('grid.openMenu') }}</button>
          <button class="menu-item" @click="onOpenFolder">{{ t('grid.openFolderMenu') }}</button>
          <button class="menu-item" @click="onSettings">{{ t('common.settings') }}</button>
          <div class="menu-sep" aria-hidden="true" />
          <button class="menu-item danger" @click="onDelete">{{ t('common.delete') }}</button>
        </div>
      </div>
    </div>

    <!-- Body -->
    <div class="body">
      <div v-if="pendingPermLine" class="warn-line">⚠ {{ pendingPermLine }}</div>

      <div class="meta-row">
        <div class="status-tag" :class="statusTone">
          <span class="status-dot" />
          <span>{{ statusLabel }}</span>
        </div>
        <div v-if="session.activeSkillName" class="skill-tag">
          ⚡ {{ session.activeSkillName }}
        </div>
        <div class="updated">{{ updatedAt }}</div>
      </div>
    </div>

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
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import type { AgentSession } from '../stores/sessions'
import { t } from '../i18n'
import { getSessionStatusLabel, getSessionStatusTone } from '../utils/sessionStatus'
import { findDomainOption, findStyleOption } from '../utils/profile'
import { getPermissionLabel } from '../utils/permissions'
import { formatMessageTime } from '../utils/messageTime'

const props = defineProps<{ session: AgentSession }>()
const emit = defineEmits<{
  (e: 'select'): void
  (e: 'open-folder'): void
  (e: 'settings'): void
  (e: 'delete'): void
}>()

const folderIconBody = '<path fill="currentColor" d="M10 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z"/>'
const moreIconBody = '<path fill="currentColor" d="M6 14q-.825 0-1.412-.587T4 12t.588-1.412T6 10t1.413.588T8 12t-.587 1.413T6 14m6 0q-.825 0-1.412-.587T10 12t.588-1.412T12 10t1.413.588T14 12t-.587 1.413T12 14m6 0q-.825 0-1.412-.587T16 12t.588-1.412T18 10t1.413.588T20 12t-.587 1.413T18 14"/>'

const hover = ref(false)
const menuOpen = ref(false)
const cardRef = ref<HTMLElement | null>(null)

const statusTone = computed(() => getSessionStatusTone(props.session))
const statusLabel = computed(() => getSessionStatusLabel(statusTone.value))

const shortDir = computed(() => {
  const d = props.session.meta.work_dir
  const parts = d.split('/')
  if (parts.length <= 2) return d
  return '…/' + parts.slice(-2).join('/')
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

function onOpen() {
  menuOpen.value = false
  emit('select')
}

function onSettings() {
  menuOpen.value = false
  emit('settings')
}

function onOpenFolder() {
  menuOpen.value = false
  emit('open-folder')
}

function onDelete() {
  menuOpen.value = false
  emit('delete')
}

function handlePointerDown(event: MouseEvent) {
  if (!menuOpen.value) return
  if (cardRef.value?.contains(event.target as Node)) return
  menuOpen.value = false
}

onMounted(() => document.addEventListener('pointerdown', handlePointerDown))
onBeforeUnmount(() => document.removeEventListener('pointerdown', handlePointerDown))
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
  font-size: 17px;
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

.menu-wrap {
  position: relative;
}

.menu-btn {
  width: 24px;
  height: 24px;
  border: none;
  background: transparent;
  color: var(--ink-4);
  cursor: pointer;
  display: grid;
  place-items: center;
  opacity: 0;
  transition: opacity .12s, background .12s, color .12s;
}

.menu-btn.visible,
.menu-btn:focus-visible {
  opacity: 1;
}

.menu-btn.open {
  background: var(--bg-2);
}

.menu-btn:hover {
  color: var(--ink-2);
  background: var(--bg-2);
}

.menu-btn svg {
  width: 14px;
  height: 14px;
}

.menu {
  position: absolute;
  top: 30px;
  right: 0;
  min-width: 144px;
  padding: 4px;
  background: var(--panel-2);
  border: 1px solid var(--rule);
  box-shadow: 3px 3px 0 0 var(--rule);
  z-index: 10;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.menu-item {
  border: none;
  background: transparent;
  text-align: left;
  padding: 6px 10px;
  font-family: var(--sans);
  font-size: 12px;
  color: var(--ink-2);
  cursor: pointer;
}

.menu-item:hover {
  background: var(--bg-2);
}

.menu-item.danger {
  color: var(--err);
}

.menu-item.danger:hover {
  background: var(--err);
  color: var(--panel-2);
}

.menu-sep {
  height: 1px;
  background: var(--rule-softer);
  margin: 4px 2px;
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
  gap: 6px;
  min-width: 0;
  flex: 1 1 auto;
  font-family: var(--mono);
  font-size: 11px;
  color: var(--ink-4);
}

.path-icon {
  width: 12px;
  height: 12px;
  flex-shrink: 0;
}

.path-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.warn-line {
  font-family: var(--mono);
  font-size: 11.5px;
  color: var(--warn);
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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

.skill-tag {
  font-family: var(--mono);
  font-size: 10px;
  letter-spacing: 0.1em;
  color: var(--accent);
  border: 1px solid var(--accent);
  padding: 2px 6px;
  white-space: nowrap;
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
