const { existsSync, mkdirSync, mkdtempSync, readdirSync, renameSync, rmSync } = require('fs')
const path = require('path')
const os = require('os')
const { spawnSync } = require('child_process')

const rootDir = path.resolve(__dirname, '..')
const releaseDir = path.join(rootDir, 'release')
const tempDir = path.join(releaseDir, '.tmp')
const builderCacheDir = path.join(rootDir, '.cache', 'electron-builder')

// Mac-only build pipeline. Windows builds run in GitHub Actions
// (.github/workflows/build-win.yml) on a real Windows runner — no wine
// or Rosetta required.
//
// Default flow signs + notarizes + staples + verifies before producing
// artifacts ready to publish to GitHub. --sign-only is the fast local
// iteration path: signs the .app for testing on the developer's machine
// but skips notarization (5–15 minutes against Apple's servers) and the
// Gatekeeper-staple verification that depends on it.
const flags = new Set(process.argv.slice(2).filter((arg) => arg.startsWith('--')))
const macSignOnly = flags.has('--sign-only')
let macSigningEnv = null

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
    return { skipSigning: true, signOnly: false }
  }

  if (!hasMacSigningIdentity()) {
    console.error('[release] Missing Developer ID Application signing identity.')
    console.error('[release] Install a valid certificate in the keychain, or provide CSC_LINK/CSC_NAME for electron-builder.')
    console.error('[release] Set BIENE_SKIP_MAC_SIGNING=1 only for local unsigned test builds.')
    process.exit(1)
  }

  // Sign-only path stops here: no Apple notary credentials required, no
  // notarytool / staple / spctl pass — just produce a signed bundle
  // that runs locally for the developer.
  if (macSignOnly) {
    console.log('[release] --sign-only: signing artifacts without submitting to Apple notary.')
    return { skipSigning: false, signOnly: true }
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
    console.error('[release] Set them, pass --sign-only to skip notarization, or pass BIENE_SKIP_MAC_SIGNING=1 to build an unsigned artifact for local testing.')
    process.exit(1)
  }

  return { skipSigning: false, signOnly: false }
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

// Validation runs on the .app extracted from the auto-updater ZIP.
// Both DMG and ZIP carry the same signed bundle that the afterSign hook
// already submitted to Apple, so checking once (and the stapled ticket
// embedded inside the .app) is enough to confirm the build.
function validateMacApp({ requireStapled }) {
  const platformDir = path.join(releaseDir, 'mac-arm64')
  const updateDir = path.join(platformDir, 'update')
  const zips = readdirSync(updateDir).filter(
    (name) => name.endsWith('.zip') && !name.endsWith('.zip.blockmap'),
  )
  if (zips.length === 0) {
    console.error(`[release] No update ZIP under ${updateDir} — nothing to validate.`)
    process.exit(1)
  }
  const zipPath = path.join(updateDir, zips[0])

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

    if (requireStapled) {
      // @electron/notarize stapled the ticket inside the .app bundle
      // itself; this final stapler check confirms the round-trip with
      // Apple's notary service actually completed.
      console.log(`[release] Validating stapled notarization ticket on .app...`)
      runChecked('xcrun', ['stapler', 'validate', appPath], 'stapler .app')
    }
  } finally {
    cleanPath(tempRoot)
  }
}

function packageMac() {
  const { skipSigning, signOnly } = getMacSigningEnv()

  const platformDir = path.join(releaseDir, 'mac-arm64')
  const updateDir = path.join(platformDir, 'update')
  const builderOutputDir = path.join(tempDir, 'mac-arm64')

  cleanPath(platformDir)
  cleanPath(builderOutputDir)
  ensureDir(updateDir)

  const signingOverrides = skipSigning
    ? ['--config.forceCodeSigning=false', '--config.mac.identity=null']
    : []

  // The afterSign hook (scripts/notarize.cjs) reads BIENE_SKIP_NOTARIZE
  // to decide whether to submit the .app to Apple. Setting it for
  // --sign-only builds keeps the hook in place without paying the
  // 5–15-minute notarization tax during fast iteration.
  const builderEnv = {}
  if (signOnly) builderEnv.BIENE_SKIP_NOTARIZE = '1'

  buildCore('darwin', 'arm64', 'core/dist/biene-core')

  // Single builder run produces both dmg and zip from the same signed +
  // (when applicable) notarized .app — afterSign stapled the ticket
  // inside the .app, so both archives carry the offline-valid bundle.
  run('npm', [
    'run',
    'package:desktop',
    '--',
    '--mac', 'dmg', 'zip',
    '--arm64',
    `--config.directories.output=${builderOutputDir}`,
    ...signingOverrides,
  ], builderEnv)

  moveFiles(builderOutputDir, platformDir, (name) => name.endsWith('.dmg'))
  moveFiles(builderOutputDir, updateDir, (name) => (
    name === 'latest-mac.yml' ||
    name.endsWith('.zip') ||
    name.endsWith('.zip.blockmap')
  ))

  if (skipSigning) return

  if (signOnly) {
    validateMacApp({ requireStapled: false })
    console.log('[release] --sign-only build complete (no notarization).')
    console.log('[release]   First-run users will need to right-click → Open or remove the quarantine xattr.')
    return
  }

  validateMacApp({ requireStapled: true })
  console.log('[release] mac build complete: .app signed + notarized + ticket stapled in place.')
}

ensureDir(builderCacheDir)
ensureDir(releaseDir)
ensureDir(tempDir)
removeLegacyReleaseArtifacts()

getMacSigningEnv()
buildRenderer()
packageMac()

cleanPath(tempDir)
