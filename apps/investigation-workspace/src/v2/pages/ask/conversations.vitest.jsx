import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, renderHook, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { ConversationError, isNotInstalled } from '../../../lib/conversationsApi.js'
import { ConversationRail, dayGroup } from './ConversationRail.jsx'
import { RunDiff, diffLead } from './RunDiff.jsx'
import { Turn } from './Turn.jsx'
import { pathOf, toUiTurn, useConversationThread } from './useConversationThread.js'

vi.mock('../../../lib/conversationsApi.js', async importOriginal => ({
  ...(await importOriginal()),
  createConversation: vi.fn(),
  askTurn: vi.fn(),
  rerunTurn: vi.fn(),
  getConversation: vi.fn(),
  getTurn: vi.fn(),
  forkTurn: vi.fn(),
  sendFeedback: vi.fn(),
  pinTurn: vi.fn(),
}))
vi.mock('../../../lib/investigationPresentation.js', () => ({
  presentInvestigationResponse: (response, options) => (response
    ? { state: 'answered', answer: response.answer, question: '', result: { columns: [], rows: [] }, citations: { groups: [], items: [] }, limitations: [], followUps: [], scope: [], original: response }
    : { state: 'failed', answer: '', errorReference: options?.error?.reference || '', result: { columns: [], rows: [] }, citations: { groups: [], items: [] }, limitations: [], followUps: [], scope: [] }),
}))
vi.mock('../../../lib/sessionActivity.js', () => ({ recordSessionActivity: vi.fn() }))
const api = await import('../../../lib/conversationsApi.js')
const { askTurn: realAskTurn } = await vi.importActual('../../../lib/conversationsApi.js')

const stored = (over = {}) => ({ turn_id: 't1', thread_id: 'th1', question: 'Who called the most?', state: 'completed', outcome: 'answered', kind: 'ask', parent_turn_id: null, created_at: '2026-11-04T10:00:00Z', started_at: '2026-11-04T10:00:00Z', completed_at: '2026-11-04T10:00:30Z', answer: { answer: '3 numbers made calls' }, reruns: { count: 0, latest_changed: null }, ...over })

// a fetch whose body arrives in the chunks given, so a line can be cut in the middle as a real connection cuts it
function streamOf(...chunks) {
  const encoder = new TextEncoder()
  let index = 0
  return { ok: true, status: 200, headers: { get: () => null }, body: { getReader: () => ({ read: async () => (index < chunks.length ? { done: false, value: encoder.encode(chunks[index++]) } : { done: true, value: undefined }) }) } }
}

describe('askTurn (the stream)', () => {
  it('reads lines that arrive cut in two, reports each phase, and resolves with the turn', async () => {
    const events = []
    const line = object => `${JSON.stringify(object)}\n`
    const fetchImpl = vi.fn(async () => streamOf(line({ event: 'phase', phase: 'accepted', label: 'Question received' }).slice(0, 20), line({ event: 'phase', phase: 'accepted', label: 'Question received' }).slice(20) + line({ event: 'heartbeat', elapsed_ms: 2000 }), line({ event: 'turn', turn: { turn_id: 't9' }, diff: null }) + line({ event: 'done' })))
    const result = await realAskTurn({ caseId: 'case-a', threadId: 'th1', question: 'q', onEvent: event => events.push(event.event + (event.phase ? `:${event.phase}` : '')), fetchImpl })
    expect(events).toEqual(['phase:accepted', 'heartbeat', 'turn', 'done'])
    expect(result.turn.turn_id).toBe('t9')
    const [url, init] = fetchImpl.mock.calls[0]
    expect(String(url)).toMatch(/\/conversations\/th1\/turns$/)
    expect(init.headers.Accept).toBe('application/x-ndjson')
    expect(JSON.parse(init.body)).toEqual({ question: 'q' })
  })

  it('rejects with the failed turn when the engine could not answer, and with the sentence when the call is refused', async () => {
    const failing = vi.fn(async () => streamOf(`${JSON.stringify({ event: 'error', code: 'engine_failed', message: 'The engine stopped.', turn: { turn_id: 'tf', state: 'failed' } })}\n${JSON.stringify({ event: 'done' })}\n`))
    await expect(realAskTurn({ caseId: 'case-a', threadId: 'th1', question: 'q', fetchImpl: failing })).rejects.toMatchObject({ code: 'engine_failed', message: 'The engine stopped.', turn: { turn_id: 'tf' } })
    const refused = vi.fn(async () => ({ ok: false, status: 403, headers: { get: () => 'req-1' }, json: async () => ({ error: 'Only the owner may add a question; fork it to continue.', code: 'forbidden', request_id: 'req-9' }) }))
    await expect(realAskTurn({ caseId: 'case-a', threadId: 'th1', question: 'q', fetchImpl: refused })).rejects.toMatchObject({ status: 403, code: 'forbidden', requestId: 'req-9', message: 'Only the owner may add a question; fork it to continue.' })
  })

  it('says the connection closed when no answer came, instead of waiting for ever', async () => {
    const dropped = vi.fn(async () => streamOf(`${JSON.stringify({ event: 'phase', phase: 'engine', label: 'Answering' })}\n`))
    await expect(realAskTurn({ caseId: 'case-a', threadId: 'th1', question: 'q', fetchImpl: dropped })).rejects.toMatchObject({ code: 'interrupted' })
  })

  it('tells a service that is not installed from one that failed', () => {
    expect(isNotInstalled(new ConversationError('x', { status: 503, code: 'not_installed' }))).toBe(true)
    expect(isNotInstalled(new ConversationError('x', { status: 404, code: '' }))).toBe(true)      // an API image that has no such route
    expect(isNotInstalled(new ConversationError('x', { status: 404, code: 'not_found' }))).toBe(false)
    expect(isNotInstalled(new ConversationError('x', { status: 503, code: 'unavailable' }))).toBe(false)
  })
})

describe('pathOf and toUiTurn', () => {
  it('draws the path from the first question to the newest turn, so a run again takes the place of the run it repeats', () => {
    const thread = { active_turn_id: 'c', turns: [stored({ turn_id: 'a', idx: 1 }), stored({ turn_id: 'b', parent_turn_id: 'a', question: 'and March?' }), stored({ turn_id: 'c', parent_turn_id: 'a', kind: 'rerun', rerun_of_turn_id: 'b', question: 'and March?' })] }
    expect(pathOf(thread).map(turn => turn.turn_id)).toEqual(['a', 'c'])
  })

  it('maps a stored turn to what the page draws, and a failed one to a recorded failure with its reference', () => {
    const ok = toUiTurn(stored({ reruns: { count: 1, latest_changed: true }, pinned_as: ['f1'] }), 'case-a')
    expect(ok).toMatchObject({ id: 't1', stored: true, busy: false, kind: 'ask', pinnedAs: ['f1'] })
    expect(ok.presentation.answer).toBe('3 numbers made calls')
    const failed = toUiTurn(stored({ turn_id: 'abcdef123456', state: 'failed', answer: null, error: { message: 'The engine stopped.' } }), 'case-a')
    expect(failed.presentation.state).toBe('failed')
    expect(failed.presentation.errorReference).toBe('abcdef12')
    expect(failed.serviceMessage).toBe('The engine stopped.')
    expect(toUiTurn(stored({ state: 'running', answer: null }), 'case-a').busy).toBe(true)
    expect(toUiTurn(stored({ answer: null, snapshot_state: 'none' }), 'case-a').answerMissing).toBe(true)
  })
})

describe('useConversationThread', () => {
  beforeEach(() => { Object.values(api).forEach(fn => fn.mockReset?.()) })

  it('creates the conversation on the first question, narrates the phases, and keeps the stored turn', async () => {
    api.createConversation.mockResolvedValue({ thread_id: 'th1' })
    let release
    api.askTurn.mockImplementationOnce(async ({ onEvent }) => {
      onEvent({ event: 'phase', phase: 'queued', label: 'Waiting', position: 2, turn_id: 't1' })
      onEvent({ event: 'heartbeat', elapsed_ms: 4000 })
      await new Promise(resolve => { release = resolve })
      return { turn: stored(), diff: null }
    })
    const changed = vi.fn()
    const { result } = renderHook(() => useConversationThread('case-a', { onChanged: changed }))
    let pending
    act(() => { pending = result.current.ask('Who called the most?') })
    await waitFor(() => expect(result.current.turns[0]?.phase?.name).toBe('queued'))
    expect(result.current.turns[0].busy).toBe(true)
    expect(result.current.turns[0].phase.position).toBe(2)
    expect(result.current.turns[0].phase.elapsedMs).toBe(4000)
    release()
    await act(async () => { await pending })
    expect(api.createConversation).toHaveBeenCalledTimes(1)
    expect(api.askTurn.mock.calls[0][0]).toMatchObject({ threadId: 'th1', question: 'Who called the most?' })
    expect(api.askTurn.mock.calls[0][0].clientRequestId).toMatch(/^[A-Za-z0-9._:-]{1,100}$/)
    expect(result.current.turns).toHaveLength(1)
    expect(result.current.turns[0]).toMatchObject({ id: 't1', stored: true, busy: false })
    expect(result.current.threadId).toBe('th1')
    expect(changed).toHaveBeenCalled()
    api.askTurn.mockResolvedValueOnce({ turn: stored({ turn_id: 't2', parent_turn_id: 't1', question: 'and for March?' }), diff: null })
    await act(async () => { await result.current.ask('and for March?') })
    expect(api.createConversation).toHaveBeenCalledTimes(1)   // the second question joins the same conversation
  })

  it('records a refused question as a failed turn with the service’s sentence', async () => {
    api.createConversation.mockResolvedValue({ thread_id: 'th1' })
    api.askTurn.mockRejectedValue(new ConversationError('Fork it to continue in your own conversation.', { status: 403, code: 'forbidden', requestId: 'req-3' }))
    const { result } = renderHook(() => useConversationThread('case-a'))
    await act(async () => { await result.current.ask('q') })
    expect(result.current.turns[0].presentation.state).toBe('failed')
    expect(result.current.turns[0].serviceMessage).toBe('Fork it to continue in your own conversation.')
    expect(result.current.busy).toBe(false)
  })

  it('runs a turn again in the same place and keeps the comparison', async () => {
    api.createConversation.mockResolvedValue({ thread_id: 'th1' })
    api.askTurn.mockResolvedValue({ turn: stored(), diff: null })
    api.rerunTurn.mockResolvedValue({ turn: stored({ turn_id: 't2', kind: 'rerun', answer: { answer: '4 numbers made calls' } }), diff: { changed: true, headline: { before: '3', after: '4', changed: true } } })
    const { result } = renderHook(() => useConversationThread('case-a'))
    await act(async () => { await result.current.ask('q') })
    await act(async () => { await result.current.rerun(result.current.turns[0]) })
    expect(result.current.turns).toHaveLength(1)
    expect(result.current.turns[0]).toMatchObject({ id: 't2', kind: 'rerun', previousTurnId: 't1' })
    expect(result.current.turns[0].diff.changed).toBe(true)
    expect(result.current.turns[0].presentation.answer).toBe('4 numbers made calls')
  })

  it('opens a stored conversation and loads the answers of the older turns one by one', async () => {
    api.getConversation.mockResolvedValue({ thread_id: 'th1', active_turn_id: 'b', turns: [stored({ turn_id: 'a', answer: null, snapshot_state: 'full' }), stored({ turn_id: 'b', parent_turn_id: 'a', question: 'and March?' })] })
    api.getTurn.mockResolvedValue(stored({ turn_id: 'a', answer: { answer: 'the first answer' } }))
    const { result } = renderHook(() => useConversationThread('case-a'))
    await act(async () => { await result.current.open('th1') })
    expect(result.current.turns.map(turn => turn.id)).toEqual(['a', 'b'])
    await waitFor(() => expect(result.current.turns[0].presentation?.answer).toBe('the first answer'))
    expect(api.getTurn).toHaveBeenCalledTimes(1)
  })

  it('puts the verdict and the pin on the turn', async () => {
    api.createConversation.mockResolvedValue({ thread_id: 'th1' })
    api.askTurn.mockResolvedValue({ turn: stored(), diff: null })
    api.sendFeedback.mockResolvedValue({ mine: { verdict: 'wrong' }, counts: { wrong: 1 } })
    api.pinTurn.mockResolvedValue({ finding: { finding_id: 'f1' }, already_pinned: false })
    const { result } = renderHook(() => useConversationThread('case-a'))
    await act(async () => { await result.current.ask('q') })
    await act(async () => { await result.current.feedback(result.current.turns[0], 'wrong') })
    await act(async () => { await result.current.pin(result.current.turns[0]) })
    expect(result.current.turns[0].feedback.mine.verdict).toBe('wrong')
    expect(result.current.turns[0].pinnedAs).toEqual(['f1'])
  })
})

describe('ConversationRail', () => {
  afterEach(cleanup)
  const item = (over = {}) => ({ thread_id: 'th1', display_title: 'Who called the most?', pinned: false, turn_count: 2, last_outcome: 'answered', last_turn_at: new Date().toISOString(), created_at: new Date().toISOString(), changed_since: false, matches: [], ...over })
  const railOf = (items, over = {}) => ({ items, status: 'ready', next: '', error: '', pin: vi.fn(), rename: vi.fn(), archive: vi.fn(), hide: vi.fn(async () => {}), restore: vi.fn(async () => {}), loadMore: vi.fn(), ...over })
  const show = (rail, props = {}) => render(<MemoryRouter><ConversationRail rail={rail} activeId="th1" onOpen={vi.fn()} onNew={vi.fn()} query="" onQuery={vi.fn()} {...props} /></MemoryRouter>)

  it('groups by the day of the last question with the pinned ones first', () => {
    const now = new Date('2026-11-10T12:00:00')
    expect(dayGroup({ pinned: true, last_turn_at: '2026-11-10T09:00:00' }, now)).toBe('Pinned')
    expect(dayGroup({ last_turn_at: '2026-11-10T09:00:00' }, now)).toBe('Today')
    expect(dayGroup({ last_turn_at: '2026-11-09T23:00:00' }, now)).toBe('Yesterday')
    expect(dayGroup({ last_turn_at: '2026-11-06T09:00:00' }, now)).toBe('This week')
    expect(dayGroup({ last_turn_at: '2026-10-01T09:00:00' }, now)).toBe('Earlier')
  })

  it('shows a mark for a conversation with a later run that differs, and the question that matched a search', () => {
    show(railOf([item({ changed_since: true, matches: [{ question: 'calls in March' }] })]), { query: 'march' })
    expect(screen.getByText('Changed')).toBeTruthy()
    expect(screen.getByText('calls in March')).toBeTruthy()
  })

  it('renames in place, and offers to bring a deleted conversation back', async () => {
    const rail = railOf([item()])
    show(rail)
    fireEvent.click(screen.getByRole('button', { name: /More for Who called the most/ }))
    fireEvent.click(screen.getByRole('menuitem', { name: 'Rename' }))
    const field = screen.getByLabelText('Name this conversation')
    fireEvent.change(field, { target: { value: 'Night calls' } })
    fireEvent.submit(field.closest('form'))
    await waitFor(() => expect(rail.rename).toHaveBeenCalledWith('th1', 'Night calls'))
    fireEvent.click(screen.getByRole('button', { name: /More for Who called the most/ }))
    fireEvent.click(screen.getByRole('menuitem', { name: 'Delete' }))
    const undo = await screen.findByRole('button', { name: /Undo/ })
    fireEvent.click(undo)
    await waitFor(() => expect(rail.restore).toHaveBeenCalledWith('th1'))
  })

  it('says why the list is empty: reading, an error, no match, or nothing yet', () => {
    show(railOf([], { status: 'loading' }))
    expect(screen.getByText('Reading the history…')).toBeTruthy()
    cleanup()
    show(railOf([], { status: 'error', error: 'The service is not answering.' }))
    expect(screen.getByText('The service is not answering.')).toBeTruthy()
    cleanup()
    show(railOf([]), { query: 'zzz' })
    expect(screen.getByText('No conversation matches.')).toBeTruthy()
  })
})

describe('RunDiff', () => {
  afterEach(cleanup)
  const diff = (over = {}) => ({ changed: true, figures_changed: true, evidence_changed: false, complete: true, since: '2026-11-03T16:37:05Z', headline: { before: '3 numbers', after: '4 numbers', changed: true }, metrics: [{ label: 'Number of records', before: '5,005', after: '5,010', delta: '5' }], rows: { before_count: 3, after_count: 4, added_total: 1, added: [['923000000204', 2]], removed_total: 0, removed: [], changed_total: 1, changed: [{ key: '923000000202', cells: [{ column: 'Calls', before: 9, after: 10 }] }] }, evidence: { added: [], removed: [], replaced: [] }, data_version: { known: true, changed: false }, query: { changed: false }, same_data_different_answer: false, ...over })

  it('leads with the sentence that matters', () => {
    expect(diffLead(diff({ changed: false }))).toMatch(/Nothing changed/)
    expect(diffLead(diff())).toBe('The figures changed.')
    expect(diffLead(diff({ same_data_different_answer: true, query: { changed: true } }))).toMatch(/data did not change, but the answer did\. The model wrote a different query/)
    expect(diffLead(diff({ same_data_different_answer: true }))).toMatch(/The query is the same/)
    expect(diffLead(diff({ figures_changed: false, evidence_changed: true }))).toMatch(/evidence/)
  })

  it('lists what was added, removed and changed, with exact counts, in a dialog that closes', () => {
    const onClose = vi.fn()
    render(<RunDiff diff={diff()} onClose={onClose} />)
    const dialog = screen.getByRole('dialog')
    expect(within(dialog).getByText('The figures changed.')).toBeTruthy()
    expect(within(dialog).getByText('3 numbers')).toBeTruthy()
    expect(within(dialog).getByText('4 numbers')).toBeTruthy()
    expect(within(dialog).getByText('923000000204 · 2')).toBeTruthy()
    expect(within(dialog).getByText(/Calls 9 → 10/)).toBeTruthy()
    expect(within(dialog).getByText('1 added')).toBeTruthy()
    fireEvent.click(within(dialog).getByRole('button', { name: 'Close the comparison' }))
    expect(onClose).toHaveBeenCalled()
  })

  it('says when the data of one run was not stored, and when a result was longer than the lists', () => {
    render(<RunDiff diff={diff({ complete: false, data_version: { known: false } })} onClose={() => {}} />)
    expect(screen.getByText(/more than 200 rows/)).toBeTruthy()
    expect(screen.getByText(/no stored version of the data/)).toBeTruthy()
  })
})

describe('Turn with the conversation service', () => {
  afterEach(cleanup)
  const base = { id: 't1', stored: true, query: 'Who called the most?', presentation: null, busy: false, startedAt: Date.now(), reruns: { count: 0 }, pinnedAs: [] }
  const answered = over => ({ ...base, presentation: { state: 'answered', answer: '3 numbers made calls', result: { columns: [], rows: [] }, citations: { groups: [], items: [], markerItems: [] }, limitations: [], followUps: [], scope: [], claimSegments: [] }, ...over })
  const show = (turn, props = {}) => render(<MemoryRouter><Turn turn={turn} index={0} last caseId="case-a" onAsk={vi.fn()} onChoose={vi.fn()} onRetry={vi.fn()} onEdit={vi.fn()} onCopy={vi.fn()} copied={false} evidenceOpen={false} onToggleEvidence={vi.fn()} {...props} /></MemoryRouter>)

  it('narrates a wait with the phase the service reports and its own clock', () => {
    show({ ...base, busy: true, phase: { name: 'queued', label: 'Waiting', position: 2, elapsedMs: 12000 } })
    expect(screen.getByText('Waiting for the language model (number 2 in the queue)…')).toBeTruthy()
    expect(screen.getByText(/12 s\./)).toBeTruthy()
    cleanup()
    show({ ...base, busy: true, phase: { name: 'saving', label: 'Saving the answer' } })
    expect(screen.getByText('Saving the answer…')).toBeTruthy()
  })

  it('offers run again, pin, fork and the verdict only when the service gives them, and marks a changed run', () => {
    const calls = { rerun: vi.fn(), pin: vi.fn(), fork: vi.fn(), feedback: vi.fn(), compare: vi.fn() }
    show(answered({ diff: { changed: true }, feedback: { mine: { verdict: 'wrong' } } }), { onRerun: calls.rerun, onPin: calls.pin, onFork: calls.fork, onFeedback: calls.feedback, onCompare: calls.compare })
    fireEvent.click(screen.getByRole('button', { name: /Run again/ }))
    fireEvent.click(screen.getByRole('button', { name: /Pin to findings/ }))
    fireEvent.click(screen.getByRole('button', { name: /Fork/ }))
    fireEvent.click(screen.getByRole('button', { name: /Changed since the earlier run/ }))
    fireEvent.click(screen.getByRole('button', { name: 'This answer is correct' }))
    expect(screen.getByRole('button', { name: 'This answer is wrong' }).getAttribute('aria-pressed')).toBe('true')
    Object.values(calls).forEach(fn => expect(fn).toHaveBeenCalled())
    expect(calls.feedback).toHaveBeenCalledWith('correct')
  })

  it('keeps the old page as it was when no service handlers are given', () => {
    show(answered({ stored: false }))
    expect(screen.queryByRole('button', { name: /Run again/ })).toBeNull()
    expect(screen.queryByRole('button', { name: /Pin to findings/ })).toBeNull()
    expect(screen.getByRole('button', { name: /Ask a variation/ })).toBeTruthy()
  })

  it('says what a stopped wait means and what a missing answer needs', () => {
    show({ ...base, aborted: true, stoppedWaiting: true })
    expect(screen.getByText(/You stopped waiting\. The question may still finish/)).toBeTruthy()
    cleanup()
    const rerun = vi.fn()
    show({ ...base, answerMissing: true }, { onRerun: rerun })
    fireEvent.click(screen.getByRole('button', { name: 'Run it again' }))
    expect(rerun).toHaveBeenCalled()
  })
})
