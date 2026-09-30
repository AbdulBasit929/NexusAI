import { Link } from 'react-router-dom'
import { ArrowRight, CircleCheckBig, Clock3, FileWarning, Rows3 } from 'lucide-react'
import { ProportionBar } from '../../components/DataVisualizations.jsx'
import { formatNumber } from '../../lib/format.js'

// The four figures that decide what the analyst does next, in reading order (Nielsen Norman Group: the most
// critical figures go top-left). Each tile says what it counts and links to the rows behind it; only "Needs review"
// carries a tone, and only when there is something to review. While a status is unknown the value is an ellipsis,
// never zero.
export function kpiTiles(kpis) {
  return [
    {
      id: 'review',
      icon: FileWarning,
      label: 'Needs review',
      value: kpis.review,
      note: 'Failed sources and completed jobs missing their retained copy',
      href: '#dashboard-attention',
      tone: kpis.review > 0 ? 'attention' : undefined,
    },
    {
      id: 'ready',
      icon: CircleCheckBig,
      label: 'Evidence ready',
      value: kpis.ready,
      valueSuffix: `of ${formatNumber(kpis.sources)}`,
      note: 'Sources you can search now',
      href: '#dashboard-readiness',
      bar: { total: kpis.sources, ready: kpis.ready, processing: kpis.processing, failed: kpis.failed },
    },
    {
      id: 'processing',
      icon: Clock3,
      label: 'Processing',
      value: kpis.processing,
      note: kpis.processing ? 'Not searchable until they finish' : 'Nothing is in progress',
      href: '#dashboard-readiness',
    },
    {
      id: 'rows',
      icon: Rows3,
      label: 'Structured rows',
      value: kpis.acceptedRows,
      note: 'Records accepted for analysis',
      href: '#dashboard-families',
    },
  ]
}

export function KpiTiles({ kpis, loading }) {
  return (
    <ul className="dash-kpis" aria-label="Workspace totals" aria-busy={loading ? 'true' : 'false'}>
      {kpiTiles(kpis).map(tile => {
        const Icon = tile.icon
        return (
          <li key={tile.id} className={`dash-kpi${tile.tone ? ` dash-kpi--${tile.tone}` : ''}`}>
            <a href={tile.href}>
              <span className="dash-kpi__head"><span className="dash-kpi__icon"><Icon aria-hidden="true" /></span><span className="dash-kpi__label">{tile.label}</span><ArrowRight className="dash-kpi__go" aria-hidden="true" /></span>
              <strong>{loading ? '…' : formatNumber(tile.value)}{!loading && tile.valueSuffix ? <span className="dash-kpi__of"> {tile.valueSuffix}</span> : null}</strong>
              {tile.bar && !loading ? <ProportionBar {...tile.bar} label="Evidence readiness across all cases" /> : null}
              <small>{tile.note}</small>
            </a>
          </li>
        )
      })}
    </ul>
  )
}
