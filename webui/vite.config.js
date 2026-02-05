import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    host: '0.0.0.0',
    proxy: {
      '/session': 'http://localhost:3000',
      '/users': 'http://localhost:3000',
      '/conversations': 'http://localhost:3000',
      '/messages': 'http://localhost:3000',
      '/groups': 'http://localhost:3000',
      '/photos': 'http://localhost:3000',
    }
  },
  preview: {
    port: 4173,
    host: '0.0.0.0'
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true
  },
  define: {
    // Do not modify this constant, it is used in the evaluation.
    "__API_URL__": JSON.stringify("http://localhost:3000"),
  }
})
