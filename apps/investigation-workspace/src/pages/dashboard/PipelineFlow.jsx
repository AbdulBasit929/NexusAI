import { useMemo } from 'react'
import { ChartCard } from '../../components/charts/ChartCard.jsx'
import { Card } from '../../components/Card.jsx'
import { SkeletonRows } from '../../components/Skeleton.jsx'
import { formatNumber } from '../../lib/format.js'
import { FLOW_TARGETS, flowGraph, flowOption, flowRows } from '../../lib/pipelineFlow.js'
import { jumpToElement } from '../../lib/jump.js'

// Where do sources and rows end up? A flow diagram (brief §6: flow or lineage), because the question is about paths and
// proportions, not one figure: sources become ready, processing or failed, ready ones keep or lack a retained copy, and
// rows read become accepted, duplicate or rejected. Widths are the complete counts the service reported. Selecting a
// stage jumps to the section that lists it. The table view has every path as exact numbers.
export function PipelineFlow({ kpis, loading }) {
  const graph = useMemo(() => flowGraph(kpis), [kpis])
  const rows = useMemo(() => flowRows(graph), [graph])
  const chart = useMemo(() => ({
    buildOption: theme => flowOption(graph, theme),
    height: 360,
    fill: true,
    label: `Sources and rows by outcome. ${rows.map(row => `${row.from} to ${row.to}: ${formatNumber(row.value)}`).join('. ')}`,
    onSelect: params => {
      const target = FLOW_TARGETS[params?.name]
      if (target) jumpToElement(target)
    },
  }), [graph, rows])

  if (loading && !rows.length) return <div className="dash-slot"><Card title="Where do sources and rows end up?"><SkeletonRows rows={4} label="Reading outcomes" /></Card></div>
  if (!rows.length) return <div className="dash-slot"><Card title="Where do sources and rows end up?"><p className="dash-card__empty">Nothing has been ingested yet.</p></Card></div>
  return (
    <div id="dashboard-flow" className="dash-slot">
      <ChartCard
        className="flow-card"
        title="Where do sources and rows end up?"
        chart={chart}
        columns={[
          { key: 'from', label: 'From' },
          { key: 'to', label: 'To' },
          { key: 'value', label: 'Count', numeric: true, render: row => formatNumber(row.value) },
        ]}
        rows={rows}
        footer={<span>{formatNumber(kpis.sources)} sources · {formatNumber((kpis.acceptedRows || 0) + (kpis.duplicateRows || 0) + (kpis.rejectedRows || 0))} rows read</span>}
      />
    </div>
  )
}
