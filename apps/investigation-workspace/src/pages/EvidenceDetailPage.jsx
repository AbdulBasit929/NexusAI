import { useEffect, useMemo, useState } from 'react'
import { useParams, useSearchParams } from 'react-router-dom'
import { CaseShell } from '../components/CaseShell.jsx'
import { getEvidence, getEvidenceContent } from '../lib/apiClient.js'
import { displayName, formatBytes } from '../lib/format.js'
import { LanguageText, ProcessingBadge, RouteState } from '../components/AnalystComponents.jsx'
import { AudioViewer, DocumentViewer, ImageViewer, LineagePanel, StructuredViewer, VideoViewer } from '../components/EvidenceViewers.jsx'
import { PageHeader } from '../components/PageHeader.jsx'

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

export default function EvidenceDetailPage() {
  const { id: caseId = '', eid: evidenceId = '' } = useParams()
  const [searchParams, setSearchParams] = useSearchParams()
  const [state, setState] = useState({ loading: true })
  const [content, setContent] = useState({ loading: true })
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
  return <CaseShell caseId={caseId}>
    <main id="workspace-main" className="detail-page evidence-viewer-page" tabIndex={-1}>
      <PageHeader
        eyebrow={detail ? `${displayName(item.modality)} evidence` : 'Evidence source'}
        title={detail ? <LanguageText>{item.original_filename || item.source_file}</LanguageText> : 'Opening evidence'}
        description="Inspect the retained source and any exact locator carried into this view."
        breadcrumbs={[{ label: 'Evidence', to: `/cases/${encodeURIComponent(caseId)}/evidence` }, { label: 'Source' }]}
        meta={detail ? [{ label: 'File size', value: formatBytes(item.size_bytes) }, { label: 'Processing state', value: <ProcessingBadge state={item.processing_status} /> }] : []}
      />
      {state.loading ? <RouteState state="loading" label="Opening evidence" /> : null}
      {state.error ? <RouteState state={state.error.status === 403 ? 'forbidden' : 'error'} label={state.error.status === 403 ? 'Evidence access is forbidden' : 'Evidence could not be opened'} reference={state.error.reference || 'DETAIL'} /> : null}
      {detail ? <>
        <div className="viewer-canvas">
          <div className="viewer-canvas__main">
            {locators.length ? <section className="locator-card" aria-labelledby="locator-heading"><h2 id="locator-heading">Exact source location</h2><dl>{locators.map(([label, value]) => <div key={label}><dt>{label}</dt><dd><LanguageText as="bdi" identifier>{value}</LanguageText></dd></div>)}</dl></section> : null}
            {modality === 'structured_records' ? <StructuredViewer detail={detail} row={searchParams.get('row')} recordType={item.detected_type} /> : null}
            {modality === 'document' ? <DocumentViewer detail={detail} objectUrl={content.objectUrl} page={searchParams.get('page')} charSpan={searchParams.get('char_span')} onPageChange={changePage} /> : null}
            {modality === 'image' && content.objectUrl ? <ImageViewer detail={detail} objectUrl={content.objectUrl} citedBBox={searchParams.get('bbox')} /> : null}
            {modality === 'audio' && content.objectUrl ? <AudioViewer detail={detail} objectUrl={content.objectUrl} sourceTime={searchParams.get('source_time')} charSpan={searchParams.get('char_span')} /> : null}
            {modality === 'video' && content.objectUrl ? <VideoViewer detail={detail} objectUrl={content.objectUrl} frame={searchParams.get('frame') || searchParams.get('source_time')} /> : null}
            {modality !== 'structured_records' && !content.objectUrl ? <p className="viewer-gap">{content.loading ? 'Loading retained source…' : 'The retained source cannot be displayed inline.'}</p> : null}
          </div>
          <LineagePanel detail={detail} version={searchParams.get('version')} />
        </div>
      </> : null}
    </main>
  </CaseShell>
}
