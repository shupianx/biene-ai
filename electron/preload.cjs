const { contextBridge, ipcRenderer } = require('electron')

function readCliArg(prefix) {
  const match = process.argv.find((arg) => arg.startsWith(prefix))
  return match ? match.slice(prefix.length) : ''
}

ipcRenderer.on('desktop:coreStatus', (_event, status) => {
  window.dispatchEvent(new CustomEvent('biene:core-status', { detail: status }))
})

ipcRenderer.on('desktop:settingsMenuAction', (_event, detail) => {
  window.dispatchEvent(new CustomEvent('biene:settings-menu-action', { detail }))
})

ipcRenderer.on('desktop:windowMaximizedChange', (_event, maximized) => {
  window.dispatchEvent(new CustomEvent('biene:window-maximized-change', { detail: Boolean(maximized) }))
})

const desktopBridge = Object.freeze({
  isElectron: true,
  initialLocale: (() => {
    const raw = readCliArg('--biene-locale=').toLowerCase()
    if (raw.startsWith('zh')) return 'zh-CN'
    if (raw.startsWith('de')) return 'de'
    return 'en'
  })(),
  initialTheme: readCliArg('--biene-theme=') === 'dark' ? 'dark' : 'light',
  platform: process.platform,
  windowKind: readCliArg('--biene-window-kind=') || 'main',
  coreBaseUrl: readCliArg('--biene-core-url='),
  coreAuthToken: readCliArg('--biene-core-token='),
  getCoreStatus() {
    return ipcRenderer.invoke('desktop:getCoreStatus')
  },
  getDesktopSettings() {
    return ipcRenderer.invoke('desktop:getSettings')
  },
  updateDesktopSettings(patch) {
    return ipcRenderer.invoke('desktop:updateSettings', patch)
  },
  openExternal(url) {
    return ipcRenderer.invoke('desktop:openExternal', url)
  },
  openPath(targetPath) {
    return ipcRenderer.invoke('desktop:openPath', targetPath)
  },
  openAgentWindow(sessionId) {
    return ipcRenderer.invoke('desktop:openAgentWindow', sessionId)
  },
  showCoreMenu(labels) {
    return ipcRenderer.invoke('desktop:showCoreMenu', labels)
  },
  showSettingsMenu(labels) {
    return ipcRenderer.invoke('desktop:showSettingsMenu', labels)
  },
  windowMinimize() {
    return ipcRenderer.invoke('desktop:windowMinimize')
  },
  windowToggleMaximize() {
    return ipcRenderer.invoke('desktop:windowToggleMaximize')
  },
  windowClose() {
    return ipcRenderer.invoke('desktop:windowClose')
  },
  windowIsMaximized() {
    return ipcRenderer.invoke('desktop:windowIsMaximized')
  },
})

contextBridge.exposeInMainWorld('bieneDesktop', desktopBridge)
