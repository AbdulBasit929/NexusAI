// What the evidence panel of the workbench says about a selected mark (docs/ux/workbench/RESEARCH_NOTE.md, rule 5): what it is, how strong it is, the values behind it, the caveat in the
// engine's own words, and the rows. Every value is read from the live reads or from the row of the answer; nothing is written that the case does not hold.
import { clockOf, labelOf, shortNumber } from './caseIndex.js'
import { suggestionsFor } from './workingSet.js'

const fact = (label, value) => [label, value]
const f = (list, label) => (list.find(([k]) => k === label) || [])[1] ?? ''
const n = value => (Number.isFinite(Number(value)) ? Number(value).toLocaleString() : String(value ?? ''))

// the columns the evidence panel lists for the marks of one kind, and the cells of one mark
const DRAWER = {
  window: { columns: ['Start', 'End', 'Length'], cells: m => [clockOf(m.time), clockOf(m.end).slice(11), f(m.facts, 'Length')] },
  meeting: { columns: ['Start', 'End', 'Score'], cells: m => [clockOf(m.time), clockOf(m.end).slice(11), f(m.facts, 'Score')] },
  successor: { columns: ['New number', 'Score'], cells: m => [labelOf(m.numbers[1]), f(m.facts, 'Score')] },
  silent: { columns: ['Number', 'Last event'], cells: m => [labelOf(m.numbers[0]), f(m.facts, 'Last event')] },
  move: { columns: ['Part of the day', 'New place from the week of', 'Distance'], cells: m => [f(m.facts, 'Part of the day'), f(m.facts, 'New place from the week of'), f(m.facts, 'Distance')] },
  alert: { columns: ['From', 'Speed', 'Distance'], cells: m => [clockOf(m.time), f(m.facts, 'Speed'), f(m.facts, 'Distance')] },
  rhythm: { columns: ['Number', 'Before', 'After'], cells: m => [labelOf(m.numbers[0]), f(m.facts, 'Average before'), f(m.facts, 'Average after')] },
  anchor: { columns: ['Part of the day', 'Cell site', 'From the week of'], cells: m => [f(m.facts, 'Part of the day'), f(m.facts, 'Cell site'), f(m.facts, 'From the week of')] },
}

const CONNECTION_CARE = 'A link is a record of contact between two numbers; it is not a finding about the people. Services and numbers that everyone contacts are left out where the query says so.'
const SITE_CARE = 'A cell site is a coverage area with a stated radius, not the position of a person.'

export function drawerOf(selection, { index, answer = null, range = null } = {}) {
  if (!selection) return null
  const contactCare = [index.connectionCaveats[0] || CONNECTION_CARE]
  const ask = numbers => suggestionsFor(index, { focus: numbers, selected: numbers[0] }).slice(0, 5)
  switch (selection.type) {
    case 'node': {
      const info = index.numberInfo(selection.id)
      const strongest = index.strongest(selection.id, 8)
      const own = info.kind === 'subject'
      return {
        key: `node:${selection.id}`, title: labelOf(selection.id), subtitle: own ? 'A subscriber of this case' : info.kind === 'service' ? 'A service or short code' : info.kind === 'hub' ? 'A number that many subscribers contact' : 'A number that subscribers contact (seen only through their records)',
        strength: 'observed', number: selection.id, carrier: info.carrier,
        facts: [fact(own ? 'Events' : 'Events with subscribers', n(info.events)), fact(own ? 'Contacts' : 'Subscribers who contact it', n(info.contacts)), fact('First event', clockOf(info.first).slice(0, 10) || '—'), fact('Last event', clockOf(info.last).slice(0, 10) || '—'),
          ...(own && info.imsi ? [fact('SIM (IMSI)', info.imsi)] : []), ...(own && info.imei ? [fact('Handset (IMEI)', info.imei)] : [])],
        caveats: contactCare, rowsTitle: 'Strongest contacts', columns: ['Number', 'Events'],
        rows: strongest.map(x => [labelOf(x), n(index.edgeBetween(selection.id, x)?.events ?? '')]), rowTargets: strongest.map(x => ({ type: 'node', id: x })), pick: null,
        ask: ask([selection.id]),
      }
    }
    case 'edge': {
      const e = index.edgeBetween(selection.a, selection.b)
      const shared = index.sharedContacts(selection.a, selection.b)
      return {
        key: `edge:${selection.a}|${selection.b}`, title: `${shortNumber(selection.a)} ↔ ${shortNumber(selection.b)}`, subtitle: 'Observed contact', strength: 'observed', numbers: [selection.a, selection.b],
        facts: e ? [fact('Events', n(e.events)), fact('Outgoing', n(e.outgoing)), fact('Incoming', n(e.incoming)), fact('Call time', `${Math.round((e.duration_s || 0) / 60).toLocaleString()} min`), fact('First', clockOf(e.first).slice(0, 10)), fact('Last', clockOf(e.last).slice(0, 10))] : [fact('Events', '0')],
        caveats: contactCare, rowsTitle: 'Shared contacts', columns: ['Number', 'With …' + shortNumber(selection.a), 'With …' + shortNumber(selection.b)],
        rows: shared.map(s => [labelOf(s.number), n(s.first), n(s.second)]), rowTargets: shared.map(s => ({ type: 'node', id: s.number })), pick: null,
        ask: ask([selection.a, selection.b]),
      }
    }
    case 'site': {
      const site = index.siteOf(selection.id)
      const visitors = index.siteVisitors(selection.id)
      return {
        key: `site:${selection.id}`, title: `Cell site ${site?.name || String(selection.id).split(':').pop()}`, subtitle: 'A coverage area, not the position of a person', strength: 'observed', site: selection.id,
        facts: [fact('Events at this site', n(site?.events ?? '—')), fact('Subscribers who used it', n(site?.visitors ?? '—')), fact('Stated radius', site?.radius_m ? `${n(site.radius_m)} m` : 'not stated'),
          fact('Position', site && Number.isFinite(site.lat) ? `${site.lat.toFixed(4)}, ${site.lon.toFixed(4)}` : 'not in the records'), fact('Cell key', String(selection.id))],
        caveats: [index.placeCaveats[0] || SITE_CARE], rowsTitle: 'Who used it', columns: ['Number', 'Events', 'First', 'Last'],
        rows: visitors.map(v => [labelOf(v.subject), n(v.events), clockOf(v.first).slice(0, 10), clockOf(v.last).slice(0, 10)]), rowTargets: visitors.map(v => ({ type: 'node', id: v.subject })), pick: null,
        ask: ask(visitors.slice(0, 1).map(v => v.subject)),
      }
    }
    case 'mark': {
      const mark = answer?.marks.find(m => m.id === selection.id)
      if (!mark) return null
      const spec = DRAWER[mark.kind] || { columns: ['Time'], cells: m => [clockOf(m.time)] }
      const same = answer.marks.filter(m => m.kind === mark.kind && (!range || m.day == null || (m.day >= range[0] && m.day <= range[1])))
      return {
        key: `mark:${mark.id}`, title: mark.title, subtitle: mark.subtitle, strength: answer.strength, numbers: mark.numbers, mark: mark.id,
        facts: mark.facts, values: answer.values, caveats: answer.caveats, rowsTitle: same.length === 1 ? 'The row' : `The ${same.length} rows`, columns: spec.columns,
        rows: same.map(m => spec.cells(m)), rowTargets: same.map(m => ({ type: 'mark', id: m.id })), pick: Math.max(0, same.findIndex(m => m.id === mark.id)),
        ask: ask(mark.numbers),
      }
    }
    default:
      return null
  }
}

// every mark in the order of time, for stepping through them with [ and ]
export function steppable(marks) {
  return [...marks].sort((x, y) => (x.day ?? 1e9) - (y.day ?? 1e9) || String(x.time).localeCompare(String(y.time)))
}
