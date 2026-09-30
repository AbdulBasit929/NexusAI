import { readinessRing } from '../../lib/caseDossier.js'
import { formatNumber } from '../../lib/format.js'

const R = 52
const C = 2 * Math.PI * R
const GAP = 3

// Readiness as a ring: one arc per state, the floored share of ready sources in the middle, and every count spelled out
// beside it, so colour is never the only signal. A ring of nothing is not drawn.
export function ReadinessRing({ summary }) {
  const ring = readinessRing(summary)
  const description = ring.parts.map(part => `${part.label} ${formatNumber(part.value)}`).join(', ')
  let offset = 0
  return (
    <div className="co-ring">
      <svg viewBox="0 0 128 128" role="img" aria-label={ring.total ? `Evidence readiness: ${description} of ${formatNumber(ring.total)} sources` : 'No evidence yet'}>
        <circle className="co-ring__track" cx="64" cy="64" r={R} />
        {ring.parts.map(part => {
          const length = (part.value / ring.total) * C
          const arc = <circle key={part.id} className={`co-ring__arc co-ring__arc--${part.id}`} cx="64" cy="64" r={R} strokeDasharray={`${Math.max(0.5, length - (ring.parts.length > 1 ? GAP : 0))} ${C}`} strokeDashoffset={-offset} />
          offset += length
          return arc
        })}
      </svg>
      <div className="co-ring__centre" aria-hidden="true">
        <b>{ring.percent === null ? '—' : `${ring.percent}%`}</b>
        <span>ready</span>
      </div>
      <ul className="co-ring__legend" aria-label="Evidence readiness">
        {ring.parts.length ? ring.parts.map(part => <li key={part.id} className={`co-ring__key co-ring__key--${part.id}`}><i aria-hidden="true" />{part.label}<b>{formatNumber(part.value)}</b></li>) : <li className="co-ring__empty">No sources</li>}
      </ul>
    </div>
  )
}
