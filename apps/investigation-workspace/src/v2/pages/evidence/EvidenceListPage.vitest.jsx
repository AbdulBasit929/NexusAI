import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'

const listEvidence = vi.fn()
const getKnownFlags = vi.fn(async () => null) // a server without known sets answers 404, which the client turns into null
const createKnownSet = vi.fn()
const addKnownHashes = vi.fn()
const retireKnownSet = vi.fn()
vi.mock('../../../lib/apiClient.js', () => ({
  listEvidence: (...args) => listEvidence(...args), getEvidence: vi.fn(async () => ({})), getEvidenceContent: vi.fn(async () => new Blob()), uploadEvidence: vi.fn(),
  getKnownFlags: (...args) => getKnownFlags(...args), createKnownSet: (...args) => createKnownSet(...args), addKnownHashes: (...args) => addKnownHashes(...args), retireKnownSet: (...args) => retireKnownSet(...args),
}))
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

describe('known sets on the evidence list', () => {
  const pictures = [
    { evidence_id: 'dddddddd-4444', original_filename: 'photo-a.jpg', modality: 'image', detected_type: 'image', size_bytes: 52000, created_at: hoursAgo(1), processing_status: 'completed' },
    { evidence_id: 'eeeeeeee-5555', original_filename: 'logo.png', modality: 'image', detected_type: 'image', size_bytes: 8000, created_at: hoursAgo(2), processing_status: 'completed' },
    { evidence_id: 'ffffffff-6666', original_filename: 'scene.jpg', modality: 'image', detected_type: 'image', size_bytes: 91000, created_at: hoursAgo(3), processing_status: 'completed' },
  ]
  const imageSummary = { evidence_total: 3, completed: 3, failed: 0, processing: 0, queued: 0, total_size_bytes: '151000', modality_counts: { image: 3 } }
  const flagsResponse = (flags, sets = []) => ({ contract_version: 'forensics.image-known-sets/v1', flags, sets, flagged_pictures: new Set(flags.map(flag => flag.evidence_id)).size, coverage: { statement: 'Checked 3 of 3 images.' } })
  const watch = { evidence_id: 'dddddddd-4444', set_id: 's1', set_name: 'Case 12 watch', set_kind: 'of_interest', tier: 'identical', distance: 0 }
  const harmless = { evidence_id: 'eeeeeeee-5555', set_id: 's2', set_name: 'Stock', set_kind: 'known_harmless', tier: 'same_image', distance: 3 }
  const sets = [{ set_id: 's1', name: 'Case 12 watch', kind: 'of_interest', hashes: 4 }, { set_id: 's2', name: 'Stock', kind: 'known_harmless', hashes: 9 }]
  const cardTitles = () => screen.getAllByRole('listitem').filter(node => node.className.includes('nx-evcard')).map(node => node.querySelector('.nx-evcard__link').getAttribute('aria-label'))

  beforeEach(() => {
    localStorage.clear()
    listEvidence.mockImplementation(async () => ({ items: pictures, summary: imageSummary, pagination: { evidence_total: 3, returned: 3, has_next: false, has_previous: false } }))
    getKnownFlags.mockImplementation(async () => null)
  })
  afterEach(() => { vi.clearAllMocks() })

  it('leaves the feature out when the server has no known sets', async () => {
    await open()
    await screen.findByRole('link', { name: 'photo-a.jpg' })
    expect(screen.queryByRole('button', { name: /Known sets/ })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Select' })).toBeNull()
  })

  it('shows the set on a flagged card, and hides pictures that only a known-harmless set flags when asked, never one on a watch list', async () => {
    getKnownFlags.mockImplementation(async () => flagsResponse([watch, harmless], sets))
    await open()
    expect(await screen.findByText('Case 12 watch', { selector: '.nx-evcard__flag span' })).toBeTruthy()
    expect(screen.getByText('Stock', { selector: '.nx-evcard__flag span' })).toBeTruthy()
    expect(cardTitles()).toEqual(['photo-a.jpg', 'logo.png', 'scene.jpg'])
    await userEvent.click(screen.getByRole('button', { name: /Hide known harmless/ }))
    expect(cardTitles()).toEqual(['photo-a.jpg', 'scene.jpg'])
    expect(screen.getByText(/picture is hidden because a known-harmless set flags it/)).toBeTruthy()
    await userEvent.click(screen.getByRole('button', { name: 'Show it' }))
    expect(cardTitles()).toEqual(['photo-a.jpg', 'logo.png', 'scene.jpg'])
  })

  it('brings the flagged pictures to the front with one chip, counted for the page, and back again', async () => {
    getKnownFlags.mockImplementation(async () => flagsResponse([watch, harmless], sets))
    await open()
    await screen.findByRole('link', { name: 'photo-a.jpg' })
    expect(cardTitles()).toEqual(['photo-a.jpg', 'logo.png', 'scene.jpg'])
    const chip = screen.getByRole('button', { name: /^Flagged/ })
    expect(chip).toHaveTextContent('2')
    await userEvent.click(chip)
    expect(cardTitles()).toEqual(['photo-a.jpg', 'logo.png'])
    expect(screen.getByRole('button', { name: /^Flagged/ })).toHaveAttribute('aria-pressed', 'true')
    await userEvent.click(screen.getByRole('button', { name: /^Flagged/ }))
    expect(cardTitles()).toEqual(['photo-a.jpg', 'logo.png', 'scene.jpg'])
  })

  it('picks pictures on their cards without opening them, and makes a set from them in the panel', async () => {
    getKnownFlags.mockImplementation(async () => flagsResponse([], []))
    createKnownSet.mockResolvedValue({ set: { set_id: 'new-set', name: 'Banners', kind: 'known_harmless' } })
    addKnownHashes.mockResolvedValue({ added: 2, already_in_set: 0, rejected_count: 0 })
    await open()
    await screen.findByRole('link', { name: 'photo-a.jpg' })
    await userEvent.click(screen.getByRole('button', { name: 'Select' }))
    await userEvent.click(screen.getByRole('button', { name: 'Select photo-a.jpg' }))
    await userEvent.click(screen.getByRole('link', { name: 'logo.png' })) // a click on a card while picking picks it; it does not navigate
    expect(screen.queryByText('the file')).toBeNull()
    expect(screen.getByRole('button', { name: 'Deselect logo.png' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByText('2 selected')).toBeTruthy()
    await userEvent.click(screen.getByRole('button', { name: 'Add to a set…' }))
    const panel = await screen.findByRole('dialog', { name: 'Known sets' })
    await userEvent.type(within(panel).getByLabelText('Name'), 'Banners')
    await userEvent.selectOptions(within(panel).getByLabelText(/^Kind/), 'known_harmless')
    await userEvent.click(within(panel).getByRole('button', { name: 'Create and add 2 selected' }))
    await waitFor(() => expect(addKnownHashes).toHaveBeenCalledTimes(1))
    expect(createKnownSet).toHaveBeenCalledWith(expect.objectContaining({ caseId: 'orion', name: 'Banners', kind: 'known_harmless' }))
    expect(addKnownHashes).toHaveBeenCalledWith(expect.objectContaining({ setId: 'new-set', evidenceIds: ['dddddddd-4444', 'eeeeeeee-5555'] }))
    expect(await within(panel).findByText('Set "Banners" created. 2 hashes added.')).toBeTruthy()
    expect(getKnownFlags.mock.calls.length).toBeGreaterThan(1) // the flags are read again after the change
  })

  it('imports a table into a chosen set and says which rows could not be used', async () => {
    getKnownFlags.mockImplementation(async () => flagsResponse([], sets))
    addKnownHashes.mockResolvedValue({ added: 1, already_in_set: 0, rejected_count: 1, rejected: [{ line: 3, reason: 'x' }] })
    await open()
    await userEvent.click(await screen.findByRole('button', { name: /Known sets/ }))
    const panel = await screen.findByRole('dialog', { name: 'Known sets' })
    expect(within(panel).getByText('Watch list')).toBeTruthy()
    expect(within(panel).getByText('4 hashes')).toBeTruthy()
    fireEvent.change(within(panel).getByPlaceholderText('sha256,pdq256,label'), { target: { value: 'sha256,label\nabc,x' } })
    await userEvent.click(within(panel).getByRole('button', { name: 'Import the table' }))
    await waitFor(() => expect(addKnownHashes).toHaveBeenCalledWith(expect.objectContaining({ setId: 's1', csv: 'sha256,label\nabc,x' })))
    expect(await within(panel).findByText('1 hash added, 1 row could not be used.')).toBeTruthy()
  })

  it('retires a set only after a second, explicit click', async () => {
    getKnownFlags.mockImplementation(async () => flagsResponse([], sets))
    retireKnownSet.mockResolvedValue({ retired: true })
    await open()
    await userEvent.click(await screen.findByRole('button', { name: /Known sets/ }))
    const panel = await screen.findByRole('dialog', { name: 'Known sets' })
    await userEvent.click(within(panel).getByRole('button', { name: 'Retire Case 12 watch' }))
    expect(retireKnownSet).not.toHaveBeenCalled()
    await userEvent.click(within(panel).getByRole('button', { name: 'Retire it' }))
    await waitFor(() => expect(retireKnownSet).toHaveBeenCalledWith(expect.objectContaining({ setId: 's1' })))
  })
})
