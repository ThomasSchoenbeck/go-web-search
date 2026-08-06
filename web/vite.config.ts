import { defineConfig, loadEnv } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// The Go serve listener (config.toml `server.addr`, default 0.0.0.0:8082).
// Override with VITE_PROXY_TARGET when the backend runs elsewhere.
const defaultProxyTarget = 'http://localhost:8082'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, '.', 'VITE_')
  const target = env.VITE_PROXY_TARGET || defaultProxyTarget

  return {
    base: '/',
    plugins: [svelte()],
    server: {
      proxy: {
        '/api': { target, changeOrigin: true },
        '/mcp': { target, changeOrigin: true },
        '/healthz': { target, changeOrigin: true },
      },
    },
  }
})
