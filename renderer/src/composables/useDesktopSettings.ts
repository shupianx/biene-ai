import { computed, ref } from 'vue'
import { getDesktopBridge, type DesktopSettings } from '../runtime'
import { getLocale, setLocale as setAppLocale } from '../i18n'
import type { AppLocale } from '../i18n/messages'

const settings = ref<DesktopSettings>({
  keepCoreRunningOnExit: true,
  locale: getLocale(),
  theme: 'light',
})
const supported = ref(false)
let initialized = false

async function initDesktopSettings() {
  if (initialized) return
  initialized = true

  const bridge = getDesktopBridge()
  if (!bridge?.getDesktopSettings) return

  supported.value = true
  try {
    settings.value = await bridge.getDesktopSettings()
    setAppLocale(settings.value.locale)
  } catch {
    // Fall back to the in-memory default.
  }
}

export function useDesktopSettings() {
  void initDesktopSettings()

  async function setKeepCoreRunningOnExit(next: boolean) {
    const bridge = getDesktopBridge()
    const previous = settings.value.keepCoreRunningOnExit
    settings.value = { ...settings.value, keepCoreRunningOnExit: next }

    if (!bridge?.updateDesktopSettings) return

    try {
      settings.value = await bridge.updateDesktopSettings({ keepCoreRunningOnExit: next })
    } catch {
      settings.value = { ...settings.value, keepCoreRunningOnExit: previous }
    }
  }

  async function setLocalePreference(next: AppLocale) {
    const bridge = getDesktopBridge()
    settings.value = { ...settings.value, locale: next }
    setAppLocale(next)

    if (!bridge?.updateDesktopSettings) return

    try {
      settings.value = await bridge.updateDesktopSettings({ locale: next })
      setAppLocale(settings.value.locale)
    } catch {
      // Keep the already-applied locale even if desktop persistence fails.
    }
  }

  return {
    desktopSettingsSupported: computed(() => supported.value),
    keepCoreRunningOnExit: computed(() => settings.value.keepCoreRunningOnExit),
    locale: computed(() => settings.value.locale),
    setKeepCoreRunningOnExit,
    setLocalePreference,
  }
}
