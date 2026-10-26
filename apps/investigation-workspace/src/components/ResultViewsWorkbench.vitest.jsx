import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { ResultViews } from './ResultViews.jsx'
import { presentInvestigationResponse } from '../lib/investigationPresentation.js'
import { getConnections, getPlaces } from '../lib/apiClient.js'
import { clearWorkbenchReads } from '../lib/workbench/useWorkbench.js'
import connections from '../lib/__fixtures__/wb/connections.json'
import places from '../lib/__fixtures__/wb/places.json'
import together from '../lib/__fixtures__/wb/answers/together.json'
import meeting from '../lib/__fixtures__/wb/answers/meeting.json'
import ranked from '../lib/__fixtures__/wb/answers/ranked.json'
import rhythm from '../lib/__fixtures__/wb/answers/rhythm.json'
import legacy from '../lib/__fixtures__/engine_ranked.json'

vi.mock('../pages/workbench/WbGraph.jsx', async () => {
  const React = await import('react')
  return {
    WbGraph: React.forwardRef(function WbGraphStub({ graph, highlight }, ref) {
      React.useImperativeHandle(ref, () => ({ fit() {}, zoom() {}, png: () => null }))
      return <div data-testid="graph" data-nodes={graph.nodes.map(n => n.id).join(',')} data-links={graph.links.map(l => l.kind).join(',')} data-highlight={(highlight?.nodes || []).join(',')} />
    }),
  }
})
vi.mock('../pages/workbench/WbTimeline.jsx', () => ({ WbTimeline: ({ marks, selectedMarkId }) => <div data-testid="timeline" data-marks={marks.map(m => m.id).join(',')} data-selected={selectedMarkId || ''} /> }))
vi.mock('./GeoMap.jsx', () => ({ GeoMap: ({ sites, track }) => <div data-testid="map" data-sites={sites.length} data-track={(track || []).length} /> }))
vi.mock('../lib/apiClient.js', async importOriginal => ({ ...(await importOriginal()), getConnections: vi.fn(), getPlaces: vi.fn() }))

const CASE = 'nexusai-wave1-dev2-20261021'
const QUESTIONS = {
  together: 'When were 923000001007 and 923000001011 at the same cell within 22 minutes of each other?',
  meeting: 'Which of the times 923000001003 and 923000001007 were at the same cell within 30 minutes could have been a meeting, given a call or SMS between them in the previous 90 minutes?',
  ranked: 'Who does 923000001000 contact most? Give the top five.',
  rhythm: 'Did the daily activity of 923000001006 change for good? From which day, and what were the daily averages before and after?',
}
const present = (response, key) => ({ ...presentInvestigationResponse(response, { caseId: CASE }), question: QUESTIONS[key] })
const mount = (presentation, onAsk) => render(<MemoryRouter><ResultViews presentation={presentation} caseId={CASE} onAsk={onAsk} /></MemoryRouter>)

describe('an answer of the Ask engine that can be drawn, inside the Ask page', () => {
  beforeEach(() => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { caseIds: [CASE] }
    clearWorkbenchReads()
    getConnections.mockReset(); getPlaces.mockReset()
    getConnections.mockResolvedValue(connections)
    getPlaces.mockResolvedValue(places)
  })
  afterEach(() => { vi.restoreAllMocks(); delete window.__INVESTIGATION_WORKSPACE_CONFIG__ })

  it('draws the windows of two numbers on the links, the places and the timeline, with the strength and the value the engine used', async () => {
    mount(present(together, 'together'))
    const graph = await screen.findByTestId('graph')
    expect(graph.dataset.links.split(',')).toContain('colocated')
    expect(screen.getByText('Inferred')).toBeInTheDocument()
    expect(screen.getByText('Co-location window')).toBeInTheDocument()
    expect(screen.getByText(/22 minutes, from your question/)).toBeInTheDocument()
    expect(screen.getByTestId('timeline').dataset.marks).toBe('window:0,window:1,window:2,window:3')
    // the table of the answer is a pane of the same picture, with the columns in the order of the query
    await userEvent.click(screen.getByRole('tab', { name: /^Table/ }))
    const table = screen.getByRole('table', { name: 'The answer' })
    expect(within(table).getAllByRole('row')).toHaveLength(1 + 4)
    expect(within(table).getAllByRole('columnheader')[0]).toHaveTextContent('Cell key')
  })

  it('opens the evidence of a mark from the keyboard, stepping through the marks in the order of time', async () => {
    mount(present(together, 'together'))
    await screen.findByTestId('graph')
    const picture = screen.getByLabelText('The answer on links, places and timeline')
    picture.focus()
    fireEvent.keyDown(picture, { key: ']' })
    const panel = await screen.findByRole('complementary', { name: /Evidence: Together at one cell site/ })
    expect(within(panel).getAllByText('2026-02-09 08:36').length).toBeGreaterThan(0)
    expect(within(panel).getAllByText('16 min').length).toBeGreaterThan(0)
    expect(within(panel).getByText(/Proximity is not association/)).toBeInTheDocument()
    expect(screen.getByTestId('timeline').dataset.selected).toBe('window:0')
    expect(screen.getByTestId('graph').dataset.highlight.split(',').sort()).toEqual(['923000001007', '923000001011'])
    fireEvent.keyDown(picture, { key: ']' })
    await waitFor(() => expect(screen.getByTestId('timeline').dataset.selected).toBe('window:1'))
    fireEvent.keyDown(picture, { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('complementary', { name: /Evidence: Together/ })).not.toBeInTheDocument())
  })

  it('marks candidates as candidates, and asks a follow-up through the Ask page', async () => {
    const onAsk = vi.fn()
    mount(present(meeting, 'meeting'), onAsk)
    await screen.findByTestId('graph')
    expect(screen.getByText('Candidate')).toBeInTheDocument()
    expect(screen.getByText('Possible meetings')).toBeInTheDocument()
    const picture = screen.getByLabelText('The answer on links, places and timeline')
    fireEvent.keyDown(picture, { key: ']' })
    const panel = await screen.findByRole('complementary', { name: /Evidence: A possible meeting/ })
    await userEvent.click(within(panel).getByText('Ask about this'))
    await userEvent.click(within(panel).getByRole('button', { name: /How are two numbers connected\?/ }))
    expect(onAsk).toHaveBeenCalledWith(expect.stringMatching(/^How are 9230000010\d\d and 9230000010\d\d connected\?$/))
  })

  it('draws a ranked list as the contacts the answer names, linked to the number asked about, and a change of rhythm as a mark with a line on the timeline', async () => {
    mount(present(ranked, 'ranked'))
    const graph = await screen.findByTestId('graph')
    expect(graph.dataset.nodes).toContain('923000001018')
    expect(screen.getByText('Observed')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /All connections of/ })).toHaveAttribute('href', `/cases/${CASE}/connections?t=923000001000`)
  })

  it('keeps the answer a table when it names nothing to draw (a count) or is not the engine\'s', async () => {
    const count = structuredClone(rhythm)
    count.enterprise.data_grid = { columns: [{ header: 'Value', key: 'value' }], rows: [{ value: 3 }], count: 1 }
    count.enterprise.derivation = { ...count.enterprise.derivation, views: ['v_comm_events'] }
    count.answer.ask_engine_v2.columns = ['value']
    mount(present(count, 'rhythm'))
    expect(screen.queryByTestId('graph')).not.toBeInTheDocument()
    expect(getConnections).not.toHaveBeenCalled()
    mount({ ...presentInvestigationResponse(legacy, { caseId: CASE }), question: 'x' })
    expect(getConnections).not.toHaveBeenCalled()
  })

  it('falls back to the answer\'s table, and says so, when the case cannot be read for the picture', async () => {
    getConnections.mockRejectedValue(Object.assign(new Error('x'), { reference: 'HTTP-502' }))
    mount(present(together, 'together'))
    expect(await screen.findByText(/The picture of this case could not be read/)).toBeInTheDocument()
    expect(screen.queryByTestId('graph')).not.toBeInTheDocument()
  })
})
