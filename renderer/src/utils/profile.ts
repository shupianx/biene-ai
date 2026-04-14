import type { AgentDomain, AgentProfile, AgentStyle } from '../api/http'
import { t } from '../i18n'

export interface SelectOption<T extends string> {
  value: T
  label: string
  description: string
}

export interface ProfileOption<T extends string> {
  value: T
  labelKey: string
  descriptionKey: string
  selectable?: boolean
}

const domainCatalog: ProfileOption<AgentDomain>[] = [
  {
    value: 'general',
    labelKey: 'profile.domain.general.label',
    descriptionKey: 'profile.domain.general.description',
    selectable: true,
  },
  {
    value: 'coding',
    labelKey: 'profile.domain.coding.label',
    descriptionKey: 'profile.domain.coding.description',
    selectable: true,
  },
]

const styleCatalog: ProfileOption<AgentStyle>[] = [
  {
    value: 'balanced',
    labelKey: 'profile.style.balanced.label',
    descriptionKey: 'profile.style.balanced.description',
    selectable: true,
  },
  {
    value: 'concise',
    labelKey: 'profile.style.concise.label',
    descriptionKey: 'profile.style.concise.description',
    selectable: true,
  },
  {
    value: 'thorough',
    labelKey: 'profile.style.thorough.label',
    descriptionKey: 'profile.style.thorough.description',
    selectable: true,
  },
  {
    value: 'skeptical',
    labelKey: 'profile.style.skeptical.label',
    descriptionKey: 'profile.style.skeptical.description',
    selectable: true,
  },
  {
    value: 'proactive',
    labelKey: 'profile.style.proactive.label',
    descriptionKey: 'profile.style.proactive.description',
    selectable: true,
  },
]

export function defaultProfile(): AgentProfile {
  return {
    domain: 'general',
    style: 'balanced',
    custom_instructions: '',
  }
}

export function cloneProfile(profile: AgentProfile): AgentProfile {
  return {
    domain: profile.domain,
    style: profile.style,
    custom_instructions: profile.custom_instructions ?? '',
  }
}

export function listDomainOptions(current?: AgentDomain) {
  return listOptions(domainCatalog, current)
}

export function listStyleOptions(current?: AgentStyle) {
  return listOptions(styleCatalog, current)
}

export function findDomainOption(value: AgentDomain) {
  return fromCatalogOption(domainCatalog.find((option) => option.value === value))
}

export function findStyleOption(value: AgentStyle) {
  return fromCatalogOption(styleCatalog.find((option) => option.value === value))
}

function listOptions<T extends string>(catalog: ProfileOption<T>[], current?: T) {
  const currentOption = current ? catalog.find((option) => option.value === current) : undefined
  const visible = catalog.filter((option) => option.selectable !== false)
  if (currentOption && visible.every((option) => option.value !== currentOption.value)) {
    return [...visible, currentOption].map((option) => toSelectOption(option))
  }
  return visible.map((option) => toSelectOption(option))
}

function fromCatalogOption<T extends string>(option?: ProfileOption<T>): SelectOption<T> | undefined {
  if (!option) return undefined
  return toSelectOption(option)
}

function toSelectOption<T extends string>(option: ProfileOption<T>): SelectOption<T> {
  return {
    value: option.value,
    label: t(option.labelKey),
    description: t(option.descriptionKey),
  }
}
