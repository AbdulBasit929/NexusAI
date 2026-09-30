import { useEffect, useRef } from 'react'
import { X } from 'lucide-react'
import { LanguageText, TechnicalDisclosure } from '../../components/AnalystComponents.jsx'
import { SourcePanel } from '../../components/Citations.jsx'
import { answerSummary } from '../../lib/answerSummary.js'

// The evidence behind one answer, opened on demand beside the thread: what it rests on, the sources with each file's share of
// the rows, how it was derived, and what it cannot claim. Keeping it here keeps the thread short and readable while the proof
// is one click away (the pattern citation-first assistants converge on). It is a labelled complementary region; Escape or
// the close button returns focus to the message that opened it.
export function EvidencePanel({ turn, onClose, caseLabel = null }) {
  const closeRef = useRef(null)
  const presentation = turn?.presentation
  useEffect(() => { closeRef.current?.focus() }, [turn?.id])
  useEffect(() => {
    const onKey = event => { if (event.key === 'Escape') onClose() }
    globalThis.document.addEventListener('keydown', onKey)
    return () => globalThis.document.removeEventListener('keydown', onKey)
  }, [onClose])
  if (!presentation) return null
  const summary = ['unsupported', 'failed', 'clarify'].includes(presentation.state) ? [] : answerSummary(presentation)
  return (
    <aside className="ch-panel" aria-labelledby="ch-panel-title">
      <header className="ch-panel__head">
        <div>
          <h2 id="ch-panel-title">Evidence for this answer</h2>
          {caseLabel ? <p className="ch-panel__case"><LanguageText as="bdi" identifier>{caseLabel}</LanguageText></p> : null}
          <p className="ch-panel__q"><LanguageText>{turn.query}</LanguageText></p>
        </div>
        <button type="button" ref={closeRef} className="ch-icon" onClick={onClose} aria-label="Close evidence panel"><X aria-hidden="true" /></button>
      </header>
      <div className="ch-panel__body">
        {summary.length ? (
          <dl className="ch-panel__facts" aria-label="What this answer rests on">
            {summary.map(item => <div key={item.id}><dt>{item.label}</dt><dd><b><LanguageText>{item.value}</LanguageText></b>{item.detail ? <small>{item.detail}</small> : null}</dd></div>)}
          </dl>
        ) : null}
        <section aria-labelledby="ch-panel-sources"><h3 id="ch-panel-sources">Sources</h3><SourcePanel citations={presentation.citations} /></section>
        {presentation.derivation ? <section aria-labelledby="ch-panel-method"><h3 id="ch-panel-method">How this was derived</h3><TechnicalDisclosure summary="Read the analysis method"><p>{presentation.derivation}</p></TechnicalDisclosure></section> : null}
        {presentation.limitations.length ? <section aria-labelledby="ch-panel-limits"><h3 id="ch-panel-limits">Limitations</h3><ul className="ch-panel__limits">{presentation.limitations.map(item => <li key={item}>{item}</li>)}</ul></section> : null}
      </div>
    </aside>
  )
}
