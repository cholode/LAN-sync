import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const backend = env.VITE_DEV_BACKEND || 'http://127.0.0.1:8080'

  return {
    plugins: [vue()],
    server: {
      host: '0.0.0.0',
      port: 5173,
      proxy: {
        '/api': {
          target: backend,
          changeOrigin: true,
          ws: true,
        },
      },
    },
    build: {
      outDir: 'dist',
      assetsDir: 'assets',
      sourcemap: false,
    },
  }
})
