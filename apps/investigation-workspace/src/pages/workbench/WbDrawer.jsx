import { useRef } from 'react'
import { ChevronDown, X } from 'lucide-react'
import { STRENGTH } from '../../lib/workbench/answerMarks.js'
import { CarrierBadge } from '../../components/CarrierBadge.jsx'
import { WbTable } from './WbTable.jsx'

// The evidence panel of the workbench: every mark opens its evidence. What the mark is and how strong it is (observed, inferred or a candidate: never colour alone), the values behind it with where each
// came from, the caveats of its analysis in the engine's own words, and the rows. A panel beside the work, never a modal over it (NN/g). Nothing here is drawn that the case does not hold.
export function Badge({ strength }) {
  const s = STRENGTH[strength]
  if (!s) return null
  return <span className={`wbp-badge wbp-badge--${strength}`} title={`${s.label}: ${s.note}`}><i aria-hidden="true" />{s.label}</span>
}

// Questions the engine is known to answer, for the numbers on screen. Choosing one asks it (on the Ask page, which is where questions are asked).
export function AskMenu({ items, onAsk, label = 'Ask about this', up = false }) {
  const ref = useRef(null)
  if (!items?.length || !onAsk) return null
  return (
    <details className={`wbp-menu${up ? ' wbp-menu--up' : ''}`} ref={ref}>
      <summary className="wbp-btn wbp-btn--primary">{label} <ChevronDown size={14} aria-hidden="true" /></summary>
      <ul className="wbp-menu__list" aria-label="Questions about this">
        {items.map(item => <li key={item.id}><button type="button" onClick={() => { onAsk?.(item); if (ref.current) ref.current.open = false }}><b>{item.label}</b><span>{item.question.length > 90 ? `${item.question.slice(0, 88)}…` : item.question}</span></button></li>)}
      </ul>
    </details>
  )
}

export function WbDrawer({ drawer, onClose, onAsk, onCopied, onRow }) {
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
          <div className="wbp-in__badges">
            <Badge strength={drawer.strength} />
            {drawer.carrier ? <CarrierBadge carrier={drawer.carrier.id} size={16} showName how={drawer.carrier.how} /> : null}
          </div>
        </div>
        <button type="button" className="wbp-icon" onClick={onClose} aria-label="Close the evidence"><X size={16} aria-hidden="true" /></button>
      </header>
      <section className="wbp-in__sec" aria-label="Values">
        <h3>Values</h3>
        <dl className="wbp-facts">{drawer.facts.map(([k, v]) => <div key={k}><dt>{k}</dt><dd>{v}</dd></div>)}</dl>
        {drawer.values?.length ? <ul className="wbp-used">{drawer.values.map(v => <li key={v.label}><b>{v.label}</b> {v.value} <span>({v.source})</span></li>)}</ul> : null}
      </section>
      <section className="wbp-in__sec" aria-label="Read with care">
        <h3>Read with care</h3>
        {drawer.caveats.map(text => <p key={text} className="wbp-caveat">{text}</p>)}
      </section>
      {drawer.rows?.length ? (
        <section className="wbp-in__sec" aria-label={drawer.rowsTitle}>
          <h3>{drawer.rowsTitle}</h3>
          <WbTable columns={drawer.columns} rows={drawer.rows} pick={drawer.pick} label={drawer.rowsTitle} onCopied={onCopied} onRow={drawer.rowTargets ? i => onRow?.(drawer.rowTargets[i]) : undefined} />
        </section>
      ) : null}
      <footer className="wbp-in__foot">
        <AskMenu items={drawer.ask} onAsk={onAsk} up />
        <span className="wbp-hint">Times on the case clock.</span>
      </footer>
    </aside>
  )
}
