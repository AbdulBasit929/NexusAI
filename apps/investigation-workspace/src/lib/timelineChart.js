import { describeBucket, familyColour, rangeStartIndex } from './caseActivity.js'
import { formatNumber } from './format.js'
import { curatedFamilyLabel } from './semanticCatalog.js'

const stamp = date => Date.parse(`${date}T00:00:00Z`)
export const dateOfStamp = value => new Date(value).toISOString().slice(0, 10)

// Lanes top to bottom: the record family with the most events first.
export function laneOrder(activity) {
  return [...activity.families].sort((left, right) => right.total - left.total || left.id.localeCompare(right.id)).map(family => family.id)
}

// The lane chart's height: a lane is 46px, plus the axis and the overview slider.
export function laneHeight(lanes) {
  return Math.min(460, Math.max(240, lanes * 46 + 120))
}

// One lane per record family on a real time axis (so quiet stretches are visibly quiet, not squeezed out), a bubble on each
// day that has events, sized by the square root of the count so a 10,000-event day does not hide a 50-event one. The
// slider below is the overview and the zoom; the dashed line marks the day chosen in the panel under the chart.
export function timelineOption(view, theme, order, { range = 'all', selected = '' } = {}) {
  const lanes = laneOrder(view)
  const peak = Math.max(1, ...view.days.flatMap(day => Object.values(day.byFamily)))
  const startIndex = rangeStartIndex(view, range)
  const first = view.days[startIndex]
  const last = view.days[view.days.length - 1]
  const span = last && first ? stamp(last.end || last.date) - stamp(first.date) : 0
  const pad = Math.max(86_400_000, span * 0.03)
  const bucket = view.bucket === 'week' ? 'week' : 'day'
  return {
    aria: { enabled: false },
    useUTC: true,
    animationDuration: 400,
    grid: { left: 8, right: 20, top: 8, bottom: 62, containLabel: true },
    tooltip: {
      trigger: 'item',
      confine: true,
      backgroundColor: theme.card,
      borderColor: theme.line,
      textStyle: { color: theme.text },
      formatter: point => {
        const day = view.days.find(entry => stamp(entry.date) === point.value[0])
        return `<strong>${day ? describeBucket(day) : ''}</strong><br/>${point.marker}${point.seriesName}: <strong>${formatNumber(point.value[2])}</strong><br/>All families: ${formatNumber(day?.total || 0)}<br/><em>Select to read this ${bucket}.</em>`
      },
    },
    xAxis: { type: 'time', min: first ? stamp(first.date) - pad : undefined, max: last ? stamp(last.end || last.date) + pad : undefined, axisLine: { lineStyle: { color: theme.line } }, axisTick: { lineStyle: { color: theme.line } }, axisLabel: { color: theme.muted, fontSize: 11, hideOverlap: true }, splitLine: { show: true, lineStyle: { color: theme.line, opacity: 0.35 } } },
    yAxis: { type: 'category', inverse: true, data: lanes.map(curatedFamilyLabel), axisTick: { show: false }, axisLine: { show: false }, axisLabel: { color: theme.text, fontSize: 12 }, splitLine: { show: true, lineStyle: { color: theme.line, opacity: 0.6 } }, splitArea: { show: true, areaStyle: { color: ['transparent', 'rgba(127,127,127,0.07)'] } } },
    dataZoom: [
      { type: 'inside', xAxisIndex: 0, filterMode: 'none' },
      { type: 'slider', xAxisIndex: 0, filterMode: 'none', height: 22, bottom: 8, borderColor: theme.line, textStyle: { color: theme.muted }, brushSelect: false, labelFormatter: value => dateOfStamp(value) },
    ],
    series: lanes.map((id, lane) => ({
      name: curatedFamilyLabel(id),
      type: 'scatter',
      symbolSize: point => 7 + 24 * Math.sqrt(point[2] / peak),
      itemStyle: { color: familyColour(id, order, theme.data), opacity: 0.82, borderColor: theme.card, borderWidth: 1 },
      emphasis: { scale: 1.25, itemStyle: { opacity: 1 } },
      data: view.days.filter(day => day.byFamily[id] > 0).map(day => [stamp(day.date), lane, day.byFamily[id]]),
      ...(lane === 0 && selected ? { markLine: { silent: true, symbol: 'none', animation: false, lineStyle: { type: 'dashed', color: theme.muted, width: 1.5 }, label: { show: false }, data: [{ xAxis: stamp(selected) }] } } : {}),
    })),
  }
}
