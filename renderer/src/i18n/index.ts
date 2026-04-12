import { ref } from 'vue'
import { messages, type Locale, type MessageKey } from './messages'

const DEFAULT_LOCALE: Locale = 'en'

export const currentLocale = ref<Locale>(DEFAULT_LOCALE)

export function setLocale(locale: string) {
  currentLocale.value = isLocale(locale) ? locale : DEFAULT_LOCALE
}

export function t(key: MessageKey, locale = currentLocale.value) {
  const table = messages[locale] ?? messages[DEFAULT_LOCALE]
  return table[key] ?? messages[DEFAULT_LOCALE][key] ?? key
}

function isLocale(value: string): value is Locale {
  return value in messages
}
