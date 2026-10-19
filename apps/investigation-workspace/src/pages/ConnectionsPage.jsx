import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useParams, useSearchParams } from 'react-router-dom'
import { Camera, Maximize2 } from 'lucide-react'
import { CaseShell } from '../components/CaseShell.jsx'
import { PageHeader } from '../components/PageHeader.jsx'
import { Skeleton } from '../components/Skeleton.jsx'
import { EmptyState } from '../components/AnalystComponents.jsx'
import { CarrierBadge } from '../components/CarrierBadge.jsx'
import { getConnections } from '../lib/apiClient.js'
import { collapse, communities, emptyFilters, filterModel, filtersFromSearch, filtersToSearch, layoutGraph, buildModel, sharedContacts, shortestPath } from '../lib/connectionsModel.js'
import { CARRIERS, probeOfficialIcons } from '../lib/carriers.js'
import { formatNumber } from '../lib/format.js'
import { ConnectionsFilterBar } from './connections/ConnectionsFilterBar.jsx'
import { ConnectionsPanel } from './connections/ConnectionsPanel.jsx'
import { ConnectionsTable } from './connections/ConnectionsTable.jsx'

// Cytoscape is fetched only when this page is opened.
const LinkChart = lazy(() => import('../components/LinkChart.jsx').then(m => ({ default: m.LinkChart })))

const OVERVIEW_ABOVE = 1500

function useConnections(caseId) {
  const [state, setState] = useState({ status: 'loading', data: null, error: null })
  useEffect(() => {
    const controller = new AbortController()
    setState({ status: 'loading', data: null, error: null })
    Promise.all([getConnections({ caseId, perSubject: 60, signal: controller.signal }), probeOfficialIcons()])
      .then(([data]) => setState({ status: data?.available ? 'ready' : 'unavailable', data, error: null }))
      .catch(error => { if (error?.name !== 'AbortError') setState({ status: 'failed', data: null, error }) })
    return () => controller.abort()
  }, [caseId])
  return state
}

function download(name, blobOrUrl) {
  const url = typeof blobOrUrl === 'string' ? blobOrUrl : URL.createObjectURL(blobOrUrl)
  const link = Object.assign(globalThis.document.createElement('a'), { href: url, download: name })
  globalThis.document.body.append(link); link.click(); link.remove()
  if (typeof blobOrUrl !== 'string') globalThis.setTimeout(() => URL.revokeObjectURL(url), 1000)
}

function Legend({ base }) {
  const present = [...new Set(base.nodes.map(n => n.carrier))].map(id => CARRIERS[id]).filter(Boolean)
  return (
    <div className="cx-legend" aria-label="Legend">
      <span className="cx-legend__group">Carrier of the SIM {present.map(c => <span key={c.id} className="cx-legend__item"><CarrierBadge carrier={c.id} size={15} showName /></span>)}</span>
      <span className="cx-legend__item"><i className="cx-sw cx-sw--square" />subscriber (badge = carrier)</span>
      <span className="cx-legend__item"><i className="cx-sw cx-sw--circle" />other number (ring = carrier by range)</span>
      <span className="cx-legend__item"><i className="cx-sw cx-sw--triangle" />everyone contacts it</span>
      <span className="cx-legend__item"><i className="cx-sw cx-sw--diamond" />service or short code</span>
      <span className="cx-legend__item"><i className="cx-ln cx-ln--solid" />contact, thicker = more events</span>
      <span className="cx-legend__item"><i className="cx-ln cx-ln--dashed" />shared contacts (inferred)</span>
    </div>
  )
}

// the same page frame as every other case page (padding, width, the skip-link target)
function Frame({ caseId, header, children }) {
  return <CaseShell caseId={caseId}><main id="workspace-main" className="catalog-page cx-page" tabIndex={-1}>{header}{children}</main></CaseShell>
}

// The panel offers about twenty number buttons before the table: a keyboard user can jump over them (the accessibility check of 2026-10-16 found the 20 stops)
function skipToTable() {
  const table = globalThis.document.getElementById('cx-table')
  if (!table) return
  table.scrollIntoView?.({ block: 'start' })
  table.focus()
}

export default function ConnectionsPage() {
  const { id: caseId } = useParams()
  const [search, setSearch] = useSearchParams()
  const state = useConnections(caseId)
  const chart = useRef(null)
  const [copied, setCopied] = useState(false)
  const [expanded, setExpanded] = useState(() => new Set())
  const [selectedEdgeId, setSelectedEdgeId] = useState(null)
  const filters = useMemo(() => filtersFromSearch(search.toString()), [search])
  const selectedId = search.get('n') || null
  const slots = useMemo(() => ({ A: search.get('a') || null, B: search.get('b') || null }), [search])

  const base = useMemo(() => (state.status === 'ready' ? buildModel(state.data) : null), [state])
  const positions = useMemo(() => (base ? layoutGraph(base.nodes, base.edges) : null), [base])
  const shown = useMemo(() => (base ? filterModel(base, filters) : null), [base, filters])

  // Overview mode: above about 1,500 numbers the contacts are grouped by community and a group opens on a click (roadmap 7.7, V5)
  const overview = useMemo(() => {
    if (!shown || shown.nodes.length <= OVERVIEW_ABOVE) return null
    return collapse(shown.nodes, shown.edges, communities(shown.nodes, shown.edges), { expanded })
  }, [shown, expanded])
  const drawn = useMemo(() => (overview ? { ...shown, nodes: overview.nodes, edges: overview.edges } : shown), [shown, overview])
  const drawnPositions = useMemo(() => {
    if (!drawn || !positions) return null
    if (!overview) return positions
    const out = new Map(positions)
    for (const g of overview.groups) {
      const pts = g.members.map(id => positions.get(id)).filter(Boolean)
      out.set(g.id, { x: pts.reduce((s, p) => s + p.x, 0) / (pts.length || 1), y: pts.reduce((s, p) => s + p.y, 0) / (pts.length || 1) })
    }
    return out
  }, [drawn, positions, overview])

  const shared = useMemo(() => (shown && slots.A && slots.B ? sharedContacts(shown, slots.A, slots.B) : []), [shown, slots])
  const path = useMemo(() => (shown && slots.A && slots.B ? shortestPath(shown, slots.A, slots.B) : null), [shown, slots])
  const selectedEdge = useMemo(() => (shown && selectedEdgeId ? shown.edges.find(e => e.id === selectedEdgeId) || null : null), [shown, selectedEdgeId])

  const patch = useCallback(entries => {
    setSearch(previous => {
      const next = new URLSearchParams(previous)
      for (const [key, value] of Object.entries(entries)) { if (value) next.set(key, value); else next.delete(key) }
      return next
    }, { replace: true })
  }, [setSearch])
  const setFilters = useCallback(next => {
    const f = next || emptyFilters()
    setSearch(previous => {
      const keep = new URLSearchParams(previous)
      for (const key of ['t', 'c', 'from', 'to', 'min', 'hide', 'show']) keep.delete(key)
      const merged = new URLSearchParams(filtersToSearch(f, keep.toString()))
      return merged
    }, { replace: true })
  }, [setSearch])
  const select = useCallback(id => {
    setSelectedEdgeId(null)
    if (id && overview?.groups.some(g => g.id === id)) { setExpanded(previous => new Set(previous).add(id)); return }
    patch({ n: id })
  }, [patch, overview])
  const setSlot = useCallback((which, id) => patch({ [which.toLowerCase()]: id }), [patch])
  const copyLink = useCallback(async () => {
    try { await globalThis.navigator.clipboard.writeText(globalThis.location.href); setCopied(true); globalThis.setTimeout(() => setCopied(false), 1800) } catch { setCopied(false) }
  }, [])
  const copyNumber = useCallback(async id => { try { await globalThis.navigator.clipboard.writeText(id) } catch { /* the number is on screen */ } }, [])

  const stats = state.status === 'ready' ? state.data.totals : {}
  const header = (
    <PageHeader
      title="Connections"
      description="Who is in contact with whom, and what they share."
      breadcrumbs={[{ label: 'Connections' }]}
      meta={state.status === 'ready' ? [
        { label: 'Subscribers', value: formatNumber(stats.subjects || 0) },
        { label: 'Numbers in contact', value: formatNumber(stats.counterparties || 0) },
        { label: 'Events', value: formatNumber(stats.events || 0) },
        { label: 'Time zone', value: state.data.time_zone },
        ...(state.data.truncated ? [{ label: 'Shown', value: `the strongest ${state.data.per_subject} contacts of each subscriber` }] : []),
      ] : []}
      actions={state.status === 'ready' ? (
        <>
          <button type="button" className="cx-button" onClick={() => chart.current?.fit()}><Maximize2 aria-hidden="true" />Fit</button>
          <button type="button" className="cx-button" onClick={() => { const png = chart.current?.png(); if (png) download('connections.png', png) }}><Camera aria-hidden="true" />Save picture</button>
        </>
      ) : null}
    />
  )

  if (state.status === 'loading') return <Frame caseId={caseId} header={header}><Skeleton lines={8} label="Reading the connections of this case" /></Frame>
  if (state.status === 'failed') return <Frame caseId={caseId} header={header}><EmptyState kind="unavailable" label="The connections could not be read" description={`The request failed (${state.error?.reference || 'no reference'}). Nothing was changed. Try again, or ask the question in Ask this case.`} /></Frame>
  if (state.status === 'unavailable') {
    return (
      <Frame caseId={caseId} header={header}>
        <EmptyState kind="unavailable" label="This case cannot be drawn as connections yet" description={`${state.data?.reason || 'The case has no typed call records.'} A chart over part of the records would look like a finding and is not shown. Ask this case still answers from the records it can read.`} />
      </Frame>
    )
  }
  return (
    <Frame caseId={caseId} header={header}>
      <button type="button" className="skip-link" onClick={skipToTable}>Skip to the table of the same numbers</button>
      <ConnectionsFilterBar base={base} shown={shown} filters={filters} onChange={setFilters} onCopyLink={copyLink} copied={copied} />
      <div className="cx-split">
        <section className="cx-canvas" aria-label="Link chart">
          {drawn.nodes.length ? (
            <Suspense fallback={<p className="chart-card__loading" role="status">Loading the chart…</p>}>
              <LinkChart ref={chart} model={drawn} positions={drawnPositions} selectedId={selectedId} selectedEdgeId={selectedEdgeId} slots={slots} pathIds={path || []} sharedIds={shared.map(s => s.id)} onSelectNode={select} onSelectEdge={id => { patch({ n: null }); setSelectedEdgeId(id) }} />
            </Suspense>
          ) : <EmptyState kind="complete-zero" label="Nothing matches these filters" description="Widen the targets, the carriers or the period, or reset the filters." />}
          <Legend base={base} />
          {overview?.groups.length ? <p className="cx-muted cx-overview" role="status">Overview: {formatNumber(shown.nodes.length)} numbers are drawn as {formatNumber(overview.groups.length)} groups of contacts; select a group to open it.</p> : null}
          {state.data.caveats?.length ? <ul className="cx-caveats">{state.data.caveats.map(c => <li key={c}>{c}</li>)}</ul> : null}
        </section>
        <ConnectionsPanel caseId={caseId} model={shown} base={base} selectedId={selectedId} selectedEdge={selectedEdge} slots={slots} shared={shared} path={path} onSelect={select} onSlot={setSlot} onClear={() => { patch({ n: null }); setSelectedEdgeId(null) }} onCopy={copyNumber} />
      </div>
      <ConnectionsTable model={drawn} selectedId={selectedId} onSelect={select} />
    </Frame>
  )
}
