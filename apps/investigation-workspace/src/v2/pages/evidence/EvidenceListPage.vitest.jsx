import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'

const listEvidence = vi.fn()
vi.mock('../../../lib/apiClient.js', () => ({ listEvidence: (...args) => listEvidence(...args), getEvidence: vi.fn(async () => ({})), getEvidenceContent: vi.fn(async () => new Blob()), uploadEvidence: vi.fn() }))
vi.mock('../../shell/AppShell.jsx', () => ({ CaseShell: ({ children }) => <div>{children}</div> }))
vi.mock('./previewStore.js', () => ({
  cachedPreview: () => undefined,
  loadPreview: vi.fn(async ({ item }) => (item.modality === 'audio' ? { kind: 'wave', peaks: [0.2, 0.9, 0.5] } : { kind: 'none' })),
}))

const NOW = Date.now()
const hoursAgo = hours => new Date(NOW - hours * 3_600_000).toISOString()
const items = [
  { evidence_id: 'aaaaaaaa-1111', original_filename: 'voice-note-01.wav', modality: 'audio', detected_type: 'audio', size_bytes: 303800, created_at: hoursAgo(1), processing_status: 'completed', media_metadata: { duration_ms: 9720, channels: 1, sample_rate_hz: 16000 }, extension: 'wav' },
  { evidence_id: 'bbbbbbbb-2222', original_filename: 'wallet-W-wallet-t1.csv', modality: 'structured_records', detected_type: 'generic', size_bytes: 15800, created_at: hoursAgo(30), processing_status: 'completed', accepted_rows: 181, total_rows: 181, extension: 'csv' },
  { evidence_id: 'cccccccc-3333', original_filename: 'broken.xlsx', modality: 'tabular', detected_type: 'spreadsheetml_2003', size_bytes: 900, created_at: hoursAgo(31), processing_status: 'failed', errors: ['bad header'] },
]
const summary = { evidence_total: 3, completed: 2, failed: 1, processing: 0, queued: 0, total_size_bytes: '320600', modality_counts: { audio: 1, structured_records: 1, tabular: 1 } }

function answer(filters) {
  const wanted = items.filter(item => (!filters.modality || item.modality === filters.modality) && (!filters.processingStatus || (filters.processingStatus === 'failed' ? item.processing_status === 'failed' : item.processing_status === 'completed')) && (!filters.q || item.original_filename.includes(filters.q)))
  return { items: wanted, summary, pagination: { evidence_total: wanted.length, returned: wanted.length, has_next: false, has_previous: false } }
}

async function open(url = '/cases/orion/evidence') {
  const { default: EvidenceListPage } = await import('./EvidenceListPage.jsx')
  return render(<MemoryRouter initialEntries={[url]}><Routes><Route path="/cases/:id/evidence" element={<EvidenceListPage />} /><Route path="/cases/:id/evidence/:evidenceId" element={<p>the file</p>} /></Routes></MemoryRouter>)
}

describe('the evidence list', () => {
  beforeEach(() => {
    localStorage.clear()
    listEvidence.mockImplementation(async ({ filters }) => answer(filters))
  })
  afterEach(() => { vi.clearAllMocks() })

  it('shows every file as a card with its kind, its fact and a way in, grouped by day, and says how many the case holds', async () => {
    await open()
    expect(await screen.findByRole('link', { name: 'voice-note-01.wav' })).toHaveAttribute('href', '/cases/orion/evidence/aaaaaaaa-1111')
    expect(screen.getByText(/^3 files · 313(\.\d)? KB · 2 ready · 1 failed$/)).toBeTruthy()
    const cards = screen.getAllByRole('listitem').filter(node => node.className.includes('nx-evcard'))
    expect(cards).toHaveLength(3)
    expect(within(cards[0]).getByText('Audio')).toBeTruthy()
    expect(within(cards[0]).getByText('0:10')).toBeTruthy()
    expect(within(cards[1]).getByText('181 rows')).toBeTruthy()
    expect(screen.getByRole('heading', { name: /Today/ })).toBeTruthy()
    expect(screen.getByRole('heading', { name: /Yesterday/ })).toBeTruthy()
  })

  it('draws the case as one bar whose legend is the filter, and asks the service for that kind only', async () => {
    await open()
    await screen.findByRole('link', { name: 'voice-note-01.wav' })
    const legend = screen.getByRole('group', { name: 'Show files of one kind' })
    expect(within(legend).getAllByRole('button').map(button => [button.querySelector('.nx-evband__name').textContent, button.querySelector('b').textContent, button.querySelector('.nx-evband__share').textContent])).toEqual([['Records', '1', '33%'], ['Audio', '1', '33%'], ['Spreadsheets', '1', '33%']])
    await userEvent.click(within(legend).getByRole('button', { name: /Audio/ }))
    await waitFor(() => expect(listEvidence).toHaveBeenLastCalledWith(expect.objectContaining({ filters: expect.objectContaining({ modality: 'audio' }) })))
    expect(await screen.findByRole('button', { name: /Audio 1/, pressed: true })).toBeTruthy()
    await userEvent.click(screen.getByRole('button', { name: 'Clear' }))
    await waitFor(() => expect(listEvidence).toHaveBeenLastCalledWith(expect.objectContaining({ filters: expect.objectContaining({ modality: undefined }) })))
  })

  it('a link that said family=audio still lands on the recordings', async () => {
    await open('/cases/orion/evidence?family=audio')
    await waitFor(() => expect(listEvidence).toHaveBeenCalledWith(expect.objectContaining({ filters: expect.objectContaining({ modality: 'audio' }) })))
  })

  it('opens a quick look from the eye and from the space bar, with the facts and the way in, and closes it with Escape', async () => {
    await open()
    const card = (await screen.findByRole('link', { name: 'voice-note-01.wav' })).closest('li')
    await userEvent.click(within(card).getByRole('button', { name: 'Quick look at voice-note-01.wav' }))
    const dialog = await screen.findByRole('dialog', { name: 'voice-note-01.wav' })
    expect(within(dialog).getAllByText('0:10').length).toBeGreaterThan(0)
    expect(within(dialog).getByText('Ready')).toBeTruthy()
    expect(within(dialog).getByRole('link', { name: /Open evidence/ })).toHaveAttribute('href', '/cases/orion/evidence/aaaaaaaa-1111')
    fireEvent.keyDown(dialog, { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    const link = screen.getByRole('link', { name: 'wallet-W-wallet-t1.csv' })
    link.focus()
    fireEvent.keyDown(link, { key: ' ' })
    expect(await screen.findByRole('dialog', { name: 'wallet-W-wallet-t1.csv' })).toBeTruthy()
  })

  it('shows a failed file as failed, with the service\'s reason on the word', async () => {
    await open()
    const card = (await screen.findByRole('link', { name: 'broken.xlsx' })).closest('li')
    expect(card).toHaveAttribute('data-status', 'failed')
    expect(within(card).getByText('Failed')).toHaveAttribute('title', 'bad header')
  })

  it('switches to a list, remembers the choice, and a row opens its file', async () => {
    await open()
    await screen.findByRole('link', { name: 'voice-note-01.wav' })
    await userEvent.click(screen.getByRole('button', { name: /List/ }))
    const table = await screen.findByRole('table')
    expect(within(table).getAllByRole('row').length).toBeGreaterThan(3)
    expect(localStorage.getItem('nexusai.evidence.view')).toBe('list')
    await userEvent.click(within(table).getByRole('link', { name: 'voice-note-01.wav' }))
    expect(await screen.findByText('the file')).toBeTruthy()
  })

  it('opens in the list when that was the last choice', async () => {
    localStorage.setItem('nexusai.evidence.view', 'list')
    await open()
    expect(await screen.findByRole('table')).toBeTruthy()
  })

  it('says so, and offers the way back, when nothing matches or the list could not be read', async () => {
    listEvidence.mockImplementation(async ({ filters }) => (filters.limit === 1 ? { summary } : { items: [], summary, pagination: { evidence_total: 0, returned: 0 } }))
    await open('/cases/orion/evidence?q=zzz')
    expect(await screen.findByRole('heading', { name: 'No file matches' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Show every file' })).toBeTruthy()
  })

  it('names a failed read and lets the analyst try again', async () => {
    listEvidence.mockImplementation(async ({ filters }) => { if (filters.limit === 1) return { summary }; throw Object.assign(new Error('x'), { status: 500 }) })
    await open()
    expect(await screen.findByRole('heading', { name: 'The evidence list could not be read' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Try again' })).toBeTruthy()
  })
})
