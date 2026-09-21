import { useState, useEffect, useCallback } from 'react'
import { useParams, useOutletContext, useSearchParams } from 'react-router-dom'
import { agentCollectionsApi } from '../utils/api'
import ConfirmDialog from '../components/ConfirmDialog'
import PageHeader from '../components/PageHeader'
import { useActiveCase } from '../contexts/ActiveCaseContext'

export default function CollectionDetails() {
  const { name } = useParams()
  const { addToast } = useOutletContext()
  const [searchParams] = useSearchParams()
  const userId = searchParams.get('user_id') || undefined
  const { caseOptions, registryState, ensureCaseRegistry, setActiveCase } = useActiveCase()
  const authorizedCase = !userId ? caseOptions.find(item => item.collectionId === name) : null
  const [activeTab, setActiveTab] = useState('entries')
  const [loading, setLoading] = useState(true)
  const [confirmDialog, setConfirmDialog] = useState(null)

  // Entries tab state
  const [entries, setEntries] = useState([])
  const [uploadFile, setUploadFile] = useState(null)
  const [uploading, setUploading] = useState(false)
  const [uploadResult, setUploadResult] = useState(null)

  // Search tab state
  const [searchQuery, setSearchQuery] = useState('')
  const [searchMaxResults, setSearchMaxResults] = useState(10)
  const [searchResults, setSearchResults] = useState([])
  const [searchMeta, setSearchMeta] = useState(null)
  const [searching, setSearching] = useState(false)

  // Entry content modal state
  const [viewEntry, setViewEntry] = useState(null)
  const [viewContent, setViewContent] = useState(null)
  const [viewLoading, setViewLoading] = useState(false)

  // Sources tab state
  const [sources, setSources] = useState([])
  const [newSourceUrl, setNewSourceUrl] = useState('')
  const [newSourceInterval, setNewSourceInterval] = useState('')
  const [addingSource, setAddingSource] = useState(false)

  const fetchEntries = useCallback(async () => {
    try {
      const data = await agentCollectionsApi.entries(name, userId)
      setEntries(Array.isArray(data.entries) ? data.entries : [])
    } catch (err) {
      addToast(`Failed to load entries: ${err.message}`, 'error')
    }
  }, [name, addToast, userId])

  const fetchSources = useCallback(async () => {
    try {
      const data = await agentCollectionsApi.sources(name, userId)
      setSources(Array.isArray(data.sources) ? data.sources : [])
    } catch (err) {
      addToast(`Failed to load sources: ${err.message}`, 'error')
    }
  }, [name, addToast, userId])

  useEffect(() => {
    const load = async () => {
      setLoading(true)
      await Promise.allSettled([fetchEntries(), fetchSources()])
      setLoading(false)
    }
    load()
  }, [fetchEntries, fetchSources])

  useEffect(() => {
    if (!userId && registryState === 'idle') ensureCaseRegistry()
  }, [ensureCaseRegistry, registryState, userId])

  const handleViewContent = async (entry) => {
    setViewEntry(entry)
    setViewContent(null)
    setViewLoading(true)
    try {
      const data = await agentCollectionsApi.entryContent(name, entry, userId)
      setViewContent(data)
    } catch (err) {
      try {
        const data = await agentCollectionsApi.rawEntryContent(name, entry, userId)
        setViewContent(data)
      } catch (rawErr) {
        addToast(`Failed to load entry content: ${rawErr.message || err.message}`, 'error')
        setViewEntry(null)
      }
    } finally {
      setViewLoading(false)
    }
  }

  const handleDeleteEntry = async (entry) => {
    setConfirmDialog({
      title: 'Delete Entry',
      message: 'Are you sure you want to delete this entry?',
      confirmLabel: 'Delete',
      danger: true,
      onConfirm: async () => {
        setConfirmDialog(null)
        try {
          await agentCollectionsApi.deleteEntry(name, entry, userId)
          addToast('Entry deleted', 'success')
          fetchEntries()
        } catch (err) {
          addToast(`Failed to delete entry: ${err.message}`, 'error')
        }
      },
    })
  }

  const handleUpload = async (e) => {
    e.preventDefault()
    if (!uploadFile) return
    setUploading(true)
    try {
      const formData = new FormData()
      formData.append('file', uploadFile)
      const result = await agentCollectionsApi.upload(name, formData, userId)
      setUploadResult(result)
      if (result?.records_status === 'queued') {
        addToast('File uploaded and structured records ingest queued', 'success')
      } else if (result?.records_status === 'failed') {
        addToast('File uploaded to KB, but records ingest forwarding failed', 'warning')
      } else {
        addToast('File uploaded successfully', 'success')
      }
      setUploadFile(null)
      fetchEntries()
    } catch (err) {
      addToast(`Upload failed: ${err.message}`, 'error')
    } finally {
      setUploading(false)
    }
  }

  const handleSearch = async (e) => {
    e.preventDefault()
    if (!searchQuery.trim()) return
    setSearching(true)
    try {
      const data = await agentCollectionsApi.search(name, searchQuery, searchMaxResults, userId)
      setSearchResults(Array.isArray(data.results) ? data.results : [])
      setSearchMeta({
        count: data.count,
        requestedMaxResults: data.requested_max_results,
        effectiveMaxResults: data.effective_max_results,
      })
    } catch (err) {
      addToast(`Search failed: ${err.message}`, 'error')
    } finally {
      setSearching(false)
    }
  }

  const handleAddSource = async (e) => {
    e.preventDefault()
    if (!newSourceUrl.trim()) return
    setAddingSource(true)
    try {
      await agentCollectionsApi.addSource(name, newSourceUrl, newSourceInterval || undefined, userId)
      addToast('Source added', 'success')
      setNewSourceUrl('')
      setNewSourceInterval('')
      fetchSources()
    } catch (err) {
      addToast(`Failed to add source: ${err.message}`, 'error')
    } finally {
      setAddingSource(false)
    }
  }

  const handleRemoveSource = async (url) => {
    setConfirmDialog({
      title: 'Remove Source',
      message: 'Are you sure you want to remove this source?',
      confirmLabel: 'Remove',
      danger: true,
      onConfirm: async () => {
        setConfirmDialog(null)
        try {
          await agentCollectionsApi.removeSource(name, url, userId)
          addToast('Source removed', 'success')
          fetchSources()
        } catch (err) {
          addToast(`Failed to remove source: ${err.message}`, 'error')
        }
      },
    })
  }

  return (
    <div className="page page--narrow">
      <style>{`
        .collection-detail-upload-form {
          display: flex;
          align-items: center;
          gap: var(--spacing-sm);
          margin-bottom: var(--spacing-md);
          flex-wrap: wrap;
        }
        .collection-detail-upload-status {
          border: 1px solid var(--color-border);
          border-radius: var(--radius-md);
          padding: var(--spacing-md);
          margin-bottom: var(--spacing-md);
          background: var(--color-bg-secondary);
        }
        .collection-detail-upload-status-title {
          display: flex;
          align-items: center;
          gap: var(--spacing-xs);
          margin: 0 0 var(--spacing-xs);
          font-size: 0.9375rem;
          font-weight: 600;
          color: var(--color-text-primary);
        }
        .collection-detail-upload-status-grid {
          display: grid;
          grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
          gap: var(--spacing-sm);
          margin-top: var(--spacing-sm);
        }
        .collection-detail-upload-status-item {
          min-width: 0;
          font-size: 0.8125rem;
          color: var(--color-text-secondary);
        }
        .collection-detail-upload-status-item strong {
          display: block;
          margin-top: 2px;
          color: var(--color-text-primary);
          overflow-wrap: anywhere;
        }
        .collection-detail-upload-status-warning {
          margin-top: var(--spacing-sm);
          padding: var(--spacing-sm);
          border-radius: var(--radius-sm);
          background: rgba(245, 158, 11, 0.12);
          color: var(--color-warning, #92400e);
          font-size: 0.8125rem;
          overflow-wrap: anywhere;
        }
        .collection-detail-search-form {
          display: flex;
          align-items: flex-end;
          gap: var(--spacing-sm);
          margin-bottom: var(--spacing-md);
          flex-wrap: wrap;
        }
        .collection-detail-search-form .collection-detail-field {
          display: flex;
          flex-direction: column;
          gap: var(--spacing-xs);
        }
        .collection-detail-search-form .collection-detail-field label {
          font-size: 0.8125rem;
          font-weight: 500;
          color: var(--color-text-secondary);
        }
        .collection-detail-result-card {
          background: var(--color-bg-secondary);
          border-radius: var(--radius-md);
          padding: var(--spacing-md);
          margin-bottom: var(--spacing-sm);
        }
        .collection-detail-result-score {
          font-size: 0.75rem;
          font-weight: 600;
          color: var(--color-primary);
          margin-bottom: var(--spacing-xs);
        }
        .collection-detail-search-meta {
          display: flex;
          flex-wrap: wrap;
          gap: var(--spacing-xs);
          margin-bottom: var(--spacing-md);
          color: var(--color-text-secondary);
          font-size: 0.8125rem;
        }
        .collection-detail-search-meta span {
          border: 1px solid var(--color-border);
          border-radius: var(--radius-sm);
          padding: 4px 8px;
          background: var(--color-bg-secondary);
        }
        .collection-detail-result-content {
          font-size: 0.875rem;
          color: var(--color-text-primary);
          white-space: pre-wrap;
          word-break: break-word;
        }
        .collection-detail-source-form {
          display: flex;
          align-items: flex-end;
          gap: var(--spacing-sm);
          margin-bottom: var(--spacing-md);
          flex-wrap: wrap;
        }
        .collection-detail-source-form .collection-detail-field {
          display: flex;
          flex-direction: column;
          gap: var(--spacing-xs);
          flex: 1;
          min-width: 200px;
        }
        .collection-detail-source-form .collection-detail-field label {
          font-size: 0.8125rem;
          font-weight: 500;
          color: var(--color-text-secondary);
        }
        .collection-detail-entry-content {
          max-width: 400px;
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
          font-size: 0.8125rem;
          color: var(--color-text-secondary);
        }
        .collection-detail-empty {
          text-align: center;
          padding: var(--spacing-xl);
          color: var(--color-text-muted);
        }
        .collection-detail-modal-overlay {
          position: fixed;
          inset: 0;
          background: rgba(0, 0, 0, 0.5);
          display: flex;
          align-items: center;
          justify-content: center;
          z-index: 1000;
        }
        .collection-detail-modal {
          background: var(--color-bg-primary);
          border-radius: var(--radius-lg);
          width: 90%;
          max-width: 700px;
          max-height: 80vh;
          display: flex;
          flex-direction: column;
          box-shadow: 0 8px 32px rgba(0, 0, 0, 0.2);
        }
        .collection-detail-modal-header {
          display: flex;
          justify-content: space-between;
          align-items: center;
          padding: var(--spacing-md) var(--spacing-lg);
          border-bottom: 1px solid var(--color-border);
        }
        .collection-detail-modal-header h3 {
          margin: 0;
          font-size: 1rem;
          font-weight: 600;
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
        }
        .collection-detail-modal-body {
          padding: var(--spacing-lg);
          overflow-y: auto;
          flex: 1;
        }
        .collection-detail-modal-content {
          white-space: pre-wrap;
          word-break: break-word;
          font-family: var(--font-mono);
          font-size: 0.8125rem;
          background: var(--color-bg-secondary);
          border-radius: var(--radius-md);
          padding: var(--spacing-md);
          max-height: 50vh;
          overflow-y: auto;
        }
      `}</style>

      <PageHeader
        title={name}
        supporting="Administrative collection details, retrieval, sources, and lifecycle management."
        actions={authorizedCase ? (
          <button className="btn btn-primary" type="button" onClick={() => setActiveCase(authorizedCase.caseId, 'overview')}>
            <i className="fas fa-shield-halved" /> Open authorized case
          </button>
        ) : null}
      />

      <div className="alert alert-info" role="note">
        <i className="fas fa-circle-info" /> {authorizedCase
          ? `Authorized case mapping verified: ${authorizedCase.displayName} (${authorizedCase.caseId}).`
          : 'Administrative collection only. No authorized selectable case mapping is available for this collection.'}
      </div>

      <div className="tabs">
        <button className={`tab ${activeTab === 'entries' ? 'tab-active' : ''}`} onClick={() => setActiveTab('entries')}>
          <i className="fas fa-list" /> Entries
        </button>
        <button className={`tab ${activeTab === 'search' ? 'tab-active' : ''}`} onClick={() => setActiveTab('search')}>
          <i className="fas fa-search" /> Search
        </button>
        <button className={`tab ${activeTab === 'sources' ? 'tab-active' : ''}`} onClick={() => setActiveTab('sources')}>
          <i className="fas fa-globe" /> Sources
        </button>
      </div>
      <p style={{ color: 'var(--color-text-secondary)', fontSize: '0.875rem', marginTop: 0 }}>
        Collection search retrieves evidence and raw previews. {authorizedCase
          ? <>Use the explicit <button className="btn-link" type="button" onClick={() => setActiveCase(authorizedCase.caseId, 'analyze')}>authorized case analysis</button> for exact counts, filters, rankings, min/max, and joins over structured files.</>
          : <>Exact records analysis requires an authorized case mapping.</>}
      </p>

      {loading ? (
        <div style={{ display: 'flex', justifyContent: 'center', padding: 'var(--spacing-xl)' }}>
          <i className="fas fa-spinner fa-spin" style={{ fontSize: '2rem', color: 'var(--color-primary)' }} />
        </div>
      ) : activeTab === 'entries' ? (
        <>
          <form className="collection-detail-upload-form" onSubmit={handleUpload}>
            <input
              className="input"
              type="file"
              onChange={(e) => setUploadFile(e.target.files[0] || null)}
              style={{ flex: 1, minWidth: 200 }}
            />
            <button className="btn btn-primary" type="submit" disabled={!uploadFile || uploading}>
              {uploading ? <><i className="fas fa-spinner fa-spin" /> Uploading...</> : <><i className="fas fa-upload" /> Upload</>}
            </button>
          </form>

          {uploadResult && (
            <div className="collection-detail-upload-status" role="status">
              <p className="collection-detail-upload-status-title">
                <i className={uploadResult.records_status === 'failed' ? 'fas fa-triangle-exclamation' : 'fas fa-circle-check'} />
                Upload pipeline status
              </p>
              <div className="collection-detail-upload-status-grid">
                <div className="collection-detail-upload-status-item">
                  KB entry
                  <strong>{uploadResult.key || 'stored'}</strong>
                </div>
                <div className="collection-detail-upload-status-item">
                  File
                  <strong>{uploadResult.filename || uploadFile?.name || '-'}</strong>
                </div>
                <div className="collection-detail-upload-status-item">
                  Records pipeline
                  <strong>{uploadResult.records_status || 'not forwarded'}</strong>
                </div>
                {uploadResult.records_ingest?.job_id && (
                  <div className="collection-detail-upload-status-item">
                    Records job
                    <strong>{uploadResult.records_ingest.job_id}</strong>
                  </div>
                )}
                {uploadResult.records_ingest?.detected_record_type && (
                  <div className="collection-detail-upload-status-item">
                    Detected type
                    <strong>{uploadResult.records_ingest.detected_record_type}</strong>
                  </div>
                )}
              </div>
              {uploadResult.records_warning && (
                <div className="collection-detail-upload-status-warning">
                  {uploadResult.records_warning}
                </div>
              )}
            </div>
          )}

          {entries.length === 0 ? (
            <div className="collection-detail-empty">
              <i className="fas fa-inbox" style={{ fontSize: '2rem', marginBottom: 'var(--spacing-sm)', display: 'block' }} />
              <p>No entries in this collection. Upload a file to get started.</p>
            </div>
          ) : (
            <div className="table-container">
              <table className="table">
                <thead>
                  <tr>
                    <th>Entry</th>
                    <th style={{ textAlign: 'right' }}>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {entries.map((entry, index) => (
                    <tr key={index}>
                      <td>
                        <div className="collection-detail-entry-content">
                          {typeof entry === 'string' ? entry : JSON.stringify(entry)}
                        </div>
                      </td>
                      <td>
                        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 'var(--spacing-xs)' }}>
                          <button className="btn btn-secondary btn-sm" onClick={() => handleViewContent(entry)} title="View Content">
                            <i className="fas fa-eye" />
                          </button>
                          <button className="btn btn-danger btn-sm" onClick={() => handleDeleteEntry(entry)} title="Delete">
                            <i className="fas fa-trash" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      ) : activeTab === 'search' ? (
        <>
          <form className="collection-detail-search-form" onSubmit={handleSearch}>
            <div className="collection-detail-field" style={{ flex: 2, minWidth: 250 }}>
              <label htmlFor="search-query">Query</label>
              <input
                id="search-query"
                className="input"
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Enter search query..."
              />
            </div>
            <div className="collection-detail-field" style={{ flex: 0, minWidth: 100 }}>
              <label htmlFor="search-max">Max Results</label>
              <input
                id="search-max"
                className="input"
                type="number"
                min={1}
                max={100}
                value={searchMaxResults}
                onChange={(e) => setSearchMaxResults(parseInt(e.target.value, 10) || 10)}
                style={{ width: 100 }}
              />
            </div>
            <button className="btn btn-primary" type="submit" disabled={!searchQuery.trim() || searching}>
              {searching ? <><i className="fas fa-spinner fa-spin" /> Searching...</> : <><i className="fas fa-search" /> Search</>}
            </button>
          </form>

          {searchMeta && (
            <div className="collection-detail-search-meta" role="status">
              <span>Returned: {searchMeta.count ?? searchResults.length}</span>
              <span>Requested max: {searchMeta.requestedMaxResults ?? searchMaxResults}</span>
              <span>Effective max: {searchMeta.effectiveMaxResults ?? searchMeta.count ?? searchResults.length}</span>
              {Number(searchMeta.effectiveMaxResults) < Number(searchMeta.requestedMaxResults) && (
                <span>Auto-capped to available indexed chunks</span>
              )}
            </div>
          )}

          {searchResults.length === 0 ? (
            <div className="collection-detail-empty">
              <i className="fas fa-search" style={{ fontSize: '2rem', marginBottom: 'var(--spacing-sm)', display: 'block' }} />
              <p>No results. Enter a query and click Search.</p>
            </div>
          ) : (
            searchResults.map((result, index) => (
              <div className="collection-detail-result-card" key={index}>
                <div className="collection-detail-result-score">
                  Similarity: {typeof result.similarity === 'number' ? result.similarity.toFixed(4) : (result.score != null ? Number(result.score).toFixed(4) : 'N/A')}
                </div>
                <div className="collection-detail-result-content">
                  {result.content || result.text || (typeof result === 'string' ? result : JSON.stringify(result))}
                </div>
              </div>
            ))
          )}
        </>
      ) : (
        <>
          <form className="collection-detail-source-form" onSubmit={handleAddSource}>
            <div className="collection-detail-field">
              <label htmlFor="source-url">URL</label>
              <input
                id="source-url"
                className="input"
                type="text"
                value={newSourceUrl}
                onChange={(e) => setNewSourceUrl(e.target.value)}
                placeholder="https://example.com/data"
              />
            </div>
            <div className="collection-detail-field" style={{ flex: 0, minWidth: 160 }}>
              <label htmlFor="source-interval">Update Interval</label>
              <input
                id="source-interval"
                className="input"
                type="text"
                value={newSourceInterval}
                onChange={(e) => setNewSourceInterval(e.target.value)}
                placeholder="e.g. 1h, 30m"
              />
            </div>
            <button className="btn btn-primary" type="submit" disabled={!newSourceUrl.trim() || addingSource}>
              {addingSource ? <><i className="fas fa-spinner fa-spin" /> Adding...</> : <><i className="fas fa-plus" /> Add Source</>}
            </button>
          </form>

          {sources.length === 0 ? (
            <div className="collection-detail-empty">
              <i className="fas fa-globe" style={{ fontSize: '2rem', marginBottom: 'var(--spacing-sm)', display: 'block' }} />
              <p>No external sources configured. Add a URL to start ingesting data.</p>
            </div>
          ) : (
            <div className="table-container">
              <table className="table">
                <thead>
                  <tr>
                    <th>URL</th>
                    <th>Interval</th>
                    <th style={{ textAlign: 'right' }}>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {sources.map((source, index) => (
                    <tr key={index}>
                      <td style={{ fontSize: '0.8125rem', wordBreak: 'break-all' }}>
                        {typeof source === 'string' ? source : (source.url || JSON.stringify(source))}
                      </td>
                      <td style={{ fontSize: '0.8125rem', color: 'var(--color-text-secondary)' }}>
                        {(typeof source === 'object' && source.update_interval) ? source.update_interval : '-'}
                      </td>
                      <td>
                        <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
                          <button
                            className="btn btn-danger btn-sm"
                            onClick={() => handleRemoveSource(typeof source === 'string' ? source : source.url)}
                            title="Remove"
                          >
                            <i className="fas fa-trash" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}

      <ConfirmDialog
        open={!!confirmDialog}
        title={confirmDialog?.title}
        message={confirmDialog?.message}
        confirmLabel={confirmDialog?.confirmLabel}
        danger={confirmDialog?.danger}
        onConfirm={confirmDialog?.onConfirm}
        onCancel={() => setConfirmDialog(null)}
      />

      {/* Entry content modal */}
      {viewEntry && (
        <div className="collection-detail-modal-overlay" onClick={() => setViewEntry(null)}>
          <div className="collection-detail-modal" onClick={(e) => e.stopPropagation()}>
            <div className="collection-detail-modal-header">
              <h3 title={viewEntry}><i className="fas fa-file-alt" style={{ marginRight: 'var(--spacing-xs)' }} />{viewEntry}</h3>
              <button className="btn btn-secondary btn-sm" onClick={() => setViewEntry(null)}>
                <i className="fas fa-times" />
              </button>
            </div>
            <div className="collection-detail-modal-body">
              {viewLoading ? (
                <div style={{ textAlign: 'center', padding: 'var(--spacing-lg)' }}>
                  <i className="fas fa-spinner fa-spin" style={{ fontSize: '1.5rem', color: 'var(--color-primary)' }} />
                </div>
              ) : viewContent ? (
                <>
                  <div style={{ display: 'flex', gap: 'var(--spacing-md)', marginBottom: 'var(--spacing-md)' }}>
                    <div style={{ fontSize: '0.8125rem', color: 'var(--color-text-muted)' }}>
                      <i className="fas fa-puzzle-piece" style={{ marginRight: 4 }} />
                      Chunks: <strong style={{ color: 'var(--color-text-primary)' }}>{viewContent.chunk_count ?? '-'}</strong>
                    </div>
                  </div>
                  <div className="collection-detail-modal-content">
                    {viewContent.content || '(empty)'}
                  </div>
                </>
              ) : (
                <p style={{ color: 'var(--color-text-muted)' }}>No content available.</p>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
