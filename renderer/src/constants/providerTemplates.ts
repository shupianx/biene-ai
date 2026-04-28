// Static UI constants that pair with the runtime-fetched template store.
//
// Template *data* (vendors, models, context windows, thinking flags) is
// owned by the backend and exposed via /api/provider-templates. The
// renderer fetches it through `stores/providerTemplates.ts` and enriches
// vendors with the icons in `providerVendorIcons.ts`.
//
// What lives here:
//   - `customTemplate` — picker sentinel for "configure manually".
//   - `defaultTemplateID` — which preset the editor opens to.
//   - Type re-exports so existing consumers can keep importing from
//     this module while migrating.
//
// What does NOT live here anymore:
//   - The actual list of vendors/models. See:
//       core/internal/templates/templates.go    (data source)
//       renderer/src/stores/providerTemplates.ts (fetch + reactive store)

import { customVendorIcon } from './providerVendorIcons'

export type {
  ProviderModelTemplate,
  ProviderVendor,
  ProviderTemplate,
} from '../stores/providerTemplates'

/**
 * Sentinel for the "no preset, configure manually" escape hatch in the
 * vendor picker. The id is what callers compare against to detect the
 * custom-template selection state.
 */
export const customTemplate = {
  id: 'custom' as const,
  icon: customVendorIcon,
}
export type CustomTemplateID = typeof customTemplate.id

/**
 * Default L2 model id picked when the editor first opens. Change this
 * one line to redirect the cursor to any model id under any vendor in
 * the backend's template list. Validation that this id actually exists
 * happens at the consumer (the picker logs and falls back when missing
 * to keep the boot sequence robust against a stale value).
 */
export const defaultTemplateID = 'deepseek-v4-flash'
