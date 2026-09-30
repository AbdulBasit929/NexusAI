// Chart colours are read from the governed tokens at draw time, never hard-coded, so light,
// dark and any later palette change flow into every chart. Fallbacks match the light theme.
const fallback = {
  text: '#111827', muted: '#3f5262', line: '#c3d3d7', card: '#fbfdfd',
  ready: '#166534', processing: '#854d0e', failed: '#a1122f', withheld: '#475569',
  data: ['#0072b2', '#c2410c', '#00795f', '#7e3fb2', '#8a6100', '#475569'],
}

export function chartTheme() {
  if (!globalThis.document) return fallback
  const style = globalThis.getComputedStyle(globalThis.document.documentElement)
  const read = (name, value) => style.getPropertyValue(name).trim() || value
  return {
    text: read('--analyst-text', fallback.text),
    muted: read('--analyst-text-muted', fallback.muted),
    line: read('--analyst-line', fallback.line),
    card: read('--analyst-surface-1', fallback.card),
    ready: read('--analyst-status-ready', fallback.ready),
    processing: read('--analyst-status-processing', fallback.processing),
    failed: read('--analyst-status-failed', fallback.failed),
    withheld: read('--analyst-status-excluded', fallback.withheld),
    data: fallback.data.map((value, index) => read(`--analyst-data-${index + 1}`, value)),
  }
}
