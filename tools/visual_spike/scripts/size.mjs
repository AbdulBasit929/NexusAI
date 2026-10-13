// Minified and gzipped size of each candidate library on its own, as Vite would ship it.
import { build } from 'vite';
import { gzipSync } from 'node:zlib';
import { readdirSync } from 'node:fs';
import { resolve } from 'node:path';
for (const f of readdirSync('scripts/size-entries')) {
  const out = await build({ configFile: false, logLevel: 'silent', build: { write: false, minify: true, rollupOptions: { input: resolve('scripts/size-entries', f) } } });
  const chunks = (Array.isArray(out) ? out[0] : out).output.filter(o => o.type === 'chunk');
  const code = chunks.map(c => c.code).join('');
  console.log(f.replace('.js', '').padEnd(20), (Buffer.byteLength(code) / 1024).toFixed(0).padStart(6), 'kB minified', (gzipSync(code).length / 1024).toFixed(0).padStart(5), 'kB gzip');
}
