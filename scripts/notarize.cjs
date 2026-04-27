/* eslint-disable @typescript-eslint/no-var-requires */
// electron-builder afterSign hook — submits the freshly-signed .app to
// Apple's notary service via @electron/notarize.
//
// Why a Node-side hook (and not just `mac.notarize: true`):
//   • The release.cjs script needs an explicit knob to opt out of
//     notarization for fast --sign-only builds. Reading
//     BIENE_SKIP_NOTARIZE here is the cleanest cut-off point.
//   • @electron/notarize internally drives `xcrun notarytool submit
//     --wait` and then `xcrun stapler staple` directly on the .app
//     bundle, so the ticket is embedded inside the .app itself. Once
//     electron-builder packs that .app into the DMG/ZIP, the embedded
//     ticket travels with it — no separate DMG-stapling pass needed.
//   • Switching to this library removes ~70 lines of manual notarytool
//     glue (credential arg munging, polling, staple, spctl) from
//     release.cjs.

const path = require('node:path')
const { notarize } = require('@electron/notarize')

/** @type {(context: import('electron-builder').AfterPackContext) => Promise<void>} */
async function notarizeAfterSign(context) {
  const { electronPlatformName, appOutDir, packager } = context
  if (electronPlatformName !== 'darwin' && electronPlatformName !== 'mas') {
    return
  }

  if (process.env.BIENE_SKIP_MAC_SIGNING === '1') {
    console.log('[notarize] BIENE_SKIP_MAC_SIGNING=1 — skipping notarization (unsigned build).')
    return
  }
  if (process.env.BIENE_SKIP_NOTARIZE === '1') {
    console.log('[notarize] BIENE_SKIP_NOTARIZE=1 — skipping notarization (sign-only build).')
    return
  }

  const appName = packager.appInfo.productFilename
  const appPath = path.join(appOutDir, `${appName}.app`)

  console.log(`[notarize] Submitting ${appName}.app to Apple notary (typically 5–15 minutes)...`)
  const opts = resolveNotarizeOptions(appPath)
  await notarize(opts)
  console.log(`[notarize] ${appName}.app: notarized + stapled in place.`)
}

// @electron/notarize accepts two auth strategies: App Store Connect API
// key (preferred for CI — no MFA prompts) or Apple-ID + app-specific
// password. We probe for the same env vars that release.cjs validates
// up front so callers can swap between the two without renaming things.
function resolveNotarizeOptions(appPath) {
  if (process.env.APPLE_API_KEY && process.env.APPLE_API_ISSUER) {
    /** @type {Parameters<typeof notarize>[0]} */
    const opts = {
      appPath,
      appleApiKey: process.env.APPLE_API_KEY,
      appleApiIssuer: process.env.APPLE_API_ISSUER,
    }
    if (process.env.APPLE_API_KEY_ID) {
      opts.appleApiKeyId = process.env.APPLE_API_KEY_ID
    }
    return opts
  }

  if (
    process.env.APPLE_ID
    && process.env.APPLE_APP_SPECIFIC_PASSWORD
    && process.env.APPLE_TEAM_ID
  ) {
    return {
      appPath,
      appleId: process.env.APPLE_ID,
      appleIdPassword: process.env.APPLE_APP_SPECIFIC_PASSWORD,
      teamId: process.env.APPLE_TEAM_ID,
    }
  }

  throw new Error(
    '[notarize] Missing Apple notary credentials. Provide one of:\n'
      + '  • APPLE_API_KEY (path to .p8) + APPLE_API_ISSUER [+ APPLE_API_KEY_ID]\n'
      + '  • APPLE_ID + APPLE_APP_SPECIFIC_PASSWORD + APPLE_TEAM_ID\n'
      + 'Or set BIENE_SKIP_NOTARIZE=1 / BIENE_SKIP_MAC_SIGNING=1 to opt out.',
  )
}

module.exports = notarizeAfterSign
module.exports.default = notarizeAfterSign
