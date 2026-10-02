import { Link } from 'react-router-dom'
import { MoonStar, Pin, PieChart, TrendingUp, X } from 'lucide-react'
import { familyColour } from '../../lib/caseActivity.js'
import { formatNumber } from '../../lib/format.js'
import { curatedFamilyLabel } from '../../lib/semanticCatalog.js'
import { weekdayRhythm } from '../../lib/timelineModel.js'

const dot = colour => ({ '--fam': colour })
const INSIGHT_ICON = { spike: TrendingUp, gap: MoonStar, mix: PieChart }

// A family's activity across the whole span as a tiny line, scaled to its own busiest day.
function Spark({ days, id }) {
  const values = days.map(day => day.byFamily[id] || 0)
  const peak = Math.max(1, ...values)
  const step = values.length > 1 ? 100 / (values.length - 1) : 0
  const points = values.map((value, index) => `${(index * step).toFixed(1)},${(18 - (value / peak) * 17).toFixed(1)}`).join(' ')
  return <svg className="tw-spark" viewBox="0 0 100 18" preserveAspectRatio="none" aria-hidden="true"><polyline points={points} fill="none" stroke="var(--fam)" strokeWidth="1.6" vectorEffect="non-scaling-stroke" /></svg>
}

// The record families as one row of filters, each with its share and trend. Choosing families narrows every number and
// picture on the page; with none chosen, all are shown.
export function FamilyStrip({ all, days, order, palette, chosen, onToggle, onClear, totals }) {
  return (
    <section className="tw-fambar" aria-labelledby="tw-fam">
      <header><h2 id="tw-fam">Record families</h2><span>{chosen.length ? `${chosen.length} chosen` : 'All shown. Choose one or more to narrow every number on this page.'}</span>{chosen.length ? <button type="button" onClick={onClear}>Clear</button> : null}</header>
      <ul className="tw-fams">
        {all.map(family => {
          const on = chosen.includes(family.id)
          const share = totals ? Math.floor((family.total / totals) * 100) : 0
          return (
            <li key={family.id} style={dot(familyColour(family.id, order, palette))}>
              <button type="button" aria-pressed={on} data-active={on || !chosen.length ? 'true' : 'false'} onClick={() => onToggle(family.id)}>
                <span className="tw-fams__top"><i aria-hidden="true" /><span className="tw-fams__name">{curatedFamilyLabel(family.id)}</span></span>
                <span className="tw-fams__num"><b>{formatNumber(family.total)}</b><small>{share < 1 ? '<1%' : `${share}%`}</small></span>
                <Spark days={days} id={family.id} />
              </button>
            </li>
          )
        })}
      </ul>
    </section>
  )
}

// What stands out, with each finding's reasoning, and the chronology questions the service suggests for this case.
export function InsightsCard({ insights, questions, caseId, canOpen, onSelect }) {
  return (
    <section className="tw-panelcard tw-insights" aria-labelledby="tw-insights">
      <header><h2 id="tw-insights">What stands out</h2><small>Arithmetic on the counts</small></header>
      {insights.length ? (
        <ul>
          {insights.map(item => {
            const Icon = INSIGHT_ICON[item.id]
            const body = <><span className="tw-insight__icon" aria-hidden="true"><Icon /></span><span><b>{item.title}</b><small>{item.detail}</small></span></>
            return <li key={item.id} className={`tw-insight tw-insight--${item.tone}`}>{item.date && canOpen(item.date) ? <button type="button" onClick={() => onSelect(item.date)} aria-label={`${item.title}. ${item.detail} Read this day.`}>{body}</button> : <div>{body}</div>}</li>
          })}
        </ul>
      ) : <p className="tw-rail__hint">Nothing unusual here: no day at least 3&times; the typical day, no gap of a week or more, and no family above half of the events.</p>}
      {questions.length ? (
        <div className="tw-ask">
          <h3>Ask about a sequence</h3>
          <ul>{questions.slice(0, 2).map(item => <li key={`${item.scope}-${item.question}`}><Link to={`/cases/${encodeURIComponent(caseId)}/investigate?${new URLSearchParams({ question: item.question, ...(item.scope !== 'all' ? { scope: item.scope } : {}) })}`}>{item.question}</Link><small>{item.family}</small></li>)}</ul>
        </div>
      ) : null}
    </section>
  )
}

// Events by day of the week across the days in view: the rhythm of the records.
export function RhythmPanel({ days }) {
  const week = weekdayRhythm({ days })
  const peak = Math.max(1, ...week.map(slot => slot.total))
  return (
    <section className="tw-panelcard" aria-labelledby="tw-rhythm">
      <header><h2 id="tw-rhythm">Weekly rhythm</h2><small>Events by weekday</small></header>
      <ol className="tw-rhythm" aria-label="Events by weekday">
        {week.map(slot => (
          <li key={slot.label} title={`${slot.label}: ${formatNumber(slot.total)} events on ${formatNumber(slot.days)} days`}>
            <span style={{ blockSize: `${Math.max(slot.total ? 6 : 2, (slot.total / peak) * 100)}%` }} />
            <small>{slot.label.slice(0, 1)}</small>
            <span className="visually-hidden">{slot.label}: {formatNumber(slot.total)} events, {slot.share}%</span>
          </li>
        ))}
      </ol>
    </section>
  )
}

// The days the analyst pinned, with their notes.
export function SavedPanel({ pins, onOpenPin, onRemovePin }) {
  return (
    <section className="tw-panelcard" aria-labelledby="tw-saved">
      <header><h2 id="tw-saved">Saved days</h2><small>This browser</small></header>
      {pins.length ? (
        <ul className="tw-pins">
          {pins.map(pin => (
            <li key={pin.date}>
              <button type="button" onClick={() => onOpenPin(pin.date)}><Pin aria-hidden="true" /><span><b>{pin.date}</b>{pin.note ? <small>{pin.note}</small> : null}</span></button>
              <button type="button" className="tw-pins__x" onClick={() => onRemovePin(pin.date)} aria-label={`Remove saved day ${pin.date}`}><X aria-hidden="true" /></button>
            </li>
          ))}
        </ul>
      ) : <p className="tw-rail__hint">Pin a day from the inspector to keep it here with a note.</p>}
    </section>
  )
}
