import { useEffect, useState } from 'react'
import { getCaseOverview } from '../lib/apiClient.js'
import { displayName } from '../lib/format.js'
import { EmptyState, LanguageText, ProcessingBadge } from './AnalystComponents.jsx'

// The wait between "evidence accepted" and "evidence answerable".
//
// This is the state the product previously had no surface for, and it is the
// one an analyst is most likely to misread. Four distinct truths must stay
// distinct -- the design system names them, and collapsing them is how a
// product ends up saying "no results" about data it simply has not read yet:
//
//   queued / running   not ready, and not a failure
//   completed          ready, and answerable
//   failed             it will not become ready without intervention
//   not-processed      never attempted
//
// There is NO percentage here and none is invented. The service reports a
// state per evidence item, so a state is what is shown. Counts are exact.

const terminal = new Set(['completed', 'failed'])

export function ProcessingWait({ caseId, pollMs = 5000, fetcher = getCaseOverview, onReady }) {
  const [state, setState] = useState({ loading: true })

  useEffect(() => {
    let cancelled = false
    let timer
    const controller = new AbortController()

    async function poll() {
      try {
        const data = await fetcher({ caseId, signal: controller.signal })
        if (cancelled) return
        setState({ data, loading: false })
        const items = data?.recent_evidence || []
        const settled = items.length > 0 && items.every(item => terminal.has(item.processing_status))
        if (settled) {
          onReady?.(data)
          return
        }
      } catch (error) {
        if (cancelled || error.name === 'AbortError') return
        setState({ error, loading: false })
      }
      // Keep polling even after an error: a transient failure to READ the
      // status is not a failure of the processing itself, and stopping here
      // would strand the analyst on a stale screen.
      timer = globalThis.setTimeout(poll, pollMs)
    }

    poll()
    return () => { cancelled = true; controller.abort(); globalThis.clearTimeout(timer) }
  }, [caseId, pollMs, fetcher, onReady])

  const items = state.data?.recent_evidence || []
  const counts = items.reduce((totals, item) => {
    const key = item.processing_status || 'not_processed'
    return { ...totals, [key]: (totals[key] || 0) + 1 }
  }, {})
  const waiting = items.filter(item => !terminal.has(item.processing_status))
  const failed = items.filter(item => item.processing_status === 'failed')

  return (
    <section className="processing-wait" aria-labelledby="processing-heading">
      <h2 id="processing-heading">Processing</h2>

      {state.loading && <p role="status">Checking what is ready…</p>}

      {state.error && (
        <p className="state-message state-message--failed" role="status">
          The processing status could not be read just now; still checking.
          {state.error.reference ? ` Reference: ${state.error.reference}` : ''}
        </p>
      )}

      {!state.loading && !items.length && (
        <EmptyState kind="not-processed" label="No evidence has been added to this case yet" description="Add evidence and wait for processing before asking questions about this case." />
      )}

      {items.length > 0 && (
        <>
          <p role="status" aria-live="polite">
            {waiting.length > 0
              ? `${waiting.length} of ${items.length} evidence items are still being processed. Questions asked now cover only what is ready.`
              : `All ${items.length} evidence items have finished processing.`}
          </p>

          <dl className="processing-wait__counts">
            {Object.entries(counts).map(([status, count]) => (
              <div key={status}>
                <dt>{displayName(status)}</dt>
                <dd>{count}</dd>
              </div>
            ))}
          </dl>

          <ul className="processing-wait__list">
            {items.map(item => (
              <li key={item.evidence_id} className={`status status--${item.processing_status}`}>
                <span><LanguageText>{item.source_file}</LanguageText></span>
                <span>
                  <ProcessingBadge state={item.processing_status} />
                  {' '}<small>{displayName(item.modality)}</small>
                </span>
              </li>
            ))}
          </ul>

          {failed.length > 0 && (
            <p className="state-message state-message--failed">
              {failed.length === 1 ? 'One evidence item could not be processed' : `${failed.length} evidence items could not be processed`}
              {' '}and will not be included in answers until that is resolved.
            </p>
          )}
        </>
      )}
    </section>
  )
}
