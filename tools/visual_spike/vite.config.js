import { defineConfig } from 'vite';

// The browser asks this origin for tiles (/tiles/...); the tile cache (tools/tilecache) answers, and only it talks to the shared tile server.
const tileCache = { '/tiles': { target: process.env.TILE_CACHE_URL || 'http://127.0.0.1:8099', changeOrigin: false } };
export default defineConfig({ server: { host: '127.0.0.1', proxy: tileCache }, preview: { proxy: tileCache }, build: { chunkSizeWarningLimit: 2000 } });
