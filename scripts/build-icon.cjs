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

// Windows uses a full-bleed square icon — Windows applies its own visual
// treatment in the taskbar, so the macOS-style padded + rounded mask would
// look undersized. Pack from the raw 1024×1024 source instead of the masked
// version that goes into icon.icns / icon.png.
//
// Pre-resize the source PNG to each standard Windows size with .NET's
// System.Drawing (via PowerShell) and feed all of them to app-builder so
// the resulting ICO contains every entry. Single-size ICOs render as the
// default icon in Explorer's small/list views.
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
  if (process.platform !== 'win32') {
    // Mac/Linux usually have ImageMagick or the Python path; the existing
    // Pillow flow can be extended later if we need ICO from those hosts.
    // For now, callers on non-Windows produce a single-size ICO via the
    // legacy single-input path.
    throw new Error('resizePngForIco currently runs only on Windows.')
  }

  // PowerShell + System.Drawing handles PNG resize cleanly with no extra
  // dependencies. HighQualityBicubic gives crisp small icons.
  const script = `
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing
$src = [System.Drawing.Image]::FromFile([string]'${sourcePath.replace(/\\/g, '\\\\')}')
foreach ($size in ${sizes.join(',')}) {
  $bmp = New-Object System.Drawing.Bitmap $size, $size
  $g = [System.Drawing.Graphics]::FromImage($bmp)
  $g.InterpolationMode = 'HighQualityBicubic'
  $g.PixelOffsetMode = 'HighQuality'
  $g.SmoothingMode = 'HighQuality'
  $g.DrawImage($src, 0, 0, $size, $size)
  $out = Join-Path '${outDir.replace(/\\/g, '\\\\')}' ("$size" + 'x' + "$size" + '.png')
  $bmp.Save($out, [System.Drawing.Imaging.ImageFormat]::Png)
  $g.Dispose(); $bmp.Dispose()
}
$src.Dispose()
`
  run('powershell', ['-NoProfile', '-NonInteractive', '-Command', script])
}

// Pillow only matters for the macOS mask. Probe python3 once so we can
// skip the mac assets cleanly on Windows where python3 typically isn't
// present (or resolves to the Microsoft Store stub that exits 9009).
function hasPython3() {
  const result = spawnSync('python3', ['--version'], { stdio: 'ignore' })
  return result.status === 0
}

function main() {
  ensureSource()
  rmSync(stagingDir, { force: true, recursive: true })
  mkdirSync(stagingDir, { recursive: true })
  mkdirSync(buildDir, { recursive: true })

  // app-builder's icon converter detects size from the filename.
  const maskedPath = path.join(stagingDir, '1024x1024.png')
  const icnsPath = path.join(buildDir, 'icon.icns')
  const icoPath = path.join(buildDir, 'icon.ico')
  const pngPath = path.join(buildDir, 'icon.png')

  // ICO first: doesn't depend on Pillow, so this works on Windows too.
  console.log('[icon] packing ico...')
  packIco(sourcePng, icoPath)
  console.log(`[icon] wrote ${path.relative(rootDir, icoPath)}`)

  if (!hasPython3()) {
    console.warn('[icon] python3 / Pillow not available — skipping icon.icns + icon.png (mac assets). Run on macOS to refresh those.')
    rmSync(stagingDir, { force: true, recursive: true })
    return
  }

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
