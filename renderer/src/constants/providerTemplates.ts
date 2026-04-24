import type { ConfigModelEntry } from '../api/http'

// `model` holds the thinking-OFF model name; the thinking toggle applies
// `thinking_on` / `thinking_off` as a shallow JSON patch to the request body.
// A patch may add a new field (Qwen), nest one (Kimi), or overwrite `model`
// itself (DeepSeek swaps to deepseek-reasoner).
type ProviderTemplateDefinition = Pick<
  ConfigModelEntry,
  'name' | 'provider' | 'model' | 'base_url' | 'thinking_available' | 'thinking_on' | 'thinking_off'
> & {
  id: string
}

export const providerTemplateList = [
  {
    id: 'deepseek-v3-2',
    name: 'Deepseek-V3.2',
    provider: 'openai_compatible',
    model: 'deepseek-chat',
    base_url: 'https://api.deepseek.com',
    thinking_available: true,
    thinking_on: { model: 'deepseek-reasoner' },
  },
  {
    id: 'deepseek-v4-chat',
    name: 'Deepseek-V4-chat',
    provider: 'openai_compatible',
    model: 'deepseek-v4-flash',
    base_url: 'https://api.deepseek.com',
    thinking_available: true,
    thinking_on: { thinking: {type: "enabled"} },
    thinking_off: { thinking: {type: "disabled"} },
  },
  {
    id: 'qwen3-6-plus',
    name: 'Qwen3.6-plus',
    provider: 'openai_compatible',
    model: 'qwen3.6-plus',
    base_url: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    thinking_available: true,
    thinking_on: { enable_thinking: true },
    thinking_off: { enable_thinking: false },
  },
  {
    id: 'kimi-k2-6',
    name: 'Kimi-K2.6',
    provider: 'openai_compatible',
    model: 'kimi-k2.6',
    base_url: 'https://api.moonshot.cn/v1',
    thinking_available: true,
    thinking_on: { thinking: { type: 'enabled' } },
    thinking_off: { thinking: { type: 'disabled' } },
  },
] as const satisfies readonly ProviderTemplateDefinition[]

export type ProviderTemplateKey = (typeof providerTemplateList)[number]['id']

export const providerTemplates = Object.fromEntries(
  providerTemplateList.map((template) => [template.id, template])
) as Record<ProviderTemplateKey, (typeof providerTemplateList)[number]>
