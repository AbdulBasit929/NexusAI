import { useCallback, useEffect, useState } from 'react'
import { listCaseRecords, updateCaseRecord } from './apiClient.js'

// The case record (decision D30): what analysts call a case and whether it is open. One shared, in-memory copy for the whole page so the rail, the directory, the
// breadcrumb and the overview agree. A service that does not have the endpoint, or a failed read, leaves the map empty and every place shows the identifier exactly
// as before: the interface never depends on this.
let records = {}
let loaded = false
let inFlight = null
const listeners = new Set()

function publish(next) {
  records = next
  listeners.forEach(listener => listener(next))
}

export function loadCaseRecords({ force = false, fetchImpl } = {}) {
  if (loaded && !force) return Promise.resolve(records)
  if (inFlight && !force) return inFlight
  inFlight = listCaseRecords({ fetchImpl })
    .then(data => {
      const next = {}
      for (const item of data?.cases || []) if (item?.case_id) next[item.case_id] = item
      loaded = true
      publish(next)
      return next
    })
    .catch(() => records)
    .finally(() => { inFlight = null })
  return inFlight
}

export function resetCaseRecordsForTest() {
  records = {}
  loaded = false
  inFlight = null
  listeners.clear()
}

export function useCaseRecords() {
  const [snapshot, setSnapshot] = useState(records)
  useEffect(() => {
    listeners.add(setSnapshot)
    setSnapshot(records)
    loadCaseRecords()
    return () => { listeners.delete(setSnapshot) }
  }, [])
  const save = useCallback(async (caseId, patch) => {
    const saved = await updateCaseRecord({ caseId, patch })
    publish({ ...records, [caseId]: saved })
    return saved
  }, [])
  return { records: snapshot, save }
}
