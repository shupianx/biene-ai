<template>
  <BaseModal :title="title" max-width="420px" :z-index="230" @close="emit('cancel')">
    <label class="fork-field">
      <span class="fork-label">{{ t('fork.nameLabel') }}</span>
      <input
        ref="inputRef"
        v-model="draft"
        class="fork-input"
        type="text"
        autocomplete="off"
        @keydown.enter.prevent="onSubmit"
        @keydown.escape.prevent="emit('cancel')"
      />
    </label>
    <p v-if="errorMsg" class="fork-error">{{ errorMsg }}</p>

    <template #footer>
      <AppButton variant="neutral" @click="emit('cancel')">{{ t('common.cancel') }}</AppButton>
      <AppButton variant="primary" :disabled="!isValid" @click="onSubmit">
        {{ t('fork.confirmButton') }}
      </AppButton>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import BaseModal from '../ui/BaseModal.vue'
import AppButton from '../ui/AppButton.vue'
import { t } from '../../i18n'

const props = defineProps<{
  /** Source agent's name (used in title + as default-name basis). */
  sourceName: string
  /** Names already taken across all agents — used to derive a fresh
   *  default and to validate user-typed values without a server round
   *  trip. The source's own name IS in this list; that's intentional —
   *  the fork must not reuse it. */
  takenNames: string[]
}>()

const emit = defineEmits<{
  (e: 'cancel'): void
  (e: 'confirm', name: string): void
}>()

const draft = ref(suggestForkName(props.sourceName, props.takenNames))
const inputRef = ref<HTMLInputElement | null>(null)

const title = computed(() =>
  t('fork.dialogTitle', { source: props.sourceName }),
)

const trimmed = computed(() => draft.value.trim())

const errorMsg = computed(() => {
  if (!trimmed.value) return t('fork.errorEmpty')
  if (props.takenNames.includes(trimmed.value)) return t('fork.errorTaken')
  return ''
})

const isValid = computed(() => !errorMsg.value)

function onSubmit() {
  if (!isValid.value) return
  emit('confirm', trimmed.value)
}

onMounted(async () => {
  // Open with the input pre-selected so the user can either accept
  // the default by hitting Enter or type a new name to overwrite it.
  await nextTick()
  inputRef.value?.focus()
  inputRef.value?.select()
})

/**
 * suggestForkName picks the first available "<source> 复制体" /
 * "<source> 复制体 2" / "<source> 复制体 3" ... that doesn't collide
 * with `taken`. Pure function so consumers can reuse the algorithm
 * outside the dialog if they ever need to (e.g. silent fork mode).
 */
function suggestForkName(source: string, taken: string[]): string {
  const suffix = t('fork.defaultNameSuffix')
  const base = `${source} ${suffix}`
  const set = new Set(taken)
  if (!set.has(base)) return base
  for (let i = 2; ; i += 1) {
    const candidate = `${base} ${i}`
    if (!set.has(candidate)) return candidate
  }
}
</script>

<style scoped>
.fork-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.fork-label {
  font-family: var(--mono);
  font-size: 9.5px;
  font-weight: 600;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--ink-4);
}

.fork-input {
  font-family: var(--sans);
  font-size: 14px;
  padding: 8px 10px;
  background: var(--panel);
  border: 1px solid var(--rule-soft);
  border-radius: 2px;
  color: var(--ink);
  outline: none;
  transition: border-color 0.12s ease;
}

.fork-input:focus {
  border-color: var(--accent);
}

.fork-error {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--err);
}
</style>
