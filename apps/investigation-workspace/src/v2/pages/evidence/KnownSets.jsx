import { useEffect, useState } from 'react'
import { Flag, Upload } from 'lucide-react'
import { addKnownHashes, createKnownSet, getKnownFlags, retireKnownSet } from '../../../lib/apiClient.js'
import { Dialog } from '../../ui/Dialog.jsx'
import { addedSentence, flagsByEvidence, KIND_LABEL } from './knownSetsModel.js'

const MAX_TABLE_BYTES = 4 * 1024 * 1024

// The flags of the case, read once per visit of the page (and again after a change): { status: 'loading' | 'ready' | 'unavailable', data, byEvidence }.
// "unavailable" covers a server without known sets (404) and any failure; the page then simply leaves the feature out.
export function useKnownFlags(caseId, tick) {
  const [state, setState] = useState({ status: 'loading' })
  const [again, setAgain] = useState(0)
  useEffect(() => {
    const controller = new AbortController()
    getKnownFlags({ caseId, signal: controller.signal })
      .then(data => { if (!controller.signal.aborted) setState(data ? { status: 'ready', data, byEvidence: flagsByEvidence(data) } : { status: 'unavailable' }) })
      .catch(error => { if (error?.name !== 'AbortError') setState({ status: 'unavailable' }) })
    return () => controller.abort()
  }, [caseId, tick, again])
  return [state, () => setAgain(value => value + 1)]
}

// Known sets: names, kinds, and the three ways to put pictures in one (the pictures selected on the page, a table of hashes, a new set made from the selection). Only hashes are kept.
export function KnownSetsPanel({ caseId, sets, selected, onChanged, onClose }) {
  const picked = [...selected]
  const [name, setName] = useState('')
  const [kind, setKind] = useState('of_interest')
  const [target, setTarget] = useState(sets[0]?.set_id || '')
  const [csv, setCsv] = useState('')
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState(null)
  const [confirming, setConfirming] = useState('')

  useEffect(() => { if (!sets.some(set => set.set_id === target)) setTarget(sets[0]?.set_id || '') }, [sets, target])

  const run = async work => {
    setBusy(true)
    setMessage(null)
    try {
      const text = await work()
      setMessage({ tone: 'ok', text })
      await onChanged()
      return true
    } catch (error) {
      setMessage({ tone: 'error', text: error?.status === 409 ? error.message : `${error?.message || 'That did not work.'}${error?.reference ? ` (${error.reference})` : ''}` })
      return false
    } finally {
      setBusy(false)
    }
  }
  const createAndAdd = async event => {
    event.preventDefault()
    const clean = name.trim()
    if (!clean) return
    const ok = await run(async () => {
      const created = await createKnownSet({ caseId, name: clean, kind })
      if (!picked.length) return `Set "${clean}" created.`
      const result = await addKnownHashes({ caseId, setId: created.set.set_id, evidenceIds: picked })
      return `Set "${clean}" created. ${addedSentence(result)}`
    })
    if (ok) setName('')
  }
  const addSelected = () => run(async () => addedSentence(await addKnownHashes({ caseId, setId: target, evidenceIds: picked })))
  const importTable = async () => {
    const ok = await run(async () => addedSentence(await addKnownHashes({ caseId, setId: target, csv })))
    if (ok) setCsv('')
  }
  const readFile = async event => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    if (file.size > MAX_TABLE_BYTES) { setMessage({ tone: 'error', text: 'That table is larger than 4 MB. Split it.' }); return }
    setCsv(await file.text())
  }
  const retire = set => run(async () => {
    await retireKnownSet({ caseId, setId: set.set_id })
    setConfirming('')
    return `"${set.name}" retired. Its hashes are kept; it no longer flags anything.`
  })

  return (
    <Dialog title="Known sets" wide onClose={onClose}>
      <div className="nx-ks">
        <p className="nx-ks__lede">Pictures you know in advance, kept as <b>hashes only</b>. A set never stores the pictures. A picture of this case that is the same file, or the same picture saved or resized, is flagged; nothing is deleted.</p>

        <section aria-label="Your sets" className="nx-ks__block">
          <h3>Your sets</h3>
          {sets.length ? (
            <ul className="nx-ks__sets">
              {sets.map(set => (
                <li key={set.set_id} className="nx-ks__set">
                  <span className="nx-ks__kind" data-tone={set.kind === 'of_interest' ? 'watch' : 'harmless'}><Flag aria-hidden="true" />{KIND_LABEL[set.kind]}</span>
                  <b className="nx-ks__name">{set.name}</b>
                  <span className="nx-tabular nx-ks__count">{`${set.hashes} ${set.hashes === 1 ? 'hash' : 'hashes'}`}</span>
                  {confirming === set.set_id
                    ? <span className="nx-ks__confirm"><button type="button" className="nx-btn nx-btn--sm" disabled={busy} onClick={() => retire(set)}>Retire it</button><button type="button" className="nx-btn nx-btn--ghost nx-btn--sm" onClick={() => setConfirming('')}>Keep</button></span>
                    : <button type="button" className="nx-btn nx-btn--ghost nx-btn--sm" disabled={busy} aria-label={`Retire ${set.name}`} onClick={() => setConfirming(set.set_id)}>Retire</button>}
                </li>
              ))}
            </ul>
          ) : <p className="nx-muted">No set yet. Make the first one below.</p>}
        </section>

        <form className="nx-ks__block" onSubmit={createAndAdd} aria-label="New set">
          <h3>New set</h3>
          <div className="nx-ks__row">
            <label className="nx-ks__field"><span>Name</span><input className="nx-input" value={name} maxLength={120} onChange={event => setName(event.target.value)} placeholder="Case 12 watch list" /></label>
            <label className="nx-ks__field"><span>Kind</span>
              <select className="nx-select" value={kind} onChange={event => setKind(event.target.value)}>
                <option value="of_interest">Watch list: flag these wherever they turn up</option>
                <option value="known_harmless">Known harmless: noise to hide (stock photos, logos)</option>
              </select>
            </label>
          </div>
          <button type="submit" className="nx-btn nx-btn--primary" disabled={busy || !name.trim()}>{picked.length ? `Create and add ${picked.length} selected` : 'Create set'}</button>
        </form>

        {sets.length ? (
          <section className="nx-ks__block" aria-label="Add to a set">
            <h3>Add to a set</h3>
            <label className="nx-ks__field"><span>Set</span>
              <select className="nx-select" value={target} onChange={event => setTarget(event.target.value)}>
                {sets.map(set => <option key={set.set_id} value={set.set_id}>{set.name}</option>)}
              </select>
            </label>
            <div className="nx-ks__row">
              <button type="button" className="nx-btn" disabled={busy || !picked.length || !target} onClick={addSelected}>{picked.length ? `Add ${picked.length} selected ${picked.length === 1 ? 'picture' : 'pictures'}` : 'Select pictures on the page first'}</button>
            </div>
            <label className="nx-ks__field"><span>Or a table of hashes (columns sha256, pdq256, label)</span>
              <textarea className="nx-input nx-ks__table" rows={4} value={csv} onChange={event => setCsv(event.target.value)} placeholder="sha256,pdq256,label" spellCheck={false} dir="ltr" />
            </label>
            <div className="nx-ks__row">
              <label className="nx-btn nx-ks__file"><Upload aria-hidden="true" />Choose a file<input type="file" accept=".csv,.tsv,.txt,text/csv,text/plain" className="nx-visually-hidden" onChange={readFile} /></label>
              <button type="button" className="nx-btn" disabled={busy || !csv.trim() || !target} onClick={importTable}>Import the table</button>
            </div>
          </section>
        ) : null}

        <p className={`nx-ks__message${message ? ` is-${message.tone}` : ''}`} role="status" aria-live="polite">{message?.text || ''}</p>
        <div className="nx-ks__foot"><button type="button" className="nx-btn" onClick={onClose}>Close</button></div>
      </div>
    </Dialog>
  )
}
