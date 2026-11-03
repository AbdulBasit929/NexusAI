import { useCallback, useEffect, useRef, useState } from 'react'
import { useParams, useSearchParams } from 'react-router-dom'
import { ArrowDown, History, MessageSquarePlus } from 'lucide-react'
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
import { Turn } from './Turn.jsx'
import { useAskThread } from './useAskThread.js'
import { useStarters } from './starters.js'

// Ask: a question in plain words, an answer that shows its evidence. One column for the conversation that uses the width it is given, a history
// of earlier questions on the left, and the proof of the answer you are reading on the right when you open it.
export default function AskPage() {
  const { id: caseId = '' } = useParams()
  const [searchParams] = useSearchParams()
  const reopened = searchParams.get('question') || ''
  const requestedScope = searchParams.get('scope')
  const [scope, setScope] = useEvidenceScope(caseId, 'investigate', requestedScope || 'all')
  const family = familyDefinition(scope)
  const canAskScope = scope === 'all' || Boolean(family.recordType)
  const { turns, busy, ask, retry, stop, reset } = useAskThread(caseId)
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
  const panelTurn = turns.find(turn => turn.id === panelId) || null

  useEffect(() => { globalThis.document.title = `Ask · ${title || caseId} · NexusAI` }, [title, caseId])

  const submittedScope = useCallback(() => ({ id: scope, label: scope === 'all' ? 'All evidence' : family.label, recordType: family.recordType || '' }), [scope, family])
  const askQuestion = useCallback((query, extra = {}) => {
    const turn = ask(query, { scope: submittedScope(), ...extra })
    return turn
  }, [ask, submittedScope])

  // A link that carries a question and `run=1` (Connections, "Ask about this case") asks it once, here; without `run` it only fills the box.
  const autoRan = useRef(false)
  useEffect(() => { if (reopened) setDraft(reopened) }, [reopened, setDraft])
  useEffect(() => {
    if (autoRan.current || !reopened || searchParams.get('run') !== '1') return
    autoRan.current = true
    setDraft('')
    askQuestion(reopened)
  }, []) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { if (requestedScope) setScope(requestedScope) }, [requestedScope, setScope])

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

  function submit(text) { setDraft(''); askQuestion(text) }
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
  function newConversation() { reset(); setPanelId(null); globalThis.setTimeout(() => composer.current?.focus(), 0) }
  function changeScope(value) { stop(); reset(); setPanelId(null); setScope(value) }

  const unavailable = canAskScope ? '' : `${family.label} questions are not available yet. The question service cannot enforce this scope, so NexusAI has not widened the question to other evidence.`
  const dock = (
    <Composer value={draft} onChange={setDraft} onSubmit={submit} busy={busy} onStop={stop} scope={scope} onScope={changeScope} inputRef={composer} unavailable={unavailable} />
  )

  return (
    <AppShell caseId={caseId} rail="collapsed" railKind="ask">
      <main id="workspace-main" className="nx-ask" tabIndex={-1} data-history={historyOpen ? 'open' : 'closed'} data-drawer={panelTurn ? 'open' : 'closed'}>
        {historyOpen ? (
          <HistoryRail caseId={caseId} entries={history} onPick={runHistory} onNew={newConversation} activeQuery={turns.at(-1)?.query} />
        ) : null}
        <section className="nx-ask__col" aria-label="Conversation">
          <div className="nx-ask__bar">
            <button type="button" className="nx-btn nx-btn--ghost nx-btn--sm" aria-pressed={historyOpen} onClick={() => setHistoryOpen(open => !open)}><History aria-hidden="true" />History</button>
            <span className="nx-ask__coverage" role="status">{coverageLine(overview) || 'Checking what this case holds…'}</span>
            {turns.length ? <button type="button" className="nx-btn nx-btn--ghost nx-btn--sm" onClick={newConversation}><MessageSquarePlus aria-hidden="true" />New conversation</button> : null}
          </div>
          <div className="nx-ask__scroll" ref={scroller} onScroll={watchScroll}>
            {turns.length ? (
              <div className="nx-thread" role="log" aria-live="polite" aria-relevant="additions text" aria-label="Conversation">
                {turns.map((turn, index) => (
                  <Turn key={turn.id} turn={turn} index={index} last={index === turns.length - 1} caseId={caseId}
                    onAsk={query => askQuestion(query)} onChoose={(option, clarification) => choose(option, clarification, turn)} onRetry={() => retry(turn)}
                    onEdit={() => pick({ query: turn.query, scope: turn.scope?.id })} onCopy={() => copyAnswer(turn)} copied={copiedId === turn.id}
                    evidenceOpen={panelId === turn.id} onToggleEvidence={() => setPanelId(panelId === turn.id ? null : turn.id)} />
                ))}
              </div>
            ) : (
              <EmptyAsk caseId={caseId} overview={overview} links={links} evidence={evidence} recent={history.slice(0, 3)} onPick={pick}>
                <Composer large value={draft} onChange={setDraft} onSubmit={submit} busy={busy} onStop={stop} scope={scope} onScope={changeScope} inputRef={composer} unavailable={unavailable} />
              </EmptyAsk>
            )}
            {!atBottom && turns.length ? <button type="button" className="nx-btn nx-jump" onClick={() => scroller.current?.scrollTo({ top: scroller.current.scrollHeight, behavior: 'smooth' })}><ArrowDown aria-hidden="true" />Latest</button> : null}
          </div>
          {turns.length ? <div className="nx-ask__dock">{dock}</div> : null}
        </section>
        {panelTurn ? <EvidenceDrawer turn={panelTurn} caseId={caseId} onClose={() => setPanelId(null)} /> : null}
      </main>
    </AppShell>
  )
}
