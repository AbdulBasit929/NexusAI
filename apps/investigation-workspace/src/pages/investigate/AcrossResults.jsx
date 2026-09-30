import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Check, CircleAlert, CircleCheckBig, CircleHelp, CircleSlash, Clock3, Copy, ExternalLink, FileSearch, SearchX } from 'lucide-react'
import { InvestigationResult } from '../../components/InvestigationResult.jsx'
import { LanguageText } from '../../components/AnalystComponents.jsx'
import { OUTCOME_LABEL, outcomeHeadline, sortOutcomes, tallyOutcomes } from '../../lib/acrossCases.js'
import { formatNumber } from '../../lib/format.js'

const ICON = { answered: CircleCheckBig, partial: CircleCheckBig, clarify: CircleHelp, processing: Clock3, 'zero-result': SearchX, unsupported: CircleSlash, failed: CircleAlert }
const OPEN_FIRST = 3

export const caseQuestionLink = (caseId, question) => `/cases/${encodeURIComponent(caseId)}/investigate?question=${encodeURIComponent(question)}`

// The results of one question asked of every case. The headline says how many cases had something; the chips are a count
// of each outcome and jump to those cases; each case keeps its own verified result, best first, so an answer is never
// blended with another case's. Cases that answered open at once, the rest are one line until asked. A case that could not
// be searched says so and can be retried alone. Coverage is the case's own count of ready sources.
export function AcrossResults({ run, coverage, onRetryCase, panelCase = null, onEvidence = () => {} }) {
  const navigate = useNavigate()
  const [copied, setCopied] = useState(null)
  async function copy(caseId, text) {
    try {
      await globalThis.navigator.clipboard.writeText(text)
      setCopied(caseId)
      globalThis.setTimeout(() => setCopied(current => (current === caseId ? null : current)), 1600)
    } catch { setCopied(null) }
  }
  const outcomes = sortOutcomes(run.outcomes)
  const tally = tallyOutcomes(outcomes)
  const done = run.outcomes.length
  const total = run.caseIds.length
  const pending = run.busy ? run.caseIds.filter(id => !run.outcomes.some(outcome => outcome.caseId === id)) : []
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
                    compact
                    onAsk={question => navigate(caseQuestionLink(caseId, question))}
                    onClarificationChoice={option => (option?.query ? navigate(caseQuestionLink(caseId, option.query)) : undefined)}
                    onRetry={() => onRetryCase(caseId)}
                    showOriginalQuestion={false}
                  />
                  <div className="ch-actions ax-case__actions" role="group" aria-label={`Actions for ${caseId}`}>
                    {!['clarify', 'unsupported', 'failed'].includes(presentation.state) ? <button type="button" className={`ch-action${panelCase === caseId ? ' is-on' : ''}`} aria-pressed={panelCase === caseId} onClick={() => onEvidence(panelCase === caseId ? null : caseId)}><FileSearch aria-hidden="true" />Evidence{(presentation.citations.groups || []).length ? <span className="ch-btn__count">{(presentation.citations.groups || []).length}</span> : null}</button> : null}
                    {['answered', 'partial', 'zero-result'].includes(presentation.state) ? <button type="button" className="ch-action" onClick={() => copy(caseId, presentation.answer || '')}>{copied === caseId ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}{copied === caseId ? 'Copied' : 'Copy'}</button> : null}
                    <a className="ch-action ax-case__open" href={link} onClick={event => { event.preventDefault(); navigate(link) }}><ExternalLink aria-hidden="true" />Continue in this case</a>
                  </div>
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
