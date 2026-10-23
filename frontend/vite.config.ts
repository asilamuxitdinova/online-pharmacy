import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

function proxy(target: string) {
  const direct = { target, changeOrigin: false }
  return {
    '/api': { target, changeOrigin: false, rewrite: (p: string) => p.replace(/^\/api/, '') },
    // Pages the backend renders itself, whose links and forms are absolute.
    '^/oauth/(authorize|token)': direct,
    '/docs': direct,
    '/openapi.yaml': direct,
  }
}

// The backend has no CORS: in development every API call goes through this
// proxy, so the browser only ever talks to one origin.
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const target = env.PHARMACY_API ?? 'http://127.0.0.1:8080'
  return {
    plugins: [react()],
    server: { port: 5173, proxy: proxy(target) },
    preview: { port: 4173, proxy: proxy(target) },
    build: {
      chunkSizeWarningLimit: 1200,
      rollupOptions: {
        output: {
          manualChunks(id) {
            if (id.includes('three') || id.includes('@react-three')) return 'three'
          },
        },
      },
    },
  }
})
