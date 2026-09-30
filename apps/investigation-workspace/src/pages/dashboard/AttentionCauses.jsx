import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { ArrowRight, ChevronDown, CircleCheckBig } from 'lucide-react'
import { Card } from '../../components/Card.jsx'
import { LanguageText } from '../../components/AnalystComponents.jsx'
import { SkeletonRows } from '../../components/Skeleton.jsx'
import { attentionCauses } from '../../lib/attentionCauses.js'
import { reviewKind } from '../../lib/dashboardCharts.js'
import { formatNumber } from '../../lib/format.js'

const ITEMS_SHOWN = 4
const CAUSES_SHOWN = 5

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

// Each cause gets a shade of its kind's colour (red for failed, amber for a missing copy), darkest for the largest, so
// the bar and the list read as one legend without relying on hue alone: the list repeats every label and count.
function shadeFor(causes) {
  const seen = { failed: 0, missing: 0 }
  return Object.fromEntries(causes.map(cause => {
    const index = seen[cause.kind]++
    return [cause.key, Math.max(38, 100 - index * 22)]
  }))
}

// D3 in the hero, as a triage card. The number is how many sources need attention; the bar shows how that splits by
// cause and the list names each cause, so the analyst fixes a cause once rather than reading source by source. Hovering
// a segment or a row lights the other. Counts are reconciled to the complete per-case totals; the named sources are only
// the most recent each case reports, and the card says so. Rules: UI_REDESIGN_BRIEF §6 (part of a whole) and §11.
export function AttentionCauses({ items, byCase, kpis, loading }) {
  const causes = useMemo(() => attentionCauses(items, byCase), [items, byCase])
  const [open, setOpen] = useState(null)
  const [more, setMore] = useState('')
  const [hot, setHot] = useState('')
  const [allCauses, setAllCauses] = useState(false)
  const total = byCase.reduce((sum, entry) => sum + entry.total, 0)
  const counted = causes.reduce((sum, cause) => sum + cause.count, 0)
  const failed = causes.filter(cause => cause.kind === 'failed').reduce((sum, cause) => sum + cause.count, 0)
  const missing = counted - failed
  const shades = useMemo(() => shadeFor(causes), [causes])
  const openKey = open === null ? causes[0]?.key : open
  const unavailable = !loading && kpis.cases > 0 && kpis.reported === 0
  const failedLinks = unnamedFailures(byCase, items)
  const shown = allCauses ? causes : causes.slice(0, CAUSES_SHOWN)
  const manyCases = byCase.length > 1

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
        <div className="tri-hero">
          <b className="tri-total">{formatNumber(counted)}</b>
          <span className="tri-hero__text">
            <strong>{counted === 1 ? 'source needs' : 'sources need'} review</strong>
            <small>{[failed ? `${formatNumber(failed)} failed` : '', missing ? `${formatNumber(missing)} missing a copy` : ''].filter(Boolean).join(' · ')}{total ? ` · of ${formatNumber(total)}` : ''}</small>
          </span>
        </div>
        <div className={`tri-bar${hot ? ' has-hot' : ''}`} aria-hidden="true">
          {causes.map(cause => (
            <i
              key={cause.key}
              className={`tri-seg tri-seg--${cause.kind}${hot === cause.key ? ' is-hot' : ''}`}
              style={{ flexGrow: cause.count, '--shade': `${shades[cause.key]}%` }}
              onMouseEnter={() => setHot(cause.key)}
              onMouseLeave={() => setHot('')}
              onClick={() => { setOpen(cause.key); setHot(cause.key) }}
            />
          ))}
        </div>
        <p className="ac-summary">{formatNumber(causes.length)} {causes.length === 1 ? 'cause' : 'causes'} across {formatNumber(total)} {total === 1 ? 'source' : 'sources'}</p>
        <ul className={`tri-list${hot ? ' has-hot' : ''}`}>
          {shown.map(cause => {
            const isOpen = openKey === cause.key
            const panelId = `tri-panel-${causes.indexOf(cause)}`
            return (
              <li
                key={cause.key}
                className={`tri-row tri-row--${cause.kind}${isOpen ? ' is-open' : ''}${hot === cause.key ? ' is-hot' : ''}`}
                style={{ '--shade': `${shades[cause.key]}%` }}
                onMouseEnter={() => setHot(cause.key)}
                onMouseLeave={() => setHot('')}
              >
                <button type="button" className="tri-head" aria-expanded={isOpen} aria-controls={panelId} onFocus={() => setHot(cause.key)} onBlur={() => setHot('')} onClick={() => setOpen(isOpen ? '' : cause.key)}>
                  <span className="tri-dot" aria-hidden="true" />
                  <span className="tri-label">{cause.label}</span>
                  {manyCases ? <span className="tri-cases">{formatNumber(cause.caseIds.length)} {cause.caseIds.length === 1 ? 'case' : 'cases'}</span> : null}
                  <span className="tri-count">{formatNumber(cause.count)}</span>
                  <ChevronDown className="ac-chev" aria-hidden="true" />
                </button>
                {isOpen ? (
                  <div id={panelId} className="tri-body">
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
        {causes.length > CAUSES_SHOWN ? <button type="button" className="ac-showall tri-morecauses" aria-expanded={allCauses} onClick={() => setAllCauses(value => !value)}>{allCauses ? 'Show fewer causes' : `Show ${formatNumber(causes.length - CAUSES_SHOWN)} more ${causes.length - CAUSES_SHOWN === 1 ? 'cause' : 'causes'}`}</button> : null}
      </>
    )
  }

  return (
    <div id="dashboard-attention" className="dash-slot">
      <Card className="needs-review triage-card" title="What needs review?">
        {body}
      </Card>
    </div>
  )
}
