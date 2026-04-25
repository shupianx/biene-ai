<template>
  <div
    v-if="visible"
    class="mention-menu"
    :class="`kind-${kind ?? 'agent'}`"
    :style="{ left: position.left + 'px', top: position.top + 'px' }"
  >
    <div
      v-for="(candidate, i) in candidates"
      :key="candidate.id"
      class="mention-item"
      :class="{ active: i === selectedIndex }"
      @mousedown.prevent="$emit('pick', candidate)"
      @mouseenter="$emit('hover', i)"
    >
      <MaterialSymbolsBookOutline
        v-if="kind === 'skill'"
        class="mention-icon"
        aria-hidden="true"
      />
      <MaterialSymbolsRobot2Outline v-else class="mention-icon" aria-hidden="true" />
      <span class="mention-name">{{ candidate.name }}</span>
      <span class="mention-id">{{ shortId(candidate.id) }}</span>
    </div>
    <div v-if="!candidates.length" class="mention-empty">
      {{ t('input.mention.empty') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import MaterialSymbolsRobot2Outline from '~icons/material-symbols/robot-2-outline'
import MaterialSymbolsBookOutline from '~icons/material-symbols/book-outline'
import type { TokenKind } from '../../utils/mentions'
import { t } from '../../i18n'

export interface MentionCandidate {
  id: string
  name: string
}

defineProps<{
  visible: boolean
  candidates: MentionCandidate[]
  selectedIndex: number
  position: { left: number; top: number }
  kind?: TokenKind
}>()

defineEmits<{
  (e: 'pick', candidate: MentionCandidate): void
  (e: 'hover', index: number): void
}>()

function shortId(id: string): string {
  if (id.length <= 10) return id
  return id.slice(0, 8) + '…'
}
</script>

<style scoped>
.mention-menu {
  position: fixed;
  z-index: 50;
  min-width: 220px;
  max-width: 320px;
  max-height: 240px;
  overflow-y: auto;
  background: var(--panel-2);
  border: 1px solid var(--rule);
  box-shadow: 2px 2px 0 0 var(--rule);
  padding: 4px 0;
  font-family: var(--sans);
}

.mention-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  cursor: pointer;
  min-width: 0;
  color: var(--ink-2);
}

.mention-menu.kind-agent .mention-item.active {
  background: color-mix(in srgb, var(--accent) 18%, var(--panel-2));
  color: var(--ink);
}

.mention-menu.kind-skill .mention-item.active {
  background: color-mix(in srgb, var(--info) 18%, var(--panel-2));
  color: var(--ink);
}

.mention-icon {
  width: 14px;
  height: 14px;
  flex: 0 0 auto;
  color: var(--ink-4);
}

.mention-item.active .mention-icon {
  color: var(--ink-2);
}

.mention-name {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
}

.mention-id {
  flex: 0 0 auto;
  font-family: var(--mono);
  font-size: 10px;
  letter-spacing: 0.04em;
  color: var(--ink-4);
}

.mention-empty {
  padding: 8px 12px;
  font-family: var(--mono);
  font-size: 11px;
  color: var(--ink-4);
}
</style>
