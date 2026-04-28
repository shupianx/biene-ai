<template>
  <div
    v-if="visible"
    class="slash-menu"
    :style="{ left: position.left + 'px', top: position.top + 'px' }"
  >
    <template v-for="(group, gi) in groups" :key="group.kind">
      <div v-if="gi > 0 && group.items.length" class="slash-divider" aria-hidden="true" />
      <div v-if="group.items.length" class="slash-group-label">
        {{ group.label }}
      </div>
      <div
        v-for="item in group.items"
        :key="`${item.kind}-${item.id}`"
        class="slash-item"
        :class="[`kind-${item.kind}`, { active: flatIndex(item) === selectedIndex }]"
        :title="item.description || undefined"
        @mousedown.prevent="$emit('pick', item)"
        @mouseenter="$emit('hover', flatIndex(item))"
      >
        <component :is="iconFor(item.kind)" class="slash-icon" aria-hidden="true" />
        <span class="slash-name">{{ item.name }}</span>
      </div>
    </template>
    <div v-if="totalCount === 0" class="slash-empty">
      {{ t('input.slash.empty') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import MaterialSymbolsBookOutline from '~icons/material-symbols/book-outline'
import MaterialSymbolsBoltOutline from '~icons/material-symbols/bolt-outline'
import { t } from '../../i18n'

export type SlashItemKind = 'command' | 'skill'

export interface SlashItem {
  kind: SlashItemKind
  /** Stable identifier within its kind (command id or skill id). */
  id: string
  /** Display name shown left-aligned. */
  name: string
  /** Right-column help/description text. */
  description: string
  /** Commands only: whether picking should auto-complete (true) or
   *  execute immediately (false). Skills always pick-as-chip. */
  hasArgs?: boolean
}

interface Props {
  visible: boolean
  groups: Array<{ kind: SlashItemKind; label: string; items: SlashItem[] }>
  selectedIndex: number
  position: { left: number; top: number }
}

const props = defineProps<Props>()

defineEmits<{
  (e: 'pick', item: SlashItem): void
  (e: 'hover', index: number): void
}>()

const totalCount = computed(() =>
  props.groups.reduce((acc, g) => acc + g.items.length, 0),
)

/** Flatten a per-group item back to its position in the full filtered
 *  list. Both InputBar and SlashMenu must agree on this ordering: it's
 *  the index sequence the keyboard ↑/↓ navigation traverses. */
function flatIndex(item: SlashItem): number {
  let idx = 0
  for (const g of props.groups) {
    for (const it of g.items) {
      if (it.kind === item.kind && it.id === item.id) return idx
      idx++
    }
  }
  return -1
}

function iconFor(kind: SlashItemKind) {
  return kind === 'command' ? MaterialSymbolsBoltOutline : MaterialSymbolsBookOutline
}
</script>

<style scoped>
.slash-menu {
  position: fixed;
  z-index: 50;
  min-width: 180px;
  max-width: 240px;
  max-height: 480px;
  overflow-y: auto;
  background: var(--panel-2);
  border: 1px solid var(--rule);
  box-shadow: 2px 2px 0 0 var(--rule);
  padding: 4px 0;
  font-family: var(--sans);
}

.slash-group-label {
  padding: 6px 10px 2px;
  font-family: var(--mono);
  font-size: 9.5px;
  font-weight: 600;
  letter-spacing: 0.18em;
  color: var(--ink-4);
  text-transform: uppercase;
}

.slash-divider {
  height: 0;
  margin: 4px 8px;
  border-top: 1px dashed var(--rule-softer);
}

.slash-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  cursor: pointer;
  min-width: 0;
  color: var(--ink-2);
}

/* Commands carry a permanent violet wash so they're visually distinct
 * from skills even when nothing is hovered/selected. The "active"
 * variant intensifies the same hue so the highlight reads as
 * "stronger-of-the-same" rather than a different colour. */
.slash-item.kind-command {
  background: color-mix(in srgb, var(--violet) 7%, transparent);
}

.slash-item.kind-command.active {
  background: color-mix(in srgb, var(--violet) 22%, var(--panel-2));
  color: var(--ink);
}

.slash-item.kind-command .slash-icon {
  color: var(--violet);
}

/* Skill rows stay neutral when idle and pick up the info-blue tint
 * only when active, matching the rest of the renderer's data colour
 * vocabulary. */
.slash-item.kind-skill.active {
  background: color-mix(in srgb, var(--info) 18%, var(--panel-2));
  color: var(--ink);
}

.slash-icon {
  width: 14px;
  height: 14px;
  flex: 0 0 auto;
  color: var(--ink-4);
}

.slash-item.active .slash-icon {
  color: var(--ink-2);
}

.slash-item.kind-command.active .slash-icon {
  color: var(--violet);
}

.slash-name {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  font-weight: 500;
}

.slash-empty {
  padding: 8px 12px;
  font-family: var(--mono);
  font-size: 11px;
  color: var(--ink-4);
}
</style>
