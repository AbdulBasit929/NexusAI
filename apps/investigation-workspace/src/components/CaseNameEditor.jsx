import { useId, useState } from 'react'
import { Pencil } from 'lucide-react'
import { useCaseRecords } from '../lib/useCaseRecords.js'

const STATUS_LABELS = { open: 'Open', on_hold: 'On hold', closed: 'Closed' }

// Name a case and set whether it is open, on hold or closed. Saved through PATCH /cases/{id}; the answer replaces the shared copy so every place that shows the case updates.
// The identifier never changes: it scopes every request. The name is plain text, 1 to 120 characters.
export function CaseNameEditor({ caseId }) {
  const { records, save } = useCaseRecords()
  const record = records[caseId]
  const [editing, setEditing] = useState(false)
  const [name, setName] = useState('')
  const [status, setStatus] = useState('open')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const nameId = useId()
  const statusId = useId()

  function open() {
    setName(record?.display_name || '')
    setStatus(record?.status || 'open')
    setError('')
    setEditing(true)
  }

  async function submit(event) {
    event.preventDefault()
    setBusy(true)
    setError('')
    try {
      await save(caseId, { display_name: name.trim() === '' ? null : name.trim(), status })
      setEditing(false)
    } catch (failure) {
      setError(failure?.message || 'The name could not be saved. Nothing was changed.')
    } finally {
      setBusy(false)
    }
  }

  if (!editing) {
    return (
      <span className="case-name-editor">
        {record?.status && record.status !== 'open' ? <span className="case-name-editor__status">{STATUS_LABELS[record.status]}</span> : null}
        <button type="button" className="co-hero__copy-btn" onClick={open}>
          <Pencil size={13} aria-hidden="true" />
          <span>{record?.display_name ? 'Rename' : 'Name this case'}</span>
        </button>
      </span>
    )
  }
  return (
    <form className="case-name-editor case-name-editor--open" onSubmit={submit} aria-label="Name this case">
      <label htmlFor={nameId}>Case name</label>
      <input id={nameId} type="text" value={name} maxLength={120} onChange={event => setName(event.target.value)} autoFocus placeholder="A name people will recognise" />
      <label htmlFor={statusId}>Status</label>
      <select id={statusId} value={status} onChange={event => setStatus(event.target.value)}>
        {Object.entries(STATUS_LABELS).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
      </select>
      <button type="submit" disabled={busy}>{busy ? 'Saving' : 'Save'}</button>
      <button type="button" onClick={() => setEditing(false)} disabled={busy}>Cancel</button>
      {error ? <span role="alert" className="case-name-editor__error">{error}</span> : null}
    </form>
  )
}
