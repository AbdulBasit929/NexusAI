import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { ArrowRight, Search, X } from 'lucide-react'
import { LanguageText } from '../../components/AnalystComponents.jsx'
import { ProportionBar } from '../../components/DataVisualizations.jsx'
import { SkeletonRows } from '../../components/Skeleton.jsx'
import { dashboardNextAction, relativeAge } from '../../lib/dashboardCases.js'
import { formatNumber } from '../../lib/format.js'

const STATE_LABEL = { complete: 'Ready', processing: 'Processing', attention: 'Needs review', 'not-processed': 'No evidence', unavailable: 'Unavailable', loading: 'Checking' }
const SORT_WEIGHT = { attention: 0, processing: 1, 'not-processed': 2, complete: 3, unavailable: 4, loading: 5 }
const FILTERS = [['all', 'All'], ['attention', 'Needs review'], ['processing', 'Processing'], ['complete', 'Ready']]
const SHOWN = 8
const SEARCH_FROM = 7

function Trend({ spark }) {
  if (!spark) return <span className="ct-muted" title="Activity was not read for this case">Not read</span>
  const peak = Math.max(1, ...spark.values)
  const width = 90 / spark.values.length
  return (
    <span className="ct-trend">
      <svg viewBox="0 0 90 24" preserveAspectRatio="none" role="img" aria-label={`${formatNumber(spark.total)} events in the 30 days to ${spark.to}`}>
        {spark.values.map((value, index) => <rect key={index} x={index * width + 0.3} y={24 - (value ? Math.max(2, (value / peak) * 24) : 0.8)} width={Math.max(0.6, width - 0.6)} height={value ? Math.max(2, (value / peak) * 24) : 0.8} rx="0.6" className={value ? 'ct-bar' : 'ct-bar ct-bar--zero'} />)}
      </svg>
      <small>{formatNumber(spark.total)}</small>
    </span>
  )
}

// One line per case, in the order an analyst should look: needs review, processing, then ready. Each row answers the
// four questions asked of a case: is its evidence usable (readiness), does it need me (review), did ingestion keep the
// rows (rows kept), and is it alive (trend, latest update). Figures are the service's summary counts; the trend is the
// same real activity as the hero chart, so a case that was not read says "Not read" instead of drawing a flat line.
export function CasesTable({ rows, sparks, familyLabel, onClearFamily, loading, now }) {
  const [filter, setFilter] = useState('all')
  const [query, setQuery] = useState('')
  const [all, setAll] = useState(false)
  const counts = useMemo(() => Object.fromEntries(FILTERS.map(([id]) => [id, id === 'all' ? rows.length : rows.filter(row => row.status === id).length])), [rows])
  const visible = useMemo(() => rows
    .filter(row => filter === 'all' || row.status === filter)
    .filter(row => row.caseId.toLocaleLowerCase().includes(query.trim().toLocaleLowerCase()))
    .sort((left, right) => SORT_WEIGHT[left.status] - SORT_WEIGHT[right.status] || left.index - right.index), [rows, filter, query])
  const shown = all ? visible : visible.slice(0, SHOWN)

  return (
    <section id="dashboard-cases" className="cases-card" aria-labelledby="dashboard-cases-title">
      <header className="cases-card__header">
        <div><h2 id="dashboard-cases-title">Which case needs me next?</h2><p>Sorted by need: review first, then processing, then ready.</p></div>
        <div className="cases-card__tools">
          <div className="seg" role="group" aria-label="Filter cases">
            {FILTERS.map(([id, label]) => <button key={id} type="button" aria-pressed={filter === id} onClick={() => setFilter(id)}>{label}<span className="seg__count">{formatNumber(counts[id])}</span></button>)}
          </div>
          {rows.length >= SEARCH_FROM ? <label className="cases-card__search"><Search aria-hidden="true" /><span className="visually-hidden">Find a case</span><input type="search" value={query} onChange={event => setQuery(event.target.value)} placeholder="Find a case" /></label> : null}
        </div>
      </header>
      {familyLabel ? <p className="em-filter"><span>Showing only cases with <b>{familyLabel}</b></span><button type="button" onClick={onClearFamily}><X aria-hidden="true" />Clear<span className="visually-hidden"> filter: {familyLabel}</span></button></p> : null}

      {loading && !rows.some(row => row.summary) ? <SkeletonRows rows={3} label="Reading cases" /> : visible.length ? (
        <div className="ct-scroll">
          <table className="ct">
            <thead>
              <tr>
                <th scope="col">Case</th>
                <th scope="col">Evidence ready</th>
                <th scope="col" className="is-numeric">Needs review</th>
                <th scope="col" className="is-numeric">Rows kept</th>
                <th scope="col">Activity, 30 days</th>
                <th scope="col">Updated</th>
                <th scope="col"><span className="visually-hidden">Next step</span></th>
              </tr>
            </thead>
            <tbody>
              {shown.map(({ caseId, status, summary, activity, state }) => {
                const review = summary ? summary.failed + summary.missingAssets : null
                const action = summary ? dashboardNextAction(summary, caseId) : { label: 'Open case', to: `/cases/${encodeURIComponent(caseId)}/overview` }
                const issues = summary ? [summary.rejectedRows ? `${formatNumber(summary.rejectedRows)} rejected` : '', summary.duplicateRows ? `${formatNumber(summary.duplicateRows)} duplicate` : ''].filter(Boolean).join(' · ') : ''
                return (
                  <tr key={caseId} className={`ct-row ct-row--${status}`}>
                    <th scope="row">
                      <Link to={`/cases/${encodeURIComponent(caseId)}/overview`}><LanguageText as="bdi" identifier>{caseId}</LanguageText></Link>
                      <span className={`ct-state ct-state--${status}`}><i aria-hidden="true" />{STATE_LABEL[status]}</span>
                    </th>
                    <td>
                      {summary ? (
                        <span className="ct-ready">
                          <ProportionBar total={summary.total} ready={summary.ready} processing={summary.inFlight} failed={summary.failed || 0} label={`${caseId} evidence readiness`} />
                          <small>{formatNumber(summary.ready)} of {formatNumber(summary.total)}</small>
                        </span>
                      ) : <span className="ct-muted">{state?.loading ? 'Reading…' : state?.error?.status === 403 ? 'Outside your access' : 'Could not be read'}</span>}
                    </td>
                    <td className="is-numeric">
                      {review === null ? <span className="ct-muted">—</span> : review ? <Link className="ct-review" to={`/cases/${encodeURIComponent(caseId)}/evidence${summary.failed ? '?status=failed' : ''}`}>{formatNumber(review)}</Link> : <span className="ct-muted">None</span>}
                    </td>
                    <td className="is-numeric">
                      {summary ? <><b>{formatNumber(summary.acceptedRows)}</b>{issues ? <small>{issues}</small> : null}</> : <span className="ct-muted">—</span>}
                    </td>
                    <td><Trend spark={sparks[caseId]} /></td>
                    <td>{activity ? <time dateTime={activity.date.toISOString()} title={activity.date.toISOString()}>{relativeAge(activity.date, now)}</time> : <span className="ct-muted">—</span>}</td>
                    <td className="ct-action"><Link to={action.to} aria-label={`${action.label}: ${caseId}`} className={status === 'attention' ? 'is-primary' : undefined}>{action.label}<ArrowRight aria-hidden="true" /></Link></td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      ) : (
        <p className="dash-card__empty">No cases match this view. <button type="button" className="dash-link-button" onClick={() => { setFilter('all'); setQuery(''); onClearFamily?.() }}>Clear view</button></p>
      )}
      {visible.length > SHOWN ? <button type="button" className="ac-showall" aria-expanded={all} onClick={() => setAll(value => !value)}>{all ? 'Show fewer cases' : `Show all ${formatNumber(visible.length)} cases`}</button> : null}
    </section>
  )
}
