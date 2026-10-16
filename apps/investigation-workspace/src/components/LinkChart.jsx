import { forwardRef, useEffect, useImperativeHandle, useMemo, useRef } from 'react'
import cytoscape from 'cytoscape'
import { CARRIERS, carrierIconUrl } from '../lib/carriers.js'
import { chartTheme } from './charts/chartTheme.js'
import { layoutGraph } from '../lib/connectionsModel.js'
import { useThemeVersion } from '../lib/useThemeVersion.js'

// The link chart (roadmap 7.8.5, from the visual spike's linkview.js): Cytoscape.js in its canvas mode, the one that draws every encoding.
//
//   subject        a rounded square filled with its carrier's badge (a number whose records the case holds)
//   contact        a circle, ringed in the carrier of its number range (a ported number may differ: the panel says how the carrier was found)
//   hub            a triangle (a number that many subscribers contact; left out of shared-contact reasoning)
//   service        a diamond (a short code or sender name)
//   group          a hexagon with a count (overview mode: a community of contacts, opened by a click)
//   observed link  a solid line, thicker for more events;  inferred link  a dashed line (two subscribers who share contacts: not a link between them)
//
// The chart is a picture of the model; selection, paths and the shared contacts of two numbers come from the page as props, so that the same state is in the address,
// the panel and the table. Every mark has a table twin (ConnectionsTable): the canvas is not the only way in.
function styleFor(theme) {
  const ring = theme.card
  return [
    { selector: 'node', style: { 'background-color': 'data(color)', width: 'data(size)', height: 'data(size)', label: 'data(label)', 'font-size': 11, color: theme.text, 'text-valign': 'bottom', 'text-margin-y': 3, 'min-zoomed-font-size': 9, 'text-outline-color': ring, 'text-outline-width': 2, 'border-width': 'data(ringW)', 'border-color': 'data(ring)' } },
    { selector: 'node[kind="subject"]', style: { shape: 'round-rectangle', 'font-weight': 700, 'font-size': 12, 'min-zoomed-font-size': 5 } },
    { selector: 'node[icon]', style: { 'background-image': 'data(icon)', 'background-fit': 'cover', 'background-clip': 'node', 'background-image-opacity': 1 } },
    { selector: 'node[kind="hub"]', style: { shape: 'triangle' } },
    { selector: 'node[kind="service"]', style: { shape: 'diamond' } },
    { selector: 'node[kind="group"]', style: { shape: 'round-hexagon', 'border-width': 3, 'border-style': 'double', 'font-weight': 700, 'min-zoomed-font-size': 4 } },
    { selector: 'edge', style: { width: 'data(width)', 'curve-style': 'haystack', 'line-color': theme.muted, opacity: 0.6, 'haystack-radius': 0 } },
    { selector: 'edge[kind="inferred"]', style: { 'line-style': 'dashed', 'line-dash-pattern': [8, 5], 'line-color': theme.data[3], 'curve-style': 'bezier', opacity: 0.95 } },
    { selector: '.hidden', style: { display: 'none' } },
    { selector: '.dim', style: { opacity: 0.1, 'text-opacity': 0 } },
    { selector: 'node.hl', style: { 'border-width': 3, 'border-color': theme.text } },
    { selector: 'edge.hl', style: { opacity: 1, 'z-index': 9 } },
    { selector: 'node.shared', style: { 'border-width': 4, 'border-color': theme.ready, 'z-index': 10 } },
    { selector: 'edge.shared', style: { 'line-color': theme.ready, opacity: 1, 'z-index': 9, width: 3 } },
    { selector: 'node.path', style: { 'border-width': 4, 'border-color': theme.failed, 'z-index': 10 } },
    { selector: 'edge.path', style: { 'line-color': theme.failed, opacity: 1, 'z-index': 10, width: 4, 'line-style': 'solid', 'curve-style': 'bezier' } },
    { selector: 'node.slot', style: { 'border-width': 5, 'border-color': theme.data[0], 'z-index': 11 } },
    { selector: ':selected', style: { 'overlay-color': theme.failed, 'overlay-opacity': 0.18, 'overlay-padding': 6 } },
  ]
}

function nodeData(n, degree, theme) {
  const carrier = CARRIERS[n.carrier]
  const subject = n.kind === 'subject'
  const size = n.kind === 'group' ? 26 + Math.min(40, Math.sqrt(n.count || 1) * 5) : 16 + Math.min(30, degree * 2.2) + (subject ? 12 : 0)
  const colour = subject ? (carrier?.colour || theme.data[0]) : n.kind === 'hub' ? theme.processing : n.kind === 'group' ? theme.withheld : n.kind === 'service' ? theme.muted : theme.line
  const ring = n.carrier && n.carrier !== 'other' && n.kind !== 'group' ? carrier.colour : theme.card
  return { id: n.id, label: n.label, kind: n.kind, size, color: colour, ring, ringW: subject || (n.carrier && n.carrier !== 'other') ? 3 : 1.5, icon: subject && n.carrier ? carrierIconUrl(n.carrier, 64) : undefined }
}

function edgeData(e) {
  return { id: e.id, source: e.source, target: e.target, kind: e.kind, count: e.count, width: e.kind === 'observed' ? 1 + Math.min(4, Math.log2(1 + e.count) / 1.6) : 1.5 + Math.min(3, e.count / 2) }
}

export const LinkChart = forwardRef(function LinkChart({ model, positions = null, selectedId = null, selectedEdgeId = null, slots = {}, pathIds = [], sharedIds = [], dimOthers = true, onSelectNode, onSelectEdge, onHover, label = 'Link chart' }, ref) {
  const host = useRef(null)
  const cy = useRef(null)
  const handlers = useRef({})
  handlers.current = { onSelectNode, onSelectEdge, onHover }
  const modelRef = useRef(model)
  modelRef.current = model
  const themeVersion = useThemeVersion()
  const pos = useMemo(() => positions || layoutGraph(model.nodes, model.edges), [positions, model])

  useImperativeHandle(ref, () => ({
    fit: () => cy.current?.fit(undefined, 40),
    zoomTo: id => { const c = cy.current; if (c) c.animate({ fit: { eles: c.getElementById(id).closedNeighborhood(':visible'), padding: 60 } }, { duration: 250 }) },
    png: () => cy.current?.png({ scale: 2, full: true, bg: chartTheme().card }),
  }), [])

  // build the instance once; elements are replaced when the model changes
  useEffect(() => {
    const theme = chartTheme()
    const instance = cytoscape({ container: host.current, elements: [], layout: { name: 'preset' }, wheelSensitivity: 0.25, minZoom: 0.15, maxZoom: 2.2, pixelRatio: 'auto', style: styleFor(theme) })
    cy.current = instance
    instance.on('tap', 'node', ev => handlers.current.onSelectNode?.(ev.target.id()))
    instance.on('tap', 'edge', ev => handlers.current.onSelectEdge?.(ev.target.id()))
    instance.on('tap', ev => { if (ev.target === instance) handlers.current.onSelectNode?.(null) })
    instance.on('mouseover', 'node', ev => handlers.current.onHover?.(ev.target.id()))
    instance.on('mouseout', 'node', () => handlers.current.onHover?.(null))
    return () => { instance.destroy(); cy.current = null }
  }, [])

  useEffect(() => {
    const instance = cy.current
    if (!instance) return
    const theme = chartTheme()
    const degree = new Map()
    for (const e of model.edges) { degree.set(e.source, (degree.get(e.source) || 0) + 1); degree.set(e.target, (degree.get(e.target) || 0) + 1) }
    const present = new Set(model.nodes.map(n => n.id))
    instance.batch(() => {
      instance.elements().remove()
      instance.add([
        ...model.nodes.map(n => ({ data: nodeData(n, degree.get(n.id) || 0, theme), position: { ...(pos.get(n.id) || { x: 0, y: 0 }) } })),
        ...model.edges.filter(e => present.has(e.source) && present.has(e.target)).map(e => ({ data: edgeData(e) })),
      ])
    })
    instance.style(styleFor(theme))
    instance.fit(undefined, 40)
  }, [model, pos])

  // a change of theme repaints in place: the colours are in the elements' data and the style, so both are read again, and the view (zoom, position) stays
  useEffect(() => {
    const instance = cy.current
    if (!instance || !themeVersion) return
    const theme = chartTheme()
    const current = modelRef.current
    const degree = new Map()
    for (const e of current.edges) { degree.set(e.source, (degree.get(e.source) || 0) + 1); degree.set(e.target, (degree.get(e.target) || 0) + 1) }
    const byId = new Map(current.nodes.map(n => [n.id, n]))
    instance.batch(() => {
      instance.nodes().forEach(node => { const n = byId.get(node.id()); if (n) node.data(nodeData(n, degree.get(n.id) || 0, theme)) })
      instance.style(styleFor(theme))
    })
  }, [themeVersion])

  // selection, paths and the rest are classes on the live elements: no rebuild, no jump
  useEffect(() => {
    const instance = cy.current
    if (!instance) return
    instance.batch(() => {
      instance.elements().removeClass('dim hl shared path slot').unselect()
      const shared = new Set(sharedIds)
      if (shared.size) {
        instance.nodes().forEach(n => { if (shared.has(n.id())) n.addClass('shared') })
        instance.edges().forEach(e => { if (shared.has(e.source().id()) || shared.has(e.target().id())) e.addClass('shared') })
      }
      for (let i = 0; i + 1 < pathIds.length; i++) {
        instance.getElementById(pathIds[i]).addClass('path'); instance.getElementById(pathIds[i + 1]).addClass('path')
        instance.edges().filter(e => (e.source().id() === pathIds[i] && e.target().id() === pathIds[i + 1]) || (e.source().id() === pathIds[i + 1] && e.target().id() === pathIds[i])).addClass('path')
      }
      for (const id of [slots.A, slots.B]) if (id) instance.getElementById(id).addClass('slot')
      if (selectedId && instance.getElementById(selectedId).nonempty()) {
        const node = instance.getElementById(selectedId)
        node.select()
        if (dimOthers) { instance.elements().addClass('dim'); node.closedNeighborhood().removeClass('dim'); node.connectedEdges().addClass('hl') }
      } else if (selectedEdgeId && instance.getElementById(selectedEdgeId).nonempty()) {
        const edge = instance.getElementById(selectedEdgeId)
        edge.select()
        if (dimOthers) { instance.elements().addClass('dim'); edge.connectedNodes().union(edge).removeClass('dim'); edge.addClass('hl') }
      }
    })
  }, [model, selectedId, selectedEdgeId, slots.A, slots.B, pathIds, sharedIds, dimOthers])

  // keyboard: arrows move between numbers in reading order, Enter opens, Escape clears
  const order = useMemo(() => [...model.nodes].sort((a, b) => (a.kind === 'subject' ? 0 : 1) - (b.kind === 'subject' ? 0 : 1) || a.label.localeCompare(b.label)).map(n => n.id), [model])
  function onKeyDown(event) {
    if (!order.length) return
    const at = order.indexOf(selectedId)
    if (event.key === 'ArrowDown' || event.key === 'ArrowRight') { event.preventDefault(); onSelectNode?.(order[Math.min(order.length - 1, at + 1)]) }
    else if (event.key === 'ArrowUp' || event.key === 'ArrowLeft') { event.preventDefault(); onSelectNode?.(order[Math.max(0, at - 1)]) }
    else if (event.key === 'Escape') { event.preventDefault(); onSelectNode?.(null) }
    else if (event.key === 'Enter' && selectedId) { event.preventDefault(); onSelectNode?.(selectedId) }
    else if (event.key === '0') { event.preventDefault(); cy.current?.fit(undefined, 40) }
  }
  return <div ref={host} className="link-chart" role="application" tabIndex={0} aria-label={`${label}. Arrow keys move between numbers, Escape clears, 0 fits the chart. A table of the same numbers is below.`} onKeyDown={onKeyDown} />
})
