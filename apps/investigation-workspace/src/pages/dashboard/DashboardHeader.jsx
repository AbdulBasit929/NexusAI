import { Link } from 'react-router-dom'
import { Plus, RefreshCw } from 'lucide-react'
import { PageHeader } from '../../components/PageHeader.jsx'
import { formatNumber } from '../../lib/format.js'

export function updatedText(date) {
  if (!date) return 'Reading…'
  return new Intl.DateTimeFormat('en-GB', { hour: '2-digit', minute: '2-digit' }).format(date)
}

export function headerFacts({ caseCount, lastUpdated, processing }) {
  const facts = [
    { label: 'Scope', value: `${formatNumber(caseCount)} ${caseCount === 1 ? 'case' : 'cases'} in this workspace` },
    { label: 'Updated', value: updatedText(lastUpdated) },
  ]
  // True only while the page really is polling (see the interval in DashboardPage).
  if (processing) facts.push({ label: 'Refresh', value: 'Every 15 s while evidence is processing' })
  return facts
}

// D1. Where am I and how fresh is this? Refresh lives here because it refreshes the whole page, not one section.
export function DashboardHeader({ caseCount, lastUpdated, processing, refreshing, onRefresh }) {
  return (
    <>
      <PageHeader
        title="Dashboard"
        description="See what needs review, whether your evidence is ready to search, and where to continue."
        meta={headerFacts({ caseCount, lastUpdated, processing })}
        actions={(
          <>
            <button type="button" className="page-header__secondary" onClick={onRefresh} disabled={refreshing}><RefreshCw aria-hidden="true" className={refreshing ? 'is-spinning' : undefined} />{refreshing ? 'Refreshing…' : 'Refresh'}</button>
            <Link className="page-header__cta" to="/cases/new"><Plus aria-hidden="true" />Add evidence</Link>
          </>
        )}
      />
      <span className="visually-hidden" role="status">{refreshing ? 'Refreshing case status' : lastUpdated ? `Case status updated at ${updatedText(lastUpdated)}` : ''}</span>
    </>
  )
}
