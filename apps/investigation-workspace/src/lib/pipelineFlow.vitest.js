import { describe, expect, it } from 'vitest'
import { FLOW_TARGETS, flowGraph, flowOption, flowRows } from './pipelineFlow.js'

const kpis = { sources: 54, ready: 50, processing: 2, failed: 2, gaps: 3, acceptedRows: 1000, duplicateRows: 40, rejectedRows: 10 }
const link = (graph, source, target) => graph.links.find(entry => entry.source === source && entry.target === target)?.value

describe('flowGraph', () => {
  it('splits sources by outcome and ready sources by retained copy, from complete counts', () => {
    const graph = flowGraph(kpis)
    expect(link(graph, 'Sources', 'Ready')).toBe(50)
    expect(link(graph, 'Sources', 'Processing')).toBe(2)
    expect(link(graph, 'Sources', 'Failed')).toBe(2)
    expect(link(graph, 'Sources', 'Not counted yet')).toBeUndefined()
    expect(link(graph, 'Ready', 'Copy kept')).toBe(47)
    expect(link(graph, 'Ready', 'Copy missing')).toBe(3)
  })

  it('splits rows into accepted, duplicate and rejected as a separate flow', () => {
    const graph = flowGraph(kpis)
    expect(link(graph, 'Rows read', 'Accepted')).toBe(1000)
    expect(link(graph, 'Rows read', 'Duplicate')).toBe(40)
    expect(link(graph, 'Rows read', 'Rejected')).toBe(10)
    expect(graph.nodes.find(node => node.name === 'Rows read').value).toBe(1050)
    expect(graph.nodes.find(node => node.name === 'Sources').value).toBe(54)
  })

  it('accounts for every source: the outgoing flows of Sources add up to the total', () => {
    const graph = flowGraph({ ...kpis, sources: 60 })
    const out = graph.links.filter(entry => entry.source === 'Sources').reduce((sum, entry) => sum + entry.value, 0)
    expect(out).toBe(60)
    expect(link(graph, 'Sources', 'Not counted yet')).toBe(6)
  })

  it('never draws an empty path, and never lets missing copies exceed ready sources', () => {
    const graph = flowGraph({ sources: 4, ready: 4, processing: 0, failed: 0, gaps: 9, acceptedRows: 0, duplicateRows: 0, rejectedRows: 0 })
    expect(graph.links.every(entry => entry.value > 0)).toBe(true)
    expect(link(graph, 'Ready', 'Copy missing')).toBe(4)
    expect(link(graph, 'Ready', 'Copy kept')).toBeUndefined()
    expect(graph.nodes.some(node => node.name === 'Rows read')).toBe(false)
    expect(flowGraph({ sources: 0, ready: 0, processing: 0, failed: 0, gaps: 0 }).links).toEqual([])
  })

  it('lists every path as an exact table row, and every stage jumps to a section', () => {
    const rows = flowRows(flowGraph(kpis))
    expect(rows).toContainEqual({ key: 'Sources>Ready', from: 'Sources', to: 'Ready', value: 50 })
    for (const node of flowGraph(kpis).nodes.filter(entry => !['Sources', 'Ready', 'Rows read'].includes(entry.name))) expect(FLOW_TARGETS[node.name]).toMatch(/^#dashboard-/)
  })
})

describe('flowOption', () => {
  const theme = { text: '#111', muted: '#555', line: '#ccc', card: '#fff', ready: '#0a0', processing: '#a80', failed: '#a00', withheld: '#777' }
  it('draws one sankey from the graph with status colours and exact tooltips', () => {
    const option = flowOption(flowGraph(kpis), theme)
    expect(option.series.every(series => series.type === 'sankey')).toBe(true)
    // Sources and rows are different units, so each flow has its own scale in its own band.
    expect(option.series).toHaveLength(2)
    const sources = option.series.find(series => series.data.some(node => node.name === 'Failed'))
    const rows = option.series.find(series => series.data.some(node => node.name === 'Accepted'))
    expect(sources.data.find(node => node.name === 'Failed').itemStyle.color).toBe('#a00')
    expect(sources.data.find(node => node.name === 'Ready').itemStyle.color).toBe('#0a0')
    expect(sources.links.every(entry => entry.source !== 'Rows read')).toBe(true)
    expect(rows.links.every(entry => entry.source === 'Rows read')).toBe(true)
    expect(sources.links.length + rows.links.length).toBe(flowGraph(kpis).links.length)
    expect(sources.top).not.toBe(rows.top)
    expect(option.title.map(title => title.text)).toEqual(['Sources, by outcome', 'Rows read, by outcome'])
    expect(flowOption(flowGraph({ ...kpis, acceptedRows: 0, duplicateRows: 0, rejectedRows: 0 }), theme).series).toHaveLength(1)
    expect(option.tooltip.formatter({ dataType: 'edge', data: { source: 'Sources', target: 'Failed', value: 2 } })).toContain('<strong>2</strong>')
    expect(option.tooltip.formatter({ dataType: 'node', name: 'Failed', value: 2 })).toContain('Select to see where')
  })
})
