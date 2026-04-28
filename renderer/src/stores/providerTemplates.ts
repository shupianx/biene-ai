// Provider templates pinia store.
//
// Templates (data: vendors + per-vendor model presets) are owned by the
// backend (core/internal/templates). The renderer fetches them once at
// boot, enriches each vendor with its icon component, and serves both
// the nested `vendors` list and a flat `byId` lookup to UI consumers.
//
// Why a store + fetch instead of static TS data:
//
//   - Single source of truth lives next to the migration code that needs
//     it (config v3 backfill of context_window). No drift risk.
//   - Templates can be extended at runtime without recompiling the
//     renderer (e.g., user-defined templates in a future feature).
//
// What stays static (in constants/):
//   - `defaultTemplateID` — the UI's "where the cursor lands when the
//     editor opens" pick. Pure UI default, not data.
//   - `customTemplate` — the sentinel for the "configure manually"
//     escape hatch in pickers.
//   - Vendor icons — pure visuals, mapped by vendor id.

import { defineStore } from 'pinia'
import { computed, ref, type Component } from 'vue'

import {
  fetchProviderTemplates,
  type ServerProviderTemplate,
  type ServerVendor,
} from '../api/http'
import { getVendorIcon } from '../constants/providerVendorIcons'

/**
 * One model preset (data + UI metadata). Mirrors what consumers used
 * to import from `constants/providerTemplates.ts` so call sites need
 * minimal changes.
 */
export type ProviderModelTemplate = ServerProviderTemplate

export interface ProviderVendor extends Omit<ServerVendor, 'models'> {
  icon: Component
  models: ProviderModelTemplate[]
}

/** Flat-dictionary entry: model template + its parent vendor's transport
 *  fields, so a single id lookup gives consumers everything they need to
 *  apply a preset to a draft. */
export type ProviderTemplate = ProviderModelTemplate & {
  vendorId: string
  vendorName: string
  provider: ServerVendor['provider']
  base_url: string
}

type LoadState = 'idle' | 'loading' | 'ready' | 'error'

export const useProviderTemplatesStore = defineStore('providerTemplates', () => {
  const vendors = ref<ProviderVendor[]>([])
  const state = ref<LoadState>('idle')
  const errorMessage = ref('')

  /** Flat dictionary keyed by template id. Useful for direct lookups
   *  (the equivalent of the old `providerTemplates[id]`). */
  const byId = computed<Record<string, ProviderTemplate>>(() => {
    const out: Record<string, ProviderTemplate> = {}
    for (const vendor of vendors.value) {
      for (const model of vendor.models) {
        out[model.id] = {
          ...model,
          vendorId: vendor.id,
          vendorName: vendor.name,
          provider: vendor.provider,
          base_url: vendor.base_url,
        }
      }
    }
    return out
  })

  let inFlight: Promise<void> | null = null

  /** Idempotent: subsequent calls re-use the in-flight promise or no-op
   *  when state is already 'ready'. Pass `force=true` to bypass the
   *  cache (e.g. user clicked "refresh templates"). */
  async function ensureLoaded(force = false): Promise<void> {
    if (!force && state.value === 'ready') return
    if (inFlight) return inFlight

    state.value = 'loading'
    errorMessage.value = ''
    inFlight = (async () => {
      try {
        const res = await fetchProviderTemplates()
        vendors.value = (res.vendors ?? []).map((vendor) => ({
          ...vendor,
          icon: getVendorIcon(vendor.id),
        }))
        state.value = 'ready'
      } catch (err) {
        errorMessage.value = err instanceof Error ? err.message : String(err)
        state.value = 'error'
        throw err
      } finally {
        inFlight = null
      }
    })()
    return inFlight
  }

  return {
    vendors,
    state,
    errorMessage,
    byId,
    ensureLoaded,
  }
})
