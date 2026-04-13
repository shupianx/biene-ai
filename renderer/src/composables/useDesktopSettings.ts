import { computed, ref } from 'vue'
import { getDesktopBridge, type DesktopSettings } from '../runtime'

const settings = ref<DesktopSettings>({
  keepCoreRunningOnExit: true,
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

  return {
    desktopSettingsSupported: computed(() => supported.value),
    keepCoreRunningOnExit: computed(() => settings.value.keepCoreRunningOnExit),
    setKeepCoreRunningOnExit,
  }
}
