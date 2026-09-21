import AgentChat from '../pages/AgentChat'
import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { useAnalystPortal } from './AnalystPortalLayout'
import { analystAskScope } from './analystAskPresentation'
import { capabilitySuggestions, citationEvidenceId, friendlyWorkspaceName } from './analystPresentation'
import { recordsApi } from '../utils/api'

export default function AnalystAsk() {
  const portal = useAnalystPortal()
  const [searchParams, setSearchParams] = useSearchParams()
  const catalogItems = portal.dataState.evidence?.items || []
  const evidenceId = searchParams.get('evidence') || ''
  const evidence = catalogItems.find(item => item?.evidence_id === evidenceId) || (evidenceId ? { evidence_id: evidenceId } : null)
  const [detail, setDetail] = useState(null)
  useEffect(() => {
    let active = true
    setDetail(null)
    if (!evidenceId || !portal.activeWorkspace.caseId) return () => { active = false }
    recordsApi.forensicCaseEvidenceDetail(portal.activeWorkspace.caseId, evidenceId, { limit: 250, include_records_preview: false })
      .then(value => { if (active) setDetail(value) })
      .catch(() => { if (active) setDetail({}) })
    return () => { active = false }
  }, [evidenceId, portal.activeWorkspace.caseId])
  const scope = analystAskScope({ ...portal.activeWorkspace, displayName: friendlyWorkspaceName(portal.activeWorkspace) }, evidence, detail)
  // Workspace suggestions are not evidence-specific promises. A selected source
  // retains its own governed query scope without unrelated workspace examples.
  const suggestions = evidenceId ? [] : capabilitySuggestions(portal.dataState.capabilities, 6)
  return (
    <div className="analyst-ask-shell">
      <header className="analyst-ask-heading">
        <div><h1>Ask</h1><p>Ask questions across the evidence available in this investigation.</p></div>
        <Link to={portal.path('/analyst/activity')}>View Activity <i className="fas fa-arrow-right" aria-hidden="true" /></Link>
      </header>
      <div className="analyst-ask-scope-controls" aria-label="Evidence scope">
        <span>{evidenceId ? 'Selected evidence' : 'All available evidence'}</span>
        {evidenceId ? <button type="button" onClick={() => { const next = new URLSearchParams(searchParams); next.delete('evidence'); next.delete('prompt'); setSearchParams(next) }}>Search all available evidence</button>
          : <Link to={portal.path('/analyst/data')}>Choose evidence in Data</Link>}
      </div>
      <AgentChat
        agentName="Forensic_Records_Analyst"
        portalMode
        portalPrompts={suggestions}
        portalScope={scope}
        resolveCitationEvidenceId={source => citationEvidenceId(source, catalogItems)}
      />
    </div>
  )
}
