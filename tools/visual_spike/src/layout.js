import { rng } from './common.js';

// A small deterministic force layout (the same for both libraries, so that the comparison is of drawing, not of layout).
export function layout(nodes, edges, iterations = 220) {
  const r = rng(7); const pos = new Map(); const n = nodes.length;
  nodes.forEach((node, i) => pos.set(node.id, { x: Math.cos(i / n * 6.283) * 300 + (r() - .5) * 40, y: Math.sin(i / n * 6.283) * 300 + (r() - .5) * 40, vx: 0, vy: 0 }));
  const k = Math.sqrt((900 * 900) / Math.max(n, 1)) * 0.45;
  for (let it = 0; it < iterations; it++) {
    const cool = 1 - it / iterations;
    if (n <= 1500) for (let i = 0; i < n; i++) for (let j = i + 1; j < n; j++) {
      const a = pos.get(nodes[i].id), b = pos.get(nodes[j].id); let dx = a.x - b.x, dy = a.y - b.y; const d2 = dx * dx + dy * dy + 0.01, f = (k * k) / d2;
      a.vx += dx * f; a.vy += dy * f; b.vx -= dx * f; b.vy -= dy * f;
    }
    for (const e of edges) {
      const a = pos.get(e.source), b = pos.get(e.target); if (!a || !b) continue;
      const dx = b.x - a.x, dy = b.y - a.y, d = Math.sqrt(dx * dx + dy * dy) + 0.01, f = (d * d) / k / d;
      a.vx += dx * f * .02; a.vy += dy * f * .02; b.vx -= dx * f * .02; b.vy -= dy * f * .02;
    }
    for (const p of pos.values()) { const s = Math.min(8, Math.hypot(p.vx, p.vy)) * cool; const m = Math.hypot(p.vx, p.vy) || 1; p.x += p.vx / m * s; p.y += p.vy / m * s; p.vx = p.vy = 0; }
  }
  // fit the result into a 900 by 700 box, so that sizes in the styles mean the same whatever the force constants did
  const xs = [...pos.values()].map(p => p.x), ys = [...pos.values()].map(p => p.y);
  const x0 = Math.min(...xs), x1 = Math.max(...xs), y0 = Math.min(...ys), y1 = Math.max(...ys);
  const s = Math.min(900 / Math.max(x1 - x0, 1), 700 / Math.max(y1 - y0, 1));
  for (const p of pos.values()) { p.x = (p.x - (x0 + x1) / 2) * s; p.y = (p.y - (y0 + y1) / 2) * s; }
  return pos;
}

export function stressGraph(count, degree) {
  const r = rng(11); const nodes = [], edges = [];
  const groups = Math.max(4, Math.round(count / 120));
  for (let i = 0; i < count; i++) nodes.push({ id: 's' + i, label: String(i), kind: i % 97 === 0 ? 'subject' : 'contact', group: i % groups });
  for (let i = 0; i < count * degree; i++) {
    const a = Math.floor(r() * count); const same = r() < 0.85; let b = same ? (Math.floor(r() * (count / groups)) * groups + (a % groups)) % count : Math.floor(r() * count);
    if (a !== b) edges.push({ source: 's' + a, target: 's' + b, kind: r() < 0.9 ? 'observed' : r() < 0.7 ? 'inferred' : 'candidate', count: 1 + Math.floor(r() * 8), rows: [] });
  }
  return { nodes, edges };
}
