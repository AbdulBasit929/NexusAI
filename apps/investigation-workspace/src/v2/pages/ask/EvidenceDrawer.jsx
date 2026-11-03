import { useEffect, useRef } from 'react'
import { X } from 'lucide-react'
import { MethodDetails } from '../../../components/AnalystComponents.jsx'
import { SourcePanel } from '../../../components/Citations.jsx'
import { AnswerEvidence } from '../../../components/AnswerBasis.jsx'
import { answerSummary } from '../../../lib/answerSummary.js'

// The proof of one answer, beside the thread: what it rests on, the records behind the figure (each opens in its file at its row), the sources with
// each file's share, how it was worked out, and what it cannot claim. Escape or the close button returns focus to the answer that opened it.
export function EvidenceDrawer({ turn, caseId, onClose }) {
  const closeRef = useRef(null)
  const presentation = turn?.presentation
  useEffect(() => { closeRef.current?.focus() }, [turn?.id])
  useEffect(() => {
    const onKey = event => { if (event.key === 'Escape') onClose() }
    globalThis.document.addEventListener('keydown', onKey)
    return () => globalThis.document.removeEventListener('keydown', onKey)
  }, [onClose])
  if (!presentation) return null
  const facts = ['unsupported', 'failed', 'clarify'].includes(presentation.state) ? [] : answerSummary(presentation)
  return (
    <aside className="nx-drawer" aria-labelledby="nx-drawer-title">
      <header className="nx-drawer__head">
        <div>
          <h2 id="nx-drawer-title">Evidence for this answer</h2>
          <p className="nx-drawer__q" dir="auto">{turn.query}</p>
        </div>
        <button type="button" ref={closeRef} className="nx-btn nx-btn--ghost nx-btn--icon" onClick={onClose} aria-label="Close the evidence"><X aria-hidden="true" /></button>
      </header>
      <div className="nx-drawer__body">
        {facts.length ? (
          <dl className="nx-facts" aria-label="What this answer rests on">
            {facts.map(item => <div key={item.id}><dt>{item.label}</dt><dd><b dir="auto">{item.value}</b>{item.detail ? <small>{item.detail}</small> : null}</dd></div>)}
          </dl>
        ) : null}
        <section aria-labelledby="nx-drawer-records">
          <h3 id="nx-drawer-records">Records behind the figure</h3>
          <AnswerEvidence evidence={presentation.original?.enterprise?.evidence} caseId={caseId} defaultOpen />
        </section>
        <section aria-labelledby="nx-drawer-sources">
          <h3 id="nx-drawer-sources">Sources</h3>
          <SourcePanel citations={presentation.citations} />
        </section>
        {presentation.derivation ? (
          <section aria-labelledby="nx-drawer-method"><h3 id="nx-drawer-method">How this was worked out</h3><p>{presentation.derivation}</p><MethodDetails method={presentation.method} /></section>
        ) : null}
        {presentation.limitations.length ? (
          <section aria-labelledby="nx-drawer-limits"><h3 id="nx-drawer-limits">What it cannot claim</h3><ul className="nx-bullets">{presentation.limitations.map(item => <li key={item}>{item}</li>)}</ul></section>
        ) : null}
      </div>
    </aside>
  )
}
