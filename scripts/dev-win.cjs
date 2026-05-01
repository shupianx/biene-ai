// Windows-specific dev launcher.
//
// Differences from scripts/dev.cjs:
//   1. `npm` on Windows is `npm.cmd`. Node's spawn can't run it without
//      shell: true, and even `npm.cmd` directly returns EINVAL since the
//      Node 20.12+ CVE-2024-27980 mitigation. We sidestep the whole problem
//      by invoking Vite's Node CLI (renderer/node_modules/vite/bin/vite.js)
//      directly with process.execPath — no shell, no .cmd shim.
//   2. Windows has no process groups, so killing the parent doesn't take
//      down child processes. We use `taskkill /T /F` on shutdown to
//      terminate the whole tree.
//   3. SIGTERM doesn't really exist on Windows; we only listen for SIGINT
//      (which Node synthesizes on Ctrl+C) and the parent's 'exit' event.

const http = require('http')
const net = require('net')
const path = require('path')
const { spawn, spawnSync } = require('child_process')

const rootDir = path.resolve(__dirname, '..')
const electronCli = path.join(rootDir, 'node_modules', 'electron', 'cli.js')
const viteCli = path.join(rootDir, 'renderer', 'node_modules', 'vite', 'bin', 'vite.js')

let rendererProcess = null
let electronProcess = null
let cleaningUp = false

function wait(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

function getFreePort() {
  return new Promise((resolve, reject) => {
    const server = net.createServer()
    server.unref()
    server.once('error', reject)
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

function killTree(pid) {
  if (!pid) return
  // /T = also kill child processes; /F = force.
  spawnSync('taskkill', ['/pid', String(pid), '/T', '/F'], {
    stdio: 'ignore',
    windowsHide: true,
  })
}

function cleanup() {
  if (cleaningUp) return
  cleaningUp = true
  killTree(electronProcess?.pid)
  killTree(rendererProcess?.pid)
}

process.on('SIGINT', () => {
  cleanup()
  process.exit(130)
})

process.on('exit', cleanup)

async function main() {
  if (process.platform !== 'win32') {
    console.error('dev-win.cjs is Windows-only. Use `npm run dev` on macOS/Linux.')
    process.exit(1)
  }

  buildCore()

  const rendererPort = await getFreePort()
  const rendererUrl = `http://localhost:${rendererPort}`

  // Run vite's Node entrypoint directly so we don't go through npm.cmd /
  // vite.cmd shims (both fail on Node 20.12+ Windows without a shell).
  rendererProcess = spawn(
    process.execPath,
    [viteCli, '--port', String(rendererPort), '--strictPort'],
    {
      cwd: path.join(rootDir, 'renderer'),
      stdio: 'inherit',
      env: process.env,
    },
  )

  rendererProcess.once('exit', (code) => {
    rendererProcess = null
    if (!cleaningUp && code !== 0 && code !== null) {
      cleanup()
      process.exit(code)
    }
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
    electronProcess = null
    cleanup()
    process.exit(code ?? 0)
  })
}

main().catch((err) => {
  console.error(err instanceof Error ? err.message : String(err))
  cleanup()
  process.exit(1)
})
