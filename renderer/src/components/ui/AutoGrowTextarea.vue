<template>
  <textarea
    ref="textareaEl"
    :value="modelValue"
    class="auto-textarea"
    :style="{ minHeight: `${minHeight}px` }"
    @input="handleInput"
  />
</template>

<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from 'vue'

const props = withDefaults(defineProps<{
  modelValue: string
  minHeight?: number
}>(), {
  minHeight: 86,
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()

const textareaEl = ref<HTMLTextAreaElement | null>(null)

function resize() {
  const el = textareaEl.value

  if (!el) return

  el.style.height = 'auto'
  el.style.height = `${Math.max(el.scrollHeight, props.minHeight)}px`
}

function handleInput(event: Event) {
  emit('update:modelValue', (event.target as HTMLTextAreaElement).value)
  resize()
}

onMounted(() => {
  resize()
})

watch(
  () => props.modelValue,
  async () => {
    await nextTick()
    resize()
  },
)
</script>

<style scoped>
.auto-textarea {
  resize: none;
  overflow: hidden;
}
</style>
