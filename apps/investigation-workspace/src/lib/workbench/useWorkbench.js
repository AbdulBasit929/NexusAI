import { useCallback, useEffect, useState } from 'react'
import { getConnections, getPlaces } from '../apiClient.js'
import { createCaseIndex } from './caseIndex.js'

// The two live reads of a case, once for the whole visit: GET /analysis/connections and GET /analysis/places. The places may be missing (a case whose records are not all typed, a failed request)
// without taking the links down with them: the workbench then says so on the Places pane and still draws the contacts. A read is kept for five minutes, so that leaving the page and coming back
// does not read the case again.
const FRESH_MS = 5 * 60 * 1000
const reads = new Map()
export const clearWorkbenchReads = () => reads.clear()

export function useCaseReads(caseId) {
  const [state, setState] = useState(() => reads.get(caseId)?.state || { status: 'loading' })
  const [nonce, setNonce] = useState(0)
  useEffect(() => {
    const hit = reads.get(caseId)
    if (hit && Date.now() - hit.at < FRESH_MS && nonce === 0) { setState(hit.state); return undefined }
    const controller = new AbortController()
    setState({ status: 'loading' })
    Promise.allSettled([getConnections({ caseId, perSubject: 60, signal: controller.signal }), getPlaces({ caseId, signal: controller.signal })]).then(([connections, places]) => {
      if (controller.signal.aborted) return
      let next
      if (connections.status === 'rejected') next = { status: 'failed', error: connections.reason }
      else if (!connections.value?.available) next = { status: 'unavailable', reason: connections.value?.reason || 'The case has no typed call records.' }
      else {
        const placesRead = places.status === 'fulfilled' ? places.value : null
        const usable = placesRead?.available ? placesRead : null
        next = {
          status: 'ready', index: createCaseIndex({ connections: connections.value, places: usable }),
          places: places.status === 'rejected' ? { status: 'failed', error: places.reason } : usable ? { status: 'ready' } : { status: 'unavailable', reason: placesRead?.reason || 'The case has no located cell visits.' },
        }
        reads.set(caseId, { at: Date.now(), state: next })
      }
      setState(next)
    })
    return () => controller.abort()
  }, [caseId, nonce])
  const reload = useCallback(() => { reads.delete(caseId); setNonce(n => n + 1) }, [caseId])
  return { ...state, reload }
}
