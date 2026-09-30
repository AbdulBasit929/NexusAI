import { afterEach, describe, expect, it, vi } from 'vitest'
import { askCase, getQueryCapabilities, listEvidence, uploadEvidence } from './apiClient.js'

// The security property this file exists to protect: in the default
// configuration the browser holds NO credential, and none is ever sent from
// it. A long-lived API key in browser storage is a credential an analyst's
// machine can leak, and this product's evidence is read in court.

function configure(config) {
  globalThis.window.__INVESTIGATION_WORKSPACE_CONFIG__ = config
}

afterEach(() => { delete globalThis.window.__INVESTIGATION_WORKSPACE_CONFIG__ })

function okResponse() {
  return {
    ok: true,
    status: 200,
    headers: { get: () => null },
    json: async () => ({}),
  }
}

describe('connection mode', () => {
  it('sends NO Authorization header when no token is configured', async () => {
    configure({ tenantId: 'default' })
    const fetchImpl = vi.fn().mockResolvedValue(okResponse())

    await askCase({ caseId: 'case-a', query: 'how many records', fetchImpl })

    const [url, init] = fetchImpl.mock.calls[0]
    expect(init.headers.Authorization).toBeUndefined()
    // and it must be same-origin through the proxy, not a cross-origin call
    // that would need a credential to succeed
    expect(String(url)).toContain('/api/query/hybrid')
  })

  it('still carries the forensic scope headers in proxy mode', async () => {
    configure({ tenantId: 'default', collectionId: 'nexusai-forensic-demo' })
    const fetchImpl = vi.fn().mockResolvedValue(okResponse())

    await askCase({ caseId: 'case-a', query: 'q', fetchImpl })

    const init = fetchImpl.mock.calls[0][1]
    expect(init.headers['X-Forensic-Tenant-ID']).toBe('default')
    expect(init.headers['X-Forensic-Collection-ID']).toBe('nexusai-forensic-demo')
    expect(init.headers['X-Forensic-Actor-Role']).toBeTruthy()
  })

  it('rejects a cross-origin API override instead of creating a browser-direct mode', async () => {
    configure({ tenantId: 'default', apiBaseUrl: 'http://localhost:8091' })
    const fetchImpl = vi.fn().mockResolvedValue(okResponse())

    await expect(askCase({ caseId: 'case-a', query: 'q', fetchImpl })).rejects.toMatchObject({
      name: 'WorkspaceConfigurationError',
      reference: 'CFG-PROXY',
    })

    expect(fetchImpl).not.toHaveBeenCalled()
  })

  it('uploads through the proxy without a credential too', async () => {
    configure({ tenantId: 'default' })
    const sent = { headers: {} }
    class FakeXHR {
      constructor() { this.upload = { addEventListener: () => {} } }
      open(_method, url) { sent.url = String(url) }
      setRequestHeader(name, value) { sent.headers[name] = value }
      addEventListener(name, handler) { if (name === 'load') this.onLoad = handler }
      send() { this.status = 200; this.responseText = '{}'; this.getResponseHeader = () => null; this.onLoad() }
    }

    await uploadEvidence({
      caseId: 'case-a',
      file: new File(['x'], 'a.csv'),
      xhrImpl: FakeXHR,
    })

    expect(sent.url).toContain('/api/webhooks/records/upload')
    expect(sent.headers.Authorization).toBeUndefined()
    expect(sent.headers['X-Forensic-Tenant-ID']).toBe('default')
  })

  it('sends evidence catalog filters and pagination to the authoritative endpoint', async () => {
    configure({ tenantId: 'default' })
    const fetchImpl = vi.fn().mockResolvedValue(okResponse())

    await listEvidence({
      caseId: 'case-a',
      filters: { detectedType: 'cdr', processingStatus: 'failed', q: 'calls', limit: 25, offset: 50 },
      fetchImpl,
    })

    const url = fetchImpl.mock.calls[0][0]
    expect(url.searchParams.get('detected_type')).toBe('cdr')
    expect(url.searchParams.get('processing_status')).toBe('failed')
    expect(url.searchParams.get('q')).toBe('calls')
    expect(url.searchParams.get('limit')).toBe('25')
    expect(url.searchParams.get('offset')).toBe('50')
  })

  it('loads collection-scoped query capabilities through the same-origin proxy', async () => {
    configure({ tenantId: 'tenant-a' })
    const fetchImpl = vi.fn().mockResolvedValue(okResponse())

    await getQueryCapabilities({ caseId: 'case-a', fetchImpl })

    const [url, init] = fetchImpl.mock.calls[0]
    expect(String(url)).toContain('/api/query/capabilities')
    expect(url.searchParams.get('tenant_id')).toBe('tenant-a')
    expect(url.searchParams.get('collection_id')).toBe('case-a')
    expect(init.headers.Authorization).toBeUndefined()
  })
})
