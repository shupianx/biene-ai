<template>
  <div v-if="req" class="card">
    <div class="card-head">
      <span class="state" :class="{ expired: isExpired }">
        <span class="state-dot" />
        <span>{{ isExpired ? t('permissions.expiredLabel') : t('sessionStatus.approval') }}</span>
      </span>
      <span class="tool-name">{{ permissionLabel }}</span>
    </div>
    <p class="desc">{{ isExpired ? t('permissions.expiredDescription') : t('permissions.approvalDescription') }}</p>
    <p v-if="permissionDescription" class="permission-desc">{{ permissionDescription }}</p>

    <div v-if="progressLine" class="progress-line" aria-live="polite">
      <span v-if="progressPath" class="progress-path">{{ progressPath }}</span>
      <span v-if="progressBytes" class="progress-bytes">{{ progressBytes }}</span>
    </div>

    <div v-if="!isExpired && collisions.length > 0" class="collisions">
      <div class="collisions-head">{{ t('permissions.collisions.title') }}</div>
      <p class="collisions-desc">{{ t('permissions.collisions.description') }}</p>
      <ul class="collisions-list">
        <li v-for="c in collisions" :key="c.target_path">{{ c.target_path }}</li>
      </ul>
      <div class="strategy" role="radiogroup">
        <label
          v-for="opt in strategyOptions"
          :key="opt.value"
          class="strategy-option"
          :class="{ selected: collisionStrategy === opt.value }"
        >
          <input
            type="radio"
            :value="opt.value"
            v-model="collisionStrategy"
          />
          <span>{{ opt.label }}</span>
        </label>
      </div>
    </div>

    <div class="actions">
      <button v-if="isExpired" class="btn deny" @click="onDeny">{{ t('common.close') }}</button>
      <template v-else>
        <button class="btn deny"   @click="onDeny">{{ t('permissions.deny') }}</button>
        <button class="btn allow"  @click="onAllow('allow')">{{ t('permissions.allowOnce') }}</button>
        <button class="btn always" @click="onAllow('always')">{{ t('permissions.allowAlways') }}</button>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import type { PermissionRequest } from '../stores/sessions'
import type { CollisionStrategy, FileCollision } from '../types/events'
import { t } from '../i18n'
import { getPermissionDescription, getPermissionLabel } from '../utils/permissions'

const props = defineProps<{ req: PermissionRequest | null }>()
const emit = defineEmits<{
  (e: 'resolve', d: 'allow' | 'always' | 'deny', resolution?: Record<string, unknown>): void
}>()

const permissionLabel = computed(() => getPermissionLabel(props.req?.permission ?? ''))
const permissionDescription = computed(() => getPermissionDescription(props.req?.permission ?? ''))
const isExpired = computed(() => Boolean(props.req?.expired))

const collisions = computed<FileCollision[]>(() => props.req?.context?.collisions ?? [])

const progressPath = computed(() => props.req?.progress?.file_path ?? '')
const progressBytes = computed(() => {
  const n = props.req?.progress?.file_text_bytes ?? 0
  if (!n) return ''
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(2)} MB`
})
const progressLine = computed(() => Boolean(progressPath.value || progressBytes.value))

const collisionStrategy = ref<CollisionStrategy>('rename')

const strategyOptions = computed(() => [
  { value: 'rename' as const,    label: t('permissions.collisions.rename') },
  { value: 'overwrite' as const, label: t('permissions.collisions.overwrite') },
  { value: 'skip' as const,      label: t('permissions.collisions.skip') },
])

function onDeny() {
  emit('resolve', 'deny')
}

function onAllow(kind: 'allow' | 'always') {
  if (collisions.value.length > 0) {
    emit('resolve', kind, { collision: collisionStrategy.value })
  } else {
    emit('resolve', kind)
  }
}
</script>

<style scoped>
.card {
  margin: 10px 0 14px;
  padding: 14px 16px;
  border: 1px solid var(--warn);
  background: var(--panel-2);
  box-shadow: 3px 3px 0 0 var(--rule);
}

.card-head {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 10px;
  padding-bottom: 8px;
  border-bottom: 1px dashed var(--rule-softer);
}

.state {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 2px 7px;
  border: 1px solid var(--warn);
  color: var(--warn);
  font-family: var(--mono);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.state.expired {
  border-color: var(--rule);
  color: var(--ink-4);
}

.state-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--warn);
  animation: bienePulse 1.6s ease-in-out infinite;
}

.state.expired .state-dot {
  background: currentColor;
  animation: none;
}

.tool-name {
  font-family: var(--mono);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.08em;
  color: var(--ink);
}

.desc {
  margin: 0 0 6px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--ink-3);
}

.permission-desc {
  margin: 0 0 14px;
  font-size: 12.5px;
  line-height: 1.5;
  color: var(--ink-2);
  padding: 8px 10px;
  background: var(--panel);
  border-left: 2px solid var(--warn);
}

.progress-line {
  margin: 0 0 12px;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  padding: 6px 10px;
  background: var(--panel);
  border: 1px dashed var(--rule-softer);
  font-family: var(--mono);
  font-size: 11.5px;
  line-height: 1.4;
  color: var(--ink-2);
}

.progress-path {
  word-break: break-all;
}

.progress-bytes {
  color: var(--ink-3);
  letter-spacing: 0.02em;
}

.collisions {
  margin: 0 0 14px;
  padding: 10px 12px;
  background: var(--panel);
  border: 1px solid var(--rule-softer);
  border-left: 2px solid var(--warn);
}

.collisions-head {
  font-family: var(--mono);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--ink-2);
  margin-bottom: 6px;
}

.collisions-desc {
  margin: 0 0 8px;
  font-size: 12.5px;
  line-height: 1.5;
  color: var(--ink-2);
}

.collisions-list {
  margin: 0 0 10px;
  padding: 0;
  list-style: none;
  max-height: 140px;
  overflow-y: auto;
  font-family: var(--mono);
  font-size: 11.5px;
  line-height: 1.55;
  color: var(--ink-3);
}

.collisions-list li {
  padding: 1px 0;
  word-break: break-all;
}

.strategy {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.strategy-option {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border: 1px solid var(--rule-softer);
  background: var(--panel-2);
  font-family: var(--mono);
  font-size: 11px;
  letter-spacing: 0.06em;
  color: var(--ink-2);
  cursor: pointer;
  transition: border-color .12s, background .12s, color .12s;
}

.strategy-option:hover {
  border-color: var(--rule-soft);
  color: var(--ink);
}

.strategy-option.selected {
  border-color: var(--ink);
  background: var(--ink);
  color: var(--panel-2);
}

.strategy-option input {
  appearance: none;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  border: 1px solid currentColor;
  margin: 0;
}

.strategy-option.selected input {
  background: currentColor;
}

.actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  flex-wrap: wrap;
}

.btn {
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
  transition: transform .12s, box-shadow .12s, background .12s;
}

.btn:hover {
  transform: translate(-1px, -1px);
  box-shadow: 2px 2px 0 0 var(--rule);
}

.btn:active {
  transform: translate(0, 0);
  box-shadow: none;
}

.deny {
  color: var(--ink-3);
}

.allow {
  background: var(--ink);
  border-color: var(--ink);
  color: var(--panel-2);
}

.always {
  background: var(--err);
  border-color: var(--err);
  color: var(--panel-2);
}
</style>
