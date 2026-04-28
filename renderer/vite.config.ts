import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import Icons from 'unplugin-icons/vite'

// The renderer is a child workspace whose own package.json version is
// always 0.0.0 (a Vite/Vue scaffolding default we don't track). The
// canonical app version lives in the root package.json — read it at
// build time and inject as __APP_VERSION__ so the About modal can
// display it without crossing the Electron bridge for static data.
const rootPkg = JSON.parse(
  readFileSync(resolve(__dirname, '..', 'package.json'), 'utf-8'),
) as { version?: string }
const appVersion = rootPkg.version ?? '0.0.0'

export default defineConfig({
  base: './',
  plugins: [tailwindcss(), vue(), Icons({ compiler: 'vue3' })],
  define: {
    __APP_VERSION__: JSON.stringify(appVersion),
  },
  build: {
    outDir: 'dist',
  },
  server: {
    // 'localhost' resolves to ::1 on Node 17+, placing Biene's dev
    // server in the same IPv6 address family as Vite's default for
    // user-scaffolded projects. That way a project running inside an
    // agent workspace that tries to bind 5173 detects the conflict
    // and auto-bumps to 5174 instead of silently co-living on IPv4.
    host: 'localhost',
    port: 5173,
    strictPort: true,
  },
})

