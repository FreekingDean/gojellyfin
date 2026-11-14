import preact from '@preact/preset-vite';
import { defineConfig } from 'vite';

export default defineConfig({
  base: '/admin/',
  plugins: [preact()],
  build: {
    outDir: '../internal/server/admin/dist',
    emptyOutDir: false,
  },
  server: {
    proxy: {
      '^/(?!admin)': 'http://localhost:8081',
    },
  },
});
