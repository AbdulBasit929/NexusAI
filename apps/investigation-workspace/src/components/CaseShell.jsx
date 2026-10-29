import { useEffect, useRef, useState } from 'react'
import { NavLink, useLocation, useNavigate } from 'react-router-dom'
import {
  ArrowLeftRight,
  Bell,
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
import { useWorkspaceAttention } from '../lib/workspaceAttention.js'
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
    connections: Network,
    close: X,
    dashboard: LayoutDashboard,
    evidence: Database,
    investigate: SearchCheck,
    menu: Menu,
    'chevron-down': ChevronDown,
    overview: PanelsTopLeft,
    settings: Settings2,
    timeline: Milestone,
    switch: ArrowLeftRight,
    collapse: PanelLeftClose,
    expand: PanelLeftOpen,
    user: CircleUserRound,
  }
  const Icon = icons[name] || PanelsTopLeft
  return <Icon className="shell-icon" strokeWidth={1.75} aria-hidden="true" />
}

function readRailPreference() {
  try { return globalThis.localStorage?.getItem(railPreferenceKey) === 'true' } catch { return false }
}

function useCompactRail() {
  const query = '(width < 1100px)'
  const [compact, setCompact] = useState(() => globalThis.matchMedia?.(query).matches || false)
  useEffect(() => {
    const media = globalThis.matchMedia?.(query)
    if (!media) return undefined
    const update = event => setCompact(event.matches)
    media.addEventListener?.('change', update)
    return () => media.removeEventListener?.('change', update)
  }, [])
  return compact
}

function Navigation({ close, collapsed = false }) {
  const link = item => {
    const accessibleName = item.label
    return (
      <NavLink key={item.path} className="shell-navigation__link" to={item.path} onClick={close} end={item.path === '/'} aria-label={collapsed ? accessibleName : undefined}>
        <ShellIcon name={item.icon} /><span className="shell-navigation__text">{item.label}</span>
        <span className="rail-tooltip" aria-hidden="true">{accessibleName}</span>
      </NavLink>
    )
  }
  return (
    <div className="shell-navigation">
      <nav className="shell-navigation__group" aria-label="Workspace">
        <span className="shell-navigation__label">Workspace</span>
        {globalLinks.map(link)}
      </nav>
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
// Arrow navigation supplements the normal Tab path; mobile uses the native select below.
function CaseTabs({ caseId }) {
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
            return (
              <li key={item.path}>
                <NavLink className="case-tabs__link" to={path}>
                  <ShellIcon name={item.icon} /><span>{item.label}</span>
                </NavLink>
              </li>
            )
          })}
        </ul>
      </div>
    </nav>
  )
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
  const compactRail = useCompactRail()
  const effectiveCollapsed = collapsed || compactRail
  const cases = configuredCaseIds()
  const attention = useWorkspaceAttention()

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
    <div className={`workspace-shell${caseId ? ' workspace-shell--case' : ''}${collapsed ? ' workspace-shell--collapsed' : ''}${compactRail ? ' workspace-shell--rail-compact' : ''}`}>
      <header className="workspace-header">
        <div className="workspace-header__identity">
          <button ref={trigger} type="button" className="drawer-trigger" aria-label="Open navigation" aria-expanded={drawerOpen} aria-controls="navigation-drawer" onClick={() => setDrawerOpen(true)}><ShellIcon name="menu" /><span>Menu</span></button>
          <NavLink className="identity-link" to="/" aria-label="NexusAI dashboard"><BrandLockup identityLed={identityLed} /></NavLink>
        </div>
        <div className="workspace-header__scope">
          {caseId && cases.length > 1 ? (
            <button type="button" className="case-switcher-trigger" onClick={openCaseSwitcher} title="Switch active case scope (Alt+S)">
              <ShellIcon name="cases" /><span className="workspace-header__scope-copy"><small>Current case</small><span className="workspace-header__scope-name"><CaseTitle caseId={caseId} /></span></span><ShellIcon name="chevron-down" />
            </button>
          ) : caseId ? (
            <div className="case-switcher-trigger case-switcher-trigger--static">
              <ShellIcon name="cases" /><span className="workspace-header__scope-copy"><small>Current case</small><span className="workspace-header__scope-name"><CaseTitle caseId={caseId} /></span></span>
            </div>
          ) : <NavLink className="case-switcher-trigger" to="/cases"><ShellIcon name="cases" /><span className="workspace-header__scope-copy"><small>Workspace</small><span className="workspace-header__scope-name">All cases</span></span></NavLink>}
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
          <Navigation collapsed={effectiveCollapsed} />
          {!compactRail ? <div className="desktop-rail__footer"><button type="button" className="rail-collapse" aria-label={collapsed ? 'Expand navigation' : 'Collapse navigation'} aria-expanded={!collapsed} onClick={() => setCollapsed(value => !value)}><ShellIcon name={collapsed ? 'expand' : 'collapse'} /><span className="rail-collapse__text">{collapsed ? 'Expand' : 'Collapse'}</span><span className="rail-tooltip" aria-hidden="true">{collapsed ? 'Expand navigation' : 'Collapse navigation'}</span></button></div> : null}
        </aside>
        <div className="workspace-content">
          <ConnectivityBanner />
          {caseId ? (
            <section className="case-context-band" aria-label="Case context">
              <CaseTabs caseId={caseId} />
            </section>
          ) : null}
          {children}
        </div>
      </div>
      {drawerOpen ? <div className="drawer-backdrop" onMouseDown={event => event.target === event.currentTarget && setDrawerOpen(false)}><aside ref={drawer} id="navigation-drawer" className="navigation-drawer" role="dialog" aria-modal="true" aria-label="Navigation"><header><BrandLockup /><button type="button" aria-label="Close navigation" onClick={() => { setDrawerOpen(false); trigger.current?.focus() }}><ShellIcon name="close" /></button></header><Navigation close={() => setDrawerOpen(false)} /></aside></div> : null}
      <CaseSwitcherDialog caseId={caseId} cases={cases} onClose={closeCaseSwitcher} onSwitch={switchCase} restoreFocusRef={caseSwitchRestore} trigger={caseSwitcherTrigger} />
    </div>
  )
}

export const AppShell = CaseShell
