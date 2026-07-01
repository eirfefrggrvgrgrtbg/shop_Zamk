import { defineConfig } from 'vite';
import tailwindcss from '@tailwindcss/vite';
export default defineConfig({
    plugins: [tailwindcss()],
    server: {
        port: 3002,
        host: '127.0.0.1',
        strictPort: true,
    }
});
