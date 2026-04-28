// Icon registry for provider vendors.
//
// Templates themselves (data: id, name, model, context_window, ...) live
// in core/internal/templates and are fetched at runtime via
// /api/provider-templates. Icons are pure UI resources, so they stay in
// the renderer — the project's "schema is data, icons are decoration"
// split, see CLAUDE.md "Schema 设计准则".
//
// To add a vendor icon:
//   - Import the iconify component below.
//   - Map it to the vendor id used in core/internal/templates/templates.go
//     (Vendor.ID — e.g. "anthropic", "deepseek").
//   - Anything not in the map falls back to MsTune as the generic
//     "custom provider" pictogram.

import { markRaw, type Component } from 'vue'

import RiDeepseekFill from '~icons/ri/deepseek-fill'
import HugeiconsQwen from '~icons/hugeicons/qwen'
import HugeiconsKimiAi from '~icons/hugeicons/kimi-ai'
import SimpleIconsAnthropic from '~icons/simple-icons/anthropic'
import SimpleIconsOpenai from '~icons/simple-icons/openai'
import MsTune from '~icons/material-symbols/tune-sharp'

const VENDOR_ICONS: Record<string, Component> = {
  anthropic: markRaw(SimpleIconsAnthropic),
  openai: markRaw(SimpleIconsOpenai),
  deepseek: markRaw(RiDeepseekFill),
  qwen: markRaw(HugeiconsQwen),
  kimi: markRaw(HugeiconsKimiAi),
}

/** Generic icon shown for vendors we haven't shipped a brand mark for. */
export const customVendorIcon: Component = markRaw(MsTune)

/** Returns the brand icon for `vendorId`, or the generic fallback. */
export function getVendorIcon(vendorId: string): Component {
  return VENDOR_ICONS[vendorId] ?? customVendorIcon
}
