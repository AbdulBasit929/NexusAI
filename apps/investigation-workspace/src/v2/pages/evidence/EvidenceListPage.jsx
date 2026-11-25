import { Fragment, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams, useSearchParams, Link } from 'react-router-dom'
import { AlertTriangle, ChevronRight, Flag, LayoutGrid, Plus, RefreshCw, Rows2, Rows3, Search, SearchX, Table2, X } from 'lucide-react'
import { AddDataDropzone } from '../../../components/AddDataDropzone.jsx'
import { LanguageText } from '../../../components/AnalystComponents.jsx'
import { evidenceFamilies } from '../../../components/ScopeChips.jsx'
import { listEvidence } from '../../../lib/apiClient.js'
import { formatBytes, formatNumber } from '../../../lib/format.js'
import { relativeAge } from '../../../lib/dashboardCases.js'
import { CaseShell } from '../../shell/AppShell.jsx'
import { Dialog } from '../../ui/Dialog.jsx'
import { EvidenceCard, evidenceHref, KIND_ICON, QuickLook, StatusMark, typeLabel } from './EvidenceCard.jsx'
import { composition, displayName, groupByDay, itemFacts, kindFromLegacyFamily, kindOf, shortId, statusCounts, statusOf } from './evidenceModel.js'
import { KnownSetsPanel, useKnownFlags } from './KnownSets.jsx'
import { applyHide, flagSummary, harmlessCount, onlyFlagged, toggled } from './knownSetsModel.js'

const PAGE_SIZE = 48
const RECORD_TYPES = evidenceFamilies.filter(family => family.recordType)
const STATUS_FILTERS = [['all', 'All'], ['completed', 'Ready'], ['processing', 'Processing'], ['failed', 'Failed']]
const STATUS_KEY = { all: 'all', completed: 'ready', processing: 'processing', failed: 'failed' }
const KIND_IDS = ['structured_records', 'image', 'audio', 'video', 'document', 'text', 'tabular']
const SIZES = [['s', 'Small cards'], ['m', 'Medium cards'], ['l', 'Large cards']]
const NO_FLAGS = new Map()

const oneOf = (value, allowed, fallback) => (allowed.includes(value) ? value : fallback)
const offsetOf = value => { const n = Number(value); return Number.isInteger(n) && n >= 0 ? n : 0 }
const remembered = (key, allowed, fallback) => { try { const value = globalThis.localStorage?.getItem(key); return allowed.includes(value) ? value : fallback } catch { return fallback } }
const remember = (key, value) => { try { globalThis.localStorage?.setItem(key, value) } catch { /* the choice is just not remembered */ } }

// The case, drawn once: one bar, one segment per kind, the width of each is its share of the files. The legend under it is the filter; hovering either lights the same kind.
function Composition({ parts, total, kind, onKind }) {
  const [hot, setHot] = useState('')
  const lit = hot || kind
  if (!parts.length) return null
  return (
    <section className="nx-evband" aria-label="What the case holds" data-lit={lit || undefined}>
      <div className="nx-evband__bar" aria-hidden="true">
        {parts.map(part => (
          <button key={part.id} type="button" tabIndex={-1} className="nx-evband__seg" data-tone={part.tone} data-on={lit === part.id || undefined} style={{ '--n': part.count }}
            title={`${part.label}: ${formatNumber(part.count)} of ${formatNumber(total)}`} onMouseEnter={() => setHot(part.id)} onMouseLeave={() => setHot('')} onClick={() => onKind(kind === part.id ? '' : part.id)} />
        ))}
      </div>
      <ul className="nx-evband__legend" role="group" aria-label="Show files of one kind">
        {parts.map(part => {
          const Icon = KIND_ICON[part.id]
          return (
            <li key={part.id}>
              <button type="button" className="nx-evband__key" data-tone={part.tone} data-on={lit === part.id || undefined} aria-pressed={kind === part.id}
                onMouseEnter={() => setHot(part.id)} onMouseLeave={() => setHot('')} onFocus={() => setHot(part.id)} onBlur={() => setHot('')} onClick={() => onKind(kind === part.id ? '' : part.id)}>
                {Icon ? <Icon aria-hidden="true" /> : <i aria-hidden="true" />}
                <span className="nx-evband__name">{part.label}</span>
                <b className="nx-tabular">{formatNumber(part.count)}</b>
                <span className="nx-evband__share nx-tabular">{part.share < 0.01 ? '<1' : Math.round(part.share * 100)}%</span>
              </button>
            </li>
          )
        })}
      </ul>
    </section>
  )
}

function Row({ item, caseId, onOpen, width, flags }) {
  const summary = flagSummary(flags)
  const href = evidenceHref(caseId, item)
  const facts = itemFacts(item)
  const when = item.created_at ? new Date(item.created_at) : null
  const known = when && !Number.isNaN(when.getTime())
  const Icon = KIND_ICON[kindOf(item.modality).id]
  return (
    <tr className={`nx-evrow${statusOf(item).key === 'failed' ? ' is-failed' : ''}`} onClick={event => { if (!event.target.closest('a, button')) onOpen(href) }}>
      <th scope="row" className="nx-evrow__name">
        <div className="nx-evrow__cell">
          <span className="nx-ev-tile" data-tone={kindOf(item.modality).tone} aria-hidden="true">{Icon ? <Icon /> : null}</span>
          <span className="nx-evrow__text">
            <Link className="nx-ev-open" to={href} aria-label={displayName(item)}><LanguageText>{displayName(item)}</LanguageText></Link>
            <small className="nx-mono" dir="ltr" title={item.evidence_id}>{shortId(item.evidence_id)}{item.extension ? ` · ${String(item.extension).toLowerCase()}` : ''}</small>
            {summary ? <span className="nx-evrow__flag" data-tone={summary.tone} title={summary.title}><Flag aria-hidden="true" />{summary.label}</span> : null}
          </span>
        </div>
      </th>
      {width !== 'narrow' ? <td><span className="nx-evkind" data-tone={kindOf(item.modality).tone}><i aria-hidden="true" />{typeLabel(item)}</span></td> : null}
      {width !== 'narrow' ? <td className="nx-evrow__facts">{facts ? <><span className="nx-tabular">{facts.main}</span>{facts.note ? <small>{facts.note}</small> : null}</> : <span className="nx-muted" aria-label="Not reported">–</span>}</td> : null}
      {width === 'wide' ? <td className="is-num nx-tabular">{item.size_bytes == null ? '–' : formatBytes(item.size_bytes)}</td> : null}
      {width === 'wide' ? <td className="nx-secondary">{known ? <time dateTime={item.created_at} title={`${new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'short', timeZone: 'UTC' }).format(when)} UTC`}>{relativeAge(when)}</time> : '–'}</td> : null}
      <td><StatusMark item={item} /></td>
      <td className="nx-evrow__go" aria-hidden="true"><ChevronRight /></td>
    </tr>
  )
}

function Skeleton({ view }) {
  if (view === 'list') return <div className="nx-evskel" role="status" aria-label="Reading the evidence list">{Array.from({ length: 9 }, (_, index) => <div key={index} className="nx-evskel__row"><i /><b /><span /><span /><span /></div>)}</div>
  return <ul className="nx-evgrid nx-evgrid--skel" role="status" aria-label="Reading the evidence list">{Array.from({ length: 12 }, (_, index) => <li key={index} className="nx-evcard nx-evcard--skel"><div className="nx-evcard__face" /><div className="nx-evcard__body"><b /><span /></div></li>)}</ul>
}

// arrow keys walk from one file to the next: along a row for left and right, to the nearest card in the next row for up and down
function walkGrid(event, root) {
  const keys = ['ArrowRight', 'ArrowLeft', 'ArrowDown', 'ArrowUp', 'Home', 'End']
  if (!keys.includes(event.key) || !root) return
  const links = [...root.querySelectorAll('a.nx-evcard__link')]
  const here = links.indexOf(globalThis.document.activeElement)
  if (here < 0) return
  event.preventDefault()
  const box = element => element.closest('.nx-evcard').getBoundingClientRect()
  const at = box(links[here])
  let target = links[here]
  if (event.key === 'ArrowRight') target = links[Math.min(links.length - 1, here + 1)]
  else if (event.key === 'ArrowLeft') target = links[Math.max(0, here - 1)]
  else if (event.key === 'Home') target = links[0]
  else if (event.key === 'End') target = links.at(-1)
  else {
    const down = event.key === 'ArrowDown'
    const beyond = links.filter(link => (down ? box(link).top > at.top + 4 : box(link).top < at.top - 4))
    if (beyond.length) {
      const row = down ? Math.min(...beyond.map(link => box(link).top)) : Math.max(...beyond.map(link => box(link).top))
      target = beyond.filter(link => Math.abs(box(link).top - row) < 4).sort((a, b) => Math.abs(box(a).left - at.left) - Math.abs(box(b).left - at.left))[0]
    }
  }
  target?.focus()
  target?.closest('.nx-evcard')?.scrollIntoView({ block: 'nearest' })
}

export default function EvidenceListPage() {
  const { id: caseId = '' } = useParams()
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const legacy = kindFromLegacyFamily(params.get('family'))
  const [kind, setKind] = useState(() => oneOf(params.get('kind') || legacy, KIND_IDS, ''))
  const [recordType, setRecordType] = useState(() => oneOf(params.get('type') || '', RECORD_TYPES.map(item => item.recordType), ''))
  const [status, setStatus] = useState(() => oneOf(params.get('status') || 'all', STATUS_FILTERS.map(([id]) => id), 'all'))
  const [text, setText] = useState(() => params.get('q') || '')
  const [query, setQuery] = useState(() => (params.get('q') || '').trim())
  const [offset, setOffset] = useState(() => offsetOf(params.get('offset')))
  const [view, setView] = useState(() => remembered('nexusai.evidence.view', ['grid', 'list'], 'grid'))
  const [cardSize, setCardSize] = useState(() => remembered('nexusai.evidence.cards', ['s', 'm', 'l'], 'm'))
  const [density, setDensity] = useState(() => remembered('nexusai.evidence.density', ['compact', 'comfortable'], 'compact'))
  const [tick, setTick] = useState(0)
  const [adding, setAdding] = useState(false)
  const [peek, setPeek] = useState(null)
  const [list, setList] = useState({ loading: true })
  const [inventory, setInventory] = useState(null)
  const [known, refreshKnown] = useKnownFlags(caseId, tick)
  const [selecting, setSelecting] = useState(false)
  const [selected, setSelected] = useState(() => new Set())
  const [hideHarmless, setHideHarmless] = useState(() => remembered('nexusai.evidence.hide-known', ['yes', 'no'], 'no') === 'yes')
  const [setsOpen, setSetsOpen] = useState(false)
  const knownButton = useRef(null)
  const [flaggedOnly, setFlaggedOnly] = useState(false)
  const addButton = useRef(null)
  const peekReturn = useRef(null)
  const tbody = useRef(null)
  const groupsRoot = useRef(null)
  const tableBox = useRef(null)
  const [tableWidth, setTableWidth] = useState('wide')

  useEffect(() => { globalThis.document.title = 'Evidence · NexusAI' }, [])
  // the list gives up columns as its own width shrinks, not the window's: wide shows every column, medium drops size and date, narrow keeps the name and the state
  useEffect(() => {
    const node = tableBox.current
    if (!node || !globalThis.ResizeObserver) return undefined
    const apply = width => setTableWidth(width < 680 ? 'narrow' : width < 900 ? 'medium' : 'wide')
    const watcher = new globalThis.ResizeObserver(entries => apply(entries[0].contentRect.width))
    watcher.observe(node)
    apply(node.getBoundingClientRect().width)
    return () => watcher.disconnect()
  }, [view, Boolean(list.data?.items?.length)])
  useEffect(() => {
    const timer = globalThis.setTimeout(() => { const next = text.trim(); if (next !== query) { setQuery(next); setOffset(0) } }, 350)
    return () => globalThis.clearTimeout(timer)
  }, [text, query])
  useEffect(() => {
    const next = new URLSearchParams()
    if (kind) next.set('kind', kind)
    if (recordType) next.set('type', recordType)
    if (status !== 'all') next.set('status', status)
    if (query) next.set('q', query)
    if (offset > 0) next.set('offset', String(offset))
    if (next.toString() !== params.toString()) setParams(next, { replace: true })
  }, [kind, recordType, status, query, offset, params, setParams])

  // the whole case, whatever the filters say: the bar, the chips and the head are counts of the case, not of what is listed below
  useEffect(() => {
    const controller = new AbortController()
    listEvidence({ caseId, signal: controller.signal, filters: { limit: 1 } }).then(data => setInventory(data?.summary || {})).catch(() => {})
    return () => controller.abort()
  }, [caseId, tick])

  useEffect(() => {
    const controller = new AbortController()
    let poll
    setList(current => ({ ...current, loading: true, error: null }))
    listEvidence({ caseId, signal: controller.signal, filters: { limit: PAGE_SIZE, offset, q: query, modality: kind || undefined, detectedType: recordType || undefined, processingStatus: status === 'all' ? undefined : status } })
      .then(data => {
        setList({ data, loading: false })
        if ((data?.items || []).some(item => statusOf(item).key === 'processing')) poll = globalThis.setTimeout(() => setTick(value => value + 1), 5000)
      })
      .catch(error => { if (error.name !== 'AbortError') setList(current => ({ ...current, error, loading: false })) })
    return () => { controller.abort(); globalThis.clearTimeout(poll) }
  }, [caseId, kind, recordType, status, query, offset, tick])

  const items = list.data?.items || []
  const pagination = list.data?.pagination || {}
  const total = Number(pagination.evidence_total ?? list.data?.summary?.evidence_total ?? 0)
  const first = items.length ? offset + 1 : 0
  const last = offset + items.length
  const counts = statusCounts(inventory)
  const parts = useMemo(() => composition(inventory?.modality_counts), [inventory])
  const byEvidence = known.byEvidence || NO_FLAGS
  const { visible: afterHide, hidden } = useMemo(() => applyHide(items, byEvidence, hideHarmless && known.status === 'ready'), [items, byEvidence, hideHarmless, known.status])
  const flaggedHere = useMemo(() => onlyFlagged(afterHide, byEvidence).length, [afterHide, byEvidence]) // what the chip would show, so the number and the result agree
  const visible = useMemo(() => (flaggedOnly && known.status === 'ready' ? onlyFlagged(afterHide, byEvidence) : afterHide), [afterHide, byEvidence, flaggedOnly, known.status])
  const harmless = useMemo(() => harmlessCount(items, byEvidence), [items, byEvidence])
  const toggle = useCallback(id => setSelected(current => toggled(current, id)), [])
  const groups = useMemo(() => groupByDay(visible), [visible])
  const filtered = Boolean(kind || recordType || status !== 'all' || query)
  const showTypes = !kind || kind === 'structured_records'

  const choose = (setter, value) => { setter(value); setOffset(0) }
  const clear = () => { setKind(''); setRecordType(''); setStatus('all'); setText(''); setQuery(''); setOffset(0) }
  const open = useCallback(href => navigate(href), [navigate])
  const closeAdding = () => { setAdding(false); globalThis.requestAnimationFrame(() => addButton.current?.focus()) }
  const openPeek = useCallback(item => { peekReturn.current = globalThis.document.activeElement; setPeek(item) }, [])
  const closePeek = () => { setPeek(null); globalThis.requestAnimationFrame(() => peekReturn.current?.focus?.()) }

  const onGridKey = event => {
    if (event.key === ' ' && event.target.matches?.('a.nx-evcard__link')) {
      const id = event.target.closest('.nx-evcard')?.dataset?.id
      const item = items.find(entry => entry.evidence_id === id)
      if (item) { event.preventDefault(); openPeek(item) }
      return
    }
    walkGrid(event, groupsRoot.current)
  }
  const walkList = event => {
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
    const links = [...(tbody.current?.querySelectorAll('a.nx-ev-open') || [])]
    const at = links.indexOf(globalThis.document.activeElement)
    if (at < 0) return
    event.preventDefault()
    links[Math.min(links.length - 1, Math.max(0, at + (event.key === 'ArrowDown' ? 1 : -1)))]?.focus()
  }

  const headline = counts.all
    ? `${formatNumber(counts.all)} ${counts.all === 1 ? 'file' : 'files'}${inventory?.total_size_bytes ? ` · ${formatBytes(inventory.total_size_bytes)}` : ''} · ${counts.ready === counts.all ? 'all ready' : `${formatNumber(counts.ready)} ready`}${counts.failed ? ` · ${formatNumber(counts.failed)} failed` : ''}${counts.processing ? ` · ${formatNumber(counts.processing)} processing` : ''}`
    : inventory ? 'No files yet.' : 'Reading the case…'

  return (
    <CaseShell caseId={caseId}>
      <main id="workspace-main" className="nx-ui nx-page nx-evidence" tabIndex={-1}>
        <header className="nx-page-head">
          <div className="nx-page-head__text">
            <h1>Evidence</h1>
            <p aria-live="polite">{headline}</p>
          </div>
          <div className="nx-page-head__actions">
            <button type="button" className="nx-btn" onClick={() => setTick(value => value + 1)} disabled={list.loading}><RefreshCw aria-hidden="true" className={list.loading ? 'is-spinning' : undefined} />Refresh</button>
            <button type="button" ref={addButton} className="nx-btn nx-btn--primary" onClick={() => setAdding(true)}><Plus aria-hidden="true" />Add evidence</button>
          </div>
        </header>

        <Composition parts={parts} total={counts.all} kind={kind} onKind={value => choose(setKind, value)} />

        <div className="nx-evbar">
          <form role="search" className="nx-evbar__search" onSubmit={event => { event.preventDefault(); setQuery(text.trim()); setOffset(0) }}>
            <Search aria-hidden="true" />
            <label className="nx-visually-hidden" htmlFor="nx-ev-search">Search evidence</label>
            <input id="nx-ev-search" type="search" className="nx-input" value={text} onChange={event => setText(event.target.value)} placeholder="File name, ID, hash or word" />
          </form>
          <div className="nx-evbar__status" role="group" aria-label="Processing state">
            {STATUS_FILTERS.map(([id, label]) => {
              const count = counts[STATUS_KEY[id]]
              return <button key={id} type="button" className={`nx-chip${id === 'failed' && count ? ' has-issue' : ''}`} aria-pressed={status === id} disabled={id !== 'all' && inventory && !count && status !== id} onClick={() => choose(setStatus, id)}>{label}{inventory ? <span className="nx-chip__count nx-tabular">{formatNumber(count)}</span> : null}</button>
            })}
          </div>
          {showTypes ? (
            <label className="nx-evbar__type"><span className="nx-visually-hidden">Record type</span>
              <select className="nx-select" value={recordType} onChange={event => choose(setRecordType, event.target.value)}>
                <option value="">All record types</option>
                {RECORD_TYPES.map(item => <option key={item.id} value={item.recordType}>{item.label}</option>)}
              </select>
            </label>
          ) : null}
          {known.status === 'ready' ? (
            <div className="nx-evbar__known" role="group" aria-label="Known sets">
              <button type="button" ref={knownButton} className="nx-btn nx-btn--sm" onClick={() => setSetsOpen(true)}><Flag aria-hidden="true" />Known sets{known.data.flagged_pictures ? <span className="nx-chip__count nx-tabular">{formatNumber(known.data.flagged_pictures)}</span> : null}</button>
              {view === 'grid' ? <button type="button" className="nx-btn nx-btn--sm" aria-pressed={selecting} onClick={() => { setSelecting(value => !value); setSelected(new Set()) }}>Select</button> : null}
              {flaggedHere || flaggedOnly ? <button type="button" className="nx-chip" aria-pressed={flaggedOnly} title="Show only the pictures of this page that a known set flags" onClick={() => setFlaggedOnly(value => !value)}>Flagged<span className="nx-chip__count nx-tabular">{formatNumber(flaggedHere)}</span></button> : null}
              {harmless || hideHarmless ? <button type="button" className="nx-chip" aria-pressed={hideHarmless} onClick={() => { const next = !hideHarmless; setHideHarmless(next); remember('nexusai.evidence.hide-known', next ? 'yes' : 'no') }}>Hide known harmless<span className="nx-chip__count nx-tabular">{formatNumber(harmless)}</span></button> : null}
            </div>
          ) : null}
          <span className="nx-evbar__space" />
          {filtered ?<button type="button" className="nx-btn nx-btn--ghost nx-btn--sm" onClick={clear}><X aria-hidden="true" />Clear</button> : null}
          {view === 'grid' ? (
            <div className="nx-segmented nx-evbar__size" role="group" aria-label="Card size">
              {SIZES.map(([id, label]) => <button key={id} type="button" aria-pressed={cardSize === id} aria-label={label} title={label} onClick={() => { setCardSize(id); remember('nexusai.evidence.cards', id) }}><span className={`nx-evbar__sz nx-evbar__sz--${id}`} aria-hidden="true" /></button>)}
            </div>
          ) : (
            <div className="nx-segmented nx-evbar__density" role="group" aria-label="Row height">
              <button type="button" aria-pressed={density === 'compact'} aria-label="Compact rows" title="Compact rows" onClick={() => { setDensity('compact'); remember('nexusai.evidence.density', 'compact') }}><Rows3 aria-hidden="true" /></button>
              <button type="button" aria-pressed={density === 'comfortable'} aria-label="Comfortable rows" title="Comfortable rows" onClick={() => { setDensity('comfortable'); remember('nexusai.evidence.density', 'comfortable') }}><Rows2 aria-hidden="true" /></button>
            </div>
          )}
          <div className="nx-segmented" role="group" aria-label="View">
            <button type="button" aria-pressed={view === 'grid'} onClick={() => { setView('grid'); remember('nexusai.evidence.view', 'grid') }}><LayoutGrid aria-hidden="true" />Cards</button>
            <button type="button" aria-pressed={view === 'list'} onClick={() => { setView('list'); remember('nexusai.evidence.view', 'list') }}><Table2 aria-hidden="true" />List</button>
          </div>
        </div>

        {hidden.length ? <p className="nx-ks__hidden" role="status"><b className="nx-tabular">{formatNumber(hidden.length)}</b> {hidden.length === 1 ? 'picture is' : 'pictures are'} hidden because a known-harmless set flags {hidden.length === 1 ? 'it' : 'them'}. <button type="button" className="nx-btn nx-btn--ghost nx-btn--sm" onClick={() => { setHideHarmless(false); remember('nexusai.evidence.hide-known', 'no') }}>Show {hidden.length === 1 ? 'it' : 'them'}</button></p> : null}

        {list.loading && !items.length ? <Skeleton view={view} /> : null}
        {list.error && !items.length ? (
          <div className="nx-empty nx-card" role="alert"><span className="nx-empty__icon"><AlertTriangle aria-hidden="true" /></span><h3>{list.error.status === 403 ? 'This case is outside your access' : 'The evidence list could not be read'}</h3><p>{list.error.status === 403 ? 'Ask the case owner for access.' : 'Nothing was changed. Try again.'}</p><div className="nx-empty__actions"><button type="button" className="nx-btn" onClick={() => setTick(value => value + 1)}>Try again</button></div></div>
        ) : null}
        {!list.loading && !list.error && !items.length ? (
          filtered
            ? <div className="nx-empty nx-card"><span className="nx-empty__icon"><SearchX aria-hidden="true" /></span><h3>No file matches</h3><p>Nothing in this case fits these filters.</p><div className="nx-empty__actions"><button type="button" className="nx-btn" onClick={clear}>Show every file</button></div></div>
            : <div className="nx-empty nx-card"><span className="nx-empty__icon"><Plus aria-hidden="true" /></span><h3>No evidence yet</h3><p>Add the first file and it is read here.</p><div className="nx-empty__actions"><button type="button" className="nx-btn nx-btn--primary" onClick={() => setAdding(true)}>Add evidence</button></div></div>
        ) : null}

        {items.length && view === 'grid' ? (
          <section className="nx-evgroups" data-size={cardSize} aria-busy={list.loading ? 'true' : 'false'} aria-label="Evidence files" ref={groupsRoot} onKeyDown={onGridKey}>
            {groups.map(group => (
              <div key={group.key || 'unknown'} className="nx-evgroup">
                <h2 className="nx-evgroup__head"><span>{group.label}</span><b className="nx-tabular">{formatNumber(group.items.length)}</b></h2>
                <ul className="nx-evgrid" role="list">
                  {group.items.map(item => <EvidenceCard key={item.evidence_id} item={item} caseId={caseId} onPeek={openPeek} flags={byEvidence.get(item.evidence_id)} selecting={selecting} selected={selected.has(item.evidence_id)} onToggle={toggle} />)}
                </ul>
              </div>
            ))}
          </section>
        ) : null}

        {items.length && view === 'list' ? (
          <section className="nx-evtable" ref={tableBox} data-density={density} aria-busy={list.loading ? 'true' : 'false'} aria-label="Evidence files">
            <div className="nx-evtable-wrap">
              <table className="nx-ev-table" onKeyDown={walkList}>
                <caption className="nx-visually-hidden">{`Showing ${formatNumber(first)} to ${formatNumber(last)} of ${formatNumber(total)} files, newest first.`}</caption>
                <colgroup><col />{tableWidth !== 'narrow' ? <><col style={{ inlineSize: 150 }} /><col style={{ inlineSize: 150 }} /></> : null}{tableWidth === 'wide' ? <><col style={{ inlineSize: 96 }} /><col style={{ inlineSize: 112 }} /></> : null}<col style={{ inlineSize: 124 }} /><col style={{ inlineSize: 40 }} /></colgroup>
                <thead>
                  <tr><th scope="col">File</th>{tableWidth !== 'narrow' ? <><th scope="col">Kind</th><th scope="col">Details</th></> : null}{tableWidth === 'wide' ? <><th scope="col" className="is-num">Size</th><th scope="col">Added</th></> : null}<th scope="col">Status</th><th scope="col"><span className="nx-visually-hidden">Open</span></th></tr>
                </thead>
                <tbody ref={tbody}>
                  {groups.map(group => (
                    <Fragment key={group.key || 'unknown'}>
                      <tr className="nx-evday"><th scope="rowgroup" colSpan={tableWidth === 'wide' ? 7 : tableWidth === 'medium' ? 5 : 3}><span>{group.label}</span><b className="nx-tabular">{formatNumber(group.items.length)}</b></th></tr>
                      {group.items.map(item => <Row key={item.evidence_id} item={item} caseId={caseId} onOpen={open} width={tableWidth} flags={byEvidence.get(item.evidence_id)} />)}
                    </Fragment>
                  ))}
                </tbody>
              </table>
            </div>
          </section>
        ) : null}

        {items.length ? (
          <nav className="nx-evfoot nx-evfoot--free" aria-label="Pages of evidence">
            <p className="nx-tabular">{`${formatNumber(first)}–${formatNumber(last)} of ${formatNumber(total)}`}{filtered ? ' matching' : ''}</p>
            <div>
              <button type="button" className="nx-btn nx-btn--sm" disabled={!pagination.has_previous} onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>Previous</button>
              <button type="button" className="nx-btn nx-btn--sm" disabled={!pagination.has_next} onClick={() => setOffset(offset + PAGE_SIZE)}>Next</button>
            </div>
          </nav>
        ) : null}
      </main>

      {selecting && selected.size ? (
        <div className="nx-ksbar" role="region" aria-label="Selected pictures">
          <b className="nx-tabular">{formatNumber(selected.size)} selected</b>
          <button type="button" className="nx-btn nx-btn--primary nx-btn--sm" onClick={() => setSetsOpen(true)}>Add to a set…</button>
          <button type="button" className="nx-btn nx-btn--ghost nx-btn--sm" onClick={() => setSelected(new Set())}>Clear</button>
        </div>
      ) : null}
      {setsOpen && known.status === 'ready' ? <KnownSetsPanel caseId={caseId} sets={known.data.sets || []} selected={selected} onChanged={refreshKnown} onClose={() => { setSetsOpen(false); setSelected(new Set()); setSelecting(false); globalThis.requestAnimationFrame(() => knownButton.current?.focus()) }} /> : null}
      {adding ? <Dialog title="Add evidence" wide onClose={closeAdding}><AddDataDropzone bare caseId={caseId} onAccepted={() => { setOffset(0); setTick(value => value + 1) }} /></Dialog> : null}
      {peek ? <QuickLook item={peek} caseId={caseId} onClose={closePeek} /> : null}
    </CaseShell>
  )
}
