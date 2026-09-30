import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { ChevronDown, Layers, Sparkles, Square, History } from 'lucide-react'
import { AppShell } from '../components/CaseShell.jsx'
import { AskInput } from '../components/AskInput.jsx'
import { EmptyState, LanguageText } from '../components/AnalystComponents.jsx'
import { AcrossResults } from './investigate/AcrossResults.jsx'
import { EvidencePanel } from './investigate/EvidencePanel.jsx'
import { CaseRail } from './investigate/CaseRail.jsx'
import { askCase, configuredCaseIds, getQueryCapabilities } from '../lib/apiClient.js'
import { askAcrossCases, planSearch, sortOutcomes } from '../lib/acrossCases.js'
import { curatedQuestions } from '../lib/dashboardCases.js'
import { formatNumber } from '../lib/format.js'
import { presentInvestigationResponse } from '../lib/investigationPresentation.js'
import { recordSessionActivity } from '../lib/sessionActivity.js'
import { summariseCase, useConfiguredCaseOverviews } from '../lib/useCaseOverview.js'
import { recordQuestionHistory, useQuestionHistoryAcross } from '../lib/workspaceState.js'

export function investigateTarget(caseId, question) {
  return `/cases/${encodeURIComponent(caseId)}/investigate?question=${encodeURIComponent(question.trim())}`
}

// What a question will and will not search, in words: every case with evidence, and the sources in each that are ready.
export function searchScopeNote({ searching, skipped, sources, ready }) {
  if (!searching) return 'No case has evidence to search yet.'
  const cases = `${formatNumber(searching)} ${searching === 1 ? 'case' : 'cases'}`
  const notReady = sources - ready
  const coverage = notReady > 0 ? `${formatNumber(ready)} of ${formatNumber(sources)} sources are ready; the rest are not searched until they finish.` : `all ${formatNumber(sources)} ${sources === 1 ? 'source is' : 'sources are'} ready.`
  return `Searching ${cases}${skipped ? `, ${formatNumber(skipped)} skipped` : ''}: ${coverage}`
}

const coverageOf = summary => (summary ? `${formatNumber(summary.ready)} of ${formatNumber(summary.total)} sources ready` : '')

// The one place to ask. The analyst does not need to remember which case holds what: a question goes to every case with
// evidence, and the answer comes back per case, best first, with the coverage of each. Narrowing to chosen cases is
// optional. Rules: UI_REDESIGN_BRIEF §11, team-lead ruling on cross-case Investigate; the server-side scope is request 11.
export default function GlobalInvestigatePage() {
  const cases = configuredCaseIds()
  const { states } = useConfiguredCaseOverviews(cases)
  const [searchParams] = useSearchParams()
  const [draft, setDraft] = useState(searchParams.get('question') || '')
  const [chosen, setChosen] = useState([])
  const [run, setRun] = useState(null)
  const [panelCase, setPanelCase] = useState(null)
  const [suggestions, setSuggestions] = useState([])
  const controller = useRef(null)
  const composer = useRef(null)

  const rows = useMemo(() => cases.map(caseId => {
    const data = states[caseId]?.data
    return { caseId, summary: data ? summariseCase(data) : null }
  }), [cases, states])
  const plan = useMemo(() => planSearch(rows, chosen), [rows, chosen])
  const coverage = useMemo(() => Object.fromEntries(rows.map(row => [row.caseId, coverageOf(row.summary)])), [rows])
  const totals = useMemo(() => rows.filter(row => plan.search.includes(row.caseId) && row.summary).reduce((sum, row) => ({ sources: sum.sources + row.summary.total, ready: sum.ready + row.summary.ready }), { sources: 0, ready: 0 }), [rows, plan])
  const loading = rows.some(row => !row.summary && !states[row.caseId]?.error)
  const busy = Boolean(run?.busy)

  // Suggestions: what the first few ready cases say they can answer, deduplicated. Nothing is invented in the browser.
  const suggestFrom = rows.filter(row => row.summary?.processingState === 'complete').slice(0, 3).map(row => row.caseId).join('|')
  useEffect(() => {
    if (!suggestFrom) return undefined
    const request = new AbortController()
    Promise.all(suggestFrom.split('|').map(caseId => getQueryCapabilities({ caseId, signal: request.signal }).then(curatedQuestions).catch(() => [])))
      .then(lists => {
        const seen = new Set()
        setSuggestions(lists.flat().filter(item => { const key = item.query.toLocaleLowerCase(); if (seen.has(key)) return false; seen.add(key); return true }).slice(0, 4))
      })
    return () => request.abort()
  }, [suggestFrom])

  const search = useCallback(async (query, caseIds, keep = []) => {
    controller.current?.abort()
    controller.current = new AbortController()
    const { signal } = controller.current
    setPanelCase(null)
    setRun({ query, caseIds: [...new Set([...keep.map(outcome => outcome.caseId), ...caseIds])], skipped: plan.skipped, outcomes: keep, busy: true })
    const outcomes = await askAcrossCases({
      caseIds,
      signal,
      ask: caseId => askCase({ caseId, query, signal }),
      present: (response, caseId, error) => presentInvestigationResponse(response, { caseId, error }),
      onOutcome: outcome => {
        if (signal.aborted) return
        setRun(current => (current?.query === query ? { ...current, outcomes: [...current.outcomes.filter(item => item.caseId !== outcome.caseId), outcome] } : current))
        if (['answered', 'partial'].includes(outcome.presentation.state)) recordSessionActivity({ caseId: outcome.caseId, query, presentation: outcome.presentation })
      },
    })
    if (signal.aborted) return
    // The question is saved once, under the case that answered best, so recent questions do not list it once per case.
    const best = sortOutcomes([...keep, ...outcomes]).find(outcome => ['answered', 'partial'].includes(outcome.presentation.state))
    if (best) recordQuestionHistory(best.caseId, query, best.presentation)
    setRun(current => (current?.query === query ? { ...current, busy: false } : current))
  }, [plan.skipped])

  const ask = useCallback(query => search(query, plan.search), [search, plan.search])
  const retryCase = useCallback(caseId => {
    if (!run) return
    return search(run.query, [caseId], run.outcomes.filter(outcome => outcome.caseId !== caseId))
  }, [run, search])
  const stop = () => { controller.current?.abort(); setRun(current => (current ? { ...current, busy: false } : current)) }
  const toggle = caseId => setChosen(current => (current.includes(caseId) ? current.filter(id => id !== caseId) : [...current, caseId]))
  useEffect(() => () => controller.current?.abort(), [])

  const recent = useQuestionHistoryAcross(cases).slice(0, 4)
  const scopeLabel = chosen.length ? `${formatNumber(chosen.length)} of ${formatNumber(cases.length)} cases` : cases.length === 1 ? '1 case' : `All ${formatNumber(cases.length)} cases`

  const composerBlock = (
    <div className="gi-composer">
      <AskInput
        busy={busy}
        value={draft}
        onChange={setDraft}
        inputRef={composer}
        onAsk={ask}
        clearOnAsk={false}
        label="Your question"
        placeholder="Ask about any evidence, in any case…"
        ariaLabel="Ask across your cases"
      />
      <div className="gi-scope">
        {cases.length > 1 ? (
          <details className="gi-scope__pop">
            <summary><Layers aria-hidden="true" /><span>Scope: {scopeLabel}</span><small className="visually-hidden">Narrow to some cases</small><ChevronDown aria-hidden="true" /></summary>
            <fieldset>
              <legend>Cases to search</legend>
              <p>Leave everything unticked to search all cases.</p>
              <ul>
                {rows.map(row => (
                  <li key={row.caseId}>
                    <label><input type="checkbox" checked={chosen.includes(row.caseId)} onChange={() => toggle(row.caseId)} /><LanguageText as="bdi" identifier>{row.caseId}</LanguageText><small>{coverage[row.caseId] || 'Checking…'}</small></label>
                  </li>
                ))}
              </ul>
              {chosen.length ? <button type="button" className="gi-scope__clear" onClick={() => setChosen([])}>Search all cases</button> : null}
            </fieldset>
          </details>
        ) : null}
        <p role="status" className="ax-scope__line gi-scope__line">{loading ? 'Checking which cases have evidence…' : searchScopeNote({ searching: plan.search.length, skipped: plan.skipped.length, ...totals })}</p>
        {busy ? <button type="button" className="gi-stop" onClick={stop}><Square aria-hidden="true" />Stop</button> : null}
      </div>
    </div>
  )

  return (
    <AppShell>
      <main id="workspace-main" className={`catalog-page gi${run ? ' gi--results' : ' gi--landing'}`} tabIndex={-1}>
        {rows.length ? (
          run ? (
            <div className={`gi-layout${panelCase ? ' gi-layout--panel' : ''}`}>
              <div className="gi-layout__bar">
                <h1 className="gi-layout__title">Ask a question</h1>
                {composerBlock}
              </div>
              <CaseRail run={run} coverage={coverage} />
              <div className="gi-layout__main"><AcrossResults run={run} coverage={coverage} onRetryCase={retryCase} panelCase={panelCase} onEvidence={setPanelCase} /></div>
              {panelCase && run.outcomes.find(outcome => outcome.caseId === panelCase) ? <EvidencePanel turn={{ id: panelCase, query: run.query, presentation: run.outcomes.find(outcome => outcome.caseId === panelCase).presentation }} caseLabel={panelCase} onClose={() => setPanelCase(null)} /> : null}
            </div>
          ) : (
            <section className="gi-hero" aria-labelledby="gi-title">
              <p className="gi-hero__eyebrow"><Sparkles aria-hidden="true" />Across every case you can open</p>
              <h1 id="gi-title">Ask a question</h1>
              <p className="gi-hero__lede">Ask once. Every case you can open is searched, so you do not need to remember where the evidence is.</p>
              {composerBlock}
              {suggestions.length ? (
                <div className="gi-suggest">
                  <h2>Try one of these</h2>
                  <ul>{suggestions.map(item => <li key={item.query}><button type="button" onClick={() => { setDraft(item.query); composer.current?.focus() }}><LanguageText>{item.query}</LanguageText></button></li>)}</ul>
                </div>
              ) : null}
              {recent.length ? (
                <div className="gi-recent">
                  <h2><History aria-hidden="true" />Your questions</h2>
                  <ul>{recent.map(entry => <li key={`${entry.caseId}-${entry.id}`}><Link to={investigateTarget(entry.caseId, entry.query)}><span><LanguageText>{entry.label || entry.query}</LanguageText></span><small><bdi dir="ltr">{entry.caseId}</bdi></small></Link></li>)}</ul>
                </div>
              ) : null}
            </section>
          )
        ) : (
          <EmptyState kind="not-processed" label="No cases are configured" description="Add evidence to create a case before asking a question."><Link to="/cases/new">Add evidence</Link></EmptyState>
        )}
      </main>
    </AppShell>
  )
}
