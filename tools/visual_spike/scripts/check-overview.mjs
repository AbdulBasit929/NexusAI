// Drives headless Chrome: a big synthetic graph is drawn in overview mode, a group is opened, a number inside a group is found, everything is closed again.
// Usage: node scripts/check-overview.mjs BASE_URL [OUT_DIR]
import { spawn } from 'node:child_process';
import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
const [base, out] = process.argv.slice(2); if (out) mkdirSync(out, { recursive: true });
const port = 9336; const profile = mkdtempSync(join(tmpdir(), 'spike-chrome-'));
const proc = spawn('C:/Program Files/Google/Chrome/Application/chrome.exe', ['--headless=new', `--remote-debugging-port=${port}`, `--user-data-dir=${profile}`, '--window-size=1400,900', '--no-first-run', 'about:blank'], { stdio: 'ignore' });
const sleep = ms => new Promise(r => setTimeout(r, ms));
let failed = 0; const check = (name, ok, detail = '') => { console.log((ok ? 'ok   ' : 'FAIL ') + name + (detail ? '  ' + detail : '')); if (!ok) failed++; };
try {
  let target; for (let i = 0; i < 50 && !target; i++) { try { target = await (await fetch(`http://127.0.0.1:${port}/json/new?about:blank`, { method: 'PUT' })).json(); } catch { await sleep(200); } }
  const ws = new WebSocket(target.webSocketDebuggerUrl); await new Promise(r => { ws.onopen = r; });
  let id = 0; const pending = new Map(); const errors = [];
  ws.onmessage = m => { const d = JSON.parse(m.data); if (d.id && pending.has(d.id)) { pending.get(d.id)(d); pending.delete(d.id); } else if (d.method === 'Runtime.exceptionThrown') errors.push(d.params.exceptionDetails.exception?.description?.slice(0, 400)); };
  const send = (method, params = {}) => new Promise(res => { const i = ++id; pending.set(i, res); ws.send(JSON.stringify({ id: i, method, params })); });
  const ev = async e => { const r = await send('Runtime.evaluate', { expression: e, awaitPromise: true, returnByValue: true }); if (!r.result) throw new Error('no result: ' + JSON.stringify(r).slice(0, 300) + ' page errors: ' + errors.join(' | ')); if (r.result.exceptionDetails) throw new Error(JSON.stringify(r.result.exceptionDetails.exception?.description)); return r.result.result.value; };
  const shot = async name => { if (!out) return; const r = await send('Page.captureScreenshot', { format: 'png' }); writeFileSync(join(out, name + '.png'), Buffer.from(r.result.data, 'base64')); };
  await send('Runtime.enable'); await send('Emulation.setDeviceMetricsOverride', { width: 1400, height: 900, deviceScaleFactor: 1, mobile: false });
  await send('Page.navigate', { url: base + '?stress=3000' });
  for (let i = 0; i < 90 && !(await ev('!!window.__cy')); i++) await sleep(1000);
  if (!(await ev('!!window.__cy'))) { console.log('the link chart did not appear; page errors:', errors.join(' | ')); process.exit(1); }
  const nodes = await ev(`window.__cy.nodes().length`); const groups = await ev(`window.__cy.nodes('[kind="group"]').length`);
  check('a graph of 3,000 numbers is drawn as an overview', groups > 0 && nodes < 1500, `${nodes} nodes, ${groups} groups`);
  await shot('overview-closed');
  const sizes = await ev(`window.__app.link.model.groups.map(g => g.members.length)`); const total = sizes.reduce((a, b) => a + b, 0);
  const contacts = await ev(`window.__app.views ? 0 : 0`);
  check('every grouped number is in exactly one group', new Set(await ev(`window.__app.link.model.groups.flatMap(g => g.members)`)).size === total, `${total} numbers in ${sizes.length} groups`);
  await ev(`void window.__cy.nodes('[kind="group"]')[0].emit('tap')`); await sleep(400);
  check('a group opens a panel that says what it is', (await ev(`document.getElementById('link-panel').innerText`)).includes('Group of'));
  const before = await ev(`window.__cy.nodes().length`);
  const first = await ev(`window.__cy.nodes('[kind="group"]')[0].id()`); const members = await ev(`window.__app.link.model.groups.find(g => g.id === ${JSON.stringify(first)}).members.length`);
  await ev(`void window.__cy.nodes('[kind="group"]')[0].emit('dbltap')`); await sleep(900);
  const after = await ev(`window.__cy.nodes().length`);
  check('opening a group replaces it by its numbers', after === before - 1 + members, `${before} -> ${after} (group of ${members})`);
  await shot('overview-opened');
  const hidden = await ev(`window.__app.link.model.groups[0].members[0]`);
  await ev(`void window.__app.link.selectNode(${JSON.stringify(hidden)})`); await sleep(600);
  check('selecting a number inside a group opens its group', await ev(`!window.__cy.getElementById(${JSON.stringify(hidden)}).empty()`));
  await ev(`void window.__app.link.expandAll()`); await sleep(1200);
  check('opening all groups shows every number', await ev(`window.__cy.nodes().length`) === 3000 && (await ev(`window.__cy.nodes('[kind="group"]').length`)) === 0, `${await ev('window.__cy.nodes().length')} nodes`);
  await ev(`void window.__app.link.collapseAll()`); await sleep(1200);
  check('grouping again returns to the overview', await ev(`window.__cy.nodes('[kind="group"]').length`) > 0);
  check('no page errors', errors.length === 0, errors.join(' | '));
  ws.close();
} finally { proc.kill(); }
process.exit(failed ? 1 : 0);
