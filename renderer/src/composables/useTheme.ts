import { computed, ref } from 'vue'

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
}

export function useTheme() {
  initTheme()

  function setTheme(next: ThemeMode) {
    syncTheme(next)
    if (typeof window === 'undefined') return
    try {
      window.localStorage.setItem(storageKey, next)
    } catch {
      // Ignore storage failures and still apply the in-memory theme.
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
