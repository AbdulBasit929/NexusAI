import { useState } from 'react'
import { Link } from 'react-router-dom'
import { CircleAlert, CircleCheckBig, MessageSquareText } from 'lucide-react'
import { formatNumber } from '../../../lib/format.js'
import { formatRate } from '../../../lib/dashboardCharts.js'
import { CaseLabel, useCaseNameOf } from '../../shell/CaseLabel.jsx'
import { Calm, Panel, PanelSkeleton } from '../../ui/Panel.jsx'
import { BarStrip, ColumnChart, Dumbbell, RangeStrip, SegBar } from '../../ui/micro.jsx'
import { TableTwin } from '../../ui/TableTwin.jsx'
import { formatDuration, heatShare, niceCeiling } from './model.js'

const MONTH = new Intl.DateTimeFormat('en-GB', { month: 'short', year: 'numeric', timeZone: 'UTC' })
const monthLabel = key => MONTH.format(new Date(`${key.slice(0, 7)}-01T00:00:00Z`))
const plural = (count, one, many = `${one}s`) => `${formatNumber(count)} ${count === 1 ? one : many}`
const colourOf = kind => (kind.slot ? `var(--nx-data-${kind.slot})` : 'var(--nx-data-muted)')
const TIMELINE_ROWS = 8

// ---------- the evidence timeline: when the events of each case happened, one row for each case ----------
// Small multiples. Own scale shows the shape of each case (the busiest month fills the row); shared scale shows the size (one case cannot hide another). The analyst chooses; the count at the end
// is the exact total either way. A case the service could not read to its end says so, a date in the future is named and left out, and the table twin lists the same figures.
export function TimelinePanel({ timelines, loading, failures }) {
  const nameOf = useCaseNameOf()
  const [scale, setScale] = useState('own')
  const shown = timelines.slice(0, TIMELINE_ROWS)
  const sharedMax = Math.max(1, ...shown.flatMap(item => item.series.map(point => point.total)))
  const twin = timelines.map(item => ({ id: item.caseId, case: nameOf(item.caseId), from: item.first ? monthLabel(item.first) : '', to: item.last ? monthLabel(item.last) : '', events: item.total, note: [item.partial ? 'first 100 days only' : '', item.futureTotal ? `${formatNumber(item.futureTotal)} events dated after today are not drawn` : ''].filter(Boolean).join('; ') }))
  const toggle = (
    <div className="nx-segmented" role="group" aria-label="Scale of the rows">
      <button type="button" aria-pressed={scale === 'own'} onClick={() => setScale('own')} title="Each row's busiest month fills the row: the shape of each case">Own scale</button>
      <button type="button" aria-pressed={scale === 'shared'} onClick={() => setScale('shared')} title="One scale for every row: the size of each case against the others">Shared scale</button>
    </div>
  )
  return (
    <Panel id="dash-timeline" title="Evidence timeline" sub="Events per month" action={timelines.length ? toggle : null} className="nx-panel--timeline">
      {loading && !timelines.length ? <PanelSkeleton rows={4} /> : timelines.length ? (
        <>
          <ol className="nx-tl">
            {shown.map(item => (
              <li key={item.caseId} className="nx-tl__row">
                <span className="nx-tl__who">
                  <Link className="nx-tl__name" to={`/cases/${encodeURIComponent(item.caseId)}/timeline`}><CaseLabel caseId={item.caseId} /></Link>
                  {item.partial ? <span className="nx-tl__note" title="The service returns at most 100 days for a case; only those are drawn and counted.">first 100 days only</span> : null}
                </span>
                <BarStrip
                  max={scale === 'shared' ? sharedMax : undefined}
                  values={item.series.map(point => ({ key: point.from, value: point.total, title: `${point.from === point.to ? monthLabel(point.from) : `${monthLabel(point.from)} – ${monthLabel(point.to)}`}: ${formatNumber(point.total)} events` }))}
                  label={`Events by month from ${item.first ? monthLabel(item.first) : '—'} to ${item.last ? monthLabel(item.last) : '—'}: ${formatNumber(item.total)}`}
                />
                <span className="nx-tl__range">{item.first ? `${monthLabel(item.first)} – ${monthLabel(item.last)}` : '—'}</span>
                <span className="nx-tl__count nx-tabular">
                  {item.futureTotal ? <CircleAlert className="nx-tl__warn" aria-label={`${formatNumber(item.futureTotal)} events are dated after today (latest ${monthLabel(item.futureLast)}) and are not drawn`} /> : null}
                  {formatNumber(item.total)}{item.partial ? '+' : ''}
                </span>
              </li>
            ))}
          </ol>
          {timelines.length > shown.length ? <p className="nx-panel__note">{formatNumber(timelines.length - shown.length)} more cases are in the table.</p> : null}
          <TableTwin label="Events per case by month range" exportName="nexusai-evidence-timeline.csv" summary={`Events per month for ${plural(timelines.length, 'case')}; ${plural(timelines.filter(item => item.partial).length, 'case')} read only as far as the service returns.`}
            columns={[{ key: 'case', label: 'Case' }, { key: 'from', label: 'From' }, { key: 'to', label: 'To' }, { key: 'events', label: 'Events' }, { key: 'note', label: 'Note' }]} rows={twin} />
        </>
      ) : <Calm icon={CircleAlert}>{failures.length ? 'The timeline could not be read' : 'No dated events yet'}</Calm>}
    </Panel>
  )
}

// ---------- what the evidence holds ----------
export function EvidenceTypesPanel({ mix, indexed, loading }) {
  return (
    <Panel id="dash-types" title="Evidence by type" sub={mix.total ? `Files · ${formatNumber(mix.total)}` : null}>
      {loading && !mix.total ? <PanelSkeleton /> : mix.total ? (
        <>
          <SegBar height={10} label="Files by type" parts={mix.kinds.map(kind => ({ id: kind.id, value: kind.files, colour: colourOf(kind), title: `${kind.label}: ${formatNumber(kind.files)} files` }))} />
          <ul className="nx-keys">
            {mix.kinds.map(kind => (
              <li key={kind.id} title={kind.failed ? `${formatNumber(kind.failed)} did not finish` : undefined}>
                <i className="nx-keys__key" style={{ background: colourOf(kind) }} />
                <span className="nx-keys__name">{kind.label}</span>
                <span className="nx-keys__value nx-tabular">{formatNumber(kind.files)}</span>
                <span className="nx-keys__share nx-tabular">{formatRate(kind.files / mix.total)}</span>
              </li>
            ))}
          </ul>
          {indexed ? <p className="nx-panel__note">{formatNumber(indexed)} items are indexed for search.</p> : null}
          <TableTwin label="Files by type" exportName="nexusai-evidence-by-type.csv" summary={`${plural(mix.total, 'file')} in all: ${mix.kinds.map(kind => `${formatNumber(kind.files)} ${kind.label.toLowerCase()}`).join(', ')}.`}
            columns={[{ key: 'type', label: 'Type' }, { key: 'files', label: 'Files' }, { key: 'failed', label: 'Did not finish' }, { key: 'share', label: 'Share' }]}
            rows={mix.kinds.map(kind => ({ id: kind.id, type: kind.label, files: kind.files, failed: kind.failed, share: formatRate(kind.files / mix.total) }))} />
        </>
      ) : <Calm icon={CircleAlert}>No files read yet</Calm>}
    </Panel>
  )
}

const KIND_COLUMNS = [{ id: 'calls', label: 'Calls' }, { id: 'sessions', label: 'Internet' }, { id: 'usage', label: 'App use' }, { id: 'other', label: 'Other' }]

// Records by kind, as a heat matrix: one row for each case, one column for each kind. The number is in every cell (the shade only helps the eye); shading is on a log scale so one huge cell does not
// turn the others blank, and a blank cell means none. The first bars give the total of each kind.
export function RecordKindsPanel({ kinds, loading }) {
  const max = Math.max(1, ...kinds.kinds.map(kind => kind.count))
  const rows = kinds.byCase.slice(0, 6)
  const cellMax = Math.max(1, ...rows.flatMap(row => Object.values(row.values)))
  const nameOf = useCaseNameOf()
  return (
    <Panel id="dash-kinds" title="Records by kind" sub={kinds.total ? `Records · ${formatNumber(kinds.total)}` : null}>
      {loading && !kinds.total ? <PanelSkeleton /> : kinds.total ? (
        <>
          <ul className="nx-mix">
            {kinds.kinds.map(kind => (
              <li key={kind.id} className="nx-mix__row nx-mix__row--static" title={kind.id === 'other' ? 'Accepted rows that are not calls, internet sessions or app usage' : undefined}>
                <span className="nx-mix__label">{kind.label}</span>
                <span className="nx-mix__bar" aria-hidden="true"><i className={kind.id === 'other' ? 'is-muted' : undefined} style={{ inlineSize: `${Math.max(2, (kind.count / max) * 100)}%` }} /></span>
                <span className="nx-mix__value nx-tabular">{formatNumber(kind.count)}</span>
              </li>
            ))}
          </ul>
          {rows.length > 1 ? (
            <div className="nx-group">
              <p className="nx-overline">By case</p>
              <table className="nx-heat">
                <caption className="nx-visually-hidden">Records of each kind in each case; shading is on a log scale and blank means none</caption>
                <thead><tr><th scope="col"><span className="nx-visually-hidden">Case</span></th>{KIND_COLUMNS.map(column => <th key={column.id} scope="col">{column.label}</th>)}</tr></thead>
                <tbody>
                  {rows.map(row => (
                    <tr key={row.caseId}>
                      <th scope="row"><Link to={`/cases/${encodeURIComponent(row.caseId)}/overview`}><CaseLabel caseId={row.caseId} /></Link></th>
                      {KIND_COLUMNS.map(column => {
                        const value = row.values[column.id]
                        const share = heatShare(value, cellMax)
                        return <td key={column.id} className="nx-heat__cell nx-tabular" style={share ? { '--p': `${share}%` } : undefined}>{value ? formatNumber(value) : <span aria-label="none">·</span>}</td>
                      })}
                    </tr>
                  ))}
                </tbody>
              </table>
              <p className="nx-panel__note">Shading is on a log scale; a dot means none.</p>
            </div>
          ) : null}
          <TableTwin label="Records of each kind" exportName="nexusai-records-by-kind.csv" summary={`${plural(kinds.total, 'record')} in all: ${kinds.kinds.map(kind => `${formatNumber(kind.count)} ${kind.label.toLowerCase()}`).join(', ')}.`}
            columns={[{ key: 'case', label: 'Case' }, ...KIND_COLUMNS.map(column => ({ key: column.id, label: column.label }))]}
            rows={kinds.byCase.map(row => ({ id: row.caseId, case: nameOf(row.caseId), ...row.values }))} />
        </>
      ) : <Calm icon={CircleAlert}>No records typed yet</Calm>}
    </Panel>
  )
}

// ---------- answers: what the engine did with the questions of the last days ----------
// Every share is shown with the count behind it, so a small week is not read as a trend. The per-day columns are 24-hour periods ending now, from the engine's own audit counts.
export function AnswersPanel({ quality, daily, loading }) {
  const asked = quality.questions > 0
  const latency = quality.latency
  const ago = day => (day.ago === 0 ? 'Last 24 hours' : `${day.ago} ${day.ago === 1 ? 'day' : 'days'} ago`)
  const dailyRows = (daily || []).map(day => ({ id: `d${day.ago}`, measure: `Questions, ${ago(day).toLowerCase()}`, value: `${formatNumber(day.questions)} (${formatNumber(day.answered)} answered, ${formatNumber(day.declined)} declined, ${formatNumber(day.abstained)} abstained)` }))
  const rows = asked ? [
    { id: 'answered', measure: 'Answered', value: formatNumber(quality.answered) },
    { id: 'declined', measure: 'Declined', value: formatNumber(quality.declined) },
    { id: 'abstained', measure: 'Abstained', value: formatNumber(quality.abstained) },
    ...(latency ? [{ id: 'median', measure: 'Median time to answer', value: formatDuration(latency.p50) }, { id: 'p90', measure: 'Nine in ten within', value: formatDuration(latency.p90) }, { id: 'max', measure: 'Slowest', value: formatDuration(latency.max) }] : []),
    ...dailyRows,
  ] : []
  return (
    <Panel id="dash-answers" title="Answers" sub="Questions · last 7 days">
      {loading && !quality.reported ? <PanelSkeleton /> : asked ? (
        <>
          <div className="nx-answers__top">
            <span className="nx-answers__figure">{formatRate(quality.rate)}</span>
            <span className="nx-answers__caption">answered of {plural(quality.questions, 'question')}</span>
          </div>
          <SegBar height={10} label="Questions answered, declined and abstained" parts={[
            { id: 'answered', value: quality.answered, colour: 'var(--nx-accent)', title: `Answered: ${formatNumber(quality.answered)}` },
            { id: 'declined', value: quality.declined, colour: 'var(--nx-border-strong)', title: `Declined: ${formatNumber(quality.declined)}` },
            { id: 'abstained', value: quality.abstained, colour: 'var(--nx-border)', title: `Abstained: ${formatNumber(quality.abstained)}` },
          ]} />
          <ul className="nx-keys">
            <li><i className="nx-keys__key" style={{ background: 'var(--nx-accent)' }} /><span className="nx-keys__name">Answered</span><span className="nx-keys__value nx-tabular">{formatNumber(quality.answered)}</span></li>
            <li><i className="nx-keys__key" style={{ background: 'var(--nx-border-strong)' }} /><span className="nx-keys__name">Declined</span><span className="nx-keys__value nx-tabular">{formatNumber(quality.declined)}</span></li>
            <li><i className="nx-keys__key" style={{ background: 'var(--nx-border)' }} /><span className="nx-keys__name">Abstained</span><span className="nx-keys__value nx-tabular">{formatNumber(quality.abstained)}</span></li>
          </ul>
          {daily && daily.some(day => day.questions) ? (
            <div className="nx-group">
              <p className="nx-overline">Questions per day</p>
              <div dir="ltr">
                <ColumnChart label={`Questions in each of the last seven 24-hour periods, oldest first: ${daily.map(day => formatNumber(day.questions)).join(', ')}`}
                  columns={daily.map(day => ({ key: day.ago, title: `${ago(day)}: ${formatNumber(day.answered)} answered, ${formatNumber(day.declined)} declined, ${formatNumber(day.abstained)} abstained`, parts: [{ id: 'answered', value: day.answered, colour: 'var(--nx-accent)' }, { id: 'declined', value: day.declined, colour: 'var(--nx-border-strong)' }, { id: 'abstained', value: day.abstained, colour: 'var(--nx-border)' }] }))} />
                <p className="nx-cols__axis" aria-hidden="true"><span>6 days ago</span><span>Last 24 hours</span></p>
              </div>
            </div>
          ) : null}
          {latency ? (
            <div className="nx-ladder">
              <p className="nx-overline">Time to answer · <CaseLabel caseId={latency.caseId} /></p>
              <SegBar height={6} label={`Median ${formatDuration(latency.p50)}, nine in ten within ${formatDuration(latency.p90)}, slowest ${formatDuration(latency.max)}`} parts={[
                { id: 'p50', value: latency.p50, colour: 'var(--nx-accent)', title: `Half within ${formatDuration(latency.p50)}` },
                { id: 'p90', value: Math.max(0, (latency.p90 ?? latency.p50) - latency.p50), colour: 'color-mix(in oklab, var(--nx-accent) 55%, var(--nx-surface))', title: `Nine in ten within ${formatDuration(latency.p90)}` },
                { id: 'max', value: Math.max(0, (latency.max ?? latency.p90 ?? latency.p50) - (latency.p90 ?? latency.p50)), colour: 'color-mix(in oklab, var(--nx-accent) 25%, var(--nx-surface))', title: `Slowest ${formatDuration(latency.max)}` },
              ]} />
              <p className="nx-ladder__marks nx-tabular"><span>median {formatDuration(latency.p50)}</span>{latency.p90 != null ? <span>90% within {formatDuration(latency.p90)}</span> : null}{latency.max != null ? <span>slowest {formatDuration(latency.max)}</span> : null}</p>
            </div>
          ) : null}
          {quality.reasons[0] ? <p className="nx-answers__reason" title="The most common reason a question was declined"><CircleAlert aria-hidden="true" /><span>Most declines: {quality.reasons[0].text} ({formatNumber(quality.reasons[0].count)})</span></p> : null}
          <TableTwin label="Answers in the last 7 days" exportName="nexusai-answers-7-days.csv" summary={`${plural(quality.questions, 'question')} in the last 7 days: ${formatNumber(quality.answered)} answered, ${formatNumber(quality.declined)} declined, ${formatNumber(quality.abstained)} abstained.`}
            columns={[{ key: 'measure', label: 'Measure' }, { key: 'value', label: 'Value' }]} rows={rows} />
        </>
      ) : <Calm icon={MessageSquareText}>No questions in the last 7 days</Calm>}
    </Panel>
  )
}

// ---------- data quality: what was set aside, and how the times are known ----------
// Rows set aside is a dumbbell (exactly two values for each case: the rejected share, and the rejected plus exact-repeat share) on one axis that starts at zero. The basis of the times is one stacked bar.
export function DataQualityPanel({ aside, basis, loading, basisLoading }) {
  const nameOf = useCaseNameOf()
  const shown = aside.items.filter(item => item.share > 0).slice(0, 6)
  const axisMax = niceCeiling(Math.max(0.1, ...shown.map(item => item.share * 100)))
  return (
    <Panel id="dash-quality" title="Data quality" sub={aside.read ? `${formatRate(aside.share)} of ${formatNumber(aside.read)} rows set aside` : null}>
      {loading && !aside.read ? <PanelSkeleton /> : (
        <>
          <div className="nx-group">
            <p className="nx-overline">Rows set aside, share of the rows read</p>
            {shown.length ? (
              <>
                <ul className="nx-aside">
                  {shown.map(item => (
                    <li key={item.caseId} className="nx-aside__row" title={`${formatNumber(item.rejected)} rejected, ${formatNumber(item.duplicates)} exact repeats, of ${formatNumber(item.read)} rows read`}>
                      <Link className="nx-aside__name" to={`/cases/${encodeURIComponent(item.caseId)}/overview`}><CaseLabel caseId={item.caseId} /></Link>
                      <Dumbbell low={(item.rejected / item.read) * 100} high={item.share * 100} axisMax={axisMax} label={`${formatRate(item.rejected / item.read)} rejected, ${formatRate(item.share)} rejected or exact repeats`} />
                      <span className="nx-aside__value nx-tabular">{formatRate(item.share)}</span>
                    </li>
                  ))}
                </ul>
                <p className="nx-axis nx-tabular" aria-hidden="true"><i /><span className="nx-axis__scale"><span>0%</span><span>{formatRate(axisMax / 200)}</span><span>{formatRate(axisMax / 100)}</span></span><i /></p>
                <p className="nx-keyline"><span><i className="nx-keys__key nx-keys__key--dot" style={{ background: 'var(--nx-accent)' }} />Rejected</span><span><i className="nx-keys__key nx-keys__key--ring" />Rejected and exact repeats</span></p>
              </>
            ) : aside.read ? <Calm icon={CircleCheckBig} tone="ok"><strong>Every row was kept</strong></Calm> : <Calm icon={CircleAlert}>No rows read yet</Calm>}
          </div>
          <div className="nx-group">
            <p className="nx-overline">How the times of the records are known</p>
            {basisLoading && !basis.total ? <PanelSkeleton rows={2} /> : basis.total ? (
              <>
                <SegBar height={10} label="Rows by the basis of their time" parts={basis.parts.map((part, index) => ({ id: part.id, value: part.rows, colour: `var(--nx-data-${index + 1})`, title: `${part.label}: ${formatNumber(part.rows)} rows` }))} />
                <ul className="nx-keys">
                  {basis.parts.map((part, index) => (
                    <li key={part.id}><i className="nx-keys__key" style={{ background: `var(--nx-data-${index + 1})` }} /><span className="nx-keys__name">{part.label}</span><span className="nx-keys__value nx-tabular">{formatNumber(part.rows)}</span><span className="nx-keys__share nx-tabular">{formatRate(part.rows / basis.total)}</span></li>
                  ))}
                </ul>
              </>
            ) : <p className="nx-muted">No typed records yet</p>}
          </div>
          <TableTwin label="Data quality for each case" exportName="nexusai-data-quality.csv" summary={`${formatRate(aside.share)} of ${formatNumber(aside.read)} rows read were set aside; the times of ${formatNumber(basis.total)} typed rows are known as: ${basis.parts.map(part => `${formatNumber(part.rows)} ${part.label.toLowerCase()}`).join(', ') || 'not yet read'}.`}
            columns={[{ key: 'case', label: 'Case' }, { key: 'read', label: 'Rows read' }, { key: 'repeats', label: 'Exact repeats' }, { key: 'rejected', label: 'Rejected' }, { key: 'share', label: 'Share set aside' }]}
            rows={aside.items.map(item => ({ id: item.caseId, case: nameOf(item.caseId), read: item.read, repeats: item.duplicates, rejected: item.rejected, share: formatRate(item.share) }))} />
        </>
      )}
    </Panel>
  )
}

// ---------- the pipeline: how long files take from upload to ready, and where the time goes ----------
// Waiting (the file sat in the queue) and processing (the file was being read) are told apart: the first is usually all of it. Every strip shares one axis; a hairline marks the median of all files.
export function TimeToReadyPanel({ data, loading }) {
  const nameOf = useCaseNameOf()
  const cases = data.cases.slice().sort((left, right) => right.median - left.median).slice(0, 6)
  const axisMax = niceCeiling(Math.max(1, ...cases.map(item => item.max)))
  return (
    <Panel id="dash-ready" title="Time to ready" sub={data.count ? `Median ${formatDuration(data.median)} from upload` : null}>
      {loading && !data.count ? <PanelSkeleton /> : cases.length ? (
        <>
          <p className="nx-split"><span><b className="nx-tabular">{formatDuration(data.waitMedian)}</b> median waiting in the queue</span><span><b className="nx-tabular">{formatDuration(data.processMedian)}</b> median being read</span></p>
          <ul className="nx-aside">
            {cases.map(item => (
              <li key={item.caseId} className="nx-aside__row" title={`${plural(item.count, 'file')}: fastest ${formatDuration(item.min)}, slowest ${formatDuration(item.max)}`}>
                <Link className="nx-aside__name" to={`/cases/${encodeURIComponent(item.caseId)}/evidence`}><CaseLabel caseId={item.caseId} /></Link>
                <RangeStrip stats={item} axisMax={axisMax} reference={data.median} label={`Median ${formatDuration(item.median)}, from ${formatDuration(item.min)} to ${formatDuration(item.max)}, over ${plural(item.count, 'file')}`} />
                <span className="nx-aside__value nx-tabular">{formatDuration(item.median)}</span>
              </li>
            ))}
          </ul>
          <p className="nx-axis nx-tabular" aria-hidden="true"><i /><span className="nx-axis__scale"><span>0</span><span>{formatDuration(axisMax / 2)}</span><span>{formatDuration(axisMax)}</span></span><i /></p>
          <p className="nx-keyline"><span><i className="nx-keys__key nx-keys__key--dot" style={{ background: 'var(--nx-accent)' }} />Median</span><span><i className="nx-keys__key nx-keys__key--bar" />Middle half of the files</span><span><i className="nx-keys__key nx-keys__key--line" />Fastest to slowest</span><span><i className="nx-keys__key nx-keys__key--tick" />Median of all files</span></p>
          <TableTwin label="Time from upload to ready for each case" exportName="nexusai-time-to-ready.csv" summary={`Median ${formatDuration(data.median)} from upload to ready over ${plural(data.count, 'file')}: ${formatDuration(data.waitMedian)} waiting in the queue and ${formatDuration(data.processMedian)} being read; the slowest took ${formatDuration(data.slowest)}.`}
            columns={[{ key: 'case', label: 'Case' }, { key: 'files', label: 'Files' }, { key: 'fastest', label: 'Fastest' }, { key: 'median', label: 'Median' }, { key: 'slowest', label: 'Slowest' }]}
            rows={data.cases.map(item => ({ id: item.caseId, case: nameOf(item.caseId), files: item.count, fastest: formatDuration(item.min), median: formatDuration(item.median), slowest: formatDuration(item.max) }))} />
        </>
      ) : <Calm icon={CircleAlert}>No finished files yet</Calm>}
    </Panel>
  )
}
