import { useEffect, useRef, useState } from 'react'
import { getEngineHealth, getTypingReport } from '../../../lib/apiClient.js'

// The reads behind the dashboard that are not part of a case's status: how the engine has done on the questions, and what the typed records hold. Each is read once for each case, a few
// at a time (the typing report is a heavy query), kept for a few minutes so coming back to the dashboard does not read them again, and drawn as they arrive.
// A case that cannot be read is named in `failed` and has no entry; it is never read as zero.
const cache = new Map()

export function resetDashboardReads() { cache.clear() }

function cached(kind, caseId) { return cache.get(`${kind}|${caseId}`) }

function useReadsAcross({ kind, caseIds, read, enabled = true, concurrency = 2, ttl = 5 * 60_000, refreshToken = 0 }) {
  const key = caseIds.join('|')
  const [state, setState] = useState(() => ({ items: Object.fromEntries(caseIds.flatMap(caseId => (cached(kind, caseId) ? [[caseId, cached(kind, caseId).value]] : []))), pending: false, failed: [] }))
  const lastToken = useRef(refreshToken)

  useEffect(() => {
    if (!enabled || !caseIds.length) return undefined
    const forced = lastToken.current !== refreshToken
    lastToken.current = refreshToken
    const now = Date.now()
    const todo = caseIds.filter(caseId => forced || !cached(kind, caseId) || now - cached(kind, caseId).at > ttl)
    setState(current => ({ items: Object.fromEntries(caseIds.flatMap(caseId => (cached(kind, caseId) ? [[caseId, cached(kind, caseId).value]] : (caseId in current.items ? [[caseId, current.items[caseId]]] : [])))), pending: todo.length > 0, failed: [] }))
    if (!todo.length) return undefined

    const controller = new AbortController()
    let next = 0
    let running = 0
    const failed = []
    const finish = () => { if (!controller.signal.aborted && next >= todo.length && running === 0) setState(current => ({ ...current, pending: false, failed })) }
    const start = () => {
      while (running < concurrency && next < todo.length && !controller.signal.aborted) {
        const caseId = todo[next]
        next += 1
        running += 1
        read({ caseId, signal: controller.signal })
          .then(value => {
            if (controller.signal.aborted) return
            cache.set(`${kind}|${caseId}`, { at: Date.now(), value })
            setState(current => ({ ...current, items: { ...current.items, [caseId]: value } }))
          })
          .catch(error => { if (error?.name !== 'AbortError') failed.push(caseId) })
          .finally(() => { running -= 1; start(); finish() })
      }
    }
    start()
    return () => controller.abort()
    // the joined key stands in for the array so an equal list does not read again
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, enabled, refreshToken, kind])

  return state
}

// Each read carries the case it belongs to, so a selector can pair it with the case's status.
const tagged = (caseId, value) => (value && typeof value === 'object' ? { collection_id: caseId, ...value } : value)

export function useEngineHealthAcross(caseIds, { days = 7, enabled = true, refreshToken = 0 } = {}) {
  return useReadsAcross({ kind: `engine-${days}`, caseIds, enabled, refreshToken, concurrency: 6, ttl: 2 * 60_000, read: ({ caseId, signal }) => getEngineHealth({ caseId, days, signal }).then(value => tagged(caseId, value)) })
}

export function useTypingReportsAcross(caseIds, { enabled = true, refreshToken = 0 } = {}) {
  return useReadsAcross({ kind: 'typing', caseIds, enabled, refreshToken, concurrency: 2, ttl: 10 * 60_000, read: ({ caseId, signal }) => getTypingReport({ caseId, signal }).then(value => tagged(caseId, value)) })
}

// For the cases that were asked something this week: the engine's counts for the last 1, 2 ... 7 days, so the days can be told apart (model.dailyQuestions).
export function useEngineDailyAcross(caseIds, { enabled = true, refreshToken = 0 } = {}) {
  return useReadsAcross({
    kind: 'engine-daily', caseIds, enabled, refreshToken, concurrency: 2, ttl: 2 * 60_000,
    read: async ({ caseId, signal }) => ({ collection_id: caseId, windows: await Promise.all([1, 2, 3, 4, 5, 6, 7].map(days => getEngineHealth({ caseId, days, signal }).then(value => value?.totals || {}))) }),
  })
}
