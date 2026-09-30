export class WorkspaceConfigurationError extends Error {
  constructor(message, reference = 'CFG-AUTH') {
    super(message)
    this.name = 'WorkspaceConfigurationError'
    this.reference = reference
  }
}

function runtimeConfig() {
  return globalThis.window?.__INVESTIGATION_WORKSPACE_CONFIG__ || {}
}

// ONE CONNECTION MODE: the same-origin proxy.
//
//   The browser holds NO credential. Requests go to a same-origin path and a
//   proxy attaches the forensic API key server-side. Nothing sensitive reaches
//   the bundle, page source, localStorage or a browser devtools session.
function proxyBasePath(config) {
  const basePath = config.apiBaseUrl ?? '/api'
  if (typeof basePath !== 'string' || !basePath.startsWith('/') || basePath.startsWith('//')) {
    throw new WorkspaceConfigurationError('The analyst workspace requires a same-origin API proxy path.', 'CFG-PROXY')
  }
  return basePath.replace(/\/$/, '')
}

function requestContext(caseId) {
  const config = runtimeConfig()
  const tenantId = config.tenantId || 'default'
  const collectionId = config.collectionId || collectionIdForCase(caseId)
  const baseUrl = proxyBasePath(config)
  const headers = {
    'X-Forensic-Tenant-ID': tenantId,
    'X-Forensic-Collection-ID': collectionId,
    'X-Forensic-Actor-ID': config.actorId || 'investigation-workspace',
    'X-Forensic-Subject-ID': config.subjectId || config.actorId || 'investigation-workspace',
    // The API's authorized roles are `user`, `admin` and `agent-worker`
    // (auth_scope.go). A human analyst is a `user`. The previous default
    // `analyst` reads correctly in English and was rejected with 403 on every
    // request -- adapting the client is right here; widening the server's
    // accepted roles to match the UI's wording would be a security change.
    'X-Forensic-Actor-Role': config.actorRole || 'user',
  }
  return { config, baseUrl, tenantId, collectionId, headers }
}

// Absolute when a base URL is configured, same-origin relative when it is not.
function endpoint(baseUrl, path) {
  const origin = globalThis.location?.origin || 'http://localhost'
  return new URL(`${baseUrl || ''}${path}`, origin)
}

async function scopedGet(path, caseId, { signal, fetchImpl = globalThis.fetch, responseType = 'json' } = {}) {
  const { baseUrl, tenantId, collectionId, headers } = requestContext(caseId)
  const url = endpoint(baseUrl, path)
  url.searchParams.set('tenant_id', tenantId)
  url.searchParams.set('collection_id', collectionId)
  const response = await fetchImpl(url, { signal, headers })
  if (!response.ok) {
    const error = new Error('The requested case evidence could not be opened.')
    error.reference = response.headers.get('x-request-id') || `HTTP-${response.status}`
    error.status = response.status
    throw error
  }
  if (responseType === 'blob') return response.blob()
  if (responseType === 'text') return response.text()
  return response.json()
}

export function configuredCaseIds() {
  const config = runtimeConfig()
  const values = Array.isArray(config.caseIds) ? config.caseIds : [config.collectionId].filter(Boolean)
  return [...new Set(values.map(collectionIdForCase).filter(Boolean))]
}

export function getCaseOverview({ caseId, signal, fetchImpl }) {
  return scopedGet('/collections/status', caseId, { signal, fetchImpl })
}

// The response also carries extensive implementation metadata. Consumers must
// project only analyst-safe fields (curated labels, availability and suggested
// questions); adapters, operations and deployment attestations never belong on
// the analyst surface.
export function getQueryCapabilities({ caseId, signal, fetchImpl }) {
  return scopedGet('/query/capabilities', caseId, { signal, fetchImpl })
}

export function listEvidence({ caseId, filters = {}, signal, fetchImpl }) {
  const query = new URLSearchParams()
  if (filters.modality) query.set('modality', filters.modality)
  if (filters.detectedType) query.set('detected_type', filters.detectedType)
  if (filters.processingStatus) query.set('processing_status', filters.processingStatus)
  if (filters.q) query.set('q', filters.q)
  query.set('limit', String(filters.limit || 100))
  query.set('offset', String(Math.max(0, Number(filters.offset) || 0)))
  return scopedGet(`/evidence?${query}`, caseId, { signal, fetchImpl })
}

export function getEvidence({ caseId, evidenceId, signal, fetchImpl }) {
  return scopedGet(`/evidence/${encodeURIComponent(evidenceId)}?include_records_preview=true`, caseId, { signal, fetchImpl })
}

export function getEvidenceContent({ caseId, evidenceId, signal, fetchImpl, responseType = 'blob' }) {
  return scopedGet(`/evidence/${encodeURIComponent(evidenceId)}/content`, caseId, { signal, fetchImpl, responseType })
}

export async function askCase({ caseId, query, recordType, signal, fetchImpl = globalThis.fetch }) {
  const { baseUrl, tenantId, collectionId, headers } = requestContext(caseId)
  const response = await fetchImpl(endpoint(baseUrl, '/query/hybrid'), {
    method: 'POST',
    signal,
    headers: {
      ...headers,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ tenant_id: tenantId, collection_id: collectionId, query, ...(recordType ? { record_type: recordType } : {}) }),
  })
  const requestReference = response.headers.get('x-request-id') || response.headers.get('x-correlation-id') || `HTTP-${response.status}`
  if (!response.ok) {
    const error = new Error('The question could not be completed.')
    error.reference = requestReference
    error.status = response.status
    throw error
  }
  return response.json()
}

// A deterministic analytical template over a whole case, with no target and no language model in the path. Used for
// figures that are aggregates (for example activity per day), never for questions. The service caps a template call at
// 100 rows, so callers must treat a full page as possibly partial.
export async function runCaseTemplate({ caseId, template, limit = 100, signal, fetchImpl = globalThis.fetch }) {
  const { baseUrl, tenantId, collectionId, headers } = requestContext(caseId)
  const response = await fetchImpl(endpoint(baseUrl, '/query/hybrid'), {
    method: 'POST',
    signal,
    headers: { ...headers, 'Content-Type': 'application/json' },
    // The platform's own corpus asks for a deterministic template as this canonical sentence, and the router keys its
    // early checks off the query text, so send it together with the explicit template and limit.
    body: JSON.stringify({ tenant_id: tenantId, collection_id: collectionId, query: `Run deterministic forensic query; template=${template}; limit=${limit}`, template, limit }),
  })
  if (!response.ok) {
    const error = new Error('The case figures could not be read.')
    error.reference = response.headers.get('x-request-id') || `HTTP-${response.status}`
    error.status = response.status
    throw error
  }
  return response.json()
}

// Evidence intake. The backend exposes exactly one way in --
// POST /webhooks/records/upload, multipart, with `file` required and
// `collection_id` required -- and a collection comes into existence when its
// first file is accepted. There is no create-case endpoint and no case entity
// in the schema, so "creating a case" is naming a collection and adding its
// first evidence. Nothing here invents an endpoint that does not exist.
//
// XMLHttpRequest rather than fetch because fetch cannot report UPLOAD progress.
// The product forbids progress bars that do not reflect real progress, so the
// only bar shown is this one, which is the real byte count of the transfer.
// What happens AFTER the bytes land -- queueing, parsing, derivation -- has no
// honest percentage, and is reported as a STATE, never as a bar.
export function uploadEvidence({ caseId, file, sourceEntry, onProgress, signal, xhrImpl }) {
  const { config, baseUrl, tenantId, collectionId, headers } = requestContext(caseId)
  const XHR = xhrImpl || globalThis.XMLHttpRequest
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      const error = new Error('The upload was cancelled.')
      error.name = 'AbortError'
      reject(error)
      return
    }
    const form = new globalThis.FormData()
    form.append('file', file)
    form.append('tenant_id', tenantId)
    form.append('collection_id', collectionId)
    form.append('case_id', collectionId)
    form.append('user_id', config.actorId || 'investigation-workspace')
    if (sourceEntry) form.append('source_entry', sourceEntry)

    const request = new XHR()
    const stopListening = () => signal?.removeEventListener?.('abort', abortRequest)
    const abortRequest = () => request.abort()
    request.open('POST', endpoint(baseUrl, '/webhooks/records/upload'))
    for (const [name, value] of Object.entries(headers)) request.setRequestHeader(name, value)
    // The authorised scope is taken FROM THE HEADERS; a body value may only
    // echo it, never widen it. This request sends `case_id` in the form, so
    // without this header `bindForensicScope` compares a non-empty supplied
    // case against an empty authorised one and refuses the upload with 403 --
    // which the analyst saw as "not authorized to add evidence to this case".
    // Sent only here, because this is the only request that supplies a case.
    request.setRequestHeader('X-Forensic-Case-ID', collectionId)
    request.upload.addEventListener('progress', event => {
      if (event.lengthComputable && onProgress) onProgress(event.loaded / event.total)
    })
    request.addEventListener('load', () => {
      stopListening()
      const reference = request.getResponseHeader('x-request-id') || `HTTP-${request.status}`
      if (request.status >= 200 && request.status < 300) {
        let body = {}
        try { body = JSON.parse(request.responseText || '{}') } catch { body = {} }
        resolve(body)
        return
      }
      // An intake refusal is a real answer the analyst must see, not a crash:
      // the file was too large, or the type is not accepted. Carry the
      // reference so it can be traced.
      const error = new Error(intakeRefusalMessage(request.status))
      error.reference = reference
      error.status = request.status
      reject(error)
    })
    request.addEventListener('error', () => {
      stopListening()
      const error = new Error('The upload did not reach the evidence service.')
      error.reference = 'NETWORK'
      reject(error)
    })
    request.addEventListener('abort', () => {
      stopListening()
      const error = new Error('The upload was cancelled.')
      error.name = 'AbortError'
      reject(error)
    })
    signal?.addEventListener?.('abort', abortRequest, { once: true })
    request.send(form)
  })
}

function intakeRefusalMessage(status) {
  if (status === 413) return 'That file is larger than this case accepts.'
  if (status === 400) return 'That file could not be accepted as evidence.'
  if (status === 401 || status === 403) return 'This workspace is not authorized to add evidence to this case.'
  return 'The evidence service could not accept that file.'
}

import { collectionIdForCase } from './caseScope.js'
