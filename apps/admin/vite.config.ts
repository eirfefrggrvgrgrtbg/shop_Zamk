import { defineConfig } from 'vite'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [tailwindcss()] as any,
  server: {
    port: 3003,
    host: '127.0.0.1',
    strictPort: true,
  }
})
