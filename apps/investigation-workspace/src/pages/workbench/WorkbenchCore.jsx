import { lazy, Suspense, useCallback, useEffect, useId, useMemo, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { Camera, Check, Keyboard, Link2, Maximize2, Minus, MoveHorizontal, Plus, RotateCcw } from 'lucide-react'
import { formatNumber } from '../../lib/format.js'
import { clockOf, labelOf, shortNumber } from '../../lib/workbench/caseIndex.js'
import { LAYER_DOT, LAYER_LABELS } from '../../lib/workbench/answerMarks.js'
import { drawerOf, steppable } from '../../lib/workbench/inspector.js'
import { PANES, parseUrl, patchUrl, selectionFromToken, selectionToken } from '../../lib/workbench/urlState.js'
import { PLACEMENT, graphOf, lanesOf, mapOf, periodsOf, placementNeeded, positioner, suggestionsFor, visibleMarks, workingSetOf } from '../../lib/workbench/workingSet.js'
import { AskMenu, Badge, WbDrawer } from './WbDrawer.jsx'
import { WbGraph } from './WbGraph.jsx'
import { WbState } from './WbStates.jsx'
import { WbTable } from './WbTable.jsx'
import { WbTimeline } from './WbTimeline.jsx'
import '../../styles/workbench.css'

// THE LINK-ANALYSIS WORKBENCH, THE PICTURE (master plan 7.3; research and rules in docs/ux/workbench/RESEARCH_NOTE.md). One working set, coordinated panes, evidence one click from every mark. It is
// used in two places, which is the owner's decision of 2026-10-26: the CONNECTIONS page shows the whole case (every subscriber and the contacts they share, the cell sites they used, the days they were
// active) and has no question field; the ASK page asks the questions, and an answer that can be drawn (a window, a meeting, a successor, a move, a speed alert, a change of rhythm, a connection)
// carries this picture inside it, drawn from the rows the engine returned. The layout is the owner's choice: one pane at a time (links, places or table) with the timeline always underneath, and
// the evidence of the selected mark beside it (below it, inside an answer).
const GeoMap = lazy(() => import('../../components/GeoMap.jsx').then(m => ({ default: m.GeoMap })))
const MIN_IN = 280
const MAX_IN = 560
const PANE_LABEL = { links: 'Links', places: 'Places', table: 'Table' }
const SHORTCUTS = [['1  2  3', 'Links, places, table'], ['T', 'Table'], ['F', 'Fit the links'], ['[  ]', 'Previous, next mark'], ['Esc', 'Close the evidence'], ['?', 'This list']]

const storedWidth = () => { try { const v = Number(globalThis.localStorage?.getItem('nexusai.viewer.workbench.inspector')); return v >= MIN_IN && v <= MAX_IN ? v : 340 } catch { return 340 } }
const rangeFromUrl = (index, from, to) => {
  if (!from && !to) return null
  const a = from ? index.dayIndex(from) : 0
  const b = to ? index.dayIndex(to) : index.days - 1
  return a != null && b != null && b >= a ? [Math.max(0, a), Math.min(index.days - 1, b)] : null
}
function download(name, url) {
  const link = Object.assign(globalThis.document.createElement('a'), { href: url, download: name })
  globalThis.document.body.append(link); link.click(); link.remove()
}

function Pane({ title, meta, tools, children }) {
  return (
    <section className="wbp-pane" aria-label={title}>
      <header className="wbp-ph"><h3>{title}</h3>{meta ? <span>{meta}</span> : null}{tools ? <div className="wbp-ph__tools">{tools}</div> : null}</header>
      {children}
    </section>
  )
}

// The state in the address (the Connections page: the same link gives the same picture) or in the page (inside an answer, where an address belongs to the whole conversation)
export function WorkbenchUrl(props) {
  const [search, setSearch] = useSearchParams()
  const url = useMemo(() => parseUrl(search.toString()), [search])
  const patch = useCallback(entries => setSearch(previous => patchUrl(previous, entries), { replace: true }), [setSearch])
  return <Body {...props} url={url} patch={patch} embedded={false} />
}
export function WorkbenchLocal(props) {
  const [text, setText] = useState('')
  const url = useMemo(() => parseUrl(text), [text])
  const patch = useCallback(entries => setText(previous => patchUrl(previous, entries).toString()), [])
  return <Body {...props} url={url} patch={patch} embedded />
}

function Body({ caseId, index, places, answer = null, embedded, url, patch, onAsk, scopeNote = '' }) {
  const uid = useId()
  const rootRef = useRef(null)
  const graphRef = useRef(null)
  const [range, setRange] = useState(() => rangeFromUrl(index, url.from, url.to))
  const [rangeVersion, setRangeVersion] = useState(0)
  const [graphLayout, setGraphLayout] = useState('')
  const [labels, setLabels] = useState('smart')
  const [help, setHelp] = useState(false)
  const [wide, setWide] = useState(false)
  const [note, setNote] = useState('')
  const [inWidth, setInWidth] = useState(storedWidth)
  useEffect(() => { if (!note) return undefined; const t = globalThis.setTimeout(() => setNote(''), 2600); return () => globalThis.clearTimeout(t) }, [note])

  // the period in the address (debounced: the brush reports every step of a drag)
  useEffect(() => {
    if (embedded) return undefined
    const from = range ? index.dayLabel(range[0]) : ''
    const to = range ? index.dayLabel(range[1]) : ''
    if (from === url.from && to === url.to) return undefined
    const timer = globalThis.setTimeout(() => patch({ from, to }), 350)
    return () => globalThis.clearTimeout(timer)
  }, [range, index, patch, url.from, url.to, embedded])

  const known = useCallback(n => index.isSubject(n) || index.numberInfo(n).events > 0, [index])
  const layerKeys = useMemo(() => ['contacts', ...(answer?.layers || [])], [answer])
  const layers = useMemo(() => Object.fromEntries(layerKeys.map(k => [k, !url.off.includes(k)])), [layerKeys, url.off])
  const toggleLayer = key => patch({ off: layers[key] ? [...url.off, key] : url.off.filter(k => k !== key) })
  const needsPlacement = useMemo(() => placementNeeded(index), [index])
  const placement = url.place || (needsPlacement ? 'city' : 'recorded')
  const selection = useMemo(() => selectionFromToken(url.sel), [url.sel])
  const select = useCallback(next => patch({ sel: selectionToken(next) }), [patch])

  const focusNumbers = useMemo(() => (answer ? answer.numbers : url.targets).filter(known), [answer, url.targets, known])
  const focusSet = useMemo(() => new Set(focusNumbers.length ? focusNumbers : (answer?.answerNumbers || []).slice(0, 3)), [focusNumbers, answer])
  const members = useMemo(() => workingSetOf(index, { focus: focusNumbers, answerNumbers: (answer?.answerNumbers || []).filter(known), added: url.added }), [index, focusNumbers, answer, url.added, known])
  const graph = useMemo(() => graphOf(index, members, { focus: focusSet, answer, layers, range }), [index, members, focusSet, answer, layers, range])
  const position = useMemo(() => positioner(index, placement), [index, placement])
  const map = useMemo(() => mapOf(index, members, { answer, layers, range, position }), [index, members, answer, layers, range, position])
  const marks = useMemo(() => visibleMarks(answer, layers, range), [answer, layers, range])
  const lanes = useMemo(() => lanesOf(index, { focus: focusNumbers, members }), [index, focusNumbers, members])
  const dayLabels = useMemo(() => Array.from({ length: index.days }, (_, i) => index.dayLabel(i)), [index])
  const markLines = useMemo(() => marks.filter(m => m.kind === 'rhythm' && m.markLineDay != null).map(m => ({ day: m.markLineDay, label: 'rhythm changes' })), [marks])
  const excluded = useMemo(() => index.excludedFor(members), [index, members])
  const drawer = useMemo(() => drawerOf(selection, { index, answer, range }), [selection, index, answer, range])
  const selectedMark = selection?.type === 'mark' ? selection.id : null
  const highlight = useMemo(() => {
    if (!selection) return null
    if (selection.type === 'node') return { nodes: [selection.id], edges: [] }
    if (selection.type === 'edge') return { nodes: [selection.a, selection.b], edges: [`c:${[selection.a, selection.b].sort().join('|')}`] }
    if (selection.type === 'mark') { const m = answer?.marks.find(x => x.id === selection.id); return m ? { nodes: m.numbers, edges: (answer.links || []).filter(l => l.markIds.includes(m.id)).map(l => l.id) } : null }
    return null
  }, [selection, answer])
  const mapSelected = selection?.type === 'site' ? selection.id : selection?.type === 'mark' ? answer?.marks.find(m => m.id === selection.id)?.cells[0] || null : null
  const pane = PANES.includes(url.pane) ? url.pane : 'links'
  const graphMode = graphLayout || (focusSet.size ? 'focus' : 'overview')
  const periods = useMemo(() => periodsOf(index), [index])
  const periodOn = p => (p.range === null ? range === null : !!range && range[0] === p.range[0] && range[1] === p.range[1])
  const setPeriod = p => { setRange(p.range); setRangeVersion(v => v + 1) }
  const askItems = useMemo(() => suggestionsFor(index, { focus: focusNumbers, selected: selection?.type === 'node' ? selection.id : null }), [index, focusNumbers, selection])

  // the table twin: the answer's rows (those the period and the layers leave), or the subscribers of the case
  const table = useMemo(() => {
    if (answer && answer.table.columns.length) {
      const visible = answer.marks.length ? new Set(marks.map(m => m.row)) : null
      const rows = []; const targets = []
      answer.table.rows.forEach((cells, i) => {
        if (visible && !visible.has(i)) return
        rows.push(cells)
        const mark = answer.marks.find(m => m.row === i)
        const number = Object.values(answer.rows[i] || {}).map(String).find(v => /^\d{10,13}$/.test(v) && known(v))
        targets.push(mark ? { type: 'mark', id: mark.id } : number ? { type: 'node', id: number } : null)
      })
      return { title: 'The answer', columns: answer.table.columns, rows, targets, pick: selectedMark ? targets.findIndex(t => t?.id === selectedMark) : null, file: 'answer' }
    }
    const subs = index.subjectsByEvents
    return {
      title: 'Subscribers of this case', columns: ['Number', 'Carrier', 'Events', 'Contacts', 'First event', 'Last event'], file: 'subscribers',
      rows: subs.map(s => { const info = index.numberInfo(s.number); return [labelOf(s.number), info.carrier.id.replace(/^./, c => c.toUpperCase()), formatNumber(s.events), formatNumber(s.contacts), clockOf(s.first).slice(0, 10), clockOf(s.last).slice(0, 10)] }),
      targets: subs.map(s => ({ type: 'node', id: s.number })), pick: selection?.type === 'node' ? subs.findIndex(s => s.number === selection.id) : null,
    }
  }, [answer, marks, index, selection, selectedMark, known])

  const step = useCallback(direction => {
    const list = steppable(marks)
    if (!list.length) return
    const at = list.findIndex(m => m.id === selectedMark)
    select({ type: 'mark', id: list[(at + direction + list.length) % list.length].id })
  }, [marks, selectedMark, select])

  // keys: on the page for the Connections page; on the picture itself inside an answer (a thread holds many, and a key must not act on all of them)
  const onKey = useCallback(event => {
    const target = event.target
    if (/INPUT|TEXTAREA|SELECT/.test(target?.tagName || '') || target?.isContentEditable || event.metaKey || event.ctrlKey || event.altKey) return
    if (event.key === 'Escape') { select(null); setHelp(false) }
    else if (event.key === 't' || event.key === 'T') patch({ pane: pane === 'table' ? null : 'table' })
    else if (event.key === '1') patch({ pane: null })
    else if (event.key === '2') patch({ pane: 'places' })
    else if (event.key === '3') patch({ pane: 'table' })
    else if (event.key === 'f' || event.key === 'F') graphRef.current?.fit()
    else if (event.key === '[') step(-1)
    else if (event.key === ']') step(1)
    else if (event.key === '?') setHelp(v => !v)
  }, [step, select, patch, pane])
  useEffect(() => {
    if (embedded) return undefined
    globalThis.addEventListener('keydown', onKey)
    return () => globalThis.removeEventListener('keydown', onKey)
  }, [embedded, onKey])

  const startResize = event => {
    event.preventDefault()
    const startX = event.clientX; const start = inWidth
    const move = e => setInWidth(Math.min(MAX_IN, Math.max(MIN_IN, start + (startX - e.clientX))))
    const up = () => { globalThis.removeEventListener('pointermove', move); globalThis.removeEventListener('pointerup', up); setInWidth(w => { try { globalThis.localStorage?.setItem('nexusai.viewer.workbench.inspector', String(w)) } catch { /* private window */ } return w }) }
    globalThis.addEventListener('pointermove', move); globalThis.addEventListener('pointerup', up)
  }
  const copyLink = async () => {
    try { await navigator.clipboard.writeText(globalThis.location.href); setNote('Link copied. The pane, selection and period are in the address.') } catch { setNote('Copy was refused; the address bar holds the state.') }
  }
  const addContacts = () => {
    if (selection?.type !== 'node') return
    patch({ n: [...new Set([...url.added, ...index.strongest(selection.id, 3).filter(n => !members.includes(n))])] })
  }
  const find = useRef(null)
  const addFound = event => {
    event.preventDefault()
    const typed = String(find.current?.value || '').replace(/\D/g, '')
    const hit = index.find(typed)[0]
    if (!hit) { setNote('No number of this case contains those digits.'); return }
    if (!members.includes(hit)) patch({ n: [...url.added, hit] })
    select({ type: 'node', id: hit })
    if (find.current) find.current.value = ''
  }
  const askAbout = item => onAsk?.(item.question)
  const connectionsLink = focusNumbers.length ? `/cases/${encodeURIComponent(caseId)}/connections?t=${focusNumbers.join(',')}` : `/cases/${encodeURIComponent(caseId)}/connections`

  const paneHeight = embedded ? 340 : 400
  const graphTools = (
    <>
      <div className="wbp-seg wbp-seg--sm" role="group" aria-label="Layout of the links">
        {[['focus', 'Focus'], ['circle', 'Circle'], ['tree', 'Tree']].map(([id, label]) => <button key={id} type="button" aria-pressed={graphMode === id || (graphMode === 'overview' && id === 'focus')} onClick={() => setGraphLayout(id)}>{label}</button>)}
      </div>
      <div className="wbp-seg wbp-seg--sm wbp-seg--labels" role="group" aria-label="Labels">
        {[['smart', 'Key labels'], ['all', 'All'], ['none', 'None']].map(([id, label]) => <button key={id} type="button" aria-pressed={labels === id} onClick={() => setLabels(id)}>{label}</button>)}
      </div>
      <button type="button" className="wbp-icon" aria-label="Zoom out" title="Zoom out" onClick={() => graphRef.current?.zoom(0.8)}><Minus size={15} aria-hidden="true" /></button>
      <button type="button" className="wbp-icon" aria-label="Zoom in" title="Zoom in" onClick={() => graphRef.current?.zoom(1.25)}><Plus size={15} aria-hidden="true" /></button>
      <button type="button" className="wbp-icon" aria-label="Fit to the view (F)" title="Fit to the view (F)" onClick={() => graphRef.current?.fit()}><Maximize2 size={14} aria-hidden="true" /></button>
      <button type="button" className="wbp-icon" aria-label="Save the picture" title="Save the picture" onClick={() => { const png = graphRef.current?.png(); if (png) download('links.png', png) }}><Camera size={15} aria-hidden="true" /></button>
    </>
  )
  const graphPane = (
    <Pane title="Links" meta={`${graph.nodes.length} numbers, ${graph.links.length} links${range ? ' in the chosen period' : ''}`} tools={graphTools}>
      {graph.nodes.length ? (
        <WbGraph ref={graphRef} graph={graph} highlight={highlight} onSelect={next => select(next)} height={paneHeight} layout={graphMode} labels={labels} label="Links among the numbers of the working set" />
      ) : (
        <WbState compact id="empty" title="None of those numbers is in the contact records" body="The numbers named are not subscribers of this case and no subscriber contacted them, so there is nothing to draw. The table still lists what the answer returned." actions={[]} />
      )}
      <ul className="wbp-key" aria-label="Key">
        <li><i className="wbp-sw wbp-sw--sq" /> subscriber</li>
        <li><i className="wbp-sw wbp-sw--ci" /> other number</li>
        <li><i className="wbp-ln" /> contact, thicker for more events</li>
        {answer?.emph?.length ? <li><i className="wbp-ln wbp-ln--accent" /> named by the answer</li> : null}
        {answer?.links.some(l => l.kind === 'colocated') ? <li><i className="wbp-ln wbp-ln--dash" /> same place, inferred</li> : null}
        {answer?.links.some(l => l.kind === 'meeting' || l.kind === 'successor') ? <li><i className="wbp-ln wbp-ln--dot" /> candidate, not a finding</li> : null}
      </ul>
    </Pane>
  )
  const placesTools = needsPlacement ? (
    <div className="wbp-seg wbp-seg--sm wbp-seg--keep" role="group" aria-label="Where the sites are drawn">
      <button type="button" aria-pressed={placement === 'recorded'} onClick={() => patch({ place: 'recorded' })}>As recorded</button>
      <button type="button" aria-pressed={placement === 'city'} onClick={() => patch({ place: 'city' })}>Over {PLACEMENT.city}</button>
    </div>
  ) : null
  const placesPane = places.status !== 'ready' ? (
    <Pane title="Places"><WbState compact id="no-map" icon="no-map" title="The places could not be drawn" body={places.status === 'failed' ? `The request failed (${places.error?.reference || 'no reference'}). The links and the table are unaffected.` : `${places.reason} A map over part of the records would look like a finding and is not shown.`} actions={[{ label: 'Show the links', onClick: () => patch({ pane: null }) }]} /></Pane>
  ) : (
    <Pane title="Places" meta={`${map.sites.length} cell site${map.sites.length === 1 ? '' : 's'}: coverage areas, not positions`} tools={placesTools}>
      <Suspense fallback={<p className="chart-card__loading" role="status">Loading the map…</p>}>
        <GeoMap sites={map.sites} track={map.track} selectedId={mapSelected} onSelect={id => select(id ? { type: 'site', id } : null)} height={paneHeight} label="Cell sites of the working set" />
      </Suspense>
      {needsPlacement && placement === 'city' ? <p className="wbp-synthetic">Synthetic placement: the generator’s grid is drawn over {PLACEMENT.city} so that a basemap exists; the positions are not real and the distances between sites are the generator’s. Switch to “As recorded” to see the stored coordinates.</p> : null}
      {map.unlocated ? <p className="wbp-synthetic">{map.unlocated} cell site{map.unlocated === 1 ? ' has' : 's have'} no position in the records and {map.unlocated === 1 ? 'is' : 'are'} not drawn; the table lists {map.unlocated === 1 ? 'it' : 'them'}.</p> : null}
      {index.placesTruncated ? <p className="wbp-synthetic">Only the first {formatNumber(index.visits.length)} hourly visits of this case were read, so a map or a lane may be missing the latest hours.</p> : null}
    </Pane>
  )
  const tablePane = (
    <Pane title="Table" meta={table.title}>
      <WbTable columns={table.columns} rows={table.rows} label={table.title} pick={table.pick} fileName={table.file} onCopied={setNote} onRow={i => { const t = table.targets[i]; if (t) select(t) }} />
    </Pane>
  )
  const timelineTools = (
    <div className="wbp-seg wbp-seg--sm wbp-seg--keep" role="group" aria-label="Period">
      {periods.map(p => <button key={p.id} type="button" aria-pressed={periodOn(p)} onClick={() => setPeriod(p)}>{p.label}</button>)}
    </div>
  )
  const hasLane = lanes[0]?.values.some(v => v > 0)
  const timelinePane = (
    <Pane title="Timeline" meta={`${lanes[0]?.id === 'all' ? 'events of the subscribers at a cell site, a day' : 'events at a cell site, a day'}${range ? `, ${range[1] - range[0] + 1} days chosen` : ''}`} tools={timelineTools}>
      {(index.days > 0 && hasLane) || marks.some(m => m.day != null) ? (
        <WbTimeline lanes={lanes} labels={dayLabels} marks={marks} markLines={markLines} range={range} onRange={setRange} onSelectMark={m => select({ type: 'mark', id: m.id })} selectedMarkId={selectedMark} zoomKey={`${answer?.question || ''}:${rangeVersion}`} />
      ) : (
        <p className="wbp-hint">This case holds no hourly cell visits, so there is no daily activity to draw. Marks of an answer that carry a date still appear here.</p>
      )}
      <p className="wbp-hint wbp-hint--tl">Drag the handles under the chart to choose a period; every pane follows it. Click a dot to open its evidence. A day’s bar counts events that carry a cell site.</p>
    </Pane>
  )

  return (
    <div ref={rootRef} className={`wbp-root wbp-core${embedded ? ' wbp-core--embed' : ''}${wide ? ' wbp-embed--wide' : ''}`} style={{ '--wbp-in': `${inWidth}px` }} onKeyDown={embedded ? onKey : undefined} aria-label={embedded ? 'The answer on links, places and timeline' : undefined}>
      {embedded ? (
        <div className="wbp-strip">
          <Badge strength={answer.strength} />
          {answer.values.length ? <ul className="wbp-values" aria-label="Values the engine used">{answer.values.map(v => <li key={v.label}><b>{v.label}</b>{v.value}, {v.source}</li>)}</ul> : null}
          <span className="wbp-strip__acts">
            {focusNumbers.length ? <Link className="wbp-btn wbp-btn--text" to={connectionsLink}>All connections of {focusNumbers.length > 1 ? 'these numbers' : labelOf(focusNumbers[0])}</Link> : null}
            <button type="button" className="wbp-btn wbp-btn--text" aria-pressed={wide} onClick={() => setWide(v => !v)}><MoveHorizontal size={13} aria-hidden="true" /> {wide ? 'Narrow' : 'Widen'}</button>
          </span>
        </div>
      ) : null}
      <section className="wbp-frame" aria-label="Workbench">
        <div className="wbp-bar">
          <div className="wbp-set" aria-label="Working set">
            <span className="wbp-set__label">Working set</span>
            <ul className="wbp-set__members">
              {members.slice(0, embedded ? 6 : 8).map(n => <li key={n} title={labelOf(n)} className={focusSet.has(n) ? 'is-focus' : undefined}><button type="button" onClick={() => select({ type: 'node', id: n })}>{shortNumber(n)}</button></li>)}
            </ul>
            {members.length > (embedded ? 6 : 8) ? <span className="wbp-set__more">+{members.length - (embedded ? 6 : 8)}</span> : null}
            <button type="button" className="wbp-btn wbp-btn--text" onClick={addContacts} disabled={selection?.type !== 'node'} title={selection?.type === 'node' ? `Add the strongest contacts of ${labelOf(selection.id)}` : 'Select a number first'}>Add contacts</button>
            {url.added.length ? <button type="button" className="wbp-btn wbp-btn--text" onClick={() => patch({ n: null })}><RotateCcw size={12} aria-hidden="true" /> Reset</button> : null}
            {!embedded ? <form className="wbp-find" onSubmit={addFound}><input ref={find} type="text" inputMode="numeric" placeholder="Find a number" aria-label="Find a number of this case by its digits" /></form> : null}
            {excluded.length ? <span className="wbp-hint" title={excluded.map(e => `${e.number} (${e.why})`).join(', ')}>{excluded.length} excluded, not hidden</span> : null}
          </div>
          <div className="wbp-tools">
            {layerKeys.length > 1 ? (
              <div className="wbp-layers" role="group" aria-label="Analyses on the canvas">
                {layerKeys.map(k => <button key={k} type="button" className="wbp-layer" aria-pressed={layers[k]} onClick={() => toggleLayer(k)}><i className={`wbp-dm wbp-dm--${LAYER_DOT[k]}`} />{LAYER_LABELS[k]}</button>)}
              </div>
            ) : null}
            {!embedded ? <AskMenu items={askItems} onAsk={askAbout} label="Ask about this case" /> : null}
            {!embedded ? <button type="button" className="wbp-btn" onClick={copyLink}><Link2 size={14} aria-hidden="true" /> Copy link</button> : null}
            <button type="button" className="wbp-icon" aria-label="Keyboard shortcuts (?)" title="Keyboard shortcuts (?)" aria-expanded={help} onClick={() => setHelp(v => !v)}><Keyboard size={16} aria-hidden="true" /></button>
          </div>
        </div>
        {help ? (
          <div className="wbp-help" role="dialog" aria-label="Keyboard shortcuts">
            {SHORTCUTS.map(([k, label]) => <span key={k}><kbd>{k}</kbd> {label}</span>)}
          </div>
        ) : null}
        <div className="wbp-body">
          <div className="wbp-canvas">
            <div className="wbp-tabs" role="tablist" aria-label="Pane">
              {PANES.map((id, i) => (
                <button key={id} role="tab" type="button" id={`${uid}-tab-${id}`} aria-selected={pane === id} aria-controls={`${uid}-pane`} onClick={() => patch({ pane: id === 'links' ? null : id })}>
                  {PANE_LABEL[id]}<span className="wbp-tabs__n">{id === 'links' ? graph.nodes.length : id === 'places' ? map.sites.length : table.rows.length}</span>{embedded ? null : <kbd aria-hidden="true">{i + 1}</kbd>}
                </button>
              ))}
            </div>
            <div id={`${uid}-pane`} role="tabpanel" aria-labelledby={`${uid}-tab-${pane}`}>
              {pane === 'links' ? graphPane : pane === 'places' ? placesPane : tablePane}
            </div>
            {timelinePane}
          </div>
          <div className="wbp-side">
            {!embedded ? (
              <div className="wbp-resizer" role="separator" aria-orientation="vertical" aria-label="Resize the evidence panel" tabIndex={0} onPointerDown={startResize} onDoubleClick={() => setInWidth(340)}
                onKeyDown={e => { if (e.key === 'ArrowLeft') setInWidth(w => Math.min(MAX_IN, w + 20)); if (e.key === 'ArrowRight') setInWidth(w => Math.max(MIN_IN, w - 20)) }} />
            ) : null}
            <WbDrawer drawer={drawer} onClose={() => select(null)} onCopied={setNote} onRow={target => target && select(target)} onAsk={askAbout} />
          </div>
        </div>
      </section>
      {!embedded ? (
        <p className="wbp-foot">
          {index.truncated ? <span>Only the busiest {formatNumber(index.subjects.length)} subscribers and their strongest {index.perSubject} contacts each were read. </span> : null}
          {index.connectionCaveats[0]}{scopeNote ? ` ${scopeNote}` : ''}
          <span className="wbp-foot__keys">{SHORTCUTS.slice(0, 4).map(([k, l]) => <span key={k}><kbd>{k}</kbd> {l}</span>)}</span>
        </p>
      ) : null}
      {note ? <div className="wbp-toast" role="status"><Check size={14} aria-hidden="true" /> {note}</div> : null}
    </div>
  )
}
