import { useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ArrowRight, CircleAlert, CircleCheckBig, Clock3, Folder } from 'lucide-react'
import { AppShell } from '../components/CaseShell.jsx'
import { EmptyState, LanguageText } from '../components/AnalystComponents.jsx'
import { PageHeader } from '../components/PageHeader.jsx'
import { QuestionHistory } from '../components/QuestionHistory.jsx'
import { configuredCaseIds } from '../lib/apiClient.js'
import { formatNumber } from '../lib/format.js'
import { summariseCase, useConfiguredCaseOverviews } from '../lib/useCaseOverview.js'

const STATE = {
  complete: { label: 'Ready', Icon: CircleCheckBig },
  processing: { label: 'Processing', Icon: Clock3 },
  attention: { label: 'Needs review', Icon: CircleAlert },
  'not-processed': { label: 'No evidence', Icon: Folder },
}

export function investigateTarget(caseId, question) {
  return `/cases/${encodeURIComponent(caseId)}/investigate?question=${encodeURIComponent(question.trim())}`
}

export function caseScopeNote(summary) {
  if (!summary) return 'Checking this case’s evidence.'
  if (summary.total === 0) return 'This case has no evidence yet, so a question cannot be answered from it.'
  if (summary.inFlight > 0 || summary.failed > 0) return `Answers cover the ${formatNumber(summary.ready)} ready source${summary.ready === 1 ? '' : 's'} only; ${formatNumber(summary.inFlight + (summary.failed || 0))} ${summary.inFlight + (summary.failed || 0) === 1 ? 'is' : 'are'} not ready and will not be searched.`
  return `Answers cover all ${formatNumber(summary.total)} ready source${summary.total === 1 ? '' : 's'} in this case.`
}

export default function GlobalInvestigatePage() {
  const cases = configuredCaseIds()
  const navigate = useNavigate()
  const { states } = useConfiguredCaseOverviews(cases)
  const [chosen, setChosen] = useState('')
  const [question, setQuestion] = useState('')
  const rows = useMemo(() => cases.map(caseId => {
    const data = states[caseId]?.data
    return { caseId, summary: data ? summariseCase(data) : null, failed: Boolean(states[caseId]?.error) }
  }), [cases, states])
  const selected = rows.find(row => row.caseId === chosen) || rows.find(row => row.summary?.processingState === 'complete') || rows[0]

  function submit(event) {
    event.preventDefault()
    if (!selected || !question.trim()) return
    navigate(investigateTarget(selected.caseId, question))
  }

  return (
    <AppShell>
      <main id="workspace-main" className="catalog-page global-investigate" tabIndex={-1}>
        <PageHeader eyebrow="Investigate" title="Ask a question" description="Each question is answered from one case’s evidence. Choose the case, then ask." />
        {rows.length ? (
          <form className="global-investigate__form" onSubmit={submit}>
            <fieldset>
              <legend>1. Choose the case to search</legend>
              <div className="global-investigate__cases">
                {rows.map(row => {
                  const state = STATE[row.summary?.processingState] || { label: row.failed ? 'Status unavailable' : 'Checking', Icon: Clock3 }
                  const active = selected?.caseId === row.caseId
                  return (
                    <label key={row.caseId} className={`global-investigate__case${active ? ' is-selected' : ''}`}>
                      <input type="radio" name="case" value={row.caseId} checked={active} onChange={() => setChosen(row.caseId)} />
                      <span className="global-investigate__case-name"><LanguageText as="bdi" identifier>{row.caseId}</LanguageText></span>
                      <span className="global-investigate__case-state"><state.Icon aria-hidden="true" />{state.label}{row.summary ? ` · ${formatNumber(row.summary.ready)} of ${formatNumber(row.summary.total)} ready` : ''}</span>
                    </label>
                  )
                })}
              </div>
            </fieldset>
            <div className="global-investigate__ask">
              <label htmlFor="global-question">2. Your question</label>
              <textarea id="global-question" rows="3" value={question} onChange={event => setQuestion(event.target.value)} placeholder="Ask about the case’s evidence…" onKeyDown={event => { if (event.key === 'Enter' && !event.shiftKey && !event.nativeEvent?.isComposing) { event.preventDefault(); event.currentTarget.form?.requestSubmit() } }} />
              <p className="global-investigate__scope" role="status"><strong>Scope: 1 case.</strong> {caseScopeNote(selected?.summary)}</p>
              <button type="submit" className="global-investigate__submit" disabled={!question.trim() || selected?.summary?.total === 0}>Ask in <LanguageText as="bdi" identifier>{selected?.caseId}</LanguageText><ArrowRight aria-hidden="true" /></button>
            </div>
          </form>
        ) : (
          <EmptyState kind="not-processed" label="No cases are configured" description="Add evidence to create a case before asking a question."><Link to="/cases/new">Add evidence</Link></EmptyState>
        )}
        {rows.length ? <QuestionHistory caseIds={cases} /> : null}
      </main>
    </AppShell>
  )
}
