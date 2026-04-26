const { existsSync, mkdirSync, mkdtempSync, readdirSync, renameSync, rmSync } = require('fs')
const path = require('path')
const os = require('os')
const { spawnSync } = require('child_process')

const rootDir = path.resolve(__dirname, '..')
const releaseDir = path.join(rootDir, 'release')
const tempDir = path.join(releaseDir, '.tmp')
const builderCacheDir = path.join(rootDir, '.cache', 'electron-builder')

const mode = process.argv[2] || 'all'
const validModes = new Set(['mac', 'win', 'all'])
let macSigningEnv = null

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

function runChecked(command, args, label) {
  const result = spawnSync(command, args, {
    cwd: rootDir,
    stdio: ['ignore', 'pipe', 'pipe'],
    env: process.env,
  })

  if (result.status !== 0) {
    const stdout = result.stdout?.toString().trim()
    const stderr = result.stderr?.toString().trim()
    console.error(`[release] ${label} failed: ${command} ${args.join(' ')}`)
    if (stdout) console.error(stdout)
    if (stderr) console.error(stderr)
    process.exit(result.status ?? 1)
  }

  return result.stdout?.toString() ?? ''
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

  const hasAppleIdCredentials = Boolean(
    process.env.APPLE_ID &&
    process.env.APPLE_APP_SPECIFIC_PASSWORD &&
    process.env.APPLE_TEAM_ID
  )
  const hasApiKeyCredentials = Boolean(
    process.env.APPLE_API_KEY &&
    process.env.APPLE_API_KEY_ID &&
    process.env.APPLE_API_ISSUER
  )

  if (!hasAppleIdCredentials && !hasApiKeyCredentials) {
    console.error('[release] Missing notarization credentials.')
    console.error('[release] Provide one of:')
    console.error('[release]   APPLE_ID + APPLE_APP_SPECIFIC_PASSWORD + APPLE_TEAM_ID')
    console.error('[release]   APPLE_API_KEY + APPLE_API_KEY_ID + APPLE_API_ISSUER')
    console.error('[release] Set them, or pass BIENE_SKIP_MAC_SIGNING=1 to build an unsigned artifact for local testing.')
    process.exit(1)
  }

  if (!hasMacSigningIdentity()) {
    console.error('[release] Missing Developer ID Application signing identity.')
    console.error('[release] Install a valid certificate in the keychain, or provide CSC_LINK/CSC_NAME for electron-builder.')
    console.error('[release] Set BIENE_SKIP_MAC_SIGNING=1 only for local unsigned test builds.')
    process.exit(1)
  }

  return { skipSigning: false }
}

function getMacSigningEnv() {
  if (!macSigningEnv) macSigningEnv = ensureMacSigningEnv()
  return macSigningEnv
}

function hasMacSigningIdentity() {
  if (process.env.CSC_LINK) return true

  const args = ['find-identity', '-v', '-p', 'codesigning']
  if (process.env.CSC_KEYCHAIN) args.push(process.env.CSC_KEYCHAIN)

  const result = spawnSync('security', args, {
    cwd: rootDir,
    stdio: ['ignore', 'pipe', 'pipe'],
  })
  if (result.status !== 0) return false

  const identities = result.stdout?.toString() ?? ''
  if (!/Developer ID Application/.test(identities)) return false
  if (process.env.CSC_NAME && !identities.includes(process.env.CSC_NAME)) return false
  return true
}

function findAppBundle(searchDir) {
  const entries = readdirSync(searchDir, { withFileTypes: true })
  for (const entry of entries) {
    const entryPath = path.join(searchDir, entry.name)
    if (entry.isDirectory() && entry.name.endsWith('.app')) return entryPath
  }

  for (const entry of entries) {
    const entryPath = path.join(searchDir, entry.name)
    if (!entry.isDirectory() || entry.isSymbolicLink()) continue
    const nested = findAppBundle(entryPath)
    if (nested) return nested
  }

  return ''
}

function buildNotarizeCredentialArgs() {
  if (
    process.env.APPLE_API_KEY &&
    process.env.APPLE_API_KEY_ID &&
    process.env.APPLE_API_ISSUER
  ) {
    return [
      '--key', process.env.APPLE_API_KEY,
      '--key-id', process.env.APPLE_API_KEY_ID,
      '--issuer', process.env.APPLE_API_ISSUER,
    ]
  }
  return [
    '--apple-id', process.env.APPLE_ID,
    '--password', process.env.APPLE_APP_SPECIFIC_PASSWORD,
    '--team-id', process.env.APPLE_TEAM_ID,
  ]
}

function notarizeMacDmg(dmgPath) {
  const credArgs = buildNotarizeCredentialArgs()
  console.log(`[release] Submitting ${path.basename(dmgPath)} to Apple notary (typically 5-15 minutes)...`)
  run('xcrun', ['notarytool', 'submit', dmgPath, '--wait', ...credArgs])

  console.log(`[release] Stapling notarization ticket to ${path.basename(dmgPath)}...`)
  runChecked('xcrun', ['stapler', 'staple', dmgPath], 'staple DMG')
}

function validateMacDmg(dmgPath) {
  console.log(`[release] Verifying notarized DMG ${path.basename(dmgPath)}...`)
  runChecked('codesign', ['--verify', '--strict', '--verbose=2', dmgPath], 'codesign DMG')
  runChecked('xcrun', ['stapler', 'validate', dmgPath], 'stapler DMG')
  runChecked(
    'spctl',
    ['-a', '-vvv', '-t', 'open', '--context', 'context:primary-signature', dmgPath],
    'Gatekeeper DMG',
  )
}

// Auto-updater ZIP carries the same signed .app as the DMG, but it is not
// individually notarized — the .app inside has no stapled ticket. Gatekeeper
// will accept it via online notary lookup (Apple recognizes the hash from the
// DMG submission). We only verify codesign integrity here.
function validateMacZip(zipPath) {
  const tempRoot = mkdtempSync(path.join(os.tmpdir(), 'biene-mac-zip-'))
  try {
    const extractDir = path.join(tempRoot, 'extract')
    ensureDir(extractDir)
    runChecked('ditto', ['-x', '-k', zipPath, extractDir], 'extract update ZIP')

    const appPath = findAppBundle(extractDir)
    if (!appPath) {
      console.error(`[release] No .app bundle found in update ZIP: ${zipPath}`)
      process.exit(1)
    }

    console.log(`[release] Verifying signature of .app inside ${path.basename(zipPath)}...`)
    runChecked(
      'codesign',
      ['--verify', '--deep', '--strict', '--verbose=2', appPath],
      'codesign zip .app',
    )
  } finally {
    cleanPath(tempRoot)
  }
}

function packageMac() {
  const { skipSigning } = getMacSigningEnv()

  const platformDir = path.join(releaseDir, 'mac-arm64')
  const updateDir = path.join(platformDir, 'update')
  const builderOutputDir = path.join(tempDir, 'mac-arm64')

  cleanPath(platformDir)
  cleanPath(builderOutputDir)
  ensureDir(updateDir)

  const signingOverrides = skipSigning
    ? ['--config.forceCodeSigning=false', '--config.mac.identity=null', '--config.mac.notarize=false']
    : []

  buildCore('darwin', 'arm64', 'core/dist/biene-core')

  // Single builder run produces both dmg and zip from the same signed/notarized .app,
  // halving signing + notarization time vs. running electron-builder twice.
  run('npm', [
    'run',
    'package:desktop',
    '--',
    '--mac', 'dmg', 'zip',
    '--arm64',
    `--config.directories.output=${builderOutputDir}`,
    ...signingOverrides,
  ])

  const dmgFiles = moveFiles(builderOutputDir, platformDir, (name) => name.endsWith('.dmg'))
  const updateFiles = moveFiles(builderOutputDir, updateDir, (name) => (
    name === 'latest-mac.yml' ||
    name.endsWith('.zip') ||
    name.endsWith('.zip.blockmap')
  ))

  if (!skipSigning) {
    const dmgPath = dmgFiles.find((name) => name.endsWith('.dmg'))
    const zipPath = updateFiles.find((name) => name.endsWith('.zip') && !name.endsWith('.zip.blockmap'))
    if (!dmgPath) throw new Error('Missing expected macOS DMG artifact.')
    if (!zipPath) throw new Error('Missing expected macOS update ZIP artifact.')
    notarizeMacDmg(dmgPath)
    validateMacDmg(dmgPath)
    validateMacZip(zipPath)
  }
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

if (mode === 'mac' || mode === 'all') {
  getMacSigningEnv()
}

buildRenderer()

if (mode === 'mac' || mode === 'all') {
  packageMac()
}

if (mode === 'win' || mode === 'all') {
  packageWin()
}

cleanPath(tempDir)
