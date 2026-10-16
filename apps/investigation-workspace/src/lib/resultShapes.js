import { chartableResult } from './resultChart.js'

// Which views an answer can be seen in (roadmap 6.8.5 and 7.8.2). The Ask engine names its result columns by a convention the prompt teaches
// (src and dst for a graph, lat and lon for a map, label and value for a ranking, t and value for a series), and the page chooses from the columns that are
// really there; the table is always one click away, and the server's own statement of the shape (`enterprise.result_shape`) is used only to pick the first view.
const numeric = value => value !== null && value !== undefined && value !== '' && Number.isFinite(Number(value))

export function answerRows(presentation) {
  const rows = presentation?.original?.enterprise?.data_grid?.rows
  return Array.isArray(rows) ? rows : []
}

export function resultShapes(presentation) {
  const rows = answerRows(presentation)
  const keys = new Set(rows.flatMap(row => Object.keys(row || {})).map(key => key.toLowerCase()))
  const views = [{ id: 'table', label: 'Table' }]
  // the rows of a graph are pairs of numbers, not labelled measures: a bar chart of one end of each pair would hide the other end, so a graph answer gets no bar chart
  const isGraph = keys.has('src') && keys.has('dst') && rows.some(row => row.src != null && row.dst != null)
  const chart = isGraph ? null : chartableResult(presentation)
  if (chart) views.push({ id: 'chart', label: chart.kind === 'time' ? 'Chart over time' : 'Bar chart' })
  if (isGraph) views.push({ id: 'graph', label: 'Link chart' })
  if (keys.has('lat') && keys.has('lon') && rows.some(row => numeric(row.lat) && numeric(row.lon))) views.push({ id: 'map', label: 'Map' })
  const stated = String(presentation?.original?.enterprise?.result_shape || '').toLowerCase()
  const wanted = { graph: 'graph', map: 'map', ranked: 'chart', series: 'chart' }[stated]
  const first = views.some(view => view.id === wanted) ? wanted : views.some(view => view.id === 'graph') ? 'graph' : views.some(view => view.id === 'map') ? 'map' : 'table'
  return { views, first, shape: stated, rows, chart }
}

// The numbers a graph answer starts from (the sources that are never a destination), so that "open in Connections" lands on them.
export function graphTargets(rows) {
  const dst = new Set(rows.map(row => String(row.dst)))
  const sources = [...new Set(rows.map(row => String(row.src)))].filter(id => !dst.has(id))
  return (sources.length ? sources : [...new Set(rows.map(row => String(row.src)))]).slice(0, 5)
}

export function mapSites(rows) {
  const seen = new Map()
  for (const row of rows) {
    if (!numeric(row.lat) || !numeric(row.lon)) continue
    const id = String(row.cell_key ?? row.id ?? `${row.lat},${row.lon}`)
    const site = seen.get(id) || { id, lat: Number(row.lat), lon: Number(row.lon), label: String(row.label ?? row.site_name ?? id), value: 0, radius: numeric(row.radius_m) ? Number(row.radius_m) : 0 }
    site.value += numeric(row.value) ? Number(row.value) : numeric(row.events) ? Number(row.events) : 1
    seen.set(id, site)
  }
  return [...seen.values()]
}
