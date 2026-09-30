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
// purpose: it changes every minute and would be announced every minute. The clock time is in the tooltip.
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

// Short on purpose: the clock time lives in the tooltip and the live region. "No read yet" is either still in flight
// ("Reading…") or finished with nothing to show; those are different facts.
export function freshnessText({ lastUpdated, loading, now = new Date() }) {
  if (!lastUpdated) return loading ? updatedText(null) : 'No successful read yet'
  return `Updated ${ageText(lastUpdated, now)}${isStale(lastUpdated, now) ? ' · out of date' : ''}`
}

const plural = (count, one, many) => (count === 1 ? one : many)

// One short line answers the page's one question, "what needs me now?", from figures already on the page and
// nothing else. It never asserts a state the data does not show, and it says plainly when it cannot tell.
export function briefing(kpis, { loading }) {
  if (loading) return { tone: 'neutral', sentences: ['Reading status…'], action: null }
  if (!kpis.cases) return { tone: 'neutral', sentences: ['No cases yet'], action: null }
  if (!kpis.reported) return { tone: 'caution', sentences: ['Status unavailable'], action: null }
  const ready = kpis.sources ? `${formatNumber(kpis.ready)} of ${formatNumber(kpis.sources)} ready` : null
  let result
  if (kpis.review > 0) {
    result = { tone: 'attention', sentences: [`${formatNumber(kpis.review)} ${plural(kpis.review, 'source needs', 'sources need')} review`, ready], action: { label: 'Review', href: '#dashboard-attention' } }
  } else if (kpis.processing > 0) {
    result = { tone: 'processing', sentences: [`${formatNumber(kpis.processing)} ${plural(kpis.processing, 'source', 'sources')} processing`, ready], action: { label: 'Progress', href: '#dashboard-cases' } }
  } else if (!kpis.sources) {
    result = { tone: 'neutral', sentences: ['No evidence yet'], action: null }
  } else {
    result = { tone: 'ok', sentences: [`All ${formatNumber(kpis.sources)} ${plural(kpis.sources, 'source', 'sources')} ready`], action: null }
  }
  if (kpis.unreported > 0) result.sentences.push(`${formatNumber(kpis.reported)} of ${formatNumber(kpis.cases)} cases reporting`)
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

// D1. A briefing, not a label: one short line under the title says what needs the analyst, and freshness sits with
// Refresh because Refresh refreshes exactly what the timestamp describes. Rules: UI_REDESIGN_BRIEF §11.
export function DashboardHeader({ kpis, loading = false, lastUpdated, processing, refreshing, onRefresh }) {
  const now = useNow()
  const summary = briefing(kpis, { loading })
  const stale = isStale(lastUpdated, now)
  const clock = lastUpdated ? `Last read at ${updatedText(lastUpdated)}` : undefined
  return (
    <>
      <PageHeader
        title="Dashboard"
        description={(
          <span className={`dash-brief dash-brief--${summary.tone}`}>
            <span className="dash-brief__dot" aria-hidden="true" />
            <span className="dash-brief__lead">{summary.sentences[0]}</span>
            {summary.action ? <a className="dash-brief__action" href={summary.action.href}>{summary.action.label}</a> : null}
            {summary.sentences.slice(1).map(sentence => <span key={sentence} className="dash-brief__rest">{sentence}</span>)}
          </span>
        )}
        actions={(
          <div className="dash-header-actions">
            <span className={`dash-fresh${stale ? ' is-stale' : ''}`} title={clock}>
              {freshnessText({ lastUpdated, loading, now })}
              {processing ? <small>Refreshing every 15 s</small> : null}
            </span>
            <button type="button" className="dash-refresh-icon" onClick={onRefresh} disabled={refreshing} aria-label={refreshing ? 'Refreshing…' : 'Refresh'} title={refreshing ? 'Refreshing…' : 'Refresh'}><RefreshCw aria-hidden="true" className={refreshing ? 'is-spinning' : undefined} /></button>
            <Link className="page-header__cta" to="/cases/new"><Plus aria-hidden="true" />Add evidence</Link>
          </div>
        )}
      />
      <span className="visually-hidden" role="status">{refreshing ? 'Refreshing case status' : lastUpdated ? `Case status updated at ${updatedText(lastUpdated)}` : ''}</span>
    </>
  )
}
