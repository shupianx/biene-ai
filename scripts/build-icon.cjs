const { existsSync, mkdirSync, copyFileSync, readFileSync, writeFileSync, rmSync } = require('fs')
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

// Standard Windows icon sizes. Explorer picks per view: 16/32 for tree view,
// 48 for medium icons, 256 for "extra large". An ICO with only one size at
// the high end falls back to the default Electron icon in tree/list views.
const WIN_ICON_SIZES = [16, 24, 32, 48, 64, 128, 256]

// Pack a multi-size ICO from the masked 1024×1024 PNG so Biene.exe and
// Explorer show the same rounded-corner square as the macOS icon. Windows
// 11 doesn't apply any rounded mask to taskbar/Explorer icons, so the
// rounded corners must be baked in here.
//
// Pre-resize the masked source to each standard Windows size and feed
// every size to packPngsAsIco — single-size ICOs render as the default
// icon in Explorer's small/list views.
function packIco(sourcePath, icoPath) {
  const resizedDir = path.join(stagingDir, 'ico-sizes')
  mkdirSync(resizedDir, { recursive: true })

  resizePngForIco(sourcePath, resizedDir, WIN_ICON_SIZES)

  const pngs = WIN_ICON_SIZES.map((size) => ({
    size,
    data: readFileSync(path.join(resizedDir, `${size}x${size}.png`)),
  }))
  writeFileSync(icoPath, packPngsAsIco(pngs))
}

// Pack PNG buffers into a multi-size ICO container. Vista+ supports PNG
// entries directly, so we don't need to convert to BMP — just write the
// 6-byte ICONDIR header, one 16-byte ICONDIRENTRY per image, then the raw
// PNG bytes. Sizes 0 in the entry mean 256.
function packPngsAsIco(entries) {
  const headerSize = 6
  const dirEntrySize = 16
  const dirSize = headerSize + dirEntrySize * entries.length
  const totalSize = dirSize + entries.reduce((acc, e) => acc + e.data.length, 0)

  const buf = Buffer.alloc(totalSize)
  buf.writeUInt16LE(0, 0)              // reserved
  buf.writeUInt16LE(1, 2)              // type = 1 (icon)
  buf.writeUInt16LE(entries.length, 4) // image count

  let imageOffset = dirSize
  entries.forEach((entry, idx) => {
    const off = headerSize + idx * dirEntrySize
    const dim = entry.size >= 256 ? 0 : entry.size
    buf.writeUInt8(dim, off)            // width
    buf.writeUInt8(dim, off + 1)        // height
    buf.writeUInt8(0, off + 2)          // color palette count
    buf.writeUInt8(0, off + 3)          // reserved
    buf.writeUInt16LE(1, off + 4)       // color planes
    buf.writeUInt16LE(32, off + 6)      // bits per pixel
    buf.writeUInt32LE(entry.data.length, off + 8)  // image bytes
    buf.writeUInt32LE(imageOffset, off + 12)       // image offset
    entry.data.copy(buf, imageOffset)
    imageOffset += entry.data.length
  })

  return buf
}

function resizePngForIco(sourcePath, outDir, sizes) {
  // Pillow is already a dependency of the macOS mask path; reusing it for
  // the ICO resizes keeps the script Mac-runnable end-to-end and avoids
  // pulling in ImageMagick or shelling out to PowerShell.
  const script = `
import sys
try:
    from PIL import Image
except ImportError:
    sys.stderr.write("Pillow not installed. Run: python3 -m pip install --user Pillow\\n")
    sys.exit(2)

src = Image.open(${JSON.stringify(sourcePath)}).convert("RGBA")
out_dir = ${JSON.stringify(outDir)}
for size in [${sizes.join(', ')}]:
    src.resize((size, size), Image.LANCZOS).save(f"{out_dir}/{size}x{size}.png")
`
  run('python3', ['-'], script)
}

function resizeMaskedToPng(maskedPath, outPath, size) {
  const script = `
import sys
try:
    from PIL import Image
except ImportError:
    sys.stderr.write("Pillow not installed. Run: python3 -m pip install --user Pillow\\n")
    sys.exit(2)

src = Image.open(${JSON.stringify(maskedPath)}).convert("RGBA")
src.resize((${size}, ${size}), Image.LANCZOS).save(${JSON.stringify(outPath)})
`
  run('python3', ['-'], script)
}

function ensurePython3() {
  const result = spawnSync('python3', ['--version'], { stdio: 'ignore' })
  if (result.status !== 0) {
    console.error('[icon] python3 not found. Install it (e.g. via Xcode CLT or Homebrew) and `python3 -m pip install --user Pillow`.')
    process.exit(1)
  }
}

function main() {
  ensureSource()
  ensurePython3()
  rmSync(stagingDir, { force: true, recursive: true })
  mkdirSync(stagingDir, { recursive: true })
  mkdirSync(buildDir, { recursive: true })

  const maskedPath = path.join(stagingDir, '1024x1024.png')
  const icnsPath = path.join(buildDir, 'icon.icns')
  const icoPath = path.join(buildDir, 'icon.ico')
  const pngPath = path.join(buildDir, 'icon.png')
  const winPngPath = path.join(buildDir, 'icon-win.png')

  // Mask first: every downstream asset (icns, ico, icon.png, icon-win.png)
  // is the same rounded-square 1024×1024 image at different sizes/formats.
  console.log('[icon] applying rounded-corner mask...')
  applyMacIconMask(maskedPath)

  console.log('[icon] packing ico...')
  packIco(maskedPath, icoPath)
  console.log(`[icon] wrote ${path.relative(rootDir, icoPath)}`)

  console.log('[icon] packing icns...')
  packIcns(maskedPath, icnsPath)
  console.log(`[icon] wrote ${path.relative(rootDir, icnsPath)}`)

  copyFileSync(maskedPath, pngPath)
  console.log(`[icon] wrote ${path.relative(rootDir, pngPath)}`)

  // BrowserWindow.icon on Windows takes a small PNG; 256 is plenty for
  // taskbar / title bar at any DPI. The rounded corners come from the
  // masked source — Windows itself doesn't round runtime window icons.
  console.log('[icon] resizing icon-win.png (256×256)...')
  resizeMaskedToPng(maskedPath, winPngPath, 256)
  console.log(`[icon] wrote ${path.relative(rootDir, winPngPath)}`)

  rmSync(stagingDir, { force: true, recursive: true })
}

main()
