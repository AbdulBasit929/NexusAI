import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { ChevronDown, Layers, Sparkles } from 'lucide-react'
import { AppShell } from '../components/CaseShell.jsx'
import { AskInput } from '../components/AskInput.jsx'
import { EmptyState, LanguageText } from '../components/AnalystComponents.jsx'
import { PageHeader } from '../components/PageHeader.jsx'
import { QuestionHistory } from '../components/QuestionHistory.jsx'
import { AcrossResults } from './investigate/AcrossResults.jsx'
import { askCase, configuredCaseIds, getQueryCapabilities } from '../lib/apiClient.js'
import { askAcrossCases, planSearch, sortOutcomes } from '../lib/acrossCases.js'
import { curatedQuestions } from '../lib/dashboardCases.js'
import { formatNumber } from '../lib/format.js'
import { presentInvestigationResponse } from '../lib/investigationPresentation.js'
import { recordSessionActivity } from '../lib/sessionActivity.js'
import { summariseCase, useConfiguredCaseOverviews } from '../lib/useCaseOverview.js'
import { recordQuestionHistory } from '../lib/workspaceState.js'

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

  return (
    <AppShell>
      <main id="workspace-main" className="catalog-page global-investigate" tabIndex={-1}>
        <PageHeader title="Ask a question" description="Ask once. Every case you can open is searched, so you do not need to remember where the evidence is." />
        {rows.length ? (
          <>
            <section className="ax-ask" aria-label="Ask across your cases">
              <AskInput
                busy={busy}
                value={draft}
                onChange={setDraft}
                inputRef={composer}
                onAsk={ask}
                clearOnAsk={false}
                label="Your question"
                placeholder="For example: who contacted this number, and in which case?"
                ariaLabel="Ask across your cases"
              />
              <div className="ax-scope">
                <p role="status" className="ax-scope__line"><Layers aria-hidden="true" />{loading ? 'Checking which cases have evidence…' : searchScopeNote({ searching: plan.search.length, skipped: plan.skipped.length, ...totals })}</p>
                {cases.length > 1 ? (
                  <details className="ax-narrow">
                    <summary>Narrow to some cases<ChevronDown aria-hidden="true" /></summary>
                    <fieldset>
                      <legend className="visually-hidden">Cases to search</legend>
                      <p>Leave everything unticked to search all cases.</p>
                      <ul>
                        {rows.map(row => (
                          <li key={row.caseId}>
                            <label><input type="checkbox" checked={chosen.includes(row.caseId)} onChange={() => toggle(row.caseId)} /><LanguageText as="bdi" identifier>{row.caseId}</LanguageText><small>{coverage[row.caseId] || 'Checking…'}</small></label>
                          </li>
                        ))}
                      </ul>
                    </fieldset>
                  </details>
                ) : null}
                {busy ? <button type="button" className="ax-stop" onClick={stop}>Stop</button> : null}
              </div>
              {!run && suggestions.length ? (
                <div className="ax-suggest">
                  <span><Sparkles aria-hidden="true" />Try one of these</span>
                  <ul>{suggestions.map(item => <li key={item.query}><button type="button" onClick={() => { setDraft(item.query); composer.current?.focus() }}><LanguageText>{item.query}</LanguageText></button></li>)}</ul>
                </div>
              ) : null}
            </section>
            {run ? <AcrossResults run={run} coverage={coverage} onRetryCase={retryCase} /> : null}
            <QuestionHistory caseIds={cases} />
          </>
        ) : (
          <EmptyState kind="not-processed" label="No cases are configured" description="Add evidence to create a case before asking a question."><Link to="/cases/new">Add evidence</Link></EmptyState>
        )}
      </main>
    </AppShell>
  )
}
