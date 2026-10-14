// Small DOM helpers shared by the views: elements, badges, statistic chips, help boxes, tables, downloads.
import { CARRIERS } from './carriers.js';
export const $ = (sel, root = document) => root.querySelector(sel);
export const $$ = (sel, root = document) => [...root.querySelectorAll(sel)];

export function h(tag, attrs = {}, ...kids) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (v == null || v === false) continue;
    if (k === 'class') el.className = v;
    else if (k === 'html') el.innerHTML = v;
    else if (k.startsWith('on')) el.addEventListener(k.slice(2), v);
    else el.setAttribute(k, v === true ? '' : v);
  }
  for (const kid of kids.flat()) if (kid != null && kid !== false) el.append(kid.nodeType ? kid : document.createTextNode(String(kid)));
  return el;
}

export const esc = s => String(s ?? '').replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
export const badge = (text, kind = 'neutral', title) => h('span', { class: 'badge ' + kind, title }, text);
export const chip = (value, label, kind = '') => h('div', { class: 'chip ' + kind }, h('b', {}, value), h('span', {}, label));

export function help(title, ...paragraphs) {
  return h('details', { class: 'help' }, h('summary', {}, title), ...paragraphs.map(p => (typeof p === 'string' ? h('p', {}, p) : p)));
}

export function table(headers, rows, { caption, onRow, sortable = true } = {}) {
  const t = h('table', { class: 'data' });
  if (caption) t.append(h('caption', {}, caption));
  const head = h('tr', {}, headers.map((x, i) => h('th', { scope: 'col', tabindex: sortable ? 0 : null, 'aria-sort': 'none', onclick: sortable ? () => sortBy(i) : null, onkeydown: sortable ? e => { if (e.key === 'Enter') sortBy(i); } : null }, x)));
  t.append(h('thead', {}, head));
  const body = h('tbody');
  rows.forEach(r => { const tr = h('tr', { tabindex: onRow ? 0 : null, onclick: onRow ? () => onRow(r) : null, onkeydown: onRow ? e => { if (e.key === 'Enter') onRow(r); } : null }, r.cells.map(c => h('td', {}, c))); tr._row = r; body.append(tr); });
  t.append(body);
  let dir = 1, last = -1;
  function sortBy(i) {
    dir = last === i ? -dir : 1; last = i;
    const trs = [...body.children].sort((a, b) => { const x = a._row.sort?.[i] ?? a.children[i].textContent, y = b._row.sort?.[i] ?? b.children[i].textContent; return (typeof x === 'number' && typeof y === 'number' ? x - y : String(x).localeCompare(String(y), undefined, { numeric: true })) * dir; });
    trs.forEach(tr => body.append(tr));
    head.querySelectorAll('th').forEach((th, j) => th.setAttribute('aria-sort', j === i ? (dir > 0 ? 'ascending' : 'descending') : 'none'));
  }
  return t;
}

export function download(name, blobOrUrl, type) {
  const url = typeof blobOrUrl === 'string' && blobOrUrl.startsWith('data:') ? blobOrUrl : URL.createObjectURL(blobOrUrl instanceof Blob ? blobOrUrl : new Blob([blobOrUrl], { type }));
  const a = h('a', { href: url, download: name }); document.body.append(a); a.click(); a.remove();
  if (!url.startsWith('data:')) setTimeout(() => URL.revokeObjectURL(url), 2000);
}

export const fmtDate = s => (s || '').slice(0, 10);
export const fmtTime = s => (s || '').slice(11, 16);
export const fmtDT = s => (s || '').slice(0, 16).replace('T', ' ');
export const pct = x => (x == null ? '—' : (x * 100).toFixed(0) + '%');
export const plural = (n, one, many) => `${n} ${n === 1 ? one : many || one + 's'}`;

// Colours that stay apart in both themes and for colour-blind readers (Okabe-Ito based), by operator and by cell.
// The views carry a carrier id (jazz, zong, ufone, telenor, scom, other) where they used to carry a file layout's name; the colours and letters come from the carrier registry.
export const OPERATOR_COLOURS = Object.fromEntries(Object.values(CARRIERS).map(c => [c.id, c.colour]));
export const OPERATOR_SHAPE = Object.fromEntries(Object.values(CARRIERS).map(c => [c.id, c.initial]));
export const carrierName = id => (CARRIERS[id] || CARRIERS.other).name;
export function hueOf(text) { let x = 0; for (const ch of String(text)) x = (x * 31 + ch.charCodeAt(0)) % 360; return x; }
export const colourOf = text => `hsl(${hueOf(text)} 58% 50%)`;
export const shortCell = key => String(key).split(':').slice(-1)[0];

// Cell sites get short names a person can say aloud: the operator's letter and a number by position on the grid (U-03), instead of the key.
const siteNames = new Map();
export function registerSites(cells) {
  const per = {};
  [...cells].sort((a, b) => a.operator.localeCompare(b.operator) || b.lat - a.lat || a.lon - b.lon).forEach(c => { per[c.operator] = (per[c.operator] || 0) + 1; siteNames.set(c.key, `${OPERATOR_SHAPE[c.operator] || '?'}-${String(per[c.operator]).padStart(2, '0')}`); });
}
export const siteName = key => siteNames.get(key) || '…' + shortCell(key);
