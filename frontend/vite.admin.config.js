import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'node:path'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const adminBackend = env.VITE_ADMIN_BACKEND || 'http://127.0.0.1:8081'
	return {
	  plugins: [vue()],
	  define: { 'import.meta.env.VITE_ADMIN_STANDALONE': JSON.stringify('true') },
    server: {
      host: '0.0.0.0',
      port: Number(env.ADMIN_FRONTEND_PORT || 5174),
      proxy: { '/api': { target: adminBackend, changeOrigin: true } },
    },
    build: {
      outDir: 'admin-dist',
      assetsDir: 'assets',
      sourcemap: false,
      rollupOptions: { input: resolve(process.cwd(), 'admin.html') },
    },
  }
})
