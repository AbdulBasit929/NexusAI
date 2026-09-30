import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { CircleAlert, CircleCheckBig, CircleHelp, CircleSlash, ChevronDown, Clock3, Layers, LoaderCircle, MessageSquarePlus, SearchX, Square } from 'lucide-react'
import { AskInput } from '../components/AskInput.jsx'
import { InvestigationResult } from '../components/InvestigationResult.jsx'
import { QuestionTrail } from '../components/QuestionTrail.jsx'
import { askCase, getQueryCapabilities } from '../lib/apiClient.js'
import { presentInvestigationResponse } from '../lib/investigationPresentation.js'
import { CaseShell } from '../components/CaseShell.jsx'
import { recordSessionActivity } from '../lib/sessionActivity.js'
import { familyDefinition, ScopeChips, useEvidenceScope } from '../components/ScopeChips.jsx'
import { EmptyState, LanguageText } from '../components/AnalystComponents.jsx'
import { recordQuestionHistory, useQuestionDraft, useQuestionHistory } from '../lib/workspaceState.js'
import { ResultComparison } from '../components/ResultComparison.jsx'
import { CaseStart, coverageLine } from './investigate/CaseStart.jsx'
import { OUTCOME_LABEL } from '../lib/acrossCases.js'
import { useCaseOverview } from '../lib/useCaseOverview.js'

let turnSequence = 0
function nextTurnId() {
  turnSequence += 1
  return `turn-${turnSequence}`
}

const capabilityScope = {
  cdr: 'cdr', ipdr: 'ipdr', anpr: 'anpr', subscriber_identity: 'subscriber', subscriber: 'subscriber',
  tower_location: 'tower', tower: 'tower', financial: 'financial', transaction: 'financial',
  logs_access_security: 'access_log', access_log: 'access_log', document: 'document', images: 'image',
  image: 'image', audio_and_stt: 'audio', audio: 'audio', video: 'video',
}

function starterScope(family) {
  const recordType = Array.isArray(family?.record_types) ? family.record_types[0] : ''
  return capabilityScope[family?.id] || capabilityScope[recordType] || 'all'
}

// Suggestions are projected only from the service's curated capability response.
// No plausible-looking sample question is invented by the browser.
export function investigationStarters(payload, selectedScope = 'all') {
  const seen = new Set()
  return (Array.isArray(payload?.families) ? payload.families : [])
    .filter(family => ['queryable', 'semantic_only', 'limited'].includes(family?.availability))
    .flatMap(family => (Array.isArray(family?.suggested_queries) ? family.suggested_queries : []).map(query => ({
      query: String(query),
      family: family.label || String(family.id || ''),
      scope: starterScope(family),
    })))
    .filter(item => selectedScope === 'all' || item.scope === selectedScope)
    .filter(item => {
      const key = `${item.scope}:${item.query.toLocaleLowerCase()}`
      if (!item.query || seen.has(key)) return false
      seen.add(key)
      return true
    })
    .slice(0, 4)
}

function NexusAnswerMark() {
  return <span className="assistant-mark" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="M6 18V6l12 12V6" /></svg></span>
}

export default function InvestigatePage() {
  const { id: caseId = '' } = useParams()
  const [searchParams] = useSearchParams()
  const reopenedQuestion = searchParams.get('question') || ''
  const requestedScope = searchParams.get('scope')
  const [scope, setScope] = useEvidenceScope(caseId, 'investigate', requestedScope || 'all')
  const selectedFamily = familyDefinition(scope)
  const canAskScope = scope === 'all' || Boolean(selectedFamily.recordType)
  // THE THREAD. Previously one `presentation` that each new question replaced,
  // so the analyst lost the answer they were reading the moment they followed
  // up on it. An investigation is a sequence -- a question, what it returned,
  // and the question that returned suggests -- and the reasoning is in the
  // sequence, not in any single answer.
  const [turns, setTurns] = useState([])
  const [comparison, setComparison] = useState([])
  const [draft, setDraft] = useQuestionDraft(caseId)
  const [capabilities, setCapabilities] = useState(null)
  const overview = useCaseOverview(caseId)
  const recent = useQuestionHistory(caseId).slice(0, 3)
  const [scopeOpen, setScopeOpen] = useState(false)
  const controller = useRef(null)
  const composer = useRef(null)
  const pendingFocus = useRef(null)

  const busy = turns.some(turn => turn.busy)
  const starters = useMemo(() => investigationStarters(capabilities, scope), [capabilities, scope])
  const decisionTrail = useMemo(() => turns.flatMap(turn => {
    if (!turn.parentId) return []
    const parent = turns.find(candidate => candidate.id === turn.parentId)
    return parent ? [{ originalQuestion: parent.query, label: turn.label || 'Clarification selected', query: turn.query }] : []
  }), [turns])

  useEffect(() => { if (reopenedQuestion) setDraft(reopenedQuestion) }, [reopenedQuestion, setDraft])
  useEffect(() => { if (requestedScope) setScope(requestedScope) }, [requestedScope, setScope])
  useEffect(() => {
    const request = new AbortController()
    getQueryCapabilities({ caseId, signal: request.signal })
      .then(setCapabilities)
      .catch(error => { if (error.name !== 'AbortError') setCapabilities(null) })
    return () => request.abort()
  }, [caseId])
  // Move focus to the answer that just arrived, not to the bottom of the page:
  // a keyboard or screen-reader user must land on the content they asked for.
  useEffect(() => {
    if (busy || !pendingFocus.current) return
    const node = globalThis.document?.getElementById(pendingFocus.current)
    pendingFocus.current = null
    node?.focus({ preventScroll: true })
    node?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
  }, [turns, busy])

  function updateTurn(id, patch) {
    setTurns(items => items.map(turn => (turn.id === id ? { ...turn, ...patch } : turn)))
  }

  async function runTurn(id, query, submittedScope) {
    controller.current?.abort()
    controller.current = new AbortController()
    pendingFocus.current = `${id}-anchor`
    try {
      const response = await askCase({ caseId, query, recordType: submittedScope?.recordType, signal: controller.current.signal })
      const presentation = presentInvestigationResponse(response, { caseId })
      updateTurn(id, { presentation, busy: false })
      recordSessionActivity({ caseId, query, presentation })
      recordQuestionHistory(caseId, query, presentation)
    } catch (error) {
      if (error.name === 'AbortError') {
        updateTurn(id, { busy: false, aborted: true })
        return
      }
      const presentation = presentInvestigationResponse(null, { caseId, error })
      updateTurn(id, { presentation, busy: false })
      recordSessionActivity({ caseId, query, presentation })
      recordQuestionHistory(caseId, query, presentation)
    }
  }

  // `parentId` records that this question came from a clarification of an
  // earlier one, so the thread can show the relationship instead of presenting
  // it as an unrelated new question.
  function ask(query, { parentId = null, label = '' } = {}) {
    const id = nextTurnId()
    const submittedScope = { id: scope, label: scope === 'all' ? 'All evidence' : selectedFamily.label, recordType: selectedFamily.recordType || '' }
    setTurns(items => [...items, { id, query, label, parentId, scope: submittedScope, presentation: null, busy: true }])
    return runTurn(id, query, submittedScope)
  }

  function retryTurn(turn) {
    updateTurn(turn.id, { busy: true, presentation: null })
    return runTurn(turn.id, turn.query, turn.scope)
  }

  function editQuestion(query) {
    setDraft(query)
    globalThis.setTimeout(() => composer.current?.focus(), 0)
  }

  function addComparison(turn) {
    if (!turn?.presentation) return
    const query = turn.query || turn.presentation.originalQuestion || 'Result'
    setComparison(items => [...items.filter(item => item.query !== query), { id: `${Date.now()}-${query}`, query, presentation: turn.presentation }].slice(-2))
  }

  function chooseClarification(option, clarification, turn) {
    if (!option?.query) return undefined
    return ask(option.query, { parentId: turn.id, label: option.label || clarification?.originalQuestion || '' })
  }

  function clearThread() {
    controller.current?.abort()
    setTurns([])
  }

  function stopQuestion() {
    controller.current?.abort()
  }

  function changeScope(value) {
    controller.current?.abort()
    setTurns([])
    setComparison([])
    setScope(value)
    setScopeOpen(false)
  }

  function chooseStarter(item) {
    if (item.scope !== scope) changeScope(item.scope)
    setDraft(item.query)
    globalThis.setTimeout(() => composer.current?.focus(), 0)
  }

  const turnIcon = { answered: CircleCheckBig, partial: CircleCheckBig, clarify: CircleHelp, processing: Clock3, 'zero-result': SearchX, unsupported: CircleSlash, failed: CircleAlert }
  const composerBlock = (
    <div className="gi-composer">
      {canAskScope
        ? <AskInput busy={busy} value={draft} onChange={setDraft} inputRef={composer} onAsk={query => ask(query)} clearOnAsk={!turns.length ? false : true} label="Ask a question about this case" placeholder="Ask about this case’s evidence…" ariaLabel="Ask about case evidence" />
        : <EmptyState kind="unavailable" label={`${selectedFamily.label} questions are unavailable`} description="The question service cannot yet enforce this family scope. NexusAI has not widened the request to other evidence." />}
      <div className="gi-scope">
        <details className="thread-composer__scope gi-scope__pop" open={scopeOpen} onToggle={event => setScopeOpen(event.currentTarget.open)}>
          <summary><Layers aria-hidden="true" /><span>Scope</span> <strong>{scope === 'all' ? 'All evidence' : selectedFamily.label}</strong><ChevronDown aria-hidden="true" /></summary>
          <div className="gi-scope__panel">
            <ScopeChips value={scope} onChange={changeScope} label="Evidence family for the next question" />
            <p><strong>{scope === 'all' ? 'All evidence' : selectedFamily.label}</strong> applies to the next question and stays selected in this browser.</p>
          </div>
        </details>
        <p className="gi-scope__line" role="status">{coverageLine(overview) || 'Checking what this case holds…'}</p>
        {busy ? <button type="button" className="gi-stop thread-composer__stop" onClick={stopQuestion}><Square aria-hidden="true" />Stop current question</button> : null}
        {turns.length > 0 ? <button type="button" className="gi-stop thread-composer__clear" onClick={clearThread}><MessageSquarePlus aria-hidden="true" />Start a new conversation</button> : null}
      </div>
    </div>
  )

  return (
    <CaseShell caseId={caseId}>
      <main id="workspace-main" className={`investigate-page catalog-page gi gi--case ${turns.length ? 'gi--results investigate-page--answered' : 'gi--landing'}`} tabIndex={-1}>
        {!turns.length ? (
          <>
            <ResultComparison items={comparison} onRemove={id => setComparison(items => items.filter(item => item.id !== id))} onClear={() => setComparison([])} />
            <CaseStart caseId={caseId} overview={overview} starters={starters} recent={recent} onPick={chooseStarter}>{composerBlock}</CaseStart>
          </>
        ) : (
          <div className="gi-case">
            <div className="gi-case__main">
              <h1 className="visually-hidden">Investigate {caseId}</h1>
              <ResultComparison items={comparison} onRemove={id => setComparison(items => items.filter(item => item.id !== id))} onClear={() => setComparison([])} />
              <QuestionTrail entries={decisionTrail} history={[]} />
              {/* One live region for the whole thread, so a dozen answers do not compete to interrupt a screen reader. */}
              <ol className="investigation-thread" role="log" aria-live="polite" aria-relevant="additions text" aria-label="Investigation conversation">
                {turns.map((turn, index) => (
                  <li key={turn.id} className={`thread-turn${turn.parentId ? ' thread-turn--branched' : ''}`}>
                    <article className="thread-turn__question" aria-label={`Question ${index + 1}`}>
                      <div className="thread-turn__meta">
                        <strong>You asked</strong>
                        <span>{turn.scope?.label || 'All evidence'} scope</span>
                      </div>
                      {turn.parentId ? <p className="thread-turn__branch">Following the clarification{turn.label ? <> · <LanguageText>{turn.label}</LanguageText></> : null}</p> : null}
                      <p className="thread-turn__text"><LanguageText>{turn.query}</LanguageText></p>
                      <button type="button" className="thread-turn__edit" onClick={() => editQuestion(turn.query)}>Ask a variation</button>
                    </article>
                    <div id={`${turn.id}-anchor`} tabIndex={-1} className="thread-turn__answer">
                      <header className="thread-turn__answer-identity"><NexusAnswerMark /><span><strong>NexusAI</strong><small>Evidence analysis</small></span></header>
                      {turn.busy && <p className="processing-announcement" role="status"><LoaderCircle aria-hidden="true" />Checking {turn.scope?.label || 'the selected'} case evidence…</p>}
                      {turn.aborted && !turn.busy && <p className="thread-turn__aborted" role="status">This question was stopped before a result was returned.</p>}
                      {turn.presentation && (
                        <InvestigationResult
                          presentation={turn.presentation}
                          idPrefix={turn.id}
                          live={false}
                          onAsk={query => ask(query)}
                          onClarificationChoice={(option, clarification) => chooseClarification(option, clarification, turn)}
                          onRetry={() => retryTurn(turn)}
                          onCompare={() => addComparison(turn)}
                          clarificationRounds={turn.parentId ? 1 : 0}
                          showOriginalQuestion={false}
                        />
                      )}
                    </div>
                  </li>
                ))}
              </ol>
              <div className="gi-dock thread-composer">{composerBlock}</div>
            </div>

            <aside className="gi-case__rail" aria-label="About this conversation">
              <section className="gi-rail">
                <h2 className="gi-rail__title">This case</h2>
                <p className="gi-rail__note">{coverageLine(overview) || 'Checking what this case holds…'}</p>
                <p className="gi-rail__links"><Link to={`/cases/${encodeURIComponent(caseId)}/overview`}>Overview</Link><Link to={`/cases/${encodeURIComponent(caseId)}/evidence`}>Evidence</Link><Link to="/investigate">Ask across all cases</Link></p>
              </section>
              <section className="gi-rail">
                <h2 className="gi-rail__title">Conversation <span>{turns.length}</span></h2>
                <ol className="gi-rail__list" aria-label="Questions in this conversation">
                  {turns.map((turn, index) => {
                    const state = turn.presentation?.state
                    const Icon = turn.busy ? LoaderCircle : turnIcon[state] || CircleHelp
                    return (
                      <li key={turn.id} className={`gi-rail__item ${turn.busy ? 'gi-rail__item--pending' : `gi-rail__item--${state || 'clarify'}`}`}>
                        <a href={`#${turn.id}-anchor`} onClick={event => { event.preventDefault(); globalThis.document?.getElementById(`${turn.id}-anchor`)?.scrollIntoView({ behavior: 'smooth', block: 'start' }) }}>
                          <span className="gi-rail__mark" aria-hidden="true"><Icon /></span>
                          <span className="gi-rail__text"><span className="gi-rail__q"><LanguageText>{turn.query}</LanguageText></span><small>{turn.busy ? 'Searching…' : turn.aborted ? 'Stopped' : OUTCOME_LABEL[state] || 'Answered'}</small></span>
                        </a>
                      </li>
                    )
                  })}
                </ol>
              </section>
            </aside>
          </div>
        )}
      </main>
    </CaseShell>
  )
}
