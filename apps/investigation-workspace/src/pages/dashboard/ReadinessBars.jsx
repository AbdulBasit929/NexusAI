import { useState } from 'react'
import { Link } from 'react-router-dom'
import { ChartColumn, Table2 } from 'lucide-react'
import { Card } from '../../components/Card.jsx'
import { LanguageText } from '../../components/AnalystComponents.jsx'
import { SkeletonRows } from '../../components/Skeleton.jsx'
import { evidenceLink } from '../../lib/dashboardCharts.js'
import { formatNumber } from '../../lib/format.js'

const SEGMENTS = [
  { id: 'ready', label: 'Ready' },
  { id: 'processing', label: 'Processing' },
  { id: 'failed', label: 'Failed' },
  { id: 'other', label: 'Not yet counted' },
]

// D4. Is each case's evidence ready to search? A 100% stacked bar per case (composition across groups), worst first.
// Built as links rather than a canvas so every segment is keyboard reachable, exact in any column width, and opens
// that case's Evidence list filtered to the state. Colour is never alone: processing is striped, failed is dotted, and
// counts are printed inside segments wide enough to hold them. Tiny non-zero segments keep a minimum visible width.
export function ReadinessBars({ rows, kpis, loading }) {
  const [view, setView] = useState('chart')
  return (
    <div id="dashboard-readiness" className="dash-slot">
      <Card
        className="readiness-card"
        title="Is each case’s evidence ready to search?"
        description="Share of each case’s sources that are ready, processing or failed. Select a segment to open those sources."
        actions={(
          <div className="chart-card__toggle" role="group" aria-label="View of: Is each case’s evidence ready to search?">
            <button type="button" aria-pressed={view === 'chart'} onClick={() => setView('chart')}><ChartColumn aria-hidden="true" />Chart</button>
            <button type="button" aria-pressed={view === 'table'} onClick={() => setView('table')}><Table2 aria-hidden="true" />Table</button>
          </div>
        )}
        footer={rows.length ? <span>{formatNumber(kpis.ready)} of {formatNumber(kpis.sources)} sources are ready across {formatNumber(rows.length)} {rows.length === 1 ? 'case' : 'cases'}.</span> : null}
      >
        {loading && !rows.length ? <SkeletonRows rows={2} label="Reading each case’s status" /> : null}
        {!loading && !rows.length ? <p className="dash-card__empty">No case has reported evidence yet.</p> : null}
        {rows.length && view === 'chart' ? (
          <>
            <ul className="readiness-bars">
              {rows.map(row => (
                <li key={row.caseId}>
                  <div className="readiness-bars__head">
                    <Link to={`/cases/${encodeURIComponent(row.caseId)}/overview`}><LanguageText as="bdi" identifier>{row.caseId}</LanguageText></Link>
                    <span>{formatNumber(row.counts.ready)} of {formatNumber(row.total)} ready</span>
                  </div>
                  <div className="readiness-bars__bar" role="group" aria-label={`Evidence readiness of ${row.caseId}`}>
                    {SEGMENTS.filter(segment => row.counts[segment.id] > 0).map(segment => {
                      const count = row.counts[segment.id]
                      return (
                        <Link key={segment.id} className={`rb-seg rb-seg--${segment.id}`} style={{ flexGrow: count }} to={evidenceLink(row.caseId, segment.id)} aria-label={`${formatNumber(count)} ${segment.label.toLowerCase()} of ${formatNumber(row.total)} sources in ${row.caseId}. Open these sources.`} title={`${segment.label}: ${formatNumber(count)}`}>
                          {count / row.total >= 0.1 ? <span aria-hidden="true">{formatNumber(count)}</span> : null}
                        </Link>
                      )
                    })}
                  </div>
                </li>
              ))}
            </ul>
            <ul className="chart-card__legend" aria-label="Legend">{SEGMENTS.map(segment => <li key={segment.id}><span className={`chart-card__swatch chart-card__swatch--${segment.id}`} aria-hidden="true" />{segment.label}</li>)}</ul>
          </>
        ) : null}
        {rows.length && view === 'table' ? (
          <div className="dash-table">
            <table>
              <thead><tr><th scope="col">Case</th>{SEGMENTS.slice(0, 3).map(segment => <th key={segment.id} scope="col" className="is-numeric">{segment.label}</th>)}<th scope="col" className="is-numeric">Sources</th></tr></thead>
              <tbody>{rows.map(row => (
                <tr key={row.caseId}>
                  <th scope="row"><Link to={`/cases/${encodeURIComponent(row.caseId)}/overview`}><LanguageText as="bdi" identifier>{row.caseId}</LanguageText></Link></th>
                  {SEGMENTS.slice(0, 3).map(segment => <td key={segment.id} className="is-numeric">{row.counts[segment.id] ? <Link to={evidenceLink(row.caseId, segment.id)}>{formatNumber(row.counts[segment.id])}</Link> : '0'}</td>)}
                  <td className="is-numeric">{formatNumber(row.total)}</td>
                </tr>
              ))}</tbody>
            </table>
          </div>
        ) : null}
      </Card>
    </div>
  )
}
