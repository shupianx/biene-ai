const { Menu, app, BrowserWindow, dialog, ipcMain, shell } = require('electron')
const { spawn } = require('child_process')
const { randomBytes } = require('crypto')
const http = require('http')
const net = require('net')
const path = require('path')
const {
  clearCoreState,
  defaultDesktopSettings,
  loadCoreState,
  loadDesktopSettings,
  saveCoreState,
  saveDesktopSettings,
} = require('./desktopState.cjs')

const ROOT_DIR = path.resolve(__dirname, '..')
const CORE_DIR = path.join(ROOT_DIR, 'core')
const RENDERER_ENTRY = path.join(ROOT_DIR, 'renderer', 'dist', 'index.html')
const IS_DEV = Boolean(process.env.BIENE_RENDERER_URL)
const DEV_CORE_BINARY = path.join(
  CORE_DIR,
  'dist',
  process.platform === 'win32' ? 'biene-core.exe' : 'biene-core',
)
const CORE_AUTH_HEADER = 'X-Biene-Token'

let coreProcess = null
let coreBaseUrl = ''
let coreAuthToken = ''
let isQuitting = false
let mainWindow = null
const agentWindows = new Map()
let desktopSettings = defaultDesktopSettings()
let corePID = 0
let coreMonitorTimer = null
let coreHealthy = false
let coreStartPromise = null
let quitAfterCoreStop = false
let processConfirmHandled = false
let loginShellPathPromise = null
const windowAppearanceOptions = new WeakMap()

function currentTheme() {
  return desktopSettings.theme === 'dark' ? 'dark' : 'light'
}

function currentLocale() {
  if (desktopSettings.locale === 'zh-CN') return 'zh-CN'
  if (desktopSettings.locale === 'de') return 'de'
  return 'en'
}

function getWindowAppearance(theme) {
  if (theme === 'dark') {
    return {
      backgroundColor: '#1A1814',
      overlayColor: '#1F1C17',
      overlaySymbolColor: '#F0EBE1',
    }
  }

  return {
    backgroundColor: '#EDE8DF',
    overlayColor: '#F6F2EA',
    overlaySymbolColor: '#14120F',
  }
}

function applyWindowAppearance(win) {
  if (!win || win.isDestroyed()) return

  const { frameless = false } = windowAppearanceOptions.get(win) ?? {}
  const appearance = getWindowAppearance(currentTheme())
  win.setBackgroundColor(appearance.backgroundColor)

  if (!frameless && process.platform !== 'darwin' && typeof win.setTitleBarOverlay === 'function') {
    win.setTitleBarOverlay({
      color: appearance.overlayColor,
      symbolColor: appearance.overlaySymbolColor,
      height: 40,
    })
  }
}

function refreshAllWindowAppearances() {
  for (const win of BrowserWindow.getAllWindows()) {
    applyWindowAppearance(win)
  }
}

function stopCoreMonitor() {
  if (coreMonitorTimer != null) {
    clearInterval(coreMonitorTimer)
    coreMonitorTimer = null
  }
}

function getFreePort() {
  return new Promise((resolve, reject) => {
    const server = net.createServer()
    server.unref()
    server.once('error', reject)
    server.listen(0, '127.0.0.1', () => {
      const address = server.address()
      const port = typeof address === 'object' && address ? address.port : 0
      server.close((err) => {
        if (err) reject(err)
        else resolve(port)
      })
    })
  })
}

function resolvePackagedWorkspaceDir() {
  if (process.platform === 'darwin') {
    return path.join(app.getPath('userData'), 'workspace')
  }

  return path.join(path.dirname(process.execPath), 'workspace')
}

function defaultLoginShell() {
  if (process.platform === 'darwin') return '/bin/zsh'
  if (process.platform === 'linux') return '/bin/bash'
  return ''
}

function mergePathValues(primary, secondary) {
  const delimiter = path.delimiter
  const seen = new Set()
  const parts = []

  for (const value of [primary, secondary]) {
    if (typeof value !== 'string' || !value.trim()) continue
    for (const entry of value.split(delimiter)) {
      const normalized = entry.trim()
      if (!normalized || seen.has(normalized)) continue
      seen.add(normalized)
      parts.push(normalized)
    }
  }

  return parts.join(delimiter)
}

function resolveLoginShellPath() {
  if (process.platform === 'win32') {
    return Promise.resolve(process.env.PATH || '')
  }
  if (loginShellPathPromise) return loginShellPathPromise

  loginShellPathPromise = new Promise((resolve) => {
    const shellPath = (typeof process.env.SHELL === 'string' && process.env.SHELL.trim())
      ? process.env.SHELL.trim()
      : defaultLoginShell()
    const marker = '__BIENE_PATH__'

    if (!shellPath) {
      resolve(process.env.PATH || '')
      return
    }

    const child = spawn(shellPath, ['-ilc', `printf '${marker}%s' "$PATH"`], {
      stdio: ['ignore', 'pipe', 'ignore'],
      env: {
        ...process.env,
      },
      windowsHide: true,
    })

    let stdout = ''
    let settled = false
    const fallback = process.env.PATH || ''

    const finish = (value) => {
      if (settled) return
      let next = fallback
      if (typeof value === 'string' && value.trim()) {
        const index = value.lastIndexOf(marker)
        if (index >= 0) {
          next = value.slice(index + marker.length).trim()
        } else {
          next = value.trim()
        }
      }
      settled = true
      resolve(next || fallback)
    }

    const timeout = setTimeout(() => {
      child.kill()
      finish(fallback)
    }, 2000)

    child.stdout?.on('data', (chunk) => {
      stdout += chunk.toString()
    })

    child.on('error', () => {
      clearTimeout(timeout)
      finish(fallback)
    })

    child.on('exit', (code) => {
      clearTimeout(timeout)
      if (code === 0) {
        finish(stdout)
        return
      }
      finish(fallback)
    })
  })

  return loginShellPathPromise
}

function resolveCoreCommand(port) {
  const authToken = ensureCoreAuthToken()

  if (app.isPackaged) {
    const binaryName = process.platform === 'win32' ? 'biene-core.exe' : 'biene-core'
    return {
      command: path.join(process.resourcesPath, 'bin', binaryName),
      args: [
        '--host',
        '127.0.0.1',
        '--port',
        String(port),
        '--workspace',
        resolvePackagedWorkspaceDir(),
      ],
      options: {
        cwd: process.resourcesPath,
        env: {
          BIENE_CORE_TOKEN: authToken,
        },
      },
    }
  }

  return {
    command: DEV_CORE_BINARY,
    args: [
      '--host',
      '127.0.0.1',
      '--port',
      String(port),
      '--workspace',
      path.join(ROOT_DIR, 'workspace'),
    ],
    options: {
      cwd: ROOT_DIR,
      env: {
        BIENE_CORE_TOKEN: authToken,
      },
    },
  }
}

function createCoreAuthToken() {
  return randomBytes(24).toString('hex')
}

function ensureCoreAuthToken() {
  if (typeof coreAuthToken === 'string' && coreAuthToken.trim()) {
    return coreAuthToken
  }
  coreAuthToken = createCoreAuthToken()
  return coreAuthToken
}

function wait(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function parsePortFromUrl(url) {
  try {
    const port = Number(new URL(String(url)).port)
    return Number.isInteger(port) && port > 0 && port <= 65535 ? port : 0
  } catch {
    return 0
  }
}

async function resolveCorePort() {
  const preferredPort = parsePortFromUrl(coreBaseUrl)
  if (preferredPort > 0) return preferredPort
  return getFreePort()
}

async function waitForCore(url, timeoutMs = 15000) {
  const startedAt = Date.now()

  while (Date.now() - startedAt < timeoutMs) {
    const ok = await checkCoreHealth(url)

    if (ok) return
    await wait(250)
  }

  throw new Error('Timed out while waiting for the local core service to start.')
}

function buildCoreAuthHeaders(token = coreAuthToken) {
  if (!token) return undefined
  return {
    [CORE_AUTH_HEADER]: token,
  }
}

function checkCoreHealth(url, token = coreAuthToken) {
  return new Promise((resolve) => {
    const req = http.get(`${url}/api/health`, { headers: buildCoreAuthHeaders(token) }, (res) => {
      res.resume()
      resolve(res.statusCode === 200)
    })

    req.on('error', () => resolve(false))
    req.setTimeout(1000, () => {
      req.destroy()
      resolve(false)
    })
  })
}

function currentCoreStatus() {
  return {
    healthy: coreHealthy,
  }
}

function normalizeCoreMenuLabels(labels) {
  return {
    killCore: typeof labels?.killCore === 'string' && labels.killCore.trim()
      ? labels.killCore
      : 'Kill core',
    runCore: typeof labels?.runCore === 'string' && labels.runCore.trim()
      ? labels.runCore
      : 'Run core',
  }
}

function normalizeSettingsMenuLabels(labels) {
  return {
    settings: typeof labels?.settings === 'string' && labels.settings.trim()
      ? labels.settings
      : 'Settings',
    about: typeof labels?.about === 'string' && labels.about.trim()
      ? labels.about
      : 'About Biene',
  }
}

function broadcastCoreStatus() {
  const status = currentCoreStatus()
  for (const win of BrowserWindow.getAllWindows()) {
    if (win.isDestroyed()) continue
    win.webContents.send('desktop:coreStatus', status)
  }
}

function setCoreHealthy(next) {
  if (coreHealthy === next) return
  coreHealthy = next
  broadcastCoreStatus()
}

function startCoreMonitor() {
  stopCoreMonitor()

  coreMonitorTimer = setInterval(async () => {
    if (!coreBaseUrl || isQuitting) return
    const ok = await checkCoreHealth(coreBaseUrl)
    setCoreHealthy(ok)
    if (ok) return

    coreProcess = null
    corePID = 0
    clearCoreState(app)
  }, 3000)
}

function pipeCoreLogs(child) {
  child.stdout?.on('data', (chunk) => process.stdout.write(`[biene-core] ${chunk}`))
  child.stderr?.on('data', (chunk) => process.stderr.write(`[biene-core] ${chunk}`))
}

function requestCoreShutdown(timeoutMs = 5000) {
  if (!coreBaseUrl || !ensureCoreAuthToken()) {
    return Promise.resolve(false)
  }

  const shutdownUrl = new URL('/api/admin/shutdown', `${coreBaseUrl}/`)

  return new Promise((resolve) => {
    const req = http.request(shutdownUrl, {
      method: 'POST',
      headers: buildCoreAuthHeaders(),
    }, async (res) => {
      res.resume()
      if (res.statusCode !== 200 && res.statusCode !== 202 && res.statusCode !== 204) {
        resolve(false)
        return
      }

      const startedAt = Date.now()
      while (Date.now() - startedAt < timeoutMs) {
        if (!(await checkCoreHealth(coreBaseUrl))) {
          resolve(true)
          return
        }
        await wait(150)
      }

      resolve(false)
    })

    req.on('error', () => resolve(false))
    req.setTimeout(1500, () => {
      req.destroy()
      resolve(false)
    })
    req.end()
  })
}

function forceStopCore() {
  if (coreProcess && !coreProcess.killed) {
    coreProcess.kill()
  } else if (corePID > 0) {
    try {
      process.kill(corePID)
    } catch {
      // Ignore missing process errors.
    }
  }
}

async function startCore() {
  if (coreHealthy) return
  if (coreStartPromise) return coreStartPromise

  coreStartPromise = startCoreOnce()
  try {
    await coreStartPromise
  } finally {
    coreStartPromise = null
  }
}

async function startCoreOnce() {
  const existingCore = loadCoreState(app)
  if (existingCore) {
    coreAuthToken = existingCore.token
    try {
      await waitForCore(existingCore.baseUrl, 1500)
      coreBaseUrl = existingCore.baseUrl
      corePID = existingCore.pid
      setCoreHealthy(true)
      startCoreMonitor()
      return
    } catch {
      setCoreHealthy(false)
      clearCoreState(app)
    }
  }

  const port = await resolveCorePort()
  coreBaseUrl = `http://127.0.0.1:${port}`
  const loginShellPath = await resolveLoginShellPath()

  const { command, args, options } = resolveCoreCommand(port)
  coreProcess = spawn(command, args, {
    ...options,
    detached: true,
    env: {
      ...process.env,
      PATH: mergePathValues(loginShellPath, process.env.PATH || ''),
      ...(options.env ?? {}),
    },
    stdio: ['ignore', 'pipe', 'pipe'],
    windowsHide: true,
  })
  corePID = coreProcess.pid ?? 0
  pipeCoreLogs(coreProcess)

  const spawnedPID = corePID
  coreProcess.once('exit', (code, signal) => {
    coreProcess = null
    if (corePID === spawnedPID) {
      corePID = 0
      clearCoreState(app)
      setCoreHealthy(false)
    }
    if (!isQuitting) {
      const detail = signal ? `signal ${signal}` : `code ${code}`
      console.error(`Biene core stopped unexpectedly (${detail}).`)
    }
  })

  await waitForCore(coreBaseUrl)
  saveCoreState(app, { baseUrl: coreBaseUrl, pid: corePID, token: ensureCoreAuthToken() })
  setCoreHealthy(true)
  startCoreMonitor()
}

async function stopCore() {
  stopCoreMonitor()
  const stoppedGracefully = await requestCoreShutdown()
  if (!stoppedGracefully) {
    forceStopCore()
  }
  coreProcess = null
  corePID = 0
  clearCoreState(app)
  setCoreHealthy(false)
}

function fetchActiveProcesses() {
  return new Promise((resolve) => {
    if (!coreBaseUrl) {
      resolve([])
      return
    }
    const req = http.get(
      `${coreBaseUrl}/api/processes/active`,
      { headers: buildCoreAuthHeaders() },
      (res) => {
        let body = ''
        res.on('data', (chunk) => {
          body += chunk.toString()
        })
        res.on('end', () => {
          try {
            const parsed = JSON.parse(body)
            resolve(Array.isArray(parsed?.processes) ? parsed.processes : [])
          } catch {
            resolve([])
          }
        })
      },
    )
    req.on('error', () => resolve([]))
    req.setTimeout(1500, () => {
      req.destroy()
      resolve([])
    })
  })
}

function quitConfirmLabels() {
  if (currentLocale() === 'zh-CN') {
    return {
      title: '仍有后台进程在运行',
      message: '以下智能体还有后台进程正在运行。退出 Biene 会终止它们。',
      quit: '仍然退出',
      cancel: '取消',
    }
  }
  return {
    title: 'Background processes are running',
    message: 'These agents still have background processes running. Quitting Biene will stop them.',
    quit: 'Quit anyway',
    cancel: 'Cancel',
  }
}

function formatProcessLine(entry) {
  const argsPart = Array.isArray(entry.args) && entry.args.length > 0
    ? ` ${entry.args.join(' ')}`
    : ''
  return `• ${entry.session_name}: ${entry.command}${argsPart}`
}

async function confirmQuitIfActiveProcesses() {
  if (!coreHealthy) return true
  const processes = await fetchActiveProcesses()
  if (processes.length === 0) return true

  const labels = quitConfirmLabels()
  const detail = processes.map(formatProcessLine).join('\n')
  const parentWin = mainWindow && !mainWindow.isDestroyed()
    ? mainWindow
    : BrowserWindow.getFocusedWindow()

  const options = {
    type: 'warning',
    title: labels.title,
    message: labels.message,
    detail,
    buttons: [labels.quit, labels.cancel],
    defaultId: 1,
    cancelId: 1,
  }
  const result = parentWin
    ? await dialog.showMessageBox(parentWin, options)
    : await dialog.showMessageBox(options)
  return result.response === 0
}

function releaseCoreForAppExit() {
  stopCoreMonitor()
  if (!coreProcess) return
  coreProcess.removeAllListeners('exit')
  coreProcess.stdout?.destroy()
  coreProcess.stderr?.destroy()
  coreProcess.unref()
  coreProcess = null
}

function showCoreMenu(event, labels) {
  const win = BrowserWindow.fromWebContents(event.sender)
  if (!win || win.isDestroyed()) return

  const menuLabels = normalizeCoreMenuLabels(labels)
  const menu = Menu.buildFromTemplate([
    {
      label: menuLabels.killCore,
      enabled: coreHealthy || corePID > 0 || Boolean(coreProcess),
      click: () => {
        void stopCore().catch((err) => {
          dialog.showErrorBox('Failed to stop Biene core', err instanceof Error ? err.message : String(err))
        })
      },
    },
    {
      label: menuLabels.runCore,
      enabled: !coreHealthy && !coreStartPromise,
      click: () => {
        void startCore().catch((err) => {
          dialog.showErrorBox('Failed to start Biene', err instanceof Error ? err.message : String(err))
        })
      },
    },
  ])

  menu.popup({ window: win })
}

function showSettingsMenu(event, labels) {
  const win = BrowserWindow.fromWebContents(event.sender)
  if (!win || win.isDestroyed()) return

  const menuLabels = normalizeSettingsMenuLabels(labels)
  const menu = Menu.buildFromTemplate([
    {
      label: menuLabels.settings,
      click: () => {
        if (!win.isDestroyed()) {
          win.webContents.send('desktop:settingsMenuAction', { action: 'settings' })
        }
      },
    },
    {
      label: menuLabels.about,
      click: () => {
        // Renderer owns the About modal so it follows the app's
        // visual language (BaseModal + blueprint mono tags). The
        // native dialog.showMessageBox path was inconsistent with
        // every other dialog in the app and unstyleable.
        if (!win.isDestroyed()) {
          win.webContents.send('desktop:settingsMenuAction', { action: 'about' })
        }
      },
    },
  ])

  menu.popup({ window: win })
}

function configureAppWindow(win) {
  win.webContents.setWindowOpenHandler(({ url }) => {
    shell.openExternal(url)
    return { action: 'deny' }
  })

  win.webContents.on('will-navigate', (event, url) => {
    const currentUrl = win.webContents.getURL()
    if (url !== currentUrl) {
      event.preventDefault()
      shell.openExternal(url)
    }
  })
}

function loadRendererRoute(win, route) {
  if (IS_DEV) {
    const baseUrl = process.env.BIENE_RENDERER_URL.replace(/\/+$/, '')
    return win.loadURL(`${baseUrl}#${route}`)
  }
  return win.loadFile(RENDERER_ENTRY, { hash: route })
}

function createAppWindow(options) {
  const isFrameless = Boolean(options.frameless)
  const appearance = getWindowAppearance(currentTheme())
  const windowOptions = {
    frame: !isFrameless,
    titleBarStyle: isFrameless ? 'default' : (process.platform === 'darwin' ? 'hiddenInset' : 'hidden'),
    backgroundColor: appearance.backgroundColor,
  }
  if (!isFrameless && process.platform !== 'darwin') {
    windowOptions.titleBarOverlay = {
      color: appearance.overlayColor,
      symbolColor: appearance.overlaySymbolColor,
      height: 40,
    }
  }

  const win = new BrowserWindow({
    width: options.width,
    height: options.height,
    minWidth: options.minWidth,
    minHeight: options.minHeight,
    show: options.show ?? true,
    resizable: options.resizable ?? true,
    maximizable: options.maximizable ?? true,
    minimizable: options.minimizable ?? true,
    skipTaskbar: options.skipTaskbar ?? false,
    parent: options.parent,
    useContentSize: true,
    center: options.center ?? true,
    title: options.title,
    autoHideMenuBar: true,
    ...windowOptions,
    webPreferences: {
      preload: path.join(__dirname, 'preload.cjs'),
      contextIsolation: true,
      nodeIntegration: false,
      additionalArguments: [
        `--biene-core-url=${coreBaseUrl}`,
        `--biene-core-token=${ensureCoreAuthToken()}`,
        `--biene-locale=${currentLocale()}`,
        `--biene-theme=${currentTheme()}`,
        `--biene-window-kind=${options.windowKind ?? 'main'}`,
      ],
    },
  })

  windowAppearanceOptions.set(win, { frameless: isFrameless })
  configureAppWindow(win)
  loadRendererRoute(win, options.route)
  win.webContents.once('did-finish-load', () => {
    if (win.isDestroyed()) return
    win.webContents.send('desktop:coreStatus', currentCoreStatus())
  })

  if (IS_DEV && options.openDevTools) {
    win.webContents.openDevTools({ mode: 'detach' })
  }

  return win
}

function openAgentWindow(sessionId) {
  const existing = agentWindows.get(sessionId)
  if (existing && !existing.isDestroyed()) {
    existing.show()
    existing.focus()
    return
  }

  const win = createAppWindow({
    route: `/agent/${encodeURIComponent(sessionId)}`,
    title: 'Biene',
    width: 540,
    height: 700,
    minWidth: 400,
    minHeight: 640,
    frameless: true,
    windowKind: 'agent',
  })

  agentWindows.set(sessionId, win)
  win.on('closed', () => {
    if (agentWindows.get(sessionId) === win) {
      agentWindows.delete(sessionId)
    }
  })
}

function registerDesktopHandlers() {
  ipcMain.handle('desktop:getCoreStatus', async () => currentCoreStatus())
  ipcMain.handle('desktop:getSettings', async () => desktopSettings)
  ipcMain.handle('desktop:updateSettings', async (_event, patch) => {
    desktopSettings = saveDesktopSettings(app, {
      ...desktopSettings,
      ...(patch && typeof patch === 'object' ? patch : {}),
    })
    refreshAllWindowAppearances()
    return desktopSettings
  })
  ipcMain.handle('desktop:openExternal', async (_event, url) => {
    await shell.openExternal(url)
  })
  ipcMain.handle('desktop:openPath', async (_event, targetPath) => {
    const pathToOpen = typeof targetPath === 'string' ? targetPath.trim() : ''
    if (!pathToOpen) {
      throw new Error('Path is required.')
    }

    const errorMessage = await shell.openPath(pathToOpen)
    if (errorMessage) {
      throw new Error(errorMessage)
    }
  })
  ipcMain.handle('desktop:openAgentWindow', async (_event, sessionId) => {
    openAgentWindow(sessionId)
  })
  ipcMain.handle('desktop:showCoreMenu', async (event, labels) => {
    showCoreMenu(event, labels)
  })
  ipcMain.handle('desktop:showSettingsMenu', async (event, labels) => {
    showSettingsMenu(event, labels)
  })
}

function createMainWindow() {
  const win = createAppWindow({
    route: '/',
    title: 'Biene',
    width: 1140,
    height: 720,
    minWidth: 960,
    minHeight: 640,
    windowKind: 'main',
    openDevTools: false,
  })
  mainWindow = win
  win.on('closed', () => {
    if (mainWindow === win) {
      mainWindow = null
    }
  })
}

app.on('before-quit', (event) => {
  if (!processConfirmHandled && !quitAfterCoreStop) {
    event.preventDefault()
    void confirmQuitIfActiveProcesses().then((proceed) => {
      if (!proceed) {
        // User canceled; allow re-checking next time quit is attempted.
        processConfirmHandled = false
        return
      }
      processConfirmHandled = true
      app.quit()
    }).catch((err) => {
      console.error('Failed to resolve quit confirmation:', err)
      processConfirmHandled = true
      app.quit()
    })
    return
  }

  isQuitting = true
  if (desktopSettings.keepCoreRunningOnExit) {
    releaseCoreForAppExit()
    return
  }
  if (quitAfterCoreStop) {
    return
  }

  event.preventDefault()
  quitAfterCoreStop = true
  void stopCore().catch((err) => {
    console.error('Failed to stop Biene core during app quit:', err)
  }).finally(() => {
    app.quit()
  })
})

app.whenReady().then(async () => {
  // In dev, point the macOS dock at our packaged icon source so the running
  // app shows the brand instead of the default Electron logo. Packaged
  // builds get the icon from the .app bundle automatically.
  if (process.platform === 'darwin' && !app.isPackaged) {
    app.dock?.setIcon(path.join(ROOT_DIR, 'build', 'icon.png'))
  }

  desktopSettings = loadDesktopSettings(app)
  registerDesktopHandlers()

  try {
    await startCore()
    createMainWindow()
  } catch (err) {
    dialog.showErrorBox('Failed to start Biene', err instanceof Error ? err.message : String(err))
    app.quit()
  }

  app.on('activate', () => {
    if (!mainWindow || mainWindow.isDestroyed()) createMainWindow()
  })
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') app.quit()
})
