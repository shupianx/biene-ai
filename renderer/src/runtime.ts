import type { AppLocale } from './i18n/messages'

export interface CoreStatus {
  healthy: boolean
}

export interface CoreMenuLabels {
  killCore: string
  runCore: string
}

export interface SettingsMenuLabels {
  settings: string
  about: string
}

export interface DesktopSettings {
  keepCoreRunningOnExit: boolean
  locale: AppLocale
  theme: 'light' | 'dark'
}

export interface DesktopBridge {
  isElectron: boolean
  initialLocale: AppLocale
  initialTheme: 'light' | 'dark'
  platform: string
  windowKind: string
  coreBaseUrl: string
  coreAuthToken: string
  getCoreStatus: () => Promise<CoreStatus>
  getDesktopSettings: () => Promise<DesktopSettings>
  updateDesktopSettings: (patch: Partial<DesktopSettings>) => Promise<DesktopSettings>
  openExternal: (url: string) => Promise<void>
  openPath: (path: string) => Promise<void>
  openAgentWindow: (sessionId: string) => Promise<void>
  showCoreMenu: (labels: CoreMenuLabels) => Promise<void>
  showSettingsMenu: (labels: SettingsMenuLabels) => Promise<void>
  windowMinimize: () => Promise<void>
  windowToggleMaximize: () => Promise<boolean>
  windowClose: () => Promise<void>
  windowIsMaximized: () => Promise<boolean>
}

declare global {
  interface Window {
    bieneDesktop?: DesktopBridge
  }
}

function normalizeBaseUrl(url: string) {
  return url.replace(/\/+$/, '')
}

export function getDesktopBridge() {
  if (typeof window === 'undefined') return undefined
  return window.bieneDesktop
}

export function getCoreBaseUrl() {
  const desktopUrl = getDesktopBridge()?.coreBaseUrl ?? ''
  const envUrl = import.meta.env.VITE_CORE_URL ?? ''
  const baseUrl = desktopUrl || envUrl
  return baseUrl ? normalizeBaseUrl(baseUrl) : ''
}

export function getCoreAuthToken() {
  const desktopToken = getDesktopBridge()?.coreAuthToken ?? ''
  const envToken = import.meta.env.VITE_CORE_TOKEN ?? ''
  const token = desktopToken || envToken
  return typeof token === 'string' ? token.trim() : ''
}

export function buildCoreHeaders(headers?: HeadersInit) {
  const next = new Headers(headers)
  const token = getCoreAuthToken()
  if (token) {
    next.set('X-Biene-Token', token)
  }
  return next
}

export function buildCoreUrl(path: string) {
  const baseUrl = getCoreBaseUrl()
  if (!baseUrl) return path
  return new URL(path, `${baseUrl}/`).toString()
}

export function buildCoreWebSocketUrl(path: string) {
  const baseUrl = getCoreBaseUrl()
  const url = baseUrl
    ? new URL(path, `${baseUrl}/`)
    : new URL(path, window.location.href)
  const token = getCoreAuthToken()
  if (token) {
    url.searchParams.set('token', token)
  }

  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  return url.toString()
}
