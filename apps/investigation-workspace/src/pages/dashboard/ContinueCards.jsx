import { Link } from 'react-router-dom'
import { ArrowRight, History, MessageCircleQuestion, Pin } from 'lucide-react'
import { LanguageText } from '../../components/AnalystComponents.jsx'
import { Card } from '../../components/Card.jsx'
import { formatNumber } from '../../lib/format.js'

const ask = (caseId, question) => `/cases/${encodeURIComponent(caseId)}/investigate?question=${encodeURIComponent(question)}`

// The two ways back into the work. "Pick up" is the analyst's own recent questions (this browser only, pinned first);
// "Suggested" is what the service says the chosen case can answer, from its own question corpus. Both are links into
// Investigate, and each says honestly when it has nothing.
export function ContinueCards({ recent, suggestions, suggestionCase, suggestionsLoading }) {
  const ordered = [...recent].sort((left, right) => Number(Boolean(right.pinned)) - Number(Boolean(left.pinned)))
  return (
    <div className="dash-grid dash-grid--even dash-grid--short">
      <div className="dash-slot">
        <Card className="continue-card" title="Pick up where you left off" actions={<span className="attention-card__count"><History aria-hidden="true" />This browser</span>}>
          {ordered.length ? (
            <ol className="cc-list">
              {ordered.map(entry => (
                <li key={`${entry.caseId}-${entry.id}`}>
                  <Link to={ask(entry.caseId, entry.query)} title={entry.label ? entry.query : undefined}>
                    <span className="cc-text">
                      <span className="cc-question"><LanguageText>{entry.label || entry.query}</LanguageText>{entry.pinned ? <Pin className="cc-pin" aria-label="Pinned" /> : null}</span>
                      {entry.label ? <small><LanguageText>{entry.query}</LanguageText></small> : null}
                    </span>
                    <span className="cc-case"><bdi dir="ltr">{entry.caseId}</bdi><ArrowRight aria-hidden="true" /></span>
                  </Link>
                </li>
              ))}
            </ol>
          ) : <p className="dash-card__empty">Questions you ask here appear as links that reopen them. <Link to="/investigate">Ask a question</Link></p>}
        </Card>
      </div>
      <div id="dashboard-suggested" className="dash-slot">
        <Card className="continue-card" title="What could I ask?" actions={suggestionCase ? <span className="attention-card__count"><MessageCircleQuestion aria-hidden="true" /><LanguageText as="bdi" identifier>{suggestionCase}</LanguageText></span> : null}>
          {suggestions.length ? (
            <ol className="cc-list">
              {suggestions.map(item => (
                <li key={item.query}>
                  <Link to={ask(suggestionCase, item.query)}>
                    <span className="cc-text"><span className="cc-question"><LanguageText>{item.query}</LanguageText></span></span>
                    <span className="cc-case"><ArrowRight aria-hidden="true" /></span>
                  </Link>
                </li>
              ))}
            </ol>
          ) : suggestionsLoading ? <p className="dash-card__empty" role="status">Loading suggestions…</p> : <p className="dash-card__empty">{suggestionCase ? `No suggested questions are reported for this case yet.` : 'Suggestions appear once a case has ready evidence.'} <Link to="/investigate">Ask your own</Link></p>}
          {suggestions.length ? <p className="dash-card__footnote">{formatNumber(suggestions.length)} from the questions this case can answer.</p> : null}
        </Card>
      </div>
    </div>
  )
}
