const http = require('http')
const path = require('path')
const { spawn, spawnSync } = require('child_process')

const rootDir = path.resolve(__dirname, '..')
// Must use 'localhost' (not 127.0.0.1) to match renderer/vite.config.ts:
// Vite binds to ::1 when host is 'localhost' on Node 17+, and a raw IPv4
// probe would miss it.
const rendererUrl = 'http://localhost:5173'
const electronCli = path.join(rootDir, 'node_modules', 'electron', 'cli.js')

let rendererProcess = null
let electronProcess = null

function wait(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function buildCore() {
  const result = spawnSync(process.execPath, [path.join(rootDir, 'scripts', 'build-core.cjs')], {
    cwd: rootDir,
    stdio: 'inherit',
    env: process.env,
  })

  if (result.status !== 0) {
    process.exit(result.status ?? 1)
  }
}

async function waitForRenderer(timeoutMs = 20000) {
  const startedAt = Date.now()

  while (Date.now() - startedAt < timeoutMs) {
    const ok = await new Promise((resolve) => {
      const req = http.get(rendererUrl, (res) => {
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

  throw new Error('Timed out while waiting for the Vite dev server.')
}

function cleanup() {
  electronProcess?.kill()
  rendererProcess?.kill()
}

process.on('SIGINT', () => {
  cleanup()
  process.exit(130)
})

process.on('SIGTERM', () => {
  cleanup()
  process.exit(143)
})

buildCore()

rendererProcess = spawn('npm', ['--prefix', 'renderer', 'run', 'dev'], {
  cwd: rootDir,
  stdio: 'inherit',
  env: process.env,
})

rendererProcess.once('exit', (code) => {
  if (code !== 0) process.exit(code ?? 1)
})

waitForRenderer()
  .then(() => {
    electronProcess = spawn(process.execPath, [electronCli, '.'], {
      cwd: rootDir,
      stdio: 'inherit',
      env: {
        ...process.env,
        BIENE_RENDERER_URL: rendererUrl,
      },
    })

    electronProcess.once('exit', (code) => {
      cleanup()
      process.exit(code ?? 0)
    })
  })
  .catch((err) => {
    console.error(err instanceof Error ? err.message : String(err))
    cleanup()
    process.exit(1)
  })
