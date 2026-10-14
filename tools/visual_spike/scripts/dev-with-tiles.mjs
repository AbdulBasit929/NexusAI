// Starts the tile cache (tools/tilecache) and the development server together, so the offline basemap is there when the map tab opens.
// The address of the organisation's shared tile server is read from the environment (TILE_UPSTREAM) or from tiles.local.json (git-ignored, never served
// to a browser); it is passed to the cache on its command line and is never written to a tracked file. Without an address only the development server
// starts and the map says so. Usage: npm run dev:tiles [-- --port 5199]
import { spawn } from 'node:child_process';
import { existsSync, mkdirSync, readFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const spike = resolve(here, '..');
const repo = resolve(spike, '..', '..');

let local = {};
try { local = JSON.parse(readFileSync(join(spike, 'tiles.local.json'), 'utf8')); } catch { /* optional file */ }
const upstream = process.env.TILE_UPSTREAM || local.upstream || '';
const sets = process.env.TILE_SETS || local.sets || 'pakistan';
const listen = process.env.TILE_CACHE_LISTEN || local.listen || '127.0.0.1:8099';
const cacheDir = process.env.TILE_CACHE_DIR || local.cacheDir || join(process.env.LOCALAPPDATA || join(homedir(), '.cache'), 'NexusAI', 'tilecache-data');
const children = [];
const stop = () => { for (const c of children) { try { c.kill(); } catch { /* already gone */ } } };
process.on('SIGINT', () => { stop(); process.exit(0); });
process.on('SIGTERM', () => { stop(); process.exit(0); });
process.on('exit', stop);

async function healthy(url, ms) {
  const end = Date.now() + ms;
  while (Date.now() < end) {
    try { const r = await fetch(url, { signal: AbortSignal.timeout(1500) }); if (r.ok) return true; } catch { /* not up yet */ }
    await new Promise(r => setTimeout(r, 500));
  }
  return false;
}

if (!upstream) {
  console.warn('[dev:tiles] No tile server address (TILE_UPSTREAM, or "upstream" in tools/visual_spike/tiles.local.json): starting the development server only; the map will say it has no basemap.');
} else if (await healthy(`http://${listen}/healthz`, 800)) {
  console.log(`[dev:tiles] A tile cache already answers on ${listen}; using it.`);
} else {
  mkdirSync(cacheDir, { recursive: true });
  console.log(`[dev:tiles] Starting the tile cache on ${listen} (sets: ${sets}; cache: ${cacheDir}); the upstream address is not printed.`);
  const cache = spawn('go', ['run', './tools/tilecache', '-upstream', upstream, '-sets', sets, '-cache-dir', cacheDir, '-listen', listen], { cwd: repo, stdio: ['ignore', 'inherit', 'inherit'] });
  children.push(cache);
  cache.on('exit', code => { console.error(`[dev:tiles] The tile cache stopped (exit ${code}). The map will report the basemap as not reachable.`); });
  if (!(await healthy(`http://${listen}/healthz`, 90000))) console.error('[dev:tiles] The tile cache did not become ready in 90 s; continuing without it.');
  else console.log('[dev:tiles] Tile cache ready.');
}

const copy = spawn(process.execPath, [join(here, 'copy-worker.mjs')], { cwd: spike, stdio: 'inherit' });
await new Promise(r => copy.on('exit', r));
const vite = spawn(process.execPath, [join(spike, 'node_modules', 'vite', 'bin', 'vite.js'), ...process.argv.slice(2)], { cwd: spike, stdio: 'inherit', env: { ...process.env, TILE_CACHE_URL: `http://${listen}` } });
children.push(vite);
vite.on('exit', code => { stop(); process.exit(code ?? 0); });
