import en from './locales/en'
import zh from './locales/zh'
import de from './locales/de'

// `en` is the source of truth (see locales/en.ts where `Messages` is exported
// from `typeof en`). `zh` and `de` import that type and annotate themselves
// with it, so missing or extra keys in either translation become a TS error
// at build time — much stricter than the old "all in one file" layout where
// drift could only be caught by eyeballing.
export const messages = {
  en,
  'zh-CN': zh,
  de,
} as const

export type AppLocale = keyof typeof messages
