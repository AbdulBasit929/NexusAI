// node --test scripts/test-overview.mjs : the overview mode groups numbers without losing calls, subjects or determinism.
import test from 'node:test';
import assert from 'node:assert/strict';
import { communities, collapse } from '../src/overview.js';

// two cliques of contacts, each called by one subject, joined by one weak link; plus a loose contact
function sample() {
  const nodes = [{ id: 'S1', kind: 'subject' }, { id: 'S2', kind: 'subject' }];
  const edges = [];
  for (const [s, prefix] of [['S1', 'a'], ['S2', 'b']]) {
    const ids = [1, 2, 3, 4].map(i => prefix + i);
    ids.forEach(id => { nodes.push({ id, kind: 'contact' }); edges.push({ source: s, target: id, kind: 'observed', count: 10, rows: [{ time: '2026-01-01 10:00:00' }] }); });
    for (let i = 0; i < ids.length; i++) for (let j = i + 1; j < ids.length; j++) edges.push({ source: ids[i], target: ids[j], kind: 'observed', count: 5, rows: [] });
  }
  nodes.push({ id: 'z1', kind: 'contact' });
  edges.push({ source: 'a1', target: 'b1', kind: 'observed', count: 1, rows: [] }, { source: 'S1', target: 'S2', kind: 'inferred', count: 2, rows: [] });
  return { nodes, edges };
}

test('communities are the same every time', () => {
  const { nodes, edges } = sample();
  assert.deepEqual([...communities(nodes, edges)], [...communities(nodes, edges)]);
});

test('the two cliques become two groups and no subject is grouped', () => {
  const { nodes, edges } = sample();
  const out = collapse(nodes, edges, communities(nodes, edges), { minGroup: 3 });
  assert.equal(out.groups.length, 2);
  assert.deepEqual(out.groups.map(g => g.members.length).sort(), [4, 4]);
  assert.ok(out.nodes.some(n => n.id === 'S1') && out.nodes.some(n => n.id === 'S2'));
  assert.equal(out.nodes.filter(n => n.kind === 'group').length, 2);
  assert.ok(out.nodes.some(n => n.id === 'z1'), 'a number in no group stays');
});

test('no call is lost: the count of every link is the sum of what it replaced', () => {
  const { nodes, edges } = sample();
  const out = collapse(nodes, edges, communities(nodes, edges), { minGroup: 3 });
  const inside = new Set(out.groups.flatMap(g => g.members));
  const wasInsideAGroup = edges.filter(e => e.kind === 'observed' && inside.has(e.source) && inside.has(e.target) && out.groupOf.get(e.source) === out.groupOf.get(e.target));
  const total = edges.filter(e => e.kind === 'observed').reduce((s, e) => s + e.count, 0);
  const kept = out.edges.filter(e => e.kind === 'observed').reduce((s, e) => s + e.count, 0);
  assert.equal(kept + wasInsideAGroup.reduce((s, e) => s + e.count, 0), total);
});

test('links that are not calls are kept as they are', () => {
  const { nodes, edges } = sample();
  const out = collapse(nodes, edges, communities(nodes, edges), { minGroup: 3 });
  assert.equal(out.edges.filter(e => e.kind === 'inferred').length, 1);
});

test('a link that is not a call and touches a grouped number is drawn to its group, never to a number that is not on the chart', () => {
  const { nodes, edges } = sample();
  edges.push({ source: 'a2', target: 'b2', kind: 'inferred', count: 3, rows: [] }, { source: 'S1', target: 'a3', kind: 'candidate', count: 1, rows: [] });
  const out = collapse(nodes, edges, communities(nodes, edges), { minGroup: 3 });
  const present = new Set(out.nodes.map(n => n.id));
  assert.ok(out.edges.every(e => present.has(e.source) && present.has(e.target)), 'every edge ends at a drawn node');
  assert.ok(out.edges.some(e => e.kind === 'inferred' && e.group), 'the inferred link between two groups is kept');
});

test('an opened group is shown as its numbers', () => {
  const { nodes, edges } = sample();
  const label = communities(nodes, edges);
  const closed = collapse(nodes, edges, label, { minGroup: 3 });
  const open = collapse(nodes, edges, label, { minGroup: 3, expanded: new Set([closed.groups[0].id]) });
  assert.equal(open.groups.length, 1);
  assert.equal(open.nodes.length, closed.nodes.length - 1 + closed.groups[0].members.length);
});

test('a group smaller than the minimum stays as numbers', () => {
  const { nodes, edges } = sample();
  const out = collapse(nodes, edges, communities(nodes, edges), { minGroup: 9 });
  assert.equal(out.groups.length, 0);
  assert.equal(out.nodes.length, nodes.length);
  assert.equal(out.edges.length, edges.length);
});
