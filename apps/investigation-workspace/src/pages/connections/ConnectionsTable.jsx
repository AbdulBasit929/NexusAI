import { useMemo, useState } from 'react'
import { Download } from 'lucide-react'
import { CARRIERS } from '../../lib/carriers.js'
import { formatNumber } from '../../lib/format.js'
import { CarrierBadge } from '../../components/CarrierBadge.jsx'

const day = value => (value ? String(value).slice(0, 10) : '')

function csv(rows) {
  const cell = value => { const s = String(value ?? ''); return /[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s }
  return rows.map(row => row.map(cell).join(',')).join('\r\n')
}

function save(name, text) {
  const url = URL.createObjectURL(new Blob(['﻿', text], { type: 'text/csv;charset=utf-8' }))
  const link = Object.assign(globalThis.document.createElement('a'), { href: url, download: name })
  globalThis.document.body.append(link); link.click(); link.remove()
  globalThis.setTimeout(() => URL.revokeObjectURL(url), 1000)
}

// The table twin of the chart: every number and every link that the chart draws, sortable, with the same selection, and the data as CSV.
// It is the keyboard and screen-reader path (roadmap V7) and the way to take the numbers out of the screen.
export function ConnectionsTable({ model, selectedId, onSelect }) {
  const [tab, setTab] = useState('numbers')
  const [sort, setSort] = useState({ key: 'events', dir: -1 })
  const byId = useMemo(() => new Map(model.nodes.map(n => [n.id, n])), [model])
  const numbers = useMemo(() => model.nodes.map(n => ({ id: n.id, number: n.label, kind: n.kind, carrier: CARRIERS[n.carrier]?.name || '', events: n.events || 0, links: n.degree ?? model.edges.filter(e => e.source === n.id || e.target === n.id).length, first: day(n.first), last: day(n.last) })), [model])
  const links = useMemo(() => model.edges.map(e => ({ id: e.id, from: byId.get(e.source)?.label || e.source, to: byId.get(e.target)?.label || e.target, kind: e.kind === 'inferred' ? 'shared contacts' : 'contact', events: e.count, outgoing: e.outgoing ?? '', incoming: e.incoming ?? '', first: day(e.first), last: day(e.last), source: e.source })), [model, byId])
  const rows = tab === 'numbers' ? numbers : links
  const sorted = useMemo(() => [...rows].sort((a, b) => { const x = a[sort.key], y = b[sort.key]; return (typeof x === 'number' && typeof y === 'number' ? x - y : String(x).localeCompare(String(y), undefined, { numeric: true })) * sort.dir }), [rows, sort])
  const columns = tab === 'numbers'
    ? [['number', 'Number'], ['kind', 'Kind'], ['carrier', 'Carrier'], ['events', 'Events'], ['links', 'Links'], ['first', 'First'], ['last', 'Last']]
    : [['from', 'From'], ['to', 'To'], ['kind', 'Kind'], ['events', 'Events'], ['outgoing', 'Outgoing'], ['incoming', 'Incoming'], ['first', 'First'], ['last', 'Last']]
  const exportCsv = () => save(`connections-${tab}.csv`, csv([columns.map(c => c[1]), ...sorted.map(r => columns.map(([key]) => r[key]))]))
  const header = key => setSort(s => ({ key, dir: s.key === key ? -s.dir : (key === 'events' || key === 'links' ? -1 : 1) }))
  return (
    <section className="cx-table" id="cx-table" tabIndex={-1} aria-label="The same numbers as a table">
      <div className="cx-table__bar">
        <div role="tablist" aria-label="Table">
          {[['numbers', `Numbers (${formatNumber(numbers.length)})`], ['links', `Links (${formatNumber(links.length)})`]].map(([key, text]) => (
            <button key={key} type="button" role="tab" aria-selected={tab === key} onClick={() => { setTab(key); setSort({ key: 'events', dir: -1 }) }}>{text}</button>
          ))}
        </div>
        <button type="button" className="cx-button" onClick={exportCsv}><Download aria-hidden="true" />Save as CSV</button>
      </div>
      <div className="cx-table__scroll">
        <table>
          <caption className="visually-hidden">{tab === 'numbers' ? 'Numbers' : 'Links'} drawn on the chart, sortable</caption>
          <thead><tr>{columns.map(([key, text]) => <th key={key} scope="col" aria-sort={sort.key === key ? (sort.dir > 0 ? 'ascending' : 'descending') : 'none'}><button type="button" onClick={() => header(key)}>{text}</button></th>)}</tr></thead>
          <tbody>
            {sorted.slice(0, 500).map(r => (
              <tr key={r.id} className={(tab === 'numbers' ? r.id : r.source) === selectedId ? 'is-selected' : undefined}>
                {columns.map(([key]) => <td key={key}>{key === 'number' ? <button type="button" className="cx-linkish" onClick={() => onSelect(r.id)}><CarrierBadge carrier={model.nodes.find(n => n.id === r.id)?.carrier} size={14} />{r[key]}</button> : typeof r[key] === 'number' ? formatNumber(r[key]) : r[key]}</td>)}
              </tr>
            ))}
          </tbody>
        </table>
        {sorted.length > 500 ? <p className="cx-muted">The first 500 of {formatNumber(sorted.length)} rows are shown; the CSV has all of them.</p> : null}
      </div>
    </section>
  )
}
