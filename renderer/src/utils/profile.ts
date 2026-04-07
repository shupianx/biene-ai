import type { AgentDomain, AgentProfile, AgentStyle } from '../api/http'

export interface SelectOption<T extends string> {
  value: T
  label: string
  description: string
}

export const domainOptions: SelectOption<AgentDomain>[] = [
  {
    value: 'general',
    label: 'General',
    description: 'General problem-solving, planning, and analysis.',
  },
  {
    value: 'coding',
    label: 'Coding',
    description: 'Software engineering work inside the workspace.',
  },
]

export const styleOptions: SelectOption<AgentStyle>[] = [
  {
    value: 'balanced',
    label: 'Balanced',
    description: 'Balanced speed, clarity, and completeness.',
  },
  {
    value: 'concise',
    label: 'Concise',
    description: 'Shorter, tighter responses with minimal preamble.',
  },
  {
    value: 'thorough',
    label: 'Thorough',
    description: 'More complete explanations with assumptions and tradeoffs.',
  },
  {
    value: 'skeptical',
    label: 'Skeptical',
    description: 'More verification, risk checking, and challenge of assumptions.',
  },
  {
    value: 'proactive',
    label: 'Proactive',
    description: 'Pushes the task forward aggressively when the next step is clear.',
  },
]

export function defaultProfile(): AgentProfile {
  return {
    domain: 'coding',
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
