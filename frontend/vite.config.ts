import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'node:path'

// In dev, API and media are proxied to the Go backend so the app is same-origin everywhere.
export default defineConfig({
  plugins: [react()],
  resolve: { alias: { '@': path.resolve(__dirname, 'src') } },
  server: {
    host: true,
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
      '/media': 'http://localhost:8080',
    },
  },
  build: { target: 'es2020', sourcemap: false, chunkSizeWarningLimit: 700 },
})
