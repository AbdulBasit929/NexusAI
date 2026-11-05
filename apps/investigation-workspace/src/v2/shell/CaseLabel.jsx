import { useCaseRecords } from '../../lib/useCaseRecords.js'
import { humanizeCaseId } from '../lib/caseLabel.js'

// The name of a case: the one an analyst gave it, otherwise its identifier read as a title. The identifier is always the title attribute.
// The text is user text and is only ever rendered as text.
export function useCaseLabel(caseId) {
  const { records } = useCaseRecords()
  const named = records[caseId]?.display_name
  if (named) return { title: named, date: '', named: true }
  return { ...humanizeCaseId(caseId), named: false }
}

export function CaseLabel({ caseId, withDate = false }) {
  const { title, date } = useCaseLabel(caseId)
  return <bdi title={caseId} dir="auto">{title || caseId}{withDate && date ? <span className="nx-muted"> · {date}</span> : null}</bdi>
}

// The same name as text, for a table cell or a sentence: the one an analyst gave, otherwise the identifier read as a title.
export function useCaseNameOf() {
  const { records } = useCaseRecords()
  return caseId => records[caseId]?.display_name || humanizeCaseId(caseId).title || caseId
}
