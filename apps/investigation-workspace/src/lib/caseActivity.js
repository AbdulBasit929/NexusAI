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
  const ordered = [...days.values()].sort((left, right) => left.date.localeCompare(right.date)).map(day => ({ ...day, topCase: Object.entries(day.byCase).sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))[0]?.[0] || null }))
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
  return activity.days.map(day => ({ key: day.date, date: day.date, total: day.total, ...Object.fromEntries(activity.families.map(family => [family.id, day.byFamily[family.id] || 0])) }))
}

// Stacked bars per day, one series per record family. The zoom slider appears when there are many days and opens on
// the most recent stretch. Only the last day's tooltip lines that hold a count are shown.
export function activityOption(activity, theme, order) {
  const many = activity.days.length > 45
  const start = many ? Math.max(0, 100 - Math.round((60 / activity.days.length) * 100)) : 0
  return {
    aria: { enabled: false },
    animationDuration: 500,
    animationEasing: 'cubicOut',
    grid: { left: 8, right: 12, top: 36, bottom: many ? 56 : 24, containLabel: true },
    legend: { top: 0, left: 0, icon: 'roundRect', itemWidth: 12, itemHeight: 12, textStyle: { color: theme.text, fontSize: 12 }, inactiveColor: theme.line },
    tooltip: {
      trigger: 'axis',
      confine: true,
      axisPointer: { type: 'shadow' },
      backgroundColor: theme.card,
      borderColor: theme.line,
      textStyle: { color: theme.text },
      formatter: points => {
        const lines = points.filter(point => point.value > 0).map(point => `${point.marker}${point.seriesName}: <strong>${formatNumber(point.value)}</strong>`)
        const day = activity.days[points[0]?.dataIndex]
        const sum = points.reduce((total, point) => total + (point.value || 0), 0)
        return `<strong>${points[0]?.axisValueLabel}</strong><br/>${lines.join('<br/>')}<br/>Total: <strong>${formatNumber(sum)}</strong>${day?.topCase && Object.keys(day.byCase).length > 1 ? `<br/><em>Most in ${day.topCase}</em>` : ''}<br/><em>Select to open this day.</em>`
      },
    },
    xAxis: { type: 'category', data: activity.days.map(day => day.date), axisTick: { show: false }, axisLine: { lineStyle: { color: theme.line } }, axisLabel: { color: theme.muted, fontSize: 11, hideOverlap: true } },
    yAxis: { type: 'value', minInterval: 1, splitLine: { lineStyle: { color: theme.line, opacity: 0.6 } }, axisLabel: { color: theme.muted, fontSize: 11, formatter: value => formatNumber(value) } },
    dataZoom: many ? [{ type: 'inside', start, end: 100 }, { type: 'slider', start, end: 100, height: 22, bottom: 6, borderColor: theme.line, textStyle: { color: theme.muted }, brushSelect: false }] : [],
    series: activity.families.map(family => ({
      name: curatedFamilyLabel(family.id),
      type: 'bar',
      stack: 'activity',
      barMaxWidth: 28,
      emphasis: { focus: 'series' },
      itemStyle: { color: familyColour(family.id, order, theme.data), borderRadius: 0 },
      data: activity.days.map(day => day.byFamily[family.id] || 0),
    })),
  }
}
