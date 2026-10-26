import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Check, ChevronDown, Keyboard, Link2, Maximize2, Minus, Plus } from 'lucide-react'
import { AppShell } from '../components/CaseShell.jsx'
import { GeoMap } from '../components/GeoMap.jsx'
import { CAVEATS, DATA, PERIODS, PRESETS, alertsOf, dayLabel, drawerFor, excludedFor, graphOf, laneOf, mapOf, marksOf, meetingsOf, moveOf, nf, rhythmOf, short, tableFor, windowsOf, workingSet } from '../lib/workbenchProof.js'
import { WbGraph } from './workbench/WbGraph.jsx'
import { WbTimeline } from './workbench/WbTimeline.jsx'
import { WbDrawer, Badge } from './workbench/WbDrawer.jsx'
import { WbTable } from './workbench/WbTable.jsx'
import { WbState } from './workbench/WbStates.jsx'
import '../styles/workbench.css'

// THE LINK-ANALYSIS WORKBENCH, DIRECTION PROOF (docs/ux/workbench/RESEARCH_NOTE.md; master plan 7.3 and 7.6). Two ways to arrange the same thing, on the application's own tokens and both of its themes,
// over a SYNTHETIC case (development case 2 of the generator): nothing here is read from a live case and nothing is sent anywhere.
//   /design/workbench?layout=a   links and places side by side, the timeline under them, the evidence of the selected mark on the right
//   /design/workbench?layout=b   one pane at a time (links, places or table) with the timeline always underneath
// The rules it follows are written, with their sources, in the research note: question first; overview, then zoom and filter, then details on demand; a small working set that grows deliberately;
// coordinated panes (a mark in one lights it in all, the period filters all); every mark opens its evidence; the table twin one key away; colour never the only encoding; the caveat in the engine's words.
const LAYER_LABELS = { contacts: 'Contacts', windows: 'Same place', meetings: 'Possible meetings', successors: 'Candidate successors', moves: 'Moves', alerts: 'Impossible speed', rhythm: 'Change of rhythm' }
const LAYER_DOT = { contacts: 'c', windows: '2', meetings: '3', successors: '1', moves: '1', alerts: 'f', rhythm: '4' }
const MIN_IN = 280
const MAX_IN = 560

function selectionOfMark(mark, preset) {
  const [a, b] = preset.focus
  if (mark.kind === 'window') return { type: 'window', index: Math.max(0, windowsOf(a, b).findIndex(w => `window:${w.start}` === mark.id)), a, b }
  if (mark.kind === 'meeting') return { type: 'meeting', index: Math.max(0, meetingsOf(a, b).findIndex(w => `meeting:${w.start}` === mark.id)), a, b }
  if (mark.kind === 'move') return { type: 'move', number: a }
  if (mark.kind === 'alert') return { type: 'alert', number: a, index: Math.max(0, alertsOf(a).findIndex(x => `alert:${x.from_time}` === mark.id)) }
  if (mark.kind === 'rhythm') return { type: 'rhythm', number: a }
  if (mark.kind === 'silent') return { type: 'node', id: preset.focus[0] }
  return null
}
function markIdOf(selection, preset) {
  if (!selection) return null
  const [a, b] = preset.focus
  if (selection.type === 'window') return `window:${windowsOf(a, b)[selection.index]?.start}`
  if (selection.type === 'meeting') return `meeting:${meetingsOf(a, b)[selection.index]?.start}`
  if (selection.type === 'move') return `move:${selection.number}`
  if (selection.type === 'alert') return `alert:${alertsOf(selection.number)[selection.index || 0]?.from_time}`
  if (selection.type === 'rhythm') return `rhythm:${selection.number}`
  return null
}
function cellOfSelection(selection, preset) {
  if (!selection) return null
  const [a, b] = preset.focus
  if (selection.type === 'site') return selection.id
  if (selection.type === 'window') return windowsOf(a, b)[selection.index]?.cell_key || null
  if (selection.type === 'meeting') return meetingsOf(a, b)[selection.index]?.cell_key || null
  if (selection.type === 'move') return moveOf(selection.number)?.to_cell || null
  if (selection.type === 'alert') return alertsOf(selection.number)[selection.index || 0]?.from_cell || null
  return null
}
const storedWidth = () => { try { const v = Number(globalThis.localStorage?.getItem('wbp.inspector')); return v >= MIN_IN && v <= MAX_IN ? v : 340 } catch { return 340 } }

function Pane({ title, meta, tools, children }) {
  return (
    <section className="wbp-pane" aria-label={title}>
      <header className="wbp-ph"><h3>{title}</h3>{meta ? <span>{meta}</span> : null}{tools ? <div className="wbp-ph__tools">{tools}</div> : null}</header>
      {children}
    </section>
  )
}

function Examples({ current, onPick }) {
  const ref = useRef(null)
  return (
    <details className="wbp-menu" ref={ref}>
      <summary>Examples <ChevronDown size={14} aria-hidden="true" /></summary>
      <ul className="wbp-menu__list" role="listbox" aria-label="Example questions">
        {PRESETS.map(p => (
          <li key={p.id} role="option" aria-selected={p.id === current}>
            <button type="button" aria-current={p.id === current} onClick={() => { onPick(p.id); if (ref.current) ref.current.open = false }}><b>{p.label}</b><span>{p.question.length > 78 ? `${p.question.slice(0, 76)}…` : p.question}</span></button>
          </li>
        ))}
      </ul>
    </details>
  )
}

const SHORTCUTS = [['T', 'Table'], ['F', 'Fit the links'], ['[  ]', 'Previous, next mark'], ['Esc', 'Close the evidence'], ['?', 'This list']]

export default function WorkbenchProofPage() {
  const [params, setParams] = useSearchParams()
  const layout = params.get('layout') === 'b' ? 'b' : 'a'
  const demoState = params.get('state')
  const preset = PRESETS.find(p => p.id === params.get('q')) || PRESETS[0]
  const [layers, setLayers] = useState(preset.layers)
  const [selection, setSelection] = useState(preset.select)
  const [range, setRange] = useState(null)
  const [rangeVersion, setRangeVersion] = useState(0)
  const [expanded, setExpanded] = useState([])
  const [lens, setLens] = useState('graph')
  const [twin, setTwin] = useState(false)
  const [note, setNote] = useState('')
  const [help, setHelp] = useState(false)
  const [graphLayout, setGraphLayout] = useState('focus')
  const [labels, setLabels] = useState('smart')
  const [inWidth, setInWidth] = useState(storedWidth)
  const graphRef = useRef(null)

  useEffect(() => { setLayers(preset.layers); setSelection(preset.select); setRange(null); setExpanded([]) }, [preset.id]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { if (!note) return undefined; const t = globalThis.setTimeout(() => setNote(''), 2600); return () => globalThis.clearTimeout(t) }, [note])

  const members = useMemo(() => workingSet(preset, expanded), [preset, expanded])
  const graph = useMemo(() => graphOf(preset, members, layers, range), [preset, members, layers, range])
  const map = useMemo(() => mapOf(preset, layers, range), [preset, layers, range])
  const marks = useMemo(() => marksOf(preset, layers, null), [preset, layers])
  const drawer = useMemo(() => {
    const d = drawerFor(selection, preset, range)
    return d ? { ...d, caveats: [CAVEATS[d.caveat]], ask: [{ id: 'ask', label: 'Ask about this', question: d.title }] } : null
  }, [selection, preset, range])
  const table = useMemo(() => tableFor(selection ? { ...preset, select: selection } : preset, layers, range), [selection, preset, layers, range])
  const excluded = useMemo(() => excludedFor(members), [members])
  const events = useMemo(() => nf.format(preset.focus.reduce((s, n) => s + (DATA.subjects.find(x => x.number === n)?.events || 0), 0)), [preset])
  const setQuestion = id => setParams(prev => { const next = new URLSearchParams(prev); next.set('q', id); next.delete('state'); return next }, { replace: true })
  const setLayout = value => setParams(prev => { const next = new URLSearchParams(prev); next.set('layout', value); return next }, { replace: true })
  const onSelectMark = useCallback(mark => setSelection(selectionOfMark(mark, preset)), [preset])
  const mapSelected = cellOfSelection(selection, preset)
  const toggleLayer = key => setLayers(prev => ({ ...prev, [key]: !prev[key] }))
  const expand = () => setExpanded(e => [...e, ...members.filter(m => !preset.focus.includes(m)).slice(0, 1)])
  const setPeriod = p => { setRange(p.range()); setRangeVersion(v => v + 1) }
  const periodOn = p => { const r = p.range(); return (r === null && range === null) || (r && range && r[0] === range[0] && r[1] === range[1]) }
  const copyLink = async () => {
    try { await navigator.clipboard.writeText(globalThis.location.href); setNote('Link copied. The question, layout and period are in the address.') } catch { setNote('Copy was refused; the address bar holds the state.') }
  }
  const step = useCallback(direction => {
    if (!marks.length) return
    const current = marks.findIndex(m => m.id === markIdOf(selection, preset))
    const next = marks[(current + direction + marks.length) % marks.length]
    setSelection(selectionOfMark(next, preset))
  }, [marks, selection, preset])

  useEffect(() => {
    const key = event => {
      if (/INPUT|TEXTAREA|SELECT/.test(event.target?.tagName || '') || event.metaKey || event.ctrlKey || event.altKey) return
      if (event.key === 'Escape') { setSelection(null); setHelp(false) }
      else if (event.key === 't' || event.key === 'T') setTwin(v => !v)
      else if (event.key === 'f' || event.key === 'F') graphRef.current?.fit()
      else if (event.key === '[') step(-1)
      else if (event.key === ']') step(1)
      else if (event.key === '?') setHelp(v => !v)
    }
    globalThis.addEventListener('keydown', key)
    return () => globalThis.removeEventListener('keydown', key)
  }, [step])

  const startResize = event => {
    event.preventDefault()
    const startX = event.clientX; const start = inWidth
    const move = e => setInWidth(Math.min(MAX_IN, Math.max(MIN_IN, start + (startX - e.clientX))))
    const up = () => { globalThis.removeEventListener('pointermove', move); globalThis.removeEventListener('pointerup', up); setInWidth(w => { try { globalThis.localStorage?.setItem('wbp.inspector', String(w)) } catch { /* private window */ } return w }) }
    globalThis.addEventListener('pointermove', move); globalThis.addEventListener('pointerup', up)
  }

  const graphTools = (
    <>
      <div className="wbp-seg wbp-seg--sm" role="group" aria-label="Layout of the links">
        {[['focus', 'Focus'], ['circle', 'Circle'], ['tree', 'Tree']].map(([id, label]) => <button key={id} type="button" aria-pressed={graphLayout === id} onClick={() => setGraphLayout(id)}>{label}</button>)}
      </div>
      <div className="wbp-seg wbp-seg--sm wbp-seg--labels" role="group" aria-label="Labels">
        {[['smart', 'Key labels'], ['all', 'All'], ['none', 'None']].map(([id, label]) => <button key={id} type="button" aria-pressed={labels === id} onClick={() => setLabels(id)}>{label}</button>)}
      </div>
      <button type="button" className="wbp-icon" aria-label="Zoom out" title="Zoom out" onClick={() => graphRef.current?.zoom(0.8)}><Minus size={15} aria-hidden="true" /></button>
      <button type="button" className="wbp-icon" aria-label="Zoom in" title="Zoom in" onClick={() => graphRef.current?.zoom(1.25)}><Plus size={15} aria-hidden="true" /></button>
      <button type="button" className="wbp-icon" aria-label="Fit to the view (F)" title="Fit to the view (F)" onClick={() => graphRef.current?.fit()}><Maximize2 size={14} aria-hidden="true" /></button>
    </>
  )
  const graphPane = height => (
    <Pane title="Links" meta={`${graph.nodes.length} numbers, ${graph.links.length} links${range ? ' in the chosen period' : ''}`} tools={graphTools}>
      <WbGraph ref={graphRef} graph={graph} selection={selection} onSelect={setSelection} height={height} layout={graphLayout} labels={labels} />
      <ul className="wbp-key" aria-label="Key">
        <li><i className="wbp-sw wbp-sw--sq" /> subscriber</li>
        <li><i className="wbp-sw wbp-sw--ci" /> other number</li>
        <li><i className="wbp-ln" /> contact, thicker for more events</li>
        <li><i className="wbp-ln wbp-ln--dash" /> same place, inferred</li>
        <li><i className="wbp-ln wbp-ln--dot" /> candidate, not a finding</li>
      </ul>
    </Pane>
  )
  const mapPane = height => (
    <Pane title="Places" meta={`${map.sites.length} cell sites: coverage areas, not positions`}>
      <GeoMap sites={map.sites} track={map.track} selectedId={mapSelected} onSelect={id => setSelection(id ? { type: 'site', id } : null)} height={height} label="Cell sites of the working set" />
      <p className="wbp-synthetic">Synthetic placement: the generator's grid is drawn over Lahore so that a basemap exists; the positions are not real.</p>
    </Pane>
  )
  const timelineTools = (
    <div className="wbp-seg wbp-seg--sm wbp-seg--keep" role="group" aria-label="Period">
      {PERIODS.map(p => <button key={p.id} type="button" aria-pressed={periodOn(p)} onClick={() => setPeriod(p)}>{p.label}</button>)}
    </div>
  )
  const dayLabels = useMemo(() => Array.from({ length: DATA.days }, (_, i) => dayLabel(i)), [])
  const markLines = useMemo(() => { const r = marks.find(m => m.kind === 'rhythm') ? rhythmOf(preset.focus[0]) : null; return r ? [{ day: dayLabels.indexOf(r.first_day), label: 'rhythm changes' }] : [] }, [marks, preset, dayLabels])
  const timelinePane = compact => {
    const lanes = compact
      ? [{ id: 'all', title: preset.focus.length > 1 ? 'All' : short(preset.focus[0]), values: dayLabels.map((_, i) => preset.focus.reduce((s, n) => s + (laneOf(n)[i] || 0), 0)) }]
      : preset.focus.slice(0, 3).map(n => ({ id: n, title: short(n), values: laneOf(n) }))
    return (
      <Pane title="Timeline" meta={`events a day${range ? `, ${range[1] - range[0] + 1} days chosen` : ''}`} tools={timelineTools}>
        <WbTimeline lanes={lanes} labels={dayLabels} marks={marks} markLines={markLines} range={range} onRange={setRange} onSelectMark={onSelectMark} selectedMarkId={markIdOf(selection, preset)} zoomKey={`${preset.id}:${compact}:${rangeVersion}`} />
        <p className="wbp-hint wbp-hint--tl">Drag the handles under the chart to choose a period; every pane follows it. Click a dot to open its evidence.</p>
      </Pane>
    )
  }

  const canvas = demoState && demoState !== 'on' ? <WbState id={demoState} /> : layout === 'a' ? (
    <>
      <div className="wbp-row">{graphPane(320)}{mapPane(320)}</div>
      {timelinePane(false)}
      {twin ? <Pane title="Table" meta={table.title}><WbTable columns={table.columns} rows={table.rows} label={table.title} onCopied={setNote} /></Pane> : null}
    </>
  ) : (
    <>
      <div className="wbp-tabs" role="tablist" aria-label="Pane">
        {[['graph', 'Links'], ['map', 'Places'], ['table', 'Table']].map(([id, label]) => <button key={id} role="tab" type="button" aria-selected={lens === id} onClick={() => setLens(id)}>{label}</button>)}
      </div>
      {lens === 'graph' ? graphPane(390) : lens === 'map' ? mapPane(390) : <Pane title="Table" meta={table.title}><WbTable columns={table.columns} rows={table.rows} label={table.title} onCopied={setNote} /></Pane>}
      {timelinePane(true)}
    </>
  )

  return (
    <AppShell>
      <main id="workspace-main" className={`wbp wbp--${layout}`} tabIndex={-1} style={{ '--wbp-in': `${inWidth}px` }}>
        <header className="wbp-head">
          <div>
            <p className="wbp-crumb">Design / Workbench</p>
            <h1>Link-analysis workbench</h1>
            <p className="wbp-sub">Direction proof on a synthetic case: the same question, arranged two ways. Nothing here is a live case.</p>
          </div>
          <div className="wbp-seg" role="group" aria-label="Layout">
            <button type="button" aria-pressed={layout === 'a'} onClick={() => setLayout('a')}>A &middot; Side by side</button>
            <button type="button" aria-pressed={layout === 'b'} onClick={() => setLayout('b')}>B &middot; One pane</button>
          </div>
        </header>

        <section className="wbp-ask" aria-label="Question">
          <div className="wbp-q">
            <span className="wbp-q__label">Question</span>
            <p className="wbp-q__text" title={preset.question}>{preset.question}</p>
            <Examples current={preset.id} onPick={setQuestion} />
          </div>
          <div className="wbp-answer-row">
            <h2 className="wbp-answer">{preset.sentence()}</h2>
            <Badge strength={preset.strength} />
          </div>
          <ul className="wbp-values" aria-label="Values the engine used">
            {preset.params.length ? preset.params.map(p => <li key={p.label}><b>{p.label}</b>{p.value}, {p.source}</li>) : <li><b>No rule values</b>a plain contact question</li>}
          </ul>
          <p className="wbp-note" role="note">{CAVEATS[preset.caveat]}</p>
          <details className="wbp-how">
            <summary>How this was determined</summary>
            <ol>{preset.derivation.map(step => <li key={step}>{step}</li>)}</ol>
          </details>
          <p className="wbp-meta">Case nexusai-wave1-dev2-20261021 (synthetic) <i>&middot;</i> scope: all evidence <i>&middot;</i> {events} events of the numbers asked about <i>&middot;</i> 72 exact repeats left out <i>&middot;</i> case clock Asia/Karachi (UTC+05:00)</p>
        </section>

        <section className="wbp-frame" aria-label="Workbench">
          <div className="wbp-bar">
            <div className="wbp-set" aria-label="Working set">
              <span className="wbp-set__label">Working set</span>
              <ul className="wbp-set__members">
                {members.slice(0, 9).map(n => <li key={n} title={n} className={preset.focus.includes(n) ? 'is-focus' : undefined}>{short(n)}</li>)}
              </ul>
              {members.length > 9 ? <span className="wbp-set__more">+{members.length - 9}</span> : null}
              <button type="button" className="wbp-btn wbp-btn--text" onClick={expand}>Expand one hop</button>
              {excluded.length ? <span className="wbp-hint" title={excluded.map(e => `${e.number} (${e.why})`).join(', ')}>{excluded.length} excluded, not hidden</span> : null}
            </div>
            <div className="wbp-tools">
              <div className="wbp-layers" role="group" aria-label="Analyses on the canvas">
                {Object.keys(layers).map(k => <button key={k} type="button" className="wbp-layer" aria-pressed={!!layers[k]} onClick={() => toggleLayer(k)}><i className={`wbp-dm wbp-dm--${LAYER_DOT[k]}`} />{LAYER_LABELS[k]}</button>)}
              </div>
              <button type="button" className="wbp-btn" aria-pressed={twin} onClick={() => setTwin(v => !v)}>Table <kbd>T</kbd></button>
              <button type="button" className="wbp-btn" onClick={copyLink}><Link2 size={14} aria-hidden="true" /> Link</button>
              <button type="button" className="wbp-btn" onClick={() => setNote('Saved (prototype): the question, working set, period and selection.')}>Save view</button>
              <button type="button" className="wbp-icon" aria-label="Keyboard shortcuts (?)" title="Keyboard shortcuts (?)" onClick={() => setHelp(v => !v)}><Keyboard size={16} aria-hidden="true" /></button>
            </div>
          </div>
          {help ? (
            <div className="wbp-help" role="dialog" aria-label="Keyboard shortcuts">
              {SHORTCUTS.map(([k, label]) => <span key={k}><kbd>{k}</kbd> {label}</span>)}
            </div>
          ) : null}
          <div className="wbp-body">
            <div className="wbp-canvas">{canvas}</div>
            <div className="wbp-side">
              <div className="wbp-resizer" role="separator" aria-orientation="vertical" aria-label="Resize the evidence panel" tabIndex={0} onPointerDown={startResize} onDoubleClick={() => setInWidth(340)}
                onKeyDown={e => { if (e.key === 'ArrowLeft') setInWidth(w => Math.min(MAX_IN, w + 20)); if (e.key === 'ArrowRight') setInWidth(w => Math.max(MIN_IN, w - 20)) }} />
              <WbDrawer drawer={drawer} onClose={() => setSelection(null)} onCopied={setNote} onAsk={() => setNote('Ask about this (prototype): the live workbench asks the engine a question with this mark as its subject.')} />
            </div>
          </div>
        </section>

        <p className="wbp-foot">Designed states: {['empty', 'broad', 'abstained', 'partial'].map(s => <a key={s} href={`?layout=${layout}&q=${preset.id}&state=${s}`}>{s}</a>)}<a href={`?layout=${layout}&q=${preset.id}`}>back to the case</a><span className="wbp-foot__keys">{SHORTCUTS.slice(0, 4).map(([k, l]) => <span key={k}><kbd>{k}</kbd> {l}</span>)}</span></p>
        {note ? <div className="wbp-toast" role="status"><Check size={14} aria-hidden="true" /> {note}</div> : null}
      </main>
    </AppShell>
  )
}
