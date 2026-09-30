import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { ArrowRight, ChevronDown, CircleAlert, CircleCheckBig, FileX2 } from 'lucide-react'
import { Card } from '../../components/Card.jsx'
import { LanguageText } from '../../components/AnalystComponents.jsx'
import { SkeletonRows } from '../../components/Skeleton.jsx'
import { attentionCauses } from '../../lib/attentionCauses.js'
import { reviewKind } from '../../lib/dashboardCharts.js'
import { formatNumber } from '../../lib/format.js'

const ICONS = { failed: CircleAlert, missing: FileX2 }
const ITEMS_SHOWN = 4

export function attentionReason(item) {
  return item.detail || (reviewKind(item) === 'failed' ? 'Did not finish processing.' : 'Retained copy is missing.')
}

export function reviewTarget(item) {
  const base = `/cases/${encodeURIComponent(item.caseId)}/evidence`
  if (item.evidenceId) return `${base}/${encodeURIComponent(item.evidenceId)}`
  return reviewKind(item) === 'failed' ? `${base}?status=failed` : base
}

// Failed sources a case reports in its summary (complete) that its recent items do not name, for the full-list links.
export function unnamedFailures(byCase, items) {
  return byCase
    .map(entry => ({ caseId: entry.caseId, failed: entry.failed, unnamed: entry.failed - items.filter(item => item.caseId === entry.caseId && reviewKind(item) === 'failed').length }))
    .filter(entry => entry.unnamed > 0)
}

// D3 in the hero. What needs review, and why? Findings are grouped by cause, so the same problem on several sources is
// one bar: the analyst sees how many distinct problems there are and how big each is, and can open the sources behind a
// cause in place. Counts are reconciled to the complete per-case totals; the named sources are only the most recent each
// case reports, and the card says so. Rules: UI_REDESIGN_BRIEF §6 (ranked comparison) and §11.
export function AttentionCauses({ items, byCase, kpis, loading }) {
  const causes = useMemo(() => attentionCauses(items, byCase), [items, byCase])
  const [open, setOpen] = useState(null)
  const [more, setMore] = useState('')
  const total = byCase.reduce((sum, entry) => sum + entry.total, 0)
  const largest = Math.max(1, ...causes.map(cause => cause.count))
  const openKey = open === null ? causes[0]?.key : open
  const unavailable = !loading && kpis.cases > 0 && kpis.reported === 0
  const failedLinks = unnamedFailures(byCase, items)

  let body
  if (loading && !causes.length) {
    body = <SkeletonRows rows={3} label="Reading what needs review" />
  } else if (!causes.length) {
    body = (
      <div className="nr-clear">
        <span className="nr-clear__chip"><CircleCheckBig aria-hidden="true" /></span>
        <div>
          <strong>{unavailable ? 'Status unavailable' : 'Nothing needs review'}</strong>
          <span>{unavailable ? 'Nothing can be listed until the status can be read.' : kpis.sources ? `${formatNumber(kpis.ready)} of ${formatNumber(kpis.sources)} sources are ready.` : 'Add evidence to begin.'}</span>
        </div>
      </div>
    )
  } else {
    body = (
      <>
        <p className="ac-summary">{formatNumber(causes.length)} {causes.length === 1 ? 'cause' : 'causes'} across {formatNumber(total)} {total === 1 ? 'source' : 'sources'}</p>
        <ul className="ac-list">
          {causes.map((cause, index) => {
            const Icon = ICONS[cause.kind]
            const isOpen = openKey === cause.key
            const panelId = `ac-panel-${index}`
            return (
              <li key={cause.key} className={`ac-row ac-row--${cause.kind}${isOpen ? ' is-open' : ''}`}>
                <button type="button" className="ac-head" aria-expanded={isOpen} aria-controls={panelId} onClick={() => setOpen(isOpen ? '' : cause.key)}>
                  <span className="ac-mark" aria-hidden="true"><Icon /></span>
                  <span className="ac-label">{cause.label}</span>
                  <span className="ac-count">{formatNumber(cause.count)}</span>
                  <ChevronDown className="ac-chev" aria-hidden="true" />
                </button>
                <span className="ac-bar" aria-hidden="true"><i style={{ inlineSize: `${(cause.count / largest) * 100}%` }} /></span>
                {isOpen ? (
                  <div id={panelId} className="ac-body">
                    {cause.items.length ? (
                      <>
                        <ul className="ac-items">
                          {(more === cause.key ? cause.items : cause.items.slice(0, ITEMS_SHOWN)).map(item => (
                            <li key={`${item.caseId}-${item.evidenceId || item.label}`}>
                              <Link to={reviewTarget(item)} aria-label={`Review ${item.label}`}>
                                <span className="ac-item__text">
                                  <span className="ac-item__name"><LanguageText>{item.label}</LanguageText></span>
                                  <span className="ac-item__case"><LanguageText as="bdi" identifier>{item.caseId}</LanguageText></span>
                                </span>
                                <ArrowRight aria-hidden="true" />
                              </Link>
                            </li>
                          ))}
                        </ul>
                        {cause.items.length > ITEMS_SHOWN ? <button type="button" className="ac-showall" aria-expanded={more === cause.key} onClick={() => setMore(current => (current === cause.key ? '' : cause.key))}>{more === cause.key ? 'Show fewer' : `Show all ${formatNumber(cause.items.length)}`}</button> : null}
                      </>
                    ) : null}
                    {cause.extra > 0 ? (
                      <p className="ac-more">
                        {cause.items.length ? `+${formatNumber(cause.extra)} more not named here.` : `${formatNumber(cause.extra)} not named here.`}
                        {cause.kind === 'failed' ? failedLinks.filter(entry => cause.caseIds.includes(entry.caseId)).map(entry => <Link key={entry.caseId} to={`/cases/${encodeURIComponent(entry.caseId)}/evidence?status=failed`}>All {formatNumber(entry.failed)} failed in <LanguageText as="bdi" identifier>{entry.caseId}</LanguageText></Link>) : cause.caseIds.map(caseId => <Link key={caseId} to={`/cases/${encodeURIComponent(caseId)}/evidence`}>Open <LanguageText as="bdi" identifier>{caseId}</LanguageText></Link>)}
                      </p>
                    ) : null}
                  </div>
                ) : null}
              </li>
            )
          })}
        </ul>
      </>
    )
  }

  return (
    <div id="dashboard-attention" className="dash-slot">
      <Card className="needs-review" title="What needs review?" actions={total ? <span className="attention-card__count">{formatNumber(total)} {total === 1 ? 'source' : 'sources'}</span> : null}>
        {body}
      </Card>
    </div>
  )
}
