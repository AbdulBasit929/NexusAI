import { useState } from 'react'
import { Link } from 'react-router-dom'
import { ArrowRight, CircleAlert, CircleCheckBig, FileX2 } from 'lucide-react'
import { Card } from '../../components/Card.jsx'
import { LanguageText } from '../../components/AnalystComponents.jsx'
import { SkeletonRows } from '../../components/Skeleton.jsx'
import { formatNumber } from '../../lib/format.js'

const VISIBLE = 6

export function attentionReason(item) {
  if (item.detail) return item.detail
  if (item.kind === 'Failed') return 'Processing did not complete.'
  return 'Processing finished, but the retained copy of this source is missing.'
}

export function reviewTarget(item) {
  const base = `/cases/${encodeURIComponent(item.caseId)}/evidence`
  if (item.evidenceId) return `${base}/${encodeURIComponent(item.evidenceId)}`
  return item.kind === 'Failed' ? `${base}?status=failed` : base
}

export function caseFailures(rows) {
  return rows.filter(row => row.summary?.failed > 0).map(row => ({ caseId: row.caseId, failed: row.summary.failed }))
}

// D3. Which sources need review before I rely on the results? A queue of named things (not a chart): what is wrong,
// why, where, and one click to review. Worst first. The footer is honest about completeness: until the service offers
// a complete feed (backend request 13) each case only lists its most recent items, and the page says so.
export function AttentionQueue({ items, kpis, failures, loading }) {
  const [showAll, setShowAll] = useState(false)
  const listedFailed = items.filter(item => item.kind === 'Failed').length
  const shown = showAll ? items : items.slice(0, VISIBLE)
  const incomplete = kpis.failed > listedFailed
  return (
    <div id="dashboard-attention" className="dash-slot">
      <Card
        className="attention-card"
        title="Which sources need review before I rely on the results?"
        description="Failed sources and completed jobs whose retained copy is missing, across every case."
        actions={items.length ? <span className="attention-card__count">{formatNumber(items.length)} named</span> : null}
        footer={incomplete ? (
          <>
            <span>{formatNumber(kpis.failed)} failed sources are reported in total; {formatNumber(listedFailed)} {listedFailed === 1 ? 'is' : 'are'} named here because each case lists only its most recent items.</span>
            <span className="attention-card__links">{failures.map(entry => <Link key={entry.caseId} to={`/cases/${encodeURIComponent(entry.caseId)}/evidence?status=failed`}>All {formatNumber(entry.failed)} failed in <LanguageText as="bdi" identifier>{entry.caseId}</LanguageText></Link>)}</span>
          </>
        ) : <span>Based on the most recent items each case reports.</span>}
      >
        {loading && !items.length ? <SkeletonRows rows={3} label="Reading the sources that need review" /> : null}
        {!loading && !items.length ? <p className="dash-card__empty"><CircleCheckBig aria-hidden="true" />No failed sources or missing retained copies were reported in the latest status.</p> : null}
        {items.length ? (
          <ul className="attention-list">
            {shown.map(item => {
              const failed = item.kind === 'Failed'
              const Icon = failed ? CircleAlert : FileX2
              return (
                <li key={`${item.caseId}-${item.evidenceId || item.label}-${item.kind}`} className="attention-row">
                  <span className="attention-row__mark" aria-hidden="true"><Icon /></span>
                  <div className="attention-row__body">
                    <span className="attention-row__problem">{item.kind}</span>
                    <LanguageText className="attention-row__source">{item.label}</LanguageText>
                    <span className="attention-row__reason"><LanguageText>{attentionReason(item)}</LanguageText></span>
                    <span className="attention-row__case">In <Link to={`/cases/${encodeURIComponent(item.caseId)}/overview`}><LanguageText as="bdi" identifier>{item.caseId}</LanguageText></Link></span>
                  </div>
                  <Link className="attention-row__review" to={reviewTarget(item)} aria-label={`Review ${item.label}`}>Review<ArrowRight aria-hidden="true" /></Link>
                </li>
              )
            })}
          </ul>
        ) : null}
        {items.length > VISIBLE ? <button type="button" className="dash-more" aria-expanded={showAll} onClick={() => setShowAll(value => !value)}>{showAll ? 'Show fewer' : `Show all ${formatNumber(items.length)}`}</button> : null}
      </Card>
    </div>
  )
}
