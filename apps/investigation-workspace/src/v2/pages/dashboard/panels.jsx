import { useMemo } from 'react'
import { Link } from 'react-router-dom'
import { ArrowRight } from 'lucide-react'
import { formatNumber } from '../../../lib/format.js'
import { formatRate } from '../../../lib/dashboardCharts.js'
import { dashboardNextAction, relativeAge } from '../../../lib/dashboardCases.js'
import { useCaseRecords } from '../../../lib/useCaseRecords.js'
import { CaseLabel } from '../../shell/CaseLabel.jsx'
import { PanelSkeleton } from '../../ui/Panel.jsx'
import { formatMoment } from './model.js'

export const WEIGHT = { attention: 0, processing: 1, 'not-processed': 2, unavailable: 3, forbidden: 3, loading: 4, complete: 5 }
export const STATUS = {
  attention: { label: 'Needs review', tone: 'danger' },
  processing: { label: 'Processing', tone: 'warn' },
  complete: { label: 'Ready', tone: 'ok' },
  'not-processed': { label: 'No evidence', tone: '' },
  loading: { label: 'Checking', tone: '' },
  unavailable: { label: 'Unavailable', tone: 'warn' },
  forbidden: { label: 'Restricted', tone: 'warn' },
}

// A case an analyst put on hold or closed says so beside its name; an open case says nothing (that is the usual state).
const RECORD_STATE = { on_hold: 'On hold', closed: 'Closed' }
export function CaseStateChip({ caseId }) {
  const { records } = useCaseRecords()
  const label = RECORD_STATE[String(records[caseId]?.status || '').toLowerCase().replace(/[\s-]+/g, '_')]
  return label ? <span className="nx-badge nx-case-state">{label}</span> : null
}

// ---------- the cases ----------
export function SourcesCell({ summary }) {
  if (!summary) return <span className="nx-muted">—</span>
  const failed = summary.failed || 0
  const other = Math.max(0, summary.total - summary.ready - summary.inFlight - failed)
  return (
    <span className="nx-sources">
      <span className="nx-stack nx-stack--thin" aria-hidden="true">
        {summary.ready > 0 ? <i className="nx-stack__seg nx-stack__seg--ready" style={{ flexGrow: summary.ready }} /> : null}
        {summary.inFlight > 0 ? <i className="nx-stack__seg nx-stack__seg--processing" style={{ flexGrow: summary.inFlight }} /> : null}
        {failed > 0 ? <i className="nx-stack__seg nx-stack__seg--failed" style={{ flexGrow: failed }} /> : null}
        {other > 0 ? <i className="nx-stack__seg nx-stack__seg--other" style={{ flexGrow: other }} /> : null}
      </span>
      <span className="nx-tabular">{formatNumber(summary.ready)}<span className="nx-muted">/{formatNumber(summary.total)}</span></span>
    </span>
  )
}

// The share of the rows read that were not kept (exact repeats and rows the reader could not accept).
function setAsideOf(summary) {
  const read = summary.acceptedRows + summary.duplicateRows + summary.rejectedRows
  return read ? (summary.duplicateRows + summary.rejectedRows) / read : null
}

// `ordered` keeps the caller's order (the Cases page sorts by the analyst's choice); otherwise the ones that need you come first.
export function CasesPanel({ rows, limit = 8, headless = false, ordered: keepOrder = false, loading }) {
  const ordered = useMemo(() => (keepOrder ? rows : rows.slice().sort((left, right) => (WEIGHT[left.status] ?? 9) - (WEIGHT[right.status] ?? 9) || left.index - right.index)), [rows, keepOrder])
  const shown = ordered.slice(0, limit)
  const mostRecords = Math.max(1, ...shown.map(row => row.summary?.acceptedRows || 0))
  return (
    <section className={`nx-panel nx-cases${headless ? ' nx-panel--flat' : ''}`} aria-labelledby={headless ? undefined : 'dash-cases'} aria-label={headless ? 'Cases' : undefined}>
      {headless ? null : (
        <header className="nx-panel__head">
          <h2 id="dash-cases" className="nx-panel__title">Cases</h2>
          <Link className="nx-panel__link" to="/cases">All {formatNumber(rows.length)}<ArrowRight aria-hidden="true" /></Link>
        </header>
      )}
      <div className="nx-panel__body nx-panel__body--flush">
        {loading && !rows.length ? <PanelSkeleton rows={2} /> : (
          <div className="nx-table-wrap nx-table-wrap--bare">
            <table className="nx-table">
              <caption className="nx-visually-hidden">Cases, the ones that need you first</caption>
              <thead><tr><th scope="col">Case</th><th scope="col">Status</th><th scope="col">Files ready</th><th scope="col" className="is-num">Records</th><th scope="col" className="is-num nx-col-aside">Set aside</th><th scope="col" className="nx-col-last">Updated</th><th scope="col"><span className="nx-visually-hidden">Next</span></th></tr></thead>
              <tbody>
                {shown.map(row => {
                  const status = STATUS[row.status] || STATUS.loading
                  const next = row.summary ? dashboardNextAction(row.summary, row.caseId) : null
                  const aside = row.summary ? setAsideOf(row.summary) : null
                  return (
                    <tr key={row.caseId}>
                      <td className="is-wrap nx-pin"><Link className="nx-case-link" to={`/cases/${encodeURIComponent(row.caseId)}/overview`} title={row.caseId}><CaseLabel caseId={row.caseId} /></Link><CaseStateChip caseId={row.caseId} /></td>
                      <td><span className={`nx-badge${status.tone ? ` nx-badge--${status.tone}` : ''}`}>{status.label}</span></td>
                      <td><SourcesCell summary={row.summary} /></td>
                      <td className="is-num nx-tabular nx-cell-bar" style={row.summary ? { '--w': row.summary.acceptedRows / mostRecords } : undefined}>{row.summary ? formatNumber(row.summary.acceptedRows) : '—'}</td>
                      <td className="is-num nx-tabular nx-col-aside">{aside === null ? '—' : formatRate(aside)}</td>
                      <td className="nx-col-last nx-secondary" title={row.activity ? formatMoment(row.activity.date) : undefined}>{row.activity ? relativeAge(row.activity.date) : '—'}</td>
                      <td className="is-num">{next ? <Link className="nx-btn nx-btn--sm" to={next.to}>{next.label}</Link> : null}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </section>
  )
}
