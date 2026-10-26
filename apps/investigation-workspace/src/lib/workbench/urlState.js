// The state of the workbench lives in its address, so that the same link gives the same picture (master plan 7.3, "Reproducible and exportable"): the question, the pane, the selected mark, the numbers
// added to the working set, the period and the layers switched off. Nothing else is remembered between visits except the width of the evidence panel, which is a preference of the person.
export const PANES = ['links', 'places', 'table']

const list = value => String(value || '').split(',').map(s => s.trim()).filter(Boolean)

export function parseUrl(search) {
  const p = new URLSearchParams(search)
  const pane = p.get('pane')
  return {
    q: (p.get('q') || '').slice(0, 600),
    pane: PANES.includes(pane) ? pane : 'links',
    sel: p.get('sel') || '',
    added: list(p.get('n')).filter(n => /^\d{8,14}$/.test(n)).slice(0, 40),
    // the numbers to start from (the address of the Connections page before the workbench, "?t=", is still read): they take the place of a question's numbers
    targets: [...new Set(list(p.get('t')).map(n => n.replace(/\D/g, '')).filter(n => /^\d{8,14}$/.test(n)))].slice(0, 6),
    from: /^\d{4}-\d{2}-\d{2}$/.test(p.get('from') || '') ? p.get('from') : '',
    to: /^\d{4}-\d{2}-\d{2}$/.test(p.get('to') || '') ? p.get('to') : '',
    off: list(p.get('off')),
    place: p.get('place') === 'recorded' ? 'recorded' : p.get('place') === 'city' ? 'city' : '',
  }
}

// sets the given keys and removes the ones whose value is empty (a plain string, a list or null)
export function patchUrl(previous, patch) {
  const next = new URLSearchParams(previous)
  for (const [key, value] of Object.entries(patch)) {
    const text = Array.isArray(value) ? value.join(',') : value
    if (text === null || text === undefined || text === '' || text === false) next.delete(key)
    else next.set(key, String(text))
  }
  return next
}

// a selection, as one token of the address: node:92300…, edge:a|b, site:cell, mark:window:2
export function selectionToken(selection) {
  if (!selection) return ''
  switch (selection.type) {
    case 'node': return `node:${selection.id}`
    case 'edge': return `edge:${selection.a}|${selection.b}`
    case 'site': return `site:${selection.id}`
    case 'mark': return `mark:${selection.id}`
    default: return ''
  }
}
export function selectionFromToken(token) {
  const text = String(token || '')
  const at = text.indexOf(':')
  if (at < 1) return null
  const type = text.slice(0, at)
  const rest = text.slice(at + 1)
  if (!rest) return null
  if (type === 'node') return { type, id: rest }
  if (type === 'site') return { type, id: rest }
  if (type === 'mark') return { type, id: rest }
  if (type === 'edge') { const [a, b] = rest.split('|'); return a && b ? { type, a, b } : null }
  return null
}
export const sameSelection = (a, b) => selectionToken(a) === selectionToken(b)
