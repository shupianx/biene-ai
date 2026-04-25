const { existsSync, mkdirSync, readdirSync, renameSync, rmSync } = require('fs')
const path = require('path')
const { spawnSync } = require('child_process')

const rootDir = path.resolve(__dirname, '..')
const releaseDir = path.join(rootDir, 'release')
const tempDir = path.join(releaseDir, '.tmp')
const builderCacheDir = path.join(rootDir, '.cache', 'electron-builder')

const mode = process.argv[2] || 'all'
const validModes = new Set(['mac', 'win', 'all'])

if (!validModes.has(mode)) {
  console.error(`Unknown release mode: ${mode}`)
  process.exit(1)
}

function run(command, args, extraEnv = {}) {
  const result = spawnSync(command, args, {
    cwd: rootDir,
    stdio: 'inherit',
    env: {
      ...process.env,
      ELECTRON_BUILDER_CACHE: builderCacheDir,
      ...extraEnv,
    },
  })

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

  for (const entry of readdirSync(sourceDir, { withFileTypes: true })) {
    if (!entry.isFile()) continue
    if (!matcher(entry.name)) continue
    renameSync(path.join(sourceDir, entry.name), path.join(destinationDir, entry.name))
  }
}

function buildRenderer() {
  run('npm', ['run', 'build:renderer'], {})
}

function buildCore(platform, arch, output) {
  run(process.execPath, [
    path.join(rootDir, 'scripts', 'build-core.cjs'),
    '--platform',
    platform,
    '--arch',
    arch,
    '--output',
    output,
  ])
}

function packageMac() {
  const platformDir = path.join(releaseDir, 'mac-arm64')
  const updateDir = path.join(platformDir, 'update')
  const dmgOutputDir = path.join(tempDir, 'mac-arm64-dmg')
  const updateOutputDir = path.join(tempDir, 'mac-arm64-update')

  cleanPath(platformDir)
  cleanPath(dmgOutputDir)
  cleanPath(updateOutputDir)
  ensureDir(updateDir)

  buildCore('darwin', 'arm64', 'core/dist/biene-core')

  run('npm', [
    'run',
    'package:desktop',
    '--',
    '--mac',
    '--arm64',
    '--config.mac.target=dmg',
    `--config.directories.output=${dmgOutputDir}`,
  ])

  moveFiles(dmgOutputDir, platformDir, (name) => name.endsWith('.dmg'))

  run('npm', [
    'run',
    'package:desktop',
    '--',
    '--mac',
    '--arm64',
    '--config.mac.target=zip',
    `--config.directories.output=${updateOutputDir}`,
  ])

  moveFiles(updateOutputDir, updateDir, (name) => (
    name === 'latest-mac.yml' ||
    name.endsWith('.zip') ||
    name.endsWith('.zip.blockmap')
  ))
}

function packageWin() {
  const platformDir = path.join(releaseDir, 'win-x64')
  const outputDir = path.join(tempDir, 'win-x64')

  cleanPath(platformDir)
  cleanPath(outputDir)
  ensureDir(platformDir)

  buildCore('windows', 'amd64', 'core/dist/biene-core.exe')

  run('npm', [
    'run',
    'package:desktop',
    '--',
    '--win',
    'zip',
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

if (mode === 'mac' || mode === 'all') {
  packageMac()
}

if (mode === 'win' || mode === 'all') {
  packageWin()
}

cleanPath(tempDir)
