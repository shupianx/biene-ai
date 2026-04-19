import { createI18n } from 'vue-i18n'
import { messages, type AppLocale } from './messages'
import { getDesktopBridge } from '../runtime'

const DEFAULT_LOCALE: AppLocale = 'en'
const STORAGE_KEY = 'biene.locale'

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
  const next = normalizeLocale(locale)
  i18n.global.locale.value = next
  if (typeof window !== 'undefined') {
    try {
      window.localStorage.setItem(STORAGE_KEY, next)
    } catch {
      // Ignore storage failures and still apply the in-memory locale.
    }
  }
}

export function t(key: string, params?: Record<string, unknown>) {
  const locale = getLocale()
  return i18n.global.t(key, params ?? {}, locale) as string
}

function resolveInitialLocale(): AppLocale {
  const bridgeLocale = getDesktopBridge()?.initialLocale
  if (bridgeLocale) return normalizeLocale(bridgeLocale)
  if (typeof window !== 'undefined') {
    try {
      const stored = window.localStorage.getItem(STORAGE_KEY)
      if (stored) return normalizeLocale(stored)
    } catch {
      // Ignore storage failures and fall back to the browser locale.
    }
  }
  if (typeof navigator === 'undefined') return DEFAULT_LOCALE
  return normalizeLocale(navigator.language)
}

function normalizeLocale(locale: unknown): AppLocale {
  const value = String(locale ?? '').toLowerCase()
  if (value.startsWith('zh')) return 'zh-CN'
  return DEFAULT_LOCALE
}
