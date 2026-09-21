import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { recordsApi } from '../utils/api'

const ActiveCaseContext = createContext(null)

function decodePathSegment(value) {
  try { return decodeURIComponent(value) } catch { return value }
}

function pathCaseId(pathname) {
  const match = pathname.match(/^\/app\/cases\/([^/]+)(?:\/|$)/)
  return match ? decodePathSegment(match[1]).trim() : ''
}

function toActiveCase(item) {
  if (!item?.case_id) return null
  return {
    ...item,
    caseId: item.case_id,
    collectionId: item.collection_id || item.case_id,
    displayName: item.display_name || item.case_id,
    access: 'authorized',
  }
}

function selectableCases(cases) {
  return cases.filter(item => item?.case_id && item.selectable)
}

function resolveDefaultCase(registry) {
  const selectable = selectableCases(registry.cases)
  return selectable.find(item => item.case_id === registry.defaultCaseId) || selectable[0] || null
}

export function ActiveCaseProvider({ children }) {
  const location = useLocation()
  const navigate = useNavigate()
  const [registry, setRegistry] = useState({ status: 'idle', cases: [], defaultCaseId: '', error: '' })
  const [currentCaseId, setCurrentCaseId] = useState('')
  const requestRef = useRef(null)
  const requestVersionRef = useRef(0)

  const explicitPathCaseId = useMemo(() => pathCaseId(location.pathname), [location.pathname])
  const explicitQueryCaseId = useMemo(() => {
    if (explicitPathCaseId) return ''
    return (new URLSearchParams(location.search).get('case') || '').trim()
  }, [explicitPathCaseId, location.search])
  const explicitCaseId = explicitPathCaseId || explicitQueryCaseId
  const resolvesDefault = location.pathname.replace(/\/$/, '') === '/app/records' || location.pathname.startsWith('/analyst')
  const needsRegistry = Boolean(explicitCaseId || resolvesDefault)

  const loadRegistry = useCallback((force = false) => {
    if (requestRef.current && !force) return requestRef.current
    const requestVersion = ++requestVersionRef.current
    setRegistry(previous => ({ ...previous, status: 'loading', error: '' }))
    const request = recordsApi.forensicCases()
      .then(data => {
        if (requestVersion !== requestVersionRef.current) return data
        setRegistry({
          status: 'ready',
          cases: Array.isArray(data?.cases) ? data.cases : [],
          defaultCaseId: data?.default_case_id || '',
          error: '',
        })
        return data
      })
      .catch(error => {
        if (requestVersion === requestVersionRef.current) {
          setRegistry({ status: 'error', cases: [], defaultCaseId: '', error: error.message })
        }
        throw error
      })
      .finally(() => {
        if (requestVersion === requestVersionRef.current) requestRef.current = null
      })
    // State owns background-load errors; this terminal handler prevents an
    // unhandled rejection when no component explicitly awaits the request.
    request.catch(() => {})
    requestRef.current = request
    return request
  }, [])

  useEffect(() => {
    if (needsRegistry && registry.status === 'idle') loadRegistry()
  }, [loadRegistry, needsRegistry, registry.status])

  const explicitItem = useMemo(
    () => explicitCaseId ? registry.cases.find(item => item.case_id === explicitCaseId) || null : null,
    [explicitCaseId, registry.cases],
  )
  const defaultItem = useMemo(() => resolveDefaultCase(registry), [registry])
  const currentItem = useMemo(
    () => currentCaseId ? registry.cases.find(item => item.case_id === currentCaseId) || null : null,
    [currentCaseId, registry.cases],
  )

  useEffect(() => {
    if (registry.status !== 'ready') return
    const resolved = explicitItem || (resolvesDefault ? (currentItem || defaultItem) : null)
    if (resolved && resolved.case_id !== currentCaseId) setCurrentCaseId(resolved.case_id)
  }, [currentCaseId, currentItem, defaultItem, explicitItem, registry.status, resolvesDefault])

  let state = 'idle'
  let error = ''
  let source = currentItem ? 'current' : 'none'
  let item = currentItem

  if (needsRegistry && (registry.status === 'idle' || registry.status === 'loading')) {
    state = 'loading'
    item = null
    source = explicitPathCaseId ? 'path' : explicitQueryCaseId ? 'query' : 'default'
  } else if (needsRegistry && registry.status === 'error') {
    state = 'error'
    error = registry.error || 'The governed case registry is unavailable.'
    item = null
    source = explicitPathCaseId ? 'path' : explicitQueryCaseId ? 'query' : 'default'
  } else if (explicitCaseId) {
    source = explicitPathCaseId ? 'path' : 'query'
    item = explicitItem
    if (item) state = 'ready'
    else {
      state = 'inaccessible'
      error = `Case ${explicitCaseId} is not an authorized accessible case.`
    }
  } else if (resolvesDefault) {
    source = currentItem ? 'current' : 'default'
    item = currentItem || defaultItem
    if (item) state = 'ready'
    else {
      state = 'inaccessible'
      error = 'No selectable forensic case is available.'
    }
  } else if (item) {
    state = 'ready'
  }

  const activeCase = toActiveCase(item)
  const defaultCase = toActiveCase(defaultItem)
  const cases = useMemo(() => registry.cases.map(toActiveCase).filter(Boolean), [registry.cases])
  const caseOptions = useMemo(() => selectableCases(cases), [cases])

  const setActiveCase = useCallback((caseId, destination = 'overview') => {
    const next = registry.cases.find(candidate => candidate.case_id === caseId && candidate.selectable)
    if (!next) return false
    setCurrentCaseId(next.case_id)
    navigate(`/app/cases/${encodeURIComponent(next.case_id)}/${encodeURIComponent(destination || 'overview')}`)
    return true
  }, [navigate, registry.cases])

  const refreshActiveCase = useCallback(() => {
    requestRef.current = null
    return loadRegistry(true)
  }, [loadRegistry])

  const value = {
    activeCase,
    defaultCase,
    cases,
    caseOptions,
    registryState: registry.status,
    state,
    error,
    source,
    requestedCaseId: explicitCaseId,
    setActiveCase,
    refreshActiveCase,
    ensureCaseRegistry: loadRegistry,
  }

  return <ActiveCaseContext.Provider value={value}>{children}</ActiveCaseContext.Provider>
}

export function useActiveCase() {
  const context = useContext(ActiveCaseContext)
  if (!context) throw new Error('useActiveCase must be used within ActiveCaseProvider')
  return context
}
