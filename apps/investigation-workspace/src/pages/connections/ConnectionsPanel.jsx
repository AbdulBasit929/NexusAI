import { Link } from 'react-router-dom'
import { ClipboardCopy, MessageSquareText } from 'lucide-react'
import { CARRIERS } from '../../lib/carriers.js'
import { formatNumber } from '../../lib/format.js'
import { CarrierBadge } from '../../components/CarrierBadge.jsx'
import { LanguageText } from '../../components/AnalystComponents.jsx'

const day = value => (value ? String(value).slice(0, 10) : '—')
const KIND = { subject: 'Subscriber (its records are in the case)', contact: 'Other number', hub: 'A number many subscribers contact', service: 'Service, short code or sender name', group: 'A group of numbers' }

// The questions the panel offers are the forms the Ask engine answers best (measured, docs/data/ask-engine-v2.md); each opens the Ask page with the question written.
export const askLink = (caseId, question) => `/cases/${encodeURIComponent(caseId)}/investigate?${new URLSearchParams({ question })}`
export const questions = {
  top: n => `Who does ${n} talk to most? Give the top five, leaving out services and numbers that everyone contacts.`,
  between: (a, b) => `How many events are there between ${a} and ${b}?`,
  incoming: (a, b) => `How many times did ${a} get a call or message from ${b}?`,
  common: (a, b) => `Which numbers do ${a} and ${b} both contact, leaving out services and numbers that everyone contacts?`,
  path: (a, b) => `How are ${a} and ${b} connected within three hops, not through services or hubs?`,
}

function NumberChip({ node, onSelect }) {
  return <button type="button" className="cx-numchip" onClick={() => onSelect(node.id)}><CarrierBadge carrier={node.carrier} size={14} /><LanguageText as="bdi" identifier>{node.label}</LanguageText></button>
}

// What is selected, in words and with the next step one click away. With nothing selected it says where to start.
export function ConnectionsPanel({ caseId, model, base, selectedId, selectedEdge, slots, shared, path, onSelect, onSlot, onClear, onCopy }) {
  const byId = new Map(base.nodes.map(n => [n.id, n]))
  const A = slots.A ? byId.get(slots.A) : null
  const B = slots.B ? byId.get(slots.B) : null

  if (selectedEdge) {
    const a = byId.get(selectedEdge.source), b = byId.get(selectedEdge.target)
    return (
      <aside className="cx-panel" aria-live="polite" aria-label="Selected link">
        <h2>{selectedEdge.kind === 'inferred' ? 'Shared contacts' : 'Contact between two numbers'}</h2>
        <p className="cx-panel__pair"><NumberChip node={a} onSelect={onSelect} /><span aria-hidden="true">–</span><NumberChip node={b} onSelect={onSelect} /></p>
        {selectedEdge.kind === 'inferred' ? (
          <p className="cx-note">These two subscribers have {formatNumber(selectedEdge.count)} contact{selectedEdge.count === 1 ? '' : 's'} in common. This is inferred from the links shown; it does not say the two numbers were in touch with each other.</p>
        ) : (
          <dl className="cx-facts">
            <div><dt>Events</dt><dd>{formatNumber(selectedEdge.count)}</dd></div>
            <div><dt>Outgoing</dt><dd>{formatNumber(selectedEdge.outgoing || 0)}</dd></div>
            <div><dt>Incoming</dt><dd>{formatNumber(selectedEdge.incoming || 0)}</dd></div>
            <div><dt>Direction unknown</dt><dd>{formatNumber(selectedEdge.unknown || 0)}</dd></div>
            <div><dt>First</dt><dd>{day(selectedEdge.first)}</dd></div>
            <div><dt>Last</dt><dd>{day(selectedEdge.last)}</dd></div>
          </dl>
        )}
        <div className="cx-actions">
          <Link className="cx-button" to={askLink(caseId, selectedEdge.kind === 'inferred' ? questions.common(a.id, b.id) : questions.between(a.id, b.id))}><MessageSquareText aria-hidden="true" />Ask about this {selectedEdge.kind === 'inferred' ? 'pair' : 'link'}</Link>
          <button type="button" className="cx-button" onClick={onClear}>Clear</button>
        </div>
      </aside>
    )
  }

  if (selectedId && byId.has(selectedId)) {
    const node = byId.get(selectedId)
    const mine = model.edges.filter(e => e.kind === 'observed' && (e.source === node.id || e.target === node.id)).sort((x, y) => y.count - x.count)
    const total = mine.reduce((sum, e) => sum + e.count, 0)
    return (
      <aside className="cx-panel" aria-live="polite" aria-label="Selected number">
        <h2><LanguageText as="bdi" identifier>{node.label}</LanguageText></h2>
        <p className="cx-panel__carrier"><CarrierBadge carrier={node.carrier} size={20} showName />
          <small>found from {node.carrierHow}</small></p>
        <p className="cx-note cx-note--plain">{KIND[node.kind]}.</p>
        <dl className="cx-facts">
          <div><dt>Events in the case</dt><dd>{formatNumber(node.events || total)}</dd></div>
          <div><dt>Numbers in contact</dt><dd>{formatNumber(mine.length)}</dd></div>
          <div><dt>First</dt><dd>{day(node.first)}</dd></div>
          <div><dt>Last</dt><dd>{day(node.last)}</dd></div>
          {node.imei ? <div><dt>Handset (IMEI)</dt><dd><LanguageText as="bdi" identifier>{node.imei}</LanguageText></dd></div> : null}
          {node.imsi ? <div><dt>SIM (IMSI)</dt><dd><LanguageText as="bdi" identifier>{node.imsi}</LanguageText></dd></div> : null}
        </dl>
        <div className="cx-actions">
          <button type="button" className="cx-button" aria-pressed={slots.A === node.id} onClick={() => onSlot('A', node.id)}>Set as A</button>
          <button type="button" className="cx-button" aria-pressed={slots.B === node.id} onClick={() => onSlot('B', node.id)}>Set as B</button>
          <Link className="cx-button" to={askLink(caseId, questions.top(node.id))}><MessageSquareText aria-hidden="true" />Ask about this number</Link>
          <button type="button" className="cx-button" onClick={() => onCopy(node.id)}><ClipboardCopy aria-hidden="true" />Copy number</button>
        </div>
        <h3>In contact with</h3>
        <ul className="cx-list">
          {mine.slice(0, 10).map(e => { const other = byId.get(e.source === node.id ? e.target : e.source); return other ? <li key={e.id}><NumberChip node={other} onSelect={onSelect} /><span>{formatNumber(e.count)} events</span></li> : null })}
          {mine.length > 10 ? <li className="cx-muted">and {formatNumber(mine.length - 10)} more (see the table below)</li> : null}
        </ul>
        {A && B ? <PairBlock caseId={caseId} A={A} B={B} shared={shared} path={path} byId={byId} onSelect={onSelect} onSlot={onSlot} /> : slots.A || slots.B ? <p className="cx-muted">Set a second number to compare: shared contacts and the shortest connection.</p> : null}
      </aside>
    )
  }

  const subjects = base.nodes.filter(n => n.kind === 'subject').sort((a, b) => (b.events || 0) - (a.events || 0))
  const strongest = base.edges.filter(e => e.kind === 'observed').sort((a, b) => b.count - a.count).slice(0, 6)
  const sharedPairs = base.edges.filter(e => e.kind === 'inferred').sort((a, b) => b.count - a.count).slice(0, 5)
  return (
    <aside className="cx-panel" aria-live="polite" aria-label="Where to start">
      <h2>Where to start</h2>
      {A && B ? <PairBlock caseId={caseId} A={A} B={B} shared={shared} path={path} byId={byId} onSelect={onSelect} onSlot={onSlot} /> : null}
      <h3>Subscribers</h3>
      <ul className="cx-list">{subjects.slice(0, 8).map(n => <li key={n.id}><NumberChip node={n} onSelect={onSelect} /><span>{formatNumber(n.events || 0)} events</span></li>)}</ul>
      <h3>Strongest contacts</h3>
      <ul className="cx-list">{strongest.map(e => <li key={e.id}><NumberChip node={byId.get(e.source)} onSelect={onSelect} /><span aria-hidden="true">–</span><NumberChip node={byId.get(e.target)} onSelect={onSelect} /><span>{formatNumber(e.count)}</span></li>)}</ul>
      {sharedPairs.length ? (
        <>
          <h3>Subscribers with shared contacts</h3>
          <ul className="cx-list">{sharedPairs.map(e => <li key={e.id}><NumberChip node={byId.get(e.source)} onSelect={onSelect} /><span aria-hidden="true">–</span><NumberChip node={byId.get(e.target)} onSelect={onSelect} /><span>{formatNumber(e.count)} shared</span></li>)}</ul>
        </>
      ) : null}
      <details className="cx-help">
        <summary>How to read this chart</summary>
        <p>A square badge is a subscriber (a number whose records the case holds); its badge and ring show the carrier of its SIM. A circle is another number it was in contact with, ringed in the carrier of its number range (a ported number may differ). A triangle is a number many subscribers contact, a diamond a service or short code. A solid line is contact recorded between two numbers (thicker is more events); a dashed line joins two subscribers who share contacts, which is inferred and does not say they were in touch.</p>
        <p>Select a number to see who it was in contact with; set two numbers as A and B to see the contacts they share and the shortest connection.</p>
      </details>
    </aside>
  )
}

function PairBlock({ caseId, A, B, shared, path, byId, onSelect, onSlot }) {
  return (
    <section className="cx-pair" aria-label="Comparing two numbers">
      <h3>A and B</h3>
      <p className="cx-panel__pair"><NumberChip node={A} onSelect={onSelect} /><span aria-hidden="true">–</span><NumberChip node={B} onSelect={onSelect} /></p>
      <h4>Contacts they share</h4>
      {shared.length ? <ul className="cx-list">{shared.slice(0, 8).map(s => { const node = byId.get(s.id); return node ? <li key={s.id}><NumberChip node={node} onSelect={onSelect} /><span>{formatNumber(s.first)} and {formatNumber(s.second)} events</span></li> : null })}</ul> : <p className="cx-muted">None, once services and numbers everyone contacts are left out.</p>}
      <h4>Shortest connection</h4>
      {path ? <p className="cx-path">{path.map((id, i) => <span key={id}>{i ? <span aria-hidden="true"> → </span> : null}<NumberChip node={byId.get(id)} onSelect={onSelect} /></span>)}<small>{path.length - 1} step{path.length === 2 ? '' : 's'}</small></p> : <p className="cx-muted">No connection within four steps over the links shown.</p>}
      <div className="cx-actions">
        <Link className="cx-button" to={askLink(caseId, questions.path(A.id, B.id))}><MessageSquareText aria-hidden="true" />Ask the connection</Link>
        <Link className="cx-button" to={askLink(caseId, questions.common(A.id, B.id))}>Ask the shared contacts</Link>
        <button type="button" className="cx-button" onClick={() => { onSlot('A', null); onSlot('B', null) }}>Clear A and B</button>
      </div>
    </section>
  )
}

export const carrierLegend = base => [...new Set(base.nodes.map(n => n.carrier))].map(id => CARRIERS[id]).filter(Boolean)
