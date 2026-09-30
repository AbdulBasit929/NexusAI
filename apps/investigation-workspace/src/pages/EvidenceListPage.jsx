import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { AddDataDropzone } from '../components/AddDataDropzone.jsx'
import { CaseShell } from '../components/CaseShell.jsx'
import { listEvidence } from '../lib/apiClient.js'
import { formatBytes, formatNumber } from '../lib/format.js'
import { EmptyState, LanguageText, ProcessingBadge, RouteState } from '../components/AnalystComponents.jsx'
import { evidenceFamilies, familyDefinition, ScopeChips, useEvidenceScope } from '../components/ScopeChips.jsx'
import { curatedFamilyLabel } from '../lib/semanticCatalog.js'
import { PageHeader } from '../components/PageHeader.jsx'
import { SelectControl } from '../components/SelectControl.jsx'

const PAGE_SIZE = 25
const activeStatuses = new Set(['registered', 'queued', 'processing', 'running'])
const statusOptions = ['completed', 'processing', 'failed']

function validStatus(value) {
  return statusOptions.includes(value) ? value : 'all'
}

function validOffset(value) {
  const number = Number(value)
  return Number.isInteger(number) && number >= 0 ? number : 0
}

function statusLabel(value) {
  return ({ completed: 'Ready', processing: 'Processing', failed: 'Failed' })[value] || value
}

function evidenceTypeLabel(item) {
  if (item.detected_type) return curatedFamilyLabel(item.detected_type)
  const match = evidenceFamilies.find(family => family.modality === String(item.modality || '').toLowerCase())
  return match?.label || String(item.modality || 'Not reported')
}

function timestamp(value) {
  if (!value) return 'Not reported'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  return `${new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'short', timeZone: 'UTC' }).format(date)} UTC`
}

export default function EvidenceListPage() {
  const { id: caseId = '' } = useParams()
  const [searchParams, setSearchParams] = useSearchParams()
  const requestedScope = familyDefinition(searchParams.get('family') || 'all').id
  const [scope, setScope] = useEvidenceScope(caseId, 'evidence', requestedScope)
  const [status, setStatus] = useState(() => validStatus(searchParams.get('status')))
  const [searchText, setSearchText] = useState(() => searchParams.get('q') || '')
  const [query, setQuery] = useState(() => (searchParams.get('q') || '').trim())
  const [offset, setOffset] = useState(() => validOffset(searchParams.get('offset')))
  const [state, setState] = useState({ loading: true })
  const [refresh, setRefresh] = useState(0)
  const [intakeOpen, setIntakeOpen] = useState(false)
  const intakeRef = useRef(null)
  const family = familyDefinition(scope)

  useEffect(() => {
    const timer = globalThis.setTimeout(() => {
      const next = searchText.trim()
      if (next !== query) {
        setQuery(next)
        setOffset(0)
      }
    }, 350)
    return () => globalThis.clearTimeout(timer)
  }, [searchText, query])

  useEffect(() => {
    const next = new URLSearchParams()
    if (scope !== 'all') next.set('family', scope)
    if (status !== 'all') next.set('status', status)
    if (query) next.set('q', query)
    if (offset > 0) next.set('offset', String(offset))
    if (next.toString() !== searchParams.toString()) setSearchParams(next, { replace: true })
  }, [scope, status, query, offset, searchParams, setSearchParams])

  useEffect(() => {
    const controller = new AbortController()
    let pollTimer
    setState(current => ({ ...current, loading: true, error: null }))
    listEvidence({
      caseId,
      signal: controller.signal,
      filters: {
        limit: PAGE_SIZE,
        offset,
        q: query,
        modality: family.modality,
        detectedType: family.recordType,
        processingStatus: status === 'all' ? undefined : status,
      },
    })
      .then(data => {
        setState({ data, loading: false })
        if ((data?.items || []).some(item => activeStatuses.has(String(item.processing_status || '').toLowerCase()))) {
          pollTimer = globalThis.setTimeout(() => setRefresh(value => value + 1), 5000)
        }
      })
      .catch(error => error.name !== 'AbortError' && setState(current => ({ ...current, error, loading: false })))
    return () => { controller.abort(); globalThis.clearTimeout(pollTimer) }
  }, [caseId, family.modality, family.recordType, offset, query, refresh, status])

  const items = state.data?.items || []
  const summary = state.data?.summary || {}
  const pagination = state.data?.pagination || {}
  const total = Number(pagination.evidence_total ?? summary.evidence_total ?? 0)
  const returned = Number(pagination.returned ?? items.length)
  const start = returned > 0 ? offset + 1 : 0
  const end = offset + returned
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const hasFilters = scope !== 'all' || status !== 'all' || Boolean(query)

  const resultDescription = useMemo(() => {
    if (!total) return 'No evidence items in the current scope.'
    return `Showing ${formatNumber(start)}–${formatNumber(end)} of ${formatNumber(total)} evidence items.`
  }, [end, start, total])

  function chooseScope(value) {
    setScope(value)
    setOffset(0)
  }

  function chooseStatus(value) {
    setStatus(validStatus(value))
    setOffset(0)
  }

  function clearFilters() {
    setScope('all')
    setStatus('all')
    setSearchText('')
    setQuery('')
    setOffset(0)
  }

  function submitSearch(event) {
    event.preventDefault()
    setQuery(searchText.trim())
    setOffset(0)
  }

  function openIntake() {
    setIntakeOpen(true)
    globalThis.requestAnimationFrame(() => intakeRef.current?.scrollIntoView({ block: 'start', behavior: 'smooth' }))
  }

  return (
    <CaseShell caseId={caseId}>
      <main id="workspace-main" className="catalog-page evidence-catalog-page" tabIndex={-1}>
        <PageHeader
          eyebrow="Evidence catalog"
          title="Evidence"
          description="Find a source item, confirm its processing state, and open its governed record."
          actions={<button type="button" className="page-header__cta" onClick={openIntake}>Add evidence</button>}
        />

        <details className="evidence-intake-disclosure" open={intakeOpen} onToggle={event => setIntakeOpen(event.currentTarget.open)} ref={intakeRef}>
          <summary>Add evidence to this case</summary>
          <AddDataDropzone caseId={caseId} onAccepted={() => { setOffset(0); setRefresh(value => value + 1) }} />
        </details>

        <section className="evidence-inventory" aria-labelledby="evidence-inventory-title" aria-busy={state.loading ? 'true' : 'false'}>
          <header className="evidence-inventory__header">
            <div>
              <p className="eyebrow">Case inventory</p>
              <h2 id="evidence-inventory-title">Source evidence</h2>
              <p>{state.data ? resultDescription : 'Search and filters are applied by the evidence service.'}</p>
            </div>
            {state.data && total > 0 ? <p className="evidence-inventory__page">Page {formatNumber(currentPage)} of {formatNumber(totalPages)}</p> : null}
          </header>

          <div className="evidence-toolbar">
            <form className="evidence-search" role="search" onSubmit={submitSearch}>
              <label htmlFor="evidence-search-input">Search evidence</label>
              <div>
                <input id="evidence-search-input" type="search" value={searchText} onChange={event => setSearchText(event.target.value)} placeholder="Filename, evidence ID, hash, or metadata" />
                <button type="submit">Search</button>
              </div>
            </form>
            <SelectControl label="Processing state" value={status} options={statusOptions} onChange={chooseStatus} optionLabel={statusLabel} />
            {hasFilters ? <button type="button" className="evidence-toolbar__clear" onClick={clearFilters}>Clear filters</button> : null}
          </div>

          <ScopeChips value={scope} onChange={chooseScope} label="Filter evidence by family" />

          {state.data && total > 0 ? (
            <dl className="evidence-summary" aria-label="Filtered evidence summary">
              <div><dt>Total</dt><dd>{formatNumber(total)}</dd></div>
              <div><dt>Ready</dt><dd>{formatNumber(summary.completed || 0)}</dd></div>
              <div><dt>Queued</dt><dd>{formatNumber(summary.queued || 0)}</dd></div>
              <div><dt>Processing</dt><dd>{formatNumber(summary.processing || 0)}</dd></div>
              <div><dt>Failed</dt><dd>{formatNumber(summary.failed || 0)}</dd></div>
              <div><dt>Size</dt><dd>{formatBytes(summary.total_size_bytes)}</dd></div>
            </dl>
          ) : null}

          {state.loading && <RouteState state="loading" label="Loading evidence inventory" description="The current search, family, state, and page are being resolved by the evidence service." />}
          {state.error && <RouteState state={state.error.status === 403 ? 'forbidden' : 'error'} label={state.error.status === 403 ? 'Evidence access is forbidden' : 'Evidence could not be loaded'} reference={state.error.reference || 'EVIDENCE'} />}
          {!state.loading && !state.error && !items.length && (
            <EmptyState
              kind={hasFilters ? 'no-match' : 'not-processed'}
              label={hasFilters ? 'No evidence matches these filters' : 'No evidence has been added'}
              description={hasFilters ? 'The evidence service checked the selected scope. It was not silently widened.' : 'Add the first source file to begin processing this case.'}
            >
              {hasFilters ? <button type="button" onClick={clearFilters}>Clear filters</button> : <button type="button" onClick={openIntake}>Add evidence</button>}
            </EmptyState>
          )}

          {!state.loading && !state.error && items.length > 0 ? (
            <div className="evidence-table-wrap">
              <table className="evidence-table">
                <caption>{resultDescription} Newest source items are shown first.</caption>
                <thead>
                  <tr>
                    <th scope="col">Evidence</th>
                    <th scope="col">Family</th>
                    <th scope="col">Size</th>
                    <th scope="col">Added</th>
                    <th scope="col">Status</th>
                    <th scope="col" className="numeric">Accepted rows</th>
                    <th scope="col"><span className="visually-hidden">Action</span></th>
                  </tr>
                </thead>
                <tbody>
                  {items.map(item => {
                    const href = `/cases/${encodeURIComponent(caseId)}/evidence/${encodeURIComponent(item.evidence_id)}`
                    return (
                      <tr key={item.evidence_id}>
                        <th scope="row" data-label="Evidence">
                          <Link to={href}><LanguageText>{item.original_filename || item.source_file || item.evidence_id}</LanguageText></Link>
                          <small><span>Evidence ID</span> <LanguageText as="bdi" identifier>{item.evidence_id}</LanguageText></small>
                        </th>
                        <td data-label="Family">{evidenceTypeLabel(item)}</td>
                        <td data-label="Size" className="numeric">{formatBytes(item.size_bytes)}</td>
                        <td data-label="Added"><time dateTime={item.created_at || undefined}>{timestamp(item.created_at)}</time></td>
                        <td data-label="Status"><ProcessingBadge state={item.processing_status} /></td>
                        <td data-label="Accepted rows" className="numeric">{item.accepted_rows == null ? 'Not reported' : formatNumber(item.accepted_rows)}</td>
                        <td data-label="Action" className="evidence-table__action"><Link to={href}>Open evidence</Link></td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          ) : null}

          {!state.loading && !state.error && items.length > 0 ? (
            <nav className="evidence-pagination" aria-label="Evidence pages">
              <p>{resultDescription}</p>
              <div>
                <button type="button" disabled={!pagination.has_previous} onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>Previous</button>
                <button type="button" disabled={!pagination.has_next} onClick={() => setOffset(offset + PAGE_SIZE)}>Next</button>
              </div>
            </nav>
          ) : null}
        </section>
      </main>
    </CaseShell>
  )
}
