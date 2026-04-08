<template>
  <div
    ref="cardRef"
    class="card"
    :class="statusTone"
    @click="emit('select')"
  >
    <div class="card-header">
      <span class="status-dot" />
      <span class="name">{{ session.meta.name }}</span>
      <div class="menu-wrap" @click.stop>
        <button class="menu-btn" :class="{ open: menuOpen }" title="More" @click="menuOpen = !menuOpen">...</button>
        <div v-if="menuOpen" class="menu">
          <button class="menu-item" @click="onSettings">Settings</button>
          <button class="menu-item danger" @click="onDelete">Delete</button>
        </div>
      </div>
    </div>
    <div class="card-dir">{{ shortDir }}</div>
    <div class="card-footer">
      <span class="status-label" :class="statusTone">{{ statusLabel }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import type { AgentSession } from '../stores/sessions'
import { getSessionStatusLabel, getSessionStatusTone } from '../utils/sessionStatus'

const props = defineProps<{ session: AgentSession }>()
const emit = defineEmits<{
  (e: 'select'): void
  (e: 'settings'): void
  (e: 'delete'): void
}>()
const statusTone = computed(() => getSessionStatusTone(props.session))
const statusLabel = computed(() => getSessionStatusLabel(statusTone.value))
const menuOpen = ref(false)
const cardRef = ref<HTMLElement | null>(null)

const shortDir = computed(() => {
  const d = props.session.meta.work_dir
  const parts = d.split('/')
  return '…/' + parts.slice(-2).join('/')
})

function onSettings() {
  menuOpen.value = false
  emit('settings')
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
  padding: 16px;
  border-radius: 12px;
  cursor: pointer;
  border: 1.5px solid #e5e7eb;
  background: #fff;
  transition: box-shadow .15s, border-color .15s, transform .1s;
  user-select: none;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.card:hover  { box-shadow: 0 4px 12px rgba(0,0,0,.08); border-color: #c7d2fe; transform: translateY(-1px); }
.card.approval { border-color: #fcd34d; }
.card.running { border-color: #6ee7b7; }
.card.error   { border-color: #fca5a5; }

.card-header {
  display: flex; align-items: center; gap: 8px;
}
.status-dot {
  width: 9px; height: 9px; border-radius: 50%; flex-shrink: 0;
  background: #d1d5db;
}
.card.approval .status-dot { background: #f59e0b; animation: pulse 1.4s ease-in-out infinite; }
.card.running .status-dot { background: #10b981; animation: pulse 1.4s ease-in-out infinite; }
.card.error   .status-dot { background: #ef4444; }

.name {
  font-size: 14px; font-weight: bold; color: #111827; flex: 1; min-width: 0;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}

.menu-wrap {
  position: relative;
  flex-shrink: 0;
}
.menu-btn {
  min-width: 28px;
  height: 28px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: #9ca3af;
  cursor: pointer;
  font-size: 18px;
  line-height: 1;
  opacity: 0;
  transition: opacity .15s, background .15s, color .15s;
}
.card:hover .menu-btn,
.menu-btn:focus-visible,
.menu-btn.open {
  opacity: 1;
}
.menu-btn:hover { background: #f3f4f6; color: #374151; }

.menu {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  min-width: 118px;
  padding: 6px;
  border-radius: 10px;
  border: 1px solid #e5e7eb;
  background: #fff;
  box-shadow: 0 16px 40px rgba(15, 23, 42, .14);
  display: flex;
  flex-direction: column;
  gap: 2px;
  z-index: 5;
}
.menu-item {
  border: none;
  background: transparent;
  text-align: left;
  font-size: 13px;
  color: #374151;
  padding: 8px 10px;
  border-radius: 8px;
  cursor: pointer;
}
.menu-item:hover { background: #f3f4f6; }
.menu-item.danger { color: #b91c1c; }
.menu-item.danger:hover { background: #fef2f2; }

.card-dir {
  font-size: 12px; color: #9ca3af; font-family: monospace;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}

.card-footer {
  display: flex; align-items: center;
}
.status-label {
  font-size: 11px; font-weight: bold; padding: 2px 8px; border-radius: 999px;
  text-transform: uppercase; background: #f3f4f6; color: #6b7280;
}
.status-label.approval { background: #fef3c7; color: #92400e; }
.status-label.running { background: #d1fae5; color: #065f46; }
.status-label.error   { background: #fee2e2; color: #991b1b; }

@keyframes pulse { 0%,100% { opacity: 1; } 50% { opacity: .4; } }
</style>
