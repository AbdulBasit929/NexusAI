import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { AddDataDropzone } from '../components/AddDataDropzone.jsx'
import { ProcessingWait } from '../components/ProcessingWait.jsx'
import { collectionIdForCase } from '../lib/caseScope.js'
import { LanguageText, RouteState } from '../components/AnalystComponents.jsx'
import { AppShell } from '../components/CaseShell.jsx'
import { PageHeader } from '../components/PageHeader.jsx'

// There is no create-case endpoint. The identifier scopes the first upload and
// the case begins only when that file is accepted, so it is never silently
// normalized into something the analyst did not choose.
const validName = /^[a-z0-9][a-z0-9-]*[a-z0-9]$/

function nameProblem(value) {
  const trimmed = value.trim()
  if (!trimmed) return 'Enter a case identifier.'
  if (trimmed !== value) return 'Remove the space at the start or end.'
  if (trimmed.length < 4) return 'Use at least 4 characters.'
  if (trimmed.length > 64) return 'Use no more than 64 characters.'
  if (!/^[a-z0-9-]+$/.test(trimmed)) return 'Use lowercase letters, numbers and hyphens only.'
  if (!validName.test(trimmed)) return 'Start and end with a letter or number.'
  return ''
}

function LifecycleIcon({ kind }) {
  const paths = {
    name: <><path d="M5 7h14M5 12h10M5 17h7" /><path d="M19 14v6m-3-3h6" /></>,
    upload: <><path d="M12 16V5m0 0-4 4m4-4 4 4" /><path d="M5 15v5h14v-5" /></>,
    ready: <><circle cx="12" cy="12" r="8" /><path d="m8 12 2.5 2.5L16 9" /></>,
  }
  return <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">{paths[kind]}</svg>
}

export default function NewCasePage() {
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [touched, setTouched] = useState(false)
  const [accepted, setAccepted] = useState(0)

  const problem = touched ? nameProblem(name) : ''
  const caseId = collectionIdForCase(name)
  const named = !nameProblem(name)
  const locked = accepted > 0

  return (
    <AppShell>
      <main id="workspace-main" className="new-case-page" tabIndex={-1}>
        <PageHeader
          eyebrow="Evidence intake"
          title="Start a case with evidence"
          description="Choose a stable case identifier, then add the first evidence. No empty case is created before a file is accepted."
          breadcrumbs={[{ label: 'Workspace', to: '/' }, { label: 'Cases', to: '/cases' }, { label: 'New case' }]}
          actions={<Link to="/cases">Back to cases</Link>}
        />

        <ol className="new-case-lifecycle" aria-label="Case start lifecycle">
          <li className={named ? 'is-complete' : 'is-current'}><LifecycleIcon kind="name" /><span><small>Step 1</small><strong>Choose identifier</strong><em>Nothing is created yet</em></span></li>
          <li className={locked ? 'is-complete' : named ? 'is-current' : ''}><LifecycleIcon kind="upload" /><span><small>Step 2</small><strong>Add first evidence</strong><em>Acceptance starts the case</em></span></li>
          <li className={locked ? 'is-current' : ''}><LifecycleIcon kind="ready" /><span><small>Step 3</small><strong>Review processing</strong><em>Ask only when evidence is ready</em></span></li>
        </ol>

        <section className="new-case__naming" aria-labelledby="case-name-heading">
          <header>
            <p className="section-kicker">Step 1</p>
            <h2 id="case-name-heading">Choose the case identifier</h2>
            <p>This identifier appears in case URLs and cannot be renamed after the first evidence is accepted.</p>
          </header>
          <label htmlFor="case-name">Case identifier</label>
          <input
            id="case-name"
            name="case-name"
            type="text"
            value={name}
            spellCheck={false}
            autoComplete="off"
            readOnly={locked}
            aria-describedby={`case-name-hint${problem ? ' case-name-problem' : ''}${named ? ' case-name-confirmation' : ''}`}
            aria-invalid={problem ? 'true' : undefined}
            onChange={event => setName(event.target.value)}
            onBlur={() => setTouched(true)}
          />
          <p id="case-name-hint" className="field-hint">4–64 lowercase letters, numbers or hyphens. Example: operation-falcon-2026</p>
          {problem && <p id="case-name-problem" className="field-error" role="alert"><strong>Case identifier:</strong> {problem}</p>}
          {named && (
            <p id="case-name-confirmation" className="case-identifier-confirmation">
              {locked ? 'Case identifier locked after acceptance:' : 'Case will use this exact identifier:'}{' '}
              <LanguageText as="strong" identifier>{caseId}</LanguageText>
            </p>
          )}
        </section>

        {!named && (
          <section className="new-case__pending-step" aria-labelledby="pending-intake-title">
            <p className="section-kicker">Step 2</p>
            <h2 id="pending-intake-title">Add the first evidence</h2>
            <p>Enter a valid case identifier above to open evidence intake. Naming alone will not create a case.</p>
          </section>
        )}

        {named && (
          <>
            <AddDataDropzone caseId={caseId} onAccepted={() => setAccepted(count => count + 1)} />
            {locked && (
              <section className="new-case__accepted" aria-label="Case acceptance">
                <RouteState state="ready" label="The case now exists" description={`${accepted.toLocaleString()} ${accepted === 1 ? 'file has' : 'files have'} been accepted for this case. Processing state—not transfer progress—determines when each file can support an answer.`}>
                  <button type="button" onClick={() => navigate(`/cases/${encodeURIComponent(caseId)}/overview`)}>Open case</button>
                </RouteState>
                <ProcessingWait caseId={caseId} />
              </section>
            )}
          </>
        )}
      </main>
    </AppShell>
  )
}
