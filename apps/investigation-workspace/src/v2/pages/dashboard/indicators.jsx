import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { ArrowUpRight, BellRing, CircleCheck, FileCheck, FolderOpen, MessageSquareText, Rows3, Timer } from 'lucide-react'
import { formatNumber } from '../../../lib/format.js'
import { formatRate } from '../../../lib/dashboardCharts.js'
import { useCaseNameOf } from '../../shell/CaseLabel.jsx'
import { CountUp } from '../../ui/CountUp.jsx'
import { ColumnChart, DotRow, Histogram, SegBar } from '../../ui/micro.jsx'
import { caseStateMix, durationHistogram, formatDuration, timeToReady } from './model.js'

// The key indicators: six tiles, each a link to the list behind its figure, each with the one picture that fits its question, and all six pictures made in one visual language so they read as a
// family: one height on one baseline, soft rounded marks with a 2px gap, grey for context, the accent only for the part that matters (the latest day, the largest case, the median), and the state
// colours (green, amber, red) only where a mark IS a state. Which picture answers which question: a mark for each case (a whole made of entities), a meter (a share of files), columns by case (where the
// volume is), stacked columns by day (a rhythm and its split), a bar split by cause (what waits), a histogram with its median (how long). Every figure is a count a service reported, a share of two
// counts, or a median of dated jobs. No target is defined for any of them, so none is drawn against a target, and no arrow or colour says whether "up" is good. A line under the picture says what it
// is; pointing at one mark makes that line say what the mark is (nothing floats over the data). The figure counts up once and the marks grow once, neither at all when less motion is asked for.
const TONE = { danger: 'var(--nx-danger)', warn: 'var(--nx-warn)', accent: 'var(--nx-accent)' }
const SHORT = { failed: 'failed', dead: 'gave up', missing: 'copy missing', stalled: 'stalled', choice: 'to choose' }
const plural = (count, one, many = `${one}s`) => `${formatNumber(count)} ${count === 1 ? one : many}`
const CASE_COLOUR = { ready: 'var(--nx-ok)', processing: 'var(--nx-warn)', review: 'var(--nx-danger)', other: 'var(--nx-border-strong)' }
const STATE_WORD = { ready: 'Ready', processing: 'Still processing', review: 'Needs review', other: 'Not reported yet' }

function Tile({ icon: Icon, label, to, alert, caption, figure, suffix, wait, children }) {
  const [probe, setProbe] = useState('')
  const point = event => setProbe(event.target.closest?.('[data-probe]')?.getAttribute('data-probe') || '')
  const shared = { className: `nx-tile${alert ? ' nx-tile--alert' : ''}`, onMouseOver: point, onMouseLeave: () => setProbe(''), 'aria-busy': wait || undefined }
  const body = (
    <>
      <span className="nx-tile__head">
        <span className="nx-tile__chip" aria-hidden="true"><Icon /></span>
        <span className="nx-tile__label">{label}</span>
        <ArrowUpRight className="nx-tile__go" aria-hidden="true" />
      </span>
      <span className="nx-tile__figure">{wait ? <i className="nx-skeleton nx-tile__sk-figure" aria-hidden="true" /> : <>{figure}{suffix ? <span className="nx-tile__suffix">{suffix}</span> : null}</>}</span>
      <span className="nx-tile__chart">{wait ? <i className="nx-skeleton nx-tile__sk-chart" aria-hidden="true" /> : children}</span>
      <span className={`nx-tile__caption${probe ? ' is-probe' : ''}`}>{wait ? ' ' : (probe || caption || ' ')}</span>
    </>
  )
  return to.startsWith('#') ? <a {...shared} href={to}>{body}</a> : <Link {...shared} to={to}>{body}</Link>
}

// The two ends of a time axis, in words: the picture has no scale of its own, so it says where it starts and ends.
function Ends({ from, to }) { return <span className="nx-tile__ends" aria-hidden="true"><span>{from}</span><span>{to}</span></span> }

export function Indicators({ kpis, rows, quality, daily, needs, loading }) {
  const nameOf = useCaseNameOf()
  const wait = loading && !kpis.reported
  const ready = useMemo(() => timeToReady(rows), [rows])
  const histogram = useMemo(() => durationHistogram(rows), [rows])
  const marks = useMemo(() => caseStateMix(rows), [rows])
  const inState = state => marks.filter(mark => mark.state === state).length
  const caseDots = marks.map(mark => ({ id: mark.caseId, colour: CASE_COLOUR[mark.state], title: `${nameOf(mark.caseId)} · ${STATE_WORD[mark.state]}` }))
  // what needs a look comes first, and two parts are enough: the colour of every bar carries the rest
  const caseCaption = [inState('review') ? `${inState('review')} to review` : '', inState('processing') ? `${inState('processing')} busy` : '', inState('ready') ? `${inState('ready')} ready` : '', inState('other') ? `${inState('other')} unread` : ''].filter(Boolean).slice(0, 2).join(' · ')
  const fileCaption = kpis.failed || kpis.processing ? [kpis.processing ? `${formatNumber(kpis.processing)} processing` : '', kpis.failed ? `${formatNumber(kpis.failed)} failed` : ''].filter(Boolean).join(' · ') : (kpis.sources ? 'All files read' : '')
  const total = kpis.acceptedRows + kpis.duplicateRows + kpis.rejectedRows
  const rate = total ? kpis.acceptedRows / total : null
  const byCase = rows.filter(row => row.summary && row.summary.acceptedRows > 0).sort((left, right) => right.summary.acceptedRows - left.summary.acceptedRows).slice(0, 8)
  const asked = quality.questions > 0
  const attention = needs.groups.filter(group => group.attention)
  const causeParts = attention.map(group => ({ id: group.id, value: group.count, colour: TONE[group.tone] || TONE.accent, title: `${group.label} · ${formatNumber(group.count)}` }))
  const needsCaption = attention.slice(0, 2).map(group => `${formatNumber(group.count)} ${SHORT[group.id] || group.label.toLowerCase()}`).join(' · ')
  const first = needs.groups.find(group => group.attention)?.items[0]
  const day = value => (value.ago === 0 ? 'Last 24 hours' : `${value.ago} ${value.ago === 1 ? 'day' : 'days'} ago`)
  const dayColumns = daily && daily.length === 7 ? daily.map(value => ({ key: value.ago, title: `${day(value)} · ${formatNumber(value.answered)} answered, ${formatNumber(value.declined)} declined, ${formatNumber(value.abstained)} abstained`, parts: [{ id: 'answered', value: value.answered, colour: 'var(--nx-accent)' }, { id: 'declined', value: value.declined, colour: 'var(--nx-border-strong)' }, { id: 'abstained', value: value.abstained, colour: 'var(--nx-border)' }] })) : null
  const timeBins = histogram.count ? histogram.bins.map(bin => ({ ...bin, title: `${bin.label} · ${plural(bin.count, 'file')}` })) : []
  return (
    <section className="nx-kpis" aria-label="Figures">
      <div className="nx-tiles">
        <Tile icon={FolderOpen} label="Cases" to="/cases" wait={wait} caption={caseCaption} figure={<CountUp value={kpis.cases} format={formatNumber} />}>
          <DotRow probe size="pill" max={24} dots={caseDots} label={`Cases by state, one bar each: ${inState('ready')} ready, ${inState('processing')} still processing, ${inState('review')} need review`} />
        </Tile>
        <Tile icon={FileCheck} label="Files ready" to="/cases" wait={wait} caption={fileCaption} figure={<CountUp value={kpis.ready} format={formatNumber} />} suffix={`/ ${formatNumber(kpis.sources)}`}>
          <SegBar probe height={14} label={`${formatNumber(kpis.ready)} of ${formatNumber(kpis.sources)} files are ready`} parts={[{ id: 'ready', value: kpis.ready, colour: 'var(--nx-ok)', title: `Ready · ${formatNumber(kpis.ready)}` }, { id: 'busy', value: kpis.processing, colour: 'var(--nx-warn)', title: `Processing · ${formatNumber(kpis.processing)}` }, { id: 'failed', value: kpis.failed, colour: 'var(--nx-danger)', title: `Failed · ${formatNumber(kpis.failed)}` }, { id: 'rest', value: Math.max(0, kpis.sources - kpis.ready - kpis.processing - kpis.failed), colour: 'var(--nx-border-strong)', title: 'Other' }]} />
        </Tile>
        <Tile icon={Rows3} label="Records" to="/cases" wait={wait} caption={rate === null ? '' : `${formatRate(rate)} accepted`} figure={<CountUp value={kpis.acceptedRows} format={formatNumber} />}>
          {byCase.length > 1 ? <ColumnChart probe height={36} label="Accepted rows in each case, the largest first" columns={byCase.map((row, index) => ({ key: row.caseId, title: `${nameOf(row.caseId)} · ${formatNumber(row.summary.acceptedRows)} records`, parts: [{ id: 'records', value: row.summary.acceptedRows, colour: index === 0 ? 'var(--nx-accent)' : 'var(--nx-border-strong)' }] }))} /> : null}
        </Tile>
        <Tile icon={MessageSquareText} label="Answered" to="/investigate" wait={wait && !quality.reported} caption={asked ? `${plural(quality.questions, 'question')} · 7 days` : (quality.reported ? 'No questions in 7 days' : '')} figure={asked ? <CountUp value={Math.round(quality.rate * 1000)} format={value => formatRate(value / 1000)} /> : '—'}>
          {asked && dayColumns ? <><ColumnChart probe height={30} label={`Questions in each of the last seven 24-hour periods, oldest first: ${daily.map(value => formatNumber(value.questions)).join(', ')}`} columns={dayColumns} /><Ends from="7 days ago" to="now" /></>
            : asked ? <SegBar probe height={14} label="Questions answered, declined and abstained" parts={[{ id: 'answered', value: quality.answered, colour: 'var(--nx-accent)', title: `Answered · ${formatNumber(quality.answered)}` }, { id: 'declined', value: quality.declined, colour: 'var(--nx-border-strong)', title: `Declined · ${formatNumber(quality.declined)}` }, { id: 'abstained', value: quality.abstained, colour: 'var(--nx-border)', title: `Abstained · ${formatNumber(quality.abstained)}` }]} />
              : (quality.reported ? <ColumnChart height={30} label="No questions in the last seven days" columns={Array.from({ length: 7 }, (_, index) => ({ key: index, title: '', parts: [] }))} /> : null)}
        </Tile>
        <Tile icon={BellRing} label="Needs you" to={first?.to || '/activity'} alert={needs.attention > 0} wait={wait} caption={needs.attention ? needsCaption : 'Nothing waiting'} figure={<CountUp value={needs.attention} format={formatNumber} />}>
          {causeParts.length ? <SegBar probe height={14} label={`${plural(needs.attention, 'thing')} waiting for you: ${attention.map(group => `${group.count} ${group.label.toLowerCase()}`).join(', ')}`} parts={causeParts} /> : <span className="nx-tile__clear"><CircleCheck aria-hidden="true" />All clear</span>}
        </Tile>
        <Tile icon={Timer} label="Time to ready" to="/cases" wait={wait} caption={histogram.count ? `wait ${formatDuration(ready.waitMedian)} · read ${formatDuration(ready.processMedian)}` : 'No finished files yet'} figure={histogram.count ? <CountUp value={Math.round(histogram.median)} format={formatDuration} /> : '—'} suffix={histogram.count ? 'median' : ''}>
          {histogram.count ? <><Histogram probe height={30} label={`${plural(histogram.count, 'file')} by time from upload to ready; the median is ${formatDuration(histogram.median)}`} bins={timeBins} /><Ends from={histogram.bins[0].label} to={histogram.bins[histogram.bins.length - 1].label} /></> : null}
        </Tile>
      </div>
    </section>
  )
}
