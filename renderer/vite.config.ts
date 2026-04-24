import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import Icons from 'unplugin-icons/vite'

export default defineConfig({
  base: './',
  plugins: [tailwindcss(), vue(), Icons({ compiler: 'vue3' })],
  build: {
    outDir: 'dist',
  },
  server: {
    // 'localhost' resolves to ::1 on Node 17+, placing Tinte's dev
    // server in the same IPv6 address family as Vite's default for
    // user-scaffolded projects. That way a project running inside an
    // agent workspace that tries to bind 5173 detects the conflict
    // and auto-bumps to 5174 instead of silently co-living on IPv4.
    host: 'localhost',
    port: 5173,
    strictPort: true,
  },
})
