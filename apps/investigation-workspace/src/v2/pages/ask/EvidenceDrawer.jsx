import { useEffect, useRef, useState } from 'react'
import { ChevronRight, Maximize2, Minimize2, X } from 'lucide-react'
import { MethodDetails } from '../../../components/AnalystComponents.jsx'
import { answerSummary } from '../../../lib/answerSummary.js'
import { ProofRecords, ProofSources } from './Proof.jsx'

const WIDE_KEY = 'nexusai.viewer.drawer.wide'
function readWide() { try { return globalThis.localStorage?.getItem(WIDE_KEY) === '1' } catch { return false } }

// The proof of one answer, beside the thread: what it rests on, the records behind the figure (each opens in its file at its row), the sources with each file's share,
// how it was worked out, and what it cannot claim. It can be made wider for a table with many columns. Escape or the close button returns focus to the answer that opened it.
export function EvidenceDrawer({ turn, caseId, onClose }) {
  const closeRef = useRef(null)
  const [wide, setWide] = useState(readWide)
  const presentation = turn?.presentation
  useEffect(() => { closeRef.current?.focus() }, [turn?.id])
  useEffect(() => {
    const onKey = event => { if (event.key === 'Escape') onClose() }
    globalThis.document.addEventListener('keydown', onKey)
    return () => globalThis.document.removeEventListener('keydown', onKey)
  }, [onClose])
  function toggleWide() {
    setWide(value => {
      try { globalThis.localStorage?.setItem(WIDE_KEY, value ? '0' : '1') } catch { /* the choice still holds for this tab */ }
      return !value
    })
  }
  if (!presentation) return null
  const facts = ['unsupported', 'failed', 'clarify'].includes(presentation.state) ? [] : answerSummary(presentation)
  return (
    <aside className={`nx-drawer${wide ? ' nx-drawer--wide' : ''}`} aria-labelledby="nx-drawer-title">
      <header className="nx-drawer__head">
        <div className="nx-drawer__title">
          <h2 id="nx-drawer-title">Evidence for this answer</h2>
          <p className="nx-drawer__q" dir="auto">{turn.query}</p>
        </div>
        <div className="nx-drawer__tools">
          <button type="button" className="nx-btn nx-btn--ghost nx-btn--icon nx-drawer__wider" onClick={toggleWide} aria-pressed={wide} aria-label={wide ? 'Make the evidence panel narrower' : 'Make the evidence panel wider'} title={wide ? 'Narrower' : 'Wider, for tables with many columns'}>{wide ? <Minimize2 aria-hidden="true" /> : <Maximize2 aria-hidden="true" />}</button>
          <button type="button" ref={closeRef} className="nx-btn nx-btn--ghost nx-btn--icon" onClick={onClose} aria-label="Close the evidence"><X aria-hidden="true" /></button>
        </div>
      </header>
      <div className="nx-drawer__body">
        {facts.length ? (
          <dl className="nx-facts" aria-label="What this answer rests on">
            {facts.map(item => <div key={item.id}><dt>{item.label}</dt><dd><b dir="auto">{item.value}</b>{item.detail ? <small>{item.detail}</small> : null}</dd></div>)}
          </dl>
        ) : null}
        <section aria-labelledby="nx-drawer-records">
          <h3 id="nx-drawer-records">Records behind the figure</h3>
          <ProofRecords evidence={presentation.original?.enterprise?.evidence} caseId={caseId} />
        </section>
        <section aria-labelledby="nx-drawer-sources">
          <h3 id="nx-drawer-sources">Sources</h3>
          <ProofSources citations={presentation.citations} />
        </section>
        {presentation.derivation ? (
          <details className="nx-disclosure nx-drawer__method">
            <summary><ChevronRight aria-hidden="true" />How this was worked out</summary>
            <div className="nx-disclosure__body nx-disclosure__body--wide"><p>{presentation.derivation}</p><MethodDetails method={presentation.method} /></div>
          </details>
        ) : null}
        {presentation.limitations.length ? (
          <section aria-labelledby="nx-drawer-limits"><h3 id="nx-drawer-limits">What it cannot claim</h3><ul className="nx-bullets">{presentation.limitations.map(item => <li key={item}>{item}</li>)}</ul></section>
        ) : null}
      </div>
    </aside>
  )
}
