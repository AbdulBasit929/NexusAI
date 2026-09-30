import { CircleAlert, CircleCheckBig, CircleHelp, CircleSlash, Clock3, LoaderCircle, SearchX } from 'lucide-react'
import { LanguageText } from '../../components/AnalystComponents.jsx'
import { OUTCOME_LABEL, sortOutcomes, tallyOutcomes } from '../../lib/acrossCases.js'
import { formatNumber } from '../../lib/format.js'

const ICON = { answered: CircleCheckBig, partial: CircleCheckBig, clarify: CircleHelp, processing: Clock3, 'zero-result': SearchX, unsupported: CircleSlash, failed: CircleAlert }
const SKIP_REASON = { no_evidence: 'no evidence yet', not_chosen: 'not selected' }

// The live status of the search, one row per case: waiting, searching, or its outcome. It is the "what is happening" a
// long-running question needs (intermediate progress keeps people waiting), and it is a table of contents for the results:
// each row jumps to that case. Counts by outcome sit above it; cases that were not searched are named, never dropped.
export function CaseRail({ run, coverage }) {
  const outcomes = sortOutcomes(run.outcomes)
  const tally = tallyOutcomes(outcomes)
  const byId = new Map(run.outcomes.map(outcome => [outcome.caseId, outcome]))
  const chips = Object.entries(tally.counts).filter(([, count]) => count > 0)
  const ordered = [...outcomes.map(outcome => outcome.caseId), ...run.caseIds.filter(id => !byId.has(id))]
  return (
    <aside className="gi-rail" aria-label="Search progress">
      <h2 className="gi-rail__title">{run.busy ? 'Searching' : 'Searched'} <span>{formatNumber(run.outcomes.length)} of {formatNumber(run.caseIds.length)}</span></h2>
      {chips.length ? (
        <ul className="ax__chips gi-rail__chips" aria-label="Outcome by case">
          {chips.map(([state, count]) => {
            const Icon = ICON[state]
            const first = outcomes.find(outcome => outcome.presentation.state === state)
            return <li key={state}><a className={`ax-chip ax-chip--${state}`} href={`#ax-${encodeURIComponent(first.caseId)}`}><Icon aria-hidden="true" /><b>{formatNumber(count)}</b>{OUTCOME_LABEL[state]}</a></li>
          })}
        </ul>
      ) : null}
      <ol className="gi-rail__list" aria-label="Cases searched">
        {ordered.map(caseId => {
          const outcome = byId.get(caseId)
          const state = outcome?.presentation.state
          const Icon = outcome ? ICON[state] : LoaderCircle
          return (
            <li key={caseId} className={`gi-rail__item ${outcome ? `gi-rail__item--${state}` : 'gi-rail__item--pending'}`}>
              {outcome ? <a href={`#ax-${encodeURIComponent(caseId)}`}><span className="gi-rail__mark" aria-hidden="true"><Icon /></span><span className="gi-rail__text"><LanguageText as="bdi" identifier>{caseId}</LanguageText><small>{OUTCOME_LABEL[state]}{coverage[caseId] ? ` · ${coverage[caseId]}` : ''}</small></span></a>
                : <span className="gi-rail__row"><span className="gi-rail__mark" aria-hidden="true"><Icon /></span><span className="gi-rail__text"><LanguageText as="bdi" identifier>{caseId}</LanguageText><small>{run.busy ? 'Searching…' : 'Not searched'}</small></span></span>}
            </li>
          )
        })}
      </ol>
      {run.skipped.length ? <p className="ax__skipped">Not searched: {run.skipped.map(item => `${item.caseId} (${SKIP_REASON[item.reason]})`).join(', ')}.</p> : null}
    </aside>
  )
}
