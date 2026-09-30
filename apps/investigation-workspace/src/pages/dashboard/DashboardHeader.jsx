import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Plus, RefreshCw } from 'lucide-react'
import { PageHeader } from '../../components/PageHeader.jsx'
import { formatNumber } from '../../lib/format.js'

export function updatedText(date) {
  if (!date) return 'Reading…'
  return new Intl.DateTimeFormat('en-GB', { hour: '2-digit', minute: '2-digit' }).format(date)
}

const STALE_AFTER_MS = 5 * 60 * 1000

// How old the figures are, in words. Kept out of the live region on purpose: it changes every minute and would
// be announced every minute. The absolute time stays alongside it so the age never has to be trusted alone.
export function ageText(date, now = new Date()) {
  if (!date) return ''
  const seconds = Math.max(0, Math.floor((now.getTime() - date.getTime()) / 1000))
  if (seconds < 45) return 'just now'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes} min ago`
  const hours = Math.floor(minutes / 60)
  return hours < 24 ? `${hours} h ago` : 'over a day ago'
}

// Polling keeps figures fresh while evidence is processing, so anything older than this means reads are failing or
// the tab was idle. Stale is its own state, stated in words, not only in colour.
export function isStale(date, now = new Date()) {
  return Boolean(date) && now.getTime() - date.getTime() > STALE_AFTER_MS
}

export function headerFacts({ caseCount, reporting = caseCount, lastUpdated, processing, loading = false, now = new Date() }) {
  const cases = `${formatNumber(caseCount)} ${caseCount === 1 ? 'case' : 'cases'}`
  // Only claim "N of M reporting" once a read has finished: while the first read is in flight nothing has reported
  // yet and that is waiting, not a fault.
  const partial = !loading && reporting < caseCount
  const stale = isStale(lastUpdated, now)
  // No read yet is either still in flight ("Reading…") or has finished with nothing to show; those are different facts.
  const updated = lastUpdated ? `${updatedText(lastUpdated)} · ${ageText(lastUpdated, now)}${stale ? ', may be out of date' : ''}` : loading ? updatedText(null) : 'No successful read yet'
  const facts = [
    { label: 'Scope', value: partial ? `${formatNumber(reporting)} of ${cases} reporting` : `${cases} in this workspace` },
    { label: 'Updated', value: updated, stale },
  ]
  // True only while the page really is polling (see the interval in DashboardPage).
  if (processing) facts.push({ label: 'Refresh', value: 'Every 15 s while evidence is processing' })
  return facts
}

function useNow(intervalMs = 30_000) {
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    const timer = globalThis.setInterval(() => setNow(new Date()), intervalMs)
    return () => globalThis.clearInterval(timer)
  }, [intervalMs])
  return now
}

// D1. Where am I and how fresh is this? Refresh lives here because it refreshes the whole page, not one section.
export function DashboardHeader({ caseCount, reporting, loading = false, lastUpdated, processing, refreshing, onRefresh }) {
  const now = useNow()
  return (
    <>
      <PageHeader
        title="Dashboard"
        description="See what needs review, whether your evidence is ready to search, and where to continue."
        meta={headerFacts({ caseCount, reporting, loading, lastUpdated, processing, now })}
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
