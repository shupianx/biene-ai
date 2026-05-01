// electron-builder afterPack hook — embeds build/icon.ico into Biene.exe
// on Windows builds. Runs between win-unpacked and the zip target.
//
// We can't let electron-builder do this itself: enabling its built-in
// signAndEditExecutable path makes it download the winCodeSign archive,
// whose macOS dylib symlinks fail to extract on Windows without admin
// rights or Developer Mode. So release-win.cjs disables that path and we
// run rcedit directly via app-builder's bundled binary.

const path = require('node:path')
const { existsSync } = require('node:fs')
const { spawnSync } = require('node:child_process')

const ROOT_DIR = path.join(__dirname, '..')
// electron-winstaller (a transitive dep of electron-builder) ships rcedit.exe
// as a vendor binary. Using it directly avoids both the app-builder rcedit
// wrapper (which wants base64-encoded args + downloads winCodeSign) and the
// rcedit-x64.exe inside winCodeSign itself (which fails to extract on
// Windows without admin / Developer Mode because of macOS dylib symlinks).
const RCEDIT_EXE = path.join(
  ROOT_DIR, 'node_modules', 'electron-winstaller', 'vendor', 'rcedit.exe',
)

/** @type {(context: import('electron-builder').AfterPackContext) => Promise<void>} */
async function setWindowsIcon(context) {
  if (context.electronPlatformName !== 'win32') return

  const productName = context.packager.appInfo.productFilename
  const exePath = path.join(context.appOutDir, `${productName}.exe`)
  const iconPath = path.join(ROOT_DIR, 'build', 'icon.ico')

  if (!existsSync(iconPath)) {
    console.warn(`[win-rcedit] ${iconPath} missing — run \`npm run build:icon\` first.`)
    return
  }
  if (!existsSync(exePath)) {
    throw new Error(`[win-rcedit] expected ${exePath} but it does not exist`)
  }
  if (!existsSync(RCEDIT_EXE)) {
    throw new Error(`[win-rcedit] ${RCEDIT_EXE} missing — install dependencies?`)
  }

  const result = spawnSync(
    RCEDIT_EXE,
    [exePath, '--set-icon', iconPath],
    { stdio: 'inherit' },
  )
  if (result.status !== 0) {
    throw new Error(`[win-rcedit] rcedit failed (exit ${result.status ?? 'unknown'})`)
  }

  console.log(`[win-rcedit] embedded ${path.basename(iconPath)} into ${productName}.exe`)
}

module.exports = setWindowsIcon
module.exports.default = setWindowsIcon
