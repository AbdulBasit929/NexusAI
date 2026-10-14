// Targets and filters shared by every view of the spike (link chart, map, timelines).
//
// Three pure pieces, free of the page so that they can be lifted into the workspace as they are (roadmap 7.8.5):
//   decorate(views)            adds the carrier to every number the views hold and names carriers instead of file layouts
//   filterViews(views, f)      the views for one set of filters: targets (subjects to look at), carriers, a period
//   FilterState                the state, kept in the address (so a view can be shared and reopened), with a change event
// and one piece of interface: mountFilterBar(container, base, state, onChange).
//
// What a filter means, so that nothing on screen is misread:
//   Targets   the subjects to look at. The link chart shows each target and the numbers it is in contact with (its neighbourhood); the map and the timelines show only the targets.
//   Carriers  keeps the subjects whose SIM carrier is selected, and, around them, only the other numbers whose carrier is selected.
//   Period    keeps a link when its first and last contact overlap the period, and cuts the movement trail to it. Counts on a link are for the whole record, not for the period.
//             Shared-contact and successor links are computed on the whole record, so they are hidden while a period is chosen (the bar says so).
import { CARRIERS, CARRIER_ORDER, pkMobile, carrierFor, carrierOfNumber, carrierOfOperator, carrierIcon, formatNumber } from './carriers.js';
import { h } from './ui.js';

const digits = v => String(v ?? '').replace(/\D/g, '');
// a number typed or linked in any common form (0300 1234567, +92 300 1234567) as the digits the data holds (923001234567)
const norm = v => { const p = pkMobile(v); return p ? '92' + p.prefix + p.rest : digits(v); };

// ---- carriers on the data ----------------------------------------------------------------------------------------------------------------------------------------
export function decorate(views) {
  const out = structuredClone(views);
  const imsiOf = new Map();
  for (const l of out.lanes?.devices || []) {
    const run = (l.runs || []).find(r => digits(r.imsi).length >= 15);
    if (run) imsiOf.set(l.number, run.imsi);
  }
  const subjects = out.graph.nodes.filter(n => n.kind === 'subject');
  const was = new Map();   // the operator or layout name a subject carried -> the carriers its subjects turned out to have
  for (const s of subjects) {
    const c = carrierFor({ number: s.id, imsi: imsiOf.get(s.id), operator: s.operator });
    (was.get(s.operator) || was.set(s.operator, []).get(s.operator)).push(c.id);
    s.layout = s.operator; s.carrier = c.id; s.carrierHow = c.how; s.operator = c.id;
  }
  const majority = list => Object.entries(list.reduce((m, id) => (m[id] = (m[id] || 0) + 1, m), {})).sort((a, b) => b[1] - a[1])[0][0];
  const carrierOfLayout = new Map([...was].map(([k, v]) => [k, majority(v)]));
  const layoutCarrier = name => carrierOfLayout.get(name) || carrierOfOperator(name)?.id || 'other';
  for (const n of out.graph.nodes) {
    if (n.kind === 'subject') continue;
    const c = out.case.real ? carrierOfNumber(n.id) : { id: 'other', how: 'a generated number has no carrier' };   // generated numbers are not issued by any carrier
    n.carrier = c.id; n.carrierHow = c.how;
  }
  const bySubject = new Map(subjects.map(s => [s.id, s]));
  for (const m of out.map.movers || []) { m.layout = m.operator; m.operator = bySubject.get(m.number)?.carrier || layoutCarrier(m.operator); }
  for (const l of out.lanes?.devices || []) { l.layout = l.operator; l.operator = bySubject.get(l.number)?.carrier || layoutCarrier(l.operator); }
  for (const c of out.map.cells || []) { c.layout = c.operator; c.operator = layoutCarrier(c.operator); }
  // Every number is shown the way a person says it (0300 1234567), the same in every view, so that a number can be found by looking across views.
  const said = new Map(out.graph.nodes.map(n => [n.label, formatNumber(n.id)]));
  const say = v => (typeof v === 'string' && said.has(v) ? said.get(v) : v);
  const relabel = o => {
    if (Array.isArray(o)) return o.forEach(relabel);
    if (!o || typeof o !== 'object') return;
    for (const [k, v] of Object.entries(o)) {
      if (k === 'label') o[k] = say(v);
      else if (k === 'labels' || k === 'holders') o[k] = Array.isArray(v) ? v.map(say) : v;
      else relabel(v);
    }
  };
  relabel(out);
  for (const d of out.map.dumps || []) for (const a of d.at_two_or_more || []) a.number = formatNumber(a.number);
  return out;
}

// ---- state ---------------------------------------------------------------------------------------------------------------------------------------------------------
export class FilterState {
  constructor(search = location.search) {
    const p = new URLSearchParams(search);
    this.targets = new Set((p.get('t') || '').split(',').map(norm).filter(Boolean));
    this.carriers = new Set((p.get('c') || '').split(',').filter(x => CARRIERS[x]));
    this.from = /^\d{4}-\d{2}-\d{2}$/.test(p.get('from') || '') ? p.get('from') : '';
    this.to = /^\d{4}-\d{2}-\d{2}$/.test(p.get('to') || '') ? p.get('to') : '';
    this.listeners = new Set();
  }
  get active() { return this.targets.size > 0 || this.carriers.size > 0 || !!this.from || !!this.to; }
  on(fn) { this.listeners.add(fn); return () => this.listeners.delete(fn); }
  emit() { this.writeUrl(); for (const fn of this.listeners) fn(this); }
  set(patch) {
    if (patch.targets) this.targets = new Set(patch.targets);
    if (patch.carriers) this.carriers = new Set(patch.carriers);
    if ('from' in patch) this.from = patch.from || '';
    if ('to' in patch) this.to = patch.to || '';
    this.emit();
  }
  reset() { this.targets.clear(); this.carriers.clear(); this.from = this.to = ''; this.emit(); }
  toParams() {
    const p = new URLSearchParams(location.search);
    for (const k of ['t', 'c', 'from', 'to']) p.delete(k);
    if (this.targets.size) p.set('t', [...this.targets].join(','));
    if (this.carriers.size) p.set('c', [...this.carriers].join(','));
    if (this.from) p.set('from', this.from);
    if (this.to) p.set('to', this.to);
    return p;
  }
  writeUrl() { const s = this.toParams().toString(); history.replaceState(null, '', location.pathname + (s ? '?' + s : '') + location.hash); }
  link() { return location.origin + location.pathname + (this.toParams().toString() ? '?' + this.toParams().toString() : ''); }
}

// ---- the filtered views ------------------------------------------------------------------------------------------------------------------------------------------
export function filterViews(views, f) {
  if (!f.active) return views;
  const g = views.graph;
  const subjects = g.nodes.filter(n => n.kind === 'subject');
  const core = new Set(subjects.filter(s => (!f.targets.size || f.targets.has(s.id)) && (!f.carriers.size || f.carriers.has(s.carrier))).map(s => s.id));
  const nodeById = new Map(g.nodes.map(n => [n.id, n]));
  const carrierOk = id => !f.carriers.size || f.carriers.has(nodeById.get(id)?.carrier);
  const overlaps = e => (!f.from || !e.last || e.last.slice(0, 10) >= f.from) && (!f.to || !e.first || e.first.slice(0, 10) <= f.to);
  const edges = g.edges.filter(e => {
    if (e.kind === 'observed') {
      const aIn = core.has(e.source), bIn = core.has(e.target);
      if (!aIn && !bIn) return false;
      const other = aIn ? e.target : e.source;
      return (core.has(other) || (!nodeById.get(other) ? false : nodeById.get(other).kind !== 'subject' && carrierOk(other))) && overlaps(e);
    }
    return core.has(e.source) && core.has(e.target) && !(f.from || f.to);   // shared-contact and successor links are computed on the whole record, so they cannot be cut to a period
  });
  const used = new Set(core);
  for (const e of edges) { used.add(e.source); used.add(e.target); }
  const nodes = g.nodes.filter(n => used.has(n.id)).map(n => (n.kind === 'subject' ? n : { ...n, degree: edges.filter(e => e.source === n.id || e.target === n.id).length }));
  const keepNumber = id => core.has(id);
  const operators = new Set(subjects.filter(s => core.has(s.id)).map(s => s.operator));
  const clip = (points) => (f.from || f.to ? points.filter(p => (!f.from || p[0].slice(0, 10) >= f.from) && (!f.to || p[0].slice(0, 10) <= f.to)) : points);
  const movers = (views.map.movers || []).filter(m => keepNumber(m.number)).map(m => ({ ...m, points: clip(m.points) }));
  const visited = new Set(movers.flatMap(m => Object.keys(m.visits || {})));
  const label2number = new Map(subjects.map(s => [s.label, s.id]));
  const labelIn = label => core.has(label) || core.has(label2number.get(label)) || [...core].some(id => id.endsWith(String(label)));
  const out = {
    ...views,
    case: { ...views.case, subjects: core.size, filtered: true },
    graph: { nodes, edges },
    map: { ...views.map, cells: f.targets.size || f.carriers.size ? views.map.cells.filter(c => operators.has(c.operator) || visited.has(c.key)) : views.map.cells, movers, colocation: (views.map.colocation || []).filter(c => keepNumber(c.x) && keepNumber(c.y)) },
    lanes: { devices: (views.lanes?.devices || []).filter(l => keepNumber(l.number)), shared_devices: (views.lanes?.shared_devices || []).filter(d => d.numbers.some(n => labelIn(n.label))) },
    pairs: (views.pairs || []).filter(p => keepNumber(p.x) && keepNumber(p.y)),
    rhythm: (views.rhythm || []).filter(r => keepNumber(r.number)),
    addresses: f.targets.size ? (views.addresses || []).filter(a => a.holders.some(labelIn)) : views.addresses,
  };
  return out;
}

export function summarise(base, shown) {
  const count = v => ({ subjects: v.graph.nodes.filter(n => n.kind === 'subject').length, numbers: v.graph.nodes.length, links: v.graph.edges.length, cells: v.map.cells.length });
  return { all: count(base), shown: count(shown) };
}

// ---- the filter bar ------------------------------------------------------------------------------------------------------------------------------------------------
export function mountFilterBar(root, base, state, status = () => '') {
  root.replaceChildren();
  const subjects = base.graph.nodes.filter(n => n.kind === 'subject').sort((a, b) => (b.events || 0) - (a.events || 0));
  const carrierCounts = new Map();
  for (const n of base.graph.nodes) { const e = carrierCounts.get(n.carrier) || { subjects: 0, numbers: 0 }; e.numbers++; if (n.kind === 'subject') e.subjects++; carrierCounts.set(n.carrier, e); }
  const present = CARRIER_ORDER.filter(id => carrierCounts.has(id));

  // targets
  const pop = h('div', { class: 'fb-pop', role: 'dialog', 'aria-label': 'Choose targets', hidden: true });
  const targetBtn = h('button', { class: 'fb-btn', 'aria-haspopup': 'dialog', 'aria-expanded': 'false', onclick: () => togglePop() });
  const listBox = h('div', { class: 'fb-list' });
  const q = h('input', { type: 'search', placeholder: 'Find a target', 'aria-label': 'Find a target', oninput: () => drawList() });
  function drawList() {
    const needle = digits(q.value);
    const rows = subjects.filter(s => !needle || digits(s.id).includes(needle) || digits(s.label).includes(needle));
    listBox.replaceChildren(...rows.map(s => {
      const id = 'fb-t-' + s.id;
      const input = h('input', { type: 'checkbox', id, checked: state.targets.has(s.id), onchange: () => { const next = new Set(state.targets); if (input.checked) next.add(s.id); else next.delete(s.id); state.set({ targets: next }); } });
      return h('label', { class: 'fb-row', for: id }, input, carrierIcon(s.carrier, 18), h('span', { class: 'fb-num' }, s.label), h('span', { class: 'fb-meta' }, `${(s.events || 0).toLocaleString('en')} events · ${CARRIERS[s.carrier].name}`));
    }));
    if (!rows.length) listBox.append(h('p', { class: 'muted small' }, 'No target matches.'));
  }
  function togglePop(force) {
    const open = force ?? pop.hidden;
    pop.hidden = !open; targetBtn.setAttribute('aria-expanded', String(open));
    if (open) { drawList(); q.focus(); }
  }
  pop.append(h('div', { class: 'fb-pophead' }, q,
    h('button', { class: 'ghost', onclick: () => { state.set({ targets: subjects.map(s => s.id) }); drawList(); } }, 'All'), h('button', { class: 'ghost', onclick: () => { state.set({ targets: [] }); drawList(); } }, 'None'),
    h('button', { class: 'ghost', onclick: () => { state.set({ targets: subjects.slice(0, 5).map(s => s.id) }); drawList(); }, title: 'The five subjects with the most events' }, 'Top 5')), listBox);
  document.addEventListener('click', ev => { if (!pop.hidden && !root.contains(ev.target)) togglePop(false); });
  document.addEventListener('keydown', ev => { if (ev.key === 'Escape' && !pop.hidden) { togglePop(false); targetBtn.focus(); } });

  // carriers
  const chips = present.map(id => {
    const b = h('button', { class: 'fb-chip', 'aria-pressed': 'false', title: `${CARRIERS[id].name}: ${carrierCounts.get(id).subjects} subjects, ${carrierCounts.get(id).numbers} numbers. ${CARRIERS[id].note}`,
      onclick: () => { const next = new Set(state.carriers); if (next.has(id)) next.delete(id); else next.add(id); state.set({ carriers: next }); } }, carrierIcon(id, 16), ' ', CARRIERS[id].name, ' ', h('small', {}, String(carrierCounts.get(id).numbers)));
    b.dataset.carrier = id; return b;
  });

  // period
  const lo = base.case.start; const hi = new Date(Date.parse(base.case.start + 'T00:00:00Z') + (base.case.days - 1) * 86400000).toISOString().slice(0, 10);
  const from = h('input', { type: 'date', min: lo, max: hi, 'aria-label': 'From', onchange: () => state.set({ from: from.value }) });
  const to = h('input', { type: 'date', min: lo, max: hi, 'aria-label': 'To', onchange: () => state.set({ to: to.value }) });
  const rel = days => () => state.set({ from: new Date(Date.parse(hi + 'T00:00:00Z') - (days - 1) * 86400000).toISOString().slice(0, 10), to: hi });
  const preset = h('div', { class: 'fb-presets' }, ...[['Whole', () => state.set({ from: '', to: '' })], ['90 days', rel(90)], ['30 days', rel(30)], ['7 days', rel(7)]].map(([t, fn]) => h('button', { class: 'ghost', onclick: fn }, t)));

  const summary = h('output', { class: 'fb-summary', 'aria-live': 'polite' });
  const reset = h('button', { class: 'ghost', onclick: () => state.reset() }, 'Reset');
  const copy = h('button', { class: 'ghost', title: 'Copy a link that opens these filters', onclick: async () => { try { await navigator.clipboard.writeText(state.link()); copy.textContent = 'Copied'; } catch { copy.textContent = state.link(); } setTimeout(() => { copy.textContent = 'Copy link'; }, 1800); } }, 'Copy link');

  root.append(
    h('div', { class: 'fb-group' }, h('span', { class: 'gl' }, 'Targets'), h('div', { class: 'fb-anchor' }, targetBtn, pop)),
    h('div', { class: 'fb-group' }, h('span', { class: 'gl' }, 'Carrier'), ...chips),
    h('div', { class: 'fb-group' }, h('span', { class: 'gl' }, 'Period'), from, h('span', {}, 'to'), to, preset),
    h('div', { class: 'fb-group right' }, summary, reset, copy));

  function refresh(shown) {
    targetBtn.replaceChildren(h('span', {}, state.targets.size ? `${state.targets.size} of ${subjects.length} selected` : `All ${subjects.length} subjects`), h('span', { 'aria-hidden': 'true' }, ' ▾'));
    for (const b of chips) b.setAttribute('aria-pressed', String(state.carriers.has(b.dataset.carrier)));
    from.value = state.from; to.value = state.to;
    reset.disabled = !state.active; root.classList.toggle('is-filtered', state.active);
    if (shown) { const s = summarise(base, shown); summary.textContent = state.active ? `Showing ${s.shown.subjects} of ${s.all.subjects} subjects · ${s.shown.numbers} of ${s.all.numbers} numbers · ${s.shown.links} of ${s.all.links} links` : `${s.all.subjects} subjects · ${s.all.numbers} numbers · ${s.all.links} links`; }
    const note = status(); if (note) summary.append(' · ', note);
  }
  refresh(base);
  return { refresh, openTargets: () => togglePop(true) };
}
