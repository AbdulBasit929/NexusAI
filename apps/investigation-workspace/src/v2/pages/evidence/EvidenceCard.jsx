import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { ArrowUpRight, AudioLines, Check, Eye, File, FileSpreadsheet, FileText, Flag, Image as ImageIcon, Play, Table2, Video, Volume2 } from 'lucide-react'
import { LanguageText } from '../../../components/AnalystComponents.jsx'
import { formatBytes } from '../../../lib/format.js'
import { relativeAge } from '../../../lib/dashboardCases.js'
import { curatedFamilyLabel } from '../../../lib/semanticCatalog.js'
import { Dialog } from '../../ui/Dialog.jsx'
import { displayName, itemFacts, kindOf, shortId, statusOf, failureReason } from './evidenceModel.js'
import { cachedPreview, loadPreview } from './previewStore.js'
import { flagSummary } from './knownSetsModel.js'

export const KIND_ICON = { structured_records: Table2, tabular: FileSpreadsheet, document: FileText, text: FileText, image: ImageIcon, audio: AudioLines, video: Video }

export function typeLabel(item) {
  const kind = kindOf(item.modality)
  if (kind.id !== 'structured_records') return kind.one
  const type = String(item.detected_type || '').toLowerCase()
  return type && type !== 'generic' ? curatedFamilyLabel(item.detected_type) : kind.one
}

export const evidenceHref = (caseId, item) => `/cases/${encodeURIComponent(caseId)}/evidence/${encodeURIComponent(item.evidence_id)}`

// A card asks for its picture when it comes near the window, once, and keeps it. A picture already made (the card was seen before) is there at once.
export function usePreview(caseId, item) {
  const ref = useRef(null)
  const [state, setState] = useState(() => { const hit = cachedPreview(caseId, item); return hit ? { status: hit.kind === 'none' ? 'none' : 'ready', data: hit } : { status: 'idle' } })
  const [seen, setSeen] = useState(state.status !== 'idle')
  useEffect(() => {
    if (seen) return undefined
    const node = ref.current
    if (!node || !('IntersectionObserver' in globalThis)) { setSeen(true); return undefined }
    const watcher = new globalThis.IntersectionObserver(entries => { if (entries.some(entry => entry.isIntersecting)) { setSeen(true); watcher.disconnect() } }, { rootMargin: '280px' })
    watcher.observe(node)
    return () => watcher.disconnect()
  }, [seen])
  useEffect(() => {
    if (!seen || state.status === 'ready' || state.status === 'none') return undefined
    const controller = new AbortController()
    setState({ status: 'loading' })
    loadPreview({ caseId, item, signal: controller.signal }).then(data => { if (!controller.signal.aborted && data) setState({ status: data.kind === 'none' ? 'none' : 'ready', data }) })
    return () => controller.abort()
    // eslint-disable-next-line react-hooks/exhaustive-deps -- the picture depends on the file, not on every field of the row
  }, [seen, caseId, item.evidence_id, item.sha256])
  return [ref, state]
}

function Wave({ peaks }) {
  if (!peaks) return <AudioLines className="nx-evcard__glyph" aria-hidden="true" />
  return (
    <svg className="nx-evcard__wave" viewBox={`0 0 ${peaks.length} 24`} preserveAspectRatio="none" aria-hidden="true">
      {peaks.map((peak, index) => <rect key={index} x={index + 0.14} y={12 - Math.max(0.9, peak * 11)} width="0.72" height={Math.max(1.8, peak * 22)} rx="0.3" />)}
    </svg>
  )
}

function Sheet({ data }) {
  return (
    <table className="nx-evcard__sheet" aria-hidden="true">
      <thead><tr>{data.columns.map((column, index) => <th key={index}>{column}</th>)}</tr></thead>
      <tbody>{data.rows.map((row, at) => <tr key={at}>{row.map((cell, index) => <td key={index}>{cell}</td>)}</tr>)}</tbody>
    </table>
  )
}

// The face of a card: the file itself where it can be read cheaply, and its facts where it cannot. Nothing here is made up.
export function Face({ item, preview, large = false }) {
  const kind = kindOf(item.modality)
  const Icon = KIND_ICON[kind.id] || File
  const facts = itemFacts(item)
  const data = preview.data
  let inside
  if (kind.id === 'image') inside = data?.kind === 'image' ? <img className="nx-evcard__img" src={data.url} alt="" width={data.width} height={data.height} draggable={false} /> : <Icon className="nx-evcard__glyph" aria-hidden="true" />
  else if (kind.id === 'audio') inside = <><Wave peaks={data?.kind === 'wave' ? data.peaks : null} /><span className="nx-evcard__play nx-evcard__play--small" aria-hidden="true"><Play /></span></>
  else if (kind.id === 'video') inside = <span className="nx-evcard__play" aria-hidden="true"><Play /></span>
  else if (kind.id === 'document' || kind.id === 'text') inside = data?.kind === 'text' ? <div className="nx-evcard__paper" dir={data.dir}>{data.lines.map((line, index) => <p key={index}>{line}</p>)}</div> : <Icon className="nx-evcard__glyph" aria-hidden="true" />
  else inside = data?.kind === 'rows' ? <Sheet data={data} /> : <Icon className="nx-evcard__glyph" aria-hidden="true" />
  const sound = kind.id === 'video' && Number(item.media_metadata?.audio_track_count) > 0
  return (
    <div className={`nx-evcard__face${large ? ' is-large' : ''}`} data-kind={kind.id} data-tone={kind.tone} data-state={preview.status}>
      {inside}
      <span className="nx-evcard__chip nx-evcard__chip--kind"><Icon aria-hidden="true" />{typeLabel(item)}</span>
      {facts ? <span className="nx-evcard__chip nx-evcard__chip--fact nx-tabular">{sound ? <Volume2 aria-label="With sound" /> : null}{facts.main}</span> : null}
    </div>
  )
}

export function StatusMark({ item }) {
  const status = statusOf(item)
  const reason = status.key === 'failed' ? failureReason(item) : ''
  return <span className={`nx-st nx-st--${status.key}`} title={reason || undefined}><i aria-hidden="true" />{status.label}</span>
}

// flags: what the known sets say about this file (knownSetsModel.js); selecting: the page is choosing pictures for a set, and a click on a picture's card chooses it instead of opening it
export function EvidenceCard({ item, caseId, onPeek, flags, selecting = false, selected = false, onToggle }) {
  const [ref, preview] = usePreview(caseId, item)
  const status = statusOf(item)
  const when = item.created_at ? new Date(item.created_at) : null
  const known = when && !Number.isNaN(when.getTime())
  const name = displayName(item)
  const summary = flagSummary(flags)
  const pickable = Boolean(selecting && onToggle && kindOf(item.modality).id === 'image')
  const pick = pickable ? event => { if (event.target.closest('.nx-evcard__pick, .nx-evcard__peek')) return; event.preventDefault(); onToggle(item.evidence_id) } : undefined
  return (
    <li className="nx-evcard" data-id={item.evidence_id} data-status={status.key} data-tone={kindOf(item.modality).tone} data-flag={summary?.tone} data-picking={pickable || undefined} data-selected={(pickable && selected) || undefined} onClickCapture={pick}>
      <div className="nx-evcard__facewrap" ref={ref}>
        <Face item={item} preview={preview} />
        {summary ? <span className="nx-evcard__flag" data-tone={summary.tone} title={summary.title}><Flag aria-hidden="true" /><span>{summary.label}</span></span> : null}
        {pickable ? <button type="button" className="nx-evcard__pick" aria-pressed={selected} aria-label={`${selected ? 'Deselect' : 'Select'} ${name}`} onClick={() => onToggle(item.evidence_id)}>{selected ? <Check aria-hidden="true" /> : null}</button> : null}
      </div>
      <div className="nx-evcard__body">
        <h3 className="nx-evcard__name"><Link className="nx-ev-open nx-evcard__link" to={evidenceHref(caseId, item)} title={name} aria-label={name}><LanguageText>{name}</LanguageText></Link></h3>
        <p className="nx-evcard__meta">
          <span className="nx-tabular">{item.size_bytes == null ? '–' : formatBytes(item.size_bytes)}</span>
          {known ? <><i aria-hidden="true" /><time dateTime={item.created_at}>{relativeAge(when)}</time></> : null}
          {status.key !== 'ready' ? <><i aria-hidden="true" /><StatusMark item={item} /></> : null}
        </p>
      </div>
      <button type="button" className="nx-evcard__peek" aria-label={`Quick look at ${name}`} title="Quick look (Space)" onClick={() => onPeek(item)}><Eye aria-hidden="true" /></button>
      <span className="nx-evcard__open" aria-hidden="true"><ArrowUpRight /></span>
    </li>
  )
}

// A larger look at one file without leaving the list: the same face, its facts, and the way in.
export function QuickLook({ item, caseId, onClose }) {
  const [, preview] = usePreview(caseId, item)
  const facts = itemFacts(item)
  const status = statusOf(item)
  const when = item.created_at ? new Date(item.created_at) : null
  const known = when && !Number.isNaN(when.getTime())
  return (
    <Dialog title={displayName(item)} onClose={onClose} wide>
      <div className="nx-evpeek">
        <Face item={item} preview={preview} large />
        <dl className="nx-evpeek__facts">
          <div><dt>Kind</dt><dd>{typeLabel(item)}</dd></div>
          {facts ? <div><dt>Details</dt><dd className="nx-tabular">{facts.main}{facts.note ? <small> · {facts.note}</small> : null}</dd></div> : null}
          <div><dt>Size</dt><dd className="nx-tabular">{item.size_bytes == null ? 'Not reported' : formatBytes(item.size_bytes)}</dd></div>
          <div><dt>Added</dt><dd>{known ? `${new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'short', timeZone: 'UTC' }).format(when)} UTC` : 'Not reported'}</dd></div>
          <div><dt>State</dt><dd><StatusMark item={item} /></dd></div>
          <div><dt>ID</dt><dd className="nx-mono" dir="ltr" title={item.evidence_id}>{shortId(item.evidence_id)}…</dd></div>
          {item.sha256 ? <div><dt>SHA-256</dt><dd className="nx-mono" dir="ltr" title={item.sha256}>{String(item.sha256).slice(0, 16)}…</dd></div> : null}
        </dl>
        <div className="nx-evpeek__actions">
          <Link className="nx-btn nx-btn--primary" to={evidenceHref(caseId, item)} onClick={onClose}>{status.key === 'ready' ? 'Open evidence' : 'Open anyway'}<ArrowUpRight aria-hidden="true" /></Link>
          <button type="button" className="nx-btn" onClick={onClose}>Close</button>
        </div>
      </div>
    </Dialog>
  )
}
