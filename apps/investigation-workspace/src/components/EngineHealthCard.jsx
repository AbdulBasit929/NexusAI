import { useEffect, useState } from 'react'
import { getEngineHealth } from '../lib/apiClient.js'
import { formatNumber } from '../lib/format.js'

// How the Ask engine has done on a case, in the last days: what it answered, how long that took, how often it had to repair, and why it declined (roadmap 7.8.3 row 9).
// It reads the engine's own audit rows through GET /analysis/engine-health: counts and times, never a question or an answer. A case with no questions yet, a service that does not
// have the endpoint or a failed read shows nothing and says nothing: the page it sits on does not depend on it.
const REASONS = {
  'no analyst view fits the question closely enough': 'No view of the records fits the question',
  'the model judged that the views cannot answer the question': 'The views cannot answer it',
  'the question uses a relative time (#) and no reference day is stated': 'A relative time ("last week") with no reference day',
}

export const seconds = ms => (ms == null ? '-' : `${(ms / 1000).toFixed(ms < 10000 ? 1 : 0)} s`)
export const reasonLabel = reason => REASONS[String(reason).replace(/\([^)]*\)/, '(#)')] || (String(reason).startsWith('the analyst views cannot speak for this case') ? 'The case is not fully typed yet: the older paths answered' : String(reason))

export function EngineHealthCard({ caseId, days = 7 }) {
  const [state, setState] = useState({ status: 'loading', data: null })
  useEffect(() => {
    const controller = new AbortController()
    setState({ status: 'loading', data: null })
    getEngineHealth({ caseId, days, signal: controller.signal })
      .then(data => setState(data?.available && data.totals?.questions > 0 ? { status: 'ready', data } : { status: 'none', data: null }))
      .catch(error => { if (error?.name !== 'AbortError') setState({ status: 'none', data: null }) })
    return () => controller.abort()
  }, [caseId, days])
  if (state.status !== 'ready') return null
  const { totals, latency_ms: latency, repairs, declines = [] } = state.data
  const answeredShare = totals.questions ? Math.round((100 * totals.answered) / totals.questions) : 0
  // reasons that differ only by a number ("only 3 of 6 files ...") read the same to an analyst: one line each, counts added
  const grouped = Object.values(declines.reduce((acc, d) => {
    const label = reasonLabel(d.reason)
    acc[label] = acc[label] || { label, count: 0 }
    acc[label].count += d.count
    return acc
  }, {})).sort((a, b) => b.count - a.count)
  const most = Math.max(1, ...grouped.map(d => d.count))
  return (
    <section className="eh" aria-labelledby={`eh-title-${caseId}`}>
      <h2 id={`eh-title-${caseId}`}>How the Ask engine has done here, last {days} days</h2>
      <dl className="eh__tiles">
        <div><dt>Questions it looked at</dt><dd>{formatNumber(totals.questions)}</dd></div>
        <div><dt>Answered</dt><dd>{formatNumber(totals.answered)} <small>{answeredShare}%</small></dd></div>
        <div><dt>Time to answer, median</dt><dd>{seconds(latency?.p50)}</dd></div>
        <div><dt>Time to answer, 9 in 10</dt><dd>{seconds(latency?.p90)}</dd></div>
        <div><dt>Repairs per answer</dt><dd>{totals.answered ? (repairs / totals.answered).toFixed(1) : '-'}</dd></div>
      </dl>
      {grouped.length ? (
        <>
          <h3 className="visually-hidden">Why it did not answer</h3>
          <ul className="eh__list" aria-label="Why the engine did not answer">
            {grouped.slice(0, 4).map(d => (
              <li key={d.label}>
                <span>{d.label}</span><b>{formatNumber(d.count)}</b>
                <span className="eh__bar" aria-hidden="true"><i style={{ inlineSize: `${Math.round((100 * d.count) / most)}%` }} /></span>
              </li>
            ))}
          </ul>
        </>
      ) : null}
      <p className="cx-muted">A question the engine does not answer is passed to the older paths, which answer it as before; an abstention says why instead of guessing.</p>
    </section>
  )
}
