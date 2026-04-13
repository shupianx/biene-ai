const { mkdirSync, readFileSync, rmSync, writeFileSync } = require('fs')
const path = require('path')

function defaultDesktopSettings() {
  return {
    keepCoreRunningOnExit: true,
    theme: 'light',
  }
}

function desktopSettingsPath(app) {
  return path.join(app.getPath('userData'), 'desktop-settings.json')
}

function coreStatePath(app) {
  return path.join(app.getPath('userData'), 'desktop-core.json')
}

function ensureParentDir(filePath) {
  mkdirSync(path.dirname(filePath), { recursive: true })
}

function readJSON(filePath) {
  try {
    return JSON.parse(readFileSync(filePath, 'utf8'))
  } catch {
    return null
  }
}

function loadDesktopSettings(app) {
  const settings = readJSON(desktopSettingsPath(app))
  const next = settings && typeof settings === 'object' ? settings : {}
  return {
    ...defaultDesktopSettings(),
    ...next,
    theme: normalizeTheme(next.theme),
  }
}

function saveDesktopSettings(app, nextSettings) {
  const normalized = {
    keepCoreRunningOnExit: Boolean(nextSettings?.keepCoreRunningOnExit ?? true),
    theme: normalizeTheme(nextSettings?.theme),
  }
  const filePath = desktopSettingsPath(app)
  ensureParentDir(filePath)
  writeFileSync(filePath, `${JSON.stringify(normalized, null, 2)}\n`, 'utf8')
  return normalized
}

function normalizeTheme(value) {
  return value === 'dark' ? 'dark' : 'light'
}

function loadCoreState(app) {
  const state = readJSON(coreStatePath(app))
  if (!state || typeof state !== 'object') return null
  if (typeof state.baseUrl !== 'string' || typeof state.pid !== 'number') return null
  if (typeof state.token !== 'string' || !state.token.trim()) return null
  return {
    baseUrl: state.baseUrl,
    pid: state.pid,
    token: state.token,
  }
}

function saveCoreState(app, state) {
  const filePath = coreStatePath(app)
  ensureParentDir(filePath)
  writeFileSync(filePath, `${JSON.stringify(state, null, 2)}\n`, 'utf8')
}

function clearCoreState(app) {
  rmSync(coreStatePath(app), { force: true })
}

module.exports = {
  clearCoreState,
  defaultDesktopSettings,
  loadCoreState,
  loadDesktopSettings,
  saveCoreState,
  saveDesktopSettings,
}
