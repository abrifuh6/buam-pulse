import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// The status page is deliberately a SEPARATE origin from the dashboard: it is
// public, unauthenticated, and customers link to it during incidents. In dev we
// still proxy /api so there is no CORS friction locally.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5174,
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
})
