import { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { CaseShell } from '../components/CaseShell.jsx'
import { RouteState } from '../components/AnalystComponents.jsx'
import { PageHeader } from '../components/PageHeader.jsx'
import { getQueryCapabilities } from '../lib/apiClient.js'
import { summariseCase, useCaseOverview } from '../lib/useCaseOverview.js'

const familyLabels = {
  cdr: 'CDR', ipdr: 'IPDR', anpr: 'ANPR', subscriber: 'Subscriber', tower_location: 'Tower',
  transaction: 'Financial', access_log: 'Access log', document: 'Documents', image: 'Images', audio: 'Audio', video: 'Video',
}

const scopeByCapability = {
  cdr: 'cdr', ipdr: 'ipdr', anpr: 'anpr', subscriber_identity: 'subscriber', subscriber: 'subscriber',
  tower_location: 'tower', tower: 'tower', logs_access_security: 'access_log', access_log: 'access_log',
  financial: 'financial', transaction: 'financial', document: 'document', images: 'image', image: 'image',
  audio_and_stt: 'audio', audio: 'audio', video: 'video',
}

function safeFamilyLabel(value) {
  if (!value) return 'Not reported'
  return familyLabels[String(value).toLowerCase()] || String(value)
}

function scopeFor(family) {
  const recordType = Array.isArray(family?.record_types) ? family.record_types[0] : ''
  return scopeByCapability[family?.id] || scopeByCapability[recordType] || 'all'
}

export function timelineQuestions(payload) {
  const seen = new Set()
  return (Array.isArray(payload?.families) ? payload.families : [])
    .filter(family => ['queryable', 'semantic_only', 'limited'].includes(family?.availability))
    .flatMap(family => (Array.isArray(family?.suggested_queries) ? family.suggested_queries : []).map(question => ({
      question: String(question),
      family: family.label || safeFamilyLabel(family.id),
      scope: scopeFor(family),
    })))
    .filter(item => /timeline|history|sequence|by hour|over time|time order/i.test(item.question))
    .filter(item => {
      const key = `${item.scope}:${item.question.toLocaleLowerCase()}`
      if (seen.has(key)) return false
      seen.add(key)
      return true
    })
    .slice(0, 6)
}

function investigateLink(caseId, item) {
  const query = new URLSearchParams({ question: item.question })
  if (item.scope !== 'all') query.set('scope', item.scope)
  return `/cases/${encodeURIComponent(caseId)}/investigate?${query}`
}

function TimelineRequirementIcon({ kind }) {
  const paths = {
    time: <><circle cx="12" cy="12" r="8" /><path d="M12 7v5l3 2" /></>,
    event: <><path d="M5 6h14M5 12h14M5 18h9" /><circle cx="3" cy="6" r=".5" /><circle cx="3" cy="12" r=".5" /><circle cx="3" cy="18" r=".5" /></>,
    identity: <><circle cx="8" cy="8" r="3" /><circle cx="17" cy="10" r="2.5" /><path d="M3 19c.7-3.2 2.4-5 5-5s4.3 1.8 5 5m1-4c2.7 0 4.4 1.3 5 4" /></>,
    source: <><path d="M7 3h8l4 4v14H7z" /><path d="M15 3v5h4M10 13h6m-6 4h4" /></>,
  }
  return <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">{paths[kind]}</svg>
}

export default function TimelinePage() {
  const { id: caseId = '' } = useParams()
  const overview = useCaseOverview(caseId)
  const summary = useMemo(() => summariseCase(overview.data), [overview.data])
  const [capabilities, setCapabilities] = useState({ loading: true })

  useEffect(() => {
    const controller = new AbortController()
    getQueryCapabilities({ caseId, signal: controller.signal })
      .then(data => setCapabilities({ data, loading: false }))
      .catch(error => error.name !== 'AbortError' && setCapabilities({ error, loading: false }))
    return () => controller.abort()
  }, [caseId])

  const questions = useMemo(() => timelineQuestions(capabilities.data), [capabilities.data])
  const familyNames = summary.families.map(family => safeFamilyLabel(family.record_type))

  return (
    <CaseShell caseId={caseId}>
      <main id="workspace-main" className="timeline-page" tabIndex={-1}>
        <PageHeader
          eyebrow="Case workspace"
          title="Timeline"
          description="Reconstruct source events across evidence families and open the retained evidence behind each event."
        />

        <section className="timeline-unavailable" aria-label="Timeline availability">
          <RouteState state="unavailable" label="A reliable source chronology is not available" description="This collection service does not provide a case-wide feed of source events with trustworthy event times and openable evidence locations. Upload and processing dates were deliberately excluded because they describe system activity, not what happened in the evidence.">
            <div className="timeline-primary-actions">
              <Link className="button-link button-link--primary" to={`/cases/${encodeURIComponent(caseId)}/evidence`}>Review evidence</Link>
              <Link className="button-link" to={`/cases/${encodeURIComponent(caseId)}/investigate`}>Ask about a specific sequence</Link>
            </div>
          </RouteState>
        </section>

        <section className="timeline-available" aria-labelledby="timeline-available-title">
          <header>
            <p className="section-kicker">Current collection</p>
            <h2 id="timeline-available-title">What is available now</h2>
            <p>Inventory facts can guide the next step, but they are not timeline events.</p>
          </header>

          {overview.loading && <RouteState state="loading" label="Checking collection evidence" description="Evidence totals and represented families will appear only after collection status resolves." />}
          {overview.error && <RouteState state={overview.error.status === 403 ? 'forbidden' : 'error'} label={overview.error.status === 403 ? 'Collection status access is forbidden' : 'Collection status could not be loaded'} description="The timeline remains unavailable; no inventory count has been inferred." reference={overview.error.reference || 'TIMELINE-INVENTORY'} />}
          {!overview.loading && !overview.error && (
            <dl className="timeline-facts" role="status" aria-atomic="true">
              <div><dt>Timeline events</dt><dd><strong>Not available</strong><span>No cross-family event feed is exposed.</span></dd></div>
              <div><dt>Evidence inventory</dt><dd><strong>{summary.total.toLocaleString()}</strong><span>{summary.total === 1 ? 'evidence item reported' : 'evidence items reported'}</span></dd></div>
              <div><dt>Ready evidence</dt><dd><strong>{summary.ready.toLocaleString()}</strong><span>reported ready for review</span></dd></div>
              <div><dt>Structured families</dt><dd><strong>{familyNames.length.toLocaleString()}</strong><span>{familyNames.length ? familyNames.join(' · ') : 'None reported'}</span></dd></div>
            </dl>
          )}
        </section>

        {questions.length > 0 && (
          <section className="timeline-questions" aria-labelledby="timeline-questions-title">
            <header>
              <p className="section-kicker">Supported alternatives</p>
              <h2 id="timeline-questions-title">Ask a narrower chronology question</h2>
              <p>These questions come from the service’s current curation for evidence present in this collection. Investigate will still withhold an answer when the evidence cannot support it.</p>
            </header>
            <ul>
              {questions.map(item => (
                <li key={`${item.scope}-${item.question}`}>
                  <span>{item.family}</span>
                  <Link to={investigateLink(caseId, item)}>{item.question}</Link>
                </li>
              ))}
            </ul>
          </section>
        )}

        <section className="timeline-requirements" aria-labelledby="timeline-requirements-title">
          <header>
            <p className="section-kicker">Completeness standard</p>
            <h2 id="timeline-requirements-title">What a trustworthy timeline needs</h2>
            <p>The view will remain unavailable until every event can carry these four facts.</p>
          </header>
          <ol>
            <li><TimelineRequirementIcon kind="time" /><span><strong>Source event time</strong><small>The time stated by the source, with its precision and time basis retained.</small></span></li>
            <li><TimelineRequirementIcon kind="event" /><span><strong>Event meaning</strong><small>A curated family and event label, not an internal identifier turned into prose.</small></span></li>
            <li><TimelineRequirementIcon kind="identity" /><span><strong>Involved identifiers</strong><small>Only identifiers actually carried by the source event.</small></span></li>
            <li><TimelineRequirementIcon kind="source" /><span><strong>Openable evidence</strong><small>An exact retained file, row, page or media time for every event.</small></span></li>
          </ol>
        </section>
      </main>
    </CaseShell>
  )
}
