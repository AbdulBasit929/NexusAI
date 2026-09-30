import { useNavigate } from 'react-router-dom'
import { CircleAlert, CircleCheckBig, CircleHelp, CircleSlash, Clock3, ExternalLink, SearchX } from 'lucide-react'
import { InvestigationResult } from '../../components/InvestigationResult.jsx'
import { LanguageText } from '../../components/AnalystComponents.jsx'
import { OUTCOME_LABEL, outcomeHeadline, sortOutcomes, tallyOutcomes } from '../../lib/acrossCases.js'
import { formatNumber } from '../../lib/format.js'

const ICON = { answered: CircleCheckBig, partial: CircleCheckBig, clarify: CircleHelp, processing: Clock3, 'zero-result': SearchX, unsupported: CircleSlash, failed: CircleAlert }
const SKIP_REASON = { no_evidence: 'no evidence yet', not_chosen: 'not selected' }
const OPEN_FIRST = 3

export const caseQuestionLink = (caseId, question) => `/cases/${encodeURIComponent(caseId)}/investigate?question=${encodeURIComponent(question)}`

// The results of one question asked of every case. The headline says how many cases had something; the chips are a count
// of each outcome and jump to those cases; each case keeps its own verified result, best first, so an answer is never
// blended with another case's. Cases that answered open at once, the rest are one line until asked. A case that could not
// be searched says so and can be retried alone. Coverage is the case's own count of ready sources.
export function AcrossResults({ run, coverage, onRetryCase }) {
  const navigate = useNavigate()
  const outcomes = sortOutcomes(run.outcomes)
  const tally = tallyOutcomes(outcomes)
  const done = run.outcomes.length
  const total = run.caseIds.length
  const pending = run.busy ? run.caseIds.filter(id => !run.outcomes.some(outcome => outcome.caseId === id)) : []
  const chips = Object.entries(tally.counts).filter(([, count]) => count > 0)
  let opened = 0

  return (
    <section className="ax" aria-labelledby="ax-title">
      <header className="ax__head">
        <div>
          <p className="ax__question"><LanguageText>{run.query}</LanguageText></p>
          <h2 id="ax-title">{run.busy ? `Searching ${formatNumber(total)} ${total === 1 ? 'case' : 'cases'}…` : outcomeHeadline(tally, total)}</h2>
        </div>
        {run.busy ? <p className="ax__progress" role="status"><progress max={total} value={done} aria-label="Cases searched" /><span>{formatNumber(done)} of {formatNumber(total)} searched</span></p> : null}
      </header>
      {chips.length ? (
        <ul className="ax__chips" aria-label="Outcome by case">
          {chips.map(([state, count]) => {
            const Icon = ICON[state]
            const first = outcomes.find(outcome => outcome.presentation.state === state)
            return <li key={state}><a className={`ax-chip ax-chip--${state}`} href={`#ax-${encodeURIComponent(first.caseId)}`}><Icon aria-hidden="true" /><b>{formatNumber(count)}</b>{OUTCOME_LABEL[state]}</a></li>
          })}
        </ul>
      ) : null}
      {run.skipped.length ? <p className="ax__skipped">Not searched: {run.skipped.map(item => `${item.caseId} (${SKIP_REASON[item.reason]})`).join(', ')}.</p> : null}

      <ol className="ax__cases">
        {outcomes.map(({ caseId, presentation }) => {
          const Icon = ICON[presentation.state]
          const isOpen = ['answered', 'partial', 'clarify'].includes(presentation.state) && opened < OPEN_FIRST
          if (isOpen) opened += 1
          const link = caseQuestionLink(caseId, run.query)
          return (
            <li key={caseId} id={`ax-${encodeURIComponent(caseId)}`} className={`ax-case ax-case--${presentation.state}`}>
              <details open={isOpen}>
                <summary>
                  <span className="ax-case__mark" aria-hidden="true"><Icon /></span>
                  <span className="ax-case__name"><LanguageText as="bdi" identifier>{caseId}</LanguageText></span>
                  <span className="ax-case__state">{OUTCOME_LABEL[presentation.state]}</span>
                  {coverage[caseId] ? <span className="ax-case__coverage">{coverage[caseId]}</span> : null}
                </summary>
                <div className="ax-case__body">
                  <InvestigationResult
                    presentation={presentation}
                    idPrefix={`ax-${caseId}`}
                    live={false}
                    onAsk={question => navigate(caseQuestionLink(caseId, question))}
                    onClarificationChoice={option => (option?.query ? navigate(caseQuestionLink(caseId, option.query)) : undefined)}
                    onRetry={() => onRetryCase(caseId)}
                    showOriginalQuestion={false}
                  />
                  <p className="ax-case__open"><a href={link} onClick={event => { event.preventDefault(); navigate(link) }}>Continue in this case<ExternalLink aria-hidden="true" /></a></p>
                </div>
              </details>
            </li>
          )
        })}
        {pending.map(caseId => (
          <li key={caseId} className="ax-case ax-case--pending" aria-label={`Searching ${caseId}`}>
            <span className="ax-case__mark" aria-hidden="true"><Clock3 /></span>
            <span className="ax-case__name"><LanguageText as="bdi" identifier>{caseId}</LanguageText></span>
            <span className="ax-case__state">Searching…</span>
          </li>
        ))}
      </ol>
    </section>
  )
}
