import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { ProofRecords, ProofSources } from './Proof.jsx'
import { EvidenceDrawer } from './EvidenceDrawer.jsx'

const row = (key, cells, extra = {}) => ({ key, cells, source_file: 'calls.xlsx', row_number: 12, evidence_id: 'e1', version_id: 'v1', row_hash: 'h1', ...extra })
const itemised = (over = {}) => ({ state: 'itemised', kind: 'rows', total_rows: 3, files: [{ evidence_id: 'e1', source_file: 'calls.xlsx', rows: 3 }], columns: ['When', 'Kind'], rows: [row('a', ['2026-02-02', 'sms']), row('b', ['2026-02-03', 'voice'], { row_number: 0 })], reconciled: 'yes', say: 'the calls listed', ...over })
const inRouter = ui => render(<MemoryRouter>{ui}</MemoryRouter>)

describe('the records behind a figure', () => {
  afterEach(() => cleanup())
  it('says how many records in how many files, whether the figure was recomputed, and lists them with a link to the row', () => {
    inRouter(<ProofRecords evidence={itemised()} caseId="case-a" />)
    expect(screen.getByText('3 records in 1 file')).toBeTruthy()
    expect(screen.getByText('Recomputed from the records: matches')).toBeTruthy()
    expect(screen.getByRole('link', { name: 'calls.xlsx' }).getAttribute('href')).toBe('/cases/case-a/evidence/e1')
    const table = screen.getByRole('table')
    expect(within(table).getAllByRole('columnheader').map(cell => cell.textContent.trim())).toEqual(['When', 'Kind', 'In your file'])
    const link = within(table).getByRole('link', { name: /calls\.xlsx · row 12/ })
    expect(link.getAttribute('href')).toBe('/cases/case-a/evidence/e1?version=v1&row=12&row_hash=h1')
    // a record that could not be placed in its file says so and offers no link
    expect(within(table).getByText(/row not found/)).toBeTruthy()
  })
  it('says the records are not listed, and why, when the engine did not itemise them', () => {
    inRouter(<ProofRecords evidence={{ state: 'not_itemised', reason: 'the answer is a ratio of two counts' }} caseId="case-a" />)
    expect(screen.getByRole('note').textContent).toMatch(/The records behind this answer are not listed\. The answer is a ratio of two counts\./)
  })
  it('lists the lines of an answer, shows the first line’s records at once and fetches another line when it is chosen', async () => {
    const evidence = { state: 'itemised', kind: 'lines', lines_total: 2, lines_token: 't1', total_rows: 5, files: [], columns: ['When'], rows: [row('a', ['2026-02-02'])], reconciled: 'yes',
      lines: [{ index: 1, label: 'A → B', records: 5, files: 1, reconciled: 'yes' }, { index: 2, label: 'A → C', records: 2, files: 1, reconciled: 'no', pending: false }] }
    const fetchLine = vi.fn(async () => ({ evidence: { files: [], columns: ['When'], rows: [row('z', ['2026-03-09'])], total_rows: 2 } }))
    inRouter(<ProofRecords evidence={evidence} caseId="case-a" fetchLine={fetchLine} />)
    const lines = within(screen.getByRole('list', { name: 'Lines of this answer' })).getAllByRole('button')
    expect(lines.map(button => button.getAttribute('aria-pressed'))).toEqual(['true', 'false'])
    expect(screen.getByRole('heading', { name: /Records behind line 1: A → B/ })).toBeTruthy()
    expect(fetchLine).not.toHaveBeenCalled()
    fireEvent.click(lines[1])
    await waitFor(() => expect(fetchLine).toHaveBeenCalledWith(expect.objectContaining({ caseId: 'case-a', token: 't1', line: 2 })))
    expect(await screen.findByText('2026-03-09')).toBeTruthy()
    expect(screen.getByRole('heading', { name: /Records behind line 2: A → C/ })).toBeTruthy()
  })
})

describe('the sources of an answer', () => {
  afterEach(() => cleanup())
  it('lists each file with its share of the records and the places to open, as links inside the app', () => {
    const citations = { groups: [{ label: 'calls.xlsx', href: '/cases/c/evidence/e1', totalContributing: 75, contributingCount: 25, strength: { id: 'strong', label: 'Source record' }, samples: [{ href: '/cases/c/evidence/e1?row=3', detail: 'Row 3' }] }] }
    inRouter(<ProofSources citations={citations} />)
    expect(screen.getByText('25 of 75 contributing rows')).toBeTruthy()
    expect(screen.getByText('Source record')).toBeTruthy()
    expect(screen.getByRole('link', { name: /Place 1/ }).getAttribute('href')).toBe('/cases/c/evidence/e1?row=3')
  })
  it('says no source could be opened when there is none', () => {
    inRouter(<ProofSources citations={{ groups: [] }} />)
    expect(screen.getByRole('note').textContent).toMatch(/No openable source/)
  })
})

describe('the evidence panel', () => {
  beforeEach(() => { localStorage.clear() })
  afterEach(() => cleanup())
  const turn = { id: 't', query: 'Who called whom?', presentation: { state: 'answered', original: { enterprise: { evidence: itemised() } }, citations: { groups: [] }, limitations: ['A limit'], derivation: '', method: null } }
  it('can be made wider and remembers the choice, and closes with Escape', () => {
    const onClose = vi.fn()
    const { container } = inRouter(<EvidenceDrawer turn={turn} caseId="case-a" onClose={onClose} />)
    const panel = container.querySelector('.nx-drawer')
    expect(panel.className).not.toMatch(/nx-drawer--wide/)
    fireEvent.click(screen.getByRole('button', { name: 'Make the evidence panel wider' }))
    expect(panel.className).toMatch(/nx-drawer--wide/)
    expect(localStorage.getItem('nexusai.viewer.drawer.wide')).toBe('1')
    expect(screen.getByRole('button', { name: 'Make the evidence panel narrower' }).getAttribute('aria-pressed')).toBe('true')
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onClose).toHaveBeenCalled()
    expect(screen.getByText('A limit')).toBeTruthy()
  })
})
