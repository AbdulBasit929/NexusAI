import { Link } from 'react-router-dom'
import './CaseWorkspaceChrome.css'
import { CASE_WORKSPACE_MODULES } from './caseWorkspaceModules'

function humanize(value) {
  if (value == null || value === '') return 'Not reported'
  return String(value).replaceAll('_', ' ').replace(/\b\w/g, letter => letter.toUpperCase())
}

export default function CaseWorkspaceChrome({
  caseId,
  collectionId,
  caseInfo,
  caseGroups,
  caseOptions,
  currentModule,
  posture,
  processingCount,
  onSwitchCase,
  children,
}) {
  return (
    <div className="case-command-center">
      <header className="case-command-header">
        <div className="case-command-identity">
          <span className="case-command-mark" aria-hidden="true"><i className="fas fa-fingerprint" /></span>
          <div>
            <span className="case-command-kicker">Active investigation</span>
            <h1>{caseInfo?.display_name || caseId}</h1>
            <div className="case-command-status-line">
              <span className={`case-command-health case-command-health--${posture.tone}`}>{posture.label}</span>
              <span>{humanize(caseInfo?.security_classification)}</span>
              <span>{humanize(caseInfo?.case_status)}</span>
              <span>{processingCount} processing</span>
            </div>
          </div>
        </div>

        <div className="case-command-selector">
          <label htmlFor="active-case-selector">
            <span>Active case</span>
            <small>{caseOptions.length} accessible</small>
          </label>
          <select id="active-case-selector" className="input" aria-label="Active collection" value={caseId} onChange={event => onSwitchCase(event.target.value)}>
            {Object.entries(caseGroups).map(([group, items]) => (
              <optgroup key={group} label={group}>
                {items.map(item => <option key={item.case_id} value={item.case_id}>{item.display_name} · {humanize(item.case_status)}</option>)}
              </optgroup>
            ))}
          </select>
        </div>

        <div className="case-command-scope" aria-label="Authorized case scope">
          <div><span>Case ID</span><code>{caseId}</code></div>
          <div><span>Collection</span><code>{collectionId}</code></div>
          <span className="case-command-scope-lock"><i className="fas fa-lock" /> Authorized scope</span>
        </div>
      </header>

      <div className="case-command-layout">
        <aside className="case-module-rail">
          <div className="case-module-rail-heading"><span>Investigation desk</span><small>Case-scoped modules</small></div>
          <nav aria-label="Case workspace sections">
            {CASE_WORKSPACE_MODULES.map(module => (
              <Link key={module.id} to={`/app/cases/${encodeURIComponent(caseId)}/${module.id}`} aria-label={module.label} aria-current={currentModule.id === module.id ? 'page' : undefined}>
                <span className="case-module-icon"><i className={`fas ${module.icon}`} /></span>
                <span className="case-module-copy"><strong>{module.label}</strong><small>{module.description}</small></span>
                <i className="fas fa-chevron-right case-module-arrow" aria-hidden="true" />
              </Link>
            ))}
          </nav>
          <div className="case-module-assurance"><i className="fas fa-shield-halved" /><span><strong>Case isolated</strong><small>Every module inherits the authorized URL scope.</small></span></div>
        </aside>

        <section className="case-module-stage" aria-labelledby="case-module-title">
          <header className="case-module-header">
            <div><span className="case-command-kicker">{currentModule.label} module</span><h2 id="case-module-title">{currentModule.label}</h2><p>{currentModule.description}</p></div>
            <span className="case-module-boundary"><i className="fas fa-circle-check" /> Bound to active case</span>
          </header>
          <div className="case-module-body">{children}</div>
        </section>
      </div>
    </div>
  )
}
