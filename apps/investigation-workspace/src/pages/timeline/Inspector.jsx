import { useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { ChevronLeft, ChevronRight, MessageSquareText, Pin, PinOff } from 'lucide-react'
import { describeBucket, familyColour } from '../../lib/caseActivity.js'
import { formatNumber } from '../../lib/format.js'
import { curatedFamilyLabel } from '../../lib/semanticCatalog.js'
import { againstTypical, dayBreakdown, dayParts } from '../../lib/timelineModel.js'
import { EventStream } from './EventStream.jsx'

const dot = colour => ({ '--fam': colour })

// The right panel: the chosen day or week read in full. Summary holds the total, how it compares, the family mix with a link
// into Investigate for each family, and the pin and note; Events is where individual events will list. Tabs follow the
// ARIA pattern (arrow keys, roving focus).
export function Inspector({ caseId, bucket, typical, order, palette, position, count, onStep, questions, pinned, note, onPin, onNote, link }) {
  const [tab, setTab] = useState('summary')
  const tabs = [['summary', 'Summary'], ['events', 'Events']]
  const refs = useRef([])
  const week = bucket.end && bucket.end !== bucket.date
  const rows = dayBreakdown(bucket, order)
  const compare = week ? '' : againstTypical(bucket.total, typical)
  function onKey(event, index) {
    if (!['ArrowLeft', 'ArrowRight'].includes(event.key)) return
    event.preventDefault()
    const next = (index + (event.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length
    setTab(tabs[next][0]); refs.current[next]?.focus()
  }
  return (
    <aside className="tw-inspector" aria-label={week ? 'Selected week' : 'Selected day'}>
      <header>
        <div>
          <p className="tw-kicker">{week ? 'Selected week' : dayParts(bucket.date).weekday}</p>
          <h2>{describeBucket(bucket)}</h2>
        </div>
        <button type="button" className="tw-iconbtn" aria-pressed={pinned} onClick={onPin} aria-label={pinned ? `Unpin ${bucket.date}` : `Pin ${bucket.date}`} title={pinned ? 'Unpin' : 'Pin this day'}>{pinned ? <PinOff aria-hidden="true" /> : <Pin aria-hidden="true" />}</button>
      </header>
      <p className="tw-total"><b>{formatNumber(bucket.total)}</b> {bucket.total === 1 ? 'event' : 'events'}</p>
      {compare ? <p className="tw-compare">{compare}</p> : null}
      <div className="tw-step" role="group" aria-label={`Move between ${week ? 'weeks' : 'days'}`}>
        <button type="button" onClick={() => onStep(-1)} disabled={position <= 0}><ChevronLeft aria-hidden="true" />Earlier</button>
        <button type="button" onClick={() => onStep(1)} disabled={position >= count - 1}>Later<ChevronRight aria-hidden="true" /></button>
      </div>

      <div className="tw-tabs" role="tablist" aria-label="Inspector">
        {tabs.map(([id, label], index) => <button key={id} ref={node => { refs.current[index] = node }} type="button" role="tab" id={`tw-tab-${id}`} aria-selected={tab === id} aria-controls={`tw-panel-${id}`} tabIndex={tab === id ? 0 : -1} onKeyDown={event => onKey(event, index)} onClick={() => setTab(id)}>{label}</button>)}
      </div>
      <div className="tw-panel" role="tabpanel" id={`tw-panel-${tab}`} aria-labelledby={`tw-tab-${tab}`}>
        {tab === 'summary' ? (
          <>
            <div className="tw-stack" role="img" aria-label={rows.map(row => `${curatedFamilyLabel(row.id)} ${formatNumber(row.count)}`).join(', ')}>
              {rows.map(row => <i key={row.id} style={{ ...dot(familyColour(row.id, order, palette)), flexGrow: row.count }} />)}
            </div>
            <ul className="tw-mix">
              {rows.map(row => (
                <li key={row.id} style={dot(familyColour(row.id, order, palette))}>
                  <i aria-hidden="true" /><span>{curatedFamilyLabel(row.id)}</span><b>{formatNumber(row.count)}</b><small>{row.share < 1 ? '<1%' : `${row.share}%`}</small>
                  <Link to={link(bucket, row.id)} aria-label={`Open ${curatedFamilyLabel(row.id)} events in Investigate`}>Open</Link>
                </li>
              ))}
            </ul>
            {pinned ? (
              <label className="tw-note"><span>Note on this day <small>Saved in this browser</small></span>
                <textarea value={note} maxLength={400} rows={3} onChange={event => onNote(event.target.value)} placeholder="Why this day matters" />
              </label>
            ) : null}
          </>
        ) : <EventStream caseId={caseId} bucket={bucket} />}
      </div>
      <Link className="tw-btn tw-btn--primary" to={link(bucket)}><MessageSquareText aria-hidden="true" />Investigate {week ? 'this week' : 'this day'}</Link>
      {questions.length ? (
        <section className="tw-ask" aria-labelledby="tw-ask">
          <h3 id="tw-ask">Ask about a sequence</h3>
          <ul>{questions.slice(0, 3).map(item => <li key={`${item.scope}-${item.question}`}><Link to={`/cases/${encodeURIComponent(caseId)}/investigate?${new URLSearchParams({ question: item.question, ...(item.scope !== 'all' ? { scope: item.scope } : {}) })}`}>{item.question}</Link><small>{item.family}</small></li>)}</ul>
        </section>
      ) : null}
    </aside>
  )
}
