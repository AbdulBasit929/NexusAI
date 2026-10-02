import { AlertTriangle, Inbox, MessageSquareText, FileText, Layers } from 'lucide-react'
import { relativeAge } from '../../lib/dashboardCases.js'
import { formatNumber } from '../../lib/format.js'

export const VIEWS = [
  { id: 'attention', label: 'Needs attention', icon: AlertTriangle, hint: 'Failed, limited, waiting or missing, not yet reviewed' },
  { id: 'all', label: 'All activity', icon: Layers },
  { id: 'questions', label: 'Questions', icon: MessageSquareText },
  { id: 'evidence', label: 'Evidence', icon: FileText },
]

// The left rail: the views (the attention queue first, as an inbox's "unread") and the cases, each with how much is in it
// and how much still needs a look. Choosing a case narrows the journal to it; choosing it again clears the choice.
export function WorkspaceRail({ view, onView, counts, cases, caseFilter, onCase }) {
  return (
    <aside className="ac-rail" aria-label="Views and cases">
      <section aria-labelledby="ac-views">
        <h2 id="ac-views">Views</h2>
        <ul>
          {VIEWS.map(item => {
            const Icon = item.icon
            return (
              <li key={item.id}>
                <button type="button" aria-pressed={view === item.id} onClick={() => onView(item.id)} title={item.hint}>
                  <Icon aria-hidden="true" /><span>{item.label}</span><b className={item.id === 'attention' && counts[item.id] ? 'is-alert' : undefined}>{formatNumber(counts[item.id] || 0)}</b>
                </button>
              </li>
            )
          })}
        </ul>
      </section>
      <section aria-labelledby="ac-cases">
        <h2 id="ac-cases">Cases</h2>
        {cases.length ? (
          <ul>
            {cases.map(row => (
              <li key={row.caseId}>
                <button type="button" aria-pressed={caseFilter === row.caseId} onClick={() => onCase(caseFilter === row.caseId ? '' : row.caseId)}>
                  <Inbox aria-hidden="true" />
                  <span className="ac-rail__case"><bdi>{row.caseId}</bdi><small>{row.latest ? relativeAge(new Date(row.latest)) : 'No dated activity'}</small></span>
                  <b className={row.open ? 'is-alert' : undefined} title={`${formatNumber(row.open)} need a look of ${formatNumber(row.total)}`}>{formatNumber(row.total)}</b>
                </button>
              </li>
            ))}
          </ul>
        ) : <p className="ac-hint">No cases are configured.</p>}
      </section>
    </aside>
  )
}
