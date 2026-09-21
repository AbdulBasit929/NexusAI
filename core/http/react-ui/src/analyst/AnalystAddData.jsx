import { useEffect, useMemo, useRef, useState } from 'react'
import { agentCollectionsApi, recordsApi } from '../utils/api'
import { intakeResponseState, sourceProductState } from './analystPresentation'
import { uploadPreflight } from '../utils/uploadPolicy'
import { intakeCapabilitySummary } from './analystIntakePolicy'
import { evidenceModalityIcon } from '../utils/evidenceModality'

const MAX_SESSION_FILES = 20

function fileKey(file) {
  return `${file.name}:${file.size}:${file.lastModified}`
}

function formatBytes(value) {
  const bytes = Number(value) || 0
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const unit = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / (1024 ** unit)).toLocaleString(undefined, { maximumFractionDigits: 1 })} ${units[unit]}`
}

function resultLabel(item) {
  const labels = {
    attention: 'Needs attention', cancelled: 'Cancelled', duplicate: 'Already added', failed: 'Failed', processing: 'Processing', ready: 'Ready', registering: 'Registering', uploading: 'Uploading', waiting: 'Waiting',
  }
  return labels[item.status] || 'Waiting'
}

export default function AnalystAddData({ caseId, collectionId, capabilities, initialFiles = [], onClose, onComplete, onOpenSource }) {
  const [items, setItems] = useState(() => Array.from(initialFiles || []).slice(0, MAX_SESSION_FILES).map(file => ({
    key: fileKey(file), file, name: file.name, size: file.size, status: 'waiting', progress: null, ...uploadPreflight(file),
  })))
  const [running, setRunning] = useState(false)
  const [dragging, setDragging] = useState(false)
  const [notice, setNotice] = useState('')
  const inputRef = useRef(null)
  const closeRef = useRef(null)
  const dialogRef = useRef(null)
  const pollingRef = useRef(false)
  const controllersRef = useRef(new Map())
  const cancelledRef = useRef(new Set())

  const blocked = useMemo(() => items.filter(item => item.blocked), [items])
  const pending = useMemo(() => items.filter(item => item.status === 'waiting'), [items])
  const finished = items.length > 0 && pending.length === 0 && !running

  const stage = fileList => {
    const incoming = Array.from(fileList || [])
    setItems(current => {
      const known = new Set(current.map(item => item.key))
      const available = Math.max(0, MAX_SESSION_FILES - current.length)
      const additions = incoming.filter(file => !known.has(fileKey(file))).slice(0, available).map(file => {
        const state = uploadPreflight(file)
        return { key: fileKey(file), file, name: file.name, size: file.size, status: 'waiting', progress: null, ...state }
      })
      const ignored = incoming.length - additions.length
      setNotice(ignored > 0 ? `${ignored} duplicate selection${ignored === 1 ? ' was' : 's were'} ignored or exceeded the ${MAX_SESSION_FILES}-file session limit.` : '')
      return [...current, ...additions]
    })
  }

  const update = (key, patch) => setItems(current => current.map(item => item.key === key ? { ...item, ...patch } : item))

  const upload = async () => {
    if (!pending.length || blocked.length || running) return
    setRunning(true)
    setNotice('')
    let cursor = 0
    const queue = pending.map(item => ({ ...item, status: 'waiting', progress: null, message: 'Waiting for an intake slot.' }))
    setItems(current => current.map(item => item.status === 'waiting' ? { ...item, progress: null, message: 'Waiting for an intake slot.' } : item))
    const workers = Array.from({ length: Math.min(2, queue.length) }, async () => {
      while (cursor < queue.length) {
        const index = cursor
        cursor += 1
        const item = queue[index]
        if (cancelledRef.current.has(item.key)) continue
        update(item.key, { status: 'uploading', progress: 0, message: 'Sending the original file securely.' })
        const controller = new AbortController()
        controllersRef.current.set(item.key, controller)
        try {
          const form = new FormData()
          form.append('file', item.file)
          form.append('case_id', caseId)
          form.append('evidence_role', 'source')
          form.append('jurisdiction', 'PK')
          form.append('source_timezone_state', 'unknown')
          form.append('source_date_order_state', 'unresolved')
          const response = await agentCollectionsApi.uploadWithProgress(collectionId || caseId, form, progress => update(item.key, {
            status: progress >= 100 ? 'registering' : 'uploading',
            progress,
            message: progress >= 100 ? 'Server admission and classification in progress.' : 'Sending the original file securely.',
          }), undefined, { signal: controller.signal })
          if (cancelledRef.current.has(item.key)) {
            update(item.key, { status: 'attention', progress: null, message: 'Cancellation raced with registration. Check Data before retrying.' })
            continue
          }
          const outcome = intakeResponseState(response)
          update(item.key, { status: outcome.id, progress: null, message: outcome.message, evidenceId: outcome.evidenceId })
        } catch (error) {
          const cancelled = error.name === 'AbortError' || cancelledRef.current.has(item.key)
          update(item.key, { status: cancelled ? 'cancelled' : 'failed', progress: null, message: cancelled ? 'Upload cancelled. Check Data before retrying because server receipt may be uncertain.' : error.message || 'The workspace could not register this source.' })
        } finally {
          controllersRef.current.delete(item.key)
        }
      }
    })
    await Promise.all(workers)
    setRunning(false)
    onComplete?.()
  }

  const cancel = item => {
    cancelledRef.current.add(item.key)
    controllersRef.current.get(item.key)?.abort()
    update(item.key, { status: 'cancelled', progress: null, message: item.status === 'waiting' ? 'Removed from this upload queue.' : 'Cancelling upload. Check Data before retrying if registration had started.' })
  }

  const retry = item => {
    cancelledRef.current.delete(item.key)
    update(item.key, { status: 'waiting', progress: null, message: 'Selected, not uploaded. Click Add selected data to begin.', evidenceId: '' })
  }

  useEffect(() => {
    closeRef.current?.focus()
    const handleKey = event => {
      if (event.key === 'Escape' && !running) onClose()
      if (event.key !== 'Tab') return
      const focusable = Array.from(dialogRef.current?.querySelectorAll('button:not(:disabled), input:not(:disabled), select:not(:disabled), [href]') || []).filter(element => element.offsetParent !== null)
      if (!focusable.length) return
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
      if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
    }
    window.addEventListener('keydown', handleKey)
    return () => window.removeEventListener('keydown', handleKey)
  }, [onClose, running])

  useEffect(() => {
    const processing = items.filter(item => item.status === 'processing' && item.evidenceId)
    if (!processing.length) return undefined
    const poll = async () => {
      if (pollingRef.current) return
      pollingRef.current = true
      try {
        const results = await Promise.all(processing.map(async item => {
          try {
            const detail = await recordsApi.forensicCaseEvidenceDetail(caseId, item.evidenceId, { limit: 1 })
            return { key: item.key, product: sourceProductState(detail?.item), detail }
          } catch {
            return null
          }
        }))
        let changed = false
        results.filter(Boolean).forEach(result => {
          if (result.product.id !== 'processing') {
            changed = true
            update(result.key, { status: result.product.id, message: result.product.description })
          }
        })
        if (changed) onComplete?.()
      } finally {
        pollingRef.current = false
      }
    }
    const timer = window.setInterval(poll, 2500)
    return () => window.clearInterval(timer)
  }, [caseId, items, onComplete])

  return (
    <div className="analyst-add-data-backdrop" role="presentation" onMouseDown={event => { if (event.target === event.currentTarget && !running) onClose() }}>
      <section ref={dialogRef} className="analyst-add-data" role="dialog" aria-modal="true" aria-labelledby="analyst-add-data-title">
        <header><div><span className="analyst-eyebrow">Unified data intake</span><h2 id="analyst-add-data-title">Add data</h2><p>Each source will be preserved, inspected, classified, and prepared automatically.</p></div><button ref={closeRef} type="button" onClick={onClose} disabled={running} aria-label="Close Add Data"><i className="fas fa-xmark" /></button></header>
        <div className={`analyst-dropzone${dragging ? ' dragging' : ''}`} onDragEnter={event => { event.preventDefault(); setDragging(true) }} onDragOver={event => event.preventDefault()} onDragLeave={() => setDragging(false)} onDrop={event => { event.preventDefault(); setDragging(false); stage(event.dataTransfer.files) }}>
          <i className="fas fa-cloud-arrow-up" aria-hidden="true" />
          <strong>Drop files here</strong>
          <span>or choose one or several files from this device</span>
          <button type="button" onClick={() => inputRef.current?.click()}>Choose files</button>
          <input ref={inputRef} className="sr-only" type="file" multiple onChange={event => { stage(event.target.files); event.target.value = '' }} aria-label="Choose files to add" />
          <small>The server—not the filename or browser MIME type—makes the final security and classification decision.</small>
          <small>{intakeCapabilitySummary(capabilities)}</small>
        </div>
        <p className="analyst-intake-notice" role="status">{running
          ? 'Keep this page open during upload and registration. Waiting files will start automatically when an intake slot is free.'
          : pending.length
            ? 'Waiting means selected, not uploaded. Click Add selected data to start; closing now discards this selection.'
            : 'Once upload and registration finish, server processing continues after this dialog is closed.'}</p>
        {notice && <p className="analyst-intake-notice" role="status">{notice}</p>}
        {items.length > 0 && <div className="analyst-intake-ledger" aria-label="Files selected for intake" aria-live="polite">
          {items.map(item => <article key={item.key} data-status={item.status}>
            <span className="analyst-intake-file"><i className={`fas ${evidenceModalityIcon({ original_filename: item.name })}`} /><span><strong title={item.name}>{item.name}</strong><small>{formatBytes(item.size)} · {item.message}</small></span></span>
            <span className="analyst-intake-state"><strong>{resultLabel(item)}</strong>{Number.isFinite(item.progress) && <progress max="100" value={item.progress}>{item.progress}%</progress>}{item.evidenceId && !running && <button type="button" onClick={() => onOpenSource(item.evidenceId, item.name)}>Open source</button>}{['waiting', 'uploading', 'registering'].includes(item.status) && <button type="button" onClick={() => cancel(item)} aria-label={`Cancel ${item.name}`}>Cancel</button>}{['cancelled', 'failed'].includes(item.status) && !running && <button type="button" onClick={() => retry(item)} aria-label={`Retry ${item.name}`}>Retry</button>}{item.status === 'waiting' && !running && <button type="button" onClick={() => setItems(current => current.filter(candidate => candidate.key !== item.key))} aria-label={`Remove ${item.name}`}><i className="fas fa-xmark" /></button>}</span>
          </article>)}
        </div>}
        {blocked.length > 0 && <div className="analyst-inline-error" role="alert">Remove the blocked file before continuing. Server limits remain authoritative.</div>}
        <footer><span><i className="fas fa-lock" /> Scope locked to this workspace · maximum two concurrent registrations</span>{finished ? <button className="analyst-secondary-action" type="button" onClick={() => inputRef.current?.click()}><i className="fas fa-plus" /> Choose more files</button> : <button className="analyst-primary-action" type="button" onClick={upload} disabled={!pending.length || blocked.length > 0 || running}>{running ? <><i className="fas fa-spinner fa-spin" /> Adding data</> : <><i className="fas fa-shield-halved" /> Add selected data</>}</button>}</footer>
      </section>
    </div>
  )
}
