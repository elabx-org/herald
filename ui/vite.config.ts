import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    proxy: {
      '/v1': 'http://localhost:8765',
      '/v2': 'http://localhost:8765',
      '/ping': 'http://localhost:8765',
    }
  },
  build: {
    outDir: '../internal/ui/dist',
    emptyOutDir: true,
  }
})
