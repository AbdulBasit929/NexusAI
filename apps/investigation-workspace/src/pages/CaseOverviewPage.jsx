import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { ArrowRight, CircleAlert, CircleCheckBig, Clock3, Plus, Send } from 'lucide-react'
import { CaseShell } from '../components/CaseShell.jsx'
import { LanguageText, RouteState } from '../components/AnalystComponents.jsx'
import { ProportionBar } from '../components/DataVisualizations.jsx'
import { reviewTarget } from './dashboard/AttentionCauses.jsx'
import { ActivityCalendar } from './case/ActivityCalendar.jsx'
import { ReadinessRing } from './case/ReadinessRing.jsx'
import { getQueryCapabilities } from '../lib/apiClient.js'
import { attentionCauses } from '../lib/attentionCauses.js'
import { familyOrder } from '../lib/caseActivity.js'
import { caseVerdict, nextSteps } from '../lib/caseDossier.js'
import { aggregateFamilyRows, curatedQuestions, dashboardRowState, latestEvidenceActivity, relativeAge } from '../lib/dashboardCases.js'
import { attentionItems, familyRows, formatRate, reviewByCase } from '../lib/dashboardCharts.js'
import { formatNumber } from '../lib/format.js'
import { summariseCase, useCaseOverview } from '../lib/useCaseOverview.js'
import { useCaseActivity } from '../lib/useCaseActivity.js'
import { useQuestionHistory } from '../lib/workspaceState.js'

const FAMILIES_SHOWN = 8
const CAUSES_SHOWN = 3
const STEP_ICON = { failed: CircleAlert, processing: Clock3, ready: CircleCheckBig, quiet: Plus }

// Data-quality notes, each a plain sentence from the collection-wide summary counts (never the bounded recent lists).
export function qualityNotes(summary, counts) {
  const missing = Number(summary.completed_jobs_missing_kb_asset || 0)
  return [
    counts.rejected > 0 ? `${formatNumber(counts.rejected)} structured rows were rejected across the collection.` : '',
    counts.duplicates > 0 ? `${formatNumber(counts.duplicates)} duplicate structured rows were excluded across the collection.` : '',
    counts.failed > 0 ? `${formatNumber(counts.failed)} evidence item${counts.failed === 1 ? '' : 's'} failed processing and ${counts.failed === 1 ? 'requires' : 'require'} review in the Evidence catalog.` : '',
    counts.processing > 0 ? `${formatNumber(counts.processing)} evidence item${counts.processing === 1 ? '' : 's'} ${counts.processing === 1 ? 'is' : 'are'} still processing and excluded from complete analysis.` : '',
    missing > 0 ? `${formatNumber(missing)} completed ingest job${missing === 1 ? '' : 's'} ${missing === 1 ? 'is' : 'are'} missing a retained evidence copy.` : '',
  ].filter(Boolean)
}

function Panel({ id, title, hint = null, className = '', children }) {
  const headingId = `${id}-title`
  return (
    <section id={id} className={`co-panel${className ? ` ${className}` : ''}`} aria-labelledby={headingId}>
      <header><h2 id={headingId}>{title}</h2>{hint ? <p>{hint}</p> : null}</header>
      {children}
    </section>
  )
}

// The case briefing. Not the portfolio dashboard in miniature: it opens with a verdict on this one case and a readiness
// ring, then answers what to do next, what the case holds, when things happened and whether ingestion kept the rows.
// Everything is the collection's own summary counts or its real per-day activity; nothing is estimated.
export default function CaseOverviewPage() {
  const { id: caseId = '' } = useParams()
  const navigate = useNavigate()
  const state = useCaseOverview(caseId)
  const [suggestions, setSuggestions] = useState([])
  const [activityToken, setActivityToken] = useState(0)
  const [draft, setDraft] = useState('')
  const activity = useCaseActivity([caseId], { refreshToken: activityToken })
  const recent = useQuestionHistory(caseId).slice(0, 3)

  const row = useMemo(() => ({ caseId, index: 0, state, status: dashboardRowState(state), summary: state.data ? summariseCase(state.data) : null, activity: state.data ? latestEvidenceActivity(state.data) : null }), [caseId, state])
  const summary = row.summary
  const families = useMemo(() => familyRows(aggregateFamilyRows([row])), [row])
  const order = useMemo(() => familyOrder(families.map(family => family.id), activity.activity.families.map(family => family.id)), [families, activity.activity.families])
  const causes = useMemo(() => attentionCauses(attentionItems([row]), reviewByCase([row])), [row])
  const verdict = caseVerdict(summary)
  const steps = nextSteps(summary, caseId)
  const raw = state.data?.summary || {}
  const counts = { accepted: Number(raw.accepted_rows || 0), rejected: Number(raw.rejected_rows || 0), duplicates: Number(raw.duplicate_rows || 0), failed: Number(raw.evidence_failed || 0), processing: Number(raw.evidence_in_flight || 0) }
  const read = counts.accepted + counts.duplicates + counts.rejected
  const notes = state.data ? qualityNotes(raw, counts) : []
  const encoded = encodeURIComponent(caseId)

  useEffect(() => {
    if (!summary?.ready) return undefined
    const controller = new AbortController()
    getQueryCapabilities({ caseId, signal: controller.signal }).then(data => setSuggestions(curatedQuestions(data).slice(0, 3))).catch(() => {})
    return () => controller.abort()
  }, [caseId, summary?.ready])

  function ask(event) {
    event.preventDefault()
    if (draft.trim()) navigate(`/cases/${encoded}/investigate?question=${encodeURIComponent(draft.trim())}`)
  }

  return (
    <CaseShell caseId={caseId}>
      <main id="workspace-main" className="catalog-page case-overview" tabIndex={-1}>
        {state.loading && <RouteState state="loading" label="Loading case status" />}
        {state.error && <RouteState state={state.error.status === 403 ? 'forbidden' : 'error'} label={state.error.status === 403 ? 'Case overview is forbidden' : 'Case overview could not be loaded'} reference={state.error.reference} />}
        {state.data && summary && (
          <>
            <header className={`co-hero co-hero--${verdict.tone}`}>
              <div className="co-hero__text">
                <h1 className="co-hero__label">Case overview</h1>
                <p className="co-hero__id"><LanguageText as="bdi" identifier>{caseId}</LanguageText></p>
                <p className="co-hero__verdict">{verdict.headline}</p>
                {verdict.detail ? <p className="co-hero__detail">{verdict.detail}</p> : null}
                <div className="co-hero__actions">
                  <Link className="co-btn co-btn--primary" to={`/cases/${encoded}/investigate`}>Ask about this case<ArrowRight aria-hidden="true" /></Link>
                  <Link className="co-btn" to={`/cases/${encoded}/evidence#add-evidence`}>Add evidence</Link>
                </div>
                <p className="co-hero__meta">
                  <span>{formatNumber(summary.total)} {summary.total === 1 ? 'source' : 'sources'}</span>
                  <span>{formatNumber(summary.acceptedRows)} structured rows</span>
                  {row.activity ? <span>Updated <time dateTime={row.activity.date.toISOString()}>{relativeAge(row.activity.date)}</time></span> : null}
                </p>
              </div>
              <ReadinessRing summary={summary} />
            </header>

            <div className="co-grid">
              <div className="co-main">
                <Panel id="case-families" title="What is in this case" hint={families.length ? `${formatNumber(summary.acceptedRows)} accepted rows across ${formatNumber(families.length)} record ${families.length === 1 ? 'family' : 'families'}` : null}>
                  {families.length ? (
                    <>
                      <ul className="co-families">
                        {families.slice(0, FAMILIES_SHOWN).map(family => (
                          <li key={family.id}>
                            <Link to={`/cases/${encoded}/evidence?family=${encodeURIComponent(family.id)}`}>
                              <span className="co-families__dot" style={{ background: `var(--analyst-data-${(Math.max(0, order.indexOf(family.id)) % 6) + 1})` }} aria-hidden="true" />
                              <span className="co-families__name">{family.label}</span>
                              <span className="co-families__bar" aria-hidden="true"><i style={{ inlineSize: `${Math.max(2, Math.floor(family.share * 100))}%`, background: `var(--analyst-data-${(Math.max(0, order.indexOf(family.id)) % 6) + 1})` }} /></span>
                              <span className="co-families__value">{formatNumber(family.value)}</span>
                              <span className="co-families__share">{family.share < 0.01 ? '<1%' : `${Math.floor(family.share * 100)}%`}</span>
                            </Link>
                          </li>
                        ))}
                      </ul>
                      {families.length > FAMILIES_SHOWN ? <p className="co-note">{formatNumber(families.length - FAMILIES_SHOWN)} smaller families are in the evidence catalog.</p> : null}
                    </>
                  ) : <p className="dash-card__empty">No structured records have been accepted yet.</p>}
                </Panel>

                <Panel id="case-activity" title="When did activity happen?">
                  <ActivityCalendar caseId={caseId} activity={activity.activity} status={activity.status} failures={activity.failures} onRetry={() => setActivityToken(token => token + 1)} />
                </Panel>

                <Panel id="case-quality" title="Data-quality notes" hint={read ? 'Did ingestion keep every row? Shares are of all rows read.' : 'No structured rows have been read yet.'}>
                  {read ? (
                    <div className="cq-rows">
                      <ProportionBar total={read} ready={counts.accepted} processing={counts.duplicates} failed={counts.rejected} label={`${caseId} structured rows`} />
                      <dl className="cq-figures">
                        <div><dt>Accepted</dt><dd>{formatNumber(counts.accepted)}</dd></div>
                        <div><dt>Duplicate</dt><dd>{formatNumber(counts.duplicates)}<small>{formatRate(counts.duplicates / read)}</small></dd></div>
                        <div className={counts.rejected ? 'is-flagged' : undefined}><dt>Rejected</dt><dd>{formatNumber(counts.rejected)}<small>{formatRate(counts.rejected / read)}</small></dd></div>
                      </dl>
                    </div>
                  ) : null}
                  {notes.length ? <ul className="cq-notes">{notes.map(note => <li key={note}>{note}</li>)}</ul> : <p className="dash-card__empty">No data-quality issues are reported for this case.</p>}
                  <p className="cq-foot">{formatNumber(counts.accepted)} accepted rows across the reported record families. <Link to={`/cases/${encoded}/evidence`}>Open the evidence catalog</Link></p>
                </Panel>
              </div>

              <aside className="co-rail" aria-label="Actions for this case">
                <Panel id="case-next" title="Next steps" className="co-panel--accent">
                  {steps.length ? (
                    <ol className="co-steps">
                      {steps.map((step, index) => {
                        const Icon = STEP_ICON[step.tone] || Plus
                        return (
                          <li key={step.id} className={`co-step co-step--${step.tone}${index === 0 ? ' is-first' : ''}`}>
                            <Link to={step.to}>
                              <span className="co-step__mark" aria-hidden="true"><Icon /></span>
                              <span className="co-step__text"><strong>{step.title}</strong><small>{step.detail}</small></span>
                              <span className="co-step__cta">{step.cta}<ArrowRight aria-hidden="true" /></span>
                            </Link>
                          </li>
                        )
                      })}
                    </ol>
                  ) : <p className="dash-card__empty">Nothing to do yet.</p>}
                </Panel>

                <Panel id="case-attention" title="Needs review">
                  {causes.length ? (
                    <ul className="co-causes">
                      {causes.slice(0, CAUSES_SHOWN).map(cause => (
                        <li key={cause.key} className={`co-cause co-cause--${cause.kind}`}>
                          <Link to={cause.items[0] ? reviewTarget(cause.items[0]) : `/cases/${encoded}/evidence${cause.kind === 'failed' ? '?status=failed' : ''}`}>
                            <span className="co-cause__label">{cause.label}</span>
                            <b>{formatNumber(cause.count)}</b>
                            <ArrowRight aria-hidden="true" />
                          </Link>
                        </li>
                      ))}
                    </ul>
                  ) : <p className="co-clear"><CircleCheckBig aria-hidden="true" />Nothing needs review.</p>}
                  {causes.length > CAUSES_SHOWN ? <p className="co-note">{formatNumber(causes.length - CAUSES_SHOWN)} more {causes.length - CAUSES_SHOWN === 1 ? 'cause' : 'causes'} in the evidence catalog.</p> : null}
                </Panel>

                <Panel id="case-ask" title="Ask about this case">
                  <form className="co-ask" onSubmit={ask}>
                    <label className="visually-hidden" htmlFor="co-ask-input">Your question</label>
                    <input id="co-ask-input" type="text" value={draft} onChange={event => setDraft(event.target.value)} placeholder="Ask about the evidence…" disabled={!summary.ready} />
                    <button type="submit" disabled={!draft.trim() || !summary.ready} aria-label="Ask"><Send aria-hidden="true" /></button>
                  </form>
                  {!summary.ready ? <p className="co-note">Nothing is ready to search yet.</p> : null}
                  {suggestions.length || recent.length ? (
                    <ul className="co-chips" aria-label="Question suggestions">
                      {suggestions.map(item => <li key={item.query}><button type="button" onClick={() => setDraft(item.query)}><LanguageText>{item.query}</LanguageText></button></li>)}
                      {recent.map(entry => <li key={entry.id}><button type="button" className="is-recent" onClick={() => setDraft(entry.query)} title="Asked before"><LanguageText>{entry.label || entry.query}</LanguageText></button></li>)}
                    </ul>
                  ) : null}
                </Panel>
              </aside>
            </div>
          </>
        )}
      </main>
    </CaseShell>
  )
}
