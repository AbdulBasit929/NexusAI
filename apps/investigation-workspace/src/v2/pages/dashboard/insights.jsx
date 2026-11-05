import { useMemo, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { CircleAlert, CircleCheckBig, MessageSquareText } from 'lucide-react'
import { formatNumber } from '../../../lib/format.js'
import { formatRate } from '../../../lib/dashboardCharts.js'
import { CaseLabel, useCaseNameOf } from '../../shell/CaseLabel.jsx'
import { Calm, Panel, PanelSkeleton } from '../../ui/Panel.jsx'
import { ColumnChart, Dumbbell, RangeStrip, SegBar } from '../../ui/micro.jsx'
import { alignTimelines, formatDuration, heatShare, niceCeiling } from './model.js'

// Every panel here is drawn for a cell of a fixed size (the board gives it one; see styles/board.css): its picture takes the height that is left, its words are a title, a note and the numbers
// beside the marks, and the same figures as a table are one button away in a dialog. Nothing in a panel grows with its content, so neighbours always share a top and a bottom.
const MONTH = new Intl.DateTimeFormat('en-GB', { month: 'short', year: 'numeric', timeZone: 'UTC' })
const monthLabel = key => MONTH.format(new Date(`${key.slice(0, 7)}-01T00:00:00Z`))
const plural = (count, one, many = `${one}s`) => `${formatNumber(count)} ${count === 1 ? one : many}`
const colourOf = kind => (kind.slot ? `var(--nx-data-${kind.slot})` : 'var(--nx-data-muted)')
const TIMELINE_ROWS = 6
const BASIS_SHORT = { confirmed_by_analyst: 'Analyst-confirmed', explicit_offset: 'Stated in file', declared: 'Source-declared', assumed: 'Assumed' }

// ---------- the evidence timeline: when the events of each case happened, every case on one calendar ----------
// The lanes share one calendar (it runs from the earliest month any case has to this month), so where the cases overlap in time can be read at a glance. "Own scale" lets each lane's busiest month fill
// its height (the shape of the case); "Shared scale" gives every lane one scale (the size). Pointing at the plot, or using the arrow keys on it, reads one month: the figure at the end of each lane
// becomes that month's figure and the title names the month, so nothing floats over the data. A case the service returned only in part ends in a dotted run, which means "not read", never "empty".
// A date in the future is named and left out, and the table lists the same figures.
const cellLabel = cell => (cell.from === cell.to ? monthLabel(cell.from) : `${monthLabel(cell.from)} – ${monthLabel(cell.to)}`)
const PARTIAL_NOTE = 'The service returns at most 100 rows for a case (one for each day and kind of record), oldest first. Only those are drawn and counted; the dotted run is the part that was not read.'

function Lane({ lane, max, active, label }) {
  return (
    <span className="nx-lane" role="img" aria-label={label} data-at={active === null ? undefined : ''} style={active === null ? undefined : { '--at': active }}>
      {lane.cells.map((cell, index) => {
        const drawn = cell.state === 'in' && cell.total > 0
        const tone = cell.state !== 'in' ? `is-${cell.state}` : drawn ? '' : 'is-empty'
        const className = [tone, index === active ? 'is-on' : ''].filter(Boolean).join(' ')
        return <i key={cell.from} className={className || undefined} style={drawn ? { '--h': `${Math.max(6, (cell.total / max) * 100)}%`, '--i': Math.min(index, 24) } : undefined} />
      })}
    </span>
  )
}

export function TimelinePanel({ timelines, loading, failures, today }) {
  const nameOf = useCaseNameOf()
  const [scale, setScale] = useState('own')
  const [active, setActive] = useState(null)
  const plot = useRef(null)
  const axis = useMemo(() => alignTimelines(timelines.slice(0, TIMELINE_ROWS), today), [timelines, today])
  const lanes = axis ? axis.lanes : []
  const count = axis ? axis.groups.length : 0
  const anyPartial = lanes.some(lane => lane.partial)
  const sharedMax = Math.max(1, ...lanes.flatMap(lane => lane.cells.map(cell => cell.total)))
  const twin = timelines.map(item => ({ id: item.caseId, case: nameOf(item.caseId), from: item.first ? monthLabel(item.first) : '', to: item.last ? monthLabel(item.last) : '', events: item.total, note: [item.partial ? 'earliest rows only' : '', item.futureTotal ? `${formatNumber(item.futureTotal)} events dated after today are not drawn` : ''].filter(Boolean).join('; ') }))
  const valueOf = lane => {
    if (active === null) return `${formatNumber(lane.total)}${lane.partial ? '+' : ''}`
    const cell = lane.cells[active]
    return cell.state === 'unread' ? 'not read' : formatNumber(cell.total)
  }
  const indexAt = event => {
    const strip = plot.current?.querySelector('.nx-lane')
    if (!strip || !count) return null
    const box = strip.getBoundingClientRect()
    if (box.width <= 0 || event.clientX < box.left || event.clientX > box.right) return null
    return Math.min(count - 1, Math.floor(((event.clientX - box.left) / box.width) * count))
  }
  const onKeyDown = event => {
    if (!count || event.target !== event.currentTarget) return
    const step = { ArrowLeft: -1, ArrowRight: 1 }[event.key]
    if (step) { event.preventDefault(); setActive(current => Math.min(count - 1, Math.max(0, (current ?? (step > 0 ? -1 : count)) + step))) }
    else if (event.key === 'Home') { event.preventDefault(); setActive(0) }
    else if (event.key === 'End') { event.preventDefault(); setActive(count - 1) }
    else if (event.key === 'Escape') setActive(null)
  }
  const readout = active === null || !axis ? '' : `${cellLabel(axis.groups[active])}: ${lanes.map(lane => `${nameOf(lane.caseId)} ${valueOf(lane)}`).join(', ')}`
  const toggle = (
    <div className="nx-segmented" role="group" aria-label="Scale of the lanes">
      <button type="button" aria-pressed={scale === 'own'} onClick={() => setScale('own')} aria-label="Own scale" title="Own scale: each lane's busiest month fills the lane, the shape of each case">Own</button>
      <button type="button" aria-pressed={scale === 'shared'} onClick={() => setScale('shared')} aria-label="Shared scale" title="Shared scale: one scale for every lane, the size of each case against the others">Shared</button>
    </div>
  )
  return (
    <Panel id="dash-timeline" title="Evidence timeline" sub={active === null || !axis ? 'Events per month' : cellLabel(axis.groups[active])} action={timelines.length ? toggle : null} className="nx-panel--timeline"
      twin={timelines.length ? { label: 'Events per case by month range', exportName: 'nexusai-evidence-timeline.csv', summary: `Events per month for ${plural(timelines.length, 'case')}; ${plural(timelines.filter(item => item.partial).length, 'case')} read only as far as the service returns.`, columns: [{ key: 'case', label: 'Case' }, { key: 'from', label: 'From' }, { key: 'to', label: 'To' }, { key: 'events', label: 'Events' }, { key: 'note', label: 'Note' }], rows: twin } : undefined}>
      {loading && !timelines.length ? <PanelSkeleton rows={4} /> : timelines.length ? (
        <>
          <div
            ref={plot}
            className="nx-tl"
            style={{ '--n': count }}
            data-reading={active === null ? undefined : ''}
            tabIndex={axis ? 0 : undefined}
            role="group"
            aria-label="Events per month in each case. Use the left and right arrow keys to read one month."
            onPointerMove={event => setActive(indexAt(event))}
            onPointerLeave={() => setActive(null)}
            onKeyDown={onKeyDown}
            onBlur={event => { if (!event.currentTarget.contains(event.relatedTarget)) setActive(null) }}
          >
            <ol className="nx-tl__lanes">
              {lanes.map(lane => (
                <li key={lane.caseId} className="nx-tl__row">
                  <span className="nx-tl__who">
                    <Link className="nx-tl__name" to={`/cases/${encodeURIComponent(lane.caseId)}/timeline`} title={lane.partial ? PARTIAL_NOTE : undefined}><CaseLabel caseId={lane.caseId} /></Link>
                  </span>
                  <Lane lane={lane} active={active} max={scale === 'shared' ? sharedMax : Math.max(1, ...lane.cells.map(cell => cell.total))}
                    label={`Events by month from ${lane.first ? monthLabel(lane.first) : '—'} to ${lane.last ? monthLabel(lane.last) : '—'}: ${formatNumber(lane.total)}${lane.partial ? ', the earliest rows only' : ''}`} />
                  <span className={`nx-tl__count nx-tabular${active !== null ? ' is-reading' : ''}`}>
                    {lane.futureTotal && active === null ? <CircleAlert className="nx-tl__warn" aria-label={`${formatNumber(lane.futureTotal)} events are dated after today (latest ${monthLabel(lane.futureLast)}) and are not drawn`} /> : null}
                    {valueOf(lane)}
                  </span>
                </li>
              ))}
            </ol>
            {axis ? (
              <div className="nx-tl__axis" aria-hidden="true">
                <span className="nx-tl__legend">{anyPartial ? <><i className="nx-tl__dash" />not read</> : null}</span>
                <span className="nx-tl__ticks">{axis.ticks.map(tick => <b key={tick.index} style={{ '--k': tick.index }}>{tick.year}</b>)}</span>
              </div>
            ) : null}
          </div>
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
// Every share is shown with the count behind it, so a small week is not read as a trend. The per-day columns are 24-hour periods ending now, from the engine's own audit counts; they take the height
// the cell leaves. The most common reason for a decline is in the tooltip of "Declined" and in the table.
export function AnswersPanel({ quality, daily, loading }) {
  const asked = quality.questions > 0
  const latency = quality.latency
  const ago = day => (day.ago === 0 ? 'Last 24 hours' : `${day.ago} ${day.ago === 1 ? 'day' : 'days'} ago`)
  const dailyRows = (daily || []).map(day => ({ id: `d${day.ago}`, measure: `Questions, ${ago(day).toLowerCase()}`, value: `${formatNumber(day.questions)} (${formatNumber(day.answered)} answered, ${formatNumber(day.declined)} declined, ${formatNumber(day.abstained)} abstained)` }))
  const reason = quality.reasons[0]
  const rows = asked ? [
    { id: 'answered', measure: 'Answered', value: formatNumber(quality.answered) },
    { id: 'declined', measure: 'Declined', value: formatNumber(quality.declined) },
    { id: 'abstained', measure: 'Abstained', value: formatNumber(quality.abstained) },
    ...(reason ? [{ id: 'reason', measure: 'Most common reason for a decline', value: `${reason.text} (${formatNumber(reason.count)})` }] : []),
    ...(latency ? [{ id: 'median', measure: 'Median time to answer', value: formatDuration(latency.p50) }, { id: 'p90', measure: 'Nine in ten within', value: formatDuration(latency.p90) }, { id: 'max', measure: 'Slowest', value: formatDuration(latency.max) }] : []),
    ...dailyRows,
  ] : []
  const keys = [
    { id: 'answered', label: 'Answered', value: quality.answered, colour: 'var(--nx-accent)' },
    { id: 'declined', label: 'Declined', value: quality.declined, colour: 'var(--nx-border-strong)', title: reason ? `Most declines: ${reason.text} (${formatNumber(reason.count)})` : undefined },
    { id: 'abstained', label: 'Abstained', value: quality.abstained, colour: 'var(--nx-border)' },
  ]
  return (
    <Panel id="dash-answers" title="Answers" sub="Questions · last 7 days"
      twin={asked ? { label: 'Answers in the last 7 days', exportName: 'nexusai-answers-7-days.csv', summary: `${plural(quality.questions, 'question')} in the last 7 days: ${formatNumber(quality.answered)} answered, ${formatNumber(quality.declined)} declined, ${formatNumber(quality.abstained)} abstained.`, columns: [{ key: 'measure', label: 'Measure' }, { key: 'value', label: 'Value' }], rows } : undefined}>
      {loading && !quality.reported ? <PanelSkeleton /> : asked ? (
        <>
          {daily && daily.some(day => day.questions) ? (
            <div className="nx-fill nx-answers__chart" dir="ltr">
              <ColumnChart height="100%" label={`Questions in each of the last seven 24-hour periods, oldest first: ${daily.map(day => formatNumber(day.questions)).join(', ')}`}
                columns={daily.map(day => ({ key: day.ago, title: `${ago(day)}: ${formatNumber(day.answered)} answered, ${formatNumber(day.declined)} declined, ${formatNumber(day.abstained)} abstained`, parts: [{ id: 'answered', value: day.answered, colour: 'var(--nx-accent)' }, { id: 'declined', value: day.declined, colour: 'var(--nx-border-strong)' }, { id: 'abstained', value: day.abstained, colour: 'var(--nx-border)' }] }))} />
            </div>
          ) : (
            <div className="nx-fill nx-answers__chart">
              <SegBar height={14} label="Questions answered, declined and abstained" parts={keys.map(key => ({ id: key.id, value: key.value, colour: key.colour, title: `${key.label}: ${formatNumber(key.value)}` }))} />
            </div>
          )}
          {daily && daily.some(day => day.questions) ? <p className="nx-cols__axis" aria-hidden="true"><span>6 days ago</span><span>Last 24 hours</span></p> : null}
          <ul className="nx-keyline nx-keyline--counts">
            {keys.map(key => <li key={key.id} title={key.title}><i className="nx-keys__key" style={{ background: key.colour }} />{key.label}<b className="nx-tabular">{formatNumber(key.value)}</b></li>)}
          </ul>
          {latency ? (
            <div className="nx-ladder">
              <SegBar height={6} label={`Median ${formatDuration(latency.p50)}, nine in ten within ${formatDuration(latency.p90)}, slowest ${formatDuration(latency.max)}`} parts={[
                { id: 'p50', value: latency.p50, colour: 'var(--nx-accent)', title: `Half within ${formatDuration(latency.p50)}` },
                { id: 'p90', value: Math.max(0, (latency.p90 ?? latency.p50) - latency.p50), colour: 'color-mix(in oklab, var(--nx-accent) 55%, var(--nx-surface))', title: `Nine in ten within ${formatDuration(latency.p90)}` },
                { id: 'max', value: Math.max(0, (latency.max ?? latency.p90 ?? latency.p50) - (latency.p90 ?? latency.p50)), colour: 'color-mix(in oklab, var(--nx-accent) 25%, var(--nx-surface))', title: `Slowest ${formatDuration(latency.max)}` },
              ]} />
              <p className="nx-ladder__marks nx-tabular" title="Time to answer in the case with the slowest answers"><span>median {formatDuration(latency.p50)}</span>{latency.p90 != null ? <span>90% within {formatDuration(latency.p90)}</span> : null}{latency.max != null ? <span>slowest {formatDuration(latency.max)}</span> : null}</p>
            </div>
          ) : null}
        </>
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
