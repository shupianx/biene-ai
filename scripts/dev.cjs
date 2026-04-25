const http = require('http')
const net = require('net')
const path = require('path')
const { spawn, spawnSync } = require('child_process')

const rootDir = path.resolve(__dirname, '..')
const electronCli = path.join(rootDir, 'node_modules', 'electron', 'cli.js')

let rendererProcess = null
let electronProcess = null

function wait(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function getFreePort() {
  return new Promise((resolve, reject) => {
    const server = net.createServer()
    server.unref()
    server.once('error', reject)
    // Probe on 'localhost' (::1 under Node 17+) so the chosen port is
    // confirmed free in the same address family Vite will bind to.
    server.listen(0, 'localhost', () => {
      const address = server.address()
      const port = typeof address === 'object' && address ? address.port : 0
      server.close((err) => {
        if (err) reject(err)
        else resolve(port)
      })
    })
  })
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

async function waitForRenderer(rendererUrl, timeoutMs = 20000) {
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

async function main() {
  buildCore()

  // Pick a free port at runtime so dev never collides with stale Vite/other
  // services on a fixed 5173. Use 'localhost' in the URL so the probe lands
  // on the same address family Vite binds to (::1 on Node 17+).
  const rendererPort = await getFreePort()
  const rendererUrl = `http://localhost:${rendererPort}`

  rendererProcess = spawn(
    'npm',
    ['--prefix', 'renderer', 'run', 'dev', '--', '--port', String(rendererPort), '--strictPort'],
    {
      cwd: rootDir,
      stdio: 'inherit',
      env: process.env,
    },
  )

  rendererProcess.once('exit', (code) => {
    if (code !== 0) process.exit(code ?? 1)
  })

  await waitForRenderer(rendererUrl)

  const electronEnv = {
    ...process.env,
    BIENE_RENDERER_URL: rendererUrl,
  }
  // VSCode's integrated terminal inherits ELECTRON_RUN_AS_NODE=1 from the
  // extension host, which makes Electron behave as plain Node and breaks
  // require('electron') in the main process.
  delete electronEnv.ELECTRON_RUN_AS_NODE

  electronProcess = spawn(process.execPath, [electronCli, '.'], {
    cwd: rootDir,
    stdio: 'inherit',
    env: electronEnv,
  })

  electronProcess.once('exit', (code) => {
    cleanup()
    process.exit(code ?? 0)
  })
}

main().catch((err) => {
  console.error(err instanceof Error ? err.message : String(err))
  cleanup()
  process.exit(1)
})
