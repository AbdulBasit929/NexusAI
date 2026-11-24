import { useCallback, useEffect, useRef, useState } from 'react'
import { useParams, useSearchParams } from 'react-router-dom'
import { ArrowDown, CircleAlert, GitFork, History, MessageSquarePlus } from 'lucide-react'
import { familyDefinition, useEvidenceScope } from '../../../components/ScopeChips.jsx'
import { useQuestionDraft, useQuestionHistory } from '../../../lib/workspaceState.js'
import { useCaseOverview } from '../../../lib/useCaseOverview.js'
import { coverageLine } from '../../../pages/investigate/CaseStart.jsx'
import { AppShell } from '../../shell/AppShell.jsx'
import { useCaseLabel } from '../../shell/CaseLabel.jsx'
import { Composer } from './Composer.jsx'
import { EmptyAsk } from './EmptyAsk.jsx'
import { EvidenceDrawer } from './EvidenceDrawer.jsx'
import { HistoryRail } from './HistoryRail.jsx'
import { ConversationRail } from './ConversationRail.jsx'
import { RunDiff } from './RunDiff.jsx'
import { Turn } from './Turn.jsx'
import { useAskThread } from './useAskThread.js'
import { useConversationRail } from './useConversationRail.js'
import { useConversationThread } from './useConversationThread.js'
import { getTurnDiff } from '../../../lib/conversationsApi.js'
import { searchImages } from '../../../lib/apiClient.js'
import { presentInvestigationResponse } from '../../../lib/investigationPresentation.js'
import { useStarters } from './starters.js'

// Ask: a question in plain words, an answer that shows its evidence. One column for the conversation that uses the width it is given, a history
// of earlier conversations on the left, and the proof of the answer you are reading on the right when you open it. When the conversation service is installed (migration 034) the history,
// the answers, "Run again" with its comparison, forks, feedback and findings are kept in the case; otherwise the page keeps the history of this browser, as before.
export default function AskPage() {
  const { id: caseId = '' } = useParams()
  const [searchParams, setSearchParams] = useSearchParams()
  const reopened = searchParams.get('question') || ''
  const requestedScope = searchParams.get('scope')
  const [scope, setScope] = useEvidenceScope(caseId, 'investigate', requestedScope || 'all')
  const family = familyDefinition(scope)
  const canAskScope = scope === 'all' || Boolean(family.recordType)
  const [railQuery, setRailQuery] = useState('')
  const rail = useConversationRail(caseId, { query: railQuery })
  const serviceReady = rail.status === 'ready'
  // a question limited to one kind of record has no place in the service yet (it takes the question only), so it stays in this browser
  const useService = serviceReady && scope === 'all'
  const local = useAskThread(caseId)
  const served = useConversationThread(caseId, { enabled: useService, onChanged: rail.reload })
  const { turns, busy, ask, retry, stop, reset } = useService ? served : local
  const [diff, setDiff] = useState(null)
  // reference-image questions (an attached picture): answered by the image search of the case, kept on this page next to the conversation
  const [attachment, setAttachment] = useState(null)
  const [imageTurns, setImageTurns] = useState([])
  const imageRun = useRef(null)
  const [editing, setEditing] = useState(null)
  const [draft, setDraft] = useQuestionDraft(caseId)
  const overview = useCaseOverview(caseId)
  const history = useQuestionHistory(caseId)
  const { evidence, links } = useStarters(caseId, scope)
  const { title } = useCaseLabel(caseId)
  const [panelId, setPanelId] = useState(null)
  const [historyOpen, setHistoryOpen] = useState(() => (globalThis.innerWidth || 1440) >= 1280)
  const [copiedId, setCopiedId] = useState(null)
  const [atBottom, setAtBottom] = useState(true)
  const scroller = useRef(null)
  const composer = useRef(null)
  const pendingFocus = useRef(null)
  const panelTurn = [...turns, ...imageTurns].find(turn => turn.id === panelId) || null

  useEffect(() => { globalThis.document.title = `Ask · ${title || caseId} · NexusAI` }, [title, caseId])
  // Opening the evidence on a screen that cannot hold three columns folds the history away, so the conversation keeps room to be read.
  useEffect(() => { if (panelId && (globalThis.innerWidth || 1440) < 1700) setHistoryOpen(false) }, [panelId])

  const submittedScope = useCallback(() => ({ id: scope, label: scope === 'all' ? 'All evidence' : family.label, recordType: family.recordType || '' }), [scope, family])
  const askQuestion = useCallback((query, extra = {}) => {
    const turn = useService ? ask(query, extra.editOfTurnId ? { editOfTurnId: extra.editOfTurnId } : {}) : ask(query, { scope: submittedScope(), ...extra })
    return turn
  }, [ask, submittedScope, useService])

  // A link that carries a question and `run=1` (Connections, "Ask about this case") asks it once, here; without `run` it only fills the box.
  // the address is read once, when the page opens: a conversation the page itself starts writes its id there, and must not be opened again (that would cancel the question being sent)
  const opened = useRef(false)
  const initialThread = useRef(searchParams.get('thread'))
  useEffect(() => {
    if (opened.current || !initialThread.current || !useService) return
    opened.current = true
    served.open(initialThread.current)
  }, [useService]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    if (!served.threadId || !useService) return
    setSearchParams(previous => { if (previous.get('thread') === served.threadId) return previous; const next = new URLSearchParams(previous); next.set('thread', served.threadId); return next }, { replace: true })
  }, [served.threadId, useService]) // eslint-disable-line react-hooks/exhaustive-deps
  const autoRan = useRef(false)
  useEffect(() => { if (reopened) setDraft(reopened) }, [reopened, setDraft])
  useEffect(() => {
    if (autoRan.current || !reopened || searchParams.get('run') !== '1') return
    autoRan.current = true
    setDraft('')
    askQuestion(reopened)
  }, []) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { if (requestedScope) setScope(requestedScope) }, [requestedScope, setScope])
  // a box drawn on a picture in the evidence viewer arrives here as ?regionOf=<evidence>&box=x0,y0,x1,y1&run=1: search for that part once, then take the search out of the address
  const regionRan = useRef(false)
  useEffect(() => {
    const evidenceId = searchParams.get('regionOf')
    const box = (searchParams.get('box') || '').split(',').map(Number)
    if (regionRan.current || !evidenceId || searchParams.get('run') !== '1' || box.length !== 4 || box.some(value => !Number.isFinite(value))) return
    regionRan.current = true
    askImageAction('region', evidenceId, '', box)
    setSearchParams(previous => { const next = new URLSearchParams(previous); ['regionOf', 'box', 'run'].forEach(key => next.delete(key)); return next }, { replace: true })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  // New question: show it. Answer arrived: move focus to it, so a keyboard or screen-reader user lands on what they asked for.
  const lastCount = useRef(0)
  useEffect(() => {
    if (turns.length > lastCount.current) {
      const turn = turns[turns.length - 1]
      pendingFocus.current = `${turn.id}-anchor`
      globalThis.setTimeout(() => globalThis.document.getElementById(pendingFocus.current)?.scrollIntoView({ block: 'start', behavior: 'smooth' }), 30)
    }
    lastCount.current = turns.length
  }, [turns])
  useEffect(() => {
    if (busy || !pendingFocus.current) return
    const node = globalThis.document.getElementById(pendingFocus.current)
    pendingFocus.current = null
    node?.focus({ preventScroll: true })
  }, [turns, busy])

  async function runImageTurn(query, run) {
    const id = `image-turn-${Date.now()}`
    const scope = { id: 'all', label: 'All evidence' }
    setImageTurns(items => [...items, { id, query, scope, presentation: null, busy: true, startedAt: Date.now() }])
    imageRun.current?.abort()
    imageRun.current = new AbortController()
    try {
      const response = await run(imageRun.current.signal)
      const presentation = { ...presentInvestigationResponse(response, { caseId }), question: query }
      setImageTurns(items => items.map(turn => (turn.id === id ? { ...turn, presentation, busy: false, finishedAt: Date.now() } : turn)))
    } catch (error) {
      if (error.name === 'AbortError') { setImageTurns(items => items.map(turn => (turn.id === id ? { ...turn, busy: false, aborted: true } : turn))); return }
      const presentation = presentInvestigationResponse(null, { caseId, error })
      setImageTurns(items => items.map(turn => (turn.id === id ? { ...turn, presentation, serviceMessage: error.status === 409 || error.status === 503 || error.status === 400 ? error.message : '', busy: false, finishedAt: Date.now() } : turn)))
    }
  }
  function askImage(file, note) {
    const query = note ? `${note} (reference image: ${file.name})` : `Find exact and similar pictures to ${file.name}`
    return runImageTurn(query, signal => searchImages({ caseId, file, signal }))
  }
  // one click on a picture of an answer: its look-alikes, or its copies and look-alikes
  function askImageAction(action, evidenceId, name, box) {
    if (action === 'region') return runImageTurn('Find the selected part of a picture elsewhere in the case', signal => searchImages({ caseId, kind: 'reference_region', evidenceId, box, signal }))
    const more = action === 'more_like_this'
    return runImageTurn(more ? `Pictures that look like ${name}` : `Find exact and similar pictures to ${name}`, signal => searchImages({ caseId, kind: more ? 'more_like_this' : 'reference', evidenceId, signal }))
  }
  function submit(text, file) {
    if (file) { setDraft(''); setAttachment(null); askImage(file, text); return }
    const editOfTurnId = editing; setEditing(null); setDraft(''); askQuestion(text, editOfTurnId ? { editOfTurnId } : {})
  }
  function pick(item) {
    if (item.scope && item.scope !== scope) { setScope(item.scope) }
    setDraft(item.query)
    globalThis.setTimeout(() => composer.current?.focus(), 0)
  }
  function runHistory(entry) {
    setDraft('')
    askQuestion(entry.query)
  }
  function choose(option, clarification, turn) {
    if (!option?.query) return undefined
    return askQuestion(option.query, { parentId: turn.id, label: option.label || clarification?.originalQuestion || '' })
  }
  async function copyAnswer(turn) {
    try {
      await globalThis.navigator.clipboard.writeText(turn.presentation?.answer || '')
      setCopiedId(turn.id)
      globalThis.setTimeout(() => setCopiedId(current => (current === turn.id ? null : current)), 1600)
    } catch { setCopiedId(null) }
  }
  function watchScroll() {
    const node = scroller.current
    if (node) setAtBottom(node.scrollHeight - node.scrollTop - node.clientHeight < 160)
  }
  function newConversation() { reset(); setImageTurns([]); setAttachment(null); setPanelId(null); setEditing(null); setSearchParams(previous => { const next = new URLSearchParams(previous); next.delete('thread'); return next }, { replace: true }); globalThis.setTimeout(() => composer.current?.focus(), 0) }
  function openConversation(item) { setPanelId(null); setEditing(null); opened.current = true; served.open(item.thread_id) }
  async function compare(turn) {
    if (turn.diff) { setDiff(turn.diff); return }
    const latest = turn.reruns?.latest_turn_id
    if (!latest) return
    try { setDiff(await getTurnDiff({ caseId, turnId: latest })) } catch { /* the badge stays; the comparison could not be read */ }
  }
  async function fork(turn) {
    await served.fork(turn)
    setPanelId(null)
    setDraft('')
    globalThis.setTimeout(() => composer.current?.focus(), 0)
  }
  function changeScope(value) { stop(); reset(); setPanelId(null); setScope(value) }

  const unavailable = canAskScope ? '' : `${family.label} questions are not available yet. The question service cannot enforce this scope, so NexusAI has not widened the question to other evidence.`
  const dock = (
    <Composer value={draft} onChange={setDraft} onSubmit={submit} busy={busy} onStop={stop} scope={scope} onScope={changeScope} inputRef={composer} unavailable={unavailable}
      attachment={attachment} onAttach={setAttachment} onClearAttachment={() => setAttachment(null)} />
  )

  return (
    <AppShell caseId={caseId} rail="collapsed" railKind="ask">
      <main id="workspace-main" className="nx-ui nx-ask" tabIndex={-1} data-history={historyOpen ? 'open' : 'closed'} data-drawer={panelTurn ? 'open' : 'closed'}>
        {historyOpen ? (
          serviceReady
            ? <ConversationRail rail={rail} activeId={served.threadId} onOpen={openConversation} onNew={newConversation} query={railQuery} onQuery={setRailQuery} />
            : <HistoryRail caseId={caseId} entries={history} onPick={runHistory} onNew={newConversation} activeQuery={turns.at(-1)?.query} />
        ) : null}
        <section className="nx-ask__col" aria-label="Conversation">
          <div className="nx-ask__bar">
            <button type="button" className="nx-btn nx-btn--ghost nx-btn--sm" aria-pressed={historyOpen} onClick={() => setHistoryOpen(open => !open)}><History aria-hidden="true" />History</button>
            <span className="nx-ask__coverage" role="status">{coverageLine(overview) || 'Checking what this case holds…'}{serviceReady && scope !== 'all' ? ' Questions limited to one kind of record are kept in this browser only.' : ''}</span>
            {turns.length ? <button type="button" className="nx-btn nx-btn--ghost nx-btn--sm" onClick={newConversation}><MessageSquarePlus aria-hidden="true" />New conversation</button> : null}
          </div>
          <div className="nx-ask__scroll" ref={scroller} onScroll={watchScroll}>
            {useService && served.opening ? <p className="nx-muted nx-ask__opening" role="status">Opening the conversation…</p> : null}
            {useService && served.problem ? <p className="nx-callout nx-callout--danger" role="alert"><CircleAlert aria-hidden="true" />{served.problem}</p> : null}
            {useService && served.thread?.forked_from && !turns.length ? <p className="nx-callout nx-callout--info" role="status"><GitFork aria-hidden="true" />This is a fork. Your first question continues from the question you forked.</p> : null}
            {turns.length || imageTurns.length ? (
              <div className="nx-thread" role="log" aria-live="polite" aria-relevant="additions text" aria-label="Conversation">
                {turns.map((turn, index) => (
                  <Turn key={turn.id} turn={turn} index={index} last={index === turns.length - 1} caseId={caseId}
                    onImageAction={askImageAction} onAsk={query => askQuestion(query)} onChoose={(option, clarification) => choose(option, clarification, turn)} onRetry={() => retry(turn)}
                    onEdit={() => { setEditing(useService && turn.stored && index === turns.length - 1 ? turn.id : null); pick({ query: turn.query, scope: turn.scope?.id }) }} onCopy={() => copyAnswer(turn)} copied={copiedId === turn.id}
                    onRerun={useService ? () => served.rerun(turn) : undefined} onFork={useService ? () => fork(turn) : undefined}
                    onFeedback={useService ? verdict => served.feedback(turn, verdict) : undefined} onPin={useService ? () => served.pin(turn) : undefined} onCompare={() => compare(turn)}
                    evidenceOpen={panelId === turn.id} onToggleEvidence={() => setPanelId(panelId === turn.id ? null : turn.id)} />
                ))}
                {imageTurns.map((turn, index) => (
                  <Turn key={turn.id} turn={turn} index={turns.length + index} last={index === imageTurns.length - 1} caseId={caseId}
                    onImageAction={askImageAction} onAsk={query => askQuestion(query)} onChoose={() => undefined} onRetry={() => undefined} onEdit={() => undefined} onCopy={() => copyAnswer(turn)} copied={copiedId === turn.id}
                    onCompare={() => undefined} evidenceOpen={panelId === turn.id} onToggleEvidence={() => setPanelId(panelId === turn.id ? null : turn.id)} />
                ))}
              </div>
            ) : (
              <EmptyAsk caseId={caseId} overview={overview} links={links} evidence={evidence} recent={serviceReady ? rail.items.slice(0, 3).map(item => ({ id: item.thread_id, query: item.last_question || item.first_question || item.display_title, label: item.display_title, state: item.last_outcome })) : history.slice(0, 3)} onPick={pick}>
                <Composer large value={draft} onChange={setDraft} onSubmit={submit} busy={busy} onStop={stop} scope={scope} onScope={changeScope} inputRef={composer} unavailable={unavailable}
                  attachment={attachment} onAttach={setAttachment} onClearAttachment={() => setAttachment(null)} />
              </EmptyAsk>
            )}
            {!atBottom && turns.length ? <button type="button" className="nx-btn nx-jump" onClick={() => scroller.current?.scrollTo({ top: scroller.current.scrollHeight, behavior: 'smooth' })}><ArrowDown aria-hidden="true" />Latest</button> : null}
          </div>
          {turns.length || imageTurns.length ? <div className="nx-ask__dock">{dock}</div> : null}
        </section>
        {panelTurn ? <EvidenceDrawer turn={panelTurn} caseId={caseId} onClose={() => setPanelId(null)} /> : null}
        {diff ? <RunDiff diff={diff} onClose={() => setDiff(null)} /> : null}
      </main>
    </AppShell>
  )
}
