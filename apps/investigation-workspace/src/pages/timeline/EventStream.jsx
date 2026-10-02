import { Link } from 'react-router-dom'
import { LanguageText } from '../../components/AnalystComponents.jsx'
import { curatedFamilyLabel } from '../../lib/semanticCatalog.js'

// Individual events inside the selected day, newest first, each with its exact time, family, summary and a link to the
// source. The service does not supply an event feed yet (docs/work/BACKEND_REQUESTS.md, rows 1 and 20), so until `events` is
// provided this says so and offers the working route: Investigate on that day. Nothing is invented to fill it.
export function EventStream({ events = null, caseId, bucket }) {
  if (!events) {
    return (
      <div className="tw-events tw-events--none">
        <p><b>Events inside a day are not available yet.</b></p>
        <p>This case reports how many records fall on each day, not the individual events with their times. When the service supplies them, each event will list here with its exact time, family, a summary and a link to the source row, page or media moment.</p>
        <Link className="tw-btn" to={`/cases/${encodeURIComponent(caseId)}/investigate?question=${encodeURIComponent(`Show activity on ${bucket.date}`)}`}>Ask Investigate about this day</Link>
      </div>
    )
  }
  if (!events.length) return <p className="tw-events tw-events--none">No events were reported for this day.</p>
  return (
    <ol className="tw-events">
      {events.map(event => (
        <li key={event.id}>
          <time dateTime={event.time}>{event.time.slice(11, 19)}</time>
          <span className="tw-events__family">{curatedFamilyLabel(event.family)}</span>
          <span className="tw-events__text"><LanguageText>{event.summary}</LanguageText></span>
          {event.href ? <Link to={event.href} aria-label={`Open source for ${event.summary}`}>Open</Link> : null}
        </li>
      ))}
    </ol>
  )
}
