import { useEffect, useId, useMemo, useRef, useState } from 'react'
import { LanguageText } from './AnalystComponents.jsx'

function numeric(value) {
  return typeof value === 'number' || /^[-+]?\d[\d,]*(?:\.\d+)?(?:%|\s[A-Z]{3})?$/.test(String(value ?? '').trim())
}

function sortable(value) {
  if (numeric(value)) return Number(String(value).replaceAll(',', '').replace(/[^\d.+-]/g, ''))
  return String(value ?? '').toLocaleLowerCase()
}

function csvValue(value) {
  const text = typeof value === 'object' && value !== null ? JSON.stringify(value) : String(value ?? '')
  return `"${text.replaceAll('"', '""')}"`
}

export function VirtualizedTable({
  columns,
  rows,
  label,
  initialRow,
  rowNumberKey = 'row_number',
  renderCell,
  exportName = 'nexusai-result.csv',
  totals,
  onRowActivate,
  onRowSelect,
  height,
}) {
  const tableId = useId().replaceAll(':', '')
  const viewport = useRef(null)
  const [filter, setFilter] = useState('')
  const [sort, setSort] = useState(null)
  const [density, setDensity] = useState('compact')
  const [scrollTop, setScrollTop] = useState(0)
  const [jump, setJump] = useState(initialRow == null ? '' : String(initialRow))
  const rowHeight = density === 'compact' ? 44 : 56
  const viewportHeight = height ?? 420
  const [selected, setSelected] = useState(null)
  const filtered = useMemo(() => {
    const needle = filter.trim().toLocaleLowerCase()
    const source = needle ? rows.filter(row => columns.some(column => String(row?.[column.key] ?? '').toLocaleLowerCase().includes(needle))) : rows
    if (!sort) return source
    return source.map((row, index) => ({ row, index })).sort((left, right) => {
      const a = sortable(left.row?.[sort.key]); const b = sortable(right.row?.[sort.key])
      const compared = a < b ? -1 : a > b ? 1 : left.index - right.index
      return sort.direction === 'asc' ? compared : -compared
    }).map(item => item.row)
  }, [columns, filter, rows, sort])
  const visibleCount = Math.ceil(viewportHeight / rowHeight) + 10
  const start = Math.max(0, Math.floor(scrollTop / rowHeight) - 5)
  const end = Math.min(filtered.length, start + visibleCount)
  const visible = filtered.slice(start, end)
  function select(row) {
    setSelected(row)
    onRowSelect?.(row)
  }
  // Arrow keys move the selection through the rows on screen, so a keyboard user can inspect a record without a pointer.
  function moveSelection(event) {
    if (!onRowSelect || !['ArrowDown', 'ArrowUp'].includes(event.key) || !filtered.length) return
    event.preventDefault()
    const current = selected ? filtered.indexOf(selected) : -1
    const next = Math.min(filtered.length - 1, Math.max(0, current + (event.key === 'ArrowDown' ? 1 : -1)))
    select(filtered[next])
    const top = next * rowHeight
    const node = viewport.current
    if (node && (top < node.scrollTop || top + rowHeight > node.scrollTop + node.clientHeight)) node.scrollTop = Math.max(0, top - rowHeight)
  }

  useEffect(() => {
    if (initialRow == null || !viewport.current) return
    const index = filtered.findIndex(row => String(row?.[rowNumberKey] ?? row?.source_row) === String(initialRow))
    if (index >= 0) {
      const target = index * rowHeight
      viewport.current.scrollTop = target
      // Programmatic scroll events are scheduled differently by browsers.
      // Drive the render window directly so a deep-linked row is never a
      // race between the first paint and the scroll event.
      setScrollTop(target)
    }
  }, [filtered, initialRow, rowHeight, rowNumberKey])

  function changeSort(key) {
    setSort(current => current?.key === key
      ? { key, direction: current.direction === 'asc' ? 'desc' : 'asc' }
      : { key, direction: 'asc' })
  }

  function jumpToRow(event) {
    event.preventDefault()
    const index = filtered.findIndex(row => String(row?.[rowNumberKey] ?? row?.source_row) === jump.trim())
    if (index >= 0 && viewport.current) {
      viewport.current.scrollTop = index * rowHeight
      viewport.current.focus()
    }
  }

  function exportRows() {
    const locatorColumn = rows.some(row => row?.__citationLocator) ? ['Citation locator'] : []
    const header = [...columns.map(column => column.label), ...locatorColumn].map(csvValue).join(',')
    const body = filtered.map(row => [...columns.map(column => row?.[column.key]), ...(locatorColumn.length ? [row.__citationLocator || ''] : [])].map(csvValue).join(','))
    const blob = new Blob([[header, ...body].join('\r\n')], { type: 'text/csv;charset=utf-8' })
    const href = URL.createObjectURL(blob)
    const anchor = globalThis.document.createElement('a')
    anchor.href = href; anchor.download = exportName; anchor.click()
    URL.revokeObjectURL(href)
  }

  return <section className={`data-table data-table--${density}`} aria-label={`${label} controls`}>
    <div className="data-table__toolbar">
      <label><span>Filter rows</span><input type="search" value={filter} onChange={event => { setFilter(event.target.value); setScrollTop(0) }} /></label>
      <div className="density-toggle" role="group" aria-label="Table density"><button type="button" aria-pressed={density === 'compact'} onClick={() => setDensity('compact')}>Compact</button><button type="button" aria-pressed={density === 'comfortable'} onClick={() => setDensity('comfortable')}>Comfortable</button></div>
      {rows.some(row => row?.[rowNumberKey] != null || row?.source_row != null) ? <form onSubmit={jumpToRow}><label><span>Jump to row</span><input inputMode="numeric" value={jump} onChange={event => setJump(event.target.value)} /></label><button type="submit">Jump</button></form> : null}
      <button type="button" onClick={exportRows}>Export current result</button>
    </div>
    <p className="data-table__summary" aria-live="polite">{filtered.length.toLocaleString()} of {rows.length.toLocaleString()} loaded rows</p>
    <div ref={viewport} className="table-shell table-shell--virtual" tabIndex="0" role="region" aria-label={label} onScroll={event => setScrollTop(event.currentTarget.scrollTop)} onKeyDown={moveSelection} style={height ? { maxBlockSize: `${height}px` } : undefined}>
      <table aria-rowcount={filtered.length + 1}>
        <thead><tr>{columns.map(column => <th key={column.key} id={`${tableId}-${column.key}`} scope="col" title={column.description || undefined}><button type="button" onClick={() => changeSort(column.key)} aria-label={`Sort by ${column.label}${sort?.key === column.key ? `, currently ${sort.direction}` : ''}`}>{column.label}<span aria-hidden="true">{sort?.key === column.key ? sort.direction === 'asc' ? ' ↑' : ' ↓' : ''}</span></button></th>)}</tr></thead>
        <tbody>
          {start > 0 ? <tr className="virtual-spacer" aria-hidden="true"><td colSpan={columns.length} style={{ blockSize: `${start * rowHeight}px` }} /></tr> : null}
          {visible.map((row, visibleIndex) => {
            const rowIndex = start + visibleIndex
            return <tr key={row?.id || row?.[rowNumberKey] || rowIndex} aria-rowindex={rowIndex + 2} aria-selected={onRowSelect ? row === selected : undefined} className={row === selected ? 'is-selected' : undefined} style={{ blockSize: `${rowHeight}px` }} onClick={() => onRowSelect && select(row)} onDoubleClick={() => onRowActivate?.(row)}>{columns.map(column => {
              const value = row?.[column.key]
              return <td key={column.key} headers={`${tableId}-${column.key}`} aria-label={String(value == null || value === '' ? '—' : value)} className={numeric(value) ? 'table-cell--numeric' : undefined} dir="auto">{renderCell ? renderCell(row, column, value, rowIndex) : <LanguageText isolate={!numeric(value)}>{value == null || value === '' ? '—' : value}</LanguageText>}</td>
            })}</tr>
          })}
          {end < filtered.length ? <tr className="virtual-spacer" aria-hidden="true"><td colSpan={columns.length} style={{ blockSize: `${(filtered.length - end) * rowHeight}px` }} /></tr> : null}
        </tbody>
        {totals ? <tfoot><tr>{columns.map(column => <td key={column.key} headers={`${tableId}-${column.key}`}>{totals[column.key] ?? (column === columns[0] ? 'Total' : '')}</td>)}</tr></tfoot> : null}
      </table>
    </div>
  </section>
}
