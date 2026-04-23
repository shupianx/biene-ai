<template>
  <div v-if="loading || !session" class="agent-page state-shell">
    <header class="state-chrome">
      <div class="brand">
        <span class="brand-text">{{ t('agent.brand') }}</span>
      </div>
      <div class="chrome-divider" aria-hidden="true" />
      <div class="chrome-name" :title="headerLabel">{{ headerLabel }}</div>
      <div class="chrome-spacer" />
      <div class="status-tag" :class="stateTone">
        <span class="status-dot" />
        <span>{{ stateBadge }}</span>
      </div>
      <IconButton :aria-label="t('common.close')" @click="closeAgentView">
        <svg class="close-icon" viewBox="0 0 24 24" aria-hidden="true" v-html="closeIconBody" />
      </IconButton>
    </header>

    <div class="state-page">
      <div class="state-card" :class="stateTone">
        <div v-if="loading" class="state-loader" aria-hidden="true">
          <span />
          <span />
          <span />
        </div>
        <div v-else class="state-symbol" aria-hidden="true">?</div>
        <p class="state-title">{{ stateTitle }}</p>
        <p class="state-hint">{{ stateHint }}</p>
        <div v-if="sessionId" class="state-meta">
          <span class="state-meta-label">{{ t('agent.requestedId') }}</span>
          <code class="state-meta-value">{{ sessionId }}</code>
        </div>
        <AppButton v-if="isMissing" variant="primary" @click="closeAgentView">{{ t('common.back') }}</AppButton>
      </div>
    </div>
  </div>
  <AgentChatView
    v-else
    :session="session"
    @close="closeAgentView"
  />
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import AgentChatView from '../components/chat/AgentChatView.vue'
import AppButton from '../components/ui/AppButton.vue'
import IconButton from '../components/ui/IconButton.vue'
import { t } from '../i18n'
import { useAgentNavigation } from '../composables/useAgentNavigation'
import { useSessionsStore } from '../stores/sessions'

const route = useRoute()
const store = useSessionsStore()
const { closeAgentView } = useAgentNavigation()
const loading = ref(true)
const closeIconBody = '<path fill="currentColor" d="m12 13.4l-4.9 4.9q-.275.275-.7.275t-.7-.275t-.275-.7t.275-.7l4.9-4.9l-4.9-4.9q-.275-.275-.275-.7t.275-.7t.7-.275t.7.275l4.9 4.9l4.9-4.9q.275-.275.7-.275t.7.275t.275.7t-.275.7L13.4 12l4.9 4.9q.275.275.275.7t-.275.7t-.7.275t-.7-.275z"/>'

const sessionId = computed(() =>
  typeof route.params.id === 'string' ? route.params.id : ''
)

const session = computed(() =>
  sessionId.value ? store.sessions[sessionId.value] ?? null : null
)
const isMissing = computed(() => !loading.value && !session.value)
const stateTone = computed(() => (loading.value ? 'loading' : 'missing'))
const stateBadge = computed(() =>
  loading.value ? t('agent.loadingShort') : t('agent.notFoundShort')
)
const stateTitle = computed(() =>
  loading.value ? t('agent.loading') : t('agent.notFound')
)
const stateHint = computed(() =>
  loading.value ? t('agent.loadingHint') : t('agent.notFoundHint')
)
const headerLabel = computed(() =>
  session.value?.meta.name || sessionId.value || stateTitle.value
)

function syncSessionMeta() {
  if (!sessionId.value) return
  void store.syncSession(sessionId.value, false, true)
}

function onVisibilityChange() {
  if (document.visibilityState !== 'visible') return
  syncSessionMeta()
}

watch(
  sessionId,
  async (id) => {
    loading.value = true

    if (!id) {
      loading.value = false
      return
    }

    // Populate the full session roster so features like @mention have peers
    // to pick from. init() is idempotent, so reopening this window is cheap.
    await Promise.all([
      store.init(false, true),
      store.syncSession(id, true, true),
    ])
    loading.value = false
  },
  { immediate: true },
)

watch(
  [loading, () => session.value?.meta.name, isMissing],
  ([isLoading, name, missing]) => {
    if (typeof name === 'string' && name) {
      document.title = `${name} · Biene`
      return
    }
    if (isLoading) {
      document.title = `${t('agent.loadingShort')} · Biene`
      return
    }
    if (missing) {
      document.title = `${t('agent.notFoundShort')} · Biene`
      return
    }
    document.title = 'Biene'
  },
  { immediate: true },
)

onMounted(() => {
  window.addEventListener('focus', syncSessionMeta)
  document.addEventListener('visibilitychange', onVisibilityChange)
})

onBeforeUnmount(() => {
  window.removeEventListener('focus', syncSessionMeta)
  document.removeEventListener('visibilitychange', onVisibilityChange)
})
</script>

<style scoped>
.agent-page {
  height: 100%;
  background: var(--bg);
  color: var(--ink);
}

.state-shell {
  display: flex;
  flex-direction: column;
}

.state-chrome {
  height: 36px;
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 8px 0 14px;
  background: var(--panel);
  border-bottom: 1px solid var(--rule-soft);
  user-select: none;
  -webkit-app-region: drag;
}

.brand {
  display: inline-flex;
  align-items: center;
}

.brand-text {
  font-family: var(--mono);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.14em;
  color: var(--ink);
}

.chrome-divider {
  width: 1px;
  height: 14px;
  background: var(--rule-soft);
}

.chrome-name {
  min-width: 0;
  font-size: 12px;
  font-weight: 500;
  color: var(--ink-2);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.chrome-spacer {
  flex: 1;
}

.status-tag {
  -webkit-app-region: no-drag;
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

.status-tag.loading {
  color: var(--accent);
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.status-tag.loading .status-dot {
  animation: bienePulse 1.6s ease-in-out infinite;
}

.close-icon {
  width: 14px;
  height: 14px;
}

.state-page {
  flex: 1;
  min-height: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 28px 20px;
}

.state-card {
  width: min(420px, 100%);
  padding: 24px 22px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  border: 1px solid var(--rule);
  background: var(--panel-2);
  box-shadow: 4px 4px 0 0 var(--rule);
  text-align: center;
}

.state-card.loading {
  border-color: var(--rule-soft);
}

.state-loader {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 18px;
}

.state-loader span {
  width: 7px;
  height: 7px;
  background: var(--accent);
  animation: bieneBlink 1.1s ease-in-out infinite;
}

.state-loader span:nth-child(2) {
  animation-delay: .15s;
}

.state-loader span:nth-child(3) {
  animation-delay: .3s;
}

.state-symbol {
  width: 48px;
  height: 48px;
  display: grid;
  place-items: center;
  border: 1px solid var(--rule-soft);
  font-family: var(--mono);
  font-size: 24px;
  color: var(--ink-3);
}

.state-title,
.state-hint {
  margin: 0;
}

.state-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--ink);
}

.state-hint {
  max-width: 32ch;
  font-size: 13px;
  line-height: 1.6;
  color: var(--ink-3);
}

.state-meta {
  width: 100%;
  padding-top: 6px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  border-top: 1px dashed var(--rule-softer);
}

.state-meta-label {
  font-family: var(--mono);
  font-size: 10px;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--ink-4);
}

.state-meta-value {
  display: inline-block;
  padding: 4px 8px;
  font-family: var(--mono);
  font-size: 12px;
  color: var(--ink-2);
  background: var(--panel);
  border: 1px solid var(--rule-softer);
  overflow-wrap: anywhere;
}

</style>
