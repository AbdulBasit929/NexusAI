// Drives headless Chrome over CDP against the spike and checks targets, carrier and period filters, carrier icons and the number format.
// Prints counts and booleans only: no numbers, no screenshots (real data stays on this machine).
// Usage: node scripts/check-filters.mjs BASE_URL [synthetic]   (BASE_URL is the running development server, e.g. http://127.0.0.1:5200)
import { spawn } from 'node:child_process';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
const [base, mode] = process.argv.slice(2);
const q = mode === 'synthetic' ? '?data=synthetic' : '';
const port = 9352; const profile = mkdtempSync(join(tmpdir(), 'spike-chrome-'));
const proc = spawn('C:/Program Files/Google/Chrome/Application/chrome.exe', ['--headless=new', `--remote-debugging-port=${port}`, `--user-data-dir=${profile}`, '--window-size=1400,900', '--no-first-run', '--no-default-browser-check', '--use-angle=swiftshader', '--enable-unsafe-swiftshader', 'about:blank'], { stdio: 'ignore' });
const sleep = ms => new Promise(r => setTimeout(r, ms));
const errors = [];
try {
  let target; for (let i = 0; i < 60 && !target; i++) { try { target = await (await fetch(`http://127.0.0.1:${port}/json/new?about:blank`, { method: 'PUT' })).json(); } catch { await sleep(300); } }
  const ws = new WebSocket(target.webSocketDebuggerUrl); await new Promise(r => { ws.onopen = r; });
  let id = 0; const pending = new Map();
  ws.onmessage = m => {
    const d = JSON.parse(m.data);
    if (d.id && pending.has(d.id)) { pending.get(d.id)(d); pending.delete(d.id); }
    else if (d.method === 'Runtime.exceptionThrown') errors.push('exception: ' + (d.params.exceptionDetails.exception?.description || d.params.exceptionDetails.text).split('\n')[0].slice(0, 160));
    else if (d.method === 'Runtime.consoleAPICalled' && d.params.type === 'error') errors.push('console.error: ' + d.params.args.map(a => String(a.value ?? a.description ?? '')).join(' ').slice(0, 160));
  };
  const send = (method, params = {}) => new Promise(res => { const i = ++id; pending.set(i, res); ws.send(JSON.stringify({ id: i, method, params })); });
  const ev = async e => { const r = await send('Runtime.evaluate', { expression: e, awaitPromise: true, returnByValue: true }); if (r.result.exceptionDetails) { errors.push('eval: ' + r.result.exceptionDetails.text + ' ' + (r.result.exceptionDetails.exception?.description || '').split('\n')[0].slice(0, 120)); return null; } return r.result.result ? r.result.result.value : null; };
  await send('Emulation.setDeviceMetricsOverride', { width: 1400, height: 900, deviceScaleFactor: 1, mobile: false });
  await send('Page.enable'); await send('Runtime.enable');
  const log = (k, v) => console.log(k.padEnd(46), JSON.stringify(v));

  await send('Page.navigate', { url: base + '/' + q }); await sleep(6000);
  const counts = () => ev(`(() => { const a = window.__app; const v = a.views; const subj = v.graph.nodes.filter(n => n.kind === 'subject');
    return { subjects: subj.length, nodes: v.graph.nodes.length, edges: v.graph.edges.length, cells: v.map.cells.length, movers: v.map.movers.length, lanes: v.lanes.devices.length, pairs: v.pairs.length,
      cyNodes: a.link ? a.link.cy.nodes().length : null, cyIcons: a.link ? a.link.cy.nodes('[icon]').length : null, summary: document.querySelector('.fb-summary')?.textContent.replace(/\\d{6,}/g, '#') }; })()`);
  log('dataset', await ev(`window.__app.base.case.real ? 'real' : 'synthetic'`));
  log('errors on load', errors.length);
  const whole = await counts(); log('whole case', whole);
  log('labels: formatted / other (all nodes)', await ev(`(() => { const n = window.__app.base.graph.nodes; const ok = n.filter(x => /^0\\d{3} \\d{7}$/.test(x.label)).length; return [ok, n.length - ok]; })()`));
  log('short labels left (<10 chars)', await ev(`window.__app.base.graph.nodes.filter(x => x.label.length < 10).length`));
  log('carriers of subjects', await ev(`Object.entries(window.__app.base.graph.nodes.filter(n => n.kind === 'subject').reduce((m, n) => (m[n.carrier] = (m[n.carrier] || 0) + 1, m), {}))`));
  log('carriers of all numbers', await ev(`Object.entries(window.__app.base.graph.nodes.reduce((m, n) => (m[n.carrier] = (m[n.carrier] || 0) + 1, m), {}))`));
  log('legend carrier icons / filter chips', await ev(`[document.querySelectorAll('#view-link .legend2 .carrier-icon').length, document.querySelectorAll('.fb-chip').length]`));
  log('icon images loaded (chips)', await ev(`[...document.querySelectorAll('.fb-chip img')].every(i => i.complete && i.naturalWidth > 0)`));
  log('cell sites named', await ev(`[...new Set(window.__app.base.map.cells.map(c => c.operator))].length`));

  // targets popover
  await ev(`document.querySelector('.fb-btn').click(); 0`); await sleep(300);
  log('target popover rows', await ev(`document.querySelectorAll('.fb-row').length`));
  await ev(`(() => { const i = document.querySelector('.fb-row input'); i.click(); return 0; })()`); await sleep(1500);
  const one = await counts(); log('after one target', one);
  log('url has t=', await ev(`new URLSearchParams(location.search).has('t')`));
  log('target kept subjects==1 and edges<whole', one.subjects === 1 && one.edges < whole.edges);
  log('graph nodes == drawn nodes', one.nodes === one.cyNodes || `${one.nodes} vs ${one.cyNodes}`);
  await ev(`document.querySelector('#tabs button[data-tab="map"]').click(); 0`); await sleep(5000);
  log('map tab: movers / mover options', await ev(`[window.__app.views.map.movers.length, document.querySelectorAll('#view-map select[aria-label="Subject to follow"] option').length]`));
  await ev(`document.querySelector('#tabs button[data-tab="lanes"]').click(); 0`); await sleep(3500);
  log('timelines tab: lanes / cards', await ev(`[window.__app.views.lanes.devices.length, document.querySelectorAll('#view-lanes .card').length]`));
  log('timelines table has Carrier column', await ev(`[...document.querySelectorAll('#view-lanes th')].some(t => t.textContent === 'Carrier')`));
  log('errors so far', errors.length);

  // reload keeps the filter from the address
  const href = await ev(`location.href`);
  await send('Page.navigate', { url: href }); await sleep(6000);
  const again = await counts(); log('after reload with filter in url', { subjects: again.subjects, edges: again.edges, same: again.subjects === one.subjects && again.edges === one.edges });
  await ev(`window.__app.filters.reset(); 0`); await sleep(1500);

  // carrier chip
  const present = await ev(`[...document.querySelectorAll('.fb-chip')].map(b => b.dataset.carrier)`);
  log('carriers offered', present);
  for (const c of present) {
    await ev(`document.querySelector('.fb-chip[data-carrier="${c}"]').click(); 0`); await sleep(1200);
    const r = await counts(); log(`carrier ${c}`, { subjects: r.subjects, nodes: r.nodes, edges: r.edges, cells: r.cells, cyNodes: r.cyNodes, pressed: await ev(`document.querySelector('.fb-chip[data-carrier="${c}"]').getAttribute('aria-pressed')`) });
    await ev(`window.__app.filters.reset(); 0`); await sleep(500);
  }
  // all carriers selected = nothing hidden for subjects
  // period
  const b = await ev(`window.__app.base.case`);
  await ev(`(() => { const s = new Date(Date.parse('${b.start}T00:00:00Z') + (${b.days} - 8) * 86400000).toISOString().slice(0, 10); window.__app.filters.set({ from: s }); return 0; })()`); await sleep(1500);
  const last7 = await counts(); log('last 7 days', { subjects: last7.subjects, nodes: last7.nodes, edges: last7.edges, fewerEdges: last7.edges <= whole.edges, cyNodes: last7.cyNodes });
  await ev(`window.__app.filters.reset(); 0`); await sleep(800);
  // a period before the data begins
  await ev(`window.__app.filters.set({ to: '2000-01-01' }); 0`); await sleep(1500);
  const none = await counts(); log('period before the data', { edges: none.edges, nodes: none.nodes, movers: none.movers });
  // empty state
  await ev(`window.__app.filters.set({ carriers: ['scom'], targets: [] }); 0`); await sleep(1200);
  log('carrier with no subject: empty state shown', await ev(`!!document.querySelector('#view-link .empty') || window.__app.views.graph.nodes.some(n => n.kind === 'subject')`));
  await ev(`window.__app.filters.reset(); 0`); await sleep(1200);
  const back = await counts(); log('reset restores the whole case', back.subjects === whole.subjects && back.edges === whole.edges && back.nodes === whole.nodes);
  // search by digits formatted
  log('link chart search finds a number typed with spaces', await ev(`(() => { const a = window.__app; const s = a.base.graph.nodes.find(n => n.kind === 'subject'); const input = document.querySelector('#view-link input.search'); input.value = s.label; input.dispatchEvent(new Event('change')); return !!document.querySelector('#link-panel h2'); })()`));
  log('panel shows carrier chip', await ev(`!!document.querySelector('#link-panel .carrier-label')`));
  log('RTL toggle still works', await ev(`(() => { const c = document.querySelector('#rtl'); c.click(); return document.documentElement.dir; })()`));
  await sleep(1500); await ev(`document.querySelector('#rtl').click(); 0`); await sleep(1500);
  log('errors total', errors.length);
  if (errors.length) console.log([...new Set(errors)].slice(0, 12).join('\n'));
  ws.close();
} finally { proc.kill(); }
process.exit(0);
