<template>
  <div v-if="req" class="card">
    <div class="card-head">
      <span class="state">
        <span class="state-dot" />
        <span>{{ t('sessionStatus.approval') }}</span>
      </span>
      <span class="tool-name">{{ permissionLabel }}</span>
    </div>
    <p class="desc">{{ t('permissions.approvalDescription') }}</p>
    <p v-if="permissionDescription" class="permission-desc">{{ permissionDescription }}</p>
    <div class="actions">
      <button class="btn deny"   @click="emit('resolve', 'deny')">{{ t('permissions.deny') }}</button>
      <button class="btn allow"  @click="emit('resolve', 'allow')">{{ t('permissions.allowOnce') }}</button>
      <button class="btn always" @click="emit('resolve', 'always')">{{ t('permissions.allowAlways') }}</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { PermissionRequest } from '../stores/sessions'
import { t } from '../i18n'
import { getPermissionDescription, getPermissionLabel } from '../utils/permissions'

const props = defineProps<{ req: PermissionRequest | null }>()
const emit = defineEmits<{ (e: 'resolve', d: 'allow' | 'always' | 'deny'): void }>()

const permissionLabel = computed(() => getPermissionLabel(props.req?.permission ?? ''))
const permissionDescription = computed(() => getPermissionDescription(props.req?.permission ?? ''))
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

.state-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--warn);
  animation: bienePulse 1.6s ease-in-out infinite;
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
