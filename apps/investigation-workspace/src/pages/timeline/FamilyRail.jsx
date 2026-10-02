import { Pin, X } from 'lucide-react'
import { familyColour } from '../../lib/caseActivity.js'
import { formatNumber } from '../../lib/format.js'
import { curatedFamilyLabel } from '../../lib/semanticCatalog.js'
import { weekdayRhythm } from '../../lib/timelineModel.js'

const dot = colour => ({ '--fam': colour })

// The record families as one row of filters, each with its share and trend. Choosing families narrows every number and
// picture on the page; with none chosen, all are shown.
export function FamilyStrip({ all, order, palette, chosen, onToggle, onClear, totals }) {
  return (
    <section className="tw-fambar" aria-labelledby="tw-fam">
      <h2 id="tw-fam">Record families</h2>
      <ul className="tw-fams">
        {all.map(family => {
          const on = chosen.includes(family.id)
          const share = totals ? Math.floor((family.total / totals) * 100) : 0
          return (
            <li key={family.id} style={dot(familyColour(family.id, order, palette))}>
              <button type="button" aria-pressed={on} data-active={on || !chosen.length ? 'true' : 'false'} onClick={() => onToggle(family.id)} title={`${share < 1 ? 'Under 1' : share}% of all events`}>
                <i aria-hidden="true" />
                <span className="tw-fams__name">{curatedFamilyLabel(family.id)}</span>
                <b>{formatNumber(family.total)}</b>
              </button>
            </li>
          )
        })}
      </ul>
      {chosen.length ? <button type="button" className="tw-fambar__clear" onClick={onClear}>Clear</button> : null}
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
