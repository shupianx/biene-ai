export const messages = {
  en: {
    'profile.domain.general.label': 'General',
    'profile.domain.general.description': 'General problem-solving, planning, and analysis.',
    'profile.domain.coding.label': 'Coding',
    'profile.domain.coding.description': 'Software engineering work inside the workspace.',
    'profile.style.balanced.label': 'Balanced',
    'profile.style.balanced.description': 'Balanced speed, clarity, and completeness.',
    'profile.style.concise.label': 'Concise',
    'profile.style.concise.description': 'Shorter, tighter responses with minimal preamble.',
    'profile.style.thorough.label': 'Thorough',
    'profile.style.thorough.description': 'More complete explanations with assumptions and tradeoffs.',
    'profile.style.skeptical.label': 'Skeptical',
    'profile.style.skeptical.description': 'More verification, risk checking, and challenge of assumptions.',
    'profile.style.proactive.label': 'Proactive',
    'profile.style.proactive.description': 'Pushes the task forward aggressively when the next step is clear.',
  },
} as const

export type Locale = keyof typeof messages
export type MessageKey = keyof (typeof messages)['en']
