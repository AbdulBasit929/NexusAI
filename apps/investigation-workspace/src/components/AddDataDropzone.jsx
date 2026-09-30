import { useRef, useState } from 'react'
import { uploadEvidence } from '../lib/apiClient.js'
import { formatBytes } from '../lib/format.js'
import { LanguageText, ProcessingBadge } from './AnalystComponents.jsx'

// Transfer and processing are different truths. Only measured bytes receive a
// progress bar; processing begins after acceptance and has state, not percent.
const phaseLabel = {
  queued: 'Waiting to send',
  sending: 'Sending',
  accepted: 'Accepted — processing has started',
  refused: 'Not accepted',
  cancelled: 'Upload cancelled',
}

let intakeSequence = 0
function nextItemId(file) {
  intakeSequence += 1
  return `${file.name}:${file.size}:${file.lastModified ?? 0}:${intakeSequence}`
}

export function AddDataDropzone({ caseId, onAccepted, uploader = uploadEvidence }) {
  const [items, setItems] = useState([])
  const [dragging, setDragging] = useState(false)
  const [dropStatus, setDropStatus] = useState('')
  const inputRef = useRef(null)
  const controllers = useRef(new Map())
  const cancelled = useRef(new Set())

  function update(id, patch) {
    setItems(current => current.map(item => (item.id === id ? { ...item, ...patch } : item)))
  }

  function cancel(id) {
    cancelled.current.add(id)
    controllers.current.get(id)?.abort()
    update(id, { phase: 'cancelled' })
  }

  function remove(id) {
    setItems(current => current.filter(item => item.id !== id))
    cancelled.current.delete(id)
  }

  async function send(files) {
    const queued = Array.from(files).map(file => ({
      id: nextItemId(file),
      file,
      phase: 'queued',
      transferred: 0,
    }))
    if (!queued.length) return
    setItems(current => [...current, ...queued])
    setDropStatus(`${queued.length} ${queued.length === 1 ? 'file' : 'files'} added to the upload queue.`)

    // Sequential transfer avoids turning a large multi-file forensic intake
    // into an uncontrolled burst. Every row still has independent state and a
    // real abort controller.
    for (const item of queued) {
      if (cancelled.current.has(item.id)) continue
      const controller = new AbortController()
      controllers.current.set(item.id, controller)
      update(item.id, { phase: 'sending' })
      try {
        const body = await uploader({
          caseId,
          file: item.file,
          sourceEntry: item.file.name,
          signal: controller.signal,
          onProgress: fraction => update(item.id, { transferred: fraction }),
        })
        if (cancelled.current.has(item.id)) continue
        update(item.id, { phase: 'accepted', transferred: 1, evidenceId: body?.evidence_id })
        onAccepted?.(body)
      } catch (error) {
        if (error.name === 'AbortError') {
          update(item.id, { phase: 'cancelled' })
        } else {
          update(item.id, { phase: 'refused', reason: error.message, reference: error.reference })
        }
      } finally {
        controllers.current.delete(item.id)
      }
    }
  }

  function onDrop(event) {
    event.preventDefault()
    setDragging(false)
    setDropStatus('Files dropped into the upload area.')
    send(event.dataTransfer?.files || [])
  }

  return (
    <section id="add-evidence" className="intake" aria-labelledby="intake-heading">
      <div className="intake__heading">
        <p className="section-kicker">Step 2</p>
        <h2 id="intake-heading">Add the first evidence</h2>
        <p>The case begins when the service accepts its first file.</p>
      </div>
      <div
        className={`intake__target${dragging ? ' intake__target--active' : ''}`}
        onDragEnter={event => { event.preventDefault(); setDragging(true); setDropStatus('Entered the evidence drop area.') }}
        onDragOver={event => event.preventDefault()}
        onDragLeave={event => {
          if (event.currentTarget.contains(event.relatedTarget)) return
          setDragging(false)
          setDropStatus('Left the evidence drop area.')
        }}
        onDrop={onDrop}
      >
        <svg viewBox="0 0 32 32" aria-hidden="true" focusable="false"><path d="M16 21V7m0 0-5 5m5-5 5 5" /><path d="M7 19v6h18v-6" /></svg>
        <div>
          <strong>Choose evidence files</strong>
          <p><span className="intake__drop-instruction">Drop files here, or </span>select them from this device.</p>
        </div>
        <button type="button" onClick={() => inputRef.current?.click()}>Choose files</button>
        <input
          ref={inputRef}
          className="intake__input"
          type="file"
          multiple
          aria-label="Choose evidence files to add to this case"
          onChange={event => { send(event.target.files); event.target.value = '' }}
        />
      </div>
      <p className="visually-hidden" role="status" aria-live="polite" aria-atomic="true">{dropStatus}</p>

      {items.length > 0 && (
        <div className="intake__queue" aria-labelledby="intake-queue-title">
          <div className="intake__queue-heading">
            <h3 id="intake-queue-title">Upload queue</h3>
            <span>{items.length} {items.length === 1 ? 'file' : 'files'}</span>
          </div>
          <ul className="intake__list">
            {items.map(item => {
              const percent = Math.max(0, Math.min(100, Math.round(item.transferred * 100)))
              return (
                <li key={item.id} className={`intake__item intake__item--${item.phase}`}>
                  <span className="intake__name">
                    <strong><LanguageText>{item.file.name}</LanguageText></strong>
                    <small>{formatBytes(item.file.size)}</small>
                  </span>
                  <span className="intake__phase">
                    <strong>{phaseLabel[item.phase]}</strong>
                    {item.phase === 'sending' && (
                      <>
                        <progress max={100} value={percent} aria-label={`Sending ${item.file.name}`} aria-valuetext={`${percent}% transferred`} />
                        <small className="intake__progress-text" aria-live="polite">{percent}% transferred</small>
                      </>
                    )}
                    {item.phase === 'refused' && <small className="intake__reason">{item.reason}{item.reference ? ` Reference: ${item.reference}` : ''}</small>}
                    {item.phase === 'cancelled' && <small>The file was not accepted in this attempt.</small>}
                    {item.phase === 'accepted' && <ProcessingBadge state="processing" />}
                  </span>
                  <span className="intake__actions">
                    {['queued', 'sending'].includes(item.phase) && <button type="button" onClick={() => cancel(item.id)}>Cancel <span className="visually-hidden">{item.file.name}</span></button>}
                    {['refused', 'cancelled'].includes(item.phase) && <button type="button" onClick={() => remove(item.id)}>Remove <span className="visually-hidden">{item.file.name}</span></button>}
                  </span>
                </li>
              )
            })}
          </ul>
        </div>
      )}

      <p className="intake__note" role="status">
        {items.some(item => item.phase === 'accepted')
          ? 'Accepted evidence is being processed. It becomes answerable once processing completes.'
          : 'Evidence is answerable only after the service finishes processing it.'}
      </p>
    </section>
  )
}
