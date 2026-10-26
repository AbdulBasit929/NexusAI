import { Link, useNavigate, useParams } from 'react-router-dom'
import { MessageSquareText } from 'lucide-react'
import { CaseShell } from '../components/CaseShell.jsx'
import { Skeleton } from '../components/Skeleton.jsx'
import { EmptyState } from '../components/AnalystComponents.jsx'
import { formatNumber } from '../lib/format.js'
import { useCaseReads } from '../lib/workbench/useWorkbench.js'
import { WorkbenchUrl } from './workbench/WorkbenchCore.jsx'

// CONNECTIONS: THE WHOLE CASE (master plan 7.3). Every subscriber of the case and the contacts they share, the cell sites they used, the days they were active, and the evidence of whatever is
// selected: a number, a link, a site. There is no question field here: questions are asked on the Ask page, and an answer that can be drawn (a window, a meeting, a successor, a move, a speed alert,
// a change of rhythm, a connection) is drawn there, with this same picture inside it. From here, "Ask about this case" takes the numbers on screen to the Ask page as a question.
function Frame({ caseId, children }) {
  return <CaseShell caseId={caseId}><main id="workspace-main" className="wbp wbp--b" tabIndex={-1}>{children}</main></CaseShell>
}

export default function WorkbenchPage() {
  const { id: caseId } = useParams()
  const navigate = useNavigate()
  const reads = useCaseReads(caseId)
  const askLink = question => `/cases/${encodeURIComponent(caseId)}/investigate?${new URLSearchParams(question ? { question, run: '1' } : {})}`.replace(/\?$/, '')

  if (reads.status === 'loading') return <Frame caseId={caseId}><Skeleton lines={8} label="Reading the connections and places of this case" /></Frame>
  if (reads.status === 'failed') {
    return (
      <Frame caseId={caseId}>
        <EmptyState kind="unavailable" label="The connections could not be read" description={`The request failed (${reads.error?.reference || 'no reference'}). Nothing was changed. Try again, or ask the question in Ask this case.`} />
        <p><button type="button" className="wbp-btn" onClick={reads.reload}>Try again</button></p>
      </Frame>
    )
  }
  if (reads.status === 'unavailable') {
    return (
      <Frame caseId={caseId}>
        <EmptyState kind="unavailable" label="This case cannot be drawn as connections yet" description={`${reads.reason} A picture over part of the records would look like a finding and is not shown. Ask this case still answers from the records it can read.`} />
      </Frame>
    )
  }
  const { index } = reads
  const totals = index.totals
  return (
    <Frame caseId={caseId}>
      <div className="wbp-root">
        <header className="wbp-head">
          <div>
            <h1>Connections</h1>
            <p className="wbp-sub">Who is in contact with whom, where they were, and when, across all the evidence of this case. {formatNumber(totals.subjects ?? index.subjects.length)} subscribers, {formatNumber(totals.counterparties ?? 0)} other numbers in contact, {formatNumber(totals.events ?? 0)} events.</p>
          </div>
          <div className="wbp-head__acts">
            <Link className="wbp-btn wbp-btn--primary" to={askLink('')}><MessageSquareText size={14} aria-hidden="true" /> Ask this case</Link>
          </div>
        </header>
        <WorkbenchUrl key={caseId} caseId={caseId} index={index} places={reads.places} onAsk={question => navigate(askLink(question))} />
      </div>
    </Frame>
  )
}
