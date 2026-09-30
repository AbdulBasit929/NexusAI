import { formatNumber } from './format.js'

// A chart for an answer, only when the answer is a ranked list or a series over time and the numbers are measures the
// service marked as such. It reads the same rows as the table below it (the table stays the exact, linkable version), so
// the chart can never say something the table does not. Anything ambiguous, such as an identifier that merely looks
// numeric (a phone number), gets no chart rather than a wrong one. Rules: UI_REDESIGN_BRIEF §6.
const MEASURE_KEY = /^m\d+$|(^|_)(count|total|events?|calls?|records?|amount|sum|duration|hits)($|_)/i
const MAX_POINTS = 12
const MIN_POINTS = 2
const MAX_TIME_POINTS = 60

const isDay = value => /^\d{4}-\d{2}-\d{2}/.test(String(value ?? ''))
const numeric = value => value !== null && value !== '' && value !== undefined && Number.isFinite(Number(value)) && Number(value) >= 0

export function chartableResult(presentation) {
  const rows = presentation?.original?.enterprise?.data_grid?.rows
  const columns = presentation?.result?.columns || []
  if (!Array.isArray(rows) || rows.length < MIN_POINTS || !columns.length) return null
  const measure = columns.find(column => MEASURE_KEY.test(column.key) && rows.every(row => numeric(row?.[column.key])))
  if (!measure) return null
  const label = columns.find(column => column.key !== measure.key && !MEASURE_KEY.test(column.key) && rows.every(row => typeof row?.[column.key] === 'string' && row[column.key].trim() !== ''))
  if (!label) return null
  const points = rows.map(row => ({ label: String(row[label.key]).trim(), value: Number(row[measure.key]) }))
  if (new Set(points.map(point => point.label)).size !== points.length) return null
  const time = points.every(point => isDay(point.label))
  if (time) {
    const series = points.map(point => ({ ...point, label: point.label.slice(0, 10) })).sort((left, right) => left.label.localeCompare(right.label))
    return series.length > MAX_TIME_POINTS ? null : { kind: 'time', labelName: label.label, valueName: measure.label, points: series, shown: series.length, total: rows.length }
  }
  const ranked = [...points].sort((left, right) => right.value - left.value || left.label.localeCompare(right.label))
  return { kind: 'ranked', labelName: label.label, valueName: measure.label, points: ranked.slice(0, MAX_POINTS), shown: Math.min(MAX_POINTS, ranked.length), total: rows.length }
}

export function resultChartLabel(chart) {
  const top = chart.points.slice(0, 3).map(point => `${point.label}: ${formatNumber(point.value)}`).join('; ')
  return chart.kind === 'time' ? `${chart.valueName} by day, ${chart.points[0].label} to ${chart.points.at(-1).label}.` : `${chart.valueName} by ${chart.labelName}, highest first. ${top}.`
}

export function resultChartOption(chart, theme) {
  const base = {
    aria: { enabled: false },
    animationDuration: 450,
    animationEasing: 'cubicOut',
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' }, confine: true, backgroundColor: theme.card, borderColor: theme.line, textStyle: { color: theme.text }, valueFormatter: value => formatNumber(value) },
  }
  const bar = { type: 'bar', barMaxWidth: chart.kind === 'ranked' ? 22 : 32, itemStyle: { color: theme.data[0], borderRadius: chart.kind === 'ranked' ? [0, 4, 4, 0] : [4, 4, 0, 0] }, data: chart.points.map(point => point.value), name: chart.valueName }
  if (chart.kind === 'ranked') {
    return {
      ...base,
      grid: { left: 8, right: 56, top: 8, bottom: 8, containLabel: true },
      xAxis: { type: 'value', minInterval: 1, splitLine: { lineStyle: { color: theme.line, opacity: 0.6 } }, axisLabel: { color: theme.muted, fontSize: 11, formatter: value => formatNumber(value) } },
      yAxis: { type: 'category', inverse: true, data: chart.points.map(point => point.label), axisTick: { show: false }, axisLine: { lineStyle: { color: theme.line } }, axisLabel: { color: theme.text, fontSize: 12, width: 160, overflow: 'truncate' } },
      series: [{ ...bar, label: { show: true, position: 'right', color: theme.text, fontSize: 12, fontWeight: 600, formatter: params => formatNumber(params.value) } }],
    }
  }
  return {
    ...base,
    grid: { left: 8, right: 16, top: 16, bottom: 8, containLabel: true },
    xAxis: { type: 'category', data: chart.points.map(point => point.label), axisTick: { show: false }, axisLine: { lineStyle: { color: theme.line } }, axisLabel: { color: theme.muted, fontSize: 11, hideOverlap: true } },
    yAxis: { type: 'value', minInterval: 1, splitLine: { lineStyle: { color: theme.line, opacity: 0.6 } }, axisLabel: { color: theme.muted, fontSize: 11, formatter: value => formatNumber(value) } },
    series: [bar],
  }
}

export function resultChartHeight(chart) {
  return chart.kind === 'ranked' ? Math.max(120, chart.points.length * 34 + 24) : 240
}
