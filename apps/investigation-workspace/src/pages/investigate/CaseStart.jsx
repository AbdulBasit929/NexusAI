import { Link } from 'react-router-dom'
import { ArrowUpRight, History, Sparkles } from 'lucide-react'
import { LanguageText } from '../../components/AnalystComponents.jsx'
import { formatNumber } from '../../lib/format.js'
import { summariseCase } from '../../lib/useCaseOverview.js'

// What is searched, in one sentence, from the case's own counts. Sources that are not ready are named as not searched.
export function coverageLine(overview) {
  if (!overview?.data) return ''
  const summary = summariseCase(overview.data)
  if (!summary.total) return 'This case has no evidence yet, so there is nothing to ask.'
  const notReady = summary.total - summary.ready
  const rows = summary.acceptedRows ? ` · ${formatNumber(summary.acceptedRows)} structured rows` : ''
  return `${formatNumber(summary.ready)} of ${formatNumber(summary.total)} sources ready${rows}${notReady > 0 ? `. The ${formatNumber(notReady)} not ready ${notReady === 1 ? 'is' : 'are'} not searched.` : ''}`
}

// The empty thread, as a hero: what this case can answer before the analyst types (how much evidence is ready), the composer
// itself, questions the service suggests for the families present, the analyst's own last questions here, and a way out to
// every case when they do not know where the evidence lives. Nothing is invented in the browser.
export function CaseStart({ caseId, overview, starters, recent, onPick, children = null }) {
  const coverage = coverageLine(overview)
  return (
    <section className="gi-hero case-start" aria-labelledby="investigate-start-title">
      <p className="gi-hero__eyebrow case-start__coverage"><Sparkles aria-hidden="true" />{coverage || 'Checking what this case holds…'}</p>
      <h1 id="investigate-start-title">What do you want to verify in <LanguageText as="bdi" identifier>{caseId}</LanguageText>?</h1>
      {children}
      {starters.length ? (
        <div className="gi-suggest" aria-label="Questions suggested by available evidence">
          <h2>Suggested from the evidence here</h2>
          <ul>
            {starters.map(item => (
              <li key={`${item.scope}-${item.query}`}>
                <button type="button" onClick={() => onPick(item)}><small>{item.family}</small><LanguageText>{item.query}</LanguageText></button>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
      {recent.length ? (
        <div className="gi-recent">
          <h2><History aria-hidden="true" />Your last questions here</h2>
          <ul>
            {recent.map(entry => <li key={entry.id}><button type="button" className="gi-recent__button" onClick={() => onPick({ query: entry.query, scope: 'all' })}><LanguageText>{entry.label || entry.query}</LanguageText></button></li>)}
          </ul>
        </div>
      ) : null}
      <p className="case-start__across">Not sure which case holds it? <Link to="/investigate">Ask across all cases<ArrowUpRight aria-hidden="true" /></Link></p>
    </section>
  )
}
