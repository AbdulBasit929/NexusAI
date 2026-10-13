// MapLibre 6 starts its worker from a file we must serve ourselves (no CDN): copy the worker and the module it imports next to the page.
import { copyFileSync, mkdirSync } from 'node:fs';
mkdirSync('public/maplibre', { recursive: true });
for (const f of ['maplibre-gl-worker.mjs', 'maplibre-gl-shared.mjs']) copyFileSync(`node_modules/maplibre-gl/dist/${f}`, `public/maplibre/${f}`);
console.log('MapLibre worker files copied to public/maplibre');
