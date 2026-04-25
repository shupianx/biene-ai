const { existsSync, mkdirSync, copyFileSync, rmSync } = require('fs')
const path = require('path')
const { spawnSync } = require('child_process')

const rootDir = path.resolve(__dirname, '..')
const sourcePng = path.join(rootDir, 'docs', 'logo.png')
const buildDir = path.join(rootDir, 'build')
const stagingDir = path.join(rootDir, 'release', '.icon-staging')

// macOS Big Sur+ icon mask: rounded "squircle" approximated as a rounded
// rectangle with radius 185 on a 1024 canvas (≈22.5% of the inner 824 area,
// matching Apple's icon template).
const CANVAS = 1024
const RADIUS = 185

const ICONSET = [
  ['icon_16x16.png', 16],
  ['icon_16x16@2x.png', 32],
  ['icon_32x32.png', 32],
  ['icon_32x32@2x.png', 64],
  ['icon_128x128.png', 128],
  ['icon_128x128@2x.png', 256],
  ['icon_256x256.png', 256],
  ['icon_256x256@2x.png', 512],
  ['icon_512x512.png', 512],
  ['icon_512x512@2x.png', 1024],
]

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

function applySquircleMask(maskedPath) {
  // Use PIL to apply the rounded-corner mask. PIL ships with macOS Python 3
  // via Xcode CLT / Homebrew; bail with a clear message if missing.
  const script = `
import sys
try:
    from PIL import Image, ImageDraw
except ImportError:
    sys.stderr.write("Pillow not installed. Run: python3 -m pip install --user Pillow\\n")
    sys.exit(2)

src = Image.open(${JSON.stringify(sourcePng)}).convert("RGBA")
if src.size != (${CANVAS}, ${CANVAS}):
    src = src.resize((${CANVAS}, ${CANVAS}), Image.LANCZOS)

mask = Image.new("L", (${CANVAS}, ${CANVAS}), 0)
ImageDraw.Draw(mask).rounded_rectangle((0, 0, ${CANVAS}, ${CANVAS}), radius=${RADIUS}, fill=255)

out = Image.new("RGBA", (${CANVAS}, ${CANVAS}), (0, 0, 0, 0))
out.paste(src, (0, 0), mask)
out.save(${JSON.stringify(maskedPath)})
`
  run('python3', ['-'], script)
}

function buildIconset(maskedPath, iconsetDir) {
  mkdirSync(iconsetDir, { recursive: true })
  for (const [name, size] of ICONSET) {
    const dst = path.join(iconsetDir, name)
    run('sips', ['-z', String(size), String(size), maskedPath, '--out', dst])
  }
}

function packIcns(iconsetDir, icnsPath) {
  run('iconutil', ['-c', 'icns', iconsetDir, '-o', icnsPath])
}

function main() {
  ensureSource()
  rmSync(stagingDir, { force: true, recursive: true })
  mkdirSync(stagingDir, { recursive: true })
  mkdirSync(buildDir, { recursive: true })

  const maskedPath = path.join(stagingDir, 'icon-1024.png')
  const iconsetDir = path.join(stagingDir, 'icon.iconset')
  const icnsPath = path.join(buildDir, 'icon.icns')
  const pngPath = path.join(buildDir, 'icon.png')

  console.log('[icon] applying squircle mask...')
  applySquircleMask(maskedPath)

  console.log('[icon] generating iconset...')
  buildIconset(maskedPath, iconsetDir)

  console.log('[icon] packing icns...')
  packIcns(iconsetDir, icnsPath)

  copyFileSync(maskedPath, pngPath)
  rmSync(stagingDir, { force: true, recursive: true })

  console.log(`[icon] wrote ${path.relative(rootDir, icnsPath)}`)
  console.log(`[icon] wrote ${path.relative(rootDir, pngPath)}`)
}

main()
