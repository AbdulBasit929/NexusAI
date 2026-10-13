// One-off: zoomed screenshot of the first inferred (dashed) edge in a chosen Cytoscape mode. Usage: node scripts/zoomshot.mjs URL LIBATTR OUT.png
import { spawn } from 'node:child_process';
import { mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
const [url, lib, outFile] = process.argv.slice(2);
const port = 9334; const profile = mkdtempSync(join(tmpdir(), 'spike-chrome-'));
const proc = spawn('C:/Program Files/Google/Chrome/Application/chrome.exe', ['--headless=new', `--remote-debugging-port=${port}`, `--user-data-dir=${profile}`, '--window-size=1280,820', '--no-first-run', 'about:blank'], { stdio: 'ignore' });
const sleep = ms => new Promise(r => setTimeout(r, ms));
try {
  let target; for (let i = 0; i < 50 && !target; i++) { try { target = await (await fetch(`http://127.0.0.1:${port}/json/new?about:blank`, { method: 'PUT' })).json(); } catch { await sleep(200); } }
  const ws = new WebSocket(target.webSocketDebuggerUrl); await new Promise(r => { ws.onopen = r; });
  let id = 0; const pending = new Map(); ws.onmessage = m => { const d = JSON.parse(m.data); if (d.id && pending.has(d.id)) { pending.get(d.id)(d); pending.delete(d.id); } };
  const send = (method, params = {}) => new Promise(res => { const i = ++id; pending.set(i, res); ws.send(JSON.stringify({ id: i, method, params })); });
  const ev = async e => (await send('Runtime.evaluate', { expression: e, awaitPromise: true, returnByValue: true })).result.result.value;
  await send('Emulation.setDeviceMetricsOverride', { width: 1280, height: 820, deviceScaleFactor: 1, mobile: false });
  await send('Page.navigate', { url }); await sleep(2500);
  await ev(`document.querySelector('[data-lib=${lib}]').click(); new Promise(r => setTimeout(r, 1500))`);
  const box = await ev(`(() => { const cy = window.__cy; cy.zoom({ level: 3, renderedPosition: cy.edges('[kind="inferred"]')[0].renderedMidpoint() }); const m = cy.edges('[kind="inferred"]')[0].renderedMidpoint(); const r = document.getElementById('graph').getBoundingClientRect(); return new Promise(res => setTimeout(() => res({ x: r.left + m.x, y: r.top + m.y }), 800)); })()`);
  const shot = await send('Page.captureScreenshot', { format: 'png', clip: { x: Math.max(0, box.x - 120), y: Math.max(0, box.y - 70), width: 240, height: 140, scale: 3 } });
  writeFileSync(outFile, Buffer.from(shot.result.data, 'base64')); console.log('wrote', outFile, JSON.stringify(box));
  ws.close();
} finally { proc.kill(); }
process.exit(0);
