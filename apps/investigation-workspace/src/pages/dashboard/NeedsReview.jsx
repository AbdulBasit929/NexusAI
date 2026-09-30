import { useState } from 'react'
import { Link } from 'react-router-dom'
import { ArrowRight, CircleAlert, CircleCheckBig, FileX2, X } from 'lucide-react'
import { Card } from '../../components/Card.jsx'
import { LanguageText } from '../../components/AnalystComponents.jsx'
import { SkeletonRows } from '../../components/Skeleton.jsx'
import { itemMatchesReviewFilter, reviewFilterCount, reviewKind } from '../../lib/dashboardCharts.js'
import { formatNumber } from '../../lib/format.js'

const VISIBLE = 5
const NO_FILTER = { caseId: null, kind: null }
const KINDS = [
  { id: 'failed', label: 'Failed', Icon: CircleAlert },
  { id: 'missing', label: 'Missing copy', Icon: FileX2 },
]

export function attentionReason(item) {
  if (item.detail) return item.detail
  return reviewKind(item) === 'failed' ? 'Did not finish processing.' : 'Retained copy is missing.'
}

export function reviewTarget(item) {
  const base = `/cases/${encodeURIComponent(item.caseId)}/evidence`
  if (item.evidenceId) return `${base}/${encodeURIComponent(item.evidenceId)}`
  return reviewKind(item) === 'failed' ? `${base}?status=failed` : base
}

// Failed sources a case reports in its summary (complete) that its recent items do not name.
export function unnamedFailures(byCase, items) {
  return byCase
    .map(entry => ({ caseId: entry.caseId, unnamed: entry.failed - items.filter(item => item.caseId === entry.caseId && reviewKind(item) === 'failed').length, failed: entry.failed }))
    .filter(entry => entry.unnamed > 0)
}

// D3. What needs review, and where? Two halves joined by one filter. Left, the answer to "where": problems ranked by
// case, each bar split into failed (solid) and missing copy (hatched, so colour is never the only signal), exact from
// the complete per-case counts. Right, the answer to "which sources": the named items with a reason and a Review
// link. Clicking a bar segment or a chip filters the list, and the chart never filters itself, so it stays a control.
// The named list is only the most recent items each case reports, so the card says so when it names fewer than the
// counts and links to the full failed list. Rules: UI_REDESIGN_BRIEF §11.
export function NeedsReview({ items, byCase, kpis, loading }) {
  const [filter, setFilter] = useState(NO_FILTER)
  const [showAll, setShowAll] = useState(false)
  const total = byCase.reduce((sum, entry) => sum + entry.total, 0)
  const largest = Math.max(1, ...byCase.map(entry => entry.total))
  const kindTotals = KINDS.map(kind => ({ ...kind, count: byCase.reduce((sum, entry) => sum + entry[kind.id], 0) }))
  const kindsPresent = kindTotals.filter(kind => kind.count > 0)
  const filtered = items.filter(item => itemMatchesReviewFilter(item, filter))
  const shown = showAll ? filtered : filtered.slice(0, VISIBLE)
  const expected = reviewFilterCount(byCase, filter)
  const unnamed = unnamedFailures(byCase, items).filter(entry => (!filter.caseId || entry.caseId === filter.caseId) && filter.kind !== 'missing')
  const filtering = Boolean(filter.caseId || filter.kind)
  const unavailable = !loading && kpis.cases > 0 && kpis.reported === 0

  const pick = (caseId, kind) => {
    setShowAll(false)
    setFilter(current => (current.caseId === caseId && current.kind === kind ? NO_FILTER : { caseId, kind }))
  }
  const setKind = kind => { setShowAll(false); setFilter(current => ({ ...current, kind })) }
  const clear = () => { setShowAll(false); setFilter(NO_FILTER) }

  let body
  if (loading && !byCase.length && !items.length) {
    body = <SkeletonRows rows={3} label="Reading the sources that need review" />
  } else if (!total && !items.length) {
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
      <div className="nr-body">
        <section className="nr-where" aria-label="Problems by case">
          <h3 className="nr-caption">By case</h3>
          <ul className="nr-bars">
            {byCase.map(entry => {
              const dim = Boolean(filter.caseId) && filter.caseId !== entry.caseId
              return (
                <li key={entry.caseId} className={`nr-bar${dim ? ' is-dim' : ''}`}>
                  <div className="nr-bar__head">
                    <Link to={`/cases/${encodeURIComponent(entry.caseId)}/overview`}><LanguageText as="bdi" identifier>{entry.caseId}</LanguageText></Link>
                    <span>{formatNumber(entry.total)}</span>
                  </div>
                  <div className="nr-bar__track" style={{ inlineSize: `${Math.max(18, (entry.total / largest) * 100)}%` }} role="group" aria-label={`Sources to review in ${entry.caseId}`}>
                    {KINDS.filter(kind => entry[kind.id] > 0).map(kind => {
                      const selected = filter.caseId === entry.caseId && filter.kind === kind.id
                      return (
                        <button
                          key={kind.id}
                          type="button"
                          className={`nr-seg nr-seg--${kind.id}${selected ? ' is-selected' : ''}`}
                          style={{ flexGrow: entry[kind.id] }}
                          aria-pressed={selected}
                          aria-label={`${formatNumber(entry[kind.id])} ${kind.label.toLowerCase()} in ${entry.caseId}. Show only these.`}
                          title={`${kind.label}: ${formatNumber(entry[kind.id])}`}
                          onClick={() => pick(entry.caseId, kind.id)}
                        >
                          {entry[kind.id] / entry.total >= 0.3 ? <span aria-hidden="true">{formatNumber(entry[kind.id])}</span> : null}
                        </button>
                      )
                    })}
                  </div>
                </li>
              )
            })}
          </ul>
          <ul className="nr-legend" aria-label="Legend">{KINDS.map(kind => <li key={kind.id}><span className={`nr-swatch nr-swatch--${kind.id}`} aria-hidden="true" />{kind.label}</li>)}</ul>
        </section>

        <section className="nr-what" aria-label="Sources to review">
          {kindsPresent.length > 1 || filter.caseId ? (
            <div className="nr-chips" role="group" aria-label="Filter the list">
              {kindsPresent.length > 1 ? (
                <>
                  <button type="button" className="nr-chip" aria-pressed={!filter.kind} onClick={() => setKind(null)}>All <b>{formatNumber(total)}</b></button>
                  {kindsPresent.map(kind => <button key={kind.id} type="button" className="nr-chip" aria-pressed={filter.kind === kind.id} onClick={() => setKind(kind.id)}>{kind.label} <b>{formatNumber(kind.count)}</b></button>)}
                </>
              ) : null}
              {filter.caseId ? <button type="button" className="nr-chip nr-chip--active" onClick={() => setFilter(current => ({ ...current, caseId: null }))}><bdi>{filter.caseId}</bdi><X aria-hidden="true" /><span className="visually-hidden">Remove this case from the filter</span></button> : null}
              {filtering ? <button type="button" className="nr-clear-all" onClick={clear}>Clear</button> : null}
            </div>
          ) : null}

          {filtered.length ? (
            <ul className="nr-list">
              {shown.map(item => {
                const kind = KINDS.find(entry => entry.id === reviewKind(item))
                return (
                  <li key={`${item.caseId}-${item.evidenceId || item.label}-${item.kind}`} className={`nr-row nr-row--${kind.id}`}>
                    <span className="nr-row__mark" aria-hidden="true"><kind.Icon /></span>
                    <div className="nr-row__body">
                      <LanguageText className="nr-row__source">{item.label}</LanguageText>
                      <span className="nr-row__reason" title={attentionReason(item)}><LanguageText>{attentionReason(item)}</LanguageText></span>
                      <span className="nr-row__meta"><b>{kind.label}</b><span aria-hidden="true">·</span><LanguageText as="bdi" identifier>{item.caseId}</LanguageText></span>
                    </div>
                    <Link className="nr-row__review" to={reviewTarget(item)} aria-label={`Review ${item.label}`}>Review<ArrowRight aria-hidden="true" /></Link>
                  </li>
                )
              })}
            </ul>
          ) : (
            <p className="nr-empty">{expected ? 'None of these are named here yet.' : 'Nothing matches this filter.'}</p>
          )}
          {filtered.length > VISIBLE ? <button type="button" className="dash-more" aria-expanded={showAll} onClick={() => setShowAll(value => !value)}>{showAll ? 'Show fewer' : `Show all ${formatNumber(filtered.length)}`}</button> : null}
          {expected > filtered.length ? (
            <p className="nr-note">
              {formatNumber(expected)} in total; {formatNumber(filtered.length)} named here, the most recent each case reports.
              {unnamed.map(entry => <Link key={entry.caseId} to={`/cases/${encodeURIComponent(entry.caseId)}/evidence?status=failed`}>All {formatNumber(entry.failed)} failed in <LanguageText as="bdi" identifier>{entry.caseId}</LanguageText></Link>)}
            </p>
          ) : null}
        </section>
      </div>
    )
  }

  return (
    <div id="dashboard-attention" className="dash-slot dash-slot--wide">
      <Card className="needs-review" title="What needs review?" actions={total ? <span className="attention-card__count">{formatNumber(total)} {total === 1 ? 'source' : 'sources'}</span> : null}>
        {body}
      </Card>
    </div>
  )
}
