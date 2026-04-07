const { contextBridge, ipcRenderer } = require('electron')

function readCliArg(prefix) {
  const match = process.argv.find((arg) => arg.startsWith(prefix))
  return match ? match.slice(prefix.length) : ''
}

const desktopBridge = Object.freeze({
  isElectron: true,
  platform: process.platform,
  windowKind: readCliArg('--biene-window-kind=') || 'main',
  coreBaseUrl: readCliArg('--biene-core-url='),
  openExternal(url) {
    return ipcRenderer.invoke('desktop:openExternal', url)
  },
  openAgentWindow(sessionId) {
    return ipcRenderer.invoke('desktop:openAgentWindow', sessionId)
  },
})

contextBridge.exposeInMainWorld('bieneDesktop', desktopBridge)
