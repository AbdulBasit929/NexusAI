import { Navigate } from 'react-router-dom'
import EmptyState from '../components/EmptyState'
import NexusLoadingState from '../components/NexusLoadingState'
import { useActiveCase } from '../contexts/ActiveCaseContext'

export default function RecordsRedirect() {
  const { activeCase, state, error, refreshActiveCase } = useActiveCase()

  if (state === 'ready' && activeCase) {
    return <Navigate to={`/app/cases/${encodeURIComponent(activeCase.caseId)}/ask`} replace />
  }

  if (state === 'error' || state === 'inaccessible') return (
    <div className="page page--narrow">
      <EmptyState
        state="error"
        eyebrow="Case context"
        title="Case workspace unavailable"
        headingLevel={1}
        body={`NexusAI could not resolve an authorized case: ${error}`}
        actions={<button type="button" className="btn btn-primary" onClick={() => refreshActiveCase()}>Retry case registry</button>}
      />
    </div>
  )

  return <NexusLoadingState label="Resolving authorized case context…" />
}
