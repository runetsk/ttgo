import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import pkg from './package.json' with { type: 'json' }

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  // The Help footer used to hardcode this and had drifted two minor versions behind
  // (v0.3.0 against a v0.5.0 tag). Read it from package.json, which the release process
  // already bumps, so it cannot go stale again.
  define: {
    __APP_VERSION__: JSON.stringify(pkg.version),
  },
  server: {
    host: '0.0.0.0',
    proxy: {
      '/api': {
        // Override when :8080 is taken (e.g. TTGO_DEV_API_TARGET=http://localhost:8090)
        target: process.env.TTGO_DEV_API_TARGET || 'http://localhost:8080',
        ws: true
      }
    }
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          'vendor-react': ['react', 'react-dom', 'react-router-dom'],
          'vendor-charts': ['recharts'],
          'vendor-editor': ['@tiptap/react', '@tiptap/starter-kit'],
          'vendor-dnd': ['@dnd-kit/core', '@dnd-kit/sortable', '@dnd-kit/modifiers', '@dnd-kit/utilities'],
        },
      },
    },
    sourcemap: true,
  },
})
