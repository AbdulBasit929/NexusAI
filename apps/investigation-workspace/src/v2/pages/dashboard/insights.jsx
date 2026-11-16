import { useMemo, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { CircleAlert, CircleCheckBig, MessageSquareText } from 'lucide-react'
import { formatNumber } from '../../../lib/format.js'
import { formatRate } from '../../../lib/dashboardCharts.js'
import { CaseLabel, useCaseNameOf } from '../../shell/CaseLabel.jsx'
import { Calm, Panel, PanelSkeleton } from '../../ui/Panel.jsx'
import { Dumbbell, RangeStrip, SegBar } from '../../ui/micro.jsx'
import { MIN_WINDOW, alignTimelines, formatDuration, heatShare, niceCeiling, panWindow, viewWindow, windowTicks, zoomWindow } from './model.js'

// Every panel here is drawn for a cell of a fixed size (the board gives it one; see styles/board.css): its picture takes the height that is left, its words are a title, a note and the numbers
// beside the marks, and the same figures as a table are one button away in a dialog. Nothing in a panel grows with its content, so neighbours always share a top and a bottom.
const MONTH = new Intl.DateTimeFormat('en-GB', { month: 'short', year: 'numeric', timeZone: 'UTC' })
const monthLabel = key => MONTH.format(new Date(`${key.slice(0, 7)}-01T00:00:00Z`))
const plural = (count, one, many = `${one}s`) => `${formatNumber(count)} ${count === 1 ? one : many}`
const colourOf = kind => (kind.slot ? `var(--nx-data-${kind.slot})` : 'var(--nx-data-muted)')
const TIMELINE_ROWS = 6
const laneColour = index => `var(--nx-data-${(index % 6) + 1})`
const RANGES = [{ id: '1y', months: 12, label: '1 y', title: 'The last year' }, { id: '3y', months: 36, label: '3 y', title: 'The last three years' }]
const BASIS_SHORT = { confirmed_by_analyst: 'Analyst-confirmed', explicit_offset: 'Stated in file', declared: 'Source-declared', assumed: 'Assumed' }

// ---------- the evidence timeline: when the events of each case happened, every case on one calendar ----------
// The lanes share one calendar (it runs from the earliest month any case has to this month), so where the cases overlap in time can be read at a glance. "Own scale" lets each lane's busiest month fill
// its height (the shape of the case); "Shared scale" gives every lane one scale (the size). Pointing at the plot, or using the arrow keys on it, reads one month: the figure at the end of each lane
// becomes that month's figure and the title names the month, so nothing floats over the data. A case the service returned only in part ends in a dotted run, which means "not read", never "empty".
// A date in the future is named and left out, and the table lists the same figures.
const cellLabel = cell => (cell.from === cell.to ? monthLabel(cell.from) : `${monthLabel(cell.from)} – ${monthLabel(cell.to)}`)
const PARTIAL_NOTE = 'The service returns at most 100 rows for a case (one for each day and kind of record), oldest first. Only those are drawn and counted; the dotted run is the part that was not read.'

function Lane({ lane, ticks, max, active, label, mark, colour }) {
  const style = { '--lane': colour, ...(active === null ? null : { '--at': active }), ...(mark ? { '--m0': mark[0], '--m1': mark[1] } : null) }
  return (
    <span className="nx-lane" role="img" aria-label={label} data-at={active === null ? undefined : ''} data-m={mark ? '' : undefined} style={style}>
      {ticks.map(tick => <b key={tick.index} className="nx-lane__tick" style={{ '--k': tick.index }} />)}
      {lane.cells.map((cell, index) => {
        const drawn = cell.state === 'in' && cell.total > 0
        const tone = cell.state !== 'in' ? `is-${cell.state}` : drawn ? '' : 'is-empty'
        const className = [tone, index === lane.peak ? 'is-peak' : '', index === active ? 'is-on' : ''].filter(Boolean).join(' ')
        return <i key={cell.from} className={className || undefined} style={drawn ? { '--h': `${Math.max(6, (cell.total / max) * 100)}%`, '--i': Math.min(index, 24) } : undefined} />
      })}
    </span>
  )
}

// The whole calendar in one thin strip (every case together), with the part in view marked: the overview of "overview first, zoom, then details" (the strip is drawn only when the cell is tall enough).
function Overview({ totals, from, to, onPick }) {
  const count = totals.length
  const top = Math.max(1, ...totals)
  const path = totals.map((value, index) => `${(((index + 0.5) / count) * 100).toFixed(2)} ${(22 - (value / top) * 20).toFixed(2)}`)
  const strip = useRef(null)
  const start = useRef(null)
  const bandRef = useRef(null)
  const [band, setBand] = useState(null)
  const mark = range => { bandRef.current = range; setBand(range) }
  const at = event => {
    const box = strip.current?.getBoundingClientRect()
    if (!box || box.width <= 0) return null
    return Math.min(count - 1, Math.max(0, Math.floor(((event.clientX - box.left) / box.width) * count)))
  }
  const shown = band || [from, to - 1]
  const whole = from === 0 && to === count
  return (
    <div
      ref={strip}
      className="nx-tl__strip"
      style={{ '--n': count, '--a': shown[0], '--b': shown[1] + 1 }}
      onPointerDown={event => { if (event.button !== 0) return; const index = at(event); if (index === null) return; start.current = index; mark([index, index]); event.currentTarget.setPointerCapture?.(event.pointerId) }}
      onPointerMove={event => { if (start.current === null) return; const index = at(event); if (index !== null) mark([Math.min(start.current, index), Math.max(start.current, index)]) }}
      onPointerUp={event => { if (start.current === null) return; const range = bandRef.current; start.current = null; mark(null); event.currentTarget.releasePointerCapture?.(event.pointerId); if (range && range[1] - range[0] + 1 >= MIN_WINDOW) onPick([range[0], range[1] + 1]) }}
      onPointerCancel={() => { start.current = null; mark(null) }}
      onDoubleClick={() => onPick(null)}
    >
      <svg viewBox="0 0 100 24" preserveAspectRatio="none" aria-hidden="true"><path className="nx-tl__area" d={`M0 24 L${path.join(' L')} L100 24 Z`} /><path className="nx-tl__line" d={`M${path.join(' L')}`} vectorEffect="non-scaling-stroke" /></svg>
      {whole && !band ? null : <i className="nx-tl__brush" />}
    </div>
  )
}

export function TimelinePanel({ timelines, loading, failures, today }) {
  const nameOf = useCaseNameOf()
  const [scale, setScale] = useState('own')
  const [active, setActive] = useState(null) // a group of the whole calendar
  const [win, setWin] = useState(null) // [from, to) in groups of the whole calendar, or null for all of it
  const [drag, setDrag] = useState(null) // [from, to] while a range is being dragged on the lanes
  const [tip, setTip] = useState(null) // where the pointer is inside the plot, for the card that names the month
  const dragStart = useRef(null)
  const dragRange = useRef(null)
  const plot = useRef(null)
  const axis = useMemo(() => alignTimelines(timelines.slice(0, TIMELINE_ROWS), today), [timelines, today])
  const whole = axis ? axis.groups.length : 0
  const [from, to] = viewWindow(whole, win)
  const count = to - from
  const windowed = whole > 0 && (from !== 0 || to !== whole)
  const groups = useMemo(() => (axis ? axis.groups.slice(from, to) : []), [axis, from, to])
  const ticks = useMemo(() => windowTicks(groups), [groups])
  const lanes = useMemo(() => (axis ? axis.lanes.map(lane => {
    const cells = lane.cells.slice(from, to)
    const best = cells.reduce((pick, cell, index) => (cell.state === 'in' && cell.total > (pick < 0 ? 0 : cells[pick].total) ? index : pick), -1)
    return { ...lane, cells, peak: best, shown: cells.reduce((total, cell) => total + (cell.state === 'in' ? cell.total : 0), 0) }
  }) : []), [axis, from, to])
  const totals = useMemo(() => (axis ? axis.groups.map((_, index) => axis.lanes.reduce((total, lane) => total + (lane.cells[index].state === 'in' ? lane.cells[index].total : 0), 0)) : []), [axis])
  const anyPartial = lanes.some(lane => lane.partial)
  const sharedMax = Math.max(1, ...lanes.flatMap(lane => lane.cells.map(cell => cell.total)))
  const twin = timelines.map(item => ({ id: item.caseId, case: nameOf(item.caseId), from: item.first ? monthLabel(item.first) : '', to: item.last ? monthLabel(item.last) : '', events: item.total, note: [item.partial ? 'earliest rows only' : '', item.futureTotal ? `${formatNumber(item.futureTotal)} events dated after today are not drawn` : ''].filter(Boolean).join('; ') }))
  const relative = active === null || active < from || active >= to ? null : active - from
  const valueOf = lane => {
    if (relative === null) return windowed ? formatNumber(lane.shown) : `${formatNumber(lane.total)}${lane.partial ? '+' : ''}`
    const cell = lane.cells[relative]
    return cell.state === 'unread' ? 'not read' : formatNumber(cell.total)
  }
  const noteOf = lane => {
    if (relative !== null) return cellLabel(groups[relative])
    return lane.peak >= 0 ? `peak ${monthLabel(lane.cells[lane.peak].from)}` : ''
  }
  const indexAt = (event, clamp = false) => {
    const strip = plot.current?.querySelector('.nx-lane')
    if (!strip || !count) return null
    const box = strip.getBoundingClientRect()
    if (box.width <= 0) return null
    if (!clamp && (event.clientX < box.left || event.clientX > box.right)) return null
    return from + Math.min(count - 1, Math.max(0, Math.floor(((event.clientX - box.left) / box.width) * count)))
  }
  const choose = range => setWin(range ? viewWindow(whole, range) : null)
  const reset = () => { setWin(null); setActive(null) }
  const onKeyDown = event => {
    if (!whole || event.target !== event.currentTarget) return
    const step = { ArrowLeft: -1, ArrowRight: 1 }[event.key]
    if (step && event.shiftKey) { event.preventDefault(); setWin(current => panWindow(whole, current, step * Math.max(1, Math.round(count / 4)))); return }
    if (step) { event.preventDefault(); setActive(current => Math.min(to - 1, Math.max(from, (current ?? (step > 0 ? from - 1 : to)) + step))) }
    else if (event.key === 'Home') { event.preventDefault(); setActive(from) }
    else if (event.key === 'End') { event.preventDefault(); setActive(to - 1) }
    else if (event.key === '+' || event.key === '=') { event.preventDefault(); setWin(current => zoomWindow(whole, current, 0.5, active ?? undefined)) }
    else if (event.key === '-' || event.key === '_') { event.preventDefault(); setWin(current => zoomWindow(whole, current, 2, active ?? undefined)) }
    else if (event.key === 'Escape') { if (active !== null) setActive(null); else setWin(null) }
  }
  const onPointerDown = event => {
    if (event.button !== 0 || !event.target.closest?.('.nx-lane')) return
    const index = indexAt(event)
    if (index === null) return
    dragStart.current = index
    dragRange.current = [index, index]
    setDrag([index, index])
    event.currentTarget.setPointerCapture?.(event.pointerId)
  }
  const onPointerMove = event => {
    if (dragStart.current !== null) {
      const index = indexAt(event, true)
      if (index !== null) { dragRange.current = [Math.min(dragStart.current, index), Math.max(dragStart.current, index)]; setDrag(dragRange.current); setActive(index) }
      return
    }
    setActive(indexAt(event))
    const box = plot.current?.getBoundingClientRect()
    setTip(box ? { x: event.clientX - box.left, y: event.clientY - box.top, w: box.width } : null)
  }
  const onPointerUp = event => {
    if (dragStart.current === null) return
    const range = dragRange.current
    dragStart.current = null
    dragRange.current = null
    setDrag(null)
    event.currentTarget.releasePointerCapture?.(event.pointerId)
    if (range && range[1] - range[0] + 1 >= MIN_WINDOW) choose([range[0], range[1] + 1])
  }
  const mark = drag ? [drag[0] - from, drag[1] - from] : null
  const readout = relative === null || !axis ? '' : `${cellLabel(groups[relative])}: ${lanes.map(lane => `${nameOf(lane.caseId)} ${valueOf(lane)}`).join(', ')}`
  const toggle = (
    <>
      <div className="nx-segmented nx-tl__range-pick" role="group" aria-label="Range">
        {RANGES.map(range => {
          const needed = axis ? Math.min(whole, Math.ceil(range.months / axis.groupSize)) : 0
          return <button key={range.id} type="button" aria-pressed={windowed && to === whole && count === needed} disabled={!axis || needed >= whole} onClick={() => choose([whole - needed, whole])} title={needed >= whole ? 'The calendar is not longer than this' : `${range.title} (or drag across the lanes)`}>{range.label}</button>
        })}
        <button type="button" aria-pressed={!windowed} onClick={reset} title="The whole calendar (double-click the lanes, or press Escape)">All</button>
      </div>
      <div className="nx-segmented" role="group" aria-label="Scale of the lanes">
        <button type="button" aria-pressed={scale === 'own'} onClick={() => setScale('own')} aria-label="Own scale" title="Own scale: each lane's busiest month fills the lane, the shape of each case">Own</button>
        <button type="button" aria-pressed={scale === 'shared'} onClick={() => setScale('shared')} aria-label="Shared scale" title="Shared scale: one scale for every lane, the size of each case against the others">Shared</button>
      </div>
    </>
  )
  const sub = relative !== null && axis ? cellLabel(groups[relative]) : (windowed && axis ? `${monthLabel(groups[0].from)} – ${monthLabel(groups[groups.length - 1].to)}` : 'Events per month · drag to zoom')
  return (
    <Panel id="dash-timeline" title="Evidence timeline" sub={sub} action={timelines.length ? toggle : null} className="nx-panel--timeline"
      twin={timelines.length ? { label: 'Events per case by month range', exportName: 'nexusai-evidence-timeline.csv', summary: `Events per month for ${plural(timelines.length, 'case')}; ${plural(timelines.filter(item => item.partial).length, 'case')} read only as far as the service returns.`, columns: [{ key: 'case', label: 'Case' }, { key: 'from', label: 'From' }, { key: 'to', label: 'To' }, { key: 'events', label: 'Events' }, { key: 'note', label: 'Note' }], rows: twin } : undefined}>
      {loading && !timelines.length ? <PanelSkeleton rows={4} /> : timelines.length ? (
        <>
          <div
            ref={plot}
            className="nx-tl"
            style={{ '--n': count }}
            data-reading={relative === null ? undefined : ''}
            data-dragging={drag ? '' : undefined}
            tabIndex={axis ? 0 : undefined}
            role="group"
            aria-label="Events per month in each case. Left and right arrow keys read one month, plus and minus zoom, shift with an arrow key moves the view, Escape goes back."
            onPointerDown={onPointerDown}
            onPointerMove={onPointerMove}
            onPointerUp={onPointerUp}
            onPointerCancel={() => { dragStart.current = null; dragRange.current = null; setDrag(null) }}
            onPointerLeave={() => { if (dragStart.current === null) { setActive(null); setTip(null) } }}
            onDoubleClick={() => setWin(null)}
            onKeyDown={onKeyDown}
            onBlur={event => { if (!event.currentTarget.contains(event.relatedTarget)) setActive(null) }}
          >
            <ol className="nx-tl__lanes">
              {lanes.map((lane, laneIndex) => (
                <li key={lane.caseId} className="nx-tl__row" style={{ '--lane': laneColour(laneIndex) }}>
                  <span className="nx-tl__who">
                    <i className="nx-tl__dot" aria-hidden="true" />
                    <Link className="nx-tl__name" to={`/cases/${encodeURIComponent(lane.caseId)}/timeline`} title={lane.partial ? PARTIAL_NOTE : nameOf(lane.caseId)}><CaseLabel caseId={lane.caseId} /></Link>
                  </span>
                  <Lane lane={lane} ticks={ticks} active={relative} mark={mark} colour={laneColour(laneIndex)} max={scale === 'shared' ? sharedMax : Math.max(1, ...lane.cells.map(cell => cell.total))}
                    label={`Events by month from ${lane.first ? monthLabel(lane.first) : '—'} to ${lane.last ? monthLabel(lane.last) : '—'}: ${formatNumber(lane.total)}${lane.partial ? ', the earliest rows only' : ''}${lane.peak >= 0 ? `; busiest ${cellLabel(lane.cells[lane.peak])}` : ''}`} />
                  <span className="nx-tl__end">
                    <span className={`nx-tl__count nx-tabular${relative !== null ? ' is-reading' : ''}`}>
                      {lane.futureTotal && relative === null ? <CircleAlert className="nx-tl__warn" aria-label={`${formatNumber(lane.futureTotal)} events are dated after today (latest ${monthLabel(lane.futureLast)}) and are not drawn`} /> : null}
                      {valueOf(lane)}
                    </span>
                    <small className="nx-tl__peak">{noteOf(lane)}</small>
                  </span>
                </li>
              ))}
            </ol>
            {tip && relative !== null && !drag ? (
              <div className="nx-tl__tip" style={tip.x > tip.w * 0.55 ? { insetInlineEnd: tip.w - tip.x + 14, insetBlockStart: Math.max(0, tip.y - 20) } : { insetInlineStart: tip.x + 14, insetBlockStart: Math.max(0, tip.y - 20) }} aria-hidden="true">
                <b>{cellLabel(groups[relative])}</b>
                <ul>
                  {lanes.map((lane, laneIndex) => ({ lane, laneIndex, cell: lane.cells[relative] })).filter(item => item.cell.state === 'in' || item.cell.state === 'unread').sort((left, right) => right.cell.total - left.cell.total).map(item => (
                    <li key={item.lane.caseId}><i style={{ background: laneColour(item.laneIndex) }} /><span>{nameOf(item.lane.caseId)}</span><b>{item.cell.state === 'unread' ? 'not read' : formatNumber(item.cell.total)}</b></li>
                  ))}
                </ul>
              </div>
            ) : null}
            {axis ? (
              <div className="nx-tl__axis" aria-hidden="true">
                <span className="nx-tl__legend">{anyPartial ? <><i className="nx-tl__dash" />not read</> : null}</span>
                <span className="nx-tl__ticks">{ticks.map(tick => <b key={tick.index} style={{ '--k': tick.index }}>{tick.label}</b>)}</span>
              </div>
            ) : null}
          </div>
          {axis ? (
            <div className="nx-tl__overview" aria-hidden="true">
              <span className="nx-tl__legend">All cases</span>
              <Overview totals={totals} from={from} to={to} onPick={choose} />
              <span className="nx-tl__range">{monthLabel(axis.groups[0].from).slice(-4)}–{monthLabel(axis.groups[whole - 1].to).slice(-4)}</span>
            </div>
          ) : null}
          <p className="nx-visually-hidden" role="status" data-readout="">{readout}</p>
        </>
      ) : <Calm icon={CircleAlert}>{failures.length ? 'The timeline could not be read' : 'No dated events yet'}</Calm>}
    </Panel>
  )
}

// ---------- what the evidence holds: sorted bars, because there are more than five kinds and a bar is the form that reads exact shares ----------
export function EvidenceTypesPanel({ mix, indexed, loading }) {
  const kinds = mix.kinds.slice().sort((left, right) => right.files - left.files || left.label.localeCompare(right.label))
  const max = Math.max(1, ...kinds.map(kind => kind.files))
  return (
    <Panel id="dash-types" title="Evidence by type" sub={mix.total ? plural(mix.total, 'file') : null} subTitle={indexed ? `${formatNumber(indexed)} items are indexed for search` : undefined}
      twin={mix.total ? { label: 'Files by type', exportName: 'nexusai-evidence-by-type.csv', summary: `${plural(mix.total, 'file')} in all: ${kinds.map(kind => `${formatNumber(kind.files)} ${kind.label.toLowerCase()}`).join(', ')}.`, columns: [{ key: 'type', label: 'Type' }, { key: 'files', label: 'Files' }, { key: 'failed', label: 'Did not finish' }, { key: 'share', label: 'Share' }], rows: kinds.map(kind => ({ id: kind.id, type: kind.label, files: kind.files, failed: kind.failed, share: formatRate(kind.files / mix.total) })) } : undefined}>
      {loading && !mix.total ? <PanelSkeleton /> : mix.total ? (
        <ul className="nx-hbars">
          {kinds.map((kind, index) => (
            <li key={kind.id} title={kind.failed ? `${formatNumber(kind.failed)} did not finish` : undefined}>
              <i className="nx-hbars__key" style={{ background: colourOf(kind) }} />
              <span className="nx-hbars__name">{kind.label}</span>
              <span className="nx-hbars__track" aria-hidden="true"><i style={{ inlineSize: `${Math.max(1.5, (kind.files / max) * 100)}%`, background: colourOf(kind), '--i': index }} /></span>
              <span className="nx-hbars__value nx-tabular">{formatNumber(kind.files)}</span>
              <span className="nx-hbars__share nx-tabular">{formatRate(kind.files / mix.total)}</span>
            </li>
          ))}
        </ul>
      ) : <Calm icon={CircleAlert}>No files read yet</Calm>}
    </Panel>
  )
}

const KIND_COLUMNS = [{ id: 'calls', label: 'Calls' }, { id: 'sessions', label: 'Internet' }, { id: 'usage', label: 'App use' }, { id: 'other', label: 'Other' }]
const MATRIX_ROWS = 5

// Records by kind, as a heat matrix: one row for each case (the largest first), one column for each kind, and a last row for all cases. The number is in every cell (the shade only helps the eye);
// shading is on a log scale so one huge cell does not turn the others blank, and a dot means none.
export function RecordKindsPanel({ kinds, loading }) {
  const nameOf = useCaseNameOf()
  const rows = kinds.byCase.slice(0, MATRIX_ROWS)
  const cellMax = Math.max(1, ...rows.flatMap(row => Object.values(row.values)))
  const totals = Object.fromEntries(kinds.kinds.map(kind => [kind.id, kind.count]))
  return (
    <Panel id="dash-kinds" title="Records by kind" sub={kinds.total ? `${plural(kinds.total, 'record')} · log shading` : null}
      twin={kinds.total ? { label: 'Records of each kind', exportName: 'nexusai-records-by-kind.csv', summary: `${plural(kinds.total, 'record')} in all: ${kinds.kinds.map(kind => `${formatNumber(kind.count)} ${kind.label.toLowerCase()}`).join(', ')}.`, columns: [{ key: 'case', label: 'Case' }, ...KIND_COLUMNS.map(column => ({ key: column.id, label: column.label }))], rows: kinds.byCase.map(row => ({ id: row.caseId, case: nameOf(row.caseId), ...row.values })) } : undefined}>
      {loading && !kinds.total ? <PanelSkeleton /> : kinds.total ? (
        <table className="nx-heat">
          <caption className="nx-visually-hidden">Records of each kind in each case; shading is on a log scale and a dot means none</caption>
          <thead><tr><th scope="col"><span className="nx-visually-hidden">Case</span></th>{KIND_COLUMNS.map(column => <th key={column.id} scope="col">{column.label}</th>)}</tr></thead>
          <tbody>
            {rows.map(row => (
              <tr key={row.caseId}>
                <th scope="row"><Link to={`/cases/${encodeURIComponent(row.caseId)}/overview`}><CaseLabel caseId={row.caseId} /></Link></th>
                {KIND_COLUMNS.map(column => <Cell key={column.id} value={row.values[column.id]} share={heatShare(row.values[column.id], cellMax)} />)}
              </tr>
            ))}
            <tr className="nx-heat__total">
              <th scope="row">All cases</th>
              {KIND_COLUMNS.map(column => <td key={column.id} className="nx-tabular">{totals[column.id] ? formatNumber(totals[column.id]) : <span aria-label="none">·</span>}</td>)}
            </tr>
          </tbody>
        </table>
      ) : <Calm icon={CircleAlert}>No records typed yet</Calm>}
    </Panel>
  )
}

function Cell({ value, share }) {
  return <td className="nx-heat__cell nx-tabular" style={share ? { '--p': `${share}%` } : undefined}>{value ? formatNumber(value) : <span aria-label="none">·</span>}</td>
}

// ---------- answers: what the engine did with the questions of the last days ----------
// One question, "were my questions answered, and if not, why?". A gauge says the share (one half-circle cut into answered, declined and abstained, so the whole is seen at once), a stacked area says how the
// week went (the same three, one value for each 24-hour period, stacked so the height is the number of questions), and under them the part an analyst can act on: the reasons for declines, ranked, and how long
// answers took. One readout serves all of it: pointing at a part of the gauge, a legend item, a day of the trend or a reason lights it, dims the rest, and says what it is (in the gauge, or on the axis of the trend);
// a click on an outcome keeps it lit. Every share is shown with the count behind it. The 24-hour periods are rolling windows ending now, from the engine's own audit counts, so most of a short history can fall in
// the last one: that is drawn as it is. The times are those of the case with the most questions (percentiles cannot be merged across cases).
const OUTCOMES = [
  { id: 'answered', label: 'Answered' },
  { id: 'declined', label: 'Declined' },
  { id: 'abstained', label: 'Abstained' },
]
const GAUGE = { cx: 100, cy: 92, r: 76 }

// The rows of "not answered": the three most common reasons for a decline, the declines no reason covers, and the questions the engine held back.
export function exceptionRows(quality) {
  const top = (quality.reasons || []).slice(0, 3)
  const rest = (quality.declined || 0) - top.reduce((sum, item) => sum + item.count, 0)
  const rows = top.map((item, index) => ({ id: `r${index}`, outcome: 'declined', label: item.text, count: item.count }))
  if (rest > 0) rows.push({ id: 'rx', outcome: 'declined', label: top.length ? 'Other reasons' : 'No reason recorded', count: rest })
  if ((quality.abstained || 0) > 0) rows.push({ id: 'abstained', outcome: 'abstained', label: 'Abstained, no reason recorded', count: quality.abstained })
  return rows
}

// A point on the half-circle: 0 is the left end, 1 the right end.
const arcPoint = fraction => {
  const angle = Math.PI * (1 - fraction)
  return [GAUGE.cx + GAUGE.r * Math.cos(angle), GAUGE.cy - GAUGE.r * Math.sin(angle)]
}
export function gaugePath(from, to) {
  const [x0, y0] = arcPoint(from)
  const [x1, y1] = arcPoint(to)
  return `M${x0.toFixed(2)} ${y0.toFixed(2)} A${GAUGE.r} ${GAUGE.r} 0 0 1 ${x1.toFixed(2)} ${y1.toFixed(2)}`
}

function Gauge({ quality, lit, onHover, onPick }) {
  const total = quality.questions || 1
  let at = 0
  const parts = OUTCOMES.filter(outcome => quality[outcome.id] > 0).map(outcome => {
    const from = at / total
    at += quality[outcome.id]
    return { ...outcome, from, to: at / total }
  })
  const gap = parts.length > 1 ? 0.006 : 0
  return (
    <svg className="nx-gauge__svg" viewBox="0 0 200 104" role="img" aria-label={`${formatRate(quality.rate)} of ${plural(quality.questions, 'question')} answered, ${formatNumber(quality.declined)} declined, ${formatNumber(quality.abstained)} abstained`} data-lit={lit || undefined}>
      <path className="nx-gauge__track" d={gaugePath(0, 1)} />
      {parts.map(part => (
        <path key={part.id} data-o={part.id} className={`nx-gauge__part${lit === part.id ? ' is-on' : ''}`} d={gaugePath(part.from + (part.from > 0 ? gap : 0), part.to - (part.to < 1 ? gap : 0))} onMouseEnter={() => onHover(part.id)} onClick={() => onPick(part.id)} />
      ))}
    </svg>
  )
}

// The week as a stacked area: one value for each 24-hour period, oldest first. The hit columns carry the readout.
function Trend({ days, lit, day, onDay }) {
  const totals = days.map(item => item.answered + item.declined + item.abstained)
  const top = Math.max(1, ...totals)
  const x = index => (index / (days.length - 1)) * 100
  const y = value => 38 - (value / top) * 34
  const layer = (lower, upper) => {
    const forward = days.map((item, index) => `${x(index).toFixed(2)} ${y(upper(item)).toFixed(2)}`)
    const back = days.map((item, index) => `${x(index).toFixed(2)} ${y(lower(item)).toFixed(2)}`).reverse()
    return `M${forward.join(' L')} L${back.join(' L')} Z`
  }
  const layers = [
    { id: 'answered', d: layer(() => 0, item => item.answered) },
    { id: 'declined', d: layer(item => item.answered, item => item.answered + item.declined) },
    { id: 'abstained', d: layer(item => item.answered + item.declined, item => item.answered + item.declined + item.abstained) },
  ]
  return (
    <svg className="nx-trend__svg" viewBox="0 0 100 40" preserveAspectRatio="none" aria-hidden="true" data-lit={lit || undefined}>
      <line className="nx-trend__base" x1="0" x2="100" y1="38" y2="38" vectorEffect="non-scaling-stroke" />
      {layers.map(item => <path key={item.id} data-o={item.id} className={`nx-trend__layer${lit === item.id ? ' is-on' : ''}`} d={item.d} />)}
      {day !== null ? <line className="nx-trend__cursor" x1={x(day)} x2={x(day)} y1="0" y2="38" vectorEffect="non-scaling-stroke" /> : null}
      {days.map((item, index) => <rect key={item.key} className="nx-trend__hit" x={x(index) - 50 / (days.length - 1)} width={100 / (days.length - 1)} y="0" height="40" onMouseEnter={() => onDay(index)} />)}
    </svg>
  )
}

export function AnswersPanel({ quality, daily, loading }) {
  const asked = quality.questions > 0
  const latency = quality.latency
  const [pinned, setPinned] = useState(null)
  const [hover, setHover] = useState(null)
  const [day, setDay] = useState(null)
  const active = hover || pinned
  const exceptions = useMemo(() => exceptionRows(quality), [quality])
  const share = value => formatRate(quality.questions ? value / quality.questions : 0)
  const ago = item => (item.ago === 0 ? 'Last 24 hours' : `${item.ago} ${item.ago === 1 ? 'day' : 'days'} ago`)
  const week = useMemo(() => (daily && daily.length === 7 && daily.some(item => item.questions) ? daily.map(item => ({ key: item.ago, ago: item.ago, answered: item.answered, declined: item.declined, abstained: item.abstained })) : null), [daily])
  const dailyRows = (daily || []).map(item => ({ id: `d${item.ago}`, measure: `Questions, ${ago(item).toLowerCase()}`, value: `${formatNumber(item.questions)} (${formatNumber(item.answered)} answered, ${formatNumber(item.declined)} declined, ${formatNumber(item.abstained)} abstained)` }))
  const top = Math.max(1, ...exceptions.map(item => item.count))
  const rows = asked ? [
    { id: 'answered', measure: 'Answered', value: formatNumber(quality.answered) },
    { id: 'declined', measure: 'Declined', value: formatNumber(quality.declined) },
    { id: 'abstained', measure: 'Abstained', value: formatNumber(quality.abstained) },
    ...quality.reasons.map((item, index) => ({ id: `reason${index}`, measure: index ? 'Reason for a decline' : 'Most common reason for a decline', value: `${item.text} (${formatNumber(item.count)})` })),
    ...(latency ? [{ id: 'median', measure: 'Median time to answer', value: formatDuration(latency.p50) }, { id: 'p90', measure: 'Nine in ten within', value: formatDuration(latency.p90) }, { id: 'max', measure: 'Slowest', value: formatDuration(latency.max) }] : []),
    ...dailyRows,
  ] : []
  const outcomeOf = key => (OUTCOMES.some(item => item.id === key) ? key : exceptions.find(item => item.id === key)?.outcome)
  const sentence = key => {
    if (!key) return `of ${formatNumber(quality.questions)} questions`
    const outcome = OUTCOMES.find(item => item.id === key)
    if (outcome) return `${outcome.label} · ${formatNumber(quality[key])} · ${share(quality[key])}`
    const row = exceptions.find(item => item.id === key)
    return row ? `${row.label} · ${formatNumber(row.count)} · ${share(row.count)}` : ''
  }
  const lit = outcomeOf(active)
  const toggle = key => setPinned(current => (current === key ? null : key))
  const dayText = day === null || !week ? '' : `${ago(week[day])} · ${formatNumber(week[day].answered)} answered · ${formatNumber(week[day].declined)} declined${week[day].abstained ? ` · ${formatNumber(week[day].abstained)} abstained` : ''}`
  return (
    <Panel id="dash-answers" title="Answers" sub="Last 7 days"
      twin={asked ? { label: 'Answers in the last 7 days', exportName: 'nexusai-answers-7-days.csv', summary: `${plural(quality.questions, 'question')} in the last 7 days: ${formatNumber(quality.answered)} answered, ${formatNumber(quality.declined)} declined, ${formatNumber(quality.abstained)} abstained.`, columns: [{ key: 'measure', label: 'Measure' }, { key: 'value', label: 'Value' }], rows } : undefined}>
      {loading && !quality.reported ? <PanelSkeleton /> : asked ? (
        <div className="nx-ans" onMouseLeave={() => { setHover(null); setDay(null) }}>
          <div className="nx-ans__hero">
            <div className="nx-gauge">
              <Gauge quality={quality} lit={lit} onHover={key => { setHover(key); setDay(null) }} onPick={toggle} />
              <div className="nx-gauge__read" aria-hidden="true">
                <b className="nx-tabular">{formatRate(quality.rate)}</b>
                <span className={active ? 'is-probe' : undefined}>{sentence(active)}</span>
              </div>
            </div>
            <div className="nx-ans__legend" role="group" aria-label="Light up one outcome">
              {OUTCOMES.map(outcome => (
                <button key={outcome.id} type="button" data-o={outcome.id} aria-pressed={pinned === outcome.id} disabled={!quality[outcome.id]}
                  onMouseEnter={() => setHover(outcome.id)} onFocus={() => setHover(outcome.id)} onBlur={() => setHover(null)} onClick={() => toggle(outcome.id)}>
                  <i aria-hidden="true" />{outcome.label}<b className="nx-tabular">{formatNumber(quality[outcome.id])}</b>
                </button>
              ))}
            </div>
          </div>
          <div className="nx-ans__side">
            {week ? (
              <figure className="nx-trend" onMouseLeave={() => setDay(null)}>
                <Trend days={week} lit={lit} day={day} onDay={index => { setDay(index); setHover(null) }} />
                <figcaption className={`nx-trend__axis${day !== null ? ' is-probe' : ''}`} aria-hidden="true">{day !== null ? <span>{dayText}</span> : <><span>7 days ago</span><span>now</span></>}</figcaption>
              </figure>
            ) : null}
            {exceptions.length ? (
              <section className="nx-ans__rest" aria-label="The questions that were not answered, by reason">
                <h3>Not answered<b className="nx-tabular">{formatNumber((quality.declined || 0) + (quality.abstained || 0))}</b></h3>
                <ol>
                  {exceptions.map((item, index) => (
                    <li key={item.id} data-o={item.outcome} title={item.label} aria-pressed={pinned === item.id} onMouseEnter={() => { setHover(item.id); setDay(null) }} onClick={() => toggle(item.id)}>
                      <span className="nx-ans__reason">{item.label}</span>
                      <b className="nx-tabular">{formatNumber(item.count)}<small>{share(item.count)}</small></b>
                      <span className="nx-ans__track" aria-hidden="true"><i style={{ inlineSize: `${Math.max(4, (item.count / top) * 100)}%`, '--i': index }} /></span>
                    </li>
                  ))}
                </ol>
              </section>
            ) : null}
            {latency ? (
              <p className="nx-ans__time nx-tabular" title="Time to answer in the case with the most questions">
                <span>median <b>{formatDuration(latency.p50)}</b></span>
                {latency.p90 != null ? <span>90% within <b>{formatDuration(latency.p90)}</b></span> : null}
                {latency.max != null ? <span>slowest <b>{formatDuration(latency.max)}</b></span> : null}
              </p>
            ) : null}
          </div>
        </div>
      ) : <Calm icon={MessageSquareText}>No questions in the last 7 days</Calm>}
    </Panel>
  )
}

// ---------- data quality: what was set aside, and how the times are known ----------
// Rows set aside is a dumbbell (exactly two values for each case: the rejected share, and the rejected plus exact-repeat share) on one axis that starts at zero; the four cases with the most set aside
// are drawn, the rest are in the table. The basis of the times is one stacked bar with its counts beside it.
export function DataQualityPanel({ aside, basis, loading, basisLoading }) {
  const nameOf = useCaseNameOf()
  const shown = aside.items.filter(item => item.share > 0).slice(0, 5)
  const axisMax = niceCeiling(Math.max(0.1, ...shown.map(item => item.share * 100)))
  return (
    <Panel id="dash-quality" title="Data quality" sub={aside.read ? `${formatRate(aside.share)} of ${formatNumber(aside.read)} rows set aside` : null}
      twin={{ label: 'Data quality for each case', exportName: 'nexusai-data-quality.csv', summary: `${formatRate(aside.share)} of ${formatNumber(aside.read)} rows read were set aside; the times of ${formatNumber(basis.total)} typed rows are known as: ${basis.parts.map(part => `${formatNumber(part.rows)} ${part.label.toLowerCase()}`).join(', ') || 'not yet read'}.`, columns: [{ key: 'case', label: 'Case' }, { key: 'read', label: 'Rows read' }, { key: 'repeats', label: 'Exact repeats' }, { key: 'rejected', label: 'Rejected' }, { key: 'share', label: 'Share set aside' }], rows: aside.items.map(item => ({ id: item.caseId, case: nameOf(item.caseId), read: item.read, repeats: item.duplicates, rejected: item.rejected, share: formatRate(item.share) })) }}>
      {loading && !aside.read ? <PanelSkeleton /> : (
        <>
          <div className="nx-quality__aside nx-fill">
            {shown.length ? (
              <>
                <p className="nx-keyline"><span><i className="nx-keys__key nx-keys__key--dot" style={{ background: 'var(--nx-accent)' }} />Rejected</span><span><i className="nx-keys__key nx-keys__key--ring" />Rejected and exact repeats</span></p>
                <ul className="nx-aside">
                  {shown.map(item => (
                    <li key={item.caseId} className="nx-aside__row" title={`${formatNumber(item.rejected)} rejected, ${formatNumber(item.duplicates)} exact repeats, of ${formatNumber(item.read)} rows read`}>
                      <Link className="nx-aside__name" to={`/cases/${encodeURIComponent(item.caseId)}/overview`}><CaseLabel caseId={item.caseId} /></Link>
                      <Dumbbell low={(item.rejected / item.read) * 100} high={item.share * 100} axisMax={axisMax} label={`${formatRate(item.rejected / item.read)} rejected, ${formatRate(item.share)} rejected or exact repeats`} />
                      <span className="nx-aside__value nx-tabular">{formatRate(item.share)}</span>
                    </li>
                  ))}
                </ul>
                <p className="nx-axis nx-tabular" aria-hidden="true"><i /><span className="nx-axis__scale"><span>0%</span><span>{formatRate(axisMax / 100)}</span></span><i /></p>
              </>
            ) : aside.read ? <Calm icon={CircleCheckBig} tone="ok"><strong>Every row was kept</strong></Calm> : <Calm icon={CircleAlert}>No rows read yet</Calm>}
          </div>
          <div className="nx-group nx-quality__basis">
            <p className="nx-overline">How the times are known</p>
            {basisLoading && !basis.total ? <PanelSkeleton rows={1} /> : basis.total ? (
              <>
                <SegBar height={8} label="Rows by the basis of their time" parts={basis.parts.map((part, index) => ({ id: part.id, value: part.rows, colour: `var(--nx-data-${index + 1})`, title: `${part.label}: ${formatNumber(part.rows)} rows` }))} />
                <ul className="nx-keyline nx-keyline--counts">
                  {basis.parts.map((part, index) => <li key={part.id} title={`${part.label}: ${formatNumber(part.rows)} rows`}><i className="nx-keys__key" style={{ background: `var(--nx-data-${index + 1})` }} />{BASIS_SHORT[part.id] || part.label}<b className="nx-tabular">{formatRate(part.rows / basis.total)}</b></li>)}
                </ul>
              </>
            ) : <p className="nx-muted">No typed records yet</p>}
          </div>
        </>
      )}
    </Panel>
  )
}

// ---------- the pipeline: how long files take from upload to ready ----------
// Waiting (the file sat in the queue) and processing (the file was being read) are told apart in the indicator above; here every case has its strip on one axis, with a hairline at the median of all files.
export function TimeToReadyPanel({ data, loading }) {
  const nameOf = useCaseNameOf()
  const cases = data.cases.slice().sort((left, right) => right.median - left.median).slice(0, 6)
  const axisMax = niceCeiling(Math.max(1, ...cases.map(item => item.max)))
  return (
    <Panel id="dash-ready" title="Time to ready" sub={data.count ? `Median ${formatDuration(data.median)}` : null}
      twin={cases.length ? { label: 'Time from upload to ready for each case', exportName: 'nexusai-time-to-ready.csv', summary: `Median ${formatDuration(data.median)} from upload to ready over ${plural(data.count, 'file')}: ${formatDuration(data.waitMedian)} waiting in the queue and ${formatDuration(data.processMedian)} being read; the slowest took ${formatDuration(data.slowest)}.`, columns: [{ key: 'case', label: 'Case' }, { key: 'files', label: 'Files' }, { key: 'fastest', label: 'Fastest' }, { key: 'median', label: 'Median' }, { key: 'slowest', label: 'Slowest' }], rows: data.cases.map(item => ({ id: item.caseId, case: nameOf(item.caseId), files: item.count, fastest: formatDuration(item.min), median: formatDuration(item.median), slowest: formatDuration(item.max) })) } : undefined}>
      {loading && !data.count ? <PanelSkeleton /> : cases.length ? (
        <>
          <ul className="nx-aside nx-fill">
            {cases.map(item => (
              <li key={item.caseId} className="nx-aside__row" title={`${plural(item.count, 'file')}: fastest ${formatDuration(item.min)}, slowest ${formatDuration(item.max)}`}>
                <Link className="nx-aside__name" to={`/cases/${encodeURIComponent(item.caseId)}/evidence`}><CaseLabel caseId={item.caseId} /></Link>
                <RangeStrip stats={item} axisMax={axisMax} reference={data.median} label={`Median ${formatDuration(item.median)}, from ${formatDuration(item.min)} to ${formatDuration(item.max)}, over ${plural(item.count, 'file')}`} />
                <span className="nx-aside__value nx-tabular">{formatDuration(item.median)}</span>
              </li>
            ))}
          </ul>
          <p className="nx-axis nx-tabular" aria-hidden="true"><i /><span className="nx-axis__scale"><span>0</span><span>{formatDuration(axisMax)}</span></span><i /></p>
          <p className="nx-keyline"><span><i className="nx-keys__key nx-keys__key--dot" style={{ background: 'var(--nx-accent)' }} />Median</span><span><i className="nx-keys__key nx-keys__key--bar" />Middle half</span><span><i className="nx-keys__key nx-keys__key--line" />Range</span></p>
        </>
      ) : <Calm icon={CircleAlert}>No finished files yet</Calm>}
    </Panel>
  )
}
