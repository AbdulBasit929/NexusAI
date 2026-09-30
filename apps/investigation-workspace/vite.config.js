import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { fileURLToPath, URL } from 'node:url'
import { readFileSync } from 'node:fs'

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

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@semantic-layer': fileURLToPath(new URL('../../semantic_layer', import.meta.url)),
    },
  },
  server: {
    host: '127.0.0.1',
    port: 4181,
    fs: { allow: [fileURLToPath(new URL('../..', import.meta.url))] },
    proxy: { '/api': apiProxy() },
  },
  preview: { host: '127.0.0.1', port: 4182, proxy: { '/api': apiProxy() } },
  test: {
    environment: 'jsdom',
    // user-event typing is slow while the dev server and browsers share the laptop; assertions are unchanged.
    testTimeout: 15_000,
    setupFiles: './src/test/setup.js',
    include: ['src/**/*.vitest.{js,jsx}'],
    exclude: ['src/ported/**'],
  },
})
