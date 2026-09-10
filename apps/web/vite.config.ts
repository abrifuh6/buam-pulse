import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Same-origin in dev, matching production: the browser only ever talks to
// this server, and /api is proxied to the Go API. No CORS anywhere.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
})
