import { afterEach, describe, expect, it, vi } from 'vitest'
import { addKnownHashes, createKnownSet, getKnownFlags, listKnownSets, retireKnownSet } from './apiClient.js'

// The known-set calls go through the same same-origin proxy and carry the same scope as every other call; a read that the server does not know (404) is "not available", a failed write says why.

afterEach(() => { delete globalThis.window.__INVESTIGATION_WORKSPACE_CONFIG__ })
const configure = () => { globalThis.window.__INVESTIGATION_WORKSPACE_CONFIG__ = { tenantId: 'default', collectionId: 'orion-lahore' } }
const reply = (status, body = {}) => ({ ok: status >= 200 && status < 300, status, headers: { get: () => null }, json: async () => body })

describe('known sets client', () => {
  it('reads the flags of the case through the proxy, with the scope in the address and no credential', async () => {
    configure()
    const fetchImpl = vi.fn().mockResolvedValue(reply(200, { flags: [], sets: [] }))
    const data = await getKnownFlags({ caseId: 'orion', sets: ['s1', 's2'], fetchImpl })
    expect(data).toEqual({ flags: [], sets: [] })
    const [url, init] = fetchImpl.mock.calls[0]
    expect(url.pathname).toBe('/api/images/known')
    expect(url.searchParams.get('collection_id')).toBe('orion-lahore')
    expect(url.searchParams.get('tenant_id')).toBe('default')
    expect(url.searchParams.get('sets')).toBe('s1,s2')
    expect(init.method).toBe('GET')
    expect(init.headers.Authorization).toBeUndefined()
  })

  it('a read the server does not have is null, not an error', async () => {
    configure()
    const fetchImpl = vi.fn().mockResolvedValue(reply(404))
    expect(await getKnownFlags({ caseId: 'orion', fetchImpl })).toBeNull()
    expect(await listKnownSets({ caseId: 'orion', fetchImpl })).toBeNull()
  })

  it('creates a set and adds hashes with JSON bodies; only what was given is sent', async () => {
    configure()
    const fetchImpl = vi.fn().mockResolvedValue(reply(200, { added: 1 }))
    await createKnownSet({ caseId: 'orion', name: 'Watch', kind: 'of_interest', fetchImpl })
    let [url, init] = fetchImpl.mock.calls[0]
    expect(url.pathname).toBe('/api/images/known-sets')
    expect(init.method).toBe('POST')
    expect(init.headers['Content-Type']).toBe('application/json')
    expect(JSON.parse(init.body)).toEqual({ name: 'Watch', kind: 'of_interest', description: '' })

    await addKnownHashes({ caseId: 'orion', setId: 'abc/def', evidenceIds: ['e1', 'e2'], fetchImpl })
    ;[url, init] = fetchImpl.mock.calls[1]
    expect(url.pathname).toBe('/api/images/known-sets/abc%2Fdef/hashes')
    expect(JSON.parse(init.body)).toEqual({ evidence_ids: ['e1', 'e2'] })

    await addKnownHashes({ caseId: 'orion', setId: 's1', csv: 'sha256\nabc', fetchImpl })
    expect(JSON.parse(fetchImpl.mock.calls[2][1].body)).toEqual({ csv: 'sha256\nabc' })
  })

  it('retires a set with a POST and no body', async () => {
    configure()
    const fetchImpl = vi.fn().mockResolvedValue(reply(200, { retired: true }))
    await retireKnownSet({ caseId: 'orion', setId: 's1', fetchImpl })
    const [url, init] = fetchImpl.mock.calls[0]
    expect(url.pathname).toBe('/api/images/known-sets/s1/retire')
    expect(init.method).toBe('POST')
    expect(init.body).toBeUndefined()
  })

  it('a refused write carries the server sentence and the status', async () => {
    configure()
    const fetchImpl = vi.fn().mockResolvedValue(reply(409, { error: 'an active set with this name already exists' }))
    await expect(createKnownSet({ caseId: 'orion', name: 'Watch', kind: 'of_interest', fetchImpl })).rejects.toMatchObject({ message: 'an active set with this name already exists', status: 409 })
  })

  it('a write to a set the server does not know is an error, not null', async () => {
    configure()
    const fetchImpl = vi.fn().mockResolvedValue(reply(404, { error: 'no such set' }))
    await expect(addKnownHashes({ caseId: 'orion', setId: 'x', csv: 'a', fetchImpl })).rejects.toMatchObject({ status: 404 })
  })
})
