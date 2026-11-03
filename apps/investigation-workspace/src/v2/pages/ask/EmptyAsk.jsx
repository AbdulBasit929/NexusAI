import { Link } from 'react-router-dom'
import { ArrowUpRight, History, MessageSquareText, Network } from 'lucide-react'
import { coverageLine } from '../../../pages/investigate/CaseStart.jsx'
import { CaseLabel } from '../../shell/CaseLabel.jsx'

function StarterGroup({ title, icon: Icon, items, onPick }) {
  if (!items.length) return null
  return (
    <section className="nx-starters" aria-label={title}>
      <h2><Icon aria-hidden="true" />{title}</h2>
      <ul>
        {items.map(item => (
          <li key={`${item.scope}-${item.query}`}>
            <button type="button" className="nx-starter" onClick={() => onPick(item)}>
              <span className="nx-starter__text" dir="auto">{item.label || item.query}</span>
              <span className="nx-starter__family">{item.family}</span>
            </button>
          </li>
        ))}
      </ul>
    </section>
  )
}

// The empty conversation: what this case holds in one line, the question box, questions the case can answer written with its own numbers,
// and the analyst's own last questions. A start, not a poster: nothing here is invented in the browser.
export function EmptyAsk({ caseId, overview, links, evidence, recent, onPick, children }) {
  const coverage = coverageLine(overview)
  return (
    <section className="nx-start" aria-labelledby="nx-start-title">
      <div className="nx-start__head">
        <p className="nx-start__eyebrow"><CaseLabel caseId={caseId} /></p>
        <h1 id="nx-start-title">What do you want to find out?</h1>
        <p className="nx-start__coverage" role="status">{coverage || 'Checking what this case holds…'}</p>
      </div>
      {children}
      <div className="nx-start__grid">
        <StarterGroup title="Connections in this case" icon={Network} items={links} onPick={onPick} />
        <StarterGroup title="From the evidence here" icon={MessageSquareText} items={evidence} onPick={onPick} />
        {recent.length ? (
          <section className="nx-starters" aria-label="Your last questions here">
            <h2><History aria-hidden="true" />Your last questions here</h2>
            <ul>
              {recent.map(entry => <li key={entry.id}><button type="button" className="nx-starter" onClick={() => onPick({ query: entry.query, scope: 'all' })}><span className="nx-starter__text" dir="auto">{entry.label || entry.query}</span></button></li>)}
            </ul>
          </section>
        ) : null}
      </div>
      <p className="nx-start__across">Not sure which case holds it? <Link to="/investigate">Ask across all cases<ArrowUpRight aria-hidden="true" /></Link></p>
    </section>
  )
}
