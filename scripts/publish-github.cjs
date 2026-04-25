const { existsSync, readdirSync, readFileSync, copyFileSync, mkdirSync, rmSync } = require('fs')
const path = require('path')
const { spawnSync } = require('child_process')

const rootDir = path.resolve(__dirname, '..')
const releaseDir = path.join(rootDir, 'release')

const platforms = process.argv.slice(2).filter(Boolean)
if (platforms.length === 0) platforms.push('mac')

const validPlatforms = new Set(['mac', 'win'])
for (const p of platforms) {
  if (!validPlatforms.has(p)) {
    console.error(`Unknown platform: ${p}. Use one or more of: mac, win`)
    process.exit(1)
  }
}

function readVersion() {
  const pkg = JSON.parse(readFileSync(path.join(rootDir, 'package.json'), 'utf8'))
  if (typeof pkg.version !== 'string' || !pkg.version) {
    console.error('package.json is missing a "version" field.')
    process.exit(1)
  }
  return pkg.version
}

function ensureGhAvailable() {
  const ver = spawnSync('gh', ['--version'], { stdio: 'ignore' })
  if (ver.status !== 0) {
    console.error('GitHub CLI (gh) is not installed.')
    console.error('  Install: brew install gh')
    console.error('  Auth:    gh auth login')
    process.exit(1)
  }

  const auth = spawnSync('gh', ['auth', 'status'], { stdio: 'ignore' })
  if (auth.status !== 0) {
    console.error('GitHub CLI is installed but not authenticated.')
    console.error('  Run: gh auth login')
    process.exit(1)
  }
}

function findOne(dir, predicate, label) {
  if (!existsSync(dir)) {
    throw new Error(`Directory not found: ${dir}`)
  }
  const matches = readdirSync(dir).filter(predicate)
  if (matches.length === 0) {
    throw new Error(`No ${label} found in ${dir}`)
  }
  if (matches.length > 1) {
    throw new Error(`Multiple ${label} candidates in ${dir}: ${matches.join(', ')}`)
  }
  return path.join(dir, matches[0])
}

function collectMacAssets(stagingDir) {
  const platformDir = path.join(releaseDir, 'mac-arm64')
  const updateDir = path.join(platformDir, 'update')

  const dmgPath = findOne(platformDir, (n) => n.endsWith('.dmg'), 'DMG')
  const zipPath = findOne(updateDir, (n) => n.endsWith('.zip') && !n.endsWith('.blockmap.zip'), 'update ZIP')
  const blockmapPath = findOne(updateDir, (n) => n.endsWith('.zip.blockmap'), 'ZIP blockmap')
  const ymlPath = path.join(updateDir, 'latest-mac.yml')
  if (!existsSync(ymlPath)) {
    throw new Error(`Missing latest-mac.yml at ${ymlPath}`)
  }

  // Rename DMG to a stable filename for landing-page links.
  // ZIP/blockmap/yml keep their versioned names so auto-updater across versions works.
  const stableDmg = path.join(stagingDir, 'Biene-arm64.dmg')
  copyFileSync(dmgPath, stableDmg)

  return [stableDmg, zipPath, blockmapPath, ymlPath]
}

function collectWinAssets(stagingDir) {
  const platformDir = path.join(releaseDir, 'win-x64')
  const zipPath = findOne(platformDir, (n) => n.endsWith('.zip'), 'Windows ZIP')

  const stableZip = path.join(stagingDir, 'Biene-x64-win.zip')
  copyFileSync(zipPath, stableZip)
  return [stableZip]
}

function releaseExists(tag) {
  const result = spawnSync('gh', ['release', 'view', tag, '--json', 'tagName'], {
    stdio: ['ignore', 'pipe', 'pipe'],
  })
  return result.status === 0
}

function createDraftRelease(tag) {
  console.log(`[publish] Creating draft release ${tag}...`)
  const result = spawnSync('gh', [
    'release', 'create', tag,
    '--draft',
    '--title', tag,
    '--notes', '',
  ], { stdio: 'inherit', cwd: rootDir })
  if (result.status !== 0) {
    process.exit(result.status ?? 1)
  }
}

function uploadAssets(tag, files) {
  console.log(`[publish] Uploading ${files.length} asset(s) to ${tag}...`)
  const result = spawnSync('gh', [
    'release', 'upload', tag,
    ...files,
    '--clobber',
  ], { stdio: 'inherit', cwd: rootDir })
  if (result.status !== 0) {
    process.exit(result.status ?? 1)
  }
}

function showReleaseUrl(tag) {
  const result = spawnSync('gh', ['release', 'view', tag, '--json', 'url', '-q', '.url'], {
    stdio: ['ignore', 'pipe', 'inherit'],
  })
  const url = result.stdout?.toString().trim()
  if (url) {
    console.log(`[publish] Draft release ready for review: ${url}`)
    console.log('[publish] Review notes/assets, then click "Publish release" on GitHub.')
  }
}

function main() {
  ensureGhAvailable()

  const version = readVersion()
  const tag = `v${version}`

  const stagingDir = path.join(releaseDir, '.publish-staging')
  rmSync(stagingDir, { force: true, recursive: true })
  mkdirSync(stagingDir, { recursive: true })

  const assets = []
  for (const platform of platforms) {
    if (platform === 'mac') assets.push(...collectMacAssets(stagingDir))
    if (platform === 'win') assets.push(...collectWinAssets(stagingDir))
  }

  console.log('[publish] Assets to upload:')
  for (const a of assets) console.log(`  - ${path.relative(rootDir, a)}`)

  if (releaseExists(tag)) {
    console.log(`[publish] Release ${tag} already exists — uploading assets to it (overwriting same-name files).`)
  } else {
    createDraftRelease(tag)
  }

  uploadAssets(tag, assets)
  rmSync(stagingDir, { force: true, recursive: true })
  showReleaseUrl(tag)
}

try {
  main()
} catch (err) {
  console.error(`[publish] ${err instanceof Error ? err.message : err}`)
  process.exit(1)
}
