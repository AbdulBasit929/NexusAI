import { useRef, useState } from 'react'
import { ArrowLeft, ArrowRight, Check, CircleAlert, CircleCheck, Link2, Lock } from 'lucide-react'
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

const STEPS = ['Identifier', 'First evidence', 'Processing']
const IN_USE_SHOWN = 4

// A three-step start that fits one screen: the identifier, the first evidence, then processing. The left panel is a live
// preview of the case being created, which stays "not created yet" until the service accepts a file; the right panel shows one
// step at a time and never advances past a step whose requirement is unmet. Nothing here creates a case: acceptance does.
export default function NewCasePage() {
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [touched, setTouched] = useState(false)
  const [accepted, setAccepted] = useState(0)
  const [stage, setStage] = useState(0)
  const field = useRef(null)

  const existing = configuredCaseIds()
  const problem = touched ? nameProblem(name) : ''
  const caseId = collectionIdForCase(name)
  const taken = existingCase(name, existing)
  const named = !nameProblem(name) && !taken
  const locked = accepted > 0
  const suggestion = !locked && name && nameProblem(name) ? suggestCaseName(name) : null
  const rules = caseNameRules(name)
  const current = locked ? 2 : named && stage === 1 ? 1 : 0
  const verdict = taken ? 'taken' : named ? 'ok' : name && touched ? 'bad' : null

  function next(event) {
    event?.preventDefault()
    if (named) setStage(1)
    else setTouched(true)
  }

  return (
    <AppShell>
      <main id="workspace-main" className="new-case-page nc-page" tabIndex={-1}>
        <div className="nc-shell">
          <aside className="nc-side" aria-label="Case preview">
            <nav className="nc-side__crumbs" aria-label="Page trail">
              <ol>
                <li><Link to="/">Workspace</Link></li>
                <li><Link to="/cases">Cases</Link></li>
                <li><span aria-current="page">New case</span></li>
              </ol>
            </nav>
            <div className="nc-side__intro">
              <h1>Start a case with evidence</h1>
              <p>Choose a stable case identifier, then add the first evidence. No empty case is created before a file is accepted.</p>
            </div>
            <div className={`nc-preview${locked ? ' is-live' : ''}`} aria-live="polite">
              <span className="nc-preview__chip">{locked ? 'Accepted · processing' : 'Not created yet'}</span>
              <p className={`nc-preview__id${named || locked ? ' is-set' : ''}`}>{named || locked ? caseId : name.trim() || 'your-case-id'}</p>
              <p className="nc-preview__url"><Link2 aria-hidden="true" />…/cases/{named || locked ? caseId : 'your-case-id'}/overview</p>
              <ul className="nc-preview__checks" aria-label="Progress of this case">
                <li className={named || locked ? 'is-done' : undefined}><Check aria-hidden="true" />Identifier chosen</li>
                <li className={locked ? 'is-done' : undefined}><Check aria-hidden="true" />First evidence accepted</li>
                <li className={locked ? 'is-done' : undefined}><Check aria-hidden="true" />Processing started</li>
              </ul>
            </div>
            <Link className="nc-side__back" to="/cases"><ArrowLeft aria-hidden="true" />Back to cases</Link>
          </aside>

          <section className="nc-panel" aria-label="Case setup">
            <ol className="nc-track" aria-label="Case start lifecycle">
              {STEPS.map((label, index) => (
                <li key={label} className={index < current ? 'is-complete' : index === current ? 'is-current' : undefined} aria-current={index === current ? 'step' : undefined}>
                  <span className="nc-track__bar" aria-hidden="true" />
                  <small>Step {index + 1}</small>
                  <strong>{label}</strong>
                </li>
              ))}
            </ol>

            <div className="nc-stage">
              {!locked && stage === 0 && (
                <form className="nc-step new-case__naming" aria-labelledby="case-name-heading" onSubmit={next}>
                  <header>
                    <h2 id="case-name-heading">Choose the case identifier</h2>
                    <p>This identifier appears in case URLs and cannot be renamed after the first evidence is accepted.</p>
                  </header>
                  <label htmlFor="case-name">Case identifier</label>
                  <div className={`nc-field${verdict ? ` nc-field--${verdict}` : ''}`}>
                    <input
                      ref={field}
                      id="case-name"
                      name="case-name"
                      type="text"
                      value={name}
                      spellCheck={false}
                      autoComplete="off"
                      autoFocus
                      placeholder="operation-falcon-2026"
                      aria-describedby={`case-name-hint${problem ? ' case-name-problem' : ''}${named ? ' case-name-confirmation' : ''}`}
                      aria-invalid={problem ? 'true' : undefined}
                      onChange={event => setName(event.target.value)}
                      onBlur={() => setTouched(true)}
                    />
                    <span className="nc-field__status" aria-hidden="true">
                      {verdict === 'ok' ? <CircleCheck /> : verdict ? <CircleAlert /> : <span className={`nc-count${name.length > MAX_LENGTH ? ' is-over' : ''}`}>{name.length}/{MAX_LENGTH}</span>}
                    </span>
                  </div>
                  <p id="case-name-hint" className="field-hint">4–64 lowercase letters, numbers or hyphens. Example: operation-falcon-2026</p>
                  <ul className="nc-rules" aria-label="Identifier rules">
                    {rules.map(rule => <li key={rule.id} className={rule.met ? 'is-met' : undefined}>{rule.met ? <Check aria-hidden="true" /> : <span className="nc-rules__dot" aria-hidden="true" />}<span>{rule.label}</span><span className="visually-hidden">{rule.met ? ', met' : ', not met'}</span></li>)}
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
                    <p id="case-name-confirmation" className="case-identifier-confirmation">Case will use this exact identifier: <LanguageText as="strong" identifier>{caseId}</LanguageText></p>
                  )}
                  {existing.length ? (
                    <p className="nc-inuse">
                      <span>Already in use</span>
                      {existing.slice(0, IN_USE_SHOWN).map(id => <code key={id}>{id}</code>)}
                      {existing.length > IN_USE_SHOWN ? <span>+{existing.length - IN_USE_SHOWN} more</span> : null}
                    </p>
                  ) : null}
                  <footer className="nc-actions">
                    <p className="nc-actions__hint new-case__pending-step">{named ? 'Next: add the first evidence. Nothing is created until a file is accepted.' : 'Enter a valid case identifier above to open evidence intake. Naming alone will not create a case.'}</p>
                    <button type="submit" className="nc-btn nc-btn--primary" disabled={!named}>Continue<ArrowRight aria-hidden="true" /></button>
                  </footer>
                </form>
              )}

              {(locked || (stage === 1 && named)) && (
                <div className="nc-step new-case__accepted">
                  {locked ? (
                    <div className="nc-locked-field">
                      <label htmlFor="case-name">Case identifier</label>
                      <div className="nc-field nc-field--ok"><input id="case-name" name="case-name" type="text" value={caseId} readOnly /><span className="nc-field__status" aria-hidden="true"><Lock /></span></div>
                      <p className="case-identifier-confirmation">Case identifier locked after acceptance: <LanguageText as="strong" identifier>{caseId}</LanguageText></p>
                    </div>
                  ) : null}
                  {locked ? (
                    <section aria-label="Case acceptance" className="nc-done">
                      <RouteState state="ready" label="The case now exists" description={`${accepted.toLocaleString()} ${accepted === 1 ? 'file has' : 'files have'} been accepted for this case. Processing state—not transfer progress—determines when each file can support an answer.`}>
                        <button type="button" onClick={() => navigate(`/cases/${encodeURIComponent(caseId)}/overview`)}>Open case</button>
                      </RouteState>
                      <ProcessingWait caseId={caseId} />
                    </section>
                  ) : null}
                  <AddDataDropzone caseId={caseId} onAccepted={() => setAccepted(count => count + 1)} />
                  {!locked ? (
                    <footer className="nc-actions">
                      <button type="button" className="nc-btn" onClick={() => setStage(0)}><ArrowLeft aria-hidden="true" />Change identifier</button>
                    </footer>
                  ) : null}
                </div>
              )}
            </div>
          </section>
        </div>
      </main>
    </AppShell>
  )
}
