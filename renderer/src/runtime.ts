export interface DesktopBridge {
  isElectron: boolean
  platform: string
  windowKind: string
  coreBaseUrl: string
  openExternal: (url: string) => Promise<void>
  openAgentWindow: (sessionId: string) => Promise<void>
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

export function buildCoreUrl(path: string) {
  const baseUrl = getCoreBaseUrl()
  if (!baseUrl) return path
  return new URL(path, `${baseUrl}/`).toString()
}
