import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const backend = process.env.VITE_BACKEND_INTERNAL ?? 'http://localhost:8080'
const wsBackend = backend.replace(/^http/, 'ws')

export default defineConfig({
  plugins: [react()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': { target: backend, changeOrigin: true },
      '/ws': { target: wsBackend, ws: true, changeOrigin: true },
    },
  },
})
