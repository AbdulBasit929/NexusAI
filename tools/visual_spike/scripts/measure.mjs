// Drives an installed Chrome over the DevTools protocol (no extra packages): opens the built spike, exercises each view, writes screenshots and
// runs the in-page measurements. Usage: node scripts/measure.mjs http://127.0.0.1:5198/ OUT_DIR [chrome.exe] [--skip-bench]
import { spawn } from 'node:child_process';
import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const args = process.argv.slice(2).filter(a => !a.startsWith('--')); const skipBench = process.argv.includes('--skip-bench');
const [url = 'http://127.0.0.1:5198/', out = '.', chrome = 'C:/Program Files/Google/Chrome/Application/chrome.exe'] = args;
mkdirSync(out, { recursive: true });
const port = 9333; const profile = mkdtempSync(join(tmpdir(), 'spike-chrome-'));
const proc = spawn(chrome, ['--headless=new', `--remote-debugging-port=${port}`, `--user-data-dir=${profile}`, '--window-size=1400,900', '--no-first-run', '--no-default-browser-check', '--disable-extensions', 'about:blank'], { stdio: 'ignore' });
const sleep = ms => new Promise(r => setTimeout(r, ms));
async function json(path, method = 'GET') { for (let i = 0; i < 50; i++) { try { return await (await fetch(`http://127.0.0.1:${port}${path}`, { method })).json(); } catch { await sleep(200); } } throw new Error('no chrome'); }
try {
  const target = await json('/json/new?about:blank', 'PUT');
  const ws = new WebSocket(target.webSocketDebuggerUrl); await new Promise(r => { ws.onopen = r; });
  let id = 0; const pending = new Map(); const logs = [];
  ws.onmessage = m => { const d = JSON.parse(m.data); if (d.id && pending.has(d.id)) { pending.get(d.id)(d); pending.delete(d.id); } else if (d.method === 'Runtime.exceptionThrown') logs.push('EXCEPTION ' + JSON.stringify(d.params.exceptionDetails.exception?.description || d.params.exceptionDetails.text).slice(0, 300)); else if (d.method === 'Runtime.consoleAPICalled' && d.params.type === 'error') logs.push('console.error ' + d.params.args.map(a => a.value ?? a.description).join(' ').slice(0, 300)); };
  const send = (method, params = {}) => new Promise(res => { const i = ++id; pending.set(i, res); ws.send(JSON.stringify({ id: i, method, params })); });
  const evalJs = async expr => { const r = await send('Runtime.evaluate', { expression: expr, awaitPromise: true, returnByValue: true }); if (r.result.exceptionDetails) throw new Error(JSON.stringify(r.result.exceptionDetails.exception?.description || r.result.exceptionDetails)); return r.result.result.value; };
  const view = (w, h) => send('Emulation.setDeviceMetricsOverride', { width: w, height: h, deviceScaleFactor: 1, mobile: false });
  const shot = async name => { const r = await send('Page.captureScreenshot', { format: 'png' }); writeFileSync(join(out, name + '.png'), Buffer.from(r.result.data, 'base64')); };
  const tab = async t => { await evalJs(`window.__app.showTab('${t}')`); await sleep(400); };
  const setSelect = (label, index) => evalJs(`(() => { const s = [...document.querySelectorAll('select')].find(x => x.getAttribute('aria-label') === ${JSON.stringify(label)}); if (typeof ${JSON.stringify(index)} === 'number') s.selectedIndex = ${JSON.stringify(index)}; else s.value = ${JSON.stringify(index)}; s.dispatchEvent(new Event('change')); return s.value; })()`);
  await send('Page.enable'); await send('Runtime.enable'); await view(1400, 900);
  await send('Page.navigate', { url }); await sleep(3000);
  console.log('webgl:', await evalJs(`(() => { const g = document.createElement('canvas').getContext('webgl2'); const e = g && g.getExtension('WEBGL_debug_renderer_info'); return g ? (e ? g.getParameter(e.UNMASKED_RENDERER_WEBGL) : g.getParameter(g.RENDERER)) : 'none'; })()`));
  console.log('config tiles:', JSON.stringify(await evalJs(`window.__app.config.tiles && (window.__app.config.tiles.tilejson || 'template')`)));

  // ---- link chart ----
  await shot('01-link-overview');
  console.log('compare:', JSON.stringify(await evalJs(`(async () => { const l = window.__app.link; const cy = l.cy; const e = cy.edges('[kind="inferred"]').sort((a, b) => b.data('count') - a.data('count'))[0]; l.setSlot('A', e.data('source')); l.setSlot('B', e.data('target')); l.showCompare(); await new Promise(r => setTimeout(r, 700)); const p = document.getElementById('link-panel'); return { text: p.innerText.slice(0, 260), verdict: (p.querySelector('.note.good, .note.bad') || {}).className }; })()`)));
  await shot('02-link-compare');
  console.log('path:', JSON.stringify(await evalJs(`(async () => { const l = window.__app.link; const cy = l.cy; const subj = cy.nodes('[kind="subject"]'); l.setSlot('A', subj[0].id()); const far = cy.nodes('[kind="contact"]').sort((a, b) => a.id().localeCompare(b.id()))[7]; l.setSlot('B', far.id()); l.showPath(); await new Promise(r => setTimeout(r, 700)); return document.getElementById('link-panel').innerText.slice(0, 240); })()`)));
  await shot('03-link-path');
  console.log('candidate edge:', JSON.stringify(await evalJs(`(async () => { const cy = window.__cy; const e = cy.edges('[kind="candidate"]')[0]; e.emit('tap'); await new Promise(r => setTimeout(r, 400)); return document.getElementById('link-panel').innerText.slice(0, 200); })()`)));
  await shot('04-link-candidate');
  console.log('keyboard:', JSON.stringify(await evalJs(`(async () => { const g = document.getElementById('graph'); g.focus(); const key = k => g.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true })); key('ArrowRight'); key('ArrowRight'); const label = g.getAttribute('aria-label'); key('Enter'); await new Promise(r => setTimeout(r, 300)); return { label, panel: document.getElementById('link-panel').innerText.slice(0, 60) }; })()`)));
  console.log('table rows:', await evalJs(`(() => { document.getElementById('table-toggle').click(); return document.querySelectorAll('#graph-table tbody tr').length; })()`)); await shot('05-link-table'); await evalJs(`document.getElementById('table-toggle').click(); 0`);
  console.log('filters:', JSON.stringify(await evalJs(`(async () => { const cy = window.__cy; const before = cy.elements(':visible').length; const t = document.querySelectorAll('.toolbar .tog input'); t[3].click(); await new Promise(r => setTimeout(r, 300)); const after = cy.elements(':visible').length; t[3].click(); return { before, afterHidingContacts: after }; })()`)));
  await evalJs(`document.getElementById('rtl').click(); new Promise(r => setTimeout(r, 1500))`); await shot('06-link-urdu-rtl'); await evalJs(`document.getElementById('rtl').click(); new Promise(r => setTimeout(r, 1500))`);

  // ---- map ----
  await tab('map'); await sleep(3500); await shot('10-map-ocean');
  await setSelect('Where to draw this case', 'islamabad'); await sleep(6000); await shot('11-map-islamabad');
  console.log('map state:', JSON.stringify(await evalJs(`(() => { const m = window.__app.mapView.map; return { styleLoaded: m.isStyleLoaded(), layers: m.getStyle().layers.map(l => l.id), zoom: +m.getZoom().toFixed(2), center: m.getCenter(), badge: document.querySelector('#view-map .status').innerText }; })()`)));
  await evalJs(`(() => { const s = document.querySelector('#view-map input[aria-label="Time along the movement"]'); s.value = 550; s.dispatchEvent(new Event('input')); return 1; })()`); await sleep(1200); await shot('12-map-trail-halfway');
  await setSelect('Subject to follow', 1); await sleep(3000); await shot('13-map-second-mover');
  await setSelect('Subject to follow', 2); await sleep(3000); await shot('14-map-trip');
  await evalJs(`(() => { const b = document.querySelector('#map-panel .colo button.linkish'); b && b.click(); return 1; })()`); await sleep(3000); await shot('15-map-colocation-window');
  await setSelect('Where to draw this case', 'karachi'); await sleep(6000); await shot('16-map-karachi');

  // ---- timelines (tall viewport, so that the whole page is in the picture) ----
  await tab('lanes'); await view(1400, 3000); await sleep(2500); await shot('20-timelines');
  console.log('cards:', JSON.stringify(await evalJs(`[...document.querySelectorAll('.card h3')].map(x => x.innerText)`)));
  await view(1400, 900);

  if (!skipBench) {
    await tab('metrics'); await evalJs(`document.getElementById('run-all').click(); 0`);
    for (let i = 0; i < 600; i++) { if (await evalJs('window.__spike.done === true')) break; await sleep(1000); }
    const results = await evalJs('window.__spike.results'); writeFileSync(join(out, 'results.json'), JSON.stringify(results, null, 1));
    for (const [k, v] of results) console.log(k.padEnd(58), v);
    await shot('30-measurements');
  }
  console.log('page errors:', logs.length ? '\n' + logs.join('\n') : 'none');
  ws.close();
} finally { proc.kill(); }
process.exit(0);
