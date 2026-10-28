import { useEffect, useId, useRef, useState } from 'react'
import { NavLink, useLocation, useNavigate } from 'react-router-dom'
import {
  ArrowLeftRight,
  Bell,
  ChevronLeft,
  ChevronRight,
  CircleCheckBig,
  Clock3,
  Database,
  Folders,
  LayoutDashboard,
  ChevronDown,
  Menu,
  MessageSquareText,
  Network,
  Milestone,
  PanelLeftClose,
  PanelLeftOpen,
  PanelsTopLeft,
  Plus,
  SearchCheck,
  Settings2,
  SunMoon,
  TriangleAlert,
  X,
  CircleUserRound,
} from 'lucide-react'
import { configuredCaseIds } from '../lib/apiClient.js'
import { BrandLockup } from './BrandLockup.jsx'
import { LanguageText } from './AnalystComponents.jsx'
import { CaseTitle } from './CaseTitle.jsx'
import { AppearanceMenu } from './AppearanceMenu.jsx'
import { WorkspaceCommands } from './WorkspaceCommands.jsx'
import { clearCaseSessionScope } from '../lib/workspaceState.js'
import { useCaseOverview } from '../lib/useCaseOverview.js'
import { sidebarCases, useWorkspaceAttention, withPending } from '../lib/workspaceAttention.js'
import { formatNumber } from '../lib/format.js'

const globalLinks = [
  { label: 'Dashboard', path: '/', icon: 'dashboard' },
  { label: 'Ask across cases', path: '/investigate', icon: 'investigate' },
  { label: 'Cases', path: '/cases', icon: 'cases' },
  { label: 'Activity', path: '/activity', icon: 'activity' },
]
const caseLinks = [
  { label: 'Overview', path: 'overview', icon: 'overview' },
  { label: 'Evidence', path: 'evidence', icon: 'evidence' },
  { label: 'Ask', path: 'investigate', icon: 'ask' },
  { label: 'Connections', path: 'connections', icon: 'connections' },
  { label: 'Timeline', path: 'timeline', icon: 'timeline' },
  { label: 'Activity', path: 'activity', icon: 'activity' },
]
// Cases with a local data inventory (public/runtime-config.js, dataInventoryCaseIds) get a Data tab.
const dataCaseIds = () => globalThis.__INVESTIGATION_WORKSPACE_CONFIG__?.dataInventoryCaseIds || []
const focusable = 'a[href], button:not([disabled]), input:not([disabled]), [tabindex]:not([tabindex="-1"])'
const railPreferenceKey = 'nexusai.viewer.navigation-collapsed'

function ShellIcon({ name }) {
  const icons = {
    activity: Clock3,
    bell: Bell,
    ask: MessageSquareText,
    alert: TriangleAlert,
    appearance: SunMoon,
    cases: Folders,
    check: CircleCheckBig,
    connections: Network,
    close: X,
    dashboard: LayoutDashboard,
    evidence: Database,
    investigate: SearchCheck,
    menu: Menu,
    plus: Plus,
    'chevron-down': ChevronDown,
    next: ChevronRight,
    overview: PanelsTopLeft,
    previous: ChevronLeft,
    settings: Settings2,
    timeline: Milestone,
    switch: ArrowLeftRight,
    collapse: PanelLeftClose,
    expand: PanelLeftOpen,
    user: CircleUserRound,
  }
  const Icon = icons[name] || PanelsTopLeft
  return <Icon className="shell-icon" aria-hidden="true" />
}

function readRailPreference() {
  try { return globalThis.localStorage?.getItem(railPreferenceKey) === 'true' } catch { return false }
}

const casesOpenKey = 'nexusai.viewer.rail-cases-open'
function readCasesOpen() {
  try { return globalThis.localStorage?.getItem(casesOpenKey) !== 'false' } catch { return true }
}

const CASE_STATE = {
  attention: { label: 'Needs review', icon: 'alert' },
  processing: { label: 'Processing', icon: 'activity' },
  complete: { label: 'Ready', icon: 'check' },
  'not-processed': { label: 'No evidence', icon: 'evidence' },
  unreported: { label: 'Status unavailable', icon: 'alert' },
  loading: { label: 'Checking status', icon: 'activity' },
}

// "Cases" is one category, not two lists: the row opens the directory, the chevron folds the cases under it,
// and the plus starts a new case. Each case is one line; status is an icon plus text for assistive
// technology, never colour alone.
function CasesCategory({ entries, activeCaseId, onNavigate, badges }) {
  const listId = useId()
  const location = useLocation()
  const [open, setOpen] = useState(readCasesOpen)
  const items = sidebarCases(entries, activeCaseId)
  useEffect(() => {
    try { globalThis.localStorage?.setItem(casesOpenKey, String(open)) } catch { /* the rail stays usable without storage */ }
  }, [open])
  const hidden = entries.length - items.length
  return (
    <div className="rail-category">
      <div className="rail-category__row">
        <NavLink className="shell-navigation__link" to="/cases" end onClick={onNavigate}>
          <ShellIcon name="cases" /><span className="shell-navigation__text">Cases</span>
          <span className="rail-category__count" aria-label={`${entries.length} ${entries.length === 1 ? 'case' : 'cases'}`}>{formatNumber(entries.length)}</span>
        </NavLink>
        <button type="button" className="rail-category__button" aria-expanded={open} aria-controls={listId} aria-label={open ? 'Hide cases' : 'Show cases'} onClick={() => setOpen(value => !value)}><ShellIcon name={open ? 'chevron-down' : 'next'} /></button>
        <NavLink className="rail-category__button" to="/cases/new" aria-label="New case" onClick={onNavigate}><ShellIcon name="plus" /></NavLink>
      </div>
      {open ? (
        <ul id={listId} className="rail-category__list" aria-label="Cases">
          {items.map(item => {
            const state = CASE_STATE[item.state]
            const inside = location.pathname.startsWith(`/cases/${encodeURIComponent(item.caseId)}/`)
            const failed = Number(entries.find(entry => entry.caseId === item.caseId)?.summary?.failed || 0)
            return (
              <li key={item.caseId}>
                <NavLink className={`rail-case rail-case--${item.state}${inside ? ' is-current' : ''}`} to={`/cases/${encodeURIComponent(item.caseId)}/overview`} aria-current={inside ? 'true' : undefined} title={`${item.caseId} · ${state.label}`} onClick={() => { if (item.caseId !== activeCaseId) clearCaseSessionScope(); onNavigate?.() }}>
                  <ShellIcon name={state.icon} />
                  <CaseTitle caseId={item.caseId} />
                  <span className="visually-hidden">, {state.label}{failed ? `, ${failed} failed ${failed === 1 ? 'source' : 'sources'}` : ''}</span>
                  {failed ? <span className="rail-case__flag" aria-hidden="true">{formatNumber(failed)}</span> : null}
                </NavLink>
              </li>
            )
          })}
          {hidden > 0 ? <li><NavLink className="rail-case rail-case--more" to="/cases" onClick={onNavigate}>{formatNumber(hidden)} more</NavLink></li> : null}
        </ul>
      ) : null}
    </div>
  )
}

function Navigation({ close, collapsed = false, badges = {}, entries = [], activeCaseId = '' }) {
  const link = item => {
    const badge = badges[item.path]
    const accessibleName = badge ? `${item.label}, ${badge.label}` : item.label
    return (
      <NavLink key={item.path} className="shell-navigation__link" to={item.path} onClick={close} end={item.path === '/'} aria-label={collapsed || badge ? accessibleName : undefined} title={collapsed ? accessibleName : undefined}>
        <ShellIcon name={item.icon} /><span className="shell-navigation__text">{item.label}</span>
        {badge ? <span className={`shell-navigation__badge shell-navigation__badge--${badge.tone}`} aria-hidden="true">{formatNumber(badge.count)}</span> : null}
        {collapsed ? <span className="rail-tooltip" aria-hidden="true">{accessibleName}</span> : null}
      </NavLink>
    )
  }
  return (
    <div className="shell-navigation">
      <nav className="shell-navigation__group" aria-label="Workspace">
        <span className="shell-navigation__label">Workspace</span>
        {globalLinks.map(item => (item.path === '/cases' && !collapsed ? <CasesCategory key={item.path} entries={entries} activeCaseId={activeCaseId} onNavigate={close} badges={badges} /> : link(item)))}
      </nav>
      {activeCaseId ? (
        <nav className="shell-navigation__group shell-navigation__case" aria-label="Case">
          <span className="shell-navigation__label">Current case</span>
          <NavLink className="shell-navigation__case-title" to={`/cases/${encodeURIComponent(activeCaseId)}/overview`} onClick={close} title={collapsed ? activeCaseId : undefined}>
            <Folders aria-hidden="true" /><CaseTitle caseId={activeCaseId} />
          </NavLink>
          {caseLinksWithData(activeCaseId).map(item => link({ ...item, path: `/cases/${encodeURIComponent(activeCaseId)}/${item.path}` }))}
        </nav>
      ) : null}
      <nav className="shell-navigation__footer" aria-label="Preferences">
        {link({ label: 'Settings', path: '/settings', icon: 'settings' })}
      </nav>
    </div>
  )
}

function caseLinksWithData(caseId) {
  const items = [...caseLinks]
  if (dataCaseIds().includes(caseId)) items.splice(items.length - 1, 0, { label: 'Data', path: 'data', icon: 'evidence' })
  return items
}

// Case sections are links (not ARIA tabs): each is a different page, so aria-current marks the location.
// Counts appear only once the status has loaded, so the bar never shifts after first paint.
// Enhanced with keyboard arrow navigation (Linear/Vercel standard) and fluid horizontal scrolling.
function CaseTabs({ caseId, badges }) {
  const navRef = useRef(null)
  const navigate = useNavigate()
  const location = useLocation()
  const items = caseLinksWithData(caseId)

  function handleKeyDown(event) {
    if (!['ArrowRight', 'ArrowLeft', 'Home', 'End'].includes(event.key)) return
    const links = Array.from(navRef.current?.querySelectorAll('a.case-tabs__link') || [])
    if (!links.length) return
    const currentIndex = links.indexOf(globalThis.document?.activeElement)
    if (currentIndex === -1) return

    event.preventDefault()
    let nextIndex = currentIndex
    if (event.key === 'ArrowRight') {
      nextIndex = (currentIndex + 1) % links.length
    } else if (event.key === 'ArrowLeft') {
      nextIndex = (currentIndex - 1 + links.length) % links.length
    } else if (event.key === 'Home') {
      nextIndex = 0
    } else if (event.key === 'End') {
      nextIndex = links.length - 1
    }
    links[nextIndex]?.focus()
  }

  return (
    <nav ref={navRef} className="case-tabs" aria-label="Case sections" onKeyDown={handleKeyDown}>
      <label className="case-tabs__select-label">
        <span className="visually-hidden">Case section</span>
        <select value={location.pathname} onChange={event => navigate(event.target.value)}>
          {items.map(item => <option key={item.path} value={`/cases/${encodeURIComponent(caseId)}/${item.path}`}>{item.label}</option>)}
        </select>
      </label>
      <div className="case-tabs__scroll">
        <ul>
          {items.map(item => {
            const path = `/cases/${encodeURIComponent(caseId)}/${item.path}`
            const badge = badges[path]
            return (
              <li key={item.path}>
                <NavLink className="case-tabs__link" to={path} aria-label={badge ? `${item.label}, ${badge.label}` : undefined}>
                  <ShellIcon name={item.icon} /><span>{item.label}</span>
                  {badge ? <span className={`case-tabs__badge case-tabs__badge--${badge.tone}`} aria-hidden="true">{formatNumber(badge.count)}</span> : null}
                </NavLink>
              </li>
            )
          })}
        </ul>
      </div>
    </nav>
  )
}

export function railBadges(attention, caseId, caseState) {
  const badges = {}
  if (attention.needReview > 0) badges['/'] = { count: attention.needReview, tone: 'attention', label: `${attention.needReview} ${attention.needReview === 1 ? 'case needs' : 'cases need'} review` }
  if (attention.processing > 0) badges['/cases'] = { count: attention.processing, tone: 'processing', label: `${attention.processing} ${attention.processing === 1 ? 'case is' : 'cases are'} processing` }
  const failed = Number(caseState?.data?.summary?.evidence_failed || 0)
  if (caseId && failed > 0) badges[`/cases/${encodeURIComponent(caseId)}/evidence`] = { count: failed, tone: 'attention', label: `${failed} failed ${failed === 1 ? 'source' : 'sources'}` }
  return badges
}

// Case-level trail only. Deeper trails (for example Evidence, then one source) belong to the page header,
// so a page never repeats "Workspace / Cases / this case" that the bar already shows.
function CaseBreadcrumb({ caseId, pathname }) {
  const onOverview = pathname === `/cases/${encodeURIComponent(caseId)}/overview` || pathname === `/cases/${caseId}/overview`
  return (
    <nav className="case-breadcrumb" aria-label="Breadcrumb">
      <ol>
        <li><NavLink to="/cases">Cases</NavLink></li>
        <li>{onOverview ? <CaseTitle caseId={caseId} as="span" aria-current="page" /> : <NavLink to={`/cases/${encodeURIComponent(caseId)}/overview`}><CaseTitle caseId={caseId} /></NavLink>}</li>
      </ol>
    </nav>
  )
}

function CaseReadiness({ state }) {
  if (state.loading) return <div className="case-context-band__summary case-context-band__summary--loading" role="status"><ShellIcon name="activity" /><span><strong>Checking evidence readiness</strong><small>Waiting for the case status response.</small></span></div>
  if (state.error) return <div className="case-context-band__summary case-context-band__summary--unavailable" role="status"><ShellIcon name="alert" /><span><strong>Evidence readiness unavailable</strong><small>The case status service did not respond.</small></span></div>
  const summary = state.data?.summary
  if (!summary || !Object.prototype.hasOwnProperty.call(summary, 'evidence_total')) return <div className="case-context-band__summary case-context-band__summary--unavailable" role="status"><ShellIcon name="alert" /><span><strong>Evidence readiness unavailable</strong><small>No authoritative total was reported.</small></span></div>
  const total = Number(summary.evidence_total || 0)
  const ready = Number(summary.evidence_completed || 0)
  const processing = Number(summary.evidence_in_flight || 0)
  const failed = Number(summary.evidence_failed || 0)
  if (total === 0) return <div className="case-context-band__summary case-context-band__summary--empty" role="status"><ShellIcon name="evidence" /><span><strong>No evidence reported</strong><small>This collection has no processed evidence.</small></span></div>
  if (failed > 0) return <div className="case-context-band__summary case-context-band__summary--attention" role="status"><ShellIcon name="alert" /><span><strong>Evidence needs review</strong><small>{formatNumber(failed)} failed · {formatNumber(ready)} ready</small></span></div>
  if (processing > 0) return <div className="case-context-band__summary case-context-band__summary--processing" role="status"><ShellIcon name="activity" /><span><strong>Evidence processing</strong><small>{formatNumber(ready)} ready · {formatNumber(processing)} processing</small></span></div>
  return <div className="case-context-band__summary case-context-band__summary--ready" role="status"><ShellIcon name="check" /><span><strong>Evidence ready</strong><small>{formatNumber(ready)} of {formatNumber(total)} sources ready</small></span></div>
}

function ActivityPopover({ attention }) {
  const [open, setOpen] = useState(false)
  const root = useRef(null)
  const active = attention.cases.filter(item => item.summary?.processingState === 'processing')
  useEffect(() => {
    if (!open) return undefined
    function dismiss(event) {
      if (event.key === 'Escape' || !root.current?.contains(event.target)) setOpen(false)
    }
    globalThis.document?.addEventListener('keydown', dismiss)
    globalThis.document?.addEventListener('pointerdown', dismiss)
    return () => {
      globalThis.document?.removeEventListener('keydown', dismiss)
      globalThis.document?.removeEventListener('pointerdown', dismiss)
    }
  }, [open])
  return (
    <div className="activity-popover" ref={root}>
      <button type="button" className="header-icon-button" aria-label={active.length ? `Activity, ${active.length} processing` : 'Activity'} aria-expanded={open} aria-controls="activity-popover-panel" onClick={() => setOpen(value => !value)}>
        <ShellIcon name="bell" />
        {active.length ? <span className="header-icon-button__badge" aria-hidden="true">{formatNumber(active.length)}</span> : null}
      </button>
      {open ? (
        <section id="activity-popover-panel" className="activity-popover__panel" aria-label="Current activity">
          <header><div><span className="eyebrow">Live status</span><h2>Activity</h2></div><NavLink to="/activity" onClick={() => setOpen(false)}>View all</NavLink></header>
          {active.length ? <ul>{active.slice(0, 4).map(item => <li key={item.caseId}><ShellIcon name="activity" /><span><CaseTitle caseId={item.caseId} /><small>Evidence processing</small></span></li>)}</ul> : <p>No active processing reported.</p>}
          <small className="activity-popover__note">Completed and failed job history is not exposed by the current service.</small>
        </section>
      ) : null}
    </div>
  )
}

function ConnectivityBanner() {
  const [online, setOnline] = useState(() => globalThis.navigator?.onLine !== false)
  useEffect(() => {
    const update = () => setOnline(globalThis.navigator?.onLine !== false)
    globalThis.addEventListener?.('online', update)
    globalThis.addEventListener?.('offline', update)
    return () => {
      globalThis.removeEventListener?.('online', update)
      globalThis.removeEventListener?.('offline', update)
    }
  }, [])
  return online ? null : <div className="connectivity-banner" role="status"><ShellIcon name="alert" /><span><strong>You are offline.</strong> Cached content may remain visible, but new requests cannot complete.</span></div>
}

function CaseSwitcherDialog({ caseId, cases, onClose, onSwitch, restoreFocusRef, trigger }) {
  const dialog = useRef(null)
  useEffect(() => {
    if (!trigger) return undefined
    const items = [...dialog.current.querySelectorAll(focusable)]
    items[0]?.focus()
    function keys(event) {
      if (event.key === 'Escape') { event.preventDefault(); onClose(); return }
      if (event.key !== 'Tab' || !items.length) return
      const first = items[0]; const last = items.at(-1)
      if (event.shiftKey && globalThis.document.activeElement === first) { event.preventDefault(); last.focus() }
      if (!event.shiftKey && globalThis.document.activeElement === last) { event.preventDefault(); first.focus() }
    }
    globalThis.document.addEventListener('keydown', keys)
    return () => {
      globalThis.document.removeEventListener('keydown', keys)
      if (restoreFocusRef.current) trigger?.focus?.()
    }
  }, [onClose, restoreFocusRef, trigger])
  if (!trigger) return null
  return <div className="case-switcher-backdrop" onMouseDown={event => event.target === event.currentTarget && onClose()}>
    <section ref={dialog} className="case-switcher-dialog" role="dialog" aria-modal="true" aria-labelledby="case-switcher-title">
      <header><div><span className="eyebrow">Workspace scope</span><h2 id="case-switcher-title">Switch case</h2></div><button type="button" aria-label="Close case switcher" onClick={onClose}><ShellIcon name="close" /></button></header>
      <p>The new case opens at its overview. Case-scoped filters from this session are cleared before any new request begins.</p>
      <ul>{cases.map(item => <li key={item} className={item === caseId ? 'case-switcher-dialog__current' : ''}>
        <LanguageText identifier>{item}</LanguageText>
        {item === caseId ? <span>Current case</span> : <button type="button" onClick={() => onSwitch(item)}>Open case</button>}
      </li>)}</ul>
    </section>
  </div>
}

export function CaseShell({ caseId = '', children, identityLed = false }) {
  const navigate = useNavigate()
  const location = useLocation()
  const trigger = useRef(null)
  const drawer = useRef(null)
  const caseSwitchRestore = useRef(true)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [caseSwitcherTrigger, setCaseSwitcherTrigger] = useState(null)
  const [collapsed, setCollapsed] = useState(readRailPreference)
  const cases = configuredCaseIds()
  const caseOverview = useCaseOverview(caseId)
  const attention = useWorkspaceAttention()
  const badges = railBadges(attention, caseId, caseOverview)
  const railEntries = withPending(cases, attention.cases, attention.settled)

  useEffect(() => { setDrawerOpen(false) }, [location.pathname])
  useEffect(() => {
    try { globalThis.localStorage?.setItem(railPreferenceKey, String(collapsed)) } catch { /* the shell remains usable in privacy modes */ }
  }, [collapsed])
  useEffect(() => {
    function handleGlobalKeyDown(event) {
      if (event.altKey && (event.key === 's' || event.key === 'S')) {
        if (cases.length > 1) {
          event.preventDefault()
          caseSwitchRestore.current = true
          setCaseSwitcherTrigger(globalThis.document?.querySelector('.case-switcher-trigger') || globalThis.document?.body)
        }
      }
    }
    globalThis.document?.addEventListener('keydown', handleGlobalKeyDown)
    return () => globalThis.document?.removeEventListener('keydown', handleGlobalKeyDown)
  }, [cases.length])
  useEffect(() => {
    if (!drawerOpen) return undefined
    const previous = globalThis.document.activeElement
    const items = [...drawer.current.querySelectorAll(focusable)]
    items[0]?.focus()
    function keys(event) {
      if (event.key === 'Escape') {
        event.preventDefault(); setDrawerOpen(false); trigger.current?.focus(); return
      }
      if (event.key !== 'Tab' || !items.length) return
      const first = items[0]; const last = items.at(-1)
      if (event.shiftKey && globalThis.document.activeElement === first) { event.preventDefault(); last.focus() }
      if (!event.shiftKey && globalThis.document.activeElement === last) { event.preventDefault(); first.focus() }
    }
    globalThis.document.addEventListener('keydown', keys)
    return () => { globalThis.document.removeEventListener('keydown', keys); previous?.focus?.() }
  }, [drawerOpen])

  function switchCase(next) {
    if (!next || next === caseId) return
    caseSwitchRestore.current = false
    setCaseSwitcherTrigger(null)
    clearCaseSessionScope()
    navigate(`/cases/${encodeURIComponent(next)}/overview`)
  }

  function openCaseSwitcher(event) {
    caseSwitchRestore.current = true
    setCaseSwitcherTrigger(event.currentTarget)
  }

  function closeCaseSwitcher() { setCaseSwitcherTrigger(null) }

  return (
    <div className={`workspace-shell${caseId ? ' workspace-shell--case' : ''}${collapsed ? ' workspace-shell--collapsed' : ''}`}>
      <header className="workspace-header">
        <div className="workspace-header__identity">
          <button ref={trigger} type="button" className="drawer-trigger" aria-label="Open navigation" aria-expanded={drawerOpen} aria-controls="navigation-drawer" onClick={() => setDrawerOpen(true)}><ShellIcon name="menu" /><span>Menu</span></button>
          <NavLink className="identity-link" to="/" aria-label="NexusAI dashboard"><BrandLockup identityLed={identityLed} /></NavLink>
        </div>
        <div className="workspace-header__scope">
          {caseId ? (
            <button type="button" className="case-switcher-trigger" onClick={openCaseSwitcher} aria-label="Switch case" title="Switch active case scope (Alt+S)">
              <Folders aria-hidden="true" /><CaseTitle caseId={caseId} /><ShellIcon name="chevron-down" />
            </button>
          ) : <NavLink className="case-switcher-trigger" to="/cases"><Folders aria-hidden="true" /><span>All cases</span></NavLink>}
        </div>
        <div className="workspace-header__search" role="search" aria-label="Workspace search"><WorkspaceCommands caseId={caseId} showTrigger /></div>
        <div className="workspace-header__utilities">
          <ActivityPopover attention={attention} />
          <AppearanceMenu />
          <button type="button" className="analyst-control" aria-label="Analyst account"><ShellIcon name="user" /><span>Analyst</span></button>
        </div>
      </header>
      <div className="workspace-frame">
        <aside className="desktop-rail">
          <Navigation collapsed={collapsed} badges={badges} entries={railEntries} activeCaseId={caseId} />
                    <div className="desktop-rail__footer"><button type="button" className="rail-collapse" aria-label={collapsed ? 'Expand navigation' : 'Collapse navigation'} aria-expanded={!collapsed} title={collapsed ? 'Expand navigation' : undefined} onClick={() => setCollapsed(value => !value)}><ShellIcon name={collapsed ? 'expand' : 'collapse'} />{collapsed ? null : <span>Collapse</span>}</button></div>
        </aside>
        <div className="workspace-content">
          <ConnectivityBanner />
          {caseId ? (
            <section className="case-context-band" aria-label="Case context">
              <div className="case-context-band__identity">
                <CaseBreadcrumb caseId={caseId} pathname={location.pathname} />
              </div>
              <CaseTabs caseId={caseId} badges={badges} />
              <CaseReadiness state={caseOverview} />
            </section>
          ) : null}
          {children}
        </div>
      </div>
      {drawerOpen ? <div className="drawer-backdrop" onMouseDown={event => event.target === event.currentTarget && setDrawerOpen(false)}><aside ref={drawer} id="navigation-drawer" className="navigation-drawer" role="dialog" aria-modal="true" aria-label="Navigation"><header><BrandLockup /><button type="button" aria-label="Close navigation" onClick={() => { setDrawerOpen(false); trigger.current?.focus() }}><ShellIcon name="close" /></button></header><Navigation close={() => setDrawerOpen(false)} badges={badges} entries={railEntries} activeCaseId={caseId} /></aside></div> : null}
      <CaseSwitcherDialog caseId={caseId} cases={cases} onClose={closeCaseSwitcher} onSwitch={switchCase} restoreFocusRef={caseSwitchRestore} trigger={caseSwitcherTrigger} />
    </div>
  )
}

export const AppShell = CaseShell
