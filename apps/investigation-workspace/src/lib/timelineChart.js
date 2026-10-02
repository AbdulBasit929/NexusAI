import { describeBucket, familyColour, rangeStartIndex } from './caseActivity.js'
import { formatNumber } from './format.js'
import { curatedFamilyLabel } from './semanticCatalog.js'

const stamp = date => Date.parse(`${date}T00:00:00Z`)
export const dateOfStamp = value => new Date(value).toISOString().slice(0, 10)

// Lanes top to bottom: the record family with the most events first.
export function laneOrder(activity) {
  return [...activity.families].sort((left, right) => right.total - left.total || left.id.localeCompare(right.id)).map(family => family.id)
}

const VOLUME = 84
const LANE = 38
const GAP = 26
const AXIS = 26
const SLIDER = 46

// The chart's height: the volume strip, one lane per family, the date axis and the zoom slider.
export function workbenchHeight(lanes) {
  return 8 + VOLUME + GAP + lanes * LANE + AXIS + SLIDER
}

// Two views on one time axis and one zoom: a stacked volume strip on top (how much happened, by family) and, under it,
// one lane per record family with a bubble on each day that has events (where each family was active), sized by the square
// root of the count so a 10,000-event day does not hide a 50-event one. A crosshair runs through both. The real time axis
// keeps quiet stretches visibly quiet. The dashed line marks the day chosen in the inspector.
export function workbenchOption(view, theme, order, { range = 'all', selected = '' } = {}) {
  const lanes = laneOrder(view)
  const peak = Math.max(1, ...view.days.flatMap(day => Object.values(day.byFamily)))
  const first = view.days[rangeStartIndex(view, range)]
  const last = view.days[view.days.length - 1]
  const span = last && first ? stamp(last.end || last.date) - stamp(first.date) : 0
  const pad = Math.max(86_400_000, span * 0.03)
  const min = first ? stamp(first.date) - pad : undefined
  const max = last ? stamp(last.end || last.date) + pad : undefined
  const bucket = view.bucket === 'week' ? 'week' : 'day'
  const left = 154
  const lanesTop = 8 + VOLUME + GAP
  const line = selected ? { markLine: { silent: true, symbol: 'none', animation: false, lineStyle: { type: 'dashed', color: theme.text, width: 1.25, opacity: 0.7 }, label: { show: false }, data: [{ xAxis: stamp(selected) }] } } : {}
  const axisBase = { type: 'time', min, max, axisLine: { lineStyle: { color: theme.line } }, axisTick: { lineStyle: { color: theme.line } }, splitLine: { show: true, lineStyle: { color: theme.line, opacity: 0.3 } }, axisPointer: { show: true, lineStyle: { color: theme.muted, type: 'solid', width: 1 }, label: { show: false } } }
  return {
    aria: { enabled: false },
    useUTC: true,
    animationDuration: 450,
    grid: [
      { left, right: 18, top: 8, height: VOLUME },
      { left, right: 18, top: lanesTop, height: lanes.length * LANE },
    ],
    axisPointer: { link: [{ xAxisIndex: 'all' }] },
    tooltip: {
      trigger: 'axis',
      confine: true,
      axisPointer: { type: 'line', snap: true },
      backgroundColor: theme.card,
      borderColor: theme.line,
      textStyle: { color: theme.text },
      formatter: points => {
        const list = Array.isArray(points) ? points : [points]
        const at = list.find(point => Array.isArray(point.value))?.value?.[0]
        const day = view.days.find(entry => stamp(entry.date) === at)
        if (!day) return ''
        const rows = Object.entries(day.byFamily).filter(([, count]) => count > 0).sort((left2, right2) => right2[1] - left2[1])
          .map(([id, count]) => `<span style="display:inline-block;inline-size:8px;block-size:8px;border-radius:2px;background:${familyColour(id, order, theme.data)};margin-inline-end:6px"></span>${curatedFamilyLabel(id)}: <strong>${formatNumber(count)}</strong>`)
        return `<strong>${describeBucket(day)}</strong><br/>${rows.join('<br/>')}<br/>Total: <strong>${formatNumber(day.total)}</strong><br/><em>Select to read this ${bucket}.</em>`
      },
    },
    xAxis: [
      { ...axisBase, gridIndex: 0, axisLabel: { show: false } },
      { ...axisBase, gridIndex: 1, axisLabel: { color: theme.muted, fontSize: 11, hideOverlap: true } },
    ],
    yAxis: [
      { type: 'value', gridIndex: 0, minInterval: 1, splitNumber: 2, axisLabel: { color: theme.muted, fontSize: 11, formatter: value => formatNumber(value) }, splitLine: { lineStyle: { color: theme.line, opacity: 0.4 } } },
      { type: 'category', gridIndex: 1, inverse: true, data: lanes.map(curatedFamilyLabel), axisTick: { show: false }, axisLine: { show: false }, axisLabel: { color: theme.text, fontSize: 12, width: 140, overflow: 'truncate' }, splitLine: { show: true, lineStyle: { color: theme.line, opacity: 0.5 } }, splitArea: { show: true, areaStyle: { color: ['transparent', 'rgba(127,127,127,0.07)'] } } },
    ],
    dataZoom: [
      { type: 'inside', xAxisIndex: [0, 1], filterMode: 'none' },
      { type: 'slider', xAxisIndex: [0, 1], filterMode: 'none', height: 24, bottom: 8, left, right: 18, borderColor: theme.line, textStyle: { color: theme.muted }, brushSelect: false, labelFormatter: value => dateOfStamp(value) },
    ],
    series: [
      ...lanes.map((id, index) => ({
        name: curatedFamilyLabel(id),
        type: 'bar',
        stack: 'volume',
        xAxisIndex: 0,
        yAxisIndex: 0,
        barMinWidth: 3,
        barMaxWidth: 22,
        itemStyle: { color: familyColour(id, order, theme.data) },
        emphasis: { focus: 'series' },
        data: view.days.filter(day => day.byFamily[id] > 0).map(day => [stamp(day.date), day.byFamily[id]]),
        ...(index === 0 ? line : {}),
      })),
      ...lanes.map((id, index) => ({
        name: curatedFamilyLabel(id),
        type: 'scatter',
        xAxisIndex: 1,
        yAxisIndex: 1,
        symbolSize: point => 5 + 15 * Math.sqrt(point[2] / peak),
        itemStyle: { color: familyColour(id, order, theme.data), opacity: 0.85, borderColor: theme.card, borderWidth: 1 },
        emphasis: { scale: 1.25, itemStyle: { opacity: 1 } },
        data: view.days.filter(day => day.byFamily[id] > 0).map(day => [stamp(day.date), index, day.byFamily[id]]),
        ...(index === 0 ? line : {}),
      })),
    ],
  }
}
