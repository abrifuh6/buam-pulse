import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Same-origin in dev, matching production: the browser only ever talks to
// this server, and /api is proxied to the Go API. No CORS anywhere.
export default defineConfig({
  // Where this app is served from.
  //
  // A static build bakes every asset URL in at build time. Served at /app with
  // base left at '/', the page loads and then asks for /assets/index.js — which
  // the marketing site answers instead. The symptom is a blank page with no
  // error, which is why this is worth stating explicitly rather than assuming.
  //
  // Configurable because the dashboard sits at / locally and behind /app in
  // AWS. Hostname separation would remove the need for this entirely.
  base: process.env.VITE_BASE ?? '/',

  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
})
