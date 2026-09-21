import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { Link, Outlet, useLocation, useNavigate } from 'react-router-dom'
import NexusLoadingState from '../components/NexusLoadingState'
import { ToastContainer, useToast } from '../components/Toast'
import { useAuth } from '../context/AuthContext'
import { useActiveCase } from '../contexts/ActiveCaseContext'
import { useTheme } from '../contexts/ThemeContext'
import { recordsApi } from '../utils/api'
import './AnalystPortal.css'
import './InvestigationWorkspace.css'
import { friendlyWorkspaceName } from './analystPresentation'
import { homeEvidenceSummary } from './analystHomePresentation'
import { investigationWorkspaceIdentity } from './investigationWorkspace'

const AnalystPortalContext = createContext(null)

function withWorkspace(path, caseId) {
  if (!caseId) return path
  const [pathname, query = ''] = path.split('?')
  const params = new URLSearchParams(query)
  params.set('case', caseId)
  return `${pathname}?${params.toString()}`
}

export function useAnalystPortal() {
  const context = useContext(AnalystPortalContext)
  if (!context) throw new Error('useAnalystPortal must be used within AnalystPortalLayout')
  return context
}

export default function AnalystPortalLayout() {
  const location = useLocation()
  const navigate = useNavigate()
  const mainRef = useRef(null)
  const { theme, toggleTheme } = useTheme()
  const { user, isAdmin, logout } = useAuth()
  const {
    activeCase,
    caseOptions,
    state: workspaceState,
    error: workspaceError,
    registryState,
    refreshActiveCase,
  } = useActiveCase()
  const { toasts, addToast, removeToast } = useToast()
  const [dataState, setDataState] = useState({ status: 'idle', error: '', caseInfo: null, statusData: null, capabilities: null, manifest: null, evidence: null })
  const [accountOpen, setAccountOpen] = useState(false)
  const [newConversationSignal, setNewConversationSignal] = useState(0)
  const caseId = activeCase?.caseId || ''

  const refresh = useCallback(async () => {
    if (!caseId) return
    const requestedCase = caseId
    setDataState(previous => ({ ...previous, status: 'loading', error: '' }))
    try {
      const [caseInfo, statusData, capabilities, manifest, evidence] = await Promise.all([
        recordsApi.forensicCase(caseId),
        recordsApi.forensicStatus({ collection_id: caseId, limit: 8 }),
        recordsApi.forensicCapabilities({ collection_id: caseId }),
        recordsApi.forensicCaseManifest(caseId),
        recordsApi.forensicCaseEvidence(caseId, { limit: 12, offset: 0 }),
      ])
      setDataState({ status: 'ready', error: '', caseInfo, statusData, capabilities, manifest, evidence, caseId: requestedCase })
    } catch (error) {
      setDataState(previous => ({ ...previous, status: 'error', error: error.message, caseId: requestedCase }))
    }
  }, [caseId])

  useEffect(() => {
    if (workspaceState === 'ready' && caseId) refresh()
  }, [caseId, refresh, workspaceState])

  useEffect(() => {
    if (!accountOpen) return undefined
    const closeOnEscape = event => { if (event.key === 'Escape') setAccountOpen(false) }
    window.addEventListener('keydown', closeOnEscape)
    return () => window.removeEventListener('keydown', closeOnEscape)
  }, [accountOpen])

  const selectWorkspace = event => {
    const nextCaseId = event.target.value
    const params = new URLSearchParams(location.search)
    params.set('case', nextCaseId)
    navigate(`/analyst?${params.toString()}`)
  }

  const evidenceSummary = homeEvidenceSummary(dataState.statusData, dataState.evidence)

  const startNewConversation = useCallback(() => {
    setAccountOpen(false)
    setNewConversationSignal(value => value + 1)
    navigate(withWorkspace('/analyst', caseId))
  }, [caseId, navigate])

  const portal = useMemo(() => ({
    activeWorkspace: activeCase,
    workspaces: caseOptions,
    workspaceState,
    workspaceError,
    registryState,
    dataState,
    refresh,
    refreshWorkspaces: refreshActiveCase,
    path: path => withWorkspace(path, caseId),
    newConversationSignal,
    startNewConversation,
  }), [activeCase, caseId, caseOptions, dataState, newConversationSignal, refresh, refreshActiveCase, registryState, startNewConversation, workspaceError, workspaceState])

  const focusMain = event => {
    event.preventDefault()
    mainRef.current?.focus({ preventScroll: true })
  }

  return (
    <AnalystPortalContext.Provider value={portal}>
      <div className="analyst-portal premium-workspace">
        <a className="skip-link" href="#analyst-main" onClick={focusMain}>Skip to content</a>
        <header className="analyst-topbar">
          <Link to={portal.path('/analyst')} className="analyst-brand" aria-label={`${investigationWorkspaceIdentity.name} home`}>
            <span className="analyst-brand-mark" aria-hidden="true"><i className="fas fa-magnifying-glass" /></span>
            <span className="analyst-brand-copy"><strong>{investigationWorkspaceIdentity.name}</strong></span>
          </Link>
          <label className="analyst-workspace-select">
            <span>Investigation</span>
            <select value={caseId} onChange={selectWorkspace} disabled={registryState !== 'ready'} aria-label="Current investigation">
              {!caseId && <option value="">Choose investigation</option>}
              {caseOptions.map(item => <option key={item.caseId} value={item.caseId}>{friendlyWorkspaceName(item)}</option>)}
            </select>
          </label>
          <div className="analyst-workspace-actions">
            <button className="workspace-theme-toggle" type="button" onClick={toggleTheme} aria-label={`Switch to ${theme === 'dark' ? 'light' : 'dark'} theme`} title={`Switch to ${theme === 'dark' ? 'light' : 'dark'} theme`}><i className={`fas ${theme === 'dark' ? 'fa-sun' : 'fa-moon'}`} aria-hidden="true" /></button>
            <Link className="analyst-evidence-count" to={portal.path('/analyst?panel=evidence')} aria-label={`Open evidence, ${evidenceSummary.total} source${evidenceSummary.total === 1 ? '' : 's'}`}><i className="fas fa-folder-open" aria-hidden="true" /><span>{evidenceSummary.total.toLocaleString()} source{evidenceSummary.total === 1 ? '' : 's'}</span>{evidenceSummary.attention > 0 && <span className="analyst-evidence-attention"><i className="fas fa-circle-exclamation" aria-hidden="true" />{evidenceSummary.attention.toLocaleString()} needs attention</span>}</Link>
            <Link className="analyst-add-trigger" to={portal.path('/analyst?add=true')}><i className="fas fa-plus" aria-hidden="true" /><span>Add data</span></Link>
            <div className={`analyst-account-menu${accountOpen ? ' open' : ''}`}>
              <button className="analyst-account-trigger" type="button" aria-label="Workspace menu" aria-haspopup="menu" aria-expanded={accountOpen} onClick={() => setAccountOpen(open => !open)}><i className="fas fa-ellipsis" aria-hidden="true" /></button>
              {accountOpen && <div className="analyst-account-panel" role="menu">
                <button type="button" role="menuitem" onClick={startNewConversation}><i className="fas fa-pen-to-square" aria-hidden="true" /> New conversation</button>
                <Link to={portal.path('/analyst?panel=history')} role="menuitem" onClick={() => setAccountOpen(false)}><i className="fas fa-clock-rotate-left" aria-hidden="true" /> History</Link>
                <p className="analyst-account-label"><strong>{user?.name || 'Local analyst'}</strong><span>{isAdmin ? 'Administrator access' : 'Analyst access'}</span></p>
                <button type="button" role="menuitem" onClick={toggleTheme}><i className={`fas ${theme === 'dark' ? 'fa-sun' : 'fa-moon'}`} /> {theme === 'dark' ? 'Light mode' : 'Dark mode'}</button>
                <button type="button" role="menuitem" onClick={() => navigate('/app/account')}><i className="fas fa-user" /> Profile</button>
                {isAdmin && <button type="button" role="menuitem" onClick={() => navigate('/app')}><i className="fas fa-sliders" /> Advanced workspace</button>}
                {user && <button type="button" role="menuitem" onClick={logout}><i className="fas fa-right-from-bracket" /> Log out</button>}
              </div>}
            </div>
          </div>
        </header>

        <main id="analyst-main" ref={mainRef} className="analyst-main" tabIndex="-1">
          {(workspaceState === 'idle' || workspaceState === 'loading') && <NexusLoadingState label="Opening your workspace…" />}
          {(workspaceState === 'error' || workspaceState === 'inaccessible') && (
            <section className="analyst-state" role="alert">
              <i className="fas fa-triangle-exclamation" />
              <h1>Workspace unavailable</h1>
              <p>{workspaceError || 'The authorized workspace could not be opened.'}</p>
              <button className="btn btn-primary" type="button" onClick={() => refreshActiveCase()}>Try again</button>
            </section>
          )}
          {workspaceState === 'ready' && activeCase && <Outlet context={{ addToast, portal }} />}
        </main>

        <ToastContainer toasts={toasts} removeToast={removeToast} />
      </div>
    </AnalystPortalContext.Provider>
  )
}
