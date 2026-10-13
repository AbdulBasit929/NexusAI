// The map view (analyses A5, A6, A7, A10): cell sites with their uncertainty, where a subject sleeps and works and how that changes, speed alerts,
// co-location windows. MapLibre GL JS with an offline raster basemap from a local tile server (address from the run-time configuration),
// drawn muted so that the analysis layers stand out; with no basemap configured the map falls back to a neutral grid.
import * as maplibregl from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';
import { MapboxOverlay } from '@deck.gl/mapbox';
import { ScatterplotLayer } from '@deck.gl/layers';
import { PLACEMENTS, placer } from './config.js';
import { rng, framesPerSecond, timed, label as lbl } from './common.js';
import { h, badge, chip, help, table, esc, fmtDT, plural, download, OPERATOR_COLOURS, shortCell, siteName } from './ui.js';

maplibregl.setWorkerUrl(new URL('/maplibre/maplibre-gl-worker.mjs', location.href).href);   // served by us (copied by scripts/copy-worker.mjs); no CDN
const epoch = s => Date.parse(s.replace(' ', 'T') + 'Z') / 1000;
const iso = t => new Date(t * 1000).toISOString().replace('T', ' ').slice(0, 16);
const circle = (lon, lat, m, steps = 48) => { const ring = []; for (let i = 0; i <= steps; i++) { const a = (i / steps) * 2 * Math.PI; ring.push([lon + (m * Math.cos(a)) / (111320 * Math.cos(lat * Math.PI / 180)), lat + (m * Math.sin(a)) / 110574]); } return ring; };
const fc = feats => ({ type: 'FeatureCollection', features: feats });

export function mountMapView(root, views, config) {
  root.replaceChildren();
  const dark = matchMedia('(prefers-color-scheme: dark)').matches;
  const cells = new Map(views.map.cells.map(c => [c.key, c]));
  const movers = views.map.movers; const pairs = views.pairs;
  const origin = views.case.origin;
  const st = { place: placer(origin, origin), placement: 'ocean', mover: movers[0]?.number || null, pair: 0, time: 1, playing: null, layers: { cells: true, dwell: true, trail: true, alerts: true, anchors: true, colocation: true }, operator: '', markers: [], popup: null, overlay: null, cur: null };

  // ---- structure ----------------------------------------------------------------------------------------------------------------------
  const mapDiv = h('div', { id: 'map' });
  const status = h('span', { class: 'status' });
  const placeSel = h('select', { 'aria-label': 'Where to draw this case', onchange: () => { st.placement = placeSel.value; const t = PLACEMENTS.find(p => p.id === st.placement); st.place = placer(origin, t); redrawAll(true); showMover(); } }, PLACEMENTS.map(p => h('option', { value: p.id }, p.label)));
  const opSel = h('select', { 'aria-label': 'Operator', onchange: () => { st.operator = opSel.value; applyOperator(); } }, h('option', { value: '' }, 'All operators'), [...new Set(views.map.cells.map(c => c.operator))].sort().map(o => h('option', { value: o }, o)));
  const moverSel = h('select', { 'aria-label': 'Subject to follow', onchange: () => { st.mover = moverSel.value; st.time = 1; slider.value = 1000; followOperator(); redrawMover(true); showMover(); } }, movers.map(m => h('option', { value: m.number }, `${lbl(m.label, 'subject')} · ${m.operator}${m.changes.length ? ' · moved' : ''}${m.short_deviations.length ? ' · away a week' : ''}${m.speed_alerts.length ? ' · speed alert' : ''}`)));
  const pairSel = h('select', { 'aria-label': 'Pair of subjects', onchange: () => { st.pair = +pairSel.value; redrawColocation(); showMover(); } }, pairs.map((p, i) => h('option', { value: i }, `${lbl(p.labels[0], 'subject')} and ${lbl(p.labels[1], 'subject')}${p.meetings.length ? ' · possible meeting' : ''}`)));
  const layerBox = Object.entries({ cells: 'Cell sites', colocation: 'Co-location', trail: 'Movement trail', dwell: 'Time spent', alerts: 'Speed alerts', anchors: 'Sleep and work places' }).map(([k, text]) => h('label', { class: 'tog' }, h('input', { type: 'checkbox', checked: true, onchange: e => { st.layers[k] = e.target.checked; applyLayerVisibility(); } }), ' ', text));
  const dim = h('input', { type: 'range', min: 0, max: 100, value: dark ? 45 : 85, 'aria-label': 'Basemap strength', oninput: () => applyBasemap() });
  const toolbar = h('div', { class: 'toolbar' },
    h('div', { class: 'group' }, h('span', { class: 'gl' }, 'Place'), placeSel, status),
    h('div', { class: 'group' }, h('span', { class: 'gl' }, 'Cell sites'), opSel),
    h('div', { class: 'group' }, h('span', { class: 'gl' }, 'Follow'), moverSel),
    h('div', { class: 'group' }, h('span', { class: 'gl' }, 'Pair'), pairSel),
    h('div', { class: 'group' }, h('span', { class: 'gl' }, 'Layers'), layerBox),
    h('div', { class: 'group' }, h('span', { class: 'gl' }, 'Basemap'), dim),
    h('div', { class: 'group right' }, h('button', { onclick: () => fit() }, 'Fit'), h('button', { onclick: () => { map.once('render', () => map.getCanvas().toBlob(b => download('map.png', b))); map.triggerRepaint(); } }, 'Save PNG')));
  const slider = h('input', { type: 'range', min: 0, max: 1000, value: 1000, 'aria-label': 'Time along the movement', oninput: () => { st.time = slider.value / 1000; redrawTrail(); } });
  const clock = h('output', { class: 'clock' }); const playBtn = h('button', { onclick: () => play() }, '▶ Play');
  const speedSel = h('select', { 'aria-label': 'Playback speed' }, h('option', { value: 1 }, '1×'), h('option', { value: 3 }, '3×'), h('option', { value: 8 }, '8×'));
  const timeBar = h('div', { class: 'timebar' }, playBtn, slider, clock, speedSel);
  const panel = h('aside', { class: 'panel', id: 'map-panel' });
  const banner = h('div', { class: 'banner', hidden: true });
  root.append(toolbar, h('div', { class: 'split' }, h('div', { class: 'canvasbox' }, mapDiv, banner, timeBar), panel));

  // ---- map and basemap ----------------------------------------------------------------------------------------------------------------
  const sources = {}; const layers = [{ id: 'bg', type: 'background', paint: { 'background-color': dark ? '#141a22' : '#eef2f6' } }];
  if (config.tiles) { sources.base = config.tiles.tilejson ? { type: 'raster', url: config.tiles.tilejson, tileSize: config.tiles.tileSize, attribution: config.tiles.attribution } : { type: 'raster', tiles: [config.tiles.url], tileSize: config.tiles.tileSize, minzoom: config.tiles.minzoom, maxzoom: config.tiles.maxzoom, attribution: config.tiles.attribution }; layers.push({ id: 'base', type: 'raster', source: 'base', paint: { 'raster-opacity': 0.85, 'raster-saturation': -0.55, 'raster-brightness-max': dark ? 0.55 : 1 } }); }
  const map = new maplibregl.Map({ container: mapDiv, style: { version: 8, sources, layers }, center: [origin.lon, origin.lat], zoom: 11, attributionControl: false, dragRotate: false, maxZoom: 19 });
  map.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'top-right'); map.addControl(new maplibregl.ScaleControl({ unit: 'metric' }), 'bottom-left'); map.addControl(new maplibregl.AttributionControl({ compact: true }), 'bottom-right');
  window.__map = map; let tileErrors = 0;
  map.on('error', e => { const msg = (e.error && e.error.message) || ''; if (config.tiles && e.sourceId === 'base') { tileErrors++; if (tileErrors === 3) status.replaceChildren(badge('basemap not reachable', 'warn', msg)); } });
  function applyBasemap() { if (!map.getLayer('base')) return; const v = dim.value / 100; map.setPaintProperty('base', 'raster-opacity', Math.max(0.05, v)); map.setPaintProperty('base', 'raster-brightness-max', dark ? 0.35 + 0.65 * v : 1); }
  status.replaceChildren(config.tiles ? badge('offline basemap', 'good', config.tiles.tilejson ? 'Tiles come through the organisation tile cache' : 'Tiles come from the tile server named in the configuration') : badge('no basemap configured: neutral grid', 'warn'));

  // ---- data for the layers (all coordinates go through st.place) ------------------------------------------------------------------------
  const pt = (lat, lon) => st.place(lat, lon);
  const cellFeatures = () => fc(views.map.cells.map(c => { const [lo, la] = pt(c.lat, c.lon); return { type: 'Feature', properties: { key: c.key, op: c.operator }, geometry: { type: 'Polygon', coordinates: [circle(lo, la, c.radius_m)] } }; }));
  const centreFeatures = () => fc(views.map.cells.map(c => ({ type: 'Feature', properties: { key: c.key, op: c.operator }, geometry: { type: 'Point', coordinates: pt(c.lat, c.lon) } })));
  const mover = () => movers.find(m => m.number === st.mover);
  const colo = () => pairs[st.pair];
  function dwellFeatures() { const m = mover(); if (!m) return fc([]); return fc(Object.entries(m.visits).filter(([k]) => cells.has(k)).map(([k, n]) => { const c = cells.get(k); return { type: 'Feature', properties: { n, key: k }, geometry: { type: 'Point', coordinates: pt(c.lat, c.lon) } }; })); }
  function trailData() {
    const m = mover(); if (!m || !m.points.length) return { line: fc([]), dots: fc([]), last: null, t: 0 };
    const t0 = epoch(m.points[0][0]), t1 = epoch(m.points[m.points.length - 1][0]); const t = t0 + (t1 - t0) * st.time;
    const upto = m.points.filter(p => epoch(p[0]) <= t); const hops = new Map(); let prev = null, lastHop = null;
    for (const p of upto) { if (prev && p[3] !== prev[3]) { const key = [prev[3], p[3]].sort().join('|'); hops.set(key, (hops.get(key) || 0) + 1); lastHop = [prev, p]; } prev = p; }
    const feats = [...hops].filter(([k]) => k.split('|').every(c => cells.has(c))).map(([k, n]) => { const [a, b] = k.split('|').map(c => cells.get(c)); return { type: 'Feature', properties: { n, latest: 0 }, geometry: { type: 'LineString', coordinates: [pt(a.lat, a.lon), pt(b.lat, b.lon)] } }; });
    if (lastHop) feats.push({ type: 'Feature', properties: { n: 6, latest: 1 }, geometry: { type: 'LineString', coordinates: [pt(lastHop[0][1], lastHop[0][2]), pt(lastHop[1][1], lastHop[1][2])] } });
    return { line: fc(feats), dots: fc([]), last: upto.length ? { at: upto[upto.length - 1] } : null, t };
  }
  const alertFeatures = () => { const m = mover(); return fc((m ? m.speed_alerts : []).filter(a => cells.has(a.from_cell) && cells.has(a.to_cell)).map(a => ({ type: 'Feature', properties: { kmh: a.speed_kmh, gap: a.gap_s, when: a.from_time, km: a.distance_km }, geometry: { type: 'LineString', coordinates: [pt(cells.get(a.from_cell).lat, cells.get(a.from_cell).lon), pt(cells.get(a.to_cell).lat, cells.get(a.to_cell).lon)] } }))); };
  const coloFeatures = () => { const counts = new Map(); (colo() ? colo().windows : []).forEach(w => counts.set(w.cell_key, (counts.get(w.cell_key) || 0) + 1)); return fc([...counts].filter(([k]) => cells.has(k)).map(([k, n]) => { const c = cells.get(k); const [lo, la] = pt(c.lat, c.lon); return { type: 'Feature', properties: { key: k, n }, geometry: { type: 'Polygon', coordinates: [circle(lo, la, c.radius_m * 0.8)] } }; })); };

  map.on('load', () => {
    map.addSource('cells', { type: 'geojson', data: cellFeatures() }); map.addSource('centres', { type: 'geojson', data: centreFeatures() }); map.addSource('colo', { type: 'geojson', data: coloFeatures() });
    map.addSource('dwell', { type: 'geojson', data: dwellFeatures() }); map.addSource('trail', { type: 'geojson', data: fc([]) }); map.addSource('dots', { type: 'geojson', data: fc([]) }); map.addSource('alerts', { type: 'geojson', data: alertFeatures() });
    const opColour = ['match', ['get', 'op'], ...Object.entries(OPERATOR_COLOURS).flat(), '#888'];
    map.addLayer({ id: 'cell-fill', type: 'fill', source: 'cells', paint: { 'fill-color': opColour, 'fill-opacity': 0.12 } });
    map.addLayer({ id: 'cell-line', type: 'line', source: 'cells', paint: { 'line-color': opColour, 'line-width': 1.2, 'line-dasharray': [3, 2], 'line-opacity': 0.8 } });
    map.addLayer({ id: 'colo-fill', type: 'fill', source: 'colo', paint: { 'fill-color': '#d97706', 'fill-opacity': 0.35 } });
    map.addLayer({ id: 'colo-line', type: 'line', source: 'colo', paint: { 'line-color': '#b45309', 'line-width': 2.5 } });
    map.addLayer({ id: 'dwell', type: 'circle', source: 'dwell', paint: { 'circle-radius': ['interpolate', ['linear'], ['sqrt', ['get', 'n']], 0, 3, 5, 10, 15, 24], 'circle-color': '#0f766e', 'circle-opacity': 0.45, 'circle-stroke-color': '#0f766e', 'circle-stroke-width': 1.5 } });
    map.addLayer({ id: 'centres', type: 'circle', source: 'centres', paint: { 'circle-radius': 2.2, 'circle-color': dark ? '#e2e8f0' : '#334155' } });
    map.addLayer({ id: 'trail', type: 'line', source: 'trail', layout: { 'line-cap': 'round', 'line-join': 'round' }, paint: { 'line-color': ['case', ['==', ['get', 'latest'], 1], '#10b981', '#047857'], 'line-width': ['interpolate', ['linear'], ['get', 'n'], 1, 1.5, 20, 4, 200, 8], 'line-opacity': ['case', ['==', ['get', 'latest'], 1], 1, 0.7] } });
    map.addLayer({ id: 'dots', type: 'circle', source: 'dots', paint: { 'circle-radius': 4, 'circle-color': '#047857', 'circle-opacity': ['*', 0.9, ['get', 'age']], 'circle-stroke-color': '#fff', 'circle-stroke-width': 0.6 } });
    map.addLayer({ id: 'alerts', type: 'line', source: 'alerts', paint: { 'line-color': '#dc2626', 'line-width': 4 } });
    map.on('click', 'cell-fill', ev => { const k = ev.features[0].properties.key; const c = cells.get(k); const m = mover(); const n = m && m.visits[k]; popup(ev.lngLat, `<strong>Cell site ${esc(siteName(k))}</strong> <span style="color:#667">(…${esc(shortCell(k))})</span><br>${esc(c.operator)}<br>Drawn with an assumed radius of ${c.radius_m} m: a site, not the position of a person.${n ? `<br>${esc(lbl(m.label, 'subject'))} used it in ${plural(n, 'event')}.` : ''}`); });
    map.on('click', 'alerts', ev => { const p = ev.features[0].properties; popup(ev.lngLat, `<strong>Speed alert</strong><br>${p.km} km in ${p.gap} s = ${p.kmh} km/h<br>${esc(fmtDT(p.when))}<br><em>Consecutive events at two cell sites; a cell site is not a position.</em>`); });
    for (const l of ['cell-fill', 'alerts']) { map.on('mouseenter', l, () => { map.getCanvas().style.cursor = 'pointer'; }); map.on('mouseleave', l, () => { map.getCanvas().style.cursor = ''; }); }
    applyBasemap(); followOperator(); redrawAll(true); showMover();
  });
  function popup(lngLat, html) { st.popup?.remove(); st.popup = new maplibregl.Popup({ maxWidth: '280px' }).setLngLat(lngLat).setHTML(html).addTo(map); }

  // ---- drawing ------------------------------------------------------------------------------------------------------------------------
  function clearMarkers() { st.markers.forEach(m => m.remove()); st.markers = []; }
  function pin(lonlat, text, cls, anchor = 'bottom', offset = [0, -12]) { const el = h('div', { class: 'pin ' + cls }, h('span', {}, text)); const m = new maplibregl.Marker({ element: el, anchor, offset }).setLngLat(lonlat).addTo(map); st.markers.push(m); return m; }
  function redrawAll(refit) { if (!map.getSource('cells')) return; map.getSource('cells').setData(cellFeatures()); map.getSource('centres').setData(centreFeatures()); redrawMover(false); redrawColocation(); applyOperator(); applyLayerVisibility(); if (refit) fit(); }
  function redrawMover(refit) { if (!map.getSource('dwell')) return; map.getSource('dwell').setData(dwellFeatures()); map.getSource('alerts').setData(alertFeatures()); redrawTrail(); drawAnchors(); applyLayerVisibility(); if (refit) fit(); }
  function redrawTrail() {
    const d = trailData(); map.getSource('trail').setData(d.line); map.getSource('dots').setData(d.dots);
    clock.textContent = d.last ? iso(d.t) : '—'; if (st.cur) { st.cur.remove(); st.cur = null; }
    if (d.last && st.layers.trail) { const p = d.last.at; st.cur = new maplibregl.Marker({ element: h('div', { class: 'now' }) }).setLngLat(pt(p[1], p[2])).addTo(map); }
  }
  function redrawColocation() { if (!map.getSource('colo')) return; map.getSource('colo').setData(coloFeatures()); applyLayerVisibility(); }
  function followOperator() { const m = mover(); st.operator = m ? m.operator : ''; opSel.value = st.operator; applyOperator(); }
  function applyOperator() { const f = st.operator ? ['==', ['get', 'op'], st.operator] : null; for (const id of ['cell-fill', 'cell-line', 'centres']) if (map.getLayer(id)) map.setFilter(id, f); }
  function applyLayerVisibility() {
    const vis = (id, on) => { if (map.getLayer(id)) map.setLayoutProperty(id, 'visibility', on ? 'visible' : 'none'); };
    vis('cell-fill', st.layers.cells); vis('cell-line', st.layers.cells); vis('centres', st.layers.cells); vis('colo-fill', st.layers.colocation); vis('colo-line', st.layers.colocation);
    vis('trail', st.layers.trail); vis('dots', st.layers.trail); vis('dwell', st.layers.dwell); vis('alerts', st.layers.alerts);
    st.markers.forEach(m => { m.getElement().style.display = st.layers.anchors ? '' : 'none'; }); if (st.cur) st.cur.getElement().style.display = st.layers.trail ? '' : 'none';
    timeBar.hidden = !(st.layers.trail && mover());
  }
  function drawAnchors() {
    clearMarkers(); const m = mover(); if (!m) return;
    const slotText = { night: 'Sleeps here', day: 'Works here' };
    for (const slot of ['night', 'day']) (m.anchors[slot] || []).forEach((a, i, all) => { const c = cells.get(a.cell_key); if (!c) return; pin(pt(c.lat, c.lon), `${slotText[slot]}${all.length > 1 ? ` (${i === 0 ? 'before' : 'after'})` : ''} · weeks ${a.first_week + 1}–${a.last_week + 1}`, slot + (i ? ' later' : ''), slot === 'night' ? 'bottom' : 'top', slot === 'night' ? [0, -12] : [0, 12]); });
    (m.short_deviations || []).forEach(d => { const c = cells.get(d.cell_key); if (c) pin(pt(c.lat, c.lon), `Away for a week (week ${d.first_week + 1})`, 'away', 'left', [18, 0]); });
    (m.speed_alerts || []).forEach(a => { if (!cells.has(a.from_cell) || !cells.has(a.to_cell)) return; const f = cells.get(a.from_cell), t = cells.get(a.to_cell); pin(pt((f.lat + t.lat) / 2, (f.lon + t.lon) / 2), `${a.speed_kmh} km/h`, 'alert', 'center', [0, 0]); });
  }
  function fit() {
    const b = new maplibregl.LngLatBounds(); const m = mover();
    const keys = m && m.points.length ? Object.keys(m.visits) : [...cells.keys()];
    (keys.length ? keys : [...cells.keys()]).forEach(k => { const c = cells.get(k); if (c) b.extend(pt(c.lat, c.lon)); });
    if (!b.isEmpty()) map.fitBounds(b, { padding: 80, animate: false, maxZoom: 15 });
  }

  // ---- panel --------------------------------------------------------------------------------------------------------------------------
  function setPanel(...kids) { panel.replaceChildren(...kids.filter(Boolean)); }
  function showMover() {
    const m = mover(); const sections = [];
    banner.hidden = st.placement === 'ocean'; banner.textContent = st.placement === 'ocean' ? '' : 'SYNTHETIC CASE: the cell sites are fictional and only laid over this place for demonstration';
    if (!m) { setPanel(h('h2', {}, 'No movement to show'), h('p', {}, 'No subject in this case has a change of sleeping place, a week away or a speed alert.'), colocationSection()); return; }
    const ch = m.changes[0]; const nights = m.anchors.night || [];
    sections.push(h('h2', {}, `Movement of ${lbl(m.label, 'subject')}`, ' ', badge(m.operator, 'op')),
      h('div', { class: 'chips' }, chip(m.points.length, 'events with a position'), chip(Object.keys(m.visits).length, 'cell sites used'), chip(m.changes.length, plural(m.changes.length, 'move', 'moves')), chip(m.speed_alerts.length, plural(m.speed_alerts.length, 'speed alert')), m.short_deviations.length ? chip(m.short_deviations.length, plural(m.short_deviations.length, 'week away', 'weeks away')) : null),
      help('How to read the map', 'Dashed circles are cell sites; each is drawn with an assumed radius, because a site tells you roughly where a phone was, not exactly. The green line follows the subject from one cell site to the next up to the time on the slider; press Play to watch it. Teal discs show how much time was spent at each site.', 'Pins say where the subject sleeps (nights, 22:00 to 07:00) and works (weekdays, 09:00 to 17:00), week by week; two pins mean the place changed for good. A red line is a speed alert: two events so close in time and so far apart that no vehicle would do it, which usually points to a data or cell-handover effect.'));
    if (ch) sections.push(h('div', { class: 'note' }, h('b', {}, 'Sleeping place changed. '), `From site ${siteName(ch.from_cell)} to site ${siteName(ch.to_cell)}, ${ch.distance_km} km apart, from the week starting ${ch.first_week_start}. It held for at least two weeks, so it is a move and not a trip.`));
    else if (nights.length) sections.push(h('p', { class: 'muted small' }, 'Sleeping place did not change for good.'));
    if (m.short_deviations.length) sections.push(h('div', { class: 'note' }, h('b', {}, 'A week away. '), `Slept somewhere else in week ${m.short_deviations[0].first_week + 1}, then came back: a trip, not a move.`));
    if (m.speed_alerts.length) sections.push(h('h3', {}, 'Speed alerts'), table(['When', 'km', 'Seconds', 'km/h'], m.speed_alerts.map(a => ({ cells: [h('button', { class: 'linkish', onclick: () => flyAlert(a) }, fmtDT(a.from_time)), a.distance_km, a.gap_s, a.speed_kmh], sort: [a.from_time, a.distance_km, a.gap_s, a.speed_kmh ?? 1e9] }))), h('p', { class: 'muted small' }, 'Two events at different cell sites, fast. A prompt to look at the rows, not proof of movement.'));
    sections.push(h('h3', {}, 'Where the time was spent'), table(['Cell site', 'Events'], Object.entries(m.visits).sort((a, b) => b[1] - a[1]).slice(0, 8).map(([k, n]) => ({ cells: [h('button', { class: 'linkish', title: shortCell(k), onclick: () => flyCell(k) }, siteName(k)), n], sort: [k, n] }))));
    setPanel(...sections, h('hr'), colocationSection());
  }
  function colocationSection() {
    const p = colo(); if (!p) return null;
    const meet = new Map(p.meetings.map(m => [m.start_local, m]));
    return h('div', { class: 'colo' },
      h('h3', {}, `${lbl(p.labels[0], 'subject')} and ${lbl(p.labels[1], 'subject')} at the same cell site`),
      h('div', { class: 'chips' }, chip(p.windows.length, plural(p.windows.length, 'window')), chip(p.observed, 'pairs of events'), chip(p.expected.toFixed(1), 'expected by chance'), chip(p.lift == null ? 'n/a' : '×' + p.lift, p.lift == null ? 'too little activity to compare' : 'times more than chance', p.lift != null && p.lift > 2 ? 'hot' : '')),
      h('p', { class: 'note bad' }, 'Proximity is not association. Being at the same cell site within 15 minutes is common for people who live or work near each other.'),
      p.windows.length ? table(['Window', 'Pairs', 'Cell', 'Possible meeting'], p.windows.map(w => { const mt = meet.get(w.start_local); return { cells: [h('button', { class: 'linkish', onclick: () => flyCell(w.cell_key) }, fmtDT(w.start_local)), w.pairs, siteName(w.cell_key), mt ? badge(`score ${mt.score}`, 'warn', `A call or message ${mt.gap_minutes} minutes before the window`) : '—'], sort: [w.start_local, w.pairs, w.cell_key, mt ? mt.score : 0] }; })) : h('p', { class: 'muted' }, 'They were never at the same cell site within 15 minutes.'),
      p.meetings.length ? h('p', { class: 'muted small' }, 'A possible meeting is a window preceded within an hour by a call or message between the two. A candidate, not a finding.') : null);
  }
  function flyCell(k) { const c = cells.get(k); if (!c) return; const [lo, la] = pt(c.lat, c.lon); map.flyTo({ center: [lo, la], zoom: Math.max(map.getZoom(), 14), duration: 600 }); popup([lo, la], `<strong>Cell site ${esc(siteName(k))}</strong> <span style="color:#667">(…${esc(shortCell(k))})</span><br>${esc(c.operator)}`); }
  function flyAlert(a) { const f = cells.get(a.from_cell), t = cells.get(a.to_cell); if (!f || !t) return; const b = new maplibregl.LngLatBounds(pt(f.lat, f.lon), pt(f.lat, f.lon)); b.extend(pt(t.lat, t.lon)); map.fitBounds(b, { padding: 120, duration: 600, maxZoom: 15 }); }
  function play() {
    if (st.playing) { clearInterval(st.playing); st.playing = null; playBtn.textContent = '▶ Play'; return; }
    if (st.time >= 0.999) { st.time = 0; slider.value = 0; }
    playBtn.textContent = '❚❚ Pause';
    st.playing = setInterval(() => { st.time = Math.min(1, st.time + 0.004 * +speedSel.value); slider.value = Math.round(st.time * 1000); redrawTrail(); if (st.time >= 1) { clearInterval(st.playing); st.playing = null; playBtn.textContent = '▶ Play'; } }, 50);
  }

  // ---- stress tests (used by the measurements tab) -----------------------------------------------------------------------------------
  const stressPoints = count => { const r = rng(5); const out = new Float32Array(count * 2); for (let i = 0; i < count; i++) { out[2 * i] = origin.lon + (r() - 0.5) * 0.07; out[2 * i + 1] = origin.lat + (r() - 0.5) * 0.07; } return out; };
  const motion = () => { const c = map.getCenter(); return framesPerSecond(i => map.jumpTo({ center: [c.lng + Math.sin(i / 10) * 0.004, c.lat], zoom: 11.5 + Math.sin(i / 25) * 0.4 }), 120); };
  return {
    map, state: st, destroy() { clearInterval(st.playing); clearMarkers(); map.remove(); },
    async stressMapLibre(count = 50000) {
      placeSel.value = 'ocean'; st.placement = 'ocean'; st.place = placer(origin, origin); redrawAll(false); map.jumpTo({ center: [origin.lon, origin.lat], zoom: 11.5 });
      const pts = stressPoints(count); const feats = new Array(count); for (let i = 0; i < count; i++) feats[i] = { type: 'Feature', properties: {}, geometry: { type: 'Point', coordinates: [pts[2 * i], pts[2 * i + 1]] } };
      const run = await timed(() => { if (map.getSource('stress')) map.getSource('stress').setData(fc(feats)); else { map.addSource('stress', { type: 'geojson', data: fc(feats) }); map.addLayer({ id: 'stress', type: 'circle', source: 'stress', paint: { 'circle-radius': 1.5, 'circle-color': '#7c3aed', 'circle-opacity': 0.5 } }); } });
      const m = await motion(); return { points: count, first_render_ms: run.ms, fps: m.fps, worst_frame_ms: m.worst_ms };
    },
    async stressDeck(count = 50000) {
      map.jumpTo({ center: [origin.lon, origin.lat], zoom: 11.5 });
      const layer = new ScatterplotLayer({ id: 'deck-stress', data: { length: count, attributes: { getPosition: { value: stressPoints(count), size: 2 } } }, getRadius: 1.5, radiusUnits: 'pixels', getFillColor: [124, 58, 237, 130] });
      const run = await timed(() => { if (!st.overlay) { st.overlay = new MapboxOverlay({ interleaved: false, layers: [layer] }); map.addControl(st.overlay); } else st.overlay.setProps({ layers: [layer] }); });
      const m = await motion(); return { points: count, first_render_ms: run.ms, fps: m.fps, worst_frame_ms: m.worst_ms };
    },
    clearStress() { if (map.getLayer('stress')) { map.removeLayer('stress'); map.removeSource('stress'); } if (st.overlay) { map.removeControl(st.overlay); st.overlay = null; } },
  };
}
