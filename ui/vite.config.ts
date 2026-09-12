import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  base: '/',
  build: {
    rollupOptions: { input: ['index.html', 'rdp.html'] },
    outDir: 'dist',
  },
  server: {
    proxy: {
      '/api': { target: 'http://localhost:8080', ws: true },
    },
  },
})
