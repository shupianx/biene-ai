import type { ConfigModelEntry } from '../api/http'

type ProviderTemplateDefinition = Pick<ConfigModelEntry, 'name' | 'provider' | 'model' | 'base_url' | 'thinking_available'> & {
  id: string
}

export const providerTemplateList = [
  {
    id: 'deepseek-v3-2',
    name: 'Deepseek-V3.2',
    provider: 'openai_compatible',
    model: 'deepseek-chat',
    base_url: 'https://api.deepseek.com',
    thinking_available: false,
  },
  {
    id: 'qwen3-6-plus',
    name: 'Qwen3.6-plus',
    provider: 'openai_compatible',
    model: 'qwen3.6-plus',
    base_url: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    thinking_available: true,
  },
] as const satisfies readonly ProviderTemplateDefinition[]

export type ProviderTemplateKey = (typeof providerTemplateList)[number]['id']

export const providerTemplates = Object.fromEntries(
  providerTemplateList.map((template) => [template.id, template])
) as Record<ProviderTemplateKey, (typeof providerTemplateList)[number]>
