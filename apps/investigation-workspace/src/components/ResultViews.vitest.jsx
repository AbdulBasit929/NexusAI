import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { ResultViews } from './ResultViews.jsx'

// Cytoscape, MapLibre and ECharts need a real canvas and WebGL; the views are tested for what they are given and what they offer, the pieces for what they draw.
vi.mock('./LinkChart.jsx', () => ({
  LinkChart: ({ model, onSelectNode }) => (
    <div data-testid="link-chart" data-nodes={model.nodes.length} data-edges={model.edges.length}>
      <button type="button" onClick={() => onSelectNode(model.nodes[0].id)}>pick first number</button>
    </div>
  ),
}))
vi.mock('./GeoMap.jsx', () => ({
  GeoMap: ({ sites }) => <div data-testid="geo-map" data-sites={sites.length} />,
}))
vi.mock('./ResultChart.jsx', () => ({ ResultChart: () => <div data-testid="result-chart" /> }))
vi.mock('./ResultTable.jsx', () => ({ ResultTable: ({ result }) => <table data-testid="result-table" data-rows={result.rows.length} /> }))

// Synthetic numbers from the reserved range.
const presentation = (keys, rows, shape = '') => ({
  result: { columns: keys.map(key => ({ key, label: key })), rows },
  citations: { items: [] },
  original: { enterprise: { result_shape: shape, data_grid: { rows } } },
})
const graphAnswer = presentation(['src', 'dst', 'events'], [
  { src: '923000001001', dst: '923000001002', events: 4 },
  { src: '923000001002', dst: '923000001003', events: 2 },
], 'graph')
const mapAnswer = presentation(['cell_key', 'lat', 'lon', 'events'], [
  { cell_key: 'c1', lat: 33.1, lon: 73.1, events: 9 },
  { cell_key: 'c2', lat: 33.2, lon: 73.2, events: 3 },
], 'map')
const plainAnswer = presentation(['label'], [{ label: 'a' }, { label: 'b' }])

function mount(answer, path = '/cases/case-a/investigate') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes><Route path="/cases/:id/investigate" element={<ResultViews presentation={answer} />} /></Routes>
    </MemoryRouter>,
  )
}

describe('ResultViews', () => {
  it('shows a plain answer as it always was: the chart above the table, with no switcher', async () => {
    mount(plainAnswer)
    expect(await screen.findByTestId('result-table')).toBeInTheDocument()
    expect(screen.getByTestId('result-chart')).toBeInTheDocument()
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument()
  })

  it('opens a graph answer on the link chart and offers the table one click away', async () => {
    mount(graphAnswer)
    const chart = await screen.findByTestId('link-chart')
    expect(chart).toHaveAttribute('data-nodes', '3')
    expect(chart).toHaveAttribute('data-edges', '2')
    const tabs = screen.getAllByRole('tab')
    expect(tabs.map(tab => tab.textContent)).toEqual(['Table', 'Link chart'])
    expect(screen.getByRole('tab', { name: 'Link chart' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('tab', { name: 'Table' })).toHaveAttribute('aria-selected', 'false')
    expect(screen.getByRole('tabpanel')).toHaveAttribute('aria-labelledby', 'rv-graph')

    await userEvent.click(screen.getByRole('tab', { name: 'Table' }))
    expect(screen.getByTestId('result-table')).toHaveAttribute('data-rows', '2')
    expect(screen.queryByTestId('link-chart')).not.toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Table' })).toHaveAttribute('aria-selected', 'true')
  })

  it('says what a link is and is not, and links to Connections starting from the first number', async () => {
    mount(graphAnswer)
    await screen.findByTestId('link-chart')
    expect(screen.getByText(/A link is a record of contact, not a finding about the people/)).toBeInTheDocument()
    expect(screen.getByText(/2 links between 3 numbers/)).toBeInTheDocument()
    const open = screen.getByRole('link', { name: 'Open in Connections' })
    expect(open.getAttribute('href')).toBe('/cases/case-a/connections?t=923000001001')
  })

  it('names the number chosen on the chart with its carrier', async () => {
    mount(graphAnswer)
    await userEvent.click(await screen.findByRole('button', { name: 'pick first number' }))
    expect(await screen.findByText('0300 0001001')).toBeInTheDocument()
  })

  it('opens a map answer on the map, says a cell site is a coverage area, and links to Places', async () => {
    mount(mapAnswer)
    expect(await screen.findByTestId('geo-map')).toHaveAttribute('data-sites', '2')
    expect(screen.getByRole('tab', { name: 'Map' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByText(/not the position of a person/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open in Places' }).getAttribute('href')).toBe('/cases/case-a/timeline?view=places')
  })

  it('works outside a router: the views are shown and the shortcuts to other pages are left out', async () => {
    render(<ResultViews presentation={graphAnswer} caseId="case-a" />)
    expect(await screen.findByTestId('link-chart')).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Open in Connections' })).not.toBeInTheDocument()
  })
})
