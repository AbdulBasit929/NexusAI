import { useCallback, useMemo } from 'react'
import EChart from '../../components/charts/EChart.jsx'
import { DATA, dayLabel, laneOf, rhythmOf, short } from '../../lib/workbenchProof.js'

// The timeline of the workbench direction proof: one lane for each number of the question (events a day, drawn as quiet bars), the marks of the analyses as small dots on the lane's top rule,
// and ONE brush (the slider underneath). Moving the brush filters the links, the map's windows and the table, because the period is the workbench's, not the chart's.
const KIND_COLOUR = { window: 2, meeting: 3, move: 1, alert: 'failed', rhythm: 4, silent: 'muted' }
const markColour = (theme, kind) => { const c = KIND_COLOUR[kind]; return c === 'failed' ? theme.failed : c === 'muted' ? theme.muted : theme.data[c ?? 0] }
const LANE = 58
const GAP = 22
const TOP = 6

export function WbTimeline({ numbers, marks, range, onRange, onSelectMark, selectedMarkId, compact = false, zoomKey }) {
  const days = DATA.days
  const labels = useMemo(() => Array.from({ length: days }, (_, i) => dayLabel(i)), [days])
  const lanes = useMemo(() => (compact
    ? [{ id: 'all', title: numbers.length > 1 ? 'All' : short(numbers[0]), values: labels.map((_, i) => numbers.reduce((s, n) => s + (laneOf(n)[i] || 0), 0)) }]
    : numbers.slice(0, 3).map(n => ({ id: n, title: short(n), values: laneOf(n) }))), [compact, numbers, labels])

  const buildOption = useCallback(theme => {
    const rhythm = marks.find(m => m.kind === 'rhythm') ? rhythmOf(numbers[0]) : null
    const series = []
    lanes.forEach((lane, i) => {
      const peak = Math.max(1, ...lane.values)
      series.push({ type: 'bar', xAxisIndex: i, yAxisIndex: i, data: lane.values, barCategoryGap: '38%', itemStyle: { color: theme.data[0], opacity: 0.42, borderRadius: [1, 1, 0, 0] }, emphasis: { itemStyle: { opacity: 0.85 } }, z: 1,
        markLine: rhythm && i === 0 ? { symbol: 'none', silent: true, lineStyle: { color: theme.data[4], type: 'dashed', width: 1.5 }, label: { formatter: 'rhythm changes', color: theme.muted, fontSize: 10.5, position: 'insideEndTop' }, data: [{ xAxis: rhythm.first_day }] } : undefined })
      const own = marks.filter(m => m.kind !== 'silent' || i === 0)
      if (own.length) {
        series.push({ type: 'scatter', xAxisIndex: i, yAxisIndex: i, symbol: 'circle', symbolSize: 9, z: 5,
          data: own.map(m => ({ value: [dayLabel(m.day), peak * 1.2], mark: m, itemStyle: { color: markColour(theme, m.kind), borderColor: m.id === selectedMarkId ? theme.text : theme.card, borderWidth: m.id === selectedMarkId ? 2.5 : 1.5 } })),
          tooltip: { formatter: p => p.data.mark.label } })
      }
    })
    return {
      animation: false, aria: { enabled: true }, textStyle: { fontFamily: 'inherit' },
      tooltip: { trigger: 'axis', axisPointer: { type: 'line', lineStyle: { color: theme.line } }, confine: true, borderColor: theme.line, backgroundColor: theme.card, textStyle: { color: theme.text, fontSize: 12 } },
      title: lanes.map((lane, i) => ({ text: lane.title, left: 0, top: TOP + i * (LANE + GAP) + 2, textStyle: { fontSize: 11, fontWeight: 500, color: theme.muted, fontFamily: 'var(--analyst-font-mono, monospace)' } })),
      grid: lanes.map((_, i) => ({ left: 52, right: 8, top: TOP + i * (LANE + GAP), height: LANE })),
      xAxis: lanes.map((_, i) => ({ type: 'category', gridIndex: i, data: labels, boundaryGap: true, axisLabel: { show: i === lanes.length - 1, color: theme.muted, fontSize: 10.5, interval: 6, margin: 8 }, axisLine: { lineStyle: { color: theme.line } }, axisTick: { show: false } })),
      yAxis: lanes.map((lane, i) => ({ type: 'value', gridIndex: i, min: 0, max: Math.max(1, ...lane.values) * 1.35, splitNumber: 2, axisLabel: { color: theme.muted, fontSize: 10, formatter: v => (v === 0 ? '' : String(Math.round(v))) }, splitLine: { lineStyle: { color: theme.line, opacity: 0.45, type: 'dotted' } } })),
      dataZoom: [{ type: 'slider', xAxisIndex: lanes.map((_, i) => i), startValue: range ? range[0] : 0, endValue: range ? range[1] : days - 1, bottom: 2, height: 16, left: 52, right: 8, brushSelect: false, showDataShadow: false, borderColor: theme.line, backgroundColor: 'transparent', fillerColor: 'rgba(110,150,150,0.16)', handleSize: '100%', handleStyle: { color: theme.card, borderColor: theme.muted, borderWidth: 1 }, moveHandleSize: 0, textStyle: { color: theme.muted, fontSize: 10 }, labelFormatter: v => dayLabel(Number(v)) }],
      series,
    }
  }, [lanes, marks, numbers, range, selectedMarkId, labels, days])

  const height = TOP + lanes.length * (LANE + GAP) + 26
  return (
    <EChart
      buildOption={buildOption}
      height={height}
      label={`Timeline: events a day for ${numbers.map(short).join(', ')} and ${marks.length} marks of the analyses, with a brush that sets the period shown in every pane.`}
      zoomKey={zoomKey}
      onZoom={({ from, to }) => onRange?.([Math.max(0, Math.round(from)), Math.min(days - 1, Math.round(to))])}
      onSelect={params => { if (params?.data?.mark) onSelectMark?.(params.data.mark) }}
    />
  )
}
