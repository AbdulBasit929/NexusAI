import tokenReport from '../../../design/tokens.generated.json'

// The committed generator report is the SSR/test fallback; the browser always reads the
// resolved CSS variables so direction, theme and contrast preferences stay authoritative.
const light = tokenReport.themes['a-light']
const fallback = {
  text: light['text-1'], muted: light['text-3'], line: light['border-subtle'], card: light['surface-1'],
  ready: light['status-ready'], processing: light['status-processing'], failed: light['status-failed'], withheld: light['status-excluded'],
  data: Array.from({ length: 6 }, (_, index) => light[`data-${index + 1}`]),
}

export function chartTheme() {
  if (!globalThis.document) return fallback
  const style = globalThis.getComputedStyle(globalThis.document.documentElement)
  const read = (name, value) => style.getPropertyValue(name).trim() || value
  return {
    text: read('--nx-text-1', fallback.text),
    muted: read('--nx-text-3', fallback.muted),
    line: read('--nx-border-subtle', fallback.line),
    card: read('--nx-surface-1', fallback.card),
    ready: read('--nx-status-ready', fallback.ready),
    processing: read('--nx-status-processing', fallback.processing),
    failed: read('--nx-status-failed', fallback.failed),
    withheld: read('--nx-status-excluded', fallback.withheld),
    data: fallback.data.map((value, index) => read(`--nx-data-${index + 1}`, value)),
  }
}

export function echartsTheme() {
  const theme = chartTheme()
  return {
    color: theme.data,
    backgroundColor: 'transparent',
    textStyle: { color: theme.text, fontFamily: 'Noto Sans Variable, sans-serif' },
    title: { textStyle: { color: theme.text, fontWeight: 600 } },
    categoryAxis: { axisLine: { lineStyle: { color: theme.line } }, axisLabel: { color: theme.muted } },
    valueAxis: { axisLine: { show: false }, axisTick: { show: false }, axisLabel: { color: theme.muted }, splitLine: { lineStyle: { color: theme.line } } },
    tooltip: { backgroundColor: theme.card, borderColor: theme.line, borderWidth: 1, textStyle: { color: theme.text } },
  }
}
