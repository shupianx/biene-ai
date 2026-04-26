<template>
  <div ref="rootRef" class="ptm-wrap" @click.stop>
    <slot name="trigger" :open="open" :toggle="toggle" />

    <div v-if="open" class="ptm-menu" :class="placementClass">
      <div class="ptm-vendors">
        <button
          v-for="vendor in vendors"
          :key="vendor.id"
          type="button"
          class="ptm-vendor"
          :class="{
            hovered: hoveredVendorId === vendor.id,
            selected: vendorIsSelected(vendor),
            empty: vendor.models.length === 0,
          }"
          @mouseenter="onVendorHover(vendor.id)"
          @click="onVendorClick(vendor)"
        >
          <component :is="vendor.icon" class="ptm-icon" aria-hidden="true" />
          <span class="ptm-vendor-name">{{ vendor.name }}</span>
          <span v-if="vendor.models.length > 0" class="ptm-chev" aria-hidden="true">›</span>
        </button>

        <div class="ptm-divider" aria-hidden="true" />

        <button
          type="button"
          class="ptm-vendor"
          :class="{ selected: selectedId === customTemplate.id, hovered: hoveredVendorId === customTemplate.id }"
          @mouseenter="onVendorHover(customTemplate.id)"
          @click="onCustomClick"
        >
          <component :is="customTemplate.icon" class="ptm-icon" aria-hidden="true" />
          <span class="ptm-vendor-name">{{ customLabel }}</span>
        </button>
      </div>

      <div
        v-if="hoveredVendor && hoveredVendor.models.length > 0"
        class="ptm-submenu"
      >
        <button
          v-for="model in hoveredVendor.models"
          :key="model.id"
          type="button"
          class="ptm-model"
          :class="{ selected: selectedId === model.id }"
          @click="onModelClick(model.id)"
        >
          <span class="ptm-model-name">{{ model.name }}</span>
          <span class="ptm-model-key">{{ model.model }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  customTemplate,
  providerVendors,
  type ProviderVendor,
} from '../../constants/providerTemplates'

type SelectedID = string

const props = withDefaults(
  defineProps<{
    selectedId: SelectedID
    customLabel: string
    placement?: 'bottom-left' | 'bottom-right'
  }>(),
  {
    placement: 'bottom-left',
  },
)

const emit = defineEmits<{
  (e: 'select', id: SelectedID): void
  (e: 'open-change', open: boolean): void
}>()

const vendors = providerVendors

const rootRef = ref<HTMLElement | null>(null)
const open = ref(false)
const hoveredVendorId = ref<string>('')

const placementClass = computed(() =>
  props.placement === 'bottom-right' ? 'ptm-menu--br' : 'ptm-menu--bl',
)

const hoveredVendor = computed<ProviderVendor | null>(() => {
  if (!hoveredVendorId.value || hoveredVendorId.value === customTemplate.id) {
    return null
  }
  return vendors.find((v) => v.id === hoveredVendorId.value) ?? null
})

function vendorIsSelected(vendor: ProviderVendor) {
  return vendor.models.some((m) => m.id === props.selectedId)
}

function onVendorHover(id: string) {
  hoveredVendorId.value = id
}

function onVendorClick(vendor: ProviderVendor) {
  if (vendor.models.length === 0) {
    // Vendor with no preset models — treat the click as "go custom with
    // this vendor's provider/base_url pre-filled". Caller decides how to
    // hydrate; we just emit a structured id the parent can interpret.
    emit('select', `vendor:${vendor.id}`)
    setOpen(false)
    return
  }
  // Pin the submenu so the user can move the cursor toward it without
  // accidentally crossing another vendor and switching panels.
  hoveredVendorId.value = vendor.id
}

function onModelClick(modelID: string) {
  emit('select', modelID)
  setOpen(false)
}

function onCustomClick() {
  emit('select', customTemplate.id)
  setOpen(false)
}

function setOpen(value: boolean) {
  if (open.value === value) return
  open.value = value
  emit('open-change', value)
  if (value) {
    // When (re)opening, pre-hover the vendor that owns the current
    // selection so the submenu shows that vendor's models immediately.
    const vendor = vendors.find((v) => vendorIsSelected(v))
    hoveredVendorId.value = vendor?.id ?? ''
  }
}

function toggle() {
  setOpen(!open.value)
}

function handlePointerDown(event: MouseEvent) {
  if (!open.value) return
  if (rootRef.value?.contains(event.target as Node)) return
  setOpen(false)
}

function handleKeydown(event: KeyboardEvent) {
  if (open.value && event.key === 'Escape') {
    setOpen(false)
  }
}

onMounted(() => {
  document.addEventListener('mousedown', handlePointerDown)
  document.addEventListener('keydown', handleKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener('mousedown', handlePointerDown)
  document.removeEventListener('keydown', handleKeydown)
})

defineExpose({ open, toggle, setOpen })
</script>

<style scoped>
.ptm-wrap {
  position: relative;
  display: block;
}

.ptm-menu {
  position: absolute;
  top: calc(100% + 4px);
  display: flex;
  align-items: flex-start;        /* L2 takes only its own content height */
  z-index: 80;
  background: transparent;        /* Border + shadow live on each panel */
  /* Submenu pops out to the right, so left/right alignment of the L1
   * panel relative to the trigger is what placement controls. */
}

.ptm-menu--bl { left: 0; }
.ptm-menu--br { right: 0; }

.ptm-vendors {
  display: flex;
  flex-direction: column;
  min-width: 180px;
  padding: 4px 0;
  background: var(--panel);
  border: 1px solid var(--rule);
  box-shadow: 4px 4px 0 0 var(--rule-soft);
}

.ptm-vendor {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px 12px;
  border: none;
  background: transparent;
  color: var(--ink-2);
  font-family: var(--sans);
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  white-space: nowrap;
}

.ptm-vendor:hover,
.ptm-vendor.hovered {
  background: var(--hover-soft);
  color: var(--ink);
}

.ptm-vendor.selected {
  color: var(--ink);
  font-weight: 600;
}

.ptm-icon {
  flex: 0 0 auto;
  width: 16px;
  height: 16px;
  color: var(--ink-3);
}

.ptm-vendor.selected .ptm-icon,
.ptm-vendor:hover .ptm-icon,
.ptm-vendor.hovered .ptm-icon {
  color: var(--ink);
}

.ptm-vendor-name {
  flex: 1 1 auto;
  min-width: 0;
}

.ptm-chev {
  flex: 0 0 auto;
  font-size: 16px;
  line-height: 1;
  color: var(--ink-4);
}

.ptm-vendor:hover .ptm-chev,
.ptm-vendor.hovered .ptm-chev {
  color: var(--ink-2);
}

.ptm-divider {
  height: 1px;
  margin: 4px 0;
  background: var(--rule-softer);
}

.ptm-submenu {
  display: flex;
  flex-direction: column;
  min-width: 200px;
  padding: 4px 0;
  background: var(--panel-2);
  /* Sits flush against L1's right edge — L1 already has a right border
   * acting as the seam, so we drop ours to avoid a 2px doubled line. */
  border: 1px solid var(--rule);
  border-left: 0;
  box-shadow: 4px 4px 0 0 var(--rule-soft);
}

.ptm-model {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  padding: 7px 12px;
  border: none;
  background: transparent;
  color: var(--ink-2);
  font-family: var(--sans);
  font-size: 13px;
  text-align: left;
  cursor: pointer;
}

.ptm-model:hover {
  background: var(--hover-soft);
  color: var(--ink);
}

.ptm-model.selected {
  color: var(--ink);
  font-weight: 600;
}

.ptm-model-key {
  font-family: var(--mono);
  font-size: 10.5px;
  color: var(--ink-4);
  letter-spacing: 0.02em;
}

.ptm-model.selected .ptm-model-key,
.ptm-model:hover .ptm-model-key {
  color: var(--ink-3);
}
</style>
