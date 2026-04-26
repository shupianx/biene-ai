const { existsSync, mkdirSync, copyFileSync, rmSync } = require('fs')
const path = require('path')
const { spawnSync } = require('child_process')
const { appBuilderPath } = require('app-builder-bin')

const rootDir = path.resolve(__dirname, '..')
const sourcePng = path.join(rootDir, 'docs', 'logo.png')
const buildDir = path.join(rootDir, 'build')
const stagingDir = path.join(rootDir, 'release', '.icon-staging')

// macOS Big Sur+ icons sit inside a 1024 canvas with visual padding. Keep the
// visible tile around 824px so it does not look oversized next to system icons.
const CANVAS = 1024
const TILE = 824
const TILE_INSET = Math.round((CANVAS - TILE) / 2)
const RADIUS = 185

function ensureSource() {
  if (!existsSync(sourcePng)) {
    console.error(`[icon] Source not found: ${sourcePng}`)
    console.error('[icon] Drop a 1024×1024 square PNG (no rounded corners) at docs/logo.png and rerun.')
    process.exit(1)
  }
}

function run(command, args, input) {
  const result = spawnSync(command, args, {
    cwd: rootDir,
    stdio: input ? ['pipe', 'inherit', 'inherit'] : 'inherit',
    input,
  })
  if (result.status !== 0) {
    console.error(`[icon] ${command} ${args.join(' ')} failed (exit ${result.status})`)
    process.exit(result.status ?? 1)
  }
}

function applyMacIconMask(maskedPath) {
  // Use PIL to inset the icon and apply a rounded-corner mask. PIL ships with macOS Python 3
  // via Xcode CLT / Homebrew; bail with a clear message if missing.
  const script = `
import sys
try:
    from PIL import Image, ImageDraw
except ImportError:
    sys.stderr.write("Pillow not installed. Run: python3 -m pip install --user Pillow\\n")
    sys.exit(2)

src = Image.open(${JSON.stringify(sourcePng)}).convert("RGBA")
if src.size != (${TILE}, ${TILE}):
    src = src.resize((${TILE}, ${TILE}), Image.LANCZOS)

mask = Image.new("L", (${CANVAS}, ${CANVAS}), 0)
ImageDraw.Draw(mask).rounded_rectangle(
    (${TILE_INSET}, ${TILE_INSET}, ${TILE_INSET + TILE}, ${TILE_INSET + TILE}),
    radius=${RADIUS},
    fill=255,
)

out = Image.new("RGBA", (${CANVAS}, ${CANVAS}), (0, 0, 0, 0))
out.paste(src, (${TILE_INSET}, ${TILE_INSET}), mask.crop((${TILE_INSET}, ${TILE_INSET}, ${TILE_INSET + TILE}, ${TILE_INSET + TILE})))
out.save(${JSON.stringify(maskedPath)})
`
  run('python3', ['-'], script)
}

function packIcns(sourcePath, icnsPath) {
  const outDir = path.join(stagingDir, 'icns')
  mkdirSync(outDir, { recursive: true })
  run(appBuilderPath, [
    'icon',
    '--format=icns',
    `--root=${path.dirname(sourcePath)}`,
    `--input=${path.basename(sourcePath)}`,
    `--out=${outDir}`,
  ])
  copyFileSync(path.join(outDir, 'icon.icns'), icnsPath)
}

function main() {
  ensureSource()
  rmSync(stagingDir, { force: true, recursive: true })
  mkdirSync(stagingDir, { recursive: true })
  mkdirSync(buildDir, { recursive: true })

  // app-builder's icon converter detects size from the filename.
  const maskedPath = path.join(stagingDir, '1024x1024.png')
  const icnsPath = path.join(buildDir, 'icon.icns')
  const pngPath = path.join(buildDir, 'icon.png')

  console.log('[icon] applying macOS icon mask...')
  applyMacIconMask(maskedPath)

  console.log('[icon] packing icns...')
  packIcns(maskedPath, icnsPath)

  copyFileSync(maskedPath, pngPath)
  rmSync(stagingDir, { force: true, recursive: true })

  console.log(`[icon] wrote ${path.relative(rootDir, icnsPath)}`)
  console.log(`[icon] wrote ${path.relative(rootDir, pngPath)}`)
}

main()
