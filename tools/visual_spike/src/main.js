import { loadViews, state, timed, rng } from './common.js';
import { loadConfig } from './config.js';
import { $, h, registerSites } from './ui.js';

const results = []; window.__spike = { results };
let views, config, link = null, mapView = null, timelines = null; const mounted = new Set();

async function showTab(tab) {
  document.querySelectorAll('#tabs button').forEach(x => x.setAttribute('aria-selected', String(x.dataset.tab === tab)));
  document.querySelectorAll('.view').forEach(v => { v.hidden = v.id !== 'view-' + tab; });
  if (!mounted.has(tab)) { mounted.add(tab); await mount(tab); }
  if (tab === 'map') mapView?.map.resize();
  if (tab === 'lanes') timelines?.resize();
}
async function mount(tab) {
  if (tab === 'link') { const { mountLinkView } = await import('./linkview.js'); link?.destroy(); link = mountLinkView($('#view-link'), views); }
  if (tab === 'map') { const { mountMapView } = await import('./map.js'); mapView?.destroy(); mapView = mountMapView($('#view-map'), views, config); }
  if (tab === 'lanes') { const { mountTimelines } = await import('./timelines.js'); timelines = mountTimelines($('#view-lanes'), views); }
}
document.querySelectorAll('#tabs button').forEach(b => { b.onclick = () => showTab(b.dataset.tab); });
$('#rtl').onchange = async ev => {
  state.rtl = ev.target.checked; document.documentElement.dir = state.rtl ? 'rtl' : 'ltr';
  for (const tab of [...mounted]) if (tab !== 'metrics') await mount(tab);
};

// ---- measurements -------------------------------------------------------------------------------------------------------------------------
function note(name, value) {
  const text = typeof value === 'string' ? value : JSON.stringify(value);
  results.push([name, text]); $('#results tbody').append(h('tr', {}, h('td', {}, name), h('td', {}, text)));
}
function stressPositions(nodes) {
  const r = rng(3); const groups = Math.max(...nodes.map(n => n.group)) + 1; const pos = new Map();
  nodes.forEach(n => { const a = (n.group / groups) * 6.283; pos.set(n.id, { x: Math.cos(a) * 4000 + (r() - .5) * 700, y: Math.sin(a) * 4000 + (r() - .5) * 700 }); });
  return pos;
}
async function runAll() {
  const { mountCytoscape, mountCytoscapeWebgl, mountSigma, layout, measureGraph, stressGraph } = await import('./libs.js');
  $('#results tbody').replaceChildren(); results.length = 0; const status = $('#run-status'); const bench = $('#bench');
  bench.style.cssText = 'width:900px;height:600px;position:relative'; bench.setAttribute('aria-hidden', 'false');
  const g = views.graph; const l0 = performance.now(); const real = layout(g.nodes, g.edges); note(`Layout of the case graph (${g.nodes.length} nodes, ${g.edges.length} edges), ms`, Math.round(performance.now() - l0));
  const libs = [['Cytoscape', mountCytoscape], ['Cytoscape WebGL', mountCytoscapeWebgl], ['Sigma', mountSigma]];
  for (const [name, mount] of libs) { status.textContent = name + ' case graph'; note(`${name}: case graph (${g.nodes.length} nodes, ${g.edges.length} edges)`, await measureGraph(mount, bench, g, real)); bench.replaceChildren(); }
  for (const [n, d] of [[1000, 3], [5000, 3], [20000, 3]]) {
    const gg = stressGraph(n, d); const pos = stressPositions(gg.nodes);
    for (const [name, mount] of libs) {
      if (n === 20000 && name === 'Cytoscape') { note(`${name}: ${n} nodes`, 'not run: already 10 fps at 5,000 nodes'); continue; }
      status.textContent = `${name} ${n} nodes`; note(`${name}: ${n} nodes, ${gg.edges.length} edges`, await measureGraph(mount, bench, gg, pos)); bench.replaceChildren();
    }
  }
  bench.style.cssText = ''; bench.setAttribute('aria-hidden', 'true');
  await showTab('map'); await new Promise(r => setTimeout(r, 2500)); status.textContent = 'map';
  for (const count of [50000, 300000]) {
    note(`MapLibre: ${count} circles`, await mapView.stressMapLibre(count)); mapView.clearStress(); await new Promise(r => setTimeout(r, 300));
    note(`deck.gl: ${count} points`, await mapView.stressDeck(count)); mapView.clearStress(); await new Promise(r => setTimeout(r, 300));
  }
  await showTab('lanes'); status.textContent = 'charts';
  const { mountTimelines, exportSvgTwice } = await import('./timelines.js');
  const c = await timed(() => mountTimelines($('#view-lanes'), views)); note('ECharts: all cards drawn, ms', c.ms);
  note('ECharts SVG export twice', exportSvgTwice(views));
  note('Text summary generated for the device card', document.querySelector('.card .aria')?.textContent.slice(0, 160) || '');
  const foreign = performance.getEntriesByType('resource').filter(r => new URL(r.name).origin !== location.origin).map(r => r.name.replace(/\/\d+\/\d+\/\d+\.png$/, '/{z}/{x}/{y}.png'));
  note('Resources fetched from another origin', foreign.length ? [...new Set(foreign)] : 'none');
  note('Heap used (MB)', performance.memory ? Math.round(performance.memory.usedJSHeapSize / 1048576) : 'not available');
  status.textContent = 'done'; window.__spike.done = true; await showTab('metrics');
}
$('#run-all').onclick = runAll;

config = await loadConfig();
views = await loadViews();
registerSites(views.map.cells);
{ // ?stress=N replaces the link chart's graph by a synthetic one of N numbers, to see overview mode (never real data)
  const n = +(new URLSearchParams(location.search).get('stress') || 0);
  if (n > 0) {
    const { stressGraph } = await import('./layout.js'); const sg = stressGraph(n, 3);
    views.graph = { nodes: sg.nodes.map(x => ({ ...x, label: 'x' + x.label, operator: ['ufone', 'zong', 'mobilink', 'telenor'][x.group % 4], events: 100, first: '2026-02-02 10:00:00', last: '2026-04-30 10:00:00' })), edges: sg.edges.map(e => ({ ...e, first: '2026-02-02 10:00:00', last: '2026-04-30 10:00:00', rows: [] })) };
  }
}
{
  const real = !!views.case.real; const other = real ? '?data=synthetic' : '?';
  $('#case-info').textContent = real ? `REAL DATA supplied for testing · ${views.case.subjects} subjects · ${views.case.days} days · shown on this machine only · ` : `synthetic case (seed ${views.case.seed}) · ${views.case.subjects} subjects · ${views.case.days} days · generated, no real data · `;
  $('#case-info').append(h('a', { href: other, style: 'margin-inline-start:6px' }, real ? 'show the generated case (every analysis)' : 'show the real data'));
}
mounted.add('link'); await mount('link');
window.__app = { showTab, mount, get link() { return link; }, get mapView() { return mapView; }, get timelines() { return timelines; }, config };
