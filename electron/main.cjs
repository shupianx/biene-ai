const { app, BrowserWindow, dialog, ipcMain, shell } = require('electron')
const { spawn } = require('child_process')
const http = require('http')
const net = require('net')
const path = require('path')

const ROOT_DIR = path.resolve(__dirname, '..')
const CORE_DIR = path.join(ROOT_DIR, 'core')
const RENDERER_ENTRY = path.join(ROOT_DIR, 'renderer', 'dist', 'index.html')
const IS_DEV = Boolean(process.env.BIENE_RENDERER_URL)

let coreProcess = null
let coreBaseUrl = ''
let isQuitting = false
let mainWindow = null
const agentWindows = new Map()

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

function resolveCoreCommand(port) {
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
        path.join(app.getPath('userData'), 'workspace'),
      ],
      options: { cwd: process.resourcesPath },
    }
  }

  return {
    command: 'go',
    args: [
      'run',
      '.',
      '--host',
      '127.0.0.1',
      '--port',
      String(port),
      '--workspace',
      path.join(ROOT_DIR, 'workspace'),
    ],
    options: { cwd: CORE_DIR },
  }
}

function wait(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

async function waitForCore(url, timeoutMs = 15000) {
  const startedAt = Date.now()

  while (Date.now() - startedAt < timeoutMs) {
    const ok = await new Promise((resolve) => {
      const req = http.get(`${url}/api/health`, (res) => {
        res.resume()
        resolve(res.statusCode === 200)
      })

      req.on('error', () => resolve(false))
      req.setTimeout(1000, () => {
        req.destroy()
        resolve(false)
      })
    })

    if (ok) return
    await wait(250)
  }

  throw new Error('Timed out while waiting for the local core service to start.')
}

function pipeCoreLogs(child) {
  child.stdout?.on('data', (chunk) => process.stdout.write(`[biene-core] ${chunk}`))
  child.stderr?.on('data', (chunk) => process.stderr.write(`[biene-core] ${chunk}`))
}

async function startCore() {
  const port = await getFreePort()
  coreBaseUrl = `http://127.0.0.1:${port}`

  const { command, args, options } = resolveCoreCommand(port)
  coreProcess = spawn(command, args, {
    ...options,
    env: process.env,
    stdio: ['ignore', 'pipe', 'pipe'],
  })
  pipeCoreLogs(coreProcess)

  coreProcess.once('exit', (code, signal) => {
    coreProcess = null
    if (!isQuitting) {
      const detail = signal ? `signal ${signal}` : `code ${code}`
      dialog.showErrorBox('Biene core stopped', `The local core service exited unexpectedly (${detail}).`)
      app.quit()
    }
  })

  await waitForCore(coreBaseUrl)
}

function stopCore() {
  if (!coreProcess || coreProcess.killed) return
  coreProcess.kill()
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
  const windowOptions = {
    frame: !isFrameless,
    titleBarStyle: isFrameless ? 'default' : (process.platform === 'darwin' ? 'hiddenInset' : 'hidden'),
    backgroundColor: '#f8fafc',
  }
  if (!isFrameless && process.platform !== 'darwin') {
    windowOptions.titleBarOverlay = {
      color: '#f8fafc',
      symbolColor: '#111827',
      height: 40,
    }
  }

  const win = new BrowserWindow({
    width: options.width,
    height: options.height,
    minWidth: options.minWidth,
    minHeight: options.minHeight,
    useContentSize: true,
    center: true,
    title: options.title,
    autoHideMenuBar: true,
    ...windowOptions,
    webPreferences: {
      preload: path.join(__dirname, 'preload.cjs'),
      contextIsolation: true,
      nodeIntegration: false,
      additionalArguments: [
        `--biene-core-url=${coreBaseUrl}`,
        `--biene-window-kind=${options.windowKind ?? 'main'}`,
      ],
    },
  })

  configureAppWindow(win)
  loadRendererRoute(win, options.route)

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
  ipcMain.handle('desktop:openExternal', async (_event, url) => {
    await shell.openExternal(url)
  })
  ipcMain.handle('desktop:openAgentWindow', async (_event, sessionId) => {
    openAgentWindow(sessionId)
  })
}

function createMainWindow() {
  const win = createAppWindow({
    route: '/',
    title: 'Biene',
    width: 1200,
    height: 780,
    minWidth: 960,
    minHeight: 640,
    windowKind: 'main',
    openDevTools: true,
  })
  mainWindow = win
  win.on('closed', () => {
    if (mainWindow === win) {
      mainWindow = null
    }
  })
}

app.on('before-quit', () => {
  isQuitting = true
  stopCore()
})

app.whenReady().then(async () => {
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
