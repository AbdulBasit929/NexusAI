import { useState } from 'react'
import { DataQualityCard } from '../../components/DataQualityCard.jsx'
import { EngineHealthCard } from '../../components/EngineHealthCard.jsx'

// Under the question box: how the data of this case was read, and how the Ask engine has done on it. Closed by default and read only when it is opened, so a case that is only
// being asked about costs the service nothing. Each card shows nothing when its read is unavailable or the case has nothing to say.
export function CaseDataPanel({ caseId }) {
  const [open, setOpen] = useState(false)
  return (
    <details className="case-data" onToggle={event => setOpen(event.currentTarget.open)}>
      <summary>About the data here and how the engine has done</summary>
      {open ? (
        <div className="case-data__body">
          <DataQualityCard caseId={caseId} />
          <EngineHealthCard caseId={caseId} />
        </div>
      ) : null}
    </details>
  )
}
