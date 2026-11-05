import { Link } from 'react-router-dom'
import { CircleAlert, CircleCheckBig, MessageSquareText } from 'lucide-react'
import { formatNumber } from '../../../lib/format.js'
import { formatRate } from '../../../lib/dashboardCharts.js'
import { CaseLabel, useCaseNameOf } from '../../shell/CaseLabel.jsx'
import { Calm, Panel, PanelSkeleton } from '../../ui/Panel.jsx'
import { BarStrip, RangeStrip, SegBar } from '../../ui/micro.jsx'
import { TableTwin } from '../../ui/TableTwin.jsx'
import { formatDuration, niceCeiling } from './model.js'

const MONTH = new Intl.DateTimeFormat('en-GB', { month: 'short', year: 'numeric', timeZone: 'UTC' })
const monthLabel = key => MONTH.format(new Date(`${key.slice(0, 7)}-01T00:00:00Z`))
const plural = (count, one, many = `${one}s`) => `${formatNumber(count)} ${count === 1 ? one : many}`
const colourOf = kind => (kind.slot ? `var(--nx-data-${kind.slot})` : 'var(--nx-data-muted)')

// ---------- the evidence timeline: when the events of each case happened, one row for each case ----------
// Each row is drawn on its own scale (the busiest month fills the height), so the shape of a case can be read next to the others; the count at the end is the exact total.
// A case the service could not read to its end says so, and a date in the future is named and left out. The table twin lists the same figures.
export function TimelinePanel({ timelines, loading, failures }) {
  const nameOf = useCaseNameOf()
  const twin = timelines.map(item => ({ id: item.caseId, case: nameOf(item.caseId), from: item.first ? monthLabel(item.first) : '', to: item.last ? monthLabel(item.last) : '', events: item.total, note: [item.partial ? 'first 100 days only' : '', item.futureTotal ? `${formatNumber(item.futureTotal)} events dated after today are not drawn` : ''].filter(Boolean).join('; ') }))
  return (
    <Panel id="dash-timeline" title="Evidence timeline" sub="Events per month · each row on its own scale" className="nx-panel--timeline">
      {loading && !timelines.length ? <PanelSkeleton rows={4} /> : timelines.length ? (
        <>
          <ol className="nx-tl">
            {timelines.map(item => (
              <li key={item.caseId} className="nx-tl__row">
                <span className="nx-tl__who">
                  <Link className="nx-tl__name" to={`/cases/${encodeURIComponent(item.caseId)}/timeline`}><CaseLabel caseId={item.caseId} /></Link>
                  {item.partial ? <span className="nx-tl__note" title="The service returns at most 100 days for a case; only those are drawn and counted.">first 100 days only</span> : null}
                </span>
                <BarStrip
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
          <TableTwin label="Events per case by month range" exportName="nexusai-evidence-timeline.csv" summary={`Events per month for ${plural(timelines.length, 'case')}; ${plural(timelines.filter(item => item.partial).length, 'case')} read only as far as the service returns.`}
            columns={[{ key: 'case', label: 'Case' }, { key: 'from', label: 'From' }, { key: 'to', label: 'To' }, { key: 'events', label: 'Events' }, { key: 'note', label: 'Note' }]} rows={twin} />
        </>
      ) : <Calm icon={CircleAlert}>{failures.length ? 'The timeline could not be read' : 'No dated events yet'}</Calm>}
    </Panel>
  )
}

// ---------- what the evidence holds ----------
export function EvidenceTypesPanel({ mix, loading }) {
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
          <TableTwin label="Files by type" exportName="nexusai-evidence-by-type.csv" summary={`${plural(mix.total, 'file')} in all: ${mix.kinds.map(kind => `${formatNumber(kind.files)} ${kind.label.toLowerCase()}`).join(', ')}.`}
            columns={[{ key: 'type', label: 'Type' }, { key: 'files', label: 'Files' }, { key: 'failed', label: 'Did not finish' }, { key: 'share', label: 'Share' }]}
            rows={mix.kinds.map(kind => ({ id: kind.id, type: kind.label, files: kind.files, failed: kind.failed, share: formatRate(kind.files / mix.total) }))} />
        </>
      ) : <Calm icon={CircleAlert}>No files read yet</Calm>}
    </Panel>
  )
}

export function RecordKindsPanel({ kinds, largest, loading }) {
  const max = Math.max(1, ...kinds.kinds.map(kind => kind.count))
  const biggest = Math.max(1, ...largest.map(item => item.records))
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
          {largest.length > 1 ? (
            <div className="nx-group">
              <p className="nx-overline">Where the records are</p>
              <ul className="nx-mix">
                {largest.map(item => (
                  <li key={item.caseId} className="nx-mix__row nx-mix__row--static">
                    <Link className="nx-mix__label nx-mix__link" to={`/cases/${encodeURIComponent(item.caseId)}/overview`}><CaseLabel caseId={item.caseId} /></Link>
                    <span className="nx-mix__bar" aria-hidden="true"><i style={{ inlineSize: `${Math.max(2, (item.records / biggest) * 100)}%` }} /></span>
                    <span className="nx-mix__value nx-tabular">{formatNumber(item.records)}</span>
                  </li>
                ))}
              </ul>
            </div>
          ) : null}
          <TableTwin label="Records of each kind" exportName="nexusai-records-by-kind.csv" summary={`${plural(kinds.total, 'record')} in all: ${kinds.kinds.map(kind => `${formatNumber(kind.count)} ${kind.label.toLowerCase()}`).join(', ')}.`}
            columns={[{ key: 'kind', label: 'Kind' }, { key: 'records', label: 'Records' }, { key: 'share', label: 'Share' }]}
            rows={kinds.kinds.map(kind => ({ id: kind.id, kind: kind.label, records: kind.count, share: formatRate(kind.count / kinds.total) }))} />
        </>
      ) : <Calm icon={CircleAlert}>No records typed yet</Calm>}
    </Panel>
  )
}

// ---------- answers: what the engine did with the questions of the last days ----------
export function AnswersPanel({ quality, loading }) {
  const asked = quality.questions > 0
  const latency = quality.latency
  const rows = asked ? [
    { id: 'answered', measure: 'Answered', value: formatNumber(quality.answered) },
    { id: 'declined', measure: 'Declined', value: formatNumber(quality.declined) },
    { id: 'abstained', measure: 'Abstained', value: formatNumber(quality.abstained) },
    ...(latency ? [{ id: 'median', measure: 'Median time to answer', value: formatDuration(latency.p50) }, { id: 'p90', measure: 'Nine in ten within', value: formatDuration(latency.p90) }, { id: 'max', measure: 'Slowest', value: formatDuration(latency.max) }] : []),
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

// ---------- the pipeline ----------
export function SetAsidePanel({ data, loading }) {
  const nameOf = useCaseNameOf()
  const shown = data.items.filter(item => item.share > 0).slice(0, 5)
  const max = Math.max(0.0001, ...shown.map(item => item.share))
  return (
    <Panel id="dash-setaside" title="Rows set aside" sub={data.read ? `${formatRate(data.share)} of ${formatNumber(data.read)} rows` : null}>
      {loading && !data.read ? <PanelSkeleton /> : shown.length ? (
        <>
          <ul className="nx-aside">
            {shown.map(item => (
              <li key={item.caseId} className="nx-aside__row" title={`${formatNumber(item.rejected)} rejected, ${formatNumber(item.duplicates)} exact repeats, of ${formatNumber(item.read)} rows read`}>
                <Link className="nx-aside__name" to={`/cases/${encodeURIComponent(item.caseId)}/overview`}><CaseLabel caseId={item.caseId} /></Link>
                <SegBar height={8} label={`${formatRate(item.share)} of the rows of this case were set aside`} parts={[{ id: 'rejected', value: item.rejected / item.read / max, colour: 'var(--nx-accent)' }, { id: 'repeats', value: item.duplicates / item.read / max, colour: 'var(--nx-border-strong)' }, { id: 'room', value: Math.max(0, 1 - item.share / max), colour: 'transparent' }]} />
                <span className="nx-aside__value nx-tabular">{formatRate(item.share)}</span>
              </li>
            ))}
          </ul>
          <p className="nx-keyline"><span><i className="nx-keys__key" style={{ background: 'var(--nx-accent)' }} />Rejected</span><span><i className="nx-keys__key" style={{ background: 'var(--nx-border-strong)' }} />Exact repeats</span></p>
          <TableTwin label="Rows set aside for each case" exportName="nexusai-rows-set-aside.csv" summary={`${formatRate(data.share)} of ${formatNumber(data.read)} rows read were set aside, the case with the largest share first.`}
            columns={[{ key: 'case', label: 'Case' }, { key: 'read', label: 'Rows read' }, { key: 'repeats', label: 'Exact repeats' }, { key: 'rejected', label: 'Rejected' }, { key: 'share', label: 'Share set aside' }]}
            rows={data.items.map(item => ({ id: item.caseId, case: nameOf(item.caseId), read: item.read, repeats: item.duplicates, rejected: item.rejected, share: formatRate(item.share) }))} />
        </>
      ) : data.read ? <Calm icon={CircleCheckBig} tone="ok"><strong>Every row was kept</strong></Calm> : <Calm icon={CircleAlert}>No rows read yet</Calm>}
    </Panel>
  )
}

export function TimeToReadyPanel({ data, loading }) {
  const nameOf = useCaseNameOf()
  const cases = data.cases.slice().sort((left, right) => right.median - left.median).slice(0, 6)
  const axisMax = niceCeiling(Math.max(1, ...cases.map(item => item.max)))
  return (
    <Panel id="dash-ready" title="Time to ready" sub={data.count ? `Median ${formatDuration(data.median)} from upload` : null}>
      {loading && !data.count ? <PanelSkeleton /> : cases.length ? (
        <>
          <ul className="nx-aside">
            {cases.map(item => (
              <li key={item.caseId} className="nx-aside__row" title={`${plural(item.count, 'file')}: fastest ${formatDuration(item.min)}, slowest ${formatDuration(item.max)}`}>
                <Link className="nx-aside__name" to={`/cases/${encodeURIComponent(item.caseId)}/evidence`}><CaseLabel caseId={item.caseId} /></Link>
                <RangeStrip stats={item} axisMax={axisMax} label={`Median ${formatDuration(item.median)}, from ${formatDuration(item.min)} to ${formatDuration(item.max)}, over ${plural(item.count, 'file')}`} />
                <span className="nx-aside__value nx-tabular">{formatDuration(item.median)}</span>
              </li>
            ))}
          </ul>
          <p className="nx-axis nx-tabular" aria-hidden="true"><i /><span className="nx-axis__scale"><span>0</span><span>{formatDuration(axisMax / 2)}</span><span>{formatDuration(axisMax)}</span></span><i /></p>
          <p className="nx-keyline"><span><i className="nx-keys__key nx-keys__key--dot" style={{ background: 'var(--nx-accent)' }} />Median</span><span><i className="nx-keys__key nx-keys__key--bar" />Middle half of the files</span><span><i className="nx-keys__key nx-keys__key--line" />Fastest to slowest</span></p>
          <TableTwin label="Time from upload to ready for each case" exportName="nexusai-time-to-ready.csv" summary={`Median ${formatDuration(data.median)} from upload to ready over ${plural(data.count, 'file')}; the slowest took ${formatDuration(data.slowest)}.`}
            columns={[{ key: 'case', label: 'Case' }, { key: 'files', label: 'Files' }, { key: 'fastest', label: 'Fastest' }, { key: 'median', label: 'Median' }, { key: 'slowest', label: 'Slowest' }]}
            rows={data.cases.map(item => ({ id: item.caseId, case: nameOf(item.caseId), files: item.count, fastest: formatDuration(item.min), median: formatDuration(item.median), slowest: formatDuration(item.max) }))} />
        </>
      ) : <Calm icon={CircleAlert}>No finished files yet</Calm>}
    </Panel>
  )
}
