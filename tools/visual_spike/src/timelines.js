// Lanes and series (analyses A5, A7, A8, A10, A11, A13): each card answers one question in plain words, shows its key numbers, draws the chart,
// says how to read it, and offers the data as a table and the picture as PNG or SVG. Apache ECharts, tree-shaken.
import * as echarts from 'echarts/core';
import { CustomChart, ScatterChart, LineChart, HeatmapChart, LinesChart, BarChart } from 'echarts/charts';
import { GridComponent, DataZoomComponent, TooltipComponent, MarkAreaComponent, MarkLineComponent, VisualMapComponent, AriaComponent, LegendComponent, TitleComponent } from 'echarts/components';
import { CanvasRenderer, SVGRenderer } from 'echarts/renderers';
import { label as lbl, state } from './common.js';
import { h, badge, chip, help, table, download, fmtDT, plural, colourOf, siteName, carrierName } from './ui.js';
import { carrierOfImsi } from './carriers.js';

echarts.use([CustomChart, ScatterChart, LineChart, HeatmapChart, LinesChart, BarChart, GridComponent, DataZoomComponent, TooltipComponent, MarkAreaComponent, MarkLineComponent, VisualMapComponent, AriaComponent, LegendComponent, TitleComponent, CanvasRenderer, SVGRenderer]);

const T = s => Date.parse(s.replace(' ', 'T') + 'Z');
const D = ms => new Date(ms).toISOString().replace('T', ' ').slice(0, 16);
const css = n => getComputedStyle(document.documentElement).getPropertyValue(n).trim();
const ink = () => css('--ink'), muted = () => css('--muted'), line = () => css('--line'), panelBg = () => css('--panel');

// ECharts numbers its CSS classes and clip ids with counters that depend on how many charts were made before; renumber them in order of
// appearance so that the same view gives the same file (specification of the interface programme, rule V8).
export function normaliseSvg(svg) {
  const seen = new Map();
  return svg.replace(/zr\d+-/g, 'zr-').replace(/zr-cls-\d+/g, m => { if (!seen.has(m)) seen.set(m, 'zr-cls-' + seen.size); return seen.get(m); });
}
export function svgOf(option, width = 960, height = 360) {
  const c = echarts.init(null, null, { renderer: 'svg', ssr: true, width, height }); c.setOption({ ...option, animation: false }); const svg = normaliseSvg(c.renderToSVGString()); c.dispose(); return svg;
}

// ECharts does not mirror a chart for right-to-left reading by itself: time runs from the right, the lane names stand on the right, and the grid is swapped.
function mirror(option) {
  const list = v => (Array.isArray(v) ? v : v ? [v] : []);
  list(option.xAxis).forEach(a => { a.inverse = true; });
  list(option.yAxis).forEach(a => { a.position = 'right'; });
  list(option.grid).forEach(gr => { const left = gr.left; gr.left = gr.right; gr.right = left; });
  return option;
}

const charts = [];
function makeCard({ analysis, title, question, chips = [], note, noteKind = 'note', chart, height = 340, helpTitle = 'How to read this', helpText = [], tableFn, side }) {
  const box = h('div', { class: 'chart', style: `height:${height}px`, role: 'img' });
  const aria = h('figcaption', { class: 'aria' });
  const tableBox = h('div', { hidden: true });
  const body = h('div', { class: side ? 'cardbody withside' : 'cardbody' }, h('div', {}, box), side || null);
  let instance, option;
  const card = h('section', { class: 'card' },
    h('header', {}, h('h3', {}, title, ' ', badge(analysis, 'info')), h('p', { class: 'q' }, question)),
    h('div', { class: 'chips' }, chips), note ? h('p', { class: noteKind }, note) : null, body, aria,
    h('div', { class: 'row' },
      h('button', { onclick: () => download(`${analysis}-${title.replace(/\W+/g, '-').toLowerCase()}.png`, instance.getDataURL({ pixelRatio: 2, backgroundColor: panelBg() })) }, 'Save PNG'),
      h('button', { onclick: () => download(`${analysis}-${title.replace(/\W+/g, '-').toLowerCase()}.svg`, svgOf(option, box.clientWidth, height), 'image/svg+xml') }, 'Save SVG'),
      tableFn ? h('button', { onclick: ev => { tableBox.hidden = !tableBox.hidden; ev.target.textContent = tableBox.hidden ? 'View as table' : 'Hide table'; if (!tableBox.hidden && !tableBox.firstChild) tableBox.append(tableFn()); } }, 'View as table') : null),
    tableBox, help(helpTitle, ...helpText));
  const paint = () => {
    instance?.dispose(); instance = echarts.init(box, null, { renderer: 'canvas' }); option = { useUTC: true, animation: false, aria: { enabled: true }, ...(state.rtl ? mirror(chart(box)) : chart(box)) }; instance.setOption(option);
    aria.textContent = box.getAttribute('aria-label') || ''; charts.push(instance); card.chart = instance;
  };
  // a chart is only created when its box has a size: in a hidden view it waits until the view is shown
  card.draw = () => { if (box.clientWidth) { paint(); return; } const ro = new ResizeObserver(() => { if (box.clientWidth) { ro.disconnect(); paint(); } }); ro.observe(box); };
  return card;
}

// ---- A8 device and SIM history ------------------------------------------------------------------------------------------------------------------
function deviceCard(views) {
  const lanes = [...views.lanes.devices].sort((a, b) => b.changes.length - a.changes.length || a.label.localeCompare(b.label));
  const changed = lanes.filter(l => l.changes.length); const shared = views.lanes.shared_devices;
  const names = lanes.map(l => `${lbl(l.label, 'subject')}  ${carrierName(l.operator)}`);
  const seenIn = new Map(); lanes.forEach(l => l.runs.forEach(r => { if (!seenIn.has(r.imei)) seenIn.set(r.imei, new Set()); seenIn.get(r.imei).add(l.label); }));
  const PALETTE = ['#e69f00', '#56b4e9', '#009e73', '#d55e00', '#cc79a7', '#0072b2', '#b8860b', '#9467bd', '#8c564b', '#17becf'];
  const interesting = [...new Set(lanes.flatMap(l => (l.runs.length > 1 ? l.runs.map(r => r.imei) : [])).concat([...seenIn].filter(([, s]) => s.size > 1).map(([k]) => k)))];
  const handsetColour = imei => (interesting.includes(imei) ? PALETTE[interesting.indexOf(imei) % PALETTE.length] : '#64748b');
  return makeCard({
    analysis: 'A8', title: 'Device and SIM history', question: 'Did any number change phone or SIM, and does one phone appear under two numbers?',
    chips: [chip(changed.length, plural(changed.length, 'number') + ' changed SIM or handset', changed.length ? 'hot' : ''), chip(shared.length, plural(shared.length, 'handset') + ' under two numbers'), chip(lanes.length, 'numbers')],
    note: shared.length ? 'A handset under two numbers at the same time is usually a dual-SIM phone, not a swap. One passed on from one number to the next is a handover.' : null,
    helpText: ['One row per number. A bar is a stretch of time in which the SIM and the handset stayed the same; its colour is the handset. When the colour changes, so did the handset; the triangle marks the change and says what changed. A dashed line joins two numbers that used the same handset. Colours are only used for handsets that matter (one that changed, or one used under two numbers); a grey-blue bar is a handset that never changed.', 'Some operators write only the handset in their files and not the SIM, so a SIM change there cannot be seen.'],
    tableFn: () => table(['Number', 'Carrier', 'SIM', 'Handset', 'From', 'To'], lanes.flatMap(l => l.runs.map(r => ({ cells: [lbl(l.label, 'subject'), carrierName(l.operator), r.imsi && r.imsi !== 'null' ? (carrierOfImsi(r.imsi) ? carrierName(carrierOfImsi(r.imsi).id) + ' SIM · ' : '') + '…' + r.imsi : 'not in export', '…' + r.imei, fmtDT(r.first), fmtDT(r.last)], sort: [l.label, carrierName(l.operator), r.imsi, r.imei, r.first, r.last] })))),
    height: Math.max(300, 44 + lanes.length * 30),
    chart: () => {
      const runs = lanes.flatMap((l, i) => l.runs.map(r => ({ value: [i, T(r.first), T(r.last)], imsi: r.imsi, imei: r.imei, number: l.label })));
      const marks = lanes.flatMap((l, i) => l.changes.map(c => ({ value: [T(c.first_seen_new), i], kind: c.kind, from: c.last_seen_old, to: c.first_seen_new })));
      const index = new Map(lanes.map((l, i) => [l.label, i]));
      const links = shared.flatMap(d => { const a = d.numbers.map(n => index.get(n.label)).filter(x => x != null); if (a.length < 2) return []; const at = Math.max(...d.numbers.map(n => T(n.first))); return [{ coords: [[at, a[0]], [at, a[1]]], relation: d.relation, imei: d.imei }]; });
      return {
        aria: { label: { description: `Device and SIM history for ${lanes.length} numbers. ${changed.length} changed SIM or handset; ${shared.length} handsets appear under two numbers.` } },
        tooltip: { confine: true, formatter: p => p.seriesType === 'custom' ? `<b>${lbl(p.data.number, 'subject')}</b><br>${p.data.imsi && p.data.imsi !== 'null' ? 'SIM …' + p.data.imsi : 'SIM not written in this export'} · handset …${p.data.imei}<br>${D(p.data.value[1]).slice(0, 10)} to ${D(p.data.value[2]).slice(0, 10)}` : p.seriesType === 'scatter' ? `<b>${p.data.kind === 'imei' ? 'Handset changed' : p.data.kind === 'imsi' ? 'SIM changed' : 'SIM and handset changed'}</b><br>last seen before: ${D(T(p.data.from))}<br>first seen after: ${D(T(p.data.to))}` : `Same handset …${p.data.imei}<br>${p.data.relation === 'concurrent' ? 'at the same time (dual SIM, or shared)' : 'one after the other (passed on)'}` },
        grid: { left: 150, right: 24, top: 18, bottom: 56 }, dataZoom: [{ type: 'inside', filterMode: 'none' }, { type: 'slider', height: 16, bottom: 6, filterMode: 'none' }],
        xAxis: { type: 'time', axisLabel: { color: muted() }, splitLine: { show: true, lineStyle: { color: line() } } },
        yAxis: { type: 'category', inverse: true, data: names, axisLabel: { color: ink(), fontSize: 12 }, axisTick: { show: false } },
        series: [
          { type: 'custom', data: runs, encode: { x: [1, 2], y: 0 }, z: 2, renderItem: (params, api) => { const y = api.coord([0, api.value(0)])[1]; const p1 = api.coord([api.value(1), 0])[0], p2 = api.coord([api.value(2), 0])[0]; const a = Math.min(p1, p2), b = Math.max(p1, p2); const hh = api.size([0, 1])[1] * 0.58; const d = runs[params.dataIndex]; const w = Math.max(3, b - a); return { type: 'group', children: [{ type: 'rect', shape: { x: a, y: y - hh / 2, width: w, height: hh, r: 3 }, style: { fill: handsetColour(d.imei), stroke: 'rgba(0,0,0,.25)' } }, w > 120 ? { type: 'text', style: { x: a + 6, y, text: d.imsi && d.imsi !== 'null' ? `SIM …${d.imsi} · handset …${d.imei}` : `handset …${d.imei}`, fill: '#fff', fontSize: 11, textVerticalAlign: 'middle', textShadowColor: 'rgba(0,0,0,.5)', textShadowBlur: 2 } } : null].filter(Boolean) }; } },
          { type: 'scatter', data: marks, symbol: 'triangle', symbolSize: 16, z: 5, itemStyle: { color: '#dc2626', borderColor: '#fff', borderWidth: 1.5 }, label: { show: true, position: 'top', color: ink(), fontSize: 11, formatter: p => p.data.kind === 'imei' ? 'handset' : p.data.kind === 'imsi' ? 'SIM' : 'SIM + handset' } },
          { type: 'lines', coordinateSystem: 'cartesian2d', data: links, lineStyle: { color: '#7c3aed', width: 2.5, type: 'dashed' }, symbol: ['circle', 'circle'], symbolSize: 7, z: 4 },
        ],
      };
    },
  });
}

// ---- A5 and A7 co-location and possible meetings ------------------------------------------------------------------------------------------------
function pairCard(views) {
  const pairs = views.pairs; let which = 0; const wrapper = h('div', { class: 'pairwrap' });
  if (!pairs.length) return h('section', { class: 'card' }, h('h3', {}, 'Co-location ', badge('A5', 'info')), h('p', {}, 'No pair of subjects was at the same cell site within the window.'));
  const select = h('select', { 'aria-label': 'Pair of subjects', onchange: () => { which = +select.value; build(); } }, pairs.map((p, i) => h('option', { value: i }, `${lbl(p.labels[0], 'subject')} and ${lbl(p.labels[1], 'subject')} · ${plural(p.windows.length, 'window')}${p.meetings.length ? ` · ${plural(p.meetings.length, 'possible meeting')}` : ''}`)));
  let current;
  function build() {
    const p = pairs[which]; const meet = new Map(p.meetings.map(m => [m.start_local, m]));
    const first = p.windows.length ? T(p.windows[0].start_local) : null; let zoomIdx = 0;
    const zoomTo = i => { const w = p.windows[i]; if (!w) return; zoomIdx = i; const t = T(w.start_local); current.chart.dispatchAction({ type: 'dataZoom', startValue: t - 4 * 3600000, endValue: t + 4 * 3600000 }); };
    const winList = h('div', { class: 'wins' }, h('h4', {}, 'Windows'), ...p.windows.map((w, i) => { const mt = meet.get(w.start_local); return h('button', { class: 'win', onclick: () => zoomTo(i) }, h('b', {}, fmtDT(w.start_local)), h('span', {}, `${plural(w.pairs, 'pair')} · ${siteName(w.cell_key)}`), mt ? badge(`possible meeting ${mt.score}`, 'warn', `a call or message ${mt.gap_minutes} minutes before`) : null); }));
    const card = makeCard({
      analysis: 'A5 · A7', title: 'Co-location and possible meetings', question: 'When were these two numbers at the same cell site, and did they speak just before?',
      chips: [chip(p.windows.length, plural(p.windows.length, 'window')), chip(p.observed, 'pairs of events at one site'), chip(p.expected.toFixed(1), 'expected by chance'), chip(p.lift == null ? 'n/a' : '×' + p.lift, p.lift == null ? 'too little activity to compare with chance' : 'times more than chance', p.lift != null && p.lift > 2 ? 'hot' : ''), chip(p.meetings.length, plural(p.meetings.length, 'possible meeting'))],
      note: 'Proximity is not association. A possible meeting is a candidate, not a finding: two numbers at one cell site within 15 minutes, with a call or message between them in the hour before.', noteKind: 'note bad',
      helpText: ['The top row is the first number, the middle row the second; each tick is one call or message. A shaded band is a window in which they were at the same cell site within 15 minutes. A diamond on the bottom row is a direct call or message between the two.', '"Expected by chance" says how many such pairs two numbers with these habits would produce if they moved independently; "times more than chance" compares the observed number with it. It is left blank when there is too little activity.', 'Drag the slider below the chart or use the buttons to move between windows.'],
      tableFn: () => table(['Window start', 'End', 'Pairs', 'Cell', 'Possible meeting'], p.windows.map(w => { const mt = meet.get(w.start_local); return { cells: [fmtDT(w.start_local), fmtDT(w.end_local), w.pairs, siteName(w.cell_key), mt ? `score ${mt.score}, contact ${mt.gap_minutes} min before` : '—'], sort: [w.start_local, w.end_local, w.pairs, w.cell_key, mt ? mt.score : 0] }; })),
      height: 300, side: winList,
      chart: () => {
        const names = [lbl(p.labels[0], 'subject'), lbl(p.labels[1], 'subject'), 'Direct contact'];
        const ev = (k, lane) => p.events[k].map(([t, cell]) => ({ value: [T(t), lane], cell }));
        const bands = p.windows.map(w => [{ xAxis: T(w.start_local) - 240000, itemStyle: { color: 'rgba(217,119,6,.28)' } }, { xAxis: T(w.end_local) + 240000 }]);
        return {
          aria: { label: { description: `Two numbers' events with ${p.windows.length} co-location windows and ${p.meetings.length} possible meetings.` } },
          tooltip: { confine: true, formatter: q => q.seriesName === 'contact' ? `Direct call or message<br>${D(q.data.value[0])}` : `<b>${names[q.data.value[1]]}</b><br>${D(q.data.value[0])}<br>${siteName(q.data.cell)}` },
          grid: { left: 104, right: 24, top: 18, bottom: 56 },
          dataZoom: [{ type: 'inside', startValue: first ? first - 4 * 3600000 : undefined, endValue: first ? first + 4 * 3600000 : undefined, filterMode: 'none' }, { type: 'slider', height: 16, bottom: 6, startValue: first ? first - 4 * 3600000 : undefined, endValue: first ? first + 4 * 3600000 : undefined, filterMode: 'none' }],
          xAxis: { type: 'time', axisLabel: { color: muted() }, splitLine: { show: true, lineStyle: { color: line() } } },
          yAxis: { type: 'category', inverse: true, data: names, axisLabel: { color: ink(), fontSize: 12 }, axisTick: { show: false }, splitLine: { show: true, lineStyle: { color: line() } } },
          series: [
            { name: 'x', type: 'scatter', data: ev(p.x, 0), symbol: 'rect', symbolSize: [3, 22], itemStyle: { color: '#0072B2' }, markArea: { silent: true, data: bands, label: { show: false } } },
            { name: 'y', type: 'scatter', data: ev(p.y, 1), symbol: 'rect', symbolSize: [3, 22], itemStyle: { color: '#009E73' } },
            { name: 'contact', type: 'scatter', data: p.contacts.map(t => ({ value: [T(t), 2] })), symbol: 'diamond', symbolSize: 13, itemStyle: { color: '#7c3aed', borderColor: '#fff', borderWidth: 1 } },
          ],
        };
      },
    });
    current = card; wrapper.replaceChildren(h('div', { class: 'row' }, h('label', {}, 'Pair ', select), h('button', { onclick: () => zoomTo((zoomIdx + 1) % Math.max(1, p.windows.length)) }, 'Next window')), card); if (wrapper.isConnected) card.draw();
  }
  build(); wrapper.draw = () => current.draw(); wrapper.cardRef = () => current;
  return wrapper;
}

// ---- A10 where do they sleep and work, week by week ----------------------------------------------------------------------------------------------
function anchorsCard(views) {
  const movers = views.map.movers.filter(m => (m.anchors.night || []).length || m.short_deviations.length); const weeks = Math.ceil(views.case.days / 7);
  if (!movers.length) return null;
  const lanes = movers.flatMap(m => [{ m, slot: 'night', name: `${lbl(m.label, 'subject')} sleeps` }, { m, slot: 'day', name: `${lbl(m.label, 'subject')} works` }]);
  const moved = movers.filter(m => m.changes.length), away = movers.filter(m => m.short_deviations.length);
  return makeCard({
    analysis: 'A10', title: 'Where do they sleep and work, week by week?', question: 'Did anyone change the place they sleep or work, and for how long?',
    chips: [chip(moved.length, plural(moved.length, 'subject') + ' moved for good', moved.length ? 'hot' : ''), chip(away.length, plural(away.length, 'subject') + ' away for a week'), chip(movers.length, 'subjects with something to see')],
    note: 'Each block is the cell site where the subject mostly was in that week (nights 22:00 to 07:00, weekdays 09:00 to 17:00). A cell site is not a position.',
    helpText: ['A change of colour that stays for two weeks or more is a move. A hatched block is a different place for less than two weeks and then back again: a trip. A week with too few events has no block.', 'Hover a block to see its cell site and weeks.'],
    tableFn: () => table(['Subject', 'When', 'Weeks', 'Cell site', 'What'], movers.flatMap(m => [...(m.anchors.night || []).map(a => ({ night: a })), ...(m.anchors.day || []).map(a => ({ day: a })), ...m.short_deviations.map(d => ({ away: d }))].map(x => { const k = Object.keys(x)[0]; const a = x[k]; return { cells: [lbl(m.label, 'subject'), k === 'night' ? 'sleeps' : k === 'day' ? 'works' : 'sleeps (away)', `${a.first_week + 1} to ${a.last_week + 1}`, siteName(a.cell_key), k === 'away' ? 'a trip' : 'steady'], sort: [m.label, k, a.first_week, a.cell_key, k] }; }))),
    height: Math.max(260, 70 + lanes.length * 44),
    chart: () => {
      const blocks = []; lanes.forEach((l, i) => { (l.m.anchors[l.slot] || []).forEach(a => blocks.push({ value: [i, a.first_week, a.last_week + 1], cell: a.cell_key, away: false })); if (l.slot === 'night') l.m.short_deviations.filter(d => d.slot === 'night').forEach(d => blocks.push({ value: [i, d.first_week, d.last_week + 1], cell: d.cell_key, away: true })); });
      return {
        aria: { label: { description: `Weekly sleeping and working places for ${movers.length} subjects.` } },
        tooltip: { confine: true, formatter: p => `<b>${lanes[p.data.value[0]].name}</b><br>cell site ${siteName(p.data.cell)}<br>weeks ${p.data.value[1] + 1} to ${p.data.value[2]}${p.data.away ? '<br><b>a trip: back within two weeks</b>' : ''}` },
        grid: { left: 150, right: 24, top: 14, bottom: 44 }, xAxis: { type: 'value', min: 0, max: weeks, interval: 1, name: 'week of the case', nameLocation: 'middle', nameGap: 22, nameTextStyle: { color: muted() }, axisLabel: { color: muted(), formatter: v => (v < weeks ? String(v + 1) : '') }, splitLine: { show: true, lineStyle: { color: line() } } },
        yAxis: { type: 'category', inverse: true, data: lanes.map(l => l.name), axisLabel: { color: ink(), fontSize: 12 }, axisTick: { show: false } },
        series: [{ type: 'custom', data: blocks, encode: { x: [1, 2], y: 0 }, renderItem: (params, api) => { const y = api.coord([0, api.value(0)])[1]; const p1 = api.coord([api.value(1), 0])[0], p2 = api.coord([api.value(2), 0])[0]; const a = Math.min(p1, p2), b = Math.max(p1, p2); const hh = api.size([0, 1])[1] * 0.7; const d = blocks[params.dataIndex]; const w = Math.max(4, b - a - 2); return { type: 'group', children: [{ type: 'rect', shape: { x: a + 1, y: y - hh / 2, width: w, height: hh, r: 4 }, style: d.away ? { fill: colourOf(d.cell), opacity: 0.55, stroke: ink(), lineDash: [4, 3], lineWidth: 1.5 } : { fill: colourOf(d.cell), stroke: 'rgba(0,0,0,.25)' } }, w > 46 ? { type: 'text', style: { x: a + 7, y, text: (d.away ? 'away ' : '') + siteName(d.cell), fill: '#fff', fontSize: 11, textVerticalAlign: 'middle', textShadowColor: 'rgba(0,0,0,.55)', textShadowBlur: 2 } } : null].filter(Boolean) }; } }],
      };
    },
  });
}

// ---- A11 change of rhythm ----------------------------------------------------------------------------------------------------------------------------
function rhythmCards(views) {
  return views.rhythm.map(r => {
    const days = r.daily.map(d => d[0]); const ch = r.change; const cp = days.indexOf(ch.first_day); const start = new Date(views.case.start + 'T00:00:00Z');
    const before = r.heat.filter(([d]) => d < cp), after = r.heat.filter(([d]) => d >= cp);
    const profile = (rows, n) => Array.from({ length: 24 }, (_, hr) => +(rows.filter(x => x[1] === hr).reduce((s, x) => s + x[2], 0) / Math.max(1, n)).toFixed(2));
    const heatDays = Array.from({ length: views.case.days }, (_, i) => new Date(start.getTime() + i * 86400000).toISOString().slice(5, 10));
    const trend = makeCard({
      analysis: 'A11', title: `Daily activity of ${lbl(r.label, 'subject')}`, question: 'Is there a day on which this number\'s rhythm changed?',
      chips: [chip(ch.mean_before, 'events a day before'), chip(ch.mean_after, 'events a day after', 'hot'), chip('×' + ch.ratio, ch.direction === 'up' ? 'busier' : 'quieter'), chip(ch.t, 'strength of evidence (t)')],
      note: `Best split of the daily counts: from ${ch.first_day} the number is ${ch.direction === 'up' ? 'busier' : 'quieter'}. A change of rhythm in the records, not a cause.`,
      helpText: ['The line is the number of events per day. The grey bands are the average before and after the detected day; the dashed vertical line is that day. The test tries every day with at least ten days on each side and keeps the split where the two averages differ most, reporting it only if the difference is large (the ratio at least 1.5) and clearly not chance (t at least 5). One unusually busy day does not pass this test.'],
      tableFn: () => table(['Day', 'Events'], r.daily.map(d => ({ cells: [d[0], d[1]], sort: [d[0], d[1]] }))), height: 300,
      chart: () => ({
        aria: { label: { description: `Daily events of ${r.label}: ${ch.mean_before} a day before ${ch.first_day} and ${ch.mean_after} after.` } },
        tooltip: { trigger: 'axis', confine: true }, grid: { left: 46, right: 24, top: 20, bottom: 56 }, dataZoom: [{ type: 'inside' }, { type: 'slider', height: 16, bottom: 6 }],
        xAxis: { type: 'category', data: days, axisLabel: { color: muted() } }, yAxis: { type: 'value', name: 'events', nameTextStyle: { color: muted() }, axisLabel: { color: muted() }, splitLine: { lineStyle: { color: line() } } },
        series: [{ type: 'line', data: r.daily.map(d => d[1]), showSymbol: false, lineStyle: { color: '#0072B2', width: 1.8 }, areaStyle: { color: 'rgba(0,114,178,.10)' },
          markArea: { silent: true, data: [[{ xAxis: 0, itemStyle: { color: 'rgba(100,116,139,.10)' }, name: 'before' }, { xAxis: cp }], [{ xAxis: cp, itemStyle: { color: 'rgba(217,119,6,.12)' }, name: 'after' }, { xAxis: days.length - 1 }]], label: { color: muted() } },
          markLine: { silent: true, symbol: 'none', data: [{ xAxis: cp, lineStyle: { color: '#d97706', type: 'dashed', width: 2 }, label: { formatter: 'change: ' + ch.first_day, color: '#b45309', position: 'end', rotate: 0, backgroundColor: 'rgba(255,255,255,.85)', padding: [2, 5], borderRadius: 3 } }, [{ coord: [0, ch.mean_before], lineStyle: { color: '#475569', type: 'solid', width: 2 } }, { coord: [cp, ch.mean_before] }], [{ coord: [cp, ch.mean_after], lineStyle: { color: '#475569', type: 'solid', width: 2 } }, { coord: [days.length - 1, ch.mean_after] }]] } }],
      }),
    });
    const maxv = Math.max(...r.heat.map(x => x[2]));
    const heat = makeCard({
      analysis: 'A11', title: `Hour of day by day, ${lbl(r.label, 'subject')}`, question: 'At what time of day is this number active, and did that shift?',
      chips: [chip(Math.max(...profile(before, cp)), 'busiest hour before (events a day)'), chip(Math.max(...profile(after, views.case.days - cp)), 'busiest hour after (events a day)', 'hot')],
      helpText: ['Each small square is one hour of one day; darker means more events. Read across to see a day, down to see an hour. The right-hand chart averages each hour over the days before and after the change.'],
      tableFn: () => table(['Hour', 'Per day before', 'Per day after'], profile(before, cp).map((v, hr) => ({ cells: [String(hr).padStart(2, '0') + ':00', v, profile(after, views.case.days - cp)[hr]], sort: [hr, v, profile(after, views.case.days - cp)[hr]] }))), height: 340,
      chart: () => ({
        aria: { label: { description: 'Events by day and hour of day, and the average per hour before and after the change.' } },
        tooltip: { confine: true, formatter: p => p.seriesType === 'heatmap' ? `${heatDays[p.data[0]]} · ${String(p.data[1]).padStart(2, '0')}:00<br><b>${p.data[2]}</b> events` : `${String(p.name).padStart(2, '0')}:00<br>${p.seriesName}: ${p.value}` },
        grid: [{ left: 44, right: '36%', top: 18, bottom: 70 }, { left: '70%', right: 24, top: 18, bottom: 70 }],
        xAxis: [{ type: 'category', gridIndex: 0, data: heatDays, axisLabel: { color: muted(), interval: 13 }, splitArea: { show: false } }, { type: 'category', gridIndex: 1, data: Array.from({ length: 24 }, (_, i) => i), axisLabel: { color: muted(), interval: 3, formatter: v => String(v).padStart(2, '0') } }],
        yAxis: [{ type: 'category', gridIndex: 0, inverse: true, data: Array.from({ length: 24 }, (_, i) => String(i).padStart(2, '0')), axisLabel: { color: muted(), interval: 2 } }, { type: 'value', gridIndex: 1, name: 'events per day', nameTextStyle: { color: muted() }, axisLabel: { color: muted() }, splitLine: { lineStyle: { color: line() } } }],
        visualMap: { min: 0, max: maxv, calculable: false, orient: 'horizontal', left: 44, bottom: 6, itemWidth: 12, itemHeight: 140, text: [maxv + '', '0'], textStyle: { color: muted() }, inRange: { color: ['#eef4fb', '#a6c4ec', '#3b73c9', '#0c2f6d'] }, seriesIndex: 0 },
        series: [{ type: 'heatmap', xAxisIndex: 0, yAxisIndex: 0, data: r.heat, progressive: 0 }, { name: 'before', type: 'line', xAxisIndex: 1, yAxisIndex: 1, data: profile(before, cp), showSymbol: false, lineStyle: { color: '#475569', width: 2 } }, { name: 'after', type: 'line', xAxisIndex: 1, yAxisIndex: 1, data: profile(after, views.case.days - cp), showSymbol: false, lineStyle: { color: '#d97706', width: 2 } }],
        legend: { data: ['before', 'after'], right: 24, bottom: 4, textStyle: { color: ink() } },
      }),
    });
    return [trend, heat];
  });
}

// ---- A13 address translation ------------------------------------------------------------------------------------------------------------------------
function addressCard(views) {
  const a = views.addresses; if (!a.length) return null;
  const count = k => a.filter(x => x.result === k).length; let only = false;
  const holder = h('div');
  const kindOf = r => (r === 'unique' ? 'good' : r === 'ambiguous' ? 'bad' : 'neutral');
  const draw = () => holder.replaceChildren(table(['Address and port', 'When', 'Result', 'Who held it'], a.filter(x => !only || x.result === 'ambiguous').map(x => ({ cells: [`${x.public_ip}:${x.port}`, fmtDT(x.instant), badge(x.result, kindOf(x.result)), x.holders.length ? x.holders.map(l => lbl(l, 'subject')).join(', ') : '—'], sort: [x.public_ip + x.port, x.instant, x.result, x.holders.join()] })), { caption: `${a.length} lookups` }));
  draw();
  return h('section', { class: 'card' },
    h('header', {}, h('h3', {}, 'Who held this public address? ', badge('A13', 'info')), h('p', { class: 'q' }, 'Many subscribers share one public address and take turns on its ports. Who had this port at this moment?')),
    h('div', { class: 'chips' }, chip(a.length, 'lookups'), chip(count('unique'), 'one holder', 'good'), chip(count('ambiguous'), 'ambiguous: two or more', 'hot'), chip(count('none'), 'nobody')),
    h('p', { class: 'note bad' }, 'Ambiguous is never resolved by guessing: when two subscribers could have held the address and port, the answer says so and both are listed.'),
    h('div', { class: 'row' }, h('label', { class: 'tog' }, h('input', { type: 'checkbox', onchange: e => { only = e.target.checked; draw(); } }), ' Show only ambiguous')), holder,
    help('How to read this', 'An operator shares each public address between many subscribers by giving each a block of ports for a stretch of time. To say who used an address you need the port and the exact moment. "One holder" means the records show a single subscriber with that port at that moment; "ambiguous" means the records show more than one.'));
}

export function mountTimelines(root, views) {
  charts.splice(0).forEach(c => c.dispose()); root.replaceChildren();
  const dev = deviceCard(views), pair = pairCard(views), anchors = anchorsCard(views), rhythm = rhythmCards(views).flat(), addr = addressCard(views);
  const wrap = h('div', { class: 'cards' }, dev, pair, anchors, ...rhythm, addr);
  root.append(h('div', { class: 'toolbar' }, h('span', { class: 'muted' }, 'Each card answers one question. Use the buttons under a chart to see its numbers as a table or to save it.')), wrap);
  [dev, anchors, ...rhythm].filter(Boolean).forEach(c => c.draw()); pair.draw?.();
  return { cards: [dev, pair, anchors, ...rhythm, addr].filter(Boolean), resize: () => charts.forEach(c => !c.isDisposed() && c.resize()), redraw: () => { [dev, anchors, ...rhythm].filter(Boolean).forEach(c => c.draw()); pair.draw?.(); } };
}

export function exportSvgTwice(views) {
  const render = () => svgOf({ ...deviceCardOption(views) }, 800, 330);
  const a = render(), b = render();
  return { identical: a === b, bytes: a.length };
}
function deviceCardOption(views) {
  const lanes = views.lanes.devices.slice(0, 12);
  return { animation: false, xAxis: { type: 'time' }, yAxis: { type: 'category', data: lanes.map(l => l.label) }, series: [{ type: 'scatter', data: lanes.flatMap((l, i) => l.runs.map(r => [T(r.first), i])) }] };
}
