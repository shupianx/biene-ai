const http = require('http')
const path = require('path')
const { spawn } = require('child_process')

const rootDir = path.resolve(__dirname, '..')
const rendererUrl = 'http://127.0.0.1:5173'
const electronCli = path.join(rootDir, 'node_modules', 'electron', 'cli.js')

let rendererProcess = null
let electronProcess = null

function wait(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
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
