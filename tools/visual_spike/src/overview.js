// Overview mode for big graphs (roadmap 7.7, rule V5): above about 1,500 numbers a link chart is unreadable, so the numbers that are not subjects are
// grouped by community and each group is drawn as one node with a count; a click opens the group. Pure functions, tested with `node --test`.

// Label propagation: every number starts in a community of its own and repeatedly takes the community most of its calls go to (heaviest first, ties to the
// smaller label), visiting the numbers in a fixed order, so the same graph always gives the same communities.
export function communities(nodes, edges, rounds = 12) {
  const ids = nodes.map(n => n.id).sort();
  const nb = new Map(ids.map(i => [i, []]));
  for (const e of edges) if (e.kind === 'observed' && nb.has(e.source) && nb.has(e.target)) { nb.get(e.source).push([e.target, e.count]); nb.get(e.target).push([e.source, e.count]); }
  const label = new Map(ids.map(i => [i, i]));
  for (let r = 0; r < rounds; r++) {
    let changed = 0;
    for (const id of ids) {
      const score = new Map();
      for (const [other, w] of nb.get(id)) score.set(label.get(other), (score.get(label.get(other)) || 0) + w);
      if (!score.size) continue;
      let best = null, bestW = -1;
      for (const [l, w] of [...score].sort((a, b) => (a[0] < b[0] ? -1 : 1))) if (w > bestW) { best = l; bestW = w; }
      if (best !== label.get(id)) { label.set(id, best); changed++; }
    }
    if (!changed) break;
  }
  return label;
}

// Groups the numbers of kind "contact" by community (subjects and hubs are never grouped, since they are what the analyst came for). A community of fewer
// than `minGroup` stays as it is, and so does a group the analyst has opened (`expanded`). Calls between numbers of two different groups (or a group and a number) are added up; other links that touch a group are drawn to the group.
export function collapse(nodes, edges, label, { minGroup = 3, expanded = new Set() } = {}) {
  const members = new Map();
  for (const n of nodes) if (n.kind === 'contact') { const l = label.get(n.id); if (!members.has(l)) members.set(l, []); members.get(l).push(n.id); }
  const groupOf = new Map(); const groups = [];
  for (const [l, ids] of [...members].sort((a, b) => (a[0] < b[0] ? -1 : 1))) {
    const gid = 'grp:' + l;
    if (ids.length >= minGroup && !expanded.has(gid)) { ids.forEach(i => groupOf.set(i, gid)); groups.push({ id: gid, members: ids.slice().sort() }); }
  }
  const outNodes = nodes.filter(n => !groupOf.has(n.id)).concat(groups.map(g => ({ id: g.id, kind: 'group', label: g.members.length + ' numbers', members: g.members, count: g.members.length })));
  const rep = id => groupOf.get(id) ?? id;
  const merged = new Map(); const outEdges = [];
  edges.forEach((e, idx) => {
    const a = rep(e.source), b = rep(e.target);
    if (a === b) return;
    if (a === e.source && b === e.target) { outEdges.push({ ...e, idx }); return; }
    if (e.kind !== 'observed') { outEdges.push({ source: a, target: b, kind: e.kind, count: e.count, links: 1, rows: [], group: true, idx: -1 }); return; }   // a link that is not a call, drawn to the group
    const key = a < b ? a + '|' + b : b + '|' + a;
    if (!merged.has(key)) merged.set(key, { source: a, target: b, kind: 'observed', count: 0, links: 0, rows: [], group: true, idx: -1 });
    const m = merged.get(key); m.count += e.count; m.links++;
    if (m.rows.length < 6) m.rows.push(...e.rows.slice(0, 2));
  });
  return { nodes: outNodes, edges: [...outEdges, ...merged.values()], groups, groupOf };
}
