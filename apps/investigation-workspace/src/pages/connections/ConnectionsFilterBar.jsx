import { useEffect, useId, useMemo, useRef, useState } from 'react'
import { ChevronDown, Link2, RotateCcw, Search } from 'lucide-react'
import { CARRIERS, CARRIER_ORDER } from '../../lib/carriers.js'
import { isFiltered, summarise } from '../../lib/connectionsModel.js'
import { formatNumber } from '../../lib/format.js'
import { CarrierBadge } from '../../components/CarrierBadge.jsx'

const DAY = 86400000
const iso = ms => new Date(ms).toISOString().slice(0, 10)

// Targets, carriers, period and what to show, in one bar over the chart (roadmap 7.8.3 row 2). Everything it sets is in the address, so a filtered view can be shared and reopened.
export function ConnectionsFilterBar({ base, shown, filters, onChange, onCopyLink, copied = false }) {
  const subjects = useMemo(() => base.nodes.filter(n => n.kind === 'subject').sort((a, b) => (b.events || 0) - (a.events || 0)), [base])
  const carrierCounts = useMemo(() => {
    const counts = new Map()
    for (const n of base.nodes) { const c = counts.get(n.carrier) || { subjects: 0, numbers: 0 }; c.numbers++; if (n.kind === 'subject') c.subjects++; counts.set(n.carrier, c) }
    return counts
  }, [base])
  const present = CARRIER_ORDER.filter(id => carrierCounts.has(id))
  const period = useMemo(() => {
    const times = base.nodes.flatMap(n => [n.first, n.last]).filter(Boolean).map(s => Date.parse(String(s).replace(' ', 'T') + 'Z')).filter(Number.isFinite)
    return times.length ? { lo: iso(Math.min(...times)), hi: iso(Math.max(...times)) } : { lo: '', hi: '' }
  }, [base])
  const sum = summarise(base, shown)
  const filtered = isFiltered(filters)
  const set = patch => onChange({ ...filters, ...patch })
  const toggleCarrier = id => { const next = new Set(filters.carriers); if (next.has(id)) next.delete(id); else next.add(id); set({ carriers: next }) }
  const lastDays = days => period.hi && set({ from: iso(Date.parse(`${period.hi}T00:00:00Z`) - (days - 1) * DAY), to: period.hi })

  return (
    <div className="cx-filterbar" role="search" aria-label="Targets and filters">
      <div className="cx-filter">
        <span className="cx-filter__label" id="cx-target-label">Targets</span>
        <TargetPicker subjects={subjects} selected={filters.targets} onChange={targets => set({ targets })} />
      </div>
      <div className="cx-filter" role="group" aria-label="Carrier">
        <span className="cx-filter__label">Carrier</span>
        {present.map(id => {
          const count = carrierCounts.get(id)
          const on = filters.carriers.has(id)
          return (
            <button key={id} type="button" className="cx-chip" aria-pressed={on} onClick={() => toggleCarrier(id)} title={`${CARRIERS[id].name}: ${count.subjects} subscribers, ${count.numbers} numbers. ${CARRIERS[id].note}`}>
              <CarrierBadge carrier={id} size={16} showName /><small>{formatNumber(count.numbers)}</small>
            </button>
          )
        })}
      </div>
      <div className="cx-filter" role="group" aria-label="Period">
        <span className="cx-filter__label">Period</span>
        <input type="date" aria-label="From" value={filters.from} min={period.lo} max={period.hi} onChange={event => set({ from: event.target.value })} />
        <span aria-hidden="true">to</span>
        <input type="date" aria-label="To" value={filters.to} min={period.lo} max={period.hi} onChange={event => set({ to: event.target.value })} />
        <span className="cx-presets">
          <button type="button" onClick={() => set({ from: '', to: '' })}>Whole</button>
          <button type="button" onClick={() => lastDays(90)}>90 days</button>
          <button type="button" onClick={() => lastDays(30)}>30 days</button>
          <button type="button" onClick={() => lastDays(7)}>7 days</button>
        </span>
      </div>
      <div className="cx-filter cx-filter--end">
        <output className={`cx-summary${filtered ? ' is-filtered' : ''}`} aria-live="polite">
          {filtered ? `Showing ${sum.shown.subjects} of ${sum.all.subjects} subscribers · ${formatNumber(sum.shown.numbers)} of ${formatNumber(sum.all.numbers)} numbers · ${formatNumber(sum.shown.links)} of ${formatNumber(sum.all.links)} links` : `${sum.all.subjects} subscribers · ${formatNumber(sum.shown.numbers)} numbers · ${formatNumber(sum.shown.links)} links`}
          {filters.from || filters.to ? ' · shared-contact links are hidden for a period' : ''}
        </output>
        <button type="button" className="cx-button" onClick={() => onChange(null)} disabled={!filtered}><RotateCcw aria-hidden="true" />Reset</button>
        <button type="button" className="cx-button" onClick={onCopyLink}><Link2 aria-hidden="true" />{copied ? 'Copied' : 'Copy link'}</button>
      </div>
      <details className="cx-more">
        <summary>What to show</summary>
        <div className="cx-filter" role="group" aria-label="What to show">
          <span className="cx-filter__label">Show</span>
          {[['observed', 'Contacts between numbers'], ['inferred', 'Shared contacts'], ['contacts', 'Other numbers'], ['hubs', 'Numbers everyone contacts'], ['services', 'Services and short codes']].map(([key, text]) => (
            <label key={key} className="cx-check"><input type="checkbox" checked={filters.show[key]} onChange={event => set({ show: { ...filters.show, [key]: event.target.checked } })} />{text}</label>
          ))}
          <label className="cx-check">At least <input type="number" min="1" max="999" value={filters.minEvents} aria-label="Minimum events for a link" onChange={event => set({ minEvents: Math.max(1, Math.min(999, Number(event.target.value) || 1)) })} /> events</label>
        </div>
      </details>
    </div>
  )
}

function TargetPicker({ subjects, selected, onChange }) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const root = useRef(null)
  const listId = useId()
  useEffect(() => {
    if (!open) return undefined
    const away = event => { if (!root.current?.contains(event.target)) setOpen(false) }
    const key = event => { if (event.key === 'Escape') setOpen(false) }
    document.addEventListener('mousedown', away); document.addEventListener('keydown', key)
    return () => { document.removeEventListener('mousedown', away); document.removeEventListener('keydown', key) }
  }, [open])
  const needle = query.replace(/\D/g, '')
  const rows = subjects.filter(s => !needle || s.id.includes(needle) || s.label.replace(/\D/g, '').includes(needle))
  const toggle = id => { const next = new Set(selected); if (next.has(id)) next.delete(id); else next.add(id); onChange(next) }
  return (
    <div className="cx-picker" ref={root}>
      <button type="button" className="cx-button cx-picker__button" aria-haspopup="dialog" aria-expanded={open} aria-controls={listId} aria-labelledby="cx-target-label cx-target-value" onClick={() => setOpen(value => !value)}>
        <span id="cx-target-value">{selected.size ? `${selected.size} of ${subjects.length} selected` : `All ${subjects.length} subscribers`}</span><ChevronDown aria-hidden="true" />
      </button>
      {open ? (
        <div className="cx-picker__pop" id={listId} role="dialog" aria-label="Choose targets">
          <div className="cx-picker__head">
            <label className="cx-picker__search"><Search aria-hidden="true" /><input type="search" placeholder="Find a number" aria-label="Find a target" value={query} onChange={event => setQuery(event.target.value)} autoFocus /></label>
            <button type="button" onClick={() => onChange(new Set(subjects.map(s => s.id)))}>All</button>
            <button type="button" onClick={() => onChange(new Set())}>None</button>
            <button type="button" onClick={() => onChange(new Set(subjects.slice(0, 5).map(s => s.id)))} title="The five subscribers with the most events">Top 5</button>
          </div>
          <ul className="cx-picker__list">
            {rows.map(s => (
              <li key={s.id}>
                <label className="cx-picker__row">
                  <input type="checkbox" checked={selected.has(s.id)} onChange={() => toggle(s.id)} />
                  <CarrierBadge carrier={s.carrier} size={18} />
                  <b>{s.label}</b>
                  <small>{formatNumber(s.events || 0)} events · {CARRIERS[s.carrier]?.name}</small>
                </label>
              </li>
            ))}
            {!rows.length ? <li className="cx-picker__none">No target matches.</li> : null}
          </ul>
        </div>
      ) : null}
    </div>
  )
}
