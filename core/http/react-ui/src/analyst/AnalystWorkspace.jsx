import { useEffect, useMemo, useRef, useState } from 'react'
import { useLocation, useNavigate, useOutletContext } from 'react-router-dom'
import AgentChat from '../pages/AgentChat'
import { recordsApi } from '../utils/api'
import AnalystAddData from './AnalystAddData'
import AnalystData from './AnalystData'
import AnalystHistory from './AnalystHistory'
import { analystAskScope } from './analystAskPresentation'
import { homeEvidenceSummary } from './analystHomePresentation'
import { capabilitySuggestions, citationEvidenceId, friendlyWorkspaceName, sourceProductState } from './analystPresentation'
import { useAnalystPortal } from './AnalystPortalLayout'

function workspaceParams(location, changes = {}) {
  const params = new URLSearchParams(location.search)
  Object.entries(changes).forEach(([key, value]) => {
    if (value === null || value === undefined || value === '') params.delete(key)
    else params.set(key, String(value))
  })
  return params
}

export default function AnalystWorkspace() {
  const portal = useAnalystPortal()
  const { addToast } = useOutletContext()
  const location = useLocation()
  const navigate = useNavigate()
  const [addFiles, setAddFiles] = useState([])
  const [detail, setDetail] = useState(null)
  const drawerRef = useRef(null)
  const drawerReturnFocusRef = useRef(null)
  const params = new URLSearchParams(location.search)
  const requestedPanel = params.get('panel') || ''
  const panel = ['evidence', 'history'].includes(requestedPanel) ? requestedPanel : ''
  const evidenceId = params.get('evidence') || ''
  const addOpen = params.get('add') === 'true'
  const items = useMemo(() => portal.dataState.evidence?.items || [], [portal.dataState.evidence?.items])
  const summary = useMemo(() => homeEvidenceSummary(portal.dataState.statusData, portal.dataState.evidence), [portal.dataState.evidence, portal.dataState.statusData])
  const evidence = items.find(item => item?.evidence_id === evidenceId) || (evidenceId ? { evidence_id: evidenceId } : null)

  useEffect(() => {
    let active = true
    setDetail(null)
    if (!evidenceId || !portal.activeWorkspace.caseId) return () => { active = false }
    recordsApi.forensicCaseEvidenceDetail(portal.activeWorkspace.caseId, evidenceId, { limit: 250, include_records_preview: false })
      .then(value => { if (active) setDetail(value) })
      .catch(() => { if (active) setDetail({}) })
    return () => { active = false }
  }, [evidenceId, portal.activeWorkspace.caseId])

  const navigateWith = changes => {
    const next = workspaceParams(location, changes)
    navigate(`/analyst${next.size ? `?${next.toString()}` : ''}`)
  }
  const openAdd = (files = []) => {
    setAddFiles(Array.from(files || []))
    navigateWith({ add: 'true', panel: null, evidence: null, analysis: null })
  }
  const closeAdd = () => {
    setAddFiles([])
    navigateWith({ add: null })
  }
  const closePanel = () => navigateWith({ panel: null, evidence: null, analysis: null, source_time: null, page: null, row: null, finding: null })
  const scope = analystAskScope({ ...portal.activeWorkspace, displayName: friendlyWorkspaceName(portal.activeWorkspace) }, evidence, detail)
  const suggestions = evidenceId ? [] : capabilitySuggestions(portal.dataState.capabilities, 6)
  const readyItems = items.filter(item => sourceProductState(item).id === 'ready')
  const waitingMessage = summary.processing > 0
    ? `${summary.processing.toLocaleString()} evidence source${summary.processing === 1 ? ' is' : 's are'} still processing.`
    : summary.attention > 0
      ? `${summary.attention.toLocaleString()} evidence source${summary.attention === 1 ? ' needs' : 's need'} attention before analysis is available.`
      : 'No evidence is currently available for analysis.'

  useEffect(() => {
    if (!panel) return undefined
    drawerReturnFocusRef.current = document.activeElement
    const drawer = drawerRef.current
    const focusableSelector = 'button:not(:disabled), [href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])'
    const focusFirst = window.requestAnimationFrame(() => drawer?.querySelector(focusableSelector)?.focus({ preventScroll: true }))
    const handleDrawerKey = event => {
      if (event.key === 'Escape') {
        const next = workspaceParams(location, { panel: null, evidence: null, analysis: null, source_time: null, page: null, row: null, finding: null })
        navigate(`/analyst${next.size ? `?${next.toString()}` : ''}`)
        return
      }
      if (event.key !== 'Tab' || !drawer) return
      const focusable = Array.from(drawer.querySelectorAll(focusableSelector)).filter(element => element.offsetParent !== null)
      if (!focusable.length) return
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
      if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
    }
    window.addEventListener('keydown', handleDrawerKey)
    return () => {
      window.cancelAnimationFrame(focusFirst)
      window.removeEventListener('keydown', handleDrawerKey)
      drawerReturnFocusRef.current?.focus?.({ preventScroll: true })
    }
  }, [location, navigate, panel])

  return (
    <div className="analyst-unified-workspace">
      {portal.dataState.status === 'error' && <div className="analyst-inline-error analyst-unified-load-error" role="alert"><strong>Evidence status is unavailable.</strong><button type="button" onClick={portal.refresh}>Try again</button><details><summary>Technical details</summary>{portal.dataState.error}</details></div>}

      {['idle', 'loading'].includes(portal.dataState.status) && !portal.dataState.evidence ? <section className="workspace-loading" role="status"><span className="workspace-skeleton" /><h1>Opening investigation</h1><p>Loading evidence scope and readiness…</p></section> : portal.dataState.status !== 'error' && summary.isEmpty ? (
        <section className="analyst-unified-empty" aria-hidden={Boolean(panel || addOpen)} inert={panel || addOpen ? true : undefined} onDragOver={event => event.preventDefault()} onDrop={event => { event.preventDefault(); openAdd(event.dataTransfer.files) }}>
          <span className="analyst-unified-empty__mark" aria-hidden="true"><i className="fas fa-folder-plus" /></span>
          <h1>Add evidence to begin</h1>
          <p>Upload records, documents, images, audio or video. Supported evidence will be prepared automatically so you can ask questions about it.</p>
          <button className="analyst-primary-action" type="button" onClick={() => openAdd()}><i className="fas fa-plus" aria-hidden="true" /> Add data</button>
          <small>or drop files here</small>
        </section>
      ) : (
        <section className="analyst-ask-shell analyst-unified-ask" aria-label="Ask about this investigation" aria-hidden={Boolean(panel || addOpen)} inert={panel || addOpen ? true : undefined}>
            {readyItems.length > 0 ? <AgentChat
              agentName="Forensic_Records_Analyst"
              portalMode
              portalPrompts={suggestions}
              portalScope={scope}
              newConversationSignal={portal.newConversationSignal}
              resolveCitationEvidenceId={source => citationEvidenceId(source, items)}
            /> : <div className="analyst-unified-not-ready" role="status"><i className={`fas ${summary.processing ? 'fa-circle-notch fa-spin' : 'fa-triangle-exclamation'}`} aria-hidden="true" /><h1>{summary.processing ? 'Preparing evidence' : 'Evidence needs attention'}</h1><p>{waitingMessage}</p><button type="button" className="analyst-secondary-action" onClick={() => navigateWith({ panel: 'evidence' })}>Review evidence</button></div>}
        </section>
      )}
      {panel && <div className="analyst-unified-drawer-backdrop" role="presentation" onMouseDown={event => { if (event.target === event.currentTarget) closePanel() }}>
        <aside ref={drawerRef} className="analyst-unified-drawer" role="dialog" aria-modal="true" aria-labelledby="analyst-unified-drawer-title">
          <header className="analyst-unified-drawer__header"><div><h2 id="analyst-unified-drawer-title">{panel === 'history' ? 'History' : 'Evidence'}</h2></div><div className="analyst-unified-drawer__actions">{panel === 'evidence' && <button className="analyst-unified-drawer__add" type="button" onClick={() => openAdd()}><i className="fas fa-plus" aria-hidden="true" /> Add data</button>}<button type="button" onClick={closePanel} aria-label={`Close ${panel === 'history' ? 'history' : 'evidence'}`}><i className="fas fa-xmark" aria-hidden="true" /></button></div></header>
          <div className="analyst-unified-drawer__content">{panel === 'history' ? <AnalystHistory /> : <AnalystData embedded onRequestAdd={openAdd} />}</div>
        </aside>
      </div>}

      {addOpen && <AnalystAddData
        key={`${portal.activeWorkspace.caseId}:${addFiles.map(file => file.name).join('|')}`}
        caseId={portal.activeWorkspace.caseId}
        collectionId={portal.activeWorkspace.collectionId}
        capabilities={portal.dataState.capabilities}
        initialFiles={addFiles}
        onClose={closeAdd}
        onComplete={async () => { await portal.refresh(); addToast?.('Source catalog refreshed.', 'success') }}
        onOpenSource={(nextEvidenceId) => { setAddFiles([]); navigateWith({ add: null, panel: 'evidence', evidence: nextEvidenceId }) }}
      />}
    </div>
  )
}
