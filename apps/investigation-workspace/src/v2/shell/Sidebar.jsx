import { NavLink, useLocation } from 'react-router-dom'
import { PanelLeftClose, PanelLeftOpen, Plus, Settings2 } from 'lucide-react'
import { configuredCaseIds } from '../../lib/apiClient.js'
import { sidebarCases, withPending } from '../../lib/workspaceAttention.js'
import { clearCaseSessionScope } from '../../lib/workspaceState.js'
import { formatNumber } from '../../lib/format.js'
import { BrandMark } from './Brand.jsx'
import { CaseLabel, useCaseLabel } from './CaseLabel.jsx'
import { caseNav, workspaceNav } from './nav.js'

export const CASE_STATE = {
  attention: { label: 'Needs review', dot: 'danger' },
  processing: { label: 'Processing', dot: 'busy' },
  complete: { label: 'Ready', dot: 'ok' },
  'not-processed': { label: 'No evidence yet', dot: '' },
  unreported: { label: 'Status unavailable', dot: 'warn' },
  loading: { label: 'Checking status', dot: '' },
}

// What the case status service reported, in the words of an analyst. Never shown as healthy until it has said so.
export function readiness(overview) {
  if (!overview || overview.loading) return { state: 'loading', text: 'Checking status…' }
  if (overview.error) return { state: 'unreported', text: 'Status unavailable' }
  const summary = overview.data?.summary
  if (!summary || !Object.prototype.hasOwnProperty.call(summary, 'evidence_total')) return { state: 'unreported', text: 'Status unavailable' }
  const total = Number(summary.evidence_total || 0)
  const ready = Number(summary.evidence_completed || 0)
  const processing = Number(summary.evidence_in_flight || 0)
  const failed = Number(summary.evidence_failed || 0)
  if (total === 0) return { state: 'not-processed', text: 'No evidence yet' }
  if (failed > 0) return { state: 'attention', text: `${formatNumber(failed)} need${failed === 1 ? 's' : ''} review`, failed }
  if (processing > 0) return { state: 'processing', text: `${formatNumber(processing)} processing`, processing }
  return { state: 'complete', text: `${formatNumber(ready)} of ${formatNumber(total)} sources ready`, ready, total }
}

function Item({ item, collapsed, count, attention, onNavigate }) {
  const Icon = item.icon
  return (
    <NavLink to={item.to} end={item.end} className="nx-nav-item" title={collapsed ? item.label : undefined} aria-label={collapsed ? item.label : undefined} onClick={onNavigate}>
      <Icon aria-hidden="true" />
      <span className="nx-nav-item__text">{item.label}</span>
      {count ? <span className={`nx-nav-item__count${attention ? ' nx-nav-item__count--attention' : ''}`}>{formatNumber(count)}</span> : null}
    </NavLink>
  )
}

function CaseCard({ caseId, overview, onNavigate }) {
  const { date } = useCaseLabel(caseId)
  const status = readiness(overview)
  const meta = CASE_STATE[status.state]
  return (
    <NavLink to={`/cases/${encodeURIComponent(caseId)}/overview`} className="nx-case-card" onClick={onNavigate} aria-label={`Case ${caseId}: open overview`}>
      <span className="nx-case-card__label">Case{date ? ` · ${date}` : ''}</span>
      <span className="nx-case-card__name"><CaseLabel caseId={caseId} /></span>
      <span className="nx-case-card__status"><span className={`nx-dot${meta.dot ? ` nx-dot--${meta.dot}` : ''}`} aria-hidden="true" />{status.text}</span>
    </NavLink>
  )
}

export function Sidebar({ caseId, overview, attention, collapsed, onToggleRail, onNavigate }) {
  const location = useLocation()
  const entries = withPending(configuredCaseIds(), attention.cases, attention.settled)
  const recent = sidebarCases(entries, caseId, 5)
  const status = caseId ? readiness(overview) : null
  const counts = { cases: entries.length }
  return (
    <aside className="nx-sidebar" aria-label="Workspace">
      <NavLink to="/" className="nx-sidebar__brand" aria-label="NexusAI, dashboard" onClick={onNavigate}>
        <BrandMark />
        <span className="nx-brand-name">NexusAI</span>
      </NavLink>
      <div className="nx-sidebar__scroll">
        <nav aria-label="Workspace sections">
          <div className="nx-nav-label">Workspace</div>
          <div className="nx-nav">
            {workspaceNav.map(item => <Item key={item.id} item={item} collapsed={collapsed} count={counts[item.id]} onNavigate={onNavigate} />)}
          </div>
        </nav>
        {caseId ? (
          <nav aria-label="This case">
            <CaseCard caseId={caseId} overview={overview} onNavigate={onNavigate} />
            <div className="nx-nav">
              {caseNav(caseId).map(item => (
                <Item key={item.id} item={item} collapsed={collapsed} onNavigate={onNavigate}
                  count={item.id === 'evidence' ? status?.failed : 0} attention />
              ))}
            </div>
          </nav>
        ) : null}
        {recent.length ? (
          <nav aria-label="Recent cases" className="nx-recent">
            <div className="nx-nav-label">Cases</div>
            {recent.map(item => {
              const meta = CASE_STATE[item.state] || CASE_STATE.unreported
              const inside = location.pathname.startsWith(`/cases/${encodeURIComponent(item.caseId)}/`)
              return (
                <NavLink key={item.caseId} to={`/cases/${encodeURIComponent(item.caseId)}/overview`} className="nx-recent__item" aria-current={inside ? 'true' : undefined}
                  onClick={() => { if (item.caseId !== caseId) clearCaseSessionScope(); onNavigate?.() }} title={`${item.caseId} · ${meta.label}`}>
                  <span className={`nx-dot${meta.dot ? ` nx-dot--${meta.dot}` : ''}`} aria-hidden="true" />
                  <span className="nx-recent__name"><CaseLabel caseId={item.caseId} /></span>
                  <span className="nx-visually-hidden">, {meta.label}</span>
                </NavLink>
              )
            })}
          </nav>
        ) : null}
      </div>
      <div className="nx-sidebar__foot">
        <NavLink to="/cases/new" className="nx-btn nx-btn--ghost nx-btn--icon" aria-label="New case" title="New case" onClick={onNavigate}><Plus aria-hidden="true" /></NavLink>
        <NavLink to="/settings" className="nx-btn nx-btn--ghost nx-btn--icon" aria-label="Settings" title="Settings" onClick={onNavigate}><Settings2 aria-hidden="true" /></NavLink>
        <span style={{ flex: 1 }} />
        <button type="button" className="nx-btn nx-btn--ghost nx-btn--icon" aria-label={collapsed ? 'Expand the navigation' : 'Collapse the navigation'} aria-pressed={collapsed} title={collapsed ? 'Expand the navigation' : 'Collapse the navigation'} onClick={onToggleRail}>
          {collapsed ? <PanelLeftOpen aria-hidden="true" /> : <PanelLeftClose aria-hidden="true" />}
        </button>
      </div>
    </aside>
  )
}
