import { existsSync, readFileSync } from 'node:fs';
import { defineConfig } from 'vite';

// The browser asks this origin for tiles (/tiles/...); the tile cache (tools/tilecache) answers, and only it talks to the shared tile server.
const tileCache = { '/tiles': { target: process.env.TILE_CACHE_URL || 'http://127.0.0.1:8099', changeOrigin: false } };
// Real view data never lives inside the repository: when SPIKE_REAL_VIEWS names a file (the vault's working folder), it is served at /views.real.json and nowhere else.
const realViews = { name: 'real-views', configureServer: serve, configurePreviewServer: serve };
function serve(server) {
  server.middlewares.use('/views.real.json', (req, res) => {
    const file = process.env.SPIKE_REAL_VIEWS;
    if (!file || !existsSync(file)) { res.statusCode = 404; res.end(); return; }
    res.setHeader('content-type', 'application/json'); res.setHeader('cache-control', 'no-store'); res.end(readFileSync(file));
  });
}
export default defineConfig({ plugins: [realViews], server: { host: '127.0.0.1', proxy: tileCache }, preview: { proxy: tileCache }, build: { chunkSizeWarningLimit: 2000 } });
