import { useEffect, useMemo, useState } from 'react'
import { Network } from 'lucide-react'
import { LanguageText } from '../../components/AnalystComponents.jsx'
import { getConnections } from '../../lib/apiClient.js'
import { buildModel } from '../../lib/connectionsModel.js'
import { formatMobile } from '../../lib/carriers.js'
import { questions } from '../connections/ConnectionsPanel.jsx'

// Questions about the connections of THIS case, written with its own numbers, in the forms the Ask engine answers best (docs/data/ask-engine-v2.md). They are offered only when the
// case can be drawn as connections (its call records are all in the typed tables); otherwise nothing is shown and nothing is said, because the other starters above still stand.
// Every number comes from the case's own contact records through the same read as the Connections page; nothing is invented in the browser.
export function linkStarters(model) {
  const subjects = model.nodes.filter(n => n.kind === 'subject').sort((a, b) => (b.events || 0) - (a.events || 0))
  if (!subjects.length) return []
  const out = []
  const top = subjects[0]
  out.push({ key: 'top', query: questions.top(top.id), label: `Who ${formatMobile(top.id)} talks to most` })
  // the pair of subscribers that share the most contacts: their shared contact, and how they are connected
  const shared = model.edges.filter(e => e.kind === 'inferred').sort((a, b) => b.count - a.count)[0]
  if (shared) {
    out.push({ key: 'common', query: questions.common(shared.source, shared.target), label: `What ${formatMobile(shared.source)} and ${formatMobile(shared.target)} both contact` })
    out.push({ key: 'path', query: questions.path(shared.source, shared.target), label: `How ${formatMobile(shared.source)} and ${formatMobile(shared.target)} are connected` })
  }
  const second = subjects.find(s => s.id !== top.id) || top
  out.push({ key: 'reach', query: `Who can be reached from ${second.id} within two hops, not through services or hubs?`, label: `Who is within two hops of ${formatMobile(second.id)}` })
  return out
}

export function LinkStarters({ caseId, onPick }) {
  const [state, setState] = useState({ status: 'loading', model: null })
  useEffect(() => {
    const controller = new AbortController()
    setState({ status: 'loading', model: null })
    getConnections({ caseId, perSubject: 12, signal: controller.signal })
      .then(data => setState(data?.available ? { status: 'ready', model: buildModel(data) } : { status: 'none', model: null }))
      .catch(error => { if (error?.name !== 'AbortError') setState({ status: 'none', model: null }) })
    return () => controller.abort()
  }, [caseId])
  const items = useMemo(() => (state.model ? linkStarters(state.model) : []), [state.model])
  if (state.status !== 'ready' || !items.length) return null
  return (
    <div className="gi-suggest gi-suggest--links" aria-label="Questions about the connections in this case">
      <h2><Network aria-hidden="true" />Connections in this case</h2>
      <ul>
        {items.map(item => (
          <li key={item.key}>
            <button type="button" onClick={() => onPick({ query: item.query, scope: 'all', family: 'link analysis' })}><small>link analysis</small><LanguageText>{item.label}</LanguageText></button>
          </li>
        ))}
      </ul>
    </div>
  )
}
