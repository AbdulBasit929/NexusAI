import { formatNumber } from '../lib/format.js'

function safeNumber(value) {
  const number = Number(value)
  return Number.isFinite(number) && number > 0 ? number : 0
}

function segmentsWithGeometry(values) {
  const normalized = values.map(item => ({ ...item, value: safeNumber(item.value) }))
  const total = normalized.reduce((sum, item) => sum + item.value, 0)
  let offset = 0
  return {
    total,
    segments: normalized.map(item => {
      const width = total ? (item.value / total) * 100 : 0
      const segment = { ...item, offset, width }
      offset += width
      return segment
    }),
  }
}

function SegmentedBar({ values, label, className = '' }) {
  const { total, segments } = segmentsWithGeometry(values)
  const description = segments.map(item => `${item.label} ${formatNumber(item.value)}`).join(', ')
  return (
    <span className={`data-bar ${className}`}>
      <svg viewBox="0 0 100 8" preserveAspectRatio="none" role="img" aria-label={`${label}: ${description}`}>
        <rect className="data-bar__track" x="0" y="0" width="100" height="8" rx="4" />
        {total ? segments.filter(item => item.width > 0).map((item, index) => (
          <rect key={item.id} className={`data-bar__segment data-bar__segment--${item.tone || ((index % 6) + 1)}`} x={item.offset} y="0" width={item.width} height="8" />
        )) : null}
      </svg>
    </span>
  )
}

export function ProportionBar({ ready = 0, processing = 0, failed = 0, total = 0, label = 'Evidence readiness' }) {
  const accounted = safeNumber(ready) + safeNumber(processing) + safeNumber(failed)
  const excluded = Math.max(0, safeNumber(total) - accounted)
  return <SegmentedBar className="proportion-bar" label={label} values={[
    { id: 'ready', label: 'ready', value: ready, tone: 'ready' },
    { id: 'processing', label: 'processing', value: processing, tone: 'processing' },
    { id: 'failed', label: 'failed', value: failed, tone: 'failed' },
    { id: 'excluded', label: 'other', value: excluded, tone: 'excluded' },
  ]} />
}

export function ReadinessDonut({ ready = 0, processing = 0, failed = 0, total = 0, label = 'Evidence readiness' }) {
  const accounted = safeNumber(ready) + safeNumber(processing) + safeNumber(failed)
  const reportedTotal = Math.max(safeNumber(total), accounted)
  const { segments } = segmentsWithGeometry([
    { id: 'ready', label: 'ready', value: ready, tone: 'ready' },
    { id: 'processing', label: 'processing', value: processing, tone: 'processing' },
    { id: 'failed', label: 'failed', value: failed, tone: 'failed' },
    { id: 'excluded', label: 'other', value: Math.max(0, reportedTotal - accounted), tone: 'excluded' },
  ])
  const description = segments.map(item => `${item.label} ${formatNumber(item.value)}`).join(', ')
  return (
    <span className="readiness-donut">
      <svg viewBox="0 0 42 42" role="img" aria-label={`${label}: ${description}`}>
        <circle className="readiness-donut__track" cx="21" cy="21" r="15.9155" />
        {reportedTotal ? segments.filter(item => item.width > 0).map(item => (
          <circle key={item.id} className={`readiness-donut__segment readiness-donut__segment--${item.tone}`} cx="21" cy="21" r="15.9155" pathLength="100" strokeDasharray={`${item.width} ${100 - item.width}`} strokeDashoffset={-item.offset} />
        )) : null}
      </svg>
      <span className="readiness-donut__value" aria-hidden="true"><strong>{formatNumber(reportedTotal)}</strong><small>sources</small></span>
    </span>
  )
}

export function StackedBar({ items = [], label = 'Accepted rows by evidence family' }) {
  return <SegmentedBar className="stacked-bar" label={label} values={items.map((item, index) => ({ id: item.id, label: item.label, value: item.value, tone: (index % 6) + 1 }))} />
}

export function MeterBar({ accepted = 0, duplicate = 0, rejected = 0, label = 'Ingestion accounting' }) {
  return <SegmentedBar className="meter-bar" label={label} values={[
    { id: 'accepted', label: 'accepted', value: accepted, tone: 'accepted' },
    { id: 'duplicate', label: 'duplicate', value: duplicate, tone: 'duplicate' },
    { id: 'rejected', label: 'rejected', value: rejected, tone: 'rejected' },
  ]} />
}
