import { lazy, Suspense, useMemo } from 'react'
import { chartableResult, resultChartHeight, resultChartLabel, resultChartOption } from '../lib/resultChart.js'
import { formatNumber } from '../lib/format.js'

// Code-split with the rest of the charts; ECharts loads only when an answer is actually chartable.
const EChart = lazy(() => import('./charts/EChart.jsx'))

// The picture of a ranked or time-series answer. It sits above the exact table, which stays the way to reach each row
// and its source, and says how many of the rows it draws.
export function ResultChart({ presentation }) {
  const chart = useMemo(() => chartableResult(presentation), [presentation])
  const buildOption = useMemo(() => (chart ? theme => resultChartOption(chart, theme) : null), [chart])
  if (!chart) return null
  return (
    <figure className="result-chart">
      <Suspense fallback={<p className="chart-card__loading" role="status">Loading chart…</p>}>
        <EChart buildOption={buildOption} height={resultChartHeight(chart)} label={resultChartLabel(chart)} />
      </Suspense>
      <figcaption>
        {chart.kind === 'time' ? `${chart.valueName} by day` : `${chart.valueName} by ${chart.labelName}, highest first`}
        {chart.total > chart.shown ? ` · top ${formatNumber(chart.shown)} of ${formatNumber(chart.total)} shown; all rows are in the table` : ''}
      </figcaption>
    </figure>
  )
}
