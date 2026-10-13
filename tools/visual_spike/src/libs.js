// Link chart on two libraries with the same data, the same encodings and the same evidence drawer.
import cytoscape from 'cytoscape';
import Graph from 'graphology';
import Sigma from 'sigma';
import { label, palette, evidenceHtml, rng, framesPerSecond, timed } from './common.js';

export { layout, stressGraph } from './layout.js';

function nodeKey(n) { return n.kind === 'hub' ? 2 : n.kind === 'subject' ? 1 : 0; }

export function mountCytoscape(el, data, { onEvidence, positions, webgl = false }) {
  const c = palette();
  const cy = cytoscape({
    container: el, ...(webgl ? { renderer: { name: 'canvas', webgl: true } } : {}), elements: [
      ...data.nodes.map(n => ({ data: { id: n.id, label: label(n.label, n.kind), kind: n.kind, size: 18 + Math.min(34, (n.degree || 0) * 3) + (n.kind === 'subject' ? 14 : 0) }, position: positions.get(n.id) })),
      ...data.edges.map((e, i) => ({ data: { id: 'e' + i, source: e.source, target: e.target, kind: e.kind, width: 1 + Math.min(3, Math.log2(1 + e.count) / 2), count: e.count, idx: i } }))],
    style: [
      { selector: 'node', style: { 'background-color': c.contact, width: 'data(size)', height: 'data(size)', label: data.nodes.length > 400 ? '' : 'data(label)', 'font-size': 11, color: c.ink, 'text-valign': 'bottom', 'text-margin-y': 3 } },
      { selector: 'node[kind="subject"]', style: { shape: 'diamond', 'background-color': c.subject } },
      { selector: 'node[kind="hub"]', style: { shape: 'triangle', 'background-color': c.hub } },
      { selector: 'edge', style: { width: 'data(width)', 'line-color': c.line, 'curve-style': 'haystack', opacity: .85 } },
      { selector: 'edge[kind="observed"]', style: { 'line-style': 'solid', 'line-color': '#8a94a3' } },
      { selector: 'edge[kind="inferred"]', style: { 'line-style': 'dashed', 'line-color': c.inferred, 'curve-style': 'bezier', opacity: 1 } },
      { selector: 'edge[kind="candidate"]', style: { 'line-style': 'dotted', 'line-color': c.candidate, 'curve-style': 'bezier', 'target-arrow-shape': 'triangle', 'target-arrow-color': c.candidate, opacity: 1 } },
      { selector: ':selected', style: { 'border-width': 3, 'border-color': c.alert, 'overlay-opacity': .15 } }],
    layout: { name: 'preset' }, wheelSensitivity: .3, pixelRatio: 1,
  });
  cy.fit(undefined, 30); window.__cy = cy;
  cy.on('tap', 'edge', ev => { const e = data.edges[ev.target.data('idx')]; onEvidence(`${label(e.source.slice(-5))} — ${label(e.target.slice(-5))} (${e.kind}, ${e.count})`, e.rows, e.kind === 'inferred' ? 'Inferred from common contacts, not seen as a call between them.' : e.kind === 'candidate' ? 'Candidate, not a finding: behavioural similarity, not identity.' : null); });
  cy.on('tap', 'node', ev => { const e = data.edges.filter(x => x.source === ev.target.id() || x.target === ev.target.id()).slice(0, 6); onEvidence(`Node ${ev.target.data('label')}`, e.flatMap(x => x.rows).slice(0, 8), null); });
  return {
    name: webgl ? 'cytoscape-webgl' : 'cytoscape', destroy: () => cy.destroy(), nodeIds: () => cy.nodes().map(n => n.id()),
    select: id => { cy.elements().unselect(); const n = cy.getElementById(id); n.select(); cy.animate({ center: { eles: n } }, { duration: 120 }); }, fit: () => cy.fit(undefined, 20),
    pan: i => { cy.zoom({ level: cy.zoom() * (i % 2 ? 1.012 : 0.988), renderedPosition: { x: el.clientWidth / 2, y: el.clientHeight / 2 } }); cy.panBy({ x: i % 2 ? 3 : -3, y: 1 }); },
    png: () => cy.png({ output: 'base64' }).length,
  };
}

export function mountSigma(el, data, { onEvidence, positions }) {
  const c = palette(); const g = new Graph({ multi: true });
  data.nodes.forEach(n => { const p = positions.get(n.id); g.addNode(n.id, { x: p.x, y: -p.y, size: 4 + Math.min(10, (n.degree || 0)) + (n.kind === 'subject' ? 5 : 0), label: label(n.label, n.kind), color: n.kind === 'subject' ? c.subject : n.kind === 'hub' ? c.hub : c.contact }); });
  data.edges.forEach((e, i) => { if (g.hasNode(e.source) && g.hasNode(e.target)) g.addEdgeWithKey('e' + i, e.source, e.target, { size: 1 + Math.min(3, Math.log2(1 + e.count) / 2), color: e.kind === 'inferred' ? c.inferred : e.kind === 'candidate' ? c.candidate : '#8a94a3', idx: i }); });
  const sigma = new Sigma(g, el, { renderEdgeLabels: false, labelRenderedSizeThreshold: data.nodes.length > 400 ? 1000 : 8, enableEdgeEvents: true, labelColor: { color: c.ink } });
  sigma.on('clickEdge', ({ edge }) => { const e = data.edges[g.getEdgeAttribute(edge, 'idx')]; onEvidence(`${label(e.source.slice(-5))} — ${label(e.target.slice(-5))} (${e.kind}, ${e.count})`, e.rows, 'Sigma draws this edge as a plain line: dashed and dotted edges need a custom WebGL edge program.'); });
  sigma.on('clickNode', ({ node }) => { const e = data.edges.filter(x => x.source === node || x.target === node).slice(0, 6); onEvidence(`Node ${g.getNodeAttribute(node, 'label')}`, e.flatMap(x => x.rows).slice(0, 8), null); });
  return {
    name: 'sigma', destroy: () => sigma.kill(), nodeIds: () => g.nodes(),
    select: id => { const a = sigma.getNodeDisplayData(id); if (a) sigma.getCamera().animate({ x: a.x, y: a.y, ratio: .5 }, { duration: 120 }); }, fit: () => sigma.getCamera().animatedReset({ duration: 0 }),
    pan: i => { const cam = sigma.getCamera(); const s = cam.getState(); cam.setState({ ...s, ratio: s.ratio * (i % 2 ? 0.992 : 1.008), x: s.x + (i % 2 ? .0005 : -.0005) }); },
    png: () => 0,
  };
}

// Keyboard: Tab focuses the chart, arrow keys move between nodes in a fixed order, Enter opens the node's evidence.
export function attachKeyboard(el, lib, openNode) {
  let index = -1;
  el.onkeydown = ev => {
    const ids = lib.nodeIds(); if (!ids.length) return;
    if (ev.key === 'ArrowRight' || ev.key === 'ArrowDown') index = (index + 1) % ids.length;
    else if (ev.key === 'ArrowLeft' || ev.key === 'ArrowUp') index = (index - 1 + ids.length) % ids.length;
    else if (ev.key === 'Enter' && index >= 0) { openNode(ids[index]); return; }
    else return;
    ev.preventDefault(); lib.select(ids[index]);
    el.setAttribute('aria-label', `Link chart. Node ${index + 1} of ${ids.length}: ${ids[index].slice(-5)}. Enter shows evidence rows.`);
  };
}

export function tableHtml(data) {
  const rows = data.edges.map(e => `<tr><td>${e.source.slice(-5)}</td><td>${e.target.slice(-5)}</td><td>${e.kind}</td><td>${e.count}</td></tr>`).join('');
  return `<table><caption>All ${data.edges.length} edges (observed / inferred / candidate)</caption><thead><tr><th>From</th><th>To</th><th>Kind</th><th>Rows</th></tr></thead><tbody>${rows}</tbody></table>`;
}

export async function measureGraph(mount, el, data, positions) {
  const first = await timed(() => mount(el, data, { onEvidence() {}, positions }));
  const lib = first.out;
  const motion = await framesPerSecond(i => lib.pan(i), 90);
  const png = lib.png();
  lib.destroy();
  return { first_render_ms: first.ms, fps: motion.fps, worst_frame_ms: motion.worst_ms, png_chars: png };
}

export const mountCytoscapeWebgl = (el, data, opts) => mountCytoscape(el, data, { ...opts, webgl: true });
