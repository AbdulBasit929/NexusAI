import { useEffect, useState } from 'react'
import { getTypingReport } from '../lib/apiClient.js'
import { formatNumber } from '../lib/format.js'

// How the data of a case was read, and what to read before trusting an answer over it (roadmap 7.8.3 rows 6 and 10): the files that are in the typed records, the call rows and the
// exact repeats that counts leave out, the kinds of event, the internet records and the evidence files, from GET /analysis/typing-report. Counts only; the findings are written by the
// service in plain words from them. A service that does not have the endpoint, a failed read or a case with nothing typed shows nothing: the page it sits on does not depend on it.
const EVENT_LABELS = { voice: 'Voice calls', sms: 'Text messages', mms: 'Multimedia messages', data: 'Data', ussd: 'USSD', other: 'Other kinds' }
const MODALITY_LABELS = { structured_records: 'Record files', image: 'Images', audio: 'Audio', video: 'Video', document: 'Documents', tabular: 'Tables', text: 'Text' }

export const eventLabel = name => EVENT_LABELS[name] || name
export const modalityLabel = name => MODALITY_LABELS[name] || String(name).replace(/_/g, ' ')

export function DataQualityCard({ caseId }) {
  const [state, setState] = useState({ status: 'loading', data: null })
  useEffect(() => {
    const controller = new AbortController()
    setState({ status: 'loading', data: null })
    getTypingReport({ caseId, signal: controller.signal })
      .then(data => {
        const something = data?.available && ((data.calls?.typed || 0) + (data.sessions?.typed || 0) + (data.usage?.typed || 0) + (data.evidence?.length || 0) > 0)
        setState(something ? { status: 'ready', data } : { status: 'none', data: null })
      })
      .catch(error => { if (error?.name !== 'AbortError') setState({ status: 'none', data: null }) })
    return () => controller.abort()
  }, [caseId])
  if (state.status !== 'ready') return null
  const { files, calls, sessions, usage, evidence = [], findings = [] } = state.data
  const kinds = (calls?.event_types || []).slice(0, 4)
  const most = Math.max(1, ...kinds.map(k => k.count))
  return (
    <section className="dq" aria-labelledby={`dq-title-${caseId}`}>
      <h2 id={`dq-title-${caseId}`}>How the data of this case was read</h2>
      <dl className="eh__tiles">
        {files?.counted ? <div><dt>Record files in the typed records</dt><dd>{formatNumber(files.typed)} <small>of {formatNumber(files.counted)}</small></dd></div> : null}
        {calls?.typed ? <div><dt>Call rows counted</dt><dd>{formatNumber(calls.effective)}{calls.repeats ? <small> ({formatNumber(calls.repeats)} repeats left out)</small> : null}</dd></div> : null}
        {sessions?.typed ? <div><dt>Internet sessions</dt><dd>{formatNumber(sessions.typed)}</dd></div> : null}
        {usage?.typed ? <div><dt>Application usage rows</dt><dd>{formatNumber(usage.typed)}</dd></div> : null}
      </dl>
      {kinds.length ? (
        <>
          <h3 className="visually-hidden">Kinds of call row</h3>
          <ul className="eh__list" aria-label="Kinds of call row">
            {kinds.map(k => (
              <li key={k.name}>
                <span>{eventLabel(k.name)}</span><b>{formatNumber(k.count)}</b>
                <span className="eh__bar" aria-hidden="true"><i style={{ inlineSize: `${Math.round((100 * k.count) / most)}%` }} /></span>
              </li>
            ))}
          </ul>
        </>
      ) : null}
      {evidence.length ? (
        <p className="cx-muted dq__evidence">
          Evidence files: {evidence.map(e => `${formatNumber(e.files)} ${modalityLabel(e.modality).toLowerCase()}${e.failed ? ` (${formatNumber(e.failed)} failed)` : ''}`).join(', ')}.
        </p>
      ) : null}
      {findings.length ? (
        <div className="dq__findings" role="note">
          <h3>Read before trusting an answer</h3>
          <ul>
            {findings.map(f => <li key={f.code + f.text}>{f.text}</li>)}
          </ul>
        </div>
      ) : <p className="cx-muted">Nothing here needs a second look.</p>}
    </section>
  )
}
