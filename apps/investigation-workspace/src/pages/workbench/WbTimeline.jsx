import { useCallback, useMemo } from 'react'
import EChart from '../../components/charts/EChart.jsx'

// The timeline of the workbench: one lane for each number of the question (events a day, drawn as quiet bars), the marks of the analyses as small dots on the lane's top rule, and ONE brush (the
// slider underneath). Moving the brush filters the links, the map and the table, because the period is the workbench's, not the chart's. It draws what it is given (lanes, day labels, marks);
// the live workbench reads them from the case, the design proof from its fixture.
const KIND_COLOUR = { window: 2, meeting: 3, successor: 1, move: 1, anchor: 1, alert: 'failed', rhythm: 4, silent: 'muted' }
const markColour = (theme, kind) => { const c = KIND_COLOUR[kind]; return c === 'failed' ? theme.failed : c === 'muted' ? theme.muted : theme.data[c ?? 0] }
const GAP = 22
const TOP = 6
const laneHeight = count => (count > 2 ? 42 : count > 1 ? 50 : 62)

export function WbTimeline({ lanes, labels, marks, markLines = [], range, onRange, onSelectMark, selectedMarkId, zoomKey, unit = 'events a day' }) {
  const days = labels.length
  const LANE = laneHeight(lanes.length)
  const placed = useMemo(() => marks.filter(m => m.day != null && m.day >= 0 && m.day < days), [marks, days])

  const buildOption = useCallback(theme => {
    const series = []
    lanes.forEach((lane, i) => {
      const peak = Math.max(1, ...lane.values)
      series.push({ type: 'bar', xAxisIndex: i, yAxisIndex: i, data: lane.values, barCategoryGap: days > 90 ? '10%' : '38%', itemStyle: { color: theme.data[0], opacity: 0.42, borderRadius: [1, 1, 0, 0] }, emphasis: { itemStyle: { opacity: 0.85 } }, z: 1,
        markLine: markLines.length && i === 0 ? { symbol: 'none', silent: true, lineStyle: { color: theme.data[4], type: 'dashed', width: 1.5 }, label: { formatter: p => p.data.name, color: theme.muted, fontSize: 10.5, position: 'insideEndTop' }, data: markLines.filter(l => l.day != null && l.day < days).map(l => ({ xAxis: l.day, name: l.label })) } : undefined })
      const own = placed.filter(m => m.kind !== 'silent' || i === 0)
      if (own.length) {
        series.push({ type: 'scatter', xAxisIndex: i, yAxisIndex: i, symbol: 'circle', symbolSize: 9, z: 5,
          data: own.map(m => ({ value: [labels[m.day], peak * 1.2], mark: m, itemStyle: { color: markColour(theme, m.kind), borderColor: m.id === selectedMarkId ? theme.text : theme.card, borderWidth: m.id === selectedMarkId ? 2.5 : 1.5 } })),
          tooltip: { formatter: p => p.data.mark.label } })
      }
    })
    return {
      animation: false, aria: { enabled: true }, textStyle: { fontFamily: 'inherit' },
      tooltip: { trigger: 'axis', axisPointer: { type: 'line', lineStyle: { color: theme.line } }, confine: true, borderColor: theme.line, backgroundColor: theme.card, textStyle: { color: theme.text, fontSize: 12 } },
      title: lanes.map((lane, i) => ({ text: lane.title, left: 0, top: TOP + i * (LANE + GAP) + 2, textStyle: { fontSize: 11, fontWeight: 500, color: theme.muted, fontFamily: 'var(--analyst-font-mono, monospace)' } })),
      grid: lanes.map((_, i) => ({ left: 52, right: 8, top: TOP + i * (LANE + GAP), height: LANE })),
      xAxis: lanes.map((_, i) => ({ type: 'category', gridIndex: i, data: labels, boundaryGap: true, axisLabel: { show: i === lanes.length - 1, color: theme.muted, fontSize: 10.5, interval: Math.max(6, Math.round(days / 10) - 1), margin: 8 }, axisLine: { lineStyle: { color: theme.line } }, axisTick: { show: false } })),
      yAxis: lanes.map((lane, i) => ({ type: 'value', gridIndex: i, min: 0, max: Math.max(1, ...lane.values) * 1.35, splitNumber: 2, axisLabel: { color: theme.muted, fontSize: 10, formatter: v => (v === 0 ? '' : String(Math.round(v))) }, splitLine: { lineStyle: { color: theme.line, opacity: 0.45, type: 'dotted' } } })),
      dataZoom: [{ type: 'slider', xAxisIndex: lanes.map((_, i) => i), startValue: range ? range[0] : 0, endValue: range ? range[1] : days - 1, bottom: 2, height: 16, left: 52, right: 8, brushSelect: false, showDataShadow: false, borderColor: theme.line, backgroundColor: 'transparent', fillerColor: 'rgba(110,150,150,0.16)', handleSize: '100%', handleStyle: { color: theme.card, borderColor: theme.muted, borderWidth: 1 }, moveHandleSize: 0, textStyle: { color: theme.muted, fontSize: 10 }, labelFormatter: v => labels[Math.round(Number(v))] || '' }],
      series,
    }
  }, [lanes, labels, placed, markLines, range, selectedMarkId, days, LANE])

  const height = TOP + lanes.length * (LANE + GAP) + 26
  return (
    <EChart
      buildOption={buildOption}
      height={height}
      label={`Timeline: ${unit} for ${lanes.map(l => l.title).join(', ')} and ${placed.length} marks of the analyses, with a brush that sets the period shown in every pane.`}
      zoomKey={zoomKey}
      onZoom={({ from, to }) => onRange?.([Math.max(0, Math.round(from)), Math.min(days - 1, Math.round(to))])}
      onSelect={params => { if (params?.data?.mark) onSelectMark?.(params.data.mark) }}
    />
  )
}
