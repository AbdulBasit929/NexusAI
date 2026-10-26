import { forwardRef, useEffect, useImperativeHandle, useRef } from 'react'
import cytoscape from 'cytoscape'
import { chartTheme } from '../../components/charts/chartTheme.js'
import { useThemeVersion } from '../../lib/useThemeVersion.js'

// The links pane of the workbench direction proof: Cytoscape, drawn with the application's own tokens. A working set, not "everything": the numbers of the question and their strongest neighbours.
// Rules applied (docs/ux/workbench/RESEARCH_NOTE.md): style differs only where it carries meaning (a subscriber is a rounded square, another number a small circle, the numbers of the question are larger
// and filled); labels of the minor numbers appear on hover or selection, so that a small working set reads at a glance and a larger one does not become a wall of text; colour is never the only
// encoding (an inferred link is dashed, a candidate dotted, with an arrow where it has a direction); every mark has its row in the table and its page in the inspector.
const cssVar = (name, fallback) => (globalThis.document ? globalThis.getComputedStyle(globalThis.document.documentElement).getPropertyValue(name).trim() || fallback : fallback)

function styleFor(theme) {
  const accent = cssVar('--analyst-accent', theme.data[0])
  return [
    { selector: 'node', style: { 'background-color': theme.card, 'border-width': 1.25, 'border-color': theme.muted, width: 'data(size)', height: 'data(size)', label: 'data(label)', color: theme.muted, 'font-size': 10.5, 'text-valign': 'bottom', 'text-margin-y': 4, 'font-family': 'inherit', 'min-zoomed-font-size': 7, 'text-outline-width': 3, 'text-outline-color': theme.card } },
    { selector: 'node[?subject]', style: { shape: 'round-rectangle', 'border-color': accent, 'border-width': 1.5 } },
    { selector: 'node[?focus]', style: { 'background-color': accent, 'border-color': accent, color: theme.text, 'font-weight': 600, 'font-size': 11.5 } },
    { selector: 'node.lbl-off', style: { label: '' } },
    { selector: 'node.hover', style: { 'border-width': 2.5, color: theme.text } },
    { selector: 'edge', style: { width: 'data(width)', 'curve-style': 'bezier', 'line-color': theme.muted, opacity: 0.4 } },
    { selector: 'edge[kind="colocated"]', style: { 'line-style': 'dashed', 'line-dash-pattern': [6, 4], 'line-color': theme.data[2], opacity: 0.95, width: 2 } },
    { selector: 'edge[kind="meeting"]', style: { 'line-style': 'dotted', 'line-color': theme.data[3], opacity: 0.95, width: 2.5 } },
    { selector: 'edge[kind="successor"]', style: { 'line-style': 'dotted', 'line-color': theme.data[1], 'target-arrow-shape': 'triangle', 'target-arrow-color': theme.data[1], 'arrow-scale': 0.9, opacity: 0.95, width: 2.5 } },
    { selector: '.dim', style: { opacity: 0.5, 'text-opacity': 0.7 } },
    { selector: 'node.sel', style: { 'border-width': 3, 'border-color': theme.text, 'z-index': 10 } },
    { selector: 'edge.sel', style: { opacity: 1, width: 3, 'line-color': accent, 'z-index': 10, 'target-arrow-color': accent } },
    { selector: ':active', style: { 'overlay-opacity': 0.06 } },
  ]
}

export const selectionOfLink = link => {
  const [a, b] = [link.source, link.target]
  if (link.kind === 'colocated') return { type: 'window', index: 0, a, b }
  if (link.kind === 'meeting') return { type: 'meeting', index: 0, a, b }
  if (link.kind === 'successor') return { type: 'successor', old: a, new: b }
  return { type: 'edge', a, b }
}

function selectedIds(selection) {
  if (!selection) return { nodes: [], edges: [] }
  switch (selection.type) {
    case 'node': return { nodes: [selection.id], edges: [] }
    case 'edge': return { nodes: [selection.a, selection.b], edges: [`c:${[selection.a, selection.b].sort().join('|')}`] }
    case 'window': return { nodes: [selection.a, selection.b], edges: [`w:${[selection.a, selection.b].sort().join('|')}`] }
    case 'meeting': return { nodes: [selection.a, selection.b], edges: [`m:${[selection.a, selection.b].sort().join('|')}`] }
    case 'successor': return { nodes: [selection.old, selection.new], edges: [`s:${selection.old}|${selection.new}`] }
    case 'move': case 'alert': case 'rhythm': return { nodes: [selection.number], edges: [] }
    default: return { nodes: [], edges: [] }
  }
}

const LAYOUTS = {
  focus: () => ({ name: 'concentric', animate: false, fit: true, padding: 34, minNodeSpacing: 26, concentric: node => (node.data('focus') ? 3 : node.data('subject') ? 2 : 1), levelWidth: () => 1 }),
  circle: () => ({ name: 'circle', animate: false, fit: true, padding: 34 }),
  tree: focus => ({ name: 'breadthfirst', animate: false, fit: true, padding: 30, directed: false, roots: focus.map(id => `#${id}`), spacingFactor: 1.15 }),
}

export const WbGraph = forwardRef(function WbGraph({ graph, selection, onSelect, height = 420, layout = 'focus', labels = 'smart', label = 'Link chart' }, ref) {
  const host = useRef(null)
  const cy = useRef(null)
  const handlers = useRef({})
  handlers.current = { onSelect }
  const labelMode = useRef(labels)
  labelMode.current = labels
  const themeVersion = useThemeVersion()

  const applyLabels = () => {
    const instance = cy.current
    if (!instance) return
    instance.nodes().forEach(node => {
      const show = labelMode.current === 'all' || (labelMode.current === 'smart' && (node.data('focus') || node.hasClass('sel') || node.hasClass('hover')))
      node.toggleClass('lbl-off', !show)
    })
  }

  useImperativeHandle(ref, () => ({
    fit: () => cy.current?.fit(undefined, 30),
    zoom: factor => { const c = cy.current; if (c) c.zoom({ level: c.zoom() * factor, renderedPosition: { x: c.width() / 2, y: c.height() / 2 } }) },
  }), [])

  useEffect(() => {
    const instance = cytoscape({ container: host.current, elements: [], style: styleFor(chartTheme()), minZoom: 0.35, maxZoom: 3, wheelSensitivity: 0.25 })
    cy.current = instance
    instance.on('tap', 'node', event => handlers.current.onSelect?.({ type: 'node', id: event.target.id() }))
    instance.on('tap', 'edge', event => handlers.current.onSelect?.(selectionOfLink(event.target.data())))
    instance.on('tap', event => { if (event.target === instance) handlers.current.onSelect?.(null) })
    instance.on('mouseover', 'node', event => { event.target.addClass('hover'); host.current.style.cursor = 'pointer'; applyLabels() })
    instance.on('mouseout', 'node', event => { event.target.removeClass('hover'); host.current.style.cursor = ''; applyLabels() })
    instance.on('mouseover', 'edge', () => { host.current.style.cursor = 'pointer' })
    instance.on('mouseout', 'edge', () => { host.current.style.cursor = '' })
    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(() => { instance.resize(); instance.fit(undefined, 30) })
    observer?.observe(host.current)
    return () => { observer?.disconnect(); instance.destroy(); cy.current = null }
  }, [themeVersion])

  useEffect(() => {
    const instance = cy.current
    if (!instance) return
    const nodes = graph.nodes.map(n => ({ group: 'nodes', data: { id: n.id, label: n.label, subject: n.subject, focus: n.focus, size: n.focus ? 24 : n.subject ? 16 : 10 } }))
    const edges = graph.links.map(l => ({ group: 'edges', data: { id: l.id, source: l.source, target: l.target, kind: l.kind, events: l.events, width: l.kind === 'observed' ? 0.75 + Math.min(2.5, Math.log2(1 + l.events) / 2.4) : 2 } }))
    instance.batch(() => { instance.elements().remove(); instance.add([...nodes, ...edges]) })
    const focusIds = graph.nodes.filter(n => n.focus).map(n => n.id)
    instance.layout((LAYOUTS[layout] || LAYOUTS.focus)(focusIds)).run()
    applyLabels()
  }, [graph, layout, themeVersion])

  useEffect(() => {
    const instance = cy.current
    if (!instance) return
    const chosen = selectedIds(selection)
    instance.elements().removeClass('sel dim')
    if (chosen.nodes.length || chosen.edges.length) {
      const keep = instance.collection()
      for (const id of chosen.nodes) keep.merge(instance.getElementById(id))
      for (const id of chosen.edges) keep.merge(instance.getElementById(id))
      keep.addClass('sel')
      instance.elements().difference(keep.union(keep.connectedEdges()).union(keep.connectedNodes())).addClass('dim')
    }
    applyLabels()
  }, [selection, graph, labels, themeVersion])

  return <div ref={host} className="wbp-graph" style={{ blockSize: height }} role="application" tabIndex={0} aria-label={`${label}. ${graph.nodes.length} numbers and ${graph.links.length} links. The same marks are in the table.`} />
})
