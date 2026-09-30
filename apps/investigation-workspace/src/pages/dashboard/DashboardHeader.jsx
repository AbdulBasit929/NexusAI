import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Plus, RefreshCw } from 'lucide-react'
import { PageHeader } from '../../components/PageHeader.jsx'
import { formatNumber } from '../../lib/format.js'

const STALE_AFTER_MS = 5 * 60 * 1000

export function updatedText(date) {
  if (!date) return 'Reading…'
  return new Intl.DateTimeFormat('en-GB', { hour: '2-digit', minute: '2-digit' }).format(date)
}

// How old the figures are, in words, floored so the age is never understated. It stays out of the live region on
// purpose: it changes every minute and would be announced every minute. The clock time sits beside it.
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

// "No read yet" is either still in flight ("Reading…") or finished with nothing to show; those are different facts.
export function freshnessText({ lastUpdated, loading, now = new Date() }) {
  if (!lastUpdated) return loading ? updatedText(null) : 'No successful read yet'
  return `Updated ${updatedText(lastUpdated)} · ${ageText(lastUpdated, now)}${isStale(lastUpdated, now) ? ', may be out of date' : ''}`
}

const plural = (count, one, many) => (count === 1 ? one : many)

// The headline answers the page's one question, "what needs me now?", from figures already on the page and nothing
// else: it never asserts a state the data does not show, and it says plainly when it cannot tell.
export function briefing(kpis, { loading }) {
  if (loading) return { tone: 'neutral', sentences: ['Reading case status…'], action: null }
  if (!kpis.cases) return { tone: 'neutral', sentences: ['No cases are configured yet.'], action: null }
  if (!kpis.reported) return { tone: 'caution', sentences: ['Case status could not be read, so no figures are shown.'], action: null }
  const ready = kpis.sources ? `${formatNumber(kpis.ready)} of ${formatNumber(kpis.sources)} ${plural(kpis.sources, 'is', 'are')} ready to search.` : null
  let result
  if (kpis.review > 0) {
    const where = kpis.reviewCases ? ` in ${formatNumber(kpis.reviewCases)} of ${formatNumber(kpis.reported)} ${plural(kpis.reported, 'case', 'cases')}` : ''
    result = { tone: 'attention', sentences: [`${formatNumber(kpis.review)} ${plural(kpis.review, 'source needs', 'sources need')} review${where}.`, ready], action: { label: 'Review them', href: '#dashboard-attention' } }
  } else if (kpis.processing > 0) {
    result = { tone: 'processing', sentences: [`${formatNumber(kpis.processing)} ${plural(kpis.processing, 'source is', 'sources are')} still processing.`, ready], action: { label: 'See progress', href: '#dashboard-readiness' } }
  } else if (!kpis.sources) {
    result = { tone: 'neutral', sentences: ['No evidence has been added yet.'], action: null }
  } else {
    result = { tone: 'ok', sentences: [`All ${formatNumber(kpis.sources)} ${plural(kpis.sources, 'source is', 'sources are')} ready to search.`], action: null }
  }
  if (kpis.unreported > 0) result.sentences.push(`Showing ${formatNumber(kpis.reported)} of ${formatNumber(kpis.cases)} cases; ${formatNumber(kpis.unreported)} could not be read.`)
  return { ...result, sentences: result.sentences.filter(Boolean) }
}

function useNow(intervalMs = 30_000) {
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    const timer = globalThis.setInterval(() => setNow(new Date()), intervalMs)
    return () => globalThis.clearInterval(timer)
  }, [intervalMs])
  return now
}

// D1. A briefing, not a label: the sentence under the title says what needs the analyst, and freshness and Refresh
// sit together because Refresh refreshes exactly what the timestamp describes.
export function DashboardHeader({ kpis, loading = false, lastUpdated, processing, refreshing, onRefresh }) {
  const now = useNow()
  const summary = briefing(kpis, { loading })
  const stale = isStale(lastUpdated, now)
  return (
    <>
      <PageHeader
        title="Dashboard"
        description={(
          <span className={`dash-brief dash-brief--${summary.tone}`}>
            {/* The action sits beside the problem it resolves, before the secondary sentence. */}
            <span className="dash-brief__lead">{summary.sentences[0]} </span>
            {summary.action ? <><a className="dash-brief__action" href={summary.action.href}>{summary.action.label}</a>{' '}</> : null}
            {summary.sentences.slice(1).map(sentence => <span key={sentence} className="dash-brief__rest">{sentence} </span>)}
          </span>
        )}
        actions={(
          <div className="dash-header-actions">
            <div className={`dash-fresh${stale ? ' is-stale' : ''}`}>
              <span>{freshnessText({ lastUpdated, loading, now })}</span>
              {processing ? <small>Refreshes every 15 s while evidence is processing</small> : null}
            </div>
            <button type="button" className="dash-refresh-icon" onClick={onRefresh} disabled={refreshing} aria-label={refreshing ? 'Refreshing…' : 'Refresh'} title={refreshing ? 'Refreshing…' : 'Refresh'}><RefreshCw aria-hidden="true" className={refreshing ? 'is-spinning' : undefined} /></button>
            <Link className="page-header__cta" to="/cases/new"><Plus aria-hidden="true" />Add evidence</Link>
          </div>
        )}
      />
      <span className="visually-hidden" role="status">{refreshing ? 'Refreshing case status' : lastUpdated ? `Case status updated at ${updatedText(lastUpdated)}` : ''}</span>
    </>
  )
}
