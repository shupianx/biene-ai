<template>
  <div class="input-bar">
    <div class="composer" :class="{ disabled, focused: focused }">
      <div v-if="stagedImages.length" class="attachment-tray">
        <div
          v-for="img in stagedImages"
          :key="img.id"
          class="attachment-chip"
        >
          <img :src="img.previewUrl" :alt="img.file.name" />
          <button
            class="attachment-remove"
            type="button"
            :aria-label="t('input.removeImage')"
            :title="t('input.removeImage')"
            @click="removeImage(img.id)"
          >
            <span aria-hidden="true">×</span>
          </button>
        </div>
      </div>
      <div
        ref="editorRef"
        class="editor"
        :class="{ empty: isEmpty }"
        :contenteditable="!disabled"
        role="textbox"
        :aria-disabled="disabled ? 'true' : 'false'"
        :data-placeholder="t('input.placeholder')"
        @input="onInput"
        @keydown="onKeydown"
        @focus="focused = true"
        @blur="onBlur"
        @paste="onPaste"
        @compositionstart="onCompositionStart"
        @compositionend="onCompositionEnd"
      />
      <div class="composer-actions">
        <IconButton
          class="attach-btn"
          :class="{ unsupported: imagesAvailable === false }"
          :disabled="disabled"
          :aria-label="attachImageLabel"
          :title="attachImageLabel"
          @click="openFilePicker"
        >
          <MaterialSymbolsImageOutline class="attach-icon" aria-hidden="true" />
        </IconButton>
        <input
          v-if="imagesAvailable !== false"
          ref="fileInputRef"
          class="file-input"
          type="file"
          accept="image/*"
          multiple
          @change="onFileInputChange"
        />
        <div v-if="thinkingAvailable" class="thinking-control">
          <span class="thinking-label">{{ t('input.thinkingToggle') }}</span>
          <ToggleSwitch
            :model-value="thinkingEnabled"
            :label="t('input.thinkingToggle')"
            @update:model-value="emit('update:thinkingEnabled', $event)"
          />
        </div>
        <button
          class="action-btn"
          :class="{ interrupt: interruptible }"
          :aria-label="buttonTitle"
          :disabled="buttonDisabled"
          :title="buttonTitle"
          @click="handleAction"
        >
          <MynauiSend
            v-if="!interruptible"
            class="send-icon"
            aria-hidden="true"
          />
          <span v-else-if="interrupting" class="interrupt-spinner" aria-hidden="true" />
          <svg
            v-else
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="currentColor"
            aria-hidden="true"
          >
            <rect x="6" y="6" width="12" height="12" />
          </svg>
          <span class="action-label">{{ actionLabel }}</span>
        </button>
      </div>
    </div>
    <AgentMentionMenu
      :visible="agentMenuOpen"
      :candidates="filteredAgents"
      :selected-index="selectedCandidateIndex"
      :position="mentionPosition"
      @pick="pickAgent"
      @hover="selectedCandidateIndex = $event"
    />
    <SlashMenu
      :visible="slashMenuOpen"
      :groups="slashGroups"
      :selected-index="selectedCandidateIndex"
      :position="mentionPosition"
      @pick="pickSlashItem"
      @hover="selectedCandidateIndex = $event"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import MynauiSend from '~icons/mynaui/send'
import MaterialSymbolsImageOutline from '~icons/material-symbols/image-outline'
import IconButton from '../ui/IconButton.vue'
import ToggleSwitch from '../ui/ToggleSwitch.vue'
import AgentMentionMenu, { type MentionCandidate } from './AgentMentionMenu.vue'
import SlashMenu, { type SlashItem } from './SlashMenu.vue'
import { chipToInlineText, type TokenKind } from '../../utils/mentions'
import { builtinCommands, parseCommandLine } from '../../commands'
import { t } from '../../i18n'

interface StagedImage {
  id: string
  file: File
  previewUrl: string
}

const props = defineProps<{
  disabled?: boolean
  interruptible?: boolean
  interrupting?: boolean
  thinkingAvailable?: boolean
  thinkingEnabled?: boolean
  // Whether the active model accepts image inputs. Defaults to true; only
  // an explicit `false` (declared via the provider template's
  // `images_available` flag) hides the attach control and silently drops
  // pasted images.
  imagesAvailable?: boolean
  mentionCandidates?: MentionCandidate[]
  skillCandidates?: MentionCandidate[]
}>()
const emit = defineEmits<{
  (e: 'send', payload: { text: string; files: File[] }): void
  (e: 'command', payload: { id: string; args: string }): void
  (e: 'update:thinkingEnabled', value: boolean): void
  (e: 'interrupt'): void
}>()

const editorRef = ref<HTMLDivElement | null>(null)
const fileInputRef = ref<HTMLInputElement | null>(null)
const stagedImages = ref<StagedImage[]>([])
const isComposing = ref(false)
const focused = ref(false)
const isEmpty = ref(true)
let compositionLockedUntil = 0

// ── Popup state ───────────────────────────────────────────────────────────
// Two triggers, two distinct menu components (AgentMentionMenu for `@`,
// SlashMenu for `/`). They share the trigger-scanning + positioning +
// keyboard-nav infrastructure but render different items and have
// different pick semantics:
//   - Agent / Skill picks become inline chips embedded in the message.
//   - Command picks either execute immediately (no args) or auto-complete
//     `/command-id ` into the editor (has args).

type TriggerKind = 'agent' | 'slash'

interface ActiveQuery {
  kind: TriggerKind
  textNode: Text
  atIdx: number       // index of the trigger character in the text node
  caretOffset: number // current cursor offset within the text node
  query: string       // what the user has typed after the trigger
}

const activeQuery = ref<ActiveQuery | null>(null)
const agentMenuOpen = computed(() => activeQuery.value?.kind === 'agent')
const slashMenuOpen = computed(() => activeQuery.value?.kind === 'slash')
const selectedCandidateIndex = ref(0)
const mentionPosition = ref({ left: 0, top: 0 })

// Agent menu candidates — substring match on name OR id.
const filteredAgents = computed<MentionCandidate[]>(() => {
  const q = activeQuery.value
  if (q?.kind !== 'agent') return []
  const all = props.mentionCandidates ?? []
  const needle = q.query.trim().toLowerCase()
  if (!needle) return all
  return all.filter(c =>
    c.name.toLowerCase().includes(needle) || c.id.toLowerCase().includes(needle),
  )
})

// Slash menu groups — commands first, then skills. Filter applies
// across both (substring match on name OR id OR description).
const slashGroups = computed(() => {
  const q = activeQuery.value
  if (q?.kind !== 'slash') return []
  const needle = q.query.trim().toLowerCase()

  const commands: SlashItem[] = builtinCommands
    .map((c): SlashItem => ({
      kind: 'command',
      id: c.id,
      name: t(c.nameKey),
      description: t(c.descriptionKey),
      hasArgs: c.hasArgs,
    }))
    .filter(c => matches(c, needle))

  const skills: SlashItem[] = (props.skillCandidates ?? [])
    .map((s): SlashItem => ({
      kind: 'skill',
      id: s.id,
      name: s.name,
      description: '',
    }))
    .filter(s => matches(s, needle))

  return [
    { kind: 'command' as const, label: t('input.slash.commandsLabel'), items: commands },
    { kind: 'skill' as const, label: t('input.slash.skillsLabel'), items: skills },
  ]
})

function matches(item: SlashItem, needle: string): boolean {
  if (!needle) return true
  return (
    item.name.toLowerCase().includes(needle) ||
    item.id.toLowerCase().includes(needle) ||
    item.description.toLowerCase().includes(needle)
  )
}

// Flat list used by ↑/↓ navigation. Must traverse groups in the same
// order SlashMenu renders them so SlashMenu.flatIndex agrees.
const flatSlashItems = computed<SlashItem[]>(() =>
  slashGroups.value.flatMap(g => g.items),
)

const totalCandidates = computed(() => {
  if (activeQuery.value?.kind === 'agent') return filteredAgents.value.length
  if (activeQuery.value?.kind === 'slash') return flatSlashItems.value.length
  return 0
})

watch(totalCandidates, (count) => {
  if (selectedCandidateIndex.value >= count) {
    selectedCandidateIndex.value = 0
  }
})

// ── Button state ──────────────────────────────────────────────────────────

const buttonDisabled = computed(() => {
  if (props.interruptible) {
    return Boolean(props.interrupting)
  }
  if (props.disabled) return true
  return isEmpty.value && stagedImages.value.length === 0
})

const buttonTitle = computed(() =>
  props.interruptible
    ? (props.interrupting ? t('input.interrupting') : t('input.interrupt'))
    : t('input.send')
)

const actionLabel = computed(() => {
  if (props.interruptible) {
    return props.interrupting ? t('input.interrupting') : t('input.interrupt')
  }
  return t('input.send')
})

const attachImageLabel = computed(() =>
  props.imagesAvailable === false
    ? t('input.attachImageUnsupported')
    : t('input.attachImage')
)

// ── Composition (IME) ─────────────────────────────────────────────────────

function onBlur() {
  focused.value = false
  // Allow a pick via mousedown (preventDefault keeps focus) to commit before
  // we tear down the menu; otherwise a real focus loss closes it.
  window.setTimeout(() => {
    if (document.activeElement !== editorRef.value) closeMention()
  }, 100)
}

function onCompositionStart() {
  isComposing.value = true
}

function onCompositionEnd() {
  isComposing.value = false
  compositionLockedUntil = Date.now() + 30
  // IME commit fires a composition text insertion; re-scan for mention.
  scanMentionQuery()
  updateEmpty()
}

// ── Editor events ─────────────────────────────────────────────────────────

function onInput() {
  updateEmpty()
  if (isComposing.value) return
  scanMentionQuery()
}

function onKeydown(event: KeyboardEvent) {
  if (isComposing.value) return
  if (Date.now() < compositionLockedUntil) return

  const popupOpen = agentMenuOpen.value || slashMenuOpen.value
  if (popupOpen) {
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      moveSelection(1)
      return
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault()
      moveSelection(-1)
      return
    }
    if (event.key === 'Escape') {
      event.preventDefault()
      closeMention()
      return
    }
    if (event.key === 'Enter' || event.key === 'Tab') {
      const picked = pickAtIndex(selectedCandidateIndex.value)
      if (picked) {
        event.preventDefault()
        return
      }
      // no candidates: fall through to normal behavior
    }
  }

  if (event.key === 'Enter' && !event.shiftKey && !event.metaKey && !event.ctrlKey) {
    if (props.interruptible) {
      event.preventDefault()
      return
    }
    event.preventDefault()
    submit()
    return
  }
  if (event.key === 'Enter' && event.shiftKey) {
    event.preventDefault()
    insertTextAtCaret('\n')
    updateEmpty()
    scanMentionQuery()
  }
}

// ── Mention query detection ───────────────────────────────────────────────

function scanMentionQuery() {
  const sel = window.getSelection()
  if (!sel || sel.rangeCount === 0 || !editorRef.value) {
    closeMention()
    return
  }
  const range = sel.getRangeAt(0)
  if (!range.collapsed) {
    closeMention()
    return
  }
  const anchor = range.startContainer
  if (!editorRef.value.contains(anchor) || anchor.nodeType !== Node.TEXT_NODE) {
    closeMention()
    return
  }
  const textNode = anchor as Text
  const caretOffset = range.startOffset
  const before = textNode.data.slice(0, caretOffset)
  const trigger = findActiveTrigger(before)
  if (!trigger) {
    closeMention()
    return
  }
  const query = before.slice(trigger.idx + 1)
  activeQuery.value = { kind: trigger.kind, textNode, atIdx: trigger.idx, caretOffset, query }
  updateMentionPosition()
}

// Scans backward from the caret for the nearest active trigger character.
// Rules:
//   '@' triggers anywhere (a name may include no preceding space, e.g.
//   "foo@bar" — matching was user-requested for agent mentions).
//   '/' triggers only at word boundaries so paths like "src/utils/foo"
//   don't open the slash menu on every slash.
// Returns null once whitespace between trigger and caret ends the query.
function findActiveTrigger(text: string): { kind: TriggerKind; idx: number } | null {
  for (let i = text.length - 1; i >= 0; i--) {
    const ch = text[i]
    if (ch === '@') return { kind: 'agent', idx: i }
    if (ch === '/') {
      if (i === 0 || /\s/.test(text[i - 1])) return { kind: 'slash', idx: i }
      return null
    }
    if (/\s/.test(ch)) return null
  }
  return null
}

// Per-trigger menu dimensions — must stay in sync with each menu
// component's CSS. Numbers are approximations used to decide whether
// the menu fits below the caret before it is actually painted, which
// avoids a post-paint reposition jitter.
const MENU_GAP = 4
const MENU_VIEWPORT_MARGIN = 8

interface MenuDims {
  width: number          // typical width used to clamp horizontal position
  itemHeight: number     // per-row height
  groupOverhead: number  // total vertical space group labels + dividers add (slash only)
  vpad: number           // top/bottom container padding
  maxHeight: number      // CSS max-height ceiling
  emptyHeight: number    // height shown when there are no candidates
}

const AGENT_MENU_DIMS: MenuDims = {
  width: 260,
  itemHeight: 32,
  groupOverhead: 0,
  vpad: 8,
  maxHeight: 240,
  emptyHeight: 40,
}

const SLASH_MENU_DIMS: MenuDims = {
  // SlashMenu CSS: min-width 180, max-width 240. Use the upper bound so
  // horizontal clamp guarantees the menu never overflows the viewport.
  width: 240,
  itemHeight: 32,
  // Each group renders a 20px label + 8px dashed divider (~28px). Two
  // groups = ~48px. Reserve regardless of whether both have items, so
  // the flip-up decision stays stable while the user types and groups
  // appear/disappear.
  groupOverhead: 48,
  vpad: 8,
  maxHeight: 480,
  emptyHeight: 40,
}

function activeMenuDims(): MenuDims {
  return activeQuery.value?.kind === 'slash' ? SLASH_MENU_DIMS : AGENT_MENU_DIMS
}

function estimateMenuHeight(count: number, dims: MenuDims): number {
  if (count === 0) return dims.emptyHeight
  return Math.min(
    count * dims.itemHeight + dims.vpad + dims.groupOverhead,
    dims.maxHeight,
  )
}

function updateMentionPosition() {
  const sel = window.getSelection()
  if (!sel || sel.rangeCount === 0) return
  const range = sel.getRangeAt(0).cloneRange()
  range.collapse(true)
  // getBoundingClientRect on a collapsed range returns a zero-width rect at
  // the caret position — unlike getClientRects, which Chromium returns as an
  // empty list for collapsed ranges. top/left are still valid.
  const rect = range.getBoundingClientRect()
  const caretValid = rect.top !== 0 || rect.left !== 0 || rect.height !== 0

  let caretLeft: number
  let caretTop: number
  let caretBottom: number
  if (caretValid) {
    caretLeft = rect.left
    caretTop = rect.top
    caretBottom = rect.bottom
  } else {
    const editorRect = editorRef.value?.getBoundingClientRect()
    if (!editorRect) return
    caretLeft = editorRect.left
    caretTop = editorRect.bottom
    caretBottom = editorRect.bottom
  }

  const dims = activeMenuDims()
  const menuHeight = estimateMenuHeight(totalCandidates.value, dims)
  const vw = window.innerWidth
  const vh = window.innerHeight
  const spaceBelow = vh - caretBottom - MENU_VIEWPORT_MARGIN
  const spaceAbove = caretTop - MENU_VIEWPORT_MARGIN

  // Prefer below; flip above only when below is too tight AND above has
  // strictly more room. Keeps the menu in one place when both sides fit.
  const flipUp = spaceBelow < menuHeight && spaceAbove > spaceBelow
  const top = flipUp
    ? caretTop - menuHeight - MENU_GAP
    : caretBottom + MENU_GAP

  const left = Math.max(
    MENU_VIEWPORT_MARGIN,
    Math.min(caretLeft, vw - dims.width - MENU_VIEWPORT_MARGIN),
  )
  const clampedTop = Math.max(
    MENU_VIEWPORT_MARGIN,
    Math.min(top, vh - menuHeight - MENU_VIEWPORT_MARGIN),
  )
  mentionPosition.value = { left, top: clampedTop }
}

function moveSelection(delta: number) {
  const count = totalCandidates.value
  if (!count) return
  selectedCandidateIndex.value =
    (selectedCandidateIndex.value + delta + count) % count
}

function closeMention() {
  activeQuery.value = null
  selectedCandidateIndex.value = 0
}

// pickAtIndex resolves the menu item at a given index in the flat
// keyboard-nav sequence and routes it. Returns true when something was
// picked (caller should preventDefault). Used by Enter/Tab.
function pickAtIndex(index: number): boolean {
  if (activeQuery.value?.kind === 'agent') {
    const picked = filteredAgents.value[index]
    if (!picked) return false
    pickAgent(picked)
    return true
  }
  if (activeQuery.value?.kind === 'slash') {
    const picked = flatSlashItems.value[index]
    if (!picked) return false
    pickSlashItem(picked)
    return true
  }
  return false
}

function pickAgent(candidate: MentionCandidate) {
  insertChip('agent', candidate.id, candidate.name)
}

function pickSlashItem(item: SlashItem) {
  if (item.kind === 'skill') {
    insertChip('skill', item.id, item.name)
    return
  }
  // Command pick. With args → auto-complete to canonical text and
  // close so the user can type arguments. Without args → execute
  // immediately and clear the editor.
  if (item.hasArgs) {
    replaceQueryWithText(`/${item.id} `)
    closeMention()
    updateEmpty()
    return
  }
  closeMention()
  clearEditor()
  emit('command', { id: item.id, args: '' })
}

function insertChip(kind: TokenKind, value: string, label: string) {
  const q = activeQuery.value
  if (!q || !editorRef.value) return
  const { textNode, atIdx, caretOffset } = q

  const range = document.createRange()
  range.setStart(textNode, atIdx)
  range.setEnd(textNode, caretOffset)
  range.deleteContents()

  const triggerChar = kind === 'skill' ? '/' : '@'
  const span = document.createElement('span')
  span.className = `mention-chip kind-${kind}`
  span.contentEditable = 'false'
  span.dataset.kind = kind
  span.dataset.value = value
  span.dataset.label = label
  span.textContent = `${triggerChar}${label}`

  range.insertNode(span)
  const space = document.createTextNode(' ')
  span.after(space)

  const sel = window.getSelection()
  const caret = document.createRange()
  caret.setStart(space, 1)
  caret.collapse(true)
  sel?.removeAllRanges()
  sel?.addRange(caret)

  closeMention()
  updateEmpty()
}

// replaceQueryWithText overwrites the active trigger + typed query
// with a literal string and places the caret at the end of it. Used
// for command auto-complete: `/comp<query>` → `/compact ` + caret.
function replaceQueryWithText(text: string) {
  const q = activeQuery.value
  if (!q || !editorRef.value) return
  const { textNode, atIdx, caretOffset } = q

  const range = document.createRange()
  range.setStart(textNode, atIdx)
  range.setEnd(textNode, caretOffset)
  range.deleteContents()

  const node = document.createTextNode(text)
  range.insertNode(node)

  const sel = window.getSelection()
  const caret = document.createRange()
  caret.setStart(node, node.length)
  caret.collapse(true)
  sel?.removeAllRanges()
  sel?.addRange(caret)
}

// ── Text insertion & paste ────────────────────────────────────────────────

function insertTextAtCaret(text: string) {
  const sel = window.getSelection()
  if (!sel || sel.rangeCount === 0) return
  const range = sel.getRangeAt(0)
  range.deleteContents()
  const node = document.createTextNode(text)
  range.insertNode(node)
  const after = document.createRange()
  after.setStart(node, node.length)
  after.collapse(true)
  sel.removeAllRanges()
  sel.addRange(after)
}

function onPaste(event: ClipboardEvent) {
  const cd = event.clipboardData
  if (!cd) return

  // When the active model can't accept images, drop pasted image data on
  // the floor and fall through to the text branch instead of staging.
  if (props.imagesAvailable !== false) {
    const images: File[] = []
    for (const item of Array.from(cd.items)) {
      if (item.kind !== 'file') continue
      if (!item.type.startsWith('image/')) continue
      const file = item.getAsFile()
      if (file) images.push(file)
    }
    if (images.length) {
      event.preventDefault()
      stageImages(images)
      return
    }
  }

  const text = cd.getData('text/plain')
  if (text == null) return
  event.preventDefault()
  insertTextAtCaret(text)
  updateEmpty()
  scanMentionQuery()
}

// ── Empty detection ───────────────────────────────────────────────────────

function updateEmpty() {
  const root = editorRef.value
  if (!root) {
    isEmpty.value = true
    return
  }
  const raw = (root.textContent ?? '').replace(/ /g, ' ')
  const hasMention = Boolean(root.querySelector('.mention-chip'))
  isEmpty.value = raw.trim().length === 0 && !hasMention
}

// ── Submit ────────────────────────────────────────────────────────────────

function extractMessage(): string {
  const root = editorRef.value
  if (!root) return ''
  return walk(root).replace(/ /g, ' ').replace(/\n{3,}/g, '\n\n').trim()
}

function walk(node: Node): string {
  if (node.nodeType === Node.TEXT_NODE) {
    return (node as Text).data
  }
  if (node.nodeType !== Node.ELEMENT_NODE) return ''
  const el = node as HTMLElement
  if (el.classList.contains('mention-chip')) {
    return chipToInlineText(el)
  }
  if (el.tagName === 'BR') return '\n'
  let out = ''
  for (const child of Array.from(el.childNodes)) {
    out += walk(child)
  }
  if (el.tagName === 'DIV' || el.tagName === 'P') out += '\n'
  return out
}

function clearEditor() {
  if (editorRef.value) editorRef.value.innerHTML = ''
  closeMention()
  isEmpty.value = true
}

async function submit() {
  if (props.disabled) return
  const text = extractMessage()
  if (!text && stagedImages.value.length === 0) return

  // Slash commands are intercepted before the message is dispatched as
  // user content. parseCommandLine returns null for unrecognised lines,
  // which fall through to the regular send path.
  if (text && stagedImages.value.length === 0) {
    const parsed = parseCommandLine(text)
    if (parsed) {
      clearEditor()
      await nextTick()
      emit('command', { id: parsed.command.id, args: parsed.args })
      return
    }
  }

  const files = stagedImages.value.map(img => img.file)
  clearEditor()
  clearStagedImages()
  await nextTick()
  emit('send', { text, files })
}

function handleAction() {
  if (props.interruptible) {
    if (!props.interrupting) emit('interrupt')
    return
  }
  submit()
}

// ── Image attachments (unchanged) ─────────────────────────────────────────

function openFilePicker() {
  if (props.imagesAvailable === false) return
  fileInputRef.value?.click()
}

function onFileInputChange(event: Event) {
  const input = event.target as HTMLInputElement
  if (!input.files) return
  const files = Array.from(input.files)
  stageImages(files)
  input.value = ''
}

function stageImages(files: File[]) {
  for (const file of files) {
    if (!file.type.startsWith('image/')) continue
    stagedImages.value.push({
      id: crypto.randomUUID(),
      file,
      previewUrl: URL.createObjectURL(file),
    })
  }
}

function removeImage(id: string) {
  const idx = stagedImages.value.findIndex(img => img.id === id)
  if (idx < 0) return
  URL.revokeObjectURL(stagedImages.value[idx].previewUrl)
  stagedImages.value.splice(idx, 1)
}

function clearStagedImages() {
  for (const img of stagedImages.value) {
    URL.revokeObjectURL(img.previewUrl)
  }
  stagedImages.value = []
}

onBeforeUnmount(() => {
  for (const img of stagedImages.value) {
    URL.revokeObjectURL(img.previewUrl)
  }
})
</script>

<style scoped>
.input-bar {
  pointer-events: auto;
}

.composer {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
  padding: 10px 12px 10px;
  border: 1px solid var(--rule-soft);
  background: var(--panel-2);
  box-shadow: 0 10px 30px rgba(20, 18, 15, 0.10);
  transition: border-color .15s, box-shadow .15s;
}

.composer.focused {
  border-color: var(--rule);
  box-shadow: 2px 2px 0 0 var(--rule);
}

.composer.disabled {
  background: var(--bg-2);
}

.attachment-tray {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 2px 0 6px;
  border-bottom: 1px dashed var(--rule-softer);
}

.attachment-chip {
  position: relative;
  width: 56px;
  height: 56px;
  border: 1px solid var(--rule-softer);
  background: var(--bg-2);
  overflow: hidden;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.attachment-chip img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.attachment-remove {
  position: absolute;
  top: 2px;
  right: 2px;
  width: 16px;
  height: 16px;
  border: none;
  background: rgba(20, 18, 15, 0.7);
  color: #fff;
  font-size: 13px;
  line-height: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  padding: 0;
}

.attachment-remove:hover {
  background: rgba(20, 18, 15, 0.88);
}

.editor {
  position: relative;
  width: 100%;
  min-height: 40px;
  max-height: 160px;
  overflow-y: auto;
  padding: 0;
  font-size: 14px;
  font-family: var(--sans);
  line-height: 1.55;
  color: var(--ink);
  background: transparent;
  outline: none;
  white-space: pre-wrap;
  word-break: break-word;
}

.editor.empty::before {
  content: attr(data-placeholder);
  color: var(--ink-4);
  pointer-events: none;
  position: absolute;
  top: 0;
  left: 0;
}

.editor[aria-disabled='true'] {
  color: var(--ink-4);
  cursor: not-allowed;
}

.editor :deep(.mention-chip) {
  display: inline-block;
  padding: 0 6px;
  margin: 0 1px;
  border-radius: 3px;
  background: color-mix(in srgb, var(--accent) 15%, var(--panel-2));
  color: var(--accent);
  font-size: 13px;
  line-height: 1.4;
  user-select: all;
  white-space: nowrap;
}

.editor :deep(.mention-chip.kind-skill) {
  background: color-mix(in srgb, var(--info) 15%, var(--panel-2));
  color: var(--info);
}

.composer-actions {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  padding-top: 6px;
  border-top: 1px dashed var(--rule-softer);
}

.attach-btn {
  margin-right: auto;
}

/* When the active model can't accept images, dim the button to half
 * opacity instead of removing it. The button still receives hover so the
 * tooltip explaining the unsupported state can show; clicks are
 * intercepted in the JS handler. */
.attach-btn.unsupported {
  opacity: 0.4;
  cursor: not-allowed;
}

.attach-btn.unsupported:hover:not(:disabled) {
  background: transparent;
  color: var(--ink-3);
}

.attach-icon {
  width: 15px;
  height: 15px;
}

.file-input {
  display: none;
}

.thinking-control {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  --toggle-track-on: color-mix(in srgb, #84befe 72%, var(--panel-2));
  --toggle-track-on-border: color-mix(in srgb, #67a8f4 68%, var(--rule-soft));
  --toggle-knob-on: #f7fbff;
}

.thinking-label {
  font-family: var(--sans);
  font-size: 12px;
  font-weight: 500;
  letter-spacing: 0.02em;
  color: color-mix(in srgb, var(--ink-4) 78%, var(--panel-2));
  white-space: nowrap;
}

.action-btn {
  height: 28px;
  padding: 0 12px;
  border: 1px solid var(--ink);
  background: var(--ink);
  color: var(--panel-2);
  display: inline-flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  font-family: var(--mono);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  transition: transform .12s, box-shadow .12s, opacity .15s;
}

.action-btn:hover:not(:disabled) {
  transform: translate(-1px, -1px);
  box-shadow: 2px 2px 0 0 var(--rule);
}

.action-btn:active:not(:disabled) {
  transform: translate(0, 0);
  box-shadow: none;
}

.action-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.action-btn:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.send-icon {
  width: 13px;
  height: 13px;
  flex-shrink: 0;
}

.action-btn.interrupt {
  border-color: color-mix(in srgb, var(--err) 42%, var(--rule-soft));
  background: color-mix(in srgb, var(--err) 14%, var(--panel-2));
  color: var(--err);
}

.interrupt-spinner {
  width: 12px;
  height: 12px;
  border: 2px solid color-mix(in srgb, var(--err) 20%, transparent);
  border-top-color: var(--err);
  animation: bieneSpin .8s linear infinite;
}

.action-label {
  line-height: 1;
}
</style>
