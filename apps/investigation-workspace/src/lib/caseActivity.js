import { formatNumber } from './format.js'
import { curatedFamilyLabel } from './semanticCatalog.js'

// Activity over time, from the deterministic `activity_by_day` template: whole-case, no target, unsampled, one row per
// day and record family. The service caps a template call at 100 rows and returns oldest first, so a full page may be
// partial and the widget says so. Everything here is a pure function of the response.
export const ACTIVITY_CAP = 100

function dayOf(value) {
  const match = /^(\d{4}-\d{2}-\d{2})/.exec(String(value ?? ''))
  return match ? match[1] : null
}

// The analyst-facing grid is a fallback source: its column keys are not guaranteed, so a row is only accepted when it
// holds a date, a family name and a count that can each be told apart by value. Anything else is left out.
function rowFromGrid(row) {
  if (!row || typeof row !== 'object') return null
  const entries = Object.entries(row)
  const date = entries.find(([, value]) => dayOf(value))?.[1]
  const count = entries.find(([key, value]) => /count|events|^m\d+$/i.test(key) && Number.isFinite(Number(value)))?.[1]
  const family = entries.find(([key, value]) => typeof value === 'string' && !dayOf(value) && /type|family|record/i.test(key))?.[1]
  return date && family && count !== undefined ? { activity_date: date, record_type: family, event_count: count } : null
}

function activityRowsOf(response) {
  const direct = response?.records?.activity_by_day
  if (Array.isArray(direct)) return direct
  const grid = response?.enterprise?.data_grid?.rows
  if (Array.isArray(grid) && grid.length) {
    const rows = grid.map(rowFromGrid)
    return rows.every(Boolean) ? rows : null
  }
  return null
}

// One case's response to `{ days, families, total, first, last, truncated }`, or `available: false` with a reason when
// the response has no activity rows to read (an unexpected shape is reported as unavailable, never as zero).
export function parseActivity(response, caseId) {
  const rows = activityRowsOf(response)
  if (!Array.isArray(rows)) return { available: false, caseId, reason: 'no_rows' }
  const days = new Map()
  const families = new Map()
  let total = 0
  for (const row of rows) {
    const date = dayOf(row.activity_date)
    const count = Number(row.event_count)
    const family = String(row.record_type || '').trim()
    if (!date || !family || !Number.isFinite(count) || count <= 0) continue
    const day = days.get(date) || { date, total: 0, byFamily: {}, byCase: {} }
    day.total += count
    day.byFamily[family] = (day.byFamily[family] || 0) + count
    day.byCase[caseId] = (day.byCase[caseId] || 0) + count
    days.set(date, day)
    families.set(family, (families.get(family) || 0) + count)
    total += count
  }
  const ordered = [...days.values()].sort((left, right) => left.date.localeCompare(right.date))
  return {
    available: true,
    caseId,
    days: ordered,
    families: [...families.entries()].map(([id, events]) => ({ id, total: events })).sort((left, right) => right.total - left.total || left.id.localeCompare(right.id)),
    total,
    first: ordered[0]?.date || null,
    last: ordered.at(-1)?.date || null,
    truncated: rows.length >= ACTIVITY_CAP,
  }
}

function topCaseOf(byCase) {
  return Object.entries(byCase).sort((left, right) => right[1] - left[1] || left[0].localeCompare(right[0]))[0]?.[0] || null
}

// Several cases to one series. Each day remembers which case holds most of its events, so a click can open that case.
export function mergeActivity(results) {
  const parsed = results.filter(result => result?.available)
  const days = new Map()
  const families = new Map()
  let total = 0
  for (const result of parsed) {
    for (const day of result.days) {
      const merged = days.get(day.date) || { date: day.date, total: 0, byFamily: {}, byCase: {} }
      merged.total += day.total
      for (const [family, count] of Object.entries(day.byFamily)) merged.byFamily[family] = (merged.byFamily[family] || 0) + count
      for (const [caseId, count] of Object.entries(day.byCase)) merged.byCase[caseId] = (merged.byCase[caseId] || 0) + count
      days.set(day.date, merged)
    }
    for (const family of result.families) families.set(family.id, (families.get(family.id) || 0) + family.total)
    total += result.total
  }
  const ordered = [...days.values()].sort((left, right) => left.date.localeCompare(right.date)).map(day => ({ ...day, topCase: topCaseOf(day.byCase) }))
  return {
    available: parsed.length > 0,
    cases: parsed.map(result => result.caseId),
    days: ordered,
    families: [...families.entries()].map(([id, events]) => ({ id, total: events })).sort((left, right) => right.total - left.total || left.id.localeCompare(right.id)),
    total,
    first: ordered[0]?.date || null,
    last: ordered.at(-1)?.date || null,
    truncated: parsed.some(result => result.truncated),
  }
}

// --- Time helpers. Dates are calendar days in UTC, exactly as the service reported them; no timezone shifting.
export function shift(date, days) {
  const moved = new Date(`${date}T00:00:00Z`)
  moved.setUTCDate(moved.getUTCDate() + days)
  return moved.toISOString().slice(0, 10)
}

export function mondayOf(date) {
  return shift(date, -((new Date(`${date}T00:00:00Z`).getUTCDay() + 6) % 7))
}

const SHORT = new Intl.DateTimeFormat('en-GB', { day: 'numeric', month: 'short', timeZone: 'UTC' })
const LONG = new Intl.DateTimeFormat('en-GB', { weekday: 'short', day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' })

export function shortDate(date) {
  return SHORT.format(new Date(`${date}T00:00:00Z`))
}

// A bucket in words: one day ("Sat, 28 Feb 2026") or a week ("23 Feb to 1 Mar 2026").
export function describeBucket(bucket) {
  if (!bucket.end || bucket.end === bucket.date) return LONG.format(new Date(`${bucket.date}T00:00:00Z`))
  return `${shortDate(bucket.date)} to ${LONG.format(new Date(`${bucket.end}T00:00:00Z`)).replace(/^\w+,? /, '')}`
}

// The question a click on a bucket asks in Investigate.
export function bucketQuestion(bucket) {
  return !bucket.end || bucket.end === bucket.date ? `Show activity on ${bucket.date}` : `Show activity from ${bucket.date} to ${bucket.end}`
}

// Days summed into calendar weeks (Monday to Sunday) when the span is long enough that daily bars turn into a fringe.
// Totals never change: every event is in exactly one bucket, and the last week ends on the last day read.
export function bucketActivity(activity, granularity) {
  if (granularity !== 'week' || !activity.days.length) return activity
  const weeks = new Map()
  for (const day of activity.days) {
    const start = mondayOf(day.date)
    const week = weeks.get(start) || { date: start, end: shift(start, 6) > activity.last ? activity.last : shift(start, 6), total: 0, byFamily: {}, byCase: {} }
    week.total += day.total
    for (const [family, count] of Object.entries(day.byFamily)) week.byFamily[family] = (week.byFamily[family] || 0) + count
    for (const [caseId, count] of Object.entries(day.byCase)) week.byCase[caseId] = (week.byCase[caseId] || 0) + count
    weeks.set(start, week)
  }
  return { ...activity, bucket: 'week', days: [...weeks.values()].sort((left, right) => left.date.localeCompare(right.date)).map(week => ({ ...week, topCase: topCaseOf(week.byCase) })) }
}

// Where a range preset starts, as an index into the buckets: the last N calendar days ending on the last day read.
export function rangeStartIndex(activity, range) {
  const count = Number(range)
  if (!count || !activity.days.length) return 0
  const cutoff = shift(activity.last, -(count - 1))
  const index = activity.days.findIndex(day => (day.end || day.date) >= cutoff)
  return index < 0 ? 0 : index
}

// Events per calendar day for each case over the last `span` days read (ending on the last day with activity), for the
// small trend beside each case. A case that was not read has no entry, so the table shows "not read" rather than a flat line.
export function caseSparks(activity, span = 30) {
  if (!activity.available || !activity.last) return {}
  const dates = Array.from({ length: span }, (_, index) => shift(activity.last, index - (span - 1)))
  const byDate = new Map(activity.days.map(day => [day.date, day]))
  return Object.fromEntries((activity.cases || []).map(caseId => {
    const values = dates.map(date => byDate.get(date)?.byCase[caseId] || 0)
    return [caseId, { values, total: values.reduce((sum, value) => sum + value, 0), from: dates[0], to: activity.last }]
  }))
}

// A record family keeps one colour on every widget: the index comes from the sorted list of every family in view.
export function familyOrder(...idLists) {
  return [...new Set(idLists.flat().filter(Boolean))].sort()
}

export function familyColour(id, order, palette) {
  const index = Math.max(0, order.indexOf(id))
  return palette[index % palette.length]
}

// Three facts derived from the days read: the busiest day, the most active record family and a typical day. They are
// computed from the same rows as the chart, so they are as complete as the chart is (and share its "partial" label).
export function activityHighlights(activity) {
  if (!activity.days.length || !activity.total) return null
  const busiest = activity.days.reduce((best, day) => (day.total > best.total ? day : best), activity.days[0])
  const top = activity.families[0]
  return {
    busiest: { date: busiest.date, total: busiest.total },
    topFamily: { id: top.id, total: top.total, share: Math.floor((top.total / activity.total) * 100) },
    typical: Math.round(activity.total / activity.days.length),
    activeDays: activity.days.length,
  }
}

export function activityRows(activity) {
  return activity.days.map(day => ({ key: day.date, date: describeBucket(day), total: day.total, ...Object.fromEntries(activity.families.map(family => [family.id, day.byFamily[family.id] || 0])) }))
}

// Stacked bars per bucket, one series per record family, a dashed line at the typical bucket, short date labels and a
// zoom slider when there are many buckets. `range` narrows the view to the last N days without changing any figure.
function legendRows(families) {
  const width = families.reduce((sum, family) => sum + 28 + curatedFamilyLabel(family.id).length * 7 + 16, 0)
  return Math.max(1, Math.ceil(width / 760))
}

export function activityOption(activity, theme, order, { range = 'all' } = {}) {
  const many = activity.days.length > 14
  const last = activity.days.length - 1
  const start = rangeStartIndex(activity, range)
  const typical = activity.days.length ? Math.round(activity.days.reduce((sum, day) => sum + day.total, 0) / activity.days.length) : 0
  const zoom = { startValue: start, endValue: last }
  return {
    aria: { enabled: false },
    animationDuration: 500,
    animationEasing: 'cubicOut',
    // The legend wraps onto more rows as families are added, so the plot starts below however many rows it needs (a row is
    // about 24px; an item is its label plus its swatch and gap, against roughly 760px of usable width).
    grid: { left: 8, right: typical ? 84 : 16, top: 14 + legendRows(activity.families) * 24, bottom: many ? 54 : 24, containLabel: true },
    legend: { top: 0, left: 0, icon: 'roundRect', itemWidth: 12, itemHeight: 12, itemGap: 16, textStyle: { color: theme.text, fontSize: 12 }, inactiveColor: theme.line },
    tooltip: {
      trigger: 'axis',
      confine: true,
      axisPointer: { type: 'shadow' },
      backgroundColor: theme.card,
      borderColor: theme.line,
      textStyle: { color: theme.text },
      formatter: points => {
        const bucket = activity.days[points[0]?.dataIndex]
        const lines = points.filter(point => point.value > 0).map(point => `${point.marker}${point.seriesName}: <strong>${formatNumber(point.value)}</strong>`)
        const sum = points.reduce((total, point) => total + (point.value || 0), 0)
        const cases = bucket && Object.keys(bucket.byCase).length > 1 && bucket.topCase ? `<br/><em>Most in ${bucket.topCase}</em>` : ''
        return `<strong>${bucket ? describeBucket(bucket) : points[0]?.axisValueLabel}</strong><br/>${lines.join('<br/>')}<br/>Total: <strong>${formatNumber(sum)}</strong>${cases}<br/><em>Select to open this ${bucket?.end && bucket.end !== bucket.date ? 'week' : 'day'}.</em>`
      },
    },
    xAxis: { type: 'category', data: activity.days.map(day => day.date), axisTick: { show: false }, axisLine: { lineStyle: { color: theme.line } }, axisLabel: { color: theme.muted, fontSize: 11, hideOverlap: true, formatter: value => shortDate(value) } },
    yAxis: { type: 'value', minInterval: 1, splitLine: { lineStyle: { color: theme.line, opacity: 0.6 } }, axisLabel: { color: theme.muted, fontSize: 11, formatter: value => formatNumber(value) } },
    dataZoom: many ? [{ type: 'inside', ...zoom }, { type: 'slider', ...zoom, height: 22, bottom: 6, borderColor: theme.line, textStyle: { color: theme.muted }, brushSelect: false, labelFormatter: (_, label) => (label ? shortDate(label) : '') }] : [],
    series: activity.families.map((family, index) => ({
      name: curatedFamilyLabel(family.id),
      type: 'bar',
      stack: 'activity',
      barMaxWidth: 32,
      emphasis: { focus: 'series' },
      itemStyle: { color: familyColour(family.id, order, theme.data), borderRadius: 0 },
      data: activity.days.map(day => day.byFamily[family.id] || 0),
      ...(index === 0 && typical ? { markLine: { silent: true, symbol: 'none', animation: false, lineStyle: { type: 'dashed', color: theme.muted, width: 1.25 }, label: { formatter: `Typical ${formatNumber(typical)}`, color: theme.muted, fontSize: 11, position: 'end' }, data: [{ yAxis: typical }] } } : {}),
    })),
  }
}
