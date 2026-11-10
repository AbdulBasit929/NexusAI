import { caseFetchContext } from './apiClient.js'
import { ConversationError } from './conversationsApi.js'

// The case layer's other half: what an analyst writes down about a case (notes, findings, hypotheses, tasks, annotations, saved views, tags) and who may do what (members, authority records,
// the audit trail). Contract: docs/work/API_CONTRACT_CONVERSATIONS_20261103.md sections 3 and 4. One shape for every kind: list, create, read, change, hide. Nothing is deleted: "remove" hides
// an object with a reason, and the audit trail keeps who and why. Errors are the ConversationError of the conversation calls: a sentence for a person, a machine code, a request id.
export const KINDS = ['findings', 'notes', 'hypotheses', 'tasks', 'annotations', 'views']

async function call({ caseId, method = 'GET', path, query, body, signal, fetchImpl = globalThis.fetch }) {
  const { headers, url } = caseFetchContext(caseId)
  const target = url(path)
  for (const [key, value] of Object.entries(query || {})) if (value !== undefined && value !== null && value !== '') target.searchParams.set(key, String(value))
  const response = await fetchImpl(target, { method, signal, headers: { ...headers, ...(body ? { 'Content-Type': 'application/json' } : {}) }, body: body ? JSON.stringify(body) : undefined })
  if (!response.ok) {
    let payload = null
    try { payload = await response.json() } catch { payload = null }
    throw new ConversationError(payload?.error || 'The case service could not do that.', { status: response.status, code: payload?.code || '', requestId: payload?.request_id || response.headers.get('x-request-id') || '' })
  }
  if (response.status === 204) return null
  return response.json()
}

const id = value => encodeURIComponent(value)
const check = kind => { if (!KINDS.includes(kind)) throw new Error(`unknown kind of case object: ${kind}`) }

export function listObjects({ caseId, kind, filters = {}, q, tag, limit, cursor, signal, fetchImpl }) {
  check(kind)
  return call({ caseId, path: `/cases/${id(caseId)}/${kind}`, query: { ...filters, q, tag, limit, cursor }, signal, fetchImpl })
}
export function createObject({ caseId, kind, body, signal, fetchImpl }) { check(kind); return call({ caseId, method: 'POST', path: `/cases/${id(caseId)}/${kind}`, body, signal, fetchImpl }) }
export function changeObject({ caseId, kind, objectId, patch, signal, fetchImpl }) { check(kind); return call({ caseId, method: 'PATCH', path: `/${kind}/${id(objectId)}`, body: patch, signal, fetchImpl }) }
export function removeObject({ caseId, kind, objectId, reason, signal, fetchImpl }) { check(kind); return call({ caseId, method: 'DELETE', path: `/${kind}/${id(objectId)}`, query: { reason }, signal, fetchImpl }) }
export const reviewFinding = ({ caseId, findingId, state, note, signal, fetchImpl }) => call({ caseId, method: 'POST', path: `/findings/${id(findingId)}/review`, body: { state, ...(note ? { note } : {}) }, signal, fetchImpl })

export const listTags = ({ caseId, signal, fetchImpl }) => call({ caseId, path: `/cases/${id(caseId)}/tags`, signal, fetchImpl })
export const setTags = ({ caseId, kind, objectId, tags, signal, fetchImpl }) => call({ caseId, method: 'PUT', path: `/cases/${id(caseId)}/objects/${id(kind)}/${id(objectId)}/tags`, body: { tags }, signal, fetchImpl })

export const getMembers = ({ caseId, signal, fetchImpl }) => call({ caseId, path: `/cases/${id(caseId)}/members`, signal, fetchImpl })
export const setMember = ({ caseId, subject, role, displayName, signal, fetchImpl }) => call({ caseId, method: 'PUT', path: `/cases/${id(caseId)}/members/${id(subject)}`, body: { role, ...(displayName ? { display_name: displayName } : {}) }, signal, fetchImpl })
export const removeMember = ({ caseId, subject, signal, fetchImpl }) => call({ caseId, method: 'DELETE', path: `/cases/${id(caseId)}/members/${id(subject)}`, signal, fetchImpl })
export const listAuthority = ({ caseId, signal, fetchImpl }) => call({ caseId, path: `/cases/${id(caseId)}/authority`, signal, fetchImpl })
export const createAuthority = ({ caseId, body, signal, fetchImpl }) => call({ caseId, method: 'POST', path: `/cases/${id(caseId)}/authority`, body, signal, fetchImpl })
export const revokeAuthority = ({ caseId, authorityId, reason, signal, fetchImpl }) => call({ caseId, method: 'POST', path: `/authority/${id(authorityId)}/revoke`, body: { reason }, signal, fetchImpl })
export const getAudit = ({ caseId, action, objectKind, objectId, limit, cursor, signal, fetchImpl }) => call({ caseId, path: `/cases/${id(caseId)}/audit`, query: { action, object_kind: objectKind, object_id: objectId, limit, cursor }, signal, fetchImpl })
export const verifyAudit = ({ caseId, signal, fetchImpl }) => call({ caseId, path: `/cases/${id(caseId)}/audit/verify`, signal, fetchImpl })
