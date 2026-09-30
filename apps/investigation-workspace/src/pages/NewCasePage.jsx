import { useState } from 'react'
import { Check, CircleAlert, CircleCheck, FilePlus2, FolderCheck, Link2, Lock, TextCursorInput } from 'lucide-react'
import { Link, useNavigate } from 'react-router-dom'
import { AddDataDropzone } from '../components/AddDataDropzone.jsx'
import { ProcessingWait } from '../components/ProcessingWait.jsx'
import { configuredCaseIds } from '../lib/apiClient.js'
import { caseNameRules, existingCase, MAX_LENGTH, suggestCaseName } from '../lib/caseName.js'
import { collectionIdForCase } from '../lib/caseScope.js'
import { LanguageText, RouteState } from '../components/AnalystComponents.jsx'
import { AppShell } from '../components/CaseShell.jsx'

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

const STEPS = [
  { id: 'name', Icon: TextCursorInput, title: 'Choose an identifier', note: 'Nothing is created yet' },
  { id: 'upload', Icon: FilePlus2, title: 'Add the first evidence', note: 'Acceptance starts the case' },
  { id: 'ready', Icon: FolderCheck, title: 'Review processing', note: 'Ask once evidence is ready' },
]
const IN_USE_SHOWN = 4

export default function NewCasePage() {
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [touched, setTouched] = useState(false)
  const [accepted, setAccepted] = useState(0)

  const existing = configuredCaseIds()
  const problem = touched ? nameProblem(name) : ''
  const caseId = collectionIdForCase(name)
  const taken = existingCase(name, existing)
  const named = !nameProblem(name) && !taken
  const locked = accepted > 0
  const suggestion = !locked && name && nameProblem(name) ? suggestCaseName(name) : null
  const current = locked ? 2 : named ? 1 : 0
  const verdict = taken ? 'taken' : named ? 'ok' : name && touched ? 'bad' : null

  return (
    <AppShell>
      <main id="workspace-main" className="new-case-page nc-page" tabIndex={-1}>
        <header className="nc-hero">
          <nav className="nc-hero__crumbs" aria-label="Page trail">
            <ol>
              <li><Link to="/">Workspace</Link></li>
              <li><Link to="/cases">Cases</Link></li>
              <li><span aria-current="page">New case</span></li>
            </ol>
          </nav>
          <div className="nc-hero__row">
            <div>
              <h1>Start a case with evidence</h1>
              <p>Choose a stable case identifier, then add the first evidence. No empty case is created before a file is accepted.</p>
            </div>
            <Link className="nc-hero__back" to="/cases">Back to cases</Link>
          </div>
          <ol className="nc-progress" aria-label="Case start lifecycle">
            {STEPS.map(({ id, Icon, title, note }, index) => (
              <li key={id} className={index < current ? 'is-complete' : index === current ? 'is-current' : undefined} aria-current={index === current ? 'step' : undefined}>
                <span className="nc-progress__mark" aria-hidden="true">{index < current ? <Check /> : <Icon />}</span>
                <span className="nc-progress__text"><small>Step {index + 1}</small><strong>{title}</strong><em>{note}</em></span>
              </li>
            ))}
          </ol>
        </header>

        <div className="nc-body">
          <section className="nc-spot new-case__naming" aria-labelledby="case-name-heading">
            <header>
              <p className="section-kicker">Step 1</p>
              <h2 id="case-name-heading">Choose the case identifier</h2>
              <p>This identifier appears in case URLs and cannot be renamed after the first evidence is accepted.</p>
            </header>
            <label htmlFor="case-name">Case identifier</label>
            <div className={`nc-field${verdict ? ` nc-field--${verdict}` : ''}`}>
              <input
                id="case-name"
                name="case-name"
                type="text"
                value={name}
                spellCheck={false}
                autoComplete="off"
                readOnly={locked}
                placeholder="operation-falcon-2026"
                aria-describedby={`case-name-hint${problem ? ' case-name-problem' : ''}${named ? ' case-name-confirmation' : ''}`}
                aria-invalid={problem ? 'true' : undefined}
                onChange={event => setName(event.target.value)}
                onBlur={() => setTouched(true)}
              />
              <span className="nc-field__status" aria-hidden="true">
                {locked ? <Lock /> : verdict === 'ok' ? <CircleCheck /> : verdict ? <CircleAlert /> : <span className={`nc-count${name.length > MAX_LENGTH ? ' is-over' : ''}`}>{name.length}/{MAX_LENGTH}</span>}
              </span>
            </div>
            <p className="nc-url" aria-hidden="true"><Link2 /><span>…/cases/</span><b className={named ? 'is-set' : undefined}>{named ? caseId : 'your-case-id'}</b><span>/overview</span></p>
            <p id="case-name-hint" className="field-hint">4–64 lowercase letters, numbers or hyphens. Example: operation-falcon-2026</p>
            <ul className="nc-rules" aria-label="Identifier rules">
              {caseNameRules(name).map(rule => <li key={rule.id} className={rule.met ? 'is-met' : undefined}>{rule.met ? <Check aria-hidden="true" /> : <span className="nc-rules__dot" aria-hidden="true" />}<span>{rule.label}</span><span className="visually-hidden">{rule.met ? ', met' : ', not met'}</span></li>)}
            </ul>
            {suggestion ? (
              <p className="nc-suggest">
                <span>Use a valid version?</span>
                <button type="button" onClick={() => { setName(suggestion); setTouched(true) }}><LanguageText as="strong" identifier>{suggestion}</LanguageText></button>
              </p>
            ) : null}
            {problem && <p id="case-name-problem" className="field-error" role="alert"><strong>Case identifier:</strong> {problem}</p>}
            {taken ? (
              <p className="nc-taken" role="status">
                <strong><LanguageText as="bdi" identifier>{taken}</LanguageText></strong> is already a case in this workspace.{' '}
                <Link to={`/cases/${encodeURIComponent(taken)}/evidence#add-evidence`}>Add evidence to it instead</Link> or choose another identifier.
              </p>
            ) : null}
            {named && (
              <p id="case-name-confirmation" className="case-identifier-confirmation">
                {locked ? 'Case identifier locked after acceptance:' : 'Case will use this exact identifier:'}{' '}
                <LanguageText as="strong" identifier>{caseId}</LanguageText>
              </p>
            )}
            {existing.length && !locked ? (
              <p className="nc-inuse">
                <span>Already in use</span>
                {existing.slice(0, IN_USE_SHOWN).map(id => <code key={id}>{id}</code>)}
                {existing.length > IN_USE_SHOWN ? <span>+{existing.length - IN_USE_SHOWN} more</span> : null}
              </p>
            ) : null}
          </section>

          {!named && (
            <section className="nc-locked new-case__pending-step" aria-labelledby="pending-intake-title">
              <span className="nc-locked__icon" aria-hidden="true"><Lock /></span>
              <div>
                <p className="section-kicker">Step 2</p>
                <h2 id="pending-intake-title">Add the first evidence</h2>
                <p>Enter a valid case identifier above to open evidence intake. Naming alone will not create a case.</p>
              </div>
              <span className="nc-locked__button" aria-hidden="true">Choose files</span>
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
          <p className="nc-foot">Evidence is kept as received. Processing runs after acceptance and you can leave this page; the case appears in Cases as soon as the first file is accepted.</p>
        </div>
      </main>
    </AppShell>
  )
}
