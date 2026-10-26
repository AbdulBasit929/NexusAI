import { X } from 'lucide-react'
import { CAVEATS, STRENGTH } from '../../lib/workbenchProof.js'
import { WbTable } from './WbTable.jsx'

// The inspector of the workbench direction proof: every mark opens its evidence. What the mark is and how strong it is (observed, inferred or a candidate: never colour alone), the values behind it with
// where each came from, the caveat of its analysis in the engine's own words, and the rows. A panel beside the work, never a modal over it (NN/g). Nothing here is drawn that the fixture does not hold.
export function Badge({ strength }) {
  const s = STRENGTH[strength]
  if (!s) return null
  return <span className={`wbp-badge wbp-badge--${strength}`} title={`${s.label}: ${s.note}`}><i aria-hidden="true" />{s.label}</span>
}

export function WbDrawer({ drawer, onClose, onAsk, onCopied }) {
  if (!drawer) {
    return (
      <aside className="wbp-inspector wbp-inspector--empty" aria-label="Evidence">
        <h2>Evidence</h2>
        <p>Select a mark on the links, the map or the timeline, or a row of a table, and its evidence opens here: the values, the rows behind them, the time basis and the caveat of the analysis.</p>
        <p className="wbp-hint">Nothing is drawn that the case does not hold. <kbd>Esc</kbd> closes it; <kbd>[</kbd> and <kbd>]</kbd> step through the marks.</p>
      </aside>
    )
  }
  return (
    <aside className="wbp-inspector" aria-label={`Evidence: ${drawer.title}`}>
      <header className="wbp-in__head">
        <div>
          <h2>{drawer.title}</h2>
          <p>{drawer.subtitle}</p>
          <Badge strength={drawer.strength} />
        </div>
        <button type="button" className="wbp-icon" onClick={onClose} aria-label="Close the evidence"><X size={16} aria-hidden="true" /></button>
      </header>
      <section className="wbp-in__sec" aria-label="Values">
        <h3>Values</h3>
        <dl className="wbp-facts">{drawer.facts.map(([k, v]) => <div key={k}><dt>{k}</dt><dd>{v}</dd></div>)}</dl>
      </section>
      <section className="wbp-in__sec" aria-label="Read with care">
        <h3>Read with care</h3>
        <p className="wbp-caveat">{CAVEATS[drawer.caveat]}</p>
      </section>
      <section className="wbp-in__sec" aria-label={drawer.rowsTitle}>
        <h3>{drawer.rowsTitle}</h3>
        <WbTable columns={drawer.columns} rows={drawer.rows} pick={drawer.pick} label={drawer.rowsTitle} onCopied={onCopied} />
      </section>
      <footer className="wbp-in__foot">
        <button type="button" className="wbp-btn wbp-btn--primary" onClick={() => onAsk?.(drawer)}>Ask about this</button>
        <span className="wbp-hint">Times on the case clock, Asia/Karachi.</span>
      </footer>
    </aside>
  )
}
