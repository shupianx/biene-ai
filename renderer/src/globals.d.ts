// Ambient globals injected by Vite at build time. This file has no
// `export` statement so its declarations remain in the global scope —
// putting them inside a module file (e.g. electron.d.ts which exports
// `{}`) would make them module-local and unreachable from .vue files.

/**
 * App version, read from the root package.json by vite.config.ts and
 * substituted in via Vite's `define`. Kept here rather than crossing
 * the Electron bridge because version is static at build time and
 * shouldn't pay an IPC round-trip cost.
 */
declare const __APP_VERSION__: string
