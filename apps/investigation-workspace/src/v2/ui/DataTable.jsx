import { useId, useMemo, useState } from 'react'
import { ArrowDown, ArrowUp, Check, Copy, Download, Search } from 'lucide-react'

// The working surface of the product: a table that shows every column it was given, the whole width available, with the count of rows said
// out loud ("Showing 1 to 50 of 408"). It sorts, filters, copies and exports what is on screen. It never hides rows silently: what is not
// shown yet is a button. Cells are text; the caller may render a cell (for a link to a source row).

const NUMBER = /^[-+]?(?:\d{1,3}(?:,\d{3})+|\d+)(?:\.\d+)?%?$/
const LONG_DIGITS = /^\+?\d{9,}$/
const IDENTIFIER_KEY = /(^|_)(id|number|msisdn|imei|imsi|iccid|account|plate|phone|hash|ip|uri|url|file|source|row|name)($|_)/i

// Numbers are right-aligned and sorted as numbers. A phone number, an IMEI or an account is a long string of digits: it is an
// identifier, left-aligned and sorted as text, so it is never mistaken for a quantity.
export function columnKind(column, rows) {
  const values = rows.map(row => row?.[column.key]).filter(value => value != null && value !== '')
  if (!values.length || IDENTIFIER_KEY.test(String(column.key))) return 'text'
  const texts = values.map(value => String(value).trim())
  if (texts.some(text => LONG_DIGITS.test(text.replace(/[\s-]/g, '')))) return 'text'
  const numeric = values.filter(value => typeof value === 'number' || NUMBER.test(String(value).trim()))
  return numeric.length / values.length >= 0.8 ? 'number' : 'text'
}

const toNumber = value => Number(String(value ?? '').replaceAll(',', '').replace('%', ''))
const cellText = value => (value == null || value === '' ? '' : typeof value === 'object' ? JSON.stringify(value) : String(value))
const csvCell = value => `"${cellText(value).replaceAll('"', '""')}"`

function toCsv(columns, rows) {
  return [columns.map(column => csvCell(column.label)).join(','), ...rows.map(row => columns.map(column => csvCell(row?.[column.key])).join(','))].join('\r\n')
}

export function DataTable({ columns, rows, label, caption, renderCell, totals, initialLimit = 50, step = 100, maxHeight, exportName = 'nexusai-result.csv', note }) {
  const id = useId()
  const [filter, setFilter] = useState('')
  const [sort, setSort] = useState(null)
  const [limit, setLimit] = useState(initialLimit)
  const [copied, setCopied] = useState(false)
  const kinds = useMemo(() => Object.fromEntries(columns.map(column => [column.key, columnKind(column, rows)])), [columns, rows])

  const shown = useMemo(() => {
    const needle = filter.trim().toLocaleLowerCase()
    const matching = needle ? rows.filter(row => columns.some(column => cellText(row?.[column.key]).toLocaleLowerCase().includes(needle))) : rows
    if (!sort) return matching
    const direction = sort.direction === 'asc' ? 1 : -1
    return matching.map((row, index) => ({ row, index })).sort((left, right) => {
      const a = left.row?.[sort.key]; const b = right.row?.[sort.key]
      const compared = kinds[sort.key] === 'number' ? toNumber(a) - toNumber(b) : cellText(a).localeCompare(cellText(b), undefined, { numeric: true, sensitivity: 'base' })
      return (compared || left.index - right.index) * (compared ? direction : 1)
    }).map(item => item.row)
  }, [columns, filter, kinds, rows, sort])

  const visible = shown.slice(0, limit)
  const cycle = key => setSort(current => (current?.key !== key ? { key, direction: 'asc' } : current.direction === 'asc' ? { key, direction: 'desc' } : null))

  async function copyCsv() {
    try { await globalThis.navigator.clipboard.writeText(toCsv(columns, shown)); setCopied(true); globalThis.setTimeout(() => setCopied(false), 1600) } catch { setCopied(false) }
  }
  function download() {
    const blob = new Blob([toCsv(columns, shown)], { type: 'text/csv;charset=utf-8' })
    const href = URL.createObjectURL(blob)
    const anchor = globalThis.document.createElement('a')
    anchor.href = href; anchor.download = exportName; anchor.click()
    URL.revokeObjectURL(href)
  }

  const counted = filter ? `${shown.length.toLocaleString()} of ${rows.length.toLocaleString()} rows match` : `${rows.length.toLocaleString()} ${rows.length === 1 ? 'row' : 'rows'}`
  return (
    <div className="nx-datatable">
      {rows.length > 10 || filter ? (
        <div className="nx-datatable__bar">
          <label className="nx-datatable__filter"><Search aria-hidden="true" /><span className="nx-visually-hidden">Filter the rows of this table</span>
            <input type="search" className="nx-input" placeholder="Filter these rows" value={filter} onChange={event => { setFilter(event.target.value); setLimit(initialLimit) }} />
          </label>
        </div>
      ) : null}
      <div className="nx-table-wrap" role="region" tabIndex={0} aria-label={label} style={maxHeight ? { '--nx-table-max': `${maxHeight}px` } : undefined}>
        <table className="nx-table" aria-rowcount={shown.length + 1}>
          {caption ? <caption className="nx-visually-hidden">{caption}</caption> : null}
          <thead>
            <tr>
              {columns.map(column => {
                const active = sort?.key === column.key
                return (
                  <th key={column.key} id={`${id}-${column.key}`} scope="col" className={kinds[column.key] === 'number' ? 'is-num' : undefined} aria-sort={active ? (sort.direction === 'asc' ? 'ascending' : 'descending') : 'none'} title={column.description || undefined}>
                    <button type="button" className="nx-th" onClick={() => cycle(column.key)}>
                      {column.label}
                      <span className="nx-th__sort" aria-hidden="true">{active ? (sort.direction === 'asc' ? <ArrowUp /> : <ArrowDown />) : null}</span>
                    </button>
                  </th>
                )
              })}
            </tr>
          </thead>
          <tbody>
            {visible.map((row, rowIndex) => (
              <tr key={row?.id || row?.row_number || rowIndex} aria-rowindex={rowIndex + 2}>
                {columns.map(column => {
                  const value = row?.[column.key]
                  const empty = value == null || value === ''
                  return (
                    <td key={column.key} headers={`${id}-${column.key}`} className={`${kinds[column.key] === 'number' ? 'is-num' : ''}${IDENTIFIER_KEY.test(String(column.key)) && !empty ? ' is-id' : ''}`.trim() || undefined} dir={kinds[column.key] === 'number' ? 'ltr' : 'auto'}>
                      {empty ? <span className="nx-muted" aria-label="no value">—</span> : renderCell ? renderCell(row, column, value, rowIndex) : cellText(value)}
                    </td>
                  )
                })}
              </tr>
            ))}
          </tbody>
          {totals ? <tfoot><tr>{columns.map((column, index) => <td key={column.key} className={kinds[column.key] === 'number' ? 'is-num' : undefined}>{totals[column.key] ?? (index === 0 ? 'Total' : '')}</td>)}</tr></tfoot> : null}
        </table>
      </div>
      <div className="nx-table-foot">
        <span role="status" aria-live="polite">
          {visible.length < shown.length ? `Showing 1 to ${visible.length.toLocaleString()} of ${shown.length.toLocaleString()}` : counted}{filter && visible.length >= shown.length ? '' : filter ? ` (${counted})` : ''}
          {note ? <span className="nx-muted"> · {note}</span> : null}
        </span>
        <span className="nx-datatable__actions">
          {visible.length < shown.length ? (
            <>
              <button type="button" className="nx-btn nx-btn--sm" onClick={() => setLimit(value => value + step)}>Show {Math.min(step, shown.length - visible.length).toLocaleString()} more</button>
              <button type="button" className="nx-btn nx-btn--sm nx-btn--ghost" onClick={() => setLimit(shown.length)}>Show all {shown.length.toLocaleString()}</button>
            </>
          ) : null}
          <button type="button" className="nx-btn nx-btn--sm nx-btn--ghost" onClick={copyCsv}>{copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}{copied ? 'Copied' : 'Copy as CSV'}</button>
          <button type="button" className="nx-btn nx-btn--sm nx-btn--ghost" onClick={download}><Download aria-hidden="true" />Export</button>
        </span>
      </div>
    </div>
  )
}
