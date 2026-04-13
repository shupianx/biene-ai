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

const desktopBridge = Object.freeze({
  isElectron: true,
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
  openAgentWindow(sessionId) {
    return ipcRenderer.invoke('desktop:openAgentWindow', sessionId)
  },
  showCoreMenu(labels) {
    return ipcRenderer.invoke('desktop:showCoreMenu', labels)
  },
  showSettingsMenu(labels) {
    return ipcRenderer.invoke('desktop:showSettingsMenu', labels)
  },
})

contextBridge.exposeInMainWorld('bieneDesktop', desktopBridge)
