import { formatNumber } from './format.js'

// Where do sources and rows end up? Two flows from the complete summary counts (never the bounded recent lists):
//   sources -> ready, processing, failed, not yet counted; ready -> retained copy kept or missing
//   rows read -> accepted, duplicate, rejected
// A stage with no count is left out, so the picture never draws an empty path. Everything is a count the service
// reported; nothing is estimated, and the two flows are separate because a source and a row are different things.
export const FLOW_TARGETS = {
  Ready: '#dashboard-cases',
  Processing: '#dashboard-cases',
  Failed: '#dashboard-attention',
  'Not counted yet': '#dashboard-cases',
  'Copy kept': '#dashboard-cases',
  'Copy missing': '#dashboard-attention',
  Accepted: '#dashboard-cases',
  Duplicate: '#dashboard-cases',
  Rejected: '#dashboard-cases',
}

const TONE = { Sources: 'neutral', Ready: 'ready', Processing: 'processing', Failed: 'failed', 'Not counted yet': 'excluded', 'Copy kept': 'ready', 'Copy missing': 'failed', 'Rows read': 'neutral', Accepted: 'ready', Duplicate: 'processing', Rejected: 'failed' }

export function flowGraph(kpis) {
  const links = []
  const link = (source, target, value) => { if (value > 0) links.push({ source, target, value }) }
  const counted = kpis.ready + kpis.processing + kpis.failed
  const gaps = Math.min(kpis.gaps || 0, kpis.ready)
  link('Sources', 'Ready', kpis.ready)
  link('Sources', 'Processing', kpis.processing)
  link('Sources', 'Failed', kpis.failed)
  link('Sources', 'Not counted yet', Math.max(0, kpis.sources - counted))
  link('Ready', 'Copy kept', kpis.ready - gaps)
  link('Ready', 'Copy missing', gaps)
  const rowsRead = (kpis.acceptedRows || 0) + (kpis.duplicateRows || 0) + (kpis.rejectedRows || 0)
  link('Rows read', 'Accepted', kpis.acceptedRows || 0)
  link('Rows read', 'Duplicate', kpis.duplicateRows || 0)
  link('Rows read', 'Rejected', kpis.rejectedRows || 0)
  const names = [...new Set(links.flatMap(entry => [entry.source, entry.target]))]
  const totals = { Sources: kpis.sources, 'Rows read': rowsRead }
  const nodes = names.map(name => ({ name, tone: TONE[name] || 'neutral', value: totals[name] ?? links.filter(entry => entry.target === name).reduce((sum, entry) => sum + entry.value, 0) }))
  return { nodes, links }
}

export function flowRows(graph) {
  return graph.links.map(entry => ({ key: `${entry.source}>${entry.target}`, from: entry.source, to: entry.target, value: entry.value }))
}

// Sources and rows are different units and differ by orders of magnitude, so each flow is its own sankey with its own
// scale, in its own band of the chart. On one shared scale the smaller flow would be a hairline.
export function flowOption(graph, theme) {
  const colour = { ready: theme.ready, processing: theme.processing, failed: theme.failed, excluded: theme.withheld, neutral: theme.muted }
  const groups = [graph.links.filter(entry => entry.source !== 'Rows read'), graph.links.filter(entry => entry.source === 'Rows read')].filter(links => links.length)
  const bands = groups.length === 2 ? [{ top: 28, bottom: '52%' }, { top: '64%', bottom: 8 }] : [{ top: 28, bottom: 8 }]
  const captions = groups.map(links => (links[0].source === 'Rows read' ? 'Rows read, by outcome' : 'Sources, by outcome'))
  const captionTops = groups.length === 2 ? [0, '57%'] : [0]
  return {
    aria: { enabled: false },
    title: captions.map((text, index) => ({ text, left: 8, top: captionTops[index], textStyle: { color: theme.muted, fontSize: 11, fontWeight: 700 } })),
    animationDuration: 600,
    animationEasing: 'cubicOut',
    tooltip: {
      trigger: 'item',
      confine: true,
      backgroundColor: theme.card,
      borderColor: theme.line,
      textStyle: { color: theme.text },
      formatter: params => (params.dataType === 'edge'
        ? `${params.data.source} → ${params.data.target}: <strong>${formatNumber(params.data.value)}</strong>`
        : `<strong>${params.name}</strong>: ${formatNumber(params.value)}<br/><em>Select to see where.</em>`),
    },
    series: groups.map((links, index) => {
      const names = new Set(links.flatMap(entry => [entry.source, entry.target]))
      return {
        type: 'sankey',
        left: 8,
        right: 110,
        ...bands[index],
        nodeAlign: 'left',
        nodeWidth: 14,
        nodeGap: 20,
        draggable: false,
        emphasis: { focus: 'adjacency' },
        lineStyle: { color: 'gradient', opacity: 0.38, curveness: 0.5 },
        label: { color: theme.text, fontSize: 12, fontWeight: 600, formatter: params => `${params.name}  ${formatNumber(params.value)}` },
        data: graph.nodes.filter(node => names.has(node.name)).map(node => ({ name: node.name, value: node.value, itemStyle: { color: colour[node.tone], borderColor: theme.card } })),
        links,
      }
    }),
  }
}
