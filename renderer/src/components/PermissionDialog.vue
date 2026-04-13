<template>
  <div v-if="req" class="card">
    <div class="card-head">
      <span class="state">{{ t('sessionStatus.approval') }}</span>
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
  margin: 8px 0 12px;
  padding: 16px;
  border: 1px solid #fcd34d;
  background: #fffbeb;
  border-radius: 14px;
  box-shadow: 0 10px 30px rgba(146, 64, 14, .08);
}
.card-head {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}
.state {
  display: inline-flex;
  align-items: center;
  padding: 3px 10px;
  border-radius: 999px;
  background: #fde68a;
  color: #92400e;
  font-size: 11px;
  font-weight: bold;
  text-transform: uppercase;
}
.desc  { margin: 0 0 4px; font-size: 14px; color: #6b7280; }
.permission-desc {
  margin: 0 0 16px;
  font-size: 13px;
  color: #92400e;
}
.tool-name    { font-weight: bold; font-size: 14px; color: #78350f; }
.actions { display: flex; justify-content: flex-end; gap: 8px; flex-wrap: wrap; }
.btn {
  padding: 8px 18px; border-radius: 8px; border: none; cursor: pointer;
  font-size: 14px; font-weight: bold; transition: opacity .15s;
}
.btn:hover { opacity: .85; }
.deny   { background: #f3f4f6; color: #374151; }
.allow  { background: #fef3c7; color: #92400e; }
.always { background: #fee2e2; color: #991b1b; }
</style>
