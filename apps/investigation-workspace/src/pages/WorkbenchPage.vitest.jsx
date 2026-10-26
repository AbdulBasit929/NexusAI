import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import WorkbenchPage from './WorkbenchPage.jsx'
import { getConnections, getPlaces } from '../lib/apiClient.js'
import { clearWorkbenchReads } from '../lib/workbench/useWorkbench.js'
import connections from '../lib/__fixtures__/wb/connections.json'
import places from '../lib/__fixtures__/wb/places.json'

// The canvas libraries (Cytoscape, ECharts, MapLibre) are replaced by stubs that show what the page hands them and let a test "click" a mark; the charts themselves are checked in the browser.
vi.mock('./workbench/WbGraph.jsx', async () => {
  const React = await import('react')
  return {
    WbGraph: React.forwardRef(function WbGraphStub({ graph, highlight, onSelect }, ref) {
      React.useImperativeHandle(ref, () => ({ fit() {}, zoom() {}, png: () => null }))
      return (
        <div data-testid="graph" data-nodes={graph.nodes.map(n => n.id).join(',')} data-focus={graph.nodes.filter(n => n.focus).map(n => n.id).join(',')} data-highlight={(highlight?.nodes || []).join(',')}>
          {graph.nodes.map(n => <button key={n.id} type="button" onClick={() => onSelect({ type: 'node', id: n.id })}>{`node ${n.id}`}</button>)}
        </div>
      )
    }),
  }
})
vi.mock('./workbench/WbTimeline.jsx', () => ({ WbTimeline: ({ lanes, marks }) => <div data-testid="timeline" data-lanes={lanes.map(l => l.id).join(',')} data-marks={marks.length} /> }))
vi.mock('../components/GeoMap.jsx', () => ({ GeoMap: ({ sites }) => <div data-testid="map" data-sites={sites.length} /> }))
vi.mock('../lib/apiClient.js', async importOriginal => ({ ...(await importOriginal()), getConnections: vi.fn(), getPlaces: vi.fn() }))

const CASE = 'nexusai-wave1-dev2-20261021'
function Where() { const l = useLocation(); return <div data-testid="where">{l.pathname}{l.search}</div> }
function mount(path = `/cases/${CASE}/connections`) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/cases/:id/connections" element={<WorkbenchPage />} />
        <Route path="/cases/:id/investigate" element={<Where />} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('Connections: the whole case, with no question field', () => {
  beforeEach(() => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds: [CASE] }
    globalThis.fetch = vi.fn(async () => ({ ok: true, status: 200, headers: { get: () => 'application/json' }, json: async () => ({ summary: {}, record_families: [], recent_evidence: [], recent_jobs: [] }) }))
    clearWorkbenchReads()
    getConnections.mockReset(); getPlaces.mockReset()
    getConnections.mockResolvedValue(connections)
    getPlaces.mockResolvedValue(places)
  })
  afterEach(() => { vi.restoreAllMocks(); delete window.__INVESTIGATION_WORKSPACE_CONFIG__ })

  it('draws every subscriber and the contacts they share, says what the page is, and has no field to type a question in', async () => {
    mount()
    const graph = await screen.findByTestId('graph')
    const ids = graph.dataset.nodes.split(',')
    expect(connections.subjects.every(s => ids.includes(s.number))).toBe(true)
    expect(ids.length).toBeGreaterThan(connections.subjects.length)
    expect(screen.getByRole('heading', { level: 1, name: 'Connections' })).toBeInTheDocument()
    expect(screen.getByText(/across all the evidence of this case/)).toBeInTheDocument()
    expect(screen.queryByRole('textbox', { name: /ask a question/i })).not.toBeInTheDocument()
    expect(getConnections).toHaveBeenCalledWith(expect.objectContaining({ caseId: CASE, perSubject: 60 }))
    expect(screen.getByTestId('timeline').dataset.lanes).toBe('all')
    // the hint about services and hubs is stated, not hidden
    expect(screen.getByText(/excluded, not hidden/)).toBeInTheDocument()
  })

  it('opens the evidence of a number from the links and says what is not known about it', async () => {
    mount()
    await userEvent.click(await screen.findByRole('button', { name: 'node 923000001003' }))
    const panel = await screen.findByRole('complementary', { name: /Evidence: / })
    expect(within(panel).getByText('Observed')).toBeInTheDocument()
    expect(within(panel).getAllByText('Events').length).toBeGreaterThan(0)
    expect(within(panel).getByText(/not a finding about the people/)).toBeInTheDocument()
    expect(screen.getByTestId('graph').dataset.highlight).toBe('923000001003')
    expect(within(panel).getAllByRole('row').length).toBeGreaterThan(2)
  })

  it('takes questions to the Ask page, which is where they are asked, with the numbers on screen in them', async () => {
    mount()
    await screen.findByTestId('graph')
    await userEvent.click(within(screen.getByRole('heading', { level: 1, name: 'Connections' }).closest('header')).getByRole('link', { name: /Ask this case/ }))
    expect(screen.getByTestId('where').textContent).toBe(`/cases/${CASE}/investigate`)
  })

  it('offers questions of forms the engine answers, and sends the chosen one to the Ask page to be run', async () => {
    mount()
    await screen.findByTestId('graph')
    await userEvent.click(screen.getByText('Ask about this case'))
    await userEvent.click(screen.getByRole('button', { name: /How are two numbers connected\?/ }))
    const where = screen.getByTestId('where').textContent
    expect(where.startsWith(`/cases/${CASE}/investigate?question=`)).toBe(true)
    expect(decodeURIComponent(where.replace(/\+/g, ' '))).toMatch(/How are 9230000010\d\d and 9230000010\d\d connected\?&run=1$/)
  })

  it('reads the numbers of an older address (?t=) as the numbers to start from', async () => {
    mount(`/cases/${CASE}/connections?t=923000001003,923000001007`)
    const graph = await screen.findByTestId('graph')
    expect(graph.dataset.focus.split(',').sort()).toEqual(['923000001003', '923000001007'])
    expect(graph.dataset.nodes.split(',').length).toBeLessThan(15)
  })

  it('switches the pane to the table, which lists the subscribers and opens one on a click', async () => {
    mount()
    await screen.findByTestId('graph')
    await userEvent.click(screen.getByRole('tab', { name: /^Table/ }))
    const table = screen.getByRole('table', { name: 'Subscribers of this case' })
    expect(within(table).getAllByRole('row')).toHaveLength(1 + connections.subjects.length)
    await userEvent.click(within(table).getAllByRole('row')[1])
    expect(await screen.findByRole('complementary', { name: /Evidence: / })).toBeInTheDocument()
  })

  it('shows the map pane when the places are read, and says why when they are not', async () => {
    mount()
    await screen.findByTestId('graph')
    await userEvent.click(screen.getByRole('tab', { name: /^Places/ }))
    expect(await screen.findByTestId('map')).toBeInTheDocument()
    expect(screen.getByText(/Synthetic placement/)).toBeInTheDocument()
  })

  it('says why a case cannot be drawn, and draws nothing over part of the records', async () => {
    getConnections.mockResolvedValue({ available: false, reason: 'The case has call-record files that are not typed yet.' })
    mount()
    expect(await screen.findByText('This case cannot be drawn as connections yet')).toBeInTheDocument()
    expect(screen.getByText(/not typed yet/)).toBeInTheDocument()
    expect(screen.queryByTestId('graph')).not.toBeInTheDocument()
  })

  it('still draws the links when only the places cannot be read, and says so on the Places pane', async () => {
    getPlaces.mockRejectedValue(Object.assign(new Error('x'), { reference: 'HTTP-500' }))
    mount()
    await screen.findByTestId('graph')
    await userEvent.click(screen.getByRole('tab', { name: /^Places/ }))
    expect(await screen.findByText('The places could not be drawn')).toBeInTheDocument()
    expect(screen.getByText(/HTTP-500/)).toBeInTheDocument()
  })

  it('reports a failed request with its reference, and can try again', async () => {
    getConnections.mockRejectedValueOnce(Object.assign(new Error('x'), { reference: 'HTTP-500' }))
    mount()
    expect(await screen.findByText('The connections could not be read')).toBeInTheDocument()
    expect(screen.getByText(/HTTP-500/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))
    await waitFor(() => expect(screen.getByTestId('graph')).toBeInTheDocument())
  })
})
