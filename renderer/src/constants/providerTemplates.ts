import { markRaw, type Component } from 'vue'
import type { ConfigModelEntry } from '../api/http'

import RiDeepseekFill from '~icons/ri/deepseek-fill'
import HugeiconsQwen from '~icons/hugeicons/qwen'
import HugeiconsKimiAi from '~icons/hugeicons/kimi-ai'
import SimpleIconsAnthropic from '~icons/simple-icons/anthropic'
import SimpleIconsOpenai from '~icons/simple-icons/openai'
import MsTune from '~icons/material-symbols/tune-sharp'

// `model` holds the thinking-OFF model name; the thinking toggle applies
// `thinking_on` / `thinking_off` as a shallow JSON patch to the request body.
// A patch may add a new field (Qwen), nest one (Kimi), or overwrite `model`
// itself (DeepSeek swaps to deepseek-reasoner).
export type ProviderModelTemplate = {
  /** Globally unique id used by detectProviderTemplate / applyProviderTemplate. */
  id: string
  /** Short display label shown inside a vendor's submenu (e.g. "V3.2 (chat)"). */
  name: string
  /** API model string sent in the chat completion `model` field. */
  model: string
  thinking_available?: boolean
  thinking_on?: ConfigModelEntry['thinking_on']
  thinking_off?: ConfigModelEntry['thinking_off']
  /**
   * Whether the model accepts image inputs. Default (unset) is treated as
   * `true`; templates for vision-incapable models should set this to
   * `false` so the composer hides the image attachment control.
   */
  images_available?: boolean
}

export type ProviderVendor = {
  id: string
  name: string
  icon: Component
  provider: ConfigModelEntry['provider']
  base_url: string
  models: ProviderModelTemplate[]
}

/**
 * Vendors are the L1 entries in the model picker. Each carries the shared
 * provider type + base_url for all its models, plus an icon shown next to
 * the vendor name in the menu.
 */
export const providerVendors: ProviderVendor[] = [
  {
    id: 'anthropic',
    name: 'Anthropic',
    icon: markRaw(SimpleIconsAnthropic),
    provider: 'anthropic',
    base_url: 'https://api.anthropic.com',
    models: [
      {
        id: 'claude-opus-4-7',
        name: 'Opus 4.7',
        model: 'claude-opus-4-7',
        thinking_available: true,
        thinking_on: { thinking: { type: 'enabled', budget_tokens: 8000 } },
        thinking_off: { thinking: { type: 'disabled' } },
      },
      {
        id: 'claude-sonnet-4-6',
        name: 'Sonnet 4.6',
        model: 'claude-sonnet-4-6',
        thinking_available: true,
        thinking_on: { thinking: { type: 'enabled', budget_tokens: 8000 } },
        thinking_off: { thinking: { type: 'disabled' } },
      },
    ],
  },
  {
    id: 'openai',
    name: 'OpenAI',
    icon: markRaw(SimpleIconsOpenai),
    provider: 'openai_compatible',
    base_url: 'https://api.openai.com/v1',
    models: [
      {
        id: 'gpt-5-5',
        name: 'GPT-5.5',
        model: 'gpt-5.5',
      },
      {
        id: 'gpt-5-4',
        name: 'GPT-5.4',
        model: 'gpt-5.4',
      },
    ],
  },
  {
    id: 'deepseek',
    name: 'DeepSeek',
    icon: markRaw(RiDeepseekFill),
    provider: 'openai_compatible',
    base_url: 'https://api.deepseek.com',
    models: [
      {
        id: 'deepseek-v4-pro',
        name: 'V4-Pro',
        model: 'deepseek-v4-pro',
        thinking_available: true,
        thinking_on: { thinking: { type: 'enabled' } },
        thinking_off: { thinking: { type: 'disabled' } },
        images_available: false,
      },
      {
        id: 'deepseek-v4-flash',
        name: 'V4-Flash',
        model: 'deepseek-v4-flash',
        thinking_available: true,
        thinking_on: { thinking: { type: 'enabled' } },
        thinking_off: { thinking: { type: 'disabled' } },
        images_available: false,
      },
      {
        id: 'deepseek-v3-2',
        name: 'V3.2 (chat)',
        model: 'deepseek-chat',
        thinking_available: true,
        thinking_on: { model: 'deepseek-reasoner' },
      },
      
    ],
  },
  {
    id: 'qwen',
    name: 'Qwen',
    icon: markRaw(HugeiconsQwen),
    provider: 'openai_compatible',
    base_url: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    models: [
      {
        id: 'qwen3-6-plus',
        name: 'Qwen3.6-plus',
        model: 'qwen3.6-plus',
        thinking_available: true,
        thinking_on: { enable_thinking: true },
        thinking_off: { enable_thinking: false },
      },
    ],
  },
  {
    id: 'kimi',
    name: 'Kimi (Moonshot)',
    icon: markRaw(HugeiconsKimiAi),
    provider: 'openai_compatible',
    base_url: 'https://api.moonshot.cn/v1',
    models: [
      {
        id: 'kimi-k2-6',
        name: 'K2.6',
        model: 'kimi-k2.6',
        thinking_available: true,
        thinking_on: { thinking: { type: 'enabled' } },
        thinking_off: { thinking: { type: 'disabled' } },
      },
    ],
  },
]

/**
 * Custom provider sentinel — the L1 entry shown last so the user can
 * always escape into a fully manual config.
 */
export const customTemplate = {
  id: 'custom' as const,
  icon: markRaw(MsTune),
}
export type CustomTemplateID = typeof customTemplate.id

/**
 * Default L2 model id. Picked when the editor first opens. Change this one
 * line to point at any model id under any vendor in `providerVendors`.
 */
export const defaultTemplateID = 'deepseek-v4-flash'

// ── Flat dictionary for back-compat ────────────────────────────────────
//
// Many call sites (DesktopSettingsModal.detectProviderTemplate,
// applyProviderTemplate, WelcomeModal preset flow) look up a template by
// model id without caring which vendor it belongs to. Surface a flat
// {[modelID]: FlatTemplate} dictionary that includes the vendor's
// provider/base_url so the consumer can splat it onto a draft.

export type ProviderTemplate = ProviderModelTemplate & {
  vendorId: string
  vendorName: string
  provider: ConfigModelEntry['provider']
  base_url: string
}

export const providerTemplates: Record<string, ProviderTemplate> = Object.fromEntries(
  providerVendors.flatMap((vendor) =>
    vendor.models.map((model) => [
      model.id,
      {
        ...model,
        vendorId: vendor.id,
        vendorName: vendor.name,
        provider: vendor.provider,
        base_url: vendor.base_url,
      } satisfies ProviderTemplate,
    ]),
  ),
)

export type ProviderTemplateKey = keyof typeof providerTemplates

// Module-load guard: catch typos in defaultTemplateID at the earliest
// possible moment instead of silently falling back to "Custom" the first
// time someone opens the editor.
if (!providerTemplates[defaultTemplateID]) {
  throw new Error(
    `providerTemplates: defaultTemplateID '${defaultTemplateID}' does not match any model id under providerVendors`,
  )
}
