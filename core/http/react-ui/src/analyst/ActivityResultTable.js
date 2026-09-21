import { createElement as h, useState } from 'react'
import { activityCell, activityTable } from './analystActivityPresentation.js'

// A historical malformed table is retained, but must not become invented facts.
export default function ActivityResultTable({ table, operation }) {
  const [sort, setSort] = useState({ column: '', descending: false })
  const [expanded, setExpanded] = useState(false)
  const view = activityTable(table, operation)
  if (table?.unavailable || view.unavailable) return h('p', { role: 'status' }, 'Result details are unavailable for this historical entry.')
  if (!view.rows.length) return null
  const rows = sort.column ? [...view.rows].sort((a, b) => {
    const left = a[sort.column], right = b[sort.column]
    const order = typeof left === 'number' && typeof right === 'number'
      ? left - right
      : activityCell(left).localeCompare(activityCell(right), undefined, { numeric: true })
    return sort.descending ? -order : order
  }) : view.rows
  const visible = rows.slice(0, expanded ? 100 : 8)
  return h('section', { className: 'analyst-activity-detail__section' },
    h('h3', null, view.title),
    h('p', { className: 'workspace-table-count' }, `Showing ${visible.length} of ${rows.length} returned rows`),
    h('div', { className: 'analyst-history-table', tabIndex: 0, role: 'region', 'aria-label': view.title || 'Investigation results' },
      h('table', null,
        h('caption', { className: 'sr-only' }, view.title),
        h('thead', null, h('tr', null, view.columns.map(column => h('th', { scope: 'col', key: column, 'aria-sort': sort.column === column ? sort.descending ? 'descending' : 'ascending' : 'none' }, h('button', { type: 'button', onClick: () => setSort({ column, descending: sort.column === column && !sort.descending }) }, column.replaceAll('_', ' '), ' ', h('i', { className: `fas ${sort.column === column ? sort.descending ? 'fa-sort-down' : 'fa-sort-up' : 'fa-sort'}`, 'aria-hidden': true })))))),
        h('tbody', null, visible.map((row, index) => h('tr', { key: index }, view.columns.map(column => h('td', { key: column, 'data-numeric': typeof row[column] === 'number' }, h('bdi', { dir: 'auto' }, activityCell(row[column]))))))))),
    rows.length > 8 && h('button', { type: 'button', className: 'analyst-secondary-action', 'aria-expanded': expanded, onClick: () => setExpanded(value => !value) }, expanded ? 'Show first 8 rows' : `Show up to ${Math.min(100, rows.length)} rows`))
}
