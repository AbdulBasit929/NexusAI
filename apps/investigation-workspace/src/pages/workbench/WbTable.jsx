import { useMemo, useState } from 'react'
import { ArrowDown, ArrowUp, Copy, Download } from 'lucide-react'

// The table of the workbench (NN/g, "Data tables: four major user tasks"; Primer and Carbon on density): a human-readable first column, sortable columns, numbers aligned to the right in tabular figures,
// a header that stays in view, a row that is easy to follow, and the rows one click from the clipboard or a file. Used for the table twin and for the rows of the evidence panel. A row can be a way
// into the workbench (onRow): it is then a button, reachable and operable by keyboard.
const NUMERIC = /^-?\d+(?:[.,]\d+)*(?:\s?(?:min|km|s|km\/h|%))?$/
const isNumeric = (rows, index) => rows.length > 0 && rows.every(row => NUMERIC.test(String(row[index] ?? '').trim()))
const keyOf = value => { const n = Number(String(value).replace(/[^\d.-]/g, '')); return Number.isFinite(n) && NUMERIC.test(String(value).trim()) ? n : String(value) }
const csvCell = value => { const text = String(value ?? ''); return /[",\n]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text }

export function WbTable({ columns, rows, pick = null, label, dense = true, onCopied, onRow, maxHeight, fileName = '' }) {
  const [sort, setSort] = useState(null)
  const numeric = useMemo(() => columns.map((_, i) => isNumeric(rows, i)), [columns, rows])
  const view = useMemo(() => {
    const all = rows.map((row, i) => ({ row, i }))
    if (!sort) return all
    return all.sort((a, b) => {
      const x = keyOf(a.row[sort.col]); const y = keyOf(b.row[sort.col])
      return (x < y ? -1 : x > y ? 1 : 0) * (sort.dir === 'asc' ? 1 : -1)
    })
  }, [rows, sort])
  const toggle = col => setSort(prev => (prev && prev.col === col ? (prev.dir === 'asc' ? { col, dir: 'desc' } : null) : { col, dir: 'asc' }))
  const copy = async () => {
    const text = [columns.join('\t'), ...view.map(({ row }) => row.join('\t'))].join('\n')
    try { await navigator.clipboard.writeText(text); onCopied?.(`${rows.length} row${rows.length === 1 ? '' : 's'} copied, tab-separated.`) } catch { onCopied?.('Copy was refused here.') }
  }
  const save = () => {
    const text = [columns.map(csvCell).join(','), ...view.map(({ row }) => row.map(csvCell).join(','))].join('\r\n')
    const url = URL.createObjectURL(new Blob([`﻿${text}`], { type: 'text/csv;charset=utf-8' }))
    const link = Object.assign(globalThis.document.createElement('a'), { href: url, download: `${fileName || 'table'}.csv` })
    globalThis.document.body.append(link); link.click(); link.remove()
    globalThis.setTimeout(() => URL.revokeObjectURL(url), 1000)
    onCopied?.(`${rows.length} row${rows.length === 1 ? '' : 's'} saved as a file.`)
  }
  return (
    <div className="wbp-tablewrap">
      <div className="wbp-scroll" style={maxHeight ? { maxBlockSize: maxHeight } : undefined}>
        <table className={`wbp-table${dense ? ' wbp-table--dense' : ''}`} aria-label={label}>
          <thead>
            <tr>
              {columns.map((c, i) => (
                <th key={`${c}-${i}`} scope="col" className={numeric[i] ? 'is-num' : undefined} aria-sort={sort?.col === i ? (sort.dir === 'asc' ? 'ascending' : 'descending') : 'none'}>
                  <button type="button" onClick={() => toggle(i)} title={`Sort by ${c}`}>{c}{sort?.col === i ? (sort.dir === 'asc' ? <ArrowUp size={12} aria-hidden="true" /> : <ArrowDown size={12} aria-hidden="true" />) : null}</button>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {view.map(({ row, i }) => (
              <tr key={i} className={`${pick === i ? 'is-picked' : ''}${onRow ? ' is-link' : ''}`.trim() || undefined} aria-selected={pick === i ? 'true' : undefined}
                tabIndex={onRow ? 0 : undefined} onClick={onRow ? () => onRow(i) : undefined} onKeyDown={onRow ? e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onRow(i) } } : undefined}>
                {row.map((cell, j) => <td key={j} className={numeric[j] ? 'is-num' : undefined}>{cell}</td>)}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="wbp-tablefoot">
        <span>{rows.length} row{rows.length === 1 ? '' : 's'}</span>
        <span className="wbp-tablefoot__acts">
          <button type="button" className="wbp-btn wbp-btn--text" onClick={copy}><Copy size={13} aria-hidden="true" /> Copy rows</button>
          <button type="button" className="wbp-btn wbp-btn--text" onClick={save}><Download size={13} aria-hidden="true" /> Save as CSV</button>
        </span>
      </div>
    </div>
  )
}
