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

function ensureMacSigningEnv() {
  if (process.env.BIENE_SKIP_MAC_SIGNING === '1') {
    console.warn('[release] BIENE_SKIP_MAC_SIGNING=1 — building unsigned macOS artifacts (will not run on other machines).')
    return { skipSigning: true }
  }

  const missing = []
  if (!process.env.APPLE_ID) missing.push('APPLE_ID')
  if (!process.env.APPLE_APP_SPECIFIC_PASSWORD) missing.push('APPLE_APP_SPECIFIC_PASSWORD')
  if (!process.env.APPLE_TEAM_ID) missing.push('APPLE_TEAM_ID')

  if (missing.length > 0) {
    console.error(`[release] Missing required env vars for macOS signing + notarization: ${missing.join(', ')}`)
    console.error('[release] Set them, or pass BIENE_SKIP_MAC_SIGNING=1 to build an unsigned artifact for local testing.')
    process.exit(1)
  }

  return { skipSigning: false }
}

function packageMac() {
  const { skipSigning } = ensureMacSigningEnv()

  const platformDir = path.join(releaseDir, 'mac-arm64')
  const updateDir = path.join(platformDir, 'update')
  const dmgOutputDir = path.join(tempDir, 'mac-arm64-dmg')
  const updateOutputDir = path.join(tempDir, 'mac-arm64-update')

  cleanPath(platformDir)
  cleanPath(dmgOutputDir)
  cleanPath(updateOutputDir)
  ensureDir(updateDir)

  const signingOverrides = skipSigning
    ? ['--config.mac.identity=null', '--config.mac.notarize=false']
    : []

  buildCore('darwin', 'arm64', 'core/dist/biene-core')

  run('npm', [
    'run',
    'package:desktop',
    '--',
    '--mac',
    '--arm64',
    '--config.mac.target=dmg',
    '--config.mac.artifactName=Biene-${arch}.${ext}',
    `--config.directories.output=${dmgOutputDir}`,
    ...signingOverrides,
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
    ...signingOverrides,
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
