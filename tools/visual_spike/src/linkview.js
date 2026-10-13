// The link chart view (analyses A1 to A4): who talks to whom, who shares contacts, how two numbers are connected.
// Built on Cytoscape.js in its canvas mode, the one that draws every encoding (shapes, dashed and dotted lines, arrows).
import cytoscape from 'cytoscape';
import { layout } from './layout.js';
import { label as lbl, palette, state } from './common.js';
import { $, h, badge, chip, help, table, download, esc, fmtDT, plural, OPERATOR_COLOURS } from './ui.js';

const KIND_TEXT = {
  observed: 'Observed: the records show calls or messages between these two numbers.',
  inferred: 'Inferred: the two numbers share contacts. They were not necessarily in touch with each other.',
  candidate: 'Candidate: behavioural similarity between a number that went silent and a newer one. Not a finding, and not identity.',
};

export function mountLinkView(root, views, opts = {}) {
  root.replaceChildren();
  const g = views.graph; const c = palette();
  const byId = new Map(g.nodes.map(n => [n.id, n])); const subjects = g.nodes.filter(n => n.kind === 'subject');
  const edges = g.edges.map((e, i) => ({ ...e, idx: i }));
  const adj = new Map(); const addAdj = (a, b, e) => { if (!adj.has(a)) adj.set(a, []); adj.get(a).push({ to: b, e }); };
  for (const e of edges) if (e.kind !== 'candidate') { addAdj(e.source, e.target, e); addAdj(e.target, e.source, e); }
  const nodeLabel = id => lbl(byId.get(id)?.label ?? String(id).slice(-5), byId.get(id)?.kind);
  const callsOf = (a, b) => edges.find(e => e.kind === 'observed' && ((e.source === a && e.target === b) || (e.source === b && e.target === a)));

  // ---- layout and elements ------------------------------------------------------------------------------------------------------------
  const pos = layout(g.nodes, edges.filter(e => e.kind !== 'candidate'));
  const degree = id => (adj.get(id) || []).length;
  const els = [
    ...g.nodes.map(n => ({ data: { id: n.id, label: lbl(n.label, n.kind), kind: n.kind, size: 16 + Math.min(30, degree(n.id) * 2.2) + (n.kind === 'subject' ? 12 : 0), color: n.kind === 'subject' ? (OPERATOR_COLOURS[n.operator] || c.subject) : n.kind === 'hub' ? c.hub : '#8a94a6' }, position: pos.get(n.id) })),
    ...edges.map(e => ({ data: { id: 'e' + e.idx, source: e.source, target: e.target, kind: e.kind, count: e.count, idx: e.idx, width: e.kind === 'observed' ? 1 + Math.min(4, Math.log2(1 + e.count) / 1.6) : e.kind === 'inferred' ? 1.5 + Math.min(3, e.count / 2) : 2 } })),
  ];

  // ---- page structure -----------------------------------------------------------------------------------------------------------------
  const tooltip = h('div', { class: 'tip', hidden: true });
  const canvas = h('div', { id: 'graph', tabindex: 0, role: 'application', 'aria-label': 'Link chart. Arrow keys move between nodes, Enter opens a node, Escape clears, slash searches.' });
  const panel = h('aside', { class: 'panel', id: 'link-panel', 'aria-live': 'polite' });
  const tableBox = h('div', { id: 'graph-table', hidden: true });
  const search = h('input', { type: 'search', placeholder: 'Find a number (last digits)…', list: 'node-list', 'aria-label': 'Find a number', class: 'search' });
  const datalist = h('datalist', { id: 'node-list' }, g.nodes.map(n => h('option', { value: n.label }, `${n.kind}${n.operator ? ' · ' + n.operator : ''}`)));
  const toggles = {};
  const toggle = (key, text, on = true, title) => { const input = h('input', { type: 'checkbox', checked: on, onchange: applyFilters }); toggles[key] = input; return h('label', { class: 'tog', title }, input, ' ', text); };
  const minCalls = h('input', { type: 'range', min: 1, max: 12, value: 1, 'aria-label': 'Minimum calls for an observed link', oninput: () => { minOut.textContent = minCalls.value; applyFilters(); } });
  const minOut = h('output', {}, '1');
  const slotA = h('span', { class: 'slot' }), slotB = h('span', { class: 'slot' });
  const btnShared = h('button', { onclick: () => showCompare(), disabled: true, title: 'A2: contacts both numbers have, hubs and service numbers left out' }, 'Shared contacts');
  const btnPath = h('button', { onclick: () => showPath(), disabled: true, title: 'A3: shortest connection over observed and inferred links, at most four hops' }, 'Connection path');
  const focusToggle = toggle('focus', 'Dim the rest when I select', true, 'Selecting a number or link dims everything that is not next to it');
  const toolbar = h('div', { class: 'toolbar' },
    h('div', { class: 'group' }, h('span', { class: 'gl' }, 'Find'), search, datalist),
    h('div', { class: 'group' }, h('span', { class: 'gl' }, 'Show'), toggle('observed', 'Calls and messages', true), toggle('inferred', 'Shared contacts', true), toggle('candidate', 'Successor candidates', true), toggle('people', 'Contacts', true, 'Turn off to see only the subjects')),
    h('div', { class: 'group' }, h('span', { class: 'gl' }, 'Links with'), minCalls, h('span', {}, minOut, ' or more calls')),
    focusToggle,
    h('div', { class: 'group' }, h('span', { class: 'gl' }, 'Compare'), slotA, slotB, btnShared, btnPath, h('button', { class: 'ghost', onclick: () => { setSlot('A', null); setSlot('B', null); resetStyles(); showOverview(); } }, 'Clear')),
    h('div', { class: 'group right' }, h('button', { onclick: () => cy.fit(undefined, 30) }, 'Fit'), h('button', { onclick: () => download('link-chart.png', cy.png({ scale: 2, full: true, bg: getComputedStyle(document.documentElement).getPropertyValue('--panel').trim() })) }, 'Save PNG'), h('button', { onclick: toggleTable, id: 'table-toggle' }, 'View as table')));
  const legend = h('div', { class: 'legend2', 'aria-label': 'Legend' },
    h('span', { class: 'lg' }, 'Subjects', ...Object.entries(OPERATOR_COLOURS).map(([k, v]) => h('span', { class: 'lg' }, h('span', { class: 'dot', style: `background:${v}` }, ''), k))),
    h('span', { class: 'lg' }, h('i', { class: 'sw circ' }), 'contact'), h('span', { class: 'lg' }, h('i', { class: 'sw tri' }), 'contacted by most subjects (left out of shared contacts)'),
    h('span', { class: 'lg' }, h('i', { class: 'ln solid' }), 'observed, thicker = more calls'), h('span', { class: 'lg' }, h('i', { class: 'ln dashed' }), 'inferred from shared contacts'), h('span', { class: 'lg' }, h('i', { class: 'ln dotted' }), 'candidate successor →'));
  root.append(toolbar, h('div', { class: 'split' }, h('div', { class: 'canvasbox' }, canvas, tooltip, legend), panel), tableBox);

  // ---- the chart ----------------------------------------------------------------------------------------------------------------------
  const cy = cytoscape({
    container: canvas, elements: els, layout: { name: 'preset' }, wheelSensitivity: 0.25, minZoom: 0.15, maxZoom: 2.2, pixelRatio: 'auto',
    style: [
      { selector: 'node', style: { 'background-color': 'data(color)', width: 'data(size)', height: 'data(size)', label: 'data(label)', 'font-size': 11, color: c.ink, 'text-valign': 'bottom', 'text-margin-y': 3, 'min-zoomed-font-size': 9, 'text-outline-color': getComputedStyle(document.documentElement).getPropertyValue('--panel').trim(), 'text-outline-width': 2, 'border-width': 1.5, 'border-color': getComputedStyle(document.documentElement).getPropertyValue('--panel').trim() } },
      { selector: 'node[kind="subject"]', style: { shape: 'diamond', 'font-weight': 700, 'font-size': 12, 'min-zoomed-font-size': 5 } },
      { selector: 'node[kind="hub"]', style: { shape: 'triangle' } },
      { selector: 'edge', style: { width: 'data(width)', 'curve-style': 'haystack', 'line-color': '#7d8797', opacity: 0.65, 'haystack-radius': 0 } },
      { selector: 'edge[kind="inferred"]', style: { 'line-style': 'dashed', 'line-dash-pattern': [8, 5], 'line-color': c.inferred, 'curve-style': 'bezier', opacity: 0.95 } },
      { selector: 'edge[kind="candidate"]', style: { 'line-style': 'dotted', 'line-color': c.candidate, 'curve-style': 'bezier', 'target-arrow-shape': 'triangle', 'target-arrow-color': c.candidate, 'arrow-scale': 1.2, opacity: 1 } },
      { selector: '.hidden', style: { display: 'none' } },
      { selector: '.dim', style: { opacity: 0.1, 'text-opacity': 0 } },
      { selector: 'node.hl', style: { 'border-width': 3, 'border-color': c.ink } },
      { selector: 'edge.hl', style: { opacity: 1, 'z-index': 9 } },
      { selector: 'node.shared', style: { 'border-width': 4, 'border-color': c.ok, 'z-index': 10 } },
      { selector: 'edge.shared', style: { 'line-color': c.ok, opacity: 1, 'z-index': 9, width: 3 } },
      { selector: 'node.path', style: { 'border-width': 4, 'border-color': c.alert, 'z-index': 10 } },
      { selector: 'edge.path', style: { 'line-color': c.alert, 'target-arrow-color': c.alert, opacity: 1, 'z-index': 10, width: 4, 'line-style': 'solid', 'curve-style': 'bezier' } },
      { selector: 'node.slotA, node.slotB', style: { 'border-width': 5, 'border-color': c.accent || '#1f5fbf', 'z-index': 11 } },
      { selector: ':selected', style: { 'overlay-color': c.alert, 'overlay-opacity': 0.18, 'overlay-padding': 6 } },
    ],
  });
  cy.fit(undefined, 40);

  let slots = { A: null, B: null };
  function setSlot(which, id) {
    slots[which] = id; cy.nodes().removeClass('slot' + which); if (id) cy.getElementById(id).addClass('slot' + which);
    const el = which === 'A' ? slotA : slotB; el.replaceChildren(id ? h('span', { class: 'chipbtn', title: 'Click to select' , onclick: () => selectNode(id) }, `${which}: ${nodeLabel(id)}`) : h('span', { class: 'muted' }, `${which}: not set`));
    btnShared.disabled = !(slots.A && slots.B) || byId.get(slots.A)?.kind !== 'subject' || byId.get(slots.B)?.kind !== 'subject'; btnPath.disabled = !(slots.A && slots.B);
    btnShared.title = btnShared.disabled && slots.A && slots.B ? 'Shared contacts are compared between two subjects' : btnShared.title;
  }
  setSlot('A', null); setSlot('B', null);

  function resetStyles() { cy.elements().removeClass('dim hl shared path'); cy.elements().unselect(); }
  function dimExcept(eles) { if (!toggles.focus.checked) return; cy.elements().addClass('dim'); eles.removeClass('dim'); }

  function applyFilters() {
    const showKind = { observed: toggles.observed.checked, inferred: toggles.inferred.checked, candidate: toggles.candidate.checked }; const people = toggles.people.checked; const min = +minCalls.value;
    cy.batch(() => {
      cy.nodes().forEach(n => n.toggleClass('hidden', n.data('kind') !== 'subject' && !people));
      cy.edges().forEach(e => { const k = e.data('kind'); const hide = !showKind[k] || (k === 'observed' && e.data('count') < min) || e.source().hasClass('hidden') || e.target().hasClass('hidden'); e.toggleClass('hidden', hide); });
      if (min > 1 || !people) cy.nodes('[kind != "subject"]').forEach(n => { if (!n.hasClass('hidden') && n.connectedEdges(':visible').empty()) n.addClass('hidden'); });
    });
  }

  // ---- panels -------------------------------------------------------------------------------------------------------------------------
  function evidenceTable(rows, caption) {
    return table(['Time', 'Type', 'Direction', 'Seconds', 'Of'], rows.map(r => ({ cells: [fmtDT(r.time), r.type, r.direction, r.duration_s ?? '', lbl(r.owner)], sort: [r.time, r.type, r.direction, r.duration_s ?? 0, r.owner] })), { caption });
  }
  function setPanel(...kids) { panel.replaceChildren(...kids); panel.scrollTop = 0; }
  function showOverview() {
    const counts = k => edges.filter(e => e.kind === k).length;
    const hubs = g.nodes.filter(n => n.kind === 'hub');
    const top = g.nodes.filter(n => n.kind === 'contact').map(n => ({ n, d: edges.filter(e => e.kind === 'observed' && (e.source === n.id || e.target === n.id)).length })).sort((a, b) => b.d - a.d || a.n.id.localeCompare(b.n.id)).slice(0, 5);
    setPanel(
      h('h2', {}, 'This case at a glance'),
      h('div', { class: 'chips' }, chip(subjects.length, 'subjects'), chip(g.nodes.filter(n => n.kind === 'contact').length, 'other numbers'), chip(hubs.length, plural(hubs.length, 'hub')), chip(counts('observed'), 'observed links'), chip(counts('inferred'), 'shared-contact links'), chip(counts('candidate'), 'successor candidates')),
      help('How to read this chart', 'Each diamond is a subject (a number whose records we hold); its colour is its operator. Circles are other numbers they called. A triangle is a number contacted by most subjects: it is left out when comparing subjects, because it says little.', 'A solid line is calls or messages seen in the records (thicker means more). A dashed purple line joins two subjects who share contacts. A dotted amber arrow joins a number that went silent to a candidate successor.', 'Click a number or a line to see the rows behind it. Mark two numbers as A and B to list their shared contacts or to find how they connect.'),
      h('h3', {}, 'Where to start'),
      h('ul', { class: 'jump' },
        ...edges.filter(e => e.kind === 'inferred').sort((a, b) => b.count - a.count).slice(0, 5).map(e => h('li', {}, h('button', { class: 'linkish', onclick: () => { setSlot('A', e.source); setSlot('B', e.target); showCompare(); } }, `${nodeLabel(e.source)} and ${nodeLabel(e.target)}`), ` share ${plural(e.count, 'contact')}`)),
        ...edges.filter(e => e.kind === 'candidate').map(e => h('li', {}, h('button', { class: 'linkish', onclick: () => selectEdge(cy.getElementById('e' + e.idx)) }, `${nodeLabel(e.source)} → ${nodeLabel(e.target)}`), ` candidate successor (score ${e.score}) `, badge('not a finding', 'warn'))),
      ),
      h('h3', {}, 'Most-called other numbers'),
      h('ul', { class: 'jump' }, ...top.map(t => h('li', {}, h('button', { class: 'linkish', onclick: () => selectNode(t.n.id) }, lbl(t.n.label)), ` called by ${plural(t.d, 'subject')}`))),
    );
  }
  function showNode(id) {
    const n = byId.get(id); const mine = edges.filter(e => e.source === id || e.target === id);
    const obs = mine.filter(e => e.kind === 'observed').sort((a, b) => b.count - a.count);
    const evidence = obs.flatMap(e => e.rows).sort((a, b) => a.time.localeCompare(b.time)).slice(0, 12);
    setPanel(
      h('h2', {}, nodeLabel(id), ' ', badge(n.kind === 'subject' ? 'subject' : n.kind === 'hub' ? 'hub' : 'contact', n.kind === 'subject' ? 'info' : 'neutral'), n.operator ? badge(n.operator, 'op') : null),
      h('div', { class: 'chips' }, n.events != null ? chip(n.events, 'events in the records') : null, chip(obs.length, plural(obs.length, 'number', 'numbers') + ' in contact'), chip(obs.reduce((s, e) => s + e.count, 0), 'calls and messages seen'), n.first ? chip(fmtDT(n.first).slice(0, 10), 'first event') : null, n.last ? chip(fmtDT(n.last).slice(0, 10), 'last event') : null),
      h('div', { class: 'row' }, h('button', { onclick: () => { setSlot('A', id); cy.getElementById(id).addClass('slotA'); } }, 'Set as A'), h('button', { onclick: () => setSlot('B', id) }, 'Set as B'), h('button', { class: 'ghost', onclick: () => cy.animate({ fit: { eles: cy.getElementById(id).closedNeighborhood(':visible'), padding: 60 } }, { duration: 250 }) }, 'Zoom to neighbours')),
      n.kind === 'hub' ? h('p', { class: 'note' }, 'Contacted by most subjects. It is left out of shared-contact comparisons, because sharing it says little.') : null,
      h('h3', {}, 'In contact with'),
      h('ul', { class: 'jump' }, ...obs.slice(0, 8).map(e => { const other = e.source === id ? e.target : e.source; return h('li', {}, h('button', { class: 'linkish', onclick: () => selectNode(other) }, nodeLabel(other)), ` ${plural(e.count, 'call or message', 'calls or messages')}`); }), obs.length > 8 ? h('li', { class: 'muted' }, `and ${obs.length - 8} more`) : null),
      h('h3', {}, 'Evidence rows'), h('p', { class: 'muted small' }, 'The first rows of the events behind these links. Click a link in the chart for its own rows.'), evidenceTable(evidence),
    );
  }
  function showEdge(e) {
    const d = edges[e.data('idx')];
    setPanel(
      h('h2', {}, `${nodeLabel(d.source)} ${d.kind === 'candidate' ? '→' : '—'} ${nodeLabel(d.target)}`, ' ', badge(d.kind, d.kind === 'observed' ? 'neutral' : d.kind === 'inferred' ? 'purple' : 'warn')),
      h('p', { class: 'note' }, KIND_TEXT[d.kind]),
      h('div', { class: 'chips' }, d.kind === 'observed' ? chip(d.count, 'calls and messages') : d.kind === 'inferred' ? chip(d.count, 'shared contacts') : chip(d.score, 'similarity score'), d.first ? chip(fmtDT(d.first).slice(0, 10), 'first') : null, d.last ? chip(fmtDT(d.last).slice(0, 10), 'last') : null),
      d.kind === 'inferred' ? h('p', {}, h('button', { onclick: () => { setSlot('A', d.source); setSlot('B', d.target); showCompare(); } }, 'List the shared contacts')) : null,
      h('h3', {}, 'Evidence rows'), evidenceTable(d.rows, d.kind === 'observed' && d.count > d.rows.length ? `First ${d.rows.length} of ${d.count} rows` : null),
    );
  }
  function showCompare() {
    resetStyles(); const A = slots.A, B = slots.B; if (!A || !B) return;
    const nb = id => new Map((adj.get(id) || []).filter(x => x.e.kind === 'observed').map(x => [x.to, x.e.count]));
    const na = nb(A), nbb = nb(B); const both = [...na.keys()].filter(k => nbb.has(k) && k !== A && k !== B);
    const shared = both.filter(k => byId.get(k).kind !== 'hub'); const left = both.filter(k => byId.get(k).kind === 'hub');
    const oracle = edges.find(e => e.kind === 'inferred' && ((e.source === A && e.target === B) || (e.source === B && e.target === A)));
    const eles = cy.collection(); [A, B, ...shared].forEach(id => eles.merge(cy.getElementById(id)));
    shared.forEach(k => { eles.merge(cy.getElementById(k)); cy.edges().filter(e => (e.data('source') === k || e.data('target') === k) && (e.data('source') === A || e.data('target') === A || e.data('source') === B || e.data('target') === B) && e.data('kind') === 'observed').forEach(e => { e.addClass('shared'); eles.merge(e); }); cy.getElementById(k).addClass('shared'); });
    dimExcept(eles); cy.animate({ fit: { eles, padding: 70 } }, { duration: 300 });
    const ok = oracle ? oracle.count === shared.length : shared.length === 0;
    setPanel(
      h('h2', {}, 'Shared contacts: ', nodeLabel(A), ' and ', nodeLabel(B)),
      h('div', { class: 'chips' }, chip(shared.length, plural(shared.length, 'shared contact')), left.length ? chip(left.length, 'left out (hub)') : null),
      h('p', { class: ok ? 'note good' : 'note bad' }, ok ? `✓ Matches the answer computed from the records (${oracle ? oracle.count : 0}).` : `✗ The chart finds ${shared.length} but the computed answer says ${oracle ? oracle.count : 0}.`),
      shared.length ? table(['Number', nodeLabel(A) + ' calls', nodeLabel(B) + ' calls'], shared.map(k => ({ cells: [h('button', { class: 'linkish', onclick: () => selectNode(k) }, nodeLabel(k)), na.get(k), nbb.get(k)], sort: [byId.get(k).label, na.get(k), nbb.get(k)] }))) : h('p', { class: 'muted' }, 'These two numbers have no contacts in common (after leaving out hubs).'),
      left.length ? h('p', { class: 'muted small' }, `Left out: ${left.map(nodeLabel).join(', ')}, contacted by most subjects, so sharing it says little.`) : null,
      h('p', { class: 'muted small' }, 'Shared contacts are a reason to look closer, not proof that the two know each other.'),
    );
  }
  function showPath() {
    resetStyles(); const A = slots.A, B = slots.B; if (!A || !B) return;
    const prev = new Map([[A, null]]); const depth = new Map([[A, 0]]); const queue = [A];
    while (queue.length) { const u = queue.shift(); if (u === B) break; if (depth.get(u) >= 4) continue; for (const { to, e } of [...(adj.get(u) || [])].sort((x, y) => (y.e.kind === 'observed') - (x.e.kind === 'observed') || y.e.count - x.e.count || x.to.localeCompare(y.to))) if (!prev.has(to)) { prev.set(to, { from: u, e }); depth.set(to, depth.get(u) + 1); queue.push(to); } }
    if (!prev.has(B)) { setPanel(h('h2', {}, 'Connection path'), h('p', {}, `No connection from ${nodeLabel(A)} to ${nodeLabel(B)} within four hops over observed and shared-contact links.`), h('p', { class: 'muted small' }, 'Absence of a path in these records is not proof that there is none.')); return; }
    const steps = []; let cur = B; while (prev.get(cur)) { steps.unshift({ from: prev.get(cur).from, to: cur, e: prev.get(cur).e }); cur = prev.get(cur).from; }
    const eles = cy.collection(); eles.merge(cy.getElementById(A)); steps.forEach(s => { eles.merge(cy.getElementById(s.to)); const ce = cy.getElementById('e' + s.e.idx); ce.addClass('path'); eles.merge(ce); cy.getElementById(s.to).addClass('path'); }); cy.getElementById(A).addClass('path');
    dimExcept(eles); cy.animate({ fit: { eles, padding: 80 } }, { duration: 300 });
    setPanel(
      h('h2', {}, 'Connection path'), h('div', { class: 'chips' }, chip(steps.length, plural(steps.length, 'hop')), chip(steps.filter(s => s.e.kind === 'inferred').length, 'inferred')),
      h('ol', { class: 'steps' }, h('li', {}, h('b', {}, nodeLabel(A))), ...steps.map(s => h('li', {}, badge(s.e.kind, s.e.kind === 'observed' ? 'neutral' : 'purple'), ` ${s.e.kind === 'observed' ? plural(s.e.count, 'call or message', 'calls or messages') : plural(s.e.count, 'shared contact')} `, h('button', { class: 'linkish', onclick: () => selectNode(s.to) }, nodeLabel(s.to))))),
      h('p', { class: 'muted small' }, 'The shortest route over observed and inferred links, at most four hops. A route is a lead, not proof of a relationship.'),
    );
  }
  function showTable() {
    tableBox.replaceChildren(table(['From', 'To', 'Kind', 'Calls or shared', 'First', 'Last'], edges.map(e => ({ cells: [nodeLabel(e.source), nodeLabel(e.target), badge(e.kind, e.kind === 'observed' ? 'neutral' : e.kind === 'inferred' ? 'purple' : 'warn'), e.count, fmtDT(e.first), fmtDT(e.last)], sort: [e.source, e.target, e.kind, e.count, e.first || '', e.last || ''] })), { caption: `All ${edges.length} links`, onRow: r => { toggleTable(); selectEdge(cy.getElementById('e' + edges.findIndex(e => nodeLabel(e.source) === r.cells[0] && nodeLabel(e.target) === r.cells[1] && e.kind === r.sort[2]))); } }));
  }
  function toggleTable() { tableBox.hidden = !tableBox.hidden; $('#table-toggle', root).textContent = tableBox.hidden ? 'View as table' : 'Hide table'; if (!tableBox.hidden) showTable(); }

  function selectNode(id) {
    resetStyles(); const n = cy.getElementById(id); if (n.empty()) return; if (n.hasClass('hidden')) { n.removeClass('hidden'); }
    n.select(); n.addClass('hl'); const nb = n.closedNeighborhood(':visible'); nb.edges().addClass('hl'); dimExcept(nb); showNode(id);
    if (!cy.extent() || !isInView(n)) cy.animate({ center: { eles: n } }, { duration: 200 });
  }
  function isInView(n) { const e = cy.extent(), p = n.position(); return p.x > e.x1 && p.x < e.x2 && p.y > e.y1 && p.y < e.y2; }
  function selectEdge(e) { if (e.empty()) return; resetStyles(); e.select(); e.addClass('hl'); const nb = e.connectedNodes(); nb.addClass('hl'); dimExcept(e.union(nb)); showEdge(e); }

  cy.on('tap', 'node', ev => selectNode(ev.target.id()));
  cy.on('tap', 'edge', ev => selectEdge(ev.target));
  cy.on('tap', ev => { if (ev.target === cy) { resetStyles(); showOverview(); } });
  cy.on('dbltap', 'node', ev => cy.animate({ fit: { eles: ev.target.closedNeighborhood(':visible'), padding: 60 } }, { duration: 250 }));
  const tipText = ele => ele.isNode() ? `${ele.data('label')} · ${byId.get(ele.id()).kind}${byId.get(ele.id()).operator ? ' · ' + byId.get(ele.id()).operator : ''} · ${plural(degree(ele.id()), 'link')}` : `${ele.data('kind')} · ${ele.data('kind') === 'inferred' ? plural(ele.data('count'), 'shared contact') : ele.data('kind') === 'observed' ? plural(ele.data('count'), 'call or message', 'calls or messages') : 'candidate'}`;
  cy.on('mouseover', 'node, edge', ev => { const p = ev.renderedPosition || ev.target.renderedMidpoint(); tooltip.textContent = tipText(ev.target); tooltip.style.left = (p.x + 12) + 'px'; tooltip.style.top = (p.y + 12) + 'px'; tooltip.hidden = false; canvas.style.cursor = 'pointer'; });
  cy.on('mouseout', 'node, edge', () => { tooltip.hidden = true; canvas.style.cursor = ''; });
  cy.on('pan zoom', () => { tooltip.hidden = true; });

  search.addEventListener('change', () => { const q = search.value.trim(); const n = g.nodes.find(x => x.label === q) || g.nodes.find(x => x.label.endsWith(q)); if (n) { selectNode(n.id); search.value = ''; } });
  // keyboard: Tab to the chart, arrows move between numbers in a fixed order, Enter opens one, Escape clears, / goes to the search box
  let kbIndex = -1; const order = [...g.nodes].sort((a, b) => (a.kind === 'subject' ? 0 : 1) - (b.kind === 'subject' ? 0 : 1) || a.label.localeCompare(b.label)).map(n => n.id);
  canvas.addEventListener('keydown', ev => {
    if (ev.key === 'ArrowRight' || ev.key === 'ArrowDown') kbIndex = (kbIndex + 1) % order.length;
    else if (ev.key === 'ArrowLeft' || ev.key === 'ArrowUp') kbIndex = (kbIndex - 1 + order.length) % order.length;
    else if (ev.key === 'Enter' && kbIndex >= 0) { selectNode(order[kbIndex]); return; }
    else if (ev.key === 'Escape') { resetStyles(); showOverview(); return; }
    else if (ev.key === '/') { ev.preventDefault(); search.focus(); return; }
    else return;
    ev.preventDefault(); const id = order[kbIndex]; cy.elements().unselect(); cy.getElementById(id).select(); cy.animate({ center: { eles: cy.getElementById(id) } }, { duration: 100 });
    canvas.setAttribute('aria-label', `Link chart. Number ${kbIndex + 1} of ${order.length}: ${nodeLabel(id)}. Enter opens it.`);
  });
  window.__cy = cy; showOverview();
  return { cy, destroy: () => cy.destroy(), selectNode, setSlot, showCompare, showPath };
}
