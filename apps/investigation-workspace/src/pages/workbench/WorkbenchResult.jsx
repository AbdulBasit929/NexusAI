import { useMemo } from 'react'
import { readAnswer } from '../../lib/workbench/answerMarks.js'
import { useCaseReads } from '../../lib/workbench/useWorkbench.js'
import { WorkbenchLocal } from './WorkbenchCore.jsx'

// An answer of the Ask engine, drawn: the picture of the workbench inside the answer, from the rows the engine returned. The case's own reads (who is in contact with whom, the cell sites, the days)
// are what the answer's marks are drawn over; they are read once for the visit. While they arrive the answer is already there in words; if they cannot be read, the answer's ordinary table stays.
export function WorkbenchResult({ presentation, caseId, onAsk, fallback }) {
  const reads = useCaseReads(caseId)
  const answer = useMemo(() => (reads.status === 'ready' ? readAnswer(presentation.original, { index: reads.index, question: presentation.question || '', caseId }) : null), [reads, presentation, caseId])
  if (reads.status === 'loading') return <p className="chart-card__loading" role="status">Preparing the picture of this answer…</p>
  if (reads.status !== 'ready') {
    return (
      <>
        <p className="wbp-hint" role="status">The picture of this case could not be read, so the answer is shown as a table. {reads.status === 'unavailable' ? reads.reason : ''}</p>
        {fallback}
      </>
    )
  }
  return <WorkbenchLocal caseId={caseId} index={reads.index} places={reads.places} answer={answer} onAsk={onAsk} />
}
