import { useEffect, useMemo, useState } from 'react'
import { getConnections, getQueryCapabilities } from '../../../lib/apiClient.js'
import { buildModel } from '../../../lib/connectionsModel.js'
import { linkStarters } from '../../../pages/investigate/LinkStarters.jsx'

const capabilityScope = {
  cdr: 'cdr', ipdr: 'ipdr', anpr: 'anpr', subscriber_identity: 'subscriber', subscriber: 'subscriber',
  tower_location: 'tower', tower: 'tower', financial: 'financial', transaction: 'financial',
  logs_access_security: 'access_log', access_log: 'access_log', document: 'document', images: 'image',
  image: 'image', audio_and_stt: 'audio', audio: 'audio', video: 'video',
}

function starterScope(family) {
  const recordType = Array.isArray(family?.record_types) ? family.record_types[0] : ''
  return capabilityScope[family?.id] || capabilityScope[recordType] || 'all'
}

// Suggestions are projected only from the service's curated capability response. No plausible-looking sample question is invented in the browser.
export function investigationStarters(payload, selectedScope = 'all') {
  const seen = new Set()
  return (Array.isArray(payload?.families) ? payload.families : [])
    .filter(family => ['queryable', 'semantic_only', 'limited'].includes(family?.availability))
    .flatMap(family => (Array.isArray(family?.suggested_queries) ? family.suggested_queries : []).map(query => ({
      query: String(query), family: family.label || String(family.id || ''), scope: starterScope(family),
    })))
    .filter(item => selectedScope === 'all' || item.scope === selectedScope)
    .filter(item => {
      const key = `${item.scope}:${item.query.toLocaleLowerCase()}`
      if (!item.query || seen.has(key)) return false
      seen.add(key)
      return true
    })
    .slice(0, 6)
}

// The two sources of starting questions for an empty thread: what the question service says this case can answer, and questions about the
// connections of this case written with its own numbers (offered only when the case can be drawn as connections).
export function useStarters(caseId, scope) {
  const [capabilities, setCapabilities] = useState(null)
  const [connections, setConnections] = useState(null)
  useEffect(() => {
    const request = new AbortController()
    getQueryCapabilities({ caseId, signal: request.signal }).then(setCapabilities).catch(error => { if (error.name !== 'AbortError') setCapabilities(null) })
    getConnections({ caseId, perSubject: 12, signal: request.signal })
      .then(data => setConnections(data?.available ? buildModel(data) : null))
      .catch(error => { if (error?.name !== 'AbortError') setConnections(null) })
    return () => request.abort()
  }, [caseId])
  const evidence = useMemo(() => investigationStarters(capabilities, scope), [capabilities, scope])
  const links = useMemo(() => (connections ? linkStarters(connections).map(item => ({ query: item.query, label: item.label, scope: 'all', family: 'Connections' })) : []), [connections])
  return { evidence, links }
}
