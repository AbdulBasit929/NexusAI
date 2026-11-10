import { caseFetchContext } from './apiClient.js'

// The conversation service (the case layer, migration 034): history, turns, re-run and diff, fork, feedback and pin. Contract: docs/work/API_CONTRACT_CONVERSATIONS_20261103.md.
// Every call goes through the same proxy and headers as the rest of the workspace. An error carries the sentence the service wrote for a person (`message`), a machine `code` and the
// `requestId`; while migration 034 is not applied every route answers 503 `not_installed`, which the page shows as its designed "not available yet" state (`isNotInstalled`).
export class ConversationError extends Error {
  constructor(message, { status = 0, code = '', requestId = '' } = {}) {
    super(message)
    this.name = 'ConversationError'
    this.status = status
    this.code = code
    this.requestId = requestId
  }
}

export const isNotInstalled = error => Boolean(error) && (error.code === 'not_installed' || (error.status === 503 && error.code !== 'unavailable') || error.status === 404 && error.code === '')

async function readError(response) {
  let body = null
  try { body = await response.json() } catch { body = null }
  return new ConversationError(body?.error || 'The conversation service could not do that.', { status: response.status, code: body?.code || '', requestId: body?.request_id || response.headers.get('x-request-id') || '' })
}

async function call({ caseId, method = 'GET', path, query, body, signal, fetchImpl = globalThis.fetch }) {
  const { headers, url } = caseFetchContext(caseId)
  const target = url(path)
  for (const [key, value] of Object.entries(query || {})) if (value !== undefined && value !== null && value !== '') target.searchParams.set(key, String(value))
  const response = await fetchImpl(target, { method, signal, headers: { ...headers, ...(body ? { 'Content-Type': 'application/json' } : {}) }, body: body ? JSON.stringify(body) : undefined })
  if (!response.ok) throw await readError(response)
  if (response.status === 204) return null
  return response.json()
}

const id = value => encodeURIComponent(value)

// ----- the history rail -----
export const listConversations = ({ caseId, q, tag, pinned, archived, mine, shared, limit, cursor, signal, fetchImpl }) =>
  call({ caseId, path: '/conversations', query: { case_id: caseId, q, tag, pinned, archived, mine, shared, limit, cursor }, signal, fetchImpl })
export const createConversation = ({ caseId, title, signal, fetchImpl }) => call({ caseId, method: 'POST', path: '/conversations', body: { case_id: caseId, ...(title ? { title } : {}) }, signal, fetchImpl })
export const getConversation = ({ caseId, threadId, answers = 'latest', signal, fetchImpl }) => call({ caseId, path: `/conversations/${id(threadId)}`, query: { answers }, signal, fetchImpl })
export const updateConversation = ({ caseId, threadId, patch, signal, fetchImpl }) => call({ caseId, method: 'PATCH', path: `/conversations/${id(threadId)}`, body: patch, signal, fetchImpl })
export const hideConversation = ({ caseId, threadId, reason, signal, fetchImpl }) => call({ caseId, method: 'DELETE', path: `/conversations/${id(threadId)}`, query: { reason }, signal, fetchImpl })
export const restoreConversation = ({ caseId, threadId, signal, fetchImpl }) => call({ caseId, method: 'POST', path: `/conversations/${id(threadId)}/restore`, signal, fetchImpl })

// ----- one turn -----
export const getTurn = ({ caseId, turnId, answer = true, signal, fetchImpl }) => call({ caseId, path: `/turns/${id(turnId)}`, query: answer ? {} : { answer: 'false' }, signal, fetchImpl })
export const getTurnDiff = ({ caseId, turnId, against, signal, fetchImpl }) => call({ caseId, path: `/turns/${id(turnId)}/diff`, query: { against }, signal, fetchImpl })
export const forkTurn = ({ caseId, turnId, title, signal, fetchImpl }) => call({ caseId, method: 'POST', path: `/turns/${id(turnId)}/fork`, body: title ? { title } : {}, signal, fetchImpl })
export const sendFeedback = ({ caseId, turnId, verdict, note, signal, fetchImpl }) => call({ caseId, method: 'POST', path: `/turns/${id(turnId)}/feedback`, body: { verdict, ...(note ? { note } : {}) }, signal, fetchImpl })
export const pinTurn = ({ caseId, turnId, title, note, basis, signal, fetchImpl }) => call({ caseId, method: 'POST', path: `/turns/${id(turnId)}/pin`, body: { ...(title ? { title } : {}), ...(note ? { note } : {}), ...(basis ? { basis } : {}) }, signal, fetchImpl })

// ----- asking, as a stream -----
// POST .../turns (or .../rerun) with Accept: application/x-ndjson answers one JSON object for each line, flushed as it happens: `phase`, `heartbeat`, `turn`, `error`, `done`. Only true phases are sent.
// `onEvent(event)` gets each one; the promise resolves with `{ turn, diff }` when the answer arrives and rejects with a ConversationError when the engine could not answer (the failed turn is on
// `error.turn`, so the history shows it) or when the call itself was refused (a status and a sentence, before any event). Closing the page does not stop the engine: reopen the thread and read the turn.
async function stream({ caseId, path, body, signal, onEvent, fetchImpl = globalThis.fetch }) {
  const { headers, url } = caseFetchContext(caseId)
  const response = await fetchImpl(url(path), { method: 'POST', signal, headers: { ...headers, 'Content-Type': 'application/json', Accept: 'application/x-ndjson' }, body: JSON.stringify(body || {}) })
  if (!response.ok) throw await readError(response)
  if (!response.body?.getReader) {   // a client without streams still gets the answer: the plain encoding
    const plain = await response.json()
    return { turn: plain.turn, diff: plain.diff || null }
  }
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  let result = null
  let failure = null
  const handle = line => {
    if (!line.trim()) return
    let event
    try { event = JSON.parse(line) } catch { return }
    onEvent?.(event)
    if (event.event === 'turn') result = { turn: event.turn, diff: event.diff || null }
    if (event.event === 'error') failure = Object.assign(new ConversationError(event.message || 'The question could not be answered.', { status: 200, code: event.code || 'engine_failed' }), { turn: event.turn || null })
  }
  for (;;) {
    const { value, done } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() || ''
    lines.forEach(handle)
  }
  buffer += decoder.decode()
  handle(buffer)
  if (failure && !result) throw failure
  if (!result) throw new ConversationError('The connection closed before the answer arrived. Open the conversation again: the question may still be running.', { code: 'interrupted' })
  return result
}

export const askTurn = ({ caseId, threadId, question, clientRequestId, parentTurnId, editOfTurnId, language, signal, onEvent, fetchImpl }) =>
  stream({ caseId, path: `/conversations/${id(threadId)}/turns`, signal, onEvent, fetchImpl, body: {
    question,
    ...(clientRequestId ? { client_request_id: clientRequestId } : {}),
    ...(parentTurnId ? { parent_turn_id: parentTurnId } : {}),
    ...(editOfTurnId ? { edit_of_turn_id: editOfTurnId } : {}),
    ...(language ? { language } : {}),
  } })
export const rerunTurn = ({ caseId, turnId, signal, onEvent, fetchImpl }) => stream({ caseId, path: `/turns/${id(turnId)}/rerun`, signal, onEvent, fetchImpl })
