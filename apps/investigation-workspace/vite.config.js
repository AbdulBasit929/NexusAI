import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { fileURLToPath, URL } from 'node:url'
import { existsSync, readFileSync } from 'node:fs'
import { join } from 'node:path'

// DEV ONLY, and a fallback -- `process.env` still wins.
//
// The dev server is launched by tooling that does not forward the repository's
// runtime environment, so without this the proxy forwards UNAUTHENTICATED and
// every request comes back 401. That reads as "the workspace is broken" when
// the API is in fact correctly failing closed, which is the single most
// misleading failure this app can show.
//
// Read from the gitignored runtime env file, never a tracked one. The value is
// attached server-side in `proxyReq` below and never reaches the bundle.
function keyFromRuntimeEnvFile() {
  try {
    const path = fileURLToPath(new URL('../../.env.forensic-runtime.local', import.meta.url))
    const line = readFileSync(path, 'utf8')
      .split(/\r?\n/)
      .find(entry => entry.trim().startsWith('FORENSIC_RECORDS_API_KEY='))
    return line ? line.slice(line.indexOf('=') + 1).trim() : ''
  } catch {
    return ''
  }
}

// SAME-ORIGIN API PROXY.
//
// The forensic API key is read from the environment of the dev server process
// and attached here, server-side. It is never written into the bundle, the
// page, localStorage or anything the browser can read -- which is the whole
// requirement: a long-lived key in browser storage is a credential an
// analyst's machine can leak, and this product's evidence is read in court.
//
// This proxy is NOT authentication. It forwards with a privileged key, so
// anyone who can reach it has that access. That is why the dev server binds to
// 127.0.0.1 and why this is a development and review convenience, not a
// deployment posture. Real multi-user auth needs the identity system that does
// not exist yet (no users/roles/membership tables) and attaches HERE when it
// does.
function apiProxy() {
  const target = process.env.NEXUSAI_API_URL || 'http://localhost:8091'
  const key = process.env.FORENSIC_RECORDS_API_KEY || keyFromRuntimeEnvFile()
  return {
    target,
    changeOrigin: true,
    rewrite: path => path.replace(/^\/api/, ''),
    configure(proxy) {
      proxy.on('proxyReq', request => {
        if (key) request.setHeader('Authorization', `Bearer ${key}`)
      })
    },
  }
}


// The data inventory of a case holds real values, so it lives in the vault, outside the repository. This dev server (127.0.0.1 only) serves it, and the CSV extracts next to it,
// at /real-data/ and nowhere else; with no vault folder there is nothing to serve (docs/data/real-data-handling-policy.md, extension of 2026-10-14).
const REAL_DATA_FILES = new Set(['inventory.json', 'files.csv', 'subscribers.csv', 'towers.csv', 'devices.csv', 'call_records.csv', 'tower_dump_rows.csv', 'internet_sessions.csv', 'app_usage.csv', 'numbers_seen.csv'])
function realDataExports() {
  const serve = server => server.middlewares.use('/real-data', (request, response) => {
    const name = decodeURIComponent((request.url || '').split('?')[0].replace(/^\//, ''))
    const dir = join(process.env.NEXUSAI_REAL_DATA_VAULT || 'C:\\NexusAI-RealDataVault', 'exports')
    const file = join(dir, name)
    if (!REAL_DATA_FILES.has(name) || !existsSync(file)) { response.statusCode = 404; response.end(); return }
    response.setHeader('content-type', name.endsWith('.json') ? 'application/json' : 'text/csv; charset=utf-8')
    response.setHeader('cache-control', 'no-store')
    if (name.endsWith('.csv')) response.setHeader('content-disposition', `attachment; filename="${name}"`)
    response.end(readFileSync(file))
  })
  return { name: 'real-data-exports', configureServer(server) { serve(server) }, configurePreviewServer(server) { serve(server) } }
}

// MapLibre GL 6 starts its worker from a file that has to be served by us (no CDN): the worker and the module it imports are served at /maplibre/ in development and
// written next to the built page. (The basemap tiles come through /tiles, which tools/tilecache answers; see `tileCache` below.)
const MAPLIBRE_FILES = ['maplibre-gl-worker.mjs', 'maplibre-gl-shared.mjs']
function maplibreWorker() {
  const dir = fileURLToPath(new URL('./node_modules/maplibre-gl/dist/', import.meta.url))
  const serve = server => server.middlewares.use('/maplibre', (request, response) => {
    const name = decodeURIComponent((request.url || '').split('?')[0].replace(/^\//, ''))
    if (!MAPLIBRE_FILES.includes(name) || !existsSync(join(dir, name))) { response.statusCode = 404; response.end(); return }
    response.setHeader('content-type', 'text/javascript; charset=utf-8')
    response.end(readFileSync(join(dir, name)))
  })
  return {
    name: 'maplibre-worker',
    configureServer(server) { serve(server) },
    configurePreviewServer(server) { serve(server) },
    generateBundle() {
      for (const name of MAPLIBRE_FILES) if (existsSync(join(dir, name))) this.emitFile({ type: 'asset', fileName: `maplibre/${name}`, source: readFileSync(join(dir, name)) })
    },
  }
}

// The offline basemap: the browser asks this origin for /tiles/..., and the tile cache (tools/tilecache, npm run dev:tiles in tools/visual_spike) answers; only the cache talks to the shared server.
const tileCache = { '/tiles': { target: process.env.TILE_CACHE_URL || 'http://127.0.0.1:8099', changeOrigin: false } }

export default defineConfig({
  plugins: [realDataExports(), maplibreWorker(), react()],
  resolve: {
    alias: {
      '@semantic-layer': fileURLToPath(new URL('../../semantic_layer', import.meta.url)),
    },
  },
  server: {
    host: '127.0.0.1',
    port: 4181,
    fs: { allow: [fileURLToPath(new URL('../..', import.meta.url))] },
    proxy: { '/api': apiProxy(), ...tileCache },
  },
  preview: { host: '127.0.0.1', port: 4182, proxy: { '/api': apiProxy(), ...tileCache } },
  test: {
    environment: 'jsdom',
    // user-event typing is slow while the dev server and browsers share the laptop; assertions are unchanged.
    testTimeout: 15_000,
    setupFiles: './src/test/setup.js',
    include: ['src/**/*.vitest.{js,jsx}'],
    exclude: ['src/ported/**'],
  },
})
