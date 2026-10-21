import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { listCaseRecords, updateCaseRecord } from './apiClient.js'
import { loadCaseRecords, resetCaseRecordsForTest } from './useCaseRecords.js'
import { CaseTitle } from '../components/CaseTitle.jsx'
import { CaseNameEditor } from '../components/CaseNameEditor.jsx'

function configure(config) {
  globalThis.window.__INVESTIGATION_WORKSPACE_CONFIG__ = config
}

function reply(body, { status = 200 } = {}) {
  return { ok: status >= 200 && status < 300, status, headers: { get: () => null }, json: async () => body }
}

const NAMED = { case_id: 'case-a', display_name: 'Harbour Road', status: 'on_hold', recorded: true, evidence_count: 4 }
const PLAIN = { case_id: 'case-b', display_name: null, status: 'open', recorded: false, evidence_count: 2 }

const realFetch = globalThis.fetch

beforeEach(() => {
  resetCaseRecordsForTest()
  configure({ tenantId: 'default' })
})
afterEach(() => {
  delete globalThis.window.__INVESTIGATION_WORKSPACE_CONFIG__
  globalThis.fetch = realFetch
})

describe('the case record calls', () => {
  it('lists every case of the tenant without naming a collection', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(reply({ cases: [NAMED, PLAIN] }))
    const data = await listCaseRecords({ fetchImpl })
    const [url, init] = fetchImpl.mock.calls[0]
    expect(String(url)).toContain('/api/cases?tenant_id=default')
    expect(init.headers['X-Forensic-Collection-ID']).toBeUndefined()
    expect(init.headers['X-Forensic-Tenant-ID']).toBe('default')
    expect(data.cases).toHaveLength(2)
  })

  it('changes one case with PATCH, scoped to that case, and sends only what was asked', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(reply({ ...NAMED, status: 'closed' }))
    await updateCaseRecord({ caseId: 'case-a', patch: { status: 'closed' }, fetchImpl })
    const [url, init] = fetchImpl.mock.calls[0]
    expect(String(url)).toContain('/api/cases/case-a?tenant_id=default')
    expect(init.method).toBe('PATCH')
    expect(init.headers['X-Forensic-Collection-ID']).toBe('case-a')
    expect(JSON.parse(init.body)).toEqual({ status: 'closed' })
  })

  it('shows the service sentence when a change is refused, and a plain one for any other failure', async () => {
    const refused = vi.fn().mockResolvedValue(reply({ error: 'display_name is longer than 120 characters' }, { status: 400 }))
    await expect(updateCaseRecord({ caseId: 'case-a', patch: { display_name: 'x' }, fetchImpl: refused })).rejects.toThrow('longer than 120')
    const broken = vi.fn().mockResolvedValue(reply({ error: 'database exploded' }, { status: 500 }))
    await expect(updateCaseRecord({ caseId: 'case-a', patch: { status: 'open' }, fetchImpl: broken })).rejects.toThrow('Nothing was changed')
  })
})

describe('the shared copy', () => {
  it('keeps an empty map when the service has no endpoint or the read fails, so identifiers show as before', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(reply({ error: 'not found' }, { status: 404 }))
    expect(await loadCaseRecords({ fetchImpl })).toEqual({})
    const down = vi.fn().mockRejectedValue(new Error('network'))
    expect(await loadCaseRecords({ force: true, fetchImpl: down })).toEqual({})
  })

  it('reads once for the whole page', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(reply({ cases: [NAMED] }))
    await Promise.all([loadCaseRecords({ fetchImpl }), loadCaseRecords({ fetchImpl })])
    await loadCaseRecords({ fetchImpl })
    expect(fetchImpl).toHaveBeenCalledTimes(1)
  })
})

describe('CaseTitle', () => {
  it('shows the identifier when there is no name, and the name (identifier in the tooltip) when there is one', async () => {
    const fetchImpl = vi.fn().mockResolvedValue(reply({ cases: [NAMED, PLAIN] }))
    globalThis.fetch = fetchImpl
    render(<><span data-testid="named"><CaseTitle caseId="case-a" showId /></span><span data-testid="plain"><CaseTitle caseId="case-b" /></span></>)
    expect(screen.getByTestId('plain').textContent).toBe('case-b')
    await waitFor(() => expect(screen.getByTestId('named').textContent).toContain('Harbour Road'))
    expect(screen.getByTestId('named').textContent).toContain('case-a')
    expect(screen.getByText('Harbour Road').getAttribute('title')).toBe('case-a')
    expect(screen.getByTestId('plain').textContent).toBe('case-b')
  })

  it('renders a name as text, never as markup', async () => {
    const hostile = { ...NAMED, display_name: '<img src=x onerror=alert(1)>' }
    globalThis.fetch = vi.fn().mockResolvedValue(reply({ cases: [hostile] }))
    const { container } = render(<CaseTitle caseId="case-a" />)
    await waitFor(() => expect(container.textContent).toContain('<img src=x'))
    expect(container.querySelector('img')).toBeNull()
  })
})

describe('CaseNameEditor', () => {
  it('saves a name and a status, then every title shows the new name', async () => {
    const saved = { ...NAMED, display_name: 'Pier Nine', status: 'closed' }
    const fetchImpl = vi.fn()
      .mockResolvedValueOnce(reply({ cases: [PLAIN, { ...NAMED, case_id: 'case-a', display_name: null, status: 'open', recorded: false }] }))
      .mockResolvedValueOnce(reply(saved))
    globalThis.fetch = fetchImpl
    render(<><CaseNameEditor caseId="case-a" /><span data-testid="title"><CaseTitle caseId="case-a" /></span></>)
    await waitFor(() => expect(fetchImpl).toHaveBeenCalledTimes(1))
    fireEvent.click(screen.getByRole('button', { name: /name this case/i }))
    fireEvent.change(screen.getByLabelText('Case name'), { target: { value: '  Pier Nine ' } })
    fireEvent.change(screen.getByLabelText('Status'), { target: { value: 'closed' } })
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Save' })) })
    await waitFor(() => expect(screen.getByTestId('title').textContent).toBe('Pier Nine'))
    const [, init] = fetchImpl.mock.calls[1]
    expect(JSON.parse(init.body)).toEqual({ display_name: 'Pier Nine', status: 'closed' })
    expect(screen.getByText('Closed')).toBeTruthy()
  })

  it('keeps the form open and says why when the service refuses', async () => {
    const fetchImpl = vi.fn()
      .mockResolvedValueOnce(reply({ cases: [PLAIN] }))
      .mockResolvedValueOnce(reply({ error: 'display_name must not contain control characters or line breaks' }, { status: 400 }))
    globalThis.fetch = fetchImpl
    render(<CaseNameEditor caseId="case-b" />)
    await waitFor(() => expect(fetchImpl).toHaveBeenCalledTimes(1))
    fireEvent.click(screen.getByRole('button', { name: /name this case/i }))
    fireEvent.change(screen.getByLabelText('Case name'), { target: { value: 'bad' } })
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Save' })) })
    const alert = await screen.findByRole('alert')
    expect(alert.textContent).toContain('control characters')
    expect(screen.getByLabelText('Case name')).toBeTruthy()
  })
})
