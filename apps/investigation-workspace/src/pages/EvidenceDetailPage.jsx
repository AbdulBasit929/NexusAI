import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { ArrowLeft, Check, Copy, Download, File, FileSpreadsheet, FileText, Image as ImageIcon, MapPin, AudioLines, Video } from 'lucide-react'
import { CaseShell } from '../components/CaseShell.jsx'
import { getEvidence, getEvidenceContent } from '../lib/apiClient.js'
import { displayName, formatBytes } from '../lib/format.js'
import { LanguageText, ProcessingBadge, RouteState } from '../components/AnalystComponents.jsx'
import { AudioViewer, DocumentViewer, ImageViewer, LineagePanel, StructuredViewer, VideoViewer } from '../components/EvidenceViewers.jsx'
import { recordFields } from '../lib/viewerTools.js'
import { curatedFamilyLabel } from '../lib/semanticCatalog.js'
import { relativeAge } from '../lib/dashboardCases.js'
import { formatNumber } from '../lib/format.js'

function locatorItems(searchParams) {
  return [
    ['Row', searchParams.get('row')],
    ['Row hash', searchParams.get('row_hash')],
    ['Page', searchParams.get('page')],
    ['Character span', searchParams.get('char_span')],
    ['Source time', searchParams.get('source_time') && `${searchParams.get('source_time')} seconds`],
    ['Source end', searchParams.get('source_end') && `${searchParams.get('source_end')} seconds`],
    ['Frame', searchParams.get('frame') && `${searchParams.get('frame')} seconds`],
    ['Region', searchParams.get('bbox')],
    ['Finding', searchParams.get('finding')],
  ].filter(([, value]) => value)
}

const MODALITY_ICON = { structured_records: FileSpreadsheet, document: FileText, image: ImageIcon, audio: AudioLines, video: Video }
const HASH_KEYS = ['sha256', 'content_hash', 'file_hash', 'source_hash', 'checksum']

function when(value) {
  const date = value ? new Date(value) : null
  return date && !Number.isNaN(date.getTime()) ? date : null
}

// The facts the service reports about this file, only those it reported: nothing is filled in for a field it left out.
export function evidenceFacts(item) {
  const added = when(item.created_at)
  const updated = when(item.updated_at)
  const fmt = date => `${new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'short', timeZone: 'UTC' }).format(date)} UTC`
  const hashKey = HASH_KEYS.find(key => item[key])
  return [
    { id: 'id', label: 'Evidence ID', value: item.evidence_id, identifier: true, copy: true },
    { id: 'name', label: 'File name', value: item.original_filename || item.source_file },
    { id: 'family', label: 'Family', value: item.detected_type ? curatedFamilyLabel(item.detected_type) : item.modality ? displayName(item.modality) : '' },
    { id: 'size', label: 'Size', value: item.size_bytes == null ? '' : formatBytes(item.size_bytes) },
    { id: 'rows', label: 'Accepted rows', value: item.accepted_rows == null ? '' : formatNumber(item.accepted_rows) },
    { id: 'added', label: 'Added', value: added ? fmt(added) : '' },
    { id: 'updated', label: 'Last updated', value: updated ? fmt(updated) : '' },
    { id: 'hash', label: 'Content hash', value: hashKey ? String(item[hashKey]) : '', identifier: true, copy: true },
  ].filter(fact => fact.value)
}

function CopyButton({ text, label }) {
  const [done, setDone] = useState(false)
  async function copy() {
    try { await globalThis.navigator.clipboard.writeText(text); setDone(true); globalThis.setTimeout(() => setDone(false), 1500) } catch { setDone(false) }
  }
  return <button type="button" className="evv-copy" onClick={copy} aria-label={done ? `${label} copied` : `Copy ${label}`}>{done ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}</button>
}

function Inspector({ detail, version, record }) {
  const [tab, setTab] = useState('details')
  const tabs = [['details', 'Details'], ['lineage', 'Lineage']]
  if (record) tabs.unshift(['record', 'Record'])
  useEffect(() => { if (record) setTab('record') }, [record])
  const refs = useRef([])
  function onKey(event, index) {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
    event.preventDefault()
    const next = event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1 : (index + (event.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length
    setTab(tabs[next][0])
    refs.current[next]?.focus()
  }
  const facts = evidenceFacts(detail.item || {})
  return (
    <aside className="evv-inspector" aria-label="Evidence inspector">
      <div className="evv-tabs" role="tablist" aria-label="Inspector">
        {tabs.map(([id, label], index) => <button key={id} ref={node => { refs.current[index] = node }} type="button" role="tab" id={`evv-tab-${id}`} aria-selected={tab === id} aria-controls={`evv-panel-${id}`} tabIndex={tab === id ? 0 : -1} onKeyDown={event => onKey(event, index)} onClick={() => setTab(id)}>{label}</button>)}
      </div>
      <div className="evv-panel" role="tabpanel" id={`evv-panel-${tab}`} aria-labelledby={`evv-tab-${tab}`}>
        {tab === 'record' && record ? (
          <dl className="evv-facts evv-record">
            {recordFields(record.row, record.columns).map(field => <div key={field.key}><dt>{field.label}</dt><dd><LanguageText>{String(field.value)}</LanguageText></dd></div>)}
          </dl>
        ) : tab === 'details' ? (
          <dl className="evv-facts">
            {facts.map(fact => <div key={fact.id}><dt>{fact.label}</dt><dd>{fact.identifier ? <LanguageText as="bdi" identifier>{fact.value}</LanguageText> : <LanguageText>{fact.value}</LanguageText>}{fact.copy ? <CopyButton text={fact.value} label={fact.label} /> : null}</dd></div>)}
          </dl>
        ) : <LineagePanel detail={detail} version={version} />}
      </div>
    </aside>
  )
}

export default function EvidenceDetailPage() {
  const { id: caseId = '', eid: evidenceId = '' } = useParams()
  const [searchParams, setSearchParams] = useSearchParams()
  const [state, setState] = useState({ loading: true })
  const [content, setContent] = useState({ loading: true })
  const [record, setRecord] = useState(null)
  useEffect(() => {
    const controller = new AbortController()
    getEvidence({ caseId, evidenceId, signal: controller.signal })
      .then(data => setState({ data, loading: false }))
      .catch(error => error.name !== 'AbortError' && setState({ error, loading: false }))
    return () => controller.abort()
  }, [caseId, evidenceId])
  const modality = state.data?.item?.modality
  useEffect(() => {
    if (!modality || modality === 'structured_records') return undefined
    const controller = new AbortController()
    let objectUrl
    getEvidenceContent({ caseId, evidenceId, signal: controller.signal })
      .then(blob => {
        objectUrl = URL.createObjectURL(blob)
        setContent({ objectUrl, loading: false })
      })
      .catch(error => error.name !== 'AbortError' && setContent({ error, loading: false }))
    return () => { controller.abort(); if (objectUrl) URL.revokeObjectURL(objectUrl) }
  }, [caseId, evidenceId, modality])
  const locators = useMemo(() => locatorItems(searchParams), [searchParams])
  const detail = state.data
  const item = detail?.item || {}
  function changePage(page) {
    const next = new URLSearchParams(searchParams)
    next.set('page', String(page)); next.delete('char_span')
    setSearchParams(next)
  }
  const encoded = encodeURIComponent(caseId)
  const Mark = MODALITY_ICON[String(modality || '').toLowerCase()] || File
  const added = when(item.created_at)
  const name = item.original_filename || item.source_file || evidenceId
  return <CaseShell caseId={caseId}>
    <main id="workspace-main" className="detail-page evidence-viewer-page evv" tabIndex={-1}>
      <header className="evv-head">
        <nav className="evv-crumbs" aria-label="Page trail"><Link to={`/cases/${encoded}/evidence`}><ArrowLeft aria-hidden="true" />Evidence</Link></nav>
        {detail ? (
          <div className="evv-title">
            <span className="evv-mark" aria-hidden="true"><Mark /></span>
            <div className="evv-title__text">
              <h1><LanguageText>{name}</LanguageText></h1>
              <p className="evv-chips">
                <span>{item.detected_type ? curatedFamilyLabel(item.detected_type) : displayName(item.modality)}</span>
                {item.size_bytes != null ? <span>{formatBytes(item.size_bytes)}</span> : null}
                {added ? <span title={added.toISOString()}>Added {relativeAge(added)}</span> : null}
                <ProcessingBadge state={item.processing_status} />
              </p>
            </div>
            <div className="evv-actions">
              {content.objectUrl ? <a className="evv-btn" href={content.objectUrl} download={name}><Download aria-hidden="true" />Download original</a> : null}
            </div>
          </div>
        ) : <h1 className="evv-title__loading">Opening evidence</h1>}
      </header>
      {state.loading ? <RouteState state="loading" label="Opening evidence" /> : null}
      {state.error ? <RouteState state={state.error.status === 403 ? 'forbidden' : 'error'} label={state.error.status === 403 ? 'Evidence access is forbidden' : 'Evidence could not be opened'} reference={state.error.reference || 'evidence-open'} /> : null}
      {detail ? <>
        {locators.length ? (
          <section className="evv-locator" aria-labelledby="locator-heading">
            <span className="evv-locator__icon" aria-hidden="true"><MapPin /></span>
            <div>
              <h2 id="locator-heading">Exact source location</h2>
              <dl>{locators.map(([label, value]) => <div key={label}><dt>{label}</dt><dd><LanguageText as="bdi" identifier>{value}</LanguageText></dd></div>)}</dl>
            </div>
          </section>
        ) : null}
        <div className="evv-body">
          <div className="evv-canvas">
            {modality === 'structured_records' ? <StructuredViewer detail={detail} row={searchParams.get('row')} recordType={item.detected_type} onRowSelect={(row, columns) => setRecord({ row, columns })} /> : null}
            {modality === 'document' ? <DocumentViewer detail={detail} objectUrl={content.objectUrl} page={searchParams.get('page')} charSpan={searchParams.get('char_span')} onPageChange={changePage} /> : null}
            {modality === 'image' && content.objectUrl ? <ImageViewer detail={detail} objectUrl={content.objectUrl} citedBBox={searchParams.get('bbox')} /> : null}
            {modality === 'audio' && content.objectUrl ? <AudioViewer detail={detail} objectUrl={content.objectUrl} sourceTime={searchParams.get('source_time')} charSpan={searchParams.get('char_span')} /> : null}
            {modality === 'video' && content.objectUrl ? <VideoViewer detail={detail} objectUrl={content.objectUrl} frame={searchParams.get('frame') || searchParams.get('source_time')} /> : null}
            {modality !== 'structured_records' && !content.objectUrl ? <p className="viewer-gap">{content.loading ? 'Loading retained source…' : 'The retained source cannot be displayed inline.'}</p> : null}
          </div>
          <Inspector detail={detail} version={searchParams.get('version')} record={record} />
        </div>
      </> : null}
    </main>
  </CaseShell>
}
