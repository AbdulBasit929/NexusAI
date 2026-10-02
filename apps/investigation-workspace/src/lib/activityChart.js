import { shortDate } from './caseActivity.js'
import { formatNumber } from './format.js'

// The activity pulse: one softly filled line (straight segments, so nothing is implied between days) per series (a case, or questions against evidence) stacked over the
// last days that have any entry. Quiet by design: no gridlines but a faint baseline, three date ticks, a crosshair and a
// tooltip with the exact counts. The count axis is hidden because the tooltip and the summary carry the numbers.
export function pulseOption({ days, series }, theme, labelOf) {
  const colourOf = key => theme.data[Math.max(0, series.findIndex(entry => entry.key === key)) % theme.data.length]
  return {
    aria: { enabled: false },
    animationDuration: 500,
    grid: { left: 4, right: 4, top: 10, bottom: 22 },
    tooltip: {
      trigger: 'axis',
      confine: true,
      axisPointer: { type: 'line', lineStyle: { color: theme.muted, width: 1 } },
      backgroundColor: theme.card,
      borderColor: theme.line,
      textStyle: { color: theme.text },
      formatter: points => {
        const list = (Array.isArray(points) ? points : [points]).filter(point => point.value > 0)
        const total = list.reduce((sum, point) => sum + point.value, 0)
        const rows = list.map(point => `${point.marker}${point.seriesName}: <strong>${formatNumber(point.value)}</strong>`)
        return `<strong>${days[points[0]?.dataIndex] || ''}</strong><br/>${rows.length ? rows.join('<br/>') : 'No entries'}${rows.length > 1 ? `<br/>Total: <strong>${formatNumber(total)}</strong>` : ''}`
      },
    },
    xAxis: {
      type: 'category',
      data: days,
      boundaryGap: false,
      axisTick: { show: false },
      axisLine: { lineStyle: { color: theme.line } },
      axisLabel: { color: theme.muted, fontSize: 11, interval: Math.max(0, Math.floor(days.length / 3) - 1), formatter: value => shortDate(value) },
    },
    yAxis: { type: 'value', show: false, minInterval: 1 },
    series: series.map(entry => ({
      name: labelOf(entry.key),
      type: 'line',
      stack: 'pulse',
      smooth: false,
      symbol: 'none',
      lineStyle: { width: 1.5, color: colourOf(entry.key) },
      itemStyle: { color: colourOf(entry.key) },
      areaStyle: { color: colourOf(entry.key), opacity: 0.22 },
      emphasis: { focus: 'series' },
      data: entry.values,
    })),
  }
}
