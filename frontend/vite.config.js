import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// En desarrollo (`npm run dev`), Vite sirve las pantallas en el puerto 5173 y manda /api al
// gateway, que tiene que estar levantado con `docker compose up`. Así el frontend usa las
// mismas rutas que cuando lo sirve NGINX (15.7).
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
});
