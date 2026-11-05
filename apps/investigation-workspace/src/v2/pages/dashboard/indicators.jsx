import { useMemo } from 'react'
import { Link } from 'react-router-dom'
import { formatNumber } from '../../../lib/format.js'
import { formatRate } from '../../../lib/dashboardCharts.js'
import { useCaseNameOf } from '../../shell/CaseLabel.jsx'
import { CountUp } from '../../ui/CountUp.jsx'
import { ColumnChart, DotRow, Histogram, SegBar } from '../../ui/micro.jsx'
import { durationHistogram, formatDuration, timeToReady } from './model.js'

// The key indicators: six figures, each with the one small picture that explains it, and no two of the same form (a state bar, a meter, bars by case, columns by day, a dot for each thing that
// waits, a histogram). Every figure is a count a service reported, a share of two counts, or a median of dated jobs; the whole tile opens the list behind it. No target is defined for any of them,
// so none is drawn against a target (a bullet bar needs one), and no arrow or colour says whether "up" is good. Motion: the figure counts up once and the bars grow once, both under half a second
// and neither at all when the analyst asked for less motion.
const TONE = { danger: 'var(--nx-danger)', warn: 'var(--nx-warn)', accent: 'var(--nx-accent)' }
const SHORT = { failed: 'failed', dead: 'gave up', missing: 'copy missing', stalled: 'stalled', choice: 'to choose' }
const plural = (count, one, many = `${one}s`) => `${formatNumber(count)} ${count === 1 ? one : many}`

function Tile({ label, to, alert, caption, children, figure }) {
  const body = (
    <>
      <span className="nx-tile__label">{label}</span>
      <span className="nx-tile__figure">{figure}</span>
      <span className="nx-tile__visual">{children}</span>
      <span className="nx-tile__caption">{caption || ' '}</span>
    </>
  )
  const className = `nx-tile${alert ? ' nx-tile--alert' : ''}`
  return to.startsWith('#') ? <a className={className} href={to}>{body}</a> : <Link className={className} to={to}>{body}</Link>
}

export function Indicators({ kpis, rows, states, quality, daily, needs, loading }) {
  const nameOf = useCaseNameOf()
  const wait = loading && !kpis.reported
  const ready = useMemo(() => timeToReady(rows), [rows])
  const histogram = useMemo(() => durationHistogram(rows), [rows])
  const mix = [
    { id: 'ok', value: rows.filter(row => row.status === 'complete').length, colour: 'var(--nx-ok)', title: 'Ready' },
    { id: 'warn', value: rows.filter(row => row.status === 'processing').length, colour: 'var(--nx-warn)', title: 'Processing' },
    { id: 'bad', value: rows.filter(row => row.status === 'attention').length, colour: 'var(--nx-danger)', title: 'Needs review' },
    { id: 'other', value: rows.filter(row => !['complete', 'processing', 'attention'].includes(row.status)).length, colour: 'var(--nx-border-strong)', title: 'Not reported yet' },
  ].map(part => ({ ...part, title: `${part.title}: ${part.value}` }))
  const caseCaption = [states?.open ? `${formatNumber(states.open)} open` : '', states?.on_hold ? `${formatNumber(states.on_hold)} on hold` : '', states?.closed ? `${formatNumber(states.closed)} closed` : ''].filter(Boolean).join(' · ')
  const fileCaption = kpis.failed || kpis.processing ? [kpis.processing ? `${formatNumber(kpis.processing)} processing` : '', kpis.failed ? `${formatNumber(kpis.failed)} failed` : ''].filter(Boolean).join(' · ') : (kpis.sources ? 'All files read' : '')
  const total = kpis.acceptedRows + kpis.duplicateRows + kpis.rejectedRows
  const rate = total ? kpis.acceptedRows / total : null
  const byCase = rows.filter(row => row.summary && row.summary.acceptedRows > 0).sort((left, right) => right.summary.acceptedRows - left.summary.acceptedRows).slice(0, 8)
  const asked = quality.questions > 0
  const attention = needs.groups.filter(group => group.attention)
  const dots = attention.flatMap(group => Array.from({ length: group.count }, (_, index) => ({ id: `${group.id}-${index}`, colour: TONE[group.tone] || TONE.accent, title: group.label })))
  const needsCaption = attention.slice(0, 2).map(group => `${formatNumber(group.count)} ${SHORT[group.id] || group.label.toLowerCase()}`).join(' · ')
  const first = needs.groups.find(group => group.attention)?.items[0]
  const day = value => (value.ago === 0 ? 'last 24 hours' : `${value.ago} ${value.ago === 1 ? 'day' : 'days'} ago`)
  return (
    <section className="nx-kpis" aria-label="Figures">
      <div className="nx-tiles">
        <Tile label="Cases" to="/cases" caption={caseCaption} figure={wait ? '—' : <CountUp value={kpis.cases} format={formatNumber} />}>
          <SegBar parts={mix} label="Cases by state" />
        </Tile>
        <Tile label="Files ready" to="/cases" caption={wait ? '' : fileCaption} figure={wait ? '—' : <><CountUp value={kpis.ready} format={formatNumber} /><span className="nx-tile__of"> / {formatNumber(kpis.sources)}</span></>}>
          <SegBar label={`${formatNumber(kpis.ready)} of ${formatNumber(kpis.sources)} files are ready`} parts={[{ id: 'ready', value: kpis.ready, colour: 'var(--nx-ok)', title: `Ready: ${formatNumber(kpis.ready)}` }, { id: 'busy', value: kpis.processing, colour: 'var(--nx-warn)', title: `Processing: ${formatNumber(kpis.processing)}` }, { id: 'failed', value: kpis.failed, colour: 'var(--nx-danger)', title: `Failed: ${formatNumber(kpis.failed)}` }, { id: 'rest', value: Math.max(0, kpis.sources - kpis.ready - kpis.processing - kpis.failed), colour: 'var(--nx-border-strong)', title: 'Other' }]} />
        </Tile>
        <Tile label="Records" to="/cases" caption={rate === null ? '' : `${formatRate(rate)} of rows accepted`} figure={wait ? '—' : <CountUp value={kpis.acceptedRows} format={formatNumber} />}>
          {byCase.length > 1 ? <ColumnChart height={36} label="Accepted rows in each case, the largest first" columns={byCase.map(row => ({ key: row.caseId, title: `${nameOf(row.caseId)}: ${formatNumber(row.summary.acceptedRows)} records`, parts: [{ id: 'records', value: row.summary.acceptedRows, colour: 'var(--nx-accent)' }] }))} /> : null}
        </Tile>
        <Tile label="Answered · 7 days" to="/investigate" caption={asked ? plural(quality.questions, 'question') : (quality.reported ? 'No questions this week' : '')} figure={asked ? <CountUp value={Math.round(quality.rate * 1000)} format={value => formatRate(value / 1000)} /> : '—'}>
          {asked ? (
            daily && daily.length === 7
              ? <ColumnChart height={36} label={`Questions in each of the last seven 24-hour periods, oldest first: ${daily.map(value => formatNumber(value.questions)).join(', ')}`} columns={daily.map(value => ({ key: value.ago, title: `${day(value)}: ${formatNumber(value.answered)} answered, ${formatNumber(value.declined)} declined, ${formatNumber(value.abstained)} abstained`, parts: [{ id: 'answered', value: value.answered, colour: 'var(--nx-accent)' }, { id: 'declined', value: value.declined, colour: 'var(--nx-border-strong)' }, { id: 'abstained', value: value.abstained, colour: 'var(--nx-border)' }] }))} />
              : <SegBar label="Questions answered, declined and abstained" parts={[{ id: 'answered', value: quality.answered, colour: 'var(--nx-accent)', title: `Answered: ${formatNumber(quality.answered)}` }, { id: 'declined', value: quality.declined, colour: 'var(--nx-border-strong)', title: `Declined: ${formatNumber(quality.declined)}` }, { id: 'abstained', value: quality.abstained, colour: 'var(--nx-border)', title: `Abstained: ${formatNumber(quality.abstained)}` }]} />
          ) : null}
        </Tile>
        <Tile label="Needs you" to={first?.to || '/activity'} alert={needs.attention > 0} caption={needs.attention ? needsCaption : (wait ? '' : 'Nothing waiting')} figure={wait ? '—' : <CountUp value={needs.attention} format={formatNumber} />}>
          {dots.length ? <DotRow dots={dots} label={`${plural(needs.attention, 'thing')} waiting for you: ${attention.map(group => `${group.count} ${group.label.toLowerCase()}`).join(', ')}`} /> : null}
        </Tile>
        <Tile label="Time to ready" to="/cases" caption={histogram.count ? `Wait ${formatDuration(ready.waitMedian)} · read ${formatDuration(ready.processMedian)}` : (wait ? '' : 'No finished files yet')} figure={histogram.count ? <CountUp value={Math.round(histogram.median)} format={formatDuration} /> : '—'}>
          {histogram.count ? <Histogram label={`${plural(histogram.count, 'file')} by time from upload to ready; the median is ${formatDuration(histogram.median)}`} bins={histogram.bins.map(bin => ({ ...bin, title: `${bin.label}: ${plural(bin.count, 'file')}` }))} /> : null}
        </Tile>
      </div>
    </section>
  )
}
