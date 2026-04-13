import { createI18n } from 'vue-i18n'
import { messages, type AppLocale } from './messages'

const DEFAULT_LOCALE: AppLocale = 'en'

export const i18n = createI18n({
  legacy: false,
  fallbackLocale: DEFAULT_LOCALE,
  locale: resolveInitialLocale(),
  messages,
})

export function getLocale(): AppLocale {
  return normalizeLocale(i18n.global.locale.value)
}

export function setLocale(locale: string) {
  i18n.global.locale.value = normalizeLocale(locale)
}

export function t(key: string, params?: Record<string, unknown>) {
  const locale = getLocale()
  return i18n.global.t(key, params ?? {}, locale) as string
}

function resolveInitialLocale(): AppLocale {
  if (typeof navigator === 'undefined') return DEFAULT_LOCALE
  return normalizeLocale(navigator.language)
}

function normalizeLocale(locale: unknown): AppLocale {
  const value = String(locale ?? '').toLowerCase()
  if (value.startsWith('zh')) return 'zh-CN'
  return DEFAULT_LOCALE
}
