import { computed, ref } from 'vue'
import { getDesktopBridge } from '../runtime'

type ThemeMode = 'light' | 'dark'

const storageKey = 'biene.theme'
const theme = ref<ThemeMode>('light')
let initialized = false

function applyTheme(next: ThemeMode) {
  if (typeof document === 'undefined') return
  document.documentElement.dataset.theme = next
  document.documentElement.style.colorScheme = next
}

function readStoredTheme(): ThemeMode {
  const bridgeTheme = getDesktopBridge()?.initialTheme
  if (bridgeTheme === 'dark' || bridgeTheme === 'light') {
    return bridgeTheme
  }
  if (typeof window === 'undefined') return 'light'
  try {
    return window.localStorage.getItem(storageKey) === 'dark' ? 'dark' : 'light'
  } catch {
    return 'light'
  }
}

function syncTheme(next: ThemeMode) {
  theme.value = next
  applyTheme(next)
}

function handleStorage(event: StorageEvent) {
  if (event.key !== storageKey) return
  syncTheme(event.newValue === 'dark' ? 'dark' : 'light')
}

export function initTheme() {
  if (initialized) return
  initialized = true
  syncTheme(readStoredTheme())
  if (typeof window !== 'undefined') {
    window.addEventListener('storage', handleStorage)
  }

  const bridge = getDesktopBridge()
  if (bridge?.getDesktopSettings) {
    void bridge.getDesktopSettings().then((settings) => {
      const next = settings.theme === 'dark' ? 'dark' : 'light'
      syncTheme(next)
      try {
        window.localStorage.setItem(storageKey, next)
      } catch {
        // Ignore storage failures and still apply the in-memory theme.
      }
    }).catch(() => {
      // Keep the already-applied fallback theme.
    })
  }
}

export function useTheme() {
  initTheme()

  function setTheme(next: ThemeMode) {
    syncTheme(next)
    const bridge = getDesktopBridge()
    if (bridge?.updateDesktopSettings) {
      void bridge.updateDesktopSettings({ theme: next }).catch(() => {
        // Ignore desktop setting persistence failures and keep the UI responsive.
      })
    }
    if (typeof window !== 'undefined') {
      try {
        window.localStorage.setItem(storageKey, next)
      } catch {
        // Ignore storage failures and still apply the in-memory theme.
      }
    }
  }

  function toggleTheme() {
    setTheme(theme.value === 'dark' ? 'light' : 'dark')
  }

  return {
    theme: computed(() => theme.value),
    isDark: computed(() => theme.value === 'dark'),
    setTheme,
    toggleTheme,
  }
}
