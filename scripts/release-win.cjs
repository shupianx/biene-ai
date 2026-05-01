// Windows-native release pipeline.
//
// Mirrors the win path in scripts/release.cjs, but invokes every child
// command via `process.execPath` against the relevant package's Node CLI
// instead of going through `npm` / `.cmd` shims. On Windows, Node's
// child_process.spawn can't resolve `npm` (because `npm.cmd` is a batch
// shim) without shell: true; even calling `npm.cmd` directly returns
// EINVAL since the Node 20.12+ CVE-2024-27980 mitigation. release.cjs's
// `spawnSync('npm', ...)` calls therefore fail silently with ENOENT.
//
// This file is Windows-only; it bails out on other platforms. The
// cross-platform / cross-compile flow continues to live in release.cjs.

const { existsSync, mkdirSync, readdirSync, renameSync, rmSync } = require('fs')
const path = require('path')
const { spawnSync } = require('child_process')

if (process.platform !== 'win32') {
  console.error('release-win.cjs is Windows-only. Use `npm run build:mac` or `node scripts/release.cjs` on macOS/Linux.')
  process.exit(1)
}

const rootDir = path.resolve(__dirname, '..')
const rendererDir = path.join(rootDir, 'renderer')
const releaseDir = path.join(rootDir, 'release')
const tempDir = path.join(releaseDir, '.tmp')
const builderCacheDir = path.join(rootDir, '.cache', 'electron-builder')

const vueTscCli = path.join(rendererDir, 'node_modules', 'vue-tsc', 'bin', 'vue-tsc.js')
const viteCli = path.join(rendererDir, 'node_modules', 'vite', 'bin', 'vite.js')
const electronBuilderCli = path.join(rootDir, 'node_modules', 'electron-builder', 'out', 'cli', 'cli.js')
const buildCoreScript = path.join(rootDir, 'scripts', 'build-core.cjs')

// runNode invokes a Node-based CLI with the current Node binary. No shell,
// no .cmd shim, and any spawn-time error (e.g. ENOENT) is surfaced
// instead of the silent `process.exit(status ?? 1)` from release.cjs.
function runNode(scriptPath, args, { cwd = rootDir, extraEnv = {} } = {}) {
  const result = spawnSync(process.execPath, [scriptPath, ...args], {
    cwd,
    stdio: 'inherit',
    env: {
      ...process.env,
      ELECTRON_BUILDER_CACHE: builderCacheDir,
      ...extraEnv,
    },
  })

  if (result.error) {
    console.error(`[release-win] failed to spawn ${scriptPath}: ${result.error.message}`)
    process.exit(1)
  }
  if (result.status !== 0) {
    process.exit(result.status ?? 1)
  }
}

function cleanPath(targetPath) {
  rmSync(targetPath, { force: true, recursive: true })
}

function ensureDir(targetPath) {
  mkdirSync(targetPath, { recursive: true })
}

function removeLegacyReleaseArtifacts() {
  if (!existsSync(releaseDir)) return

  for (const entry of readdirSync(releaseDir, { withFileTypes: true })) {
    if (entry.name === '.tmp') continue
    if (entry.isDirectory()) continue

    if (
      entry.name === '.DS_Store' ||
      entry.name.startsWith('Biene-') ||
      entry.name.startsWith('latest') ||
      entry.name.endsWith('.blockmap') ||
      entry.name.endsWith('.7z')
    ) {
      cleanPath(path.join(releaseDir, entry.name))
    }
  }
}

function moveFiles(sourceDir, destinationDir, matcher) {
  if (!existsSync(sourceDir)) {
    throw new Error(`Missing expected build output directory: ${sourceDir}`)
  }

  ensureDir(destinationDir)

  const moved = []
  for (const entry of readdirSync(sourceDir, { withFileTypes: true })) {
    if (!entry.isFile()) continue
    if (!matcher(entry.name)) continue
    const destinationPath = path.join(destinationDir, entry.name)
    renameSync(path.join(sourceDir, entry.name), destinationPath)
    moved.push(destinationPath)
  }

  return moved
}

// renderer/package.json :: "build" expands to `vue-tsc -b && vite build`.
// We chain the two CLIs directly so we don't need npm to demux the script.
function buildRenderer() {
  runNode(vueTscCli, ['-b'], { cwd: rendererDir })
  runNode(viteCli, ['build'], { cwd: rendererDir })
}

function buildCore(platform, arch, output) {
  runNode(buildCoreScript, [
    '--platform', platform,
    '--arch', arch,
    '--output', output,
  ])
}

function packageWin() {
  const platformDir = path.join(releaseDir, 'win-x64')
  const outputDir = path.join(tempDir, 'win-x64')

  cleanPath(platformDir)
  cleanPath(outputDir)
  ensureDir(platformDir)

  buildCore('windows', 'amd64', 'core/dist/biene-core.exe')

  // electron-builder's Node CLI accepts the same arg list as the npm
  // bin shim. Skip exe sign+edit since this path doesn't carry a Windows
  // signing identity (matches the existing scripts/release.cjs behavior).
  runNode(electronBuilderCli, [
    '--config', path.join(rootDir, 'electron-builder.json'),
    '--win', 'zip',
    '--x64',
    '--config.win.signAndEditExecutable=false',
    `--config.directories.output=${outputDir}`,
  ])

  moveFiles(outputDir, platformDir, (name) => name.endsWith('.zip'))
}

ensureDir(builderCacheDir)
ensureDir(releaseDir)
ensureDir(tempDir)
removeLegacyReleaseArtifacts()

buildRenderer()
packageWin()

cleanPath(tempDir)
