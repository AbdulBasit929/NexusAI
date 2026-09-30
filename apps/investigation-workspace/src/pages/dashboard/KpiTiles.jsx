import { Link } from 'react-router-dom'
import { ArrowRight } from 'lucide-react'
import { ProportionBar } from '../../components/DataVisualizations.jsx'
import { formatNumber } from '../../lib/format.js'

// Floored, never rounded: 999 of 1,000 ready is 99%, so a real failure is not rounded into "100%".
function readyPercent(kpis) {
  return kpis.sources ? `${Math.floor((kpis.ready / kpis.sources) * 100)}%` : null
}

function reviewDetail(kpis) {
  const parts = []
  if (kpis.failed > 0) parts.push(`${formatNumber(kpis.failed)} failed`)
  if (kpis.gaps > 0) parts.push(`${formatNumber(kpis.gaps)} missing retained copy`)
  return parts.length ? parts.join(' · ') : 'Nothing to review'
}

// The four figures that decide what the analyst does next, in reading order, as one summary strip rather than four
// boxes: the first cell (what needs review) is the point of the page and carries the only tone. Each says what it
// counts, with units, and links to the rows behind it. While a status is unknown the value is an ellipsis and when
// nothing reported it is a dash, never a measured zero.
export function kpiTiles(kpis) {
  // Totals cover only the cases that reported. Say so on the cell, so a partial sum is never read as the whole.
  const scope = kpis.unreported > 0 && kpis.reported > 0 ? `Across ${formatNumber(kpis.reported)} of ${formatNumber(kpis.cases)} cases` : null
  return [
    { id: 'review', label: 'Sources to review', value: kpis.review, detail: reviewDetail(kpis), href: '#dashboard-attention', tone: kpis.review > 0 ? 'attention' : undefined, scope },
    {
      id: 'ready',
      label: 'Ready to search',
      value: kpis.ready,
      valueSuffix: `of ${formatNumber(kpis.sources)}`,
      percent: readyPercent(kpis),
      detail: kpis.sources ? 'Sources you can search now' : 'No sources added yet',
      href: '#dashboard-readiness',
      // A bar of nothing says nothing, so it is drawn only when there is something to measure.
      bar: kpis.sources ? { total: kpis.sources, ready: kpis.ready, processing: kpis.processing, failed: kpis.failed } : null,
      scope,
    },
    { id: 'processing', label: 'Processing', value: kpis.processing, detail: kpis.processing ? 'Not searchable until they finish' : 'Nothing in progress', href: '#dashboard-readiness', quiet: !kpis.processing, scope },
    { id: 'rows', label: 'Structured rows', value: kpis.acceptedRows, detail: 'Accepted for analysis', href: '#dashboard-families', scope },
  ]
}

export function KpiTiles({ kpis, loading }) {
  const unavailable = !loading && kpis.cases > 0 && kpis.reported === 0
  return (
    <ul className="dash-kpis dash-strip" aria-label="Workspace totals" aria-busy={loading ? 'true' : 'false'}>
      {kpiTiles(kpis).map(tile => (
        <li key={tile.id} className={`dash-kpi${tile.tone ? ` dash-kpi--${tile.tone}` : ''}${tile.quiet ? ' dash-kpi--quiet' : ''}`}>
          <a href={tile.href}>
            <span className="dash-kpi__label">{tile.label}<ArrowRight className="dash-kpi__go" aria-hidden="true" /></span>
            <strong>
              {loading ? '…' : unavailable ? '—' : formatNumber(tile.value)}
              {!loading && !unavailable && tile.valueSuffix && kpis.sources ? <span className="dash-kpi__of"> {tile.valueSuffix}</span> : null}
              {!loading && !unavailable && tile.percent ? <span className="dash-kpi__pct">{tile.percent}</span> : null}
            </strong>
            {tile.bar && !loading && !unavailable ? <ProportionBar {...tile.bar} label="Evidence readiness across all cases" /> : null}
            <small>{unavailable ? 'Status unavailable' : tile.detail}</small>
            {tile.scope && !loading && !unavailable ? <small className="dash-kpi__scope">{tile.scope}</small> : null}
          </a>
        </li>
      ))}
    </ul>
  )
}
