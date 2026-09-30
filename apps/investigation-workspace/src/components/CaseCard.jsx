import { Link } from 'react-router-dom'
import { LanguageText } from './AnalystComponents.jsx'
import { formatNumber } from '../lib/format.js'
import { curatedFamilyLabel } from '../lib/semanticCatalog.js'
import { summariseCase, useCaseOverview } from '../lib/useCaseOverview.js'

const STATE_LABEL = {
  complete: 'Ready',
  processing: 'Processing',
  attention: 'Needs review',
  'not-processed': 'No evidence yet',
}

function caseOrientation(summary) {
  if (summary.failed > 0) return `${formatNumber(summary.failed)} evidence item${summary.failed === 1 ? '' : 's'} failed processing and ${summary.failed === 1 ? 'needs' : 'need'} review.`
  if (summary.missingAssets > 0) return `${formatNumber(summary.missingAssets)} completed ingest job${summary.missingAssets === 1 ? '' : 's'} ${summary.missingAssets === 1 ? 'has' : 'have'} no retained asset.`
  if (summary.inFlight > 0) return `${formatNumber(summary.inFlight)} evidence item${summary.inFlight === 1 ? '' : 's'} still processing and excluded from complete analysis.`
  if (summary.total === 0) return 'No evidence has been processed for this collection.'
  return 'All reported evidence is ready for analysis.'
}

function nextAction(summary, caseId) {
  if (summary.processingState === 'attention') return { label: 'Review evidence', to: `/cases/${encodeURIComponent(caseId)}/evidence` }
  if (summary.ready > 0) return { label: 'Investigate', to: `/cases/${encodeURIComponent(caseId)}/investigate` }
  if (summary.inFlight > 0) return { label: 'View processing', to: `/cases/${encodeURIComponent(caseId)}/evidence` }
  return { label: 'Add evidence', to: `/cases/${encodeURIComponent(caseId)}/evidence` }
}

// A case card states what is actually in the case. The previous card carried
// the collection identifier and the words "Open overview", which told an
// analyst choosing between cases nothing at all.
//
// It loads its own status. If that fails the card still renders and SAYS it
// could not read the status -- it never shows a zero, because a zero here reads
// as "this case is empty" and that is a claim about evidence.
export function CaseCard({ caseId, headingLevel: Heading = 'h3' }) {
  const state = useCaseOverview(caseId)
  const summary = state.data ? summariseCase(state.data) : null
  const primaryAction = summary ? nextAction(summary, caseId) : null
  const families = summary ? summary.families.slice(0, 4) : []
  const overflow = summary ? Math.max(0, summary.families.length - families.length) : 0

  return (
    <article className={`case-card case-card--${summary ? summary.processingState : 'unknown'}`}>
      <div className="case-card__head">
        <Heading className="case-card__title">
          <Link to={`/cases/${encodeURIComponent(caseId)}/overview`}>
            <LanguageText as="bdi" identifier>{caseId}</LanguageText>
          </Link>
        </Heading>
        {summary && (
          <p className={`case-card__state case-card__state--${summary.processingState}`}>
            <span aria-hidden="true" className="case-card__dot" />
            {STATE_LABEL[summary.processingState]}
          </p>
        )}
      </div>

      {state.loading && <p className="case-card__pending" role="status">Reading case status…</p>}

      {state.error && (
        <p className="case-card__pending case-card__pending--error">
          {state.error.status === 403
            ? 'This case status is not available to you.'
            : 'Case status could not be read. The case can still be opened.'}
        </p>
      )}

      {summary && (
        <>
          <dl className="case-card__metrics">
            <div>
              <dt>Evidence</dt>
              <dd>{formatNumber(summary.total)}</dd>
            </div>
            <div>
              <dt>Ready</dt>
              <dd>{formatNumber(summary.ready)}</dd>
            </div>
            <div>
              <dt>Processing</dt>
              <dd>{formatNumber(summary.inFlight)}</dd>
            </div>
            {summary.failed !== null ? <div><dt>Failed</dt><dd>{formatNumber(summary.failed)}</dd></div> : null}
          </dl>

          <p className="case-card__note">{caseOrientation(summary)}</p>

          {summary.acceptedRows > 0 ? <p className="case-card__rows"><strong>{formatNumber(summary.acceptedRows)}</strong> accepted structured rows</p> : null}

          {families.length > 0 && (
            <ul className="case-card__families" aria-label="Evidence families">
              {families.map(family => (
                <li key={family.record_type}>
                  {curatedFamilyLabel(family.record_type)}
                  <span>{formatNumber(family.accepted_rows)}</span>
                </li>
              ))}
              {overflow > 0 && <li className="case-card__families-more">+{overflow} more</li>}
            </ul>
          )}
        </>
      )}

      <div className="case-card__actions">
        {primaryAction ? <Link className="case-card__primary" to={primaryAction.to}>{primaryAction.label}</Link> : null}
        <Link to={`/cases/${encodeURIComponent(caseId)}/overview`}>Overview</Link>
      </div>
    </article>
  )
}
