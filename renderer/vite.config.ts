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
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
  },
})
