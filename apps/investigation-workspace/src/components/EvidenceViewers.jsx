import { useEffect, useMemo, useRef, useState } from 'react'
import { EvidenceStrengthBadge } from './Citations.jsx'
import { LanguageText } from './AnalystComponents.jsx'
import { VirtualizedTable } from './VirtualizedTable.jsx'
import { humanizeKey } from '../lib/format.js'
import { semanticCatalog } from '../lib/semanticCatalog.js'
import { lineage, transcriptCues, transcriptText, videoEvents } from '../lib/viewerPresentation.js'
import { strength } from '../lib/viewerStrength.js'
import { formatClock } from '../lib/viewerTools.js'
import { DocumentReader } from './viewers/DocumentReader.jsx'
import { HighlightedText } from './viewers/HighlightedText.jsx'
import { ImageInspector } from './viewers/ImageInspector.jsx'
import { MediaPlayer } from './viewers/MediaPlayer.jsx'
import { TranscriptPanel } from './viewers/TranscriptPanel.jsx'

export { HighlightedText }

function number(value) { const result = Number(value); return Number.isFinite(result) ? result : null }

export function LineagePanel({ detail, version }) {
  const model = lineage(detail, version)
  return <aside className="lineage-panel" aria-labelledby="lineage-heading"><h2 id="lineage-heading">Evidence lineage</h2><ol><li><span>Source file</span><strong><LanguageText>{model.source}</LanguageText></strong></li><li><span>Version</span><strong><LanguageText as="bdi" identifier>{model.version}</LanguageText></strong></li>{model.artifacts.map(artifact => <li className="lineage-artifact" key={artifact.id}><div className="lineage-artifact__heading"><span>{artifact.label}</span><EvidenceStrengthBadge strength={strength(artifact.confidence)} /></div><dl><div><dt>Recorded producer</dt><dd><LanguageText as="bdi" identifier>{artifact.producer}</LanguageText></dd></div><div><dt>Producer version</dt><dd><LanguageText as="bdi" identifier>{artifact.producerVersion}</LanguageText></dd></div></dl></li>)}</ol>{!model.artifacts.length ? <p>No derived artifacts were reported for this source.</p> : null}</aside>
}

export function DocumentViewer(props) { return <DocumentReader {...props} /> }

export function StructuredViewer({ detail, row, recordType, onRowSelect }) {
  const previewRows = detail.records_preview || []
  const keys = [...new Set(previewRows.flatMap(item => Object.keys(item || {})))].filter(key => !['metadata', 'row_hash', 'source_hash'].includes(key))
  const columns = keys.map(key => { const display = semanticCatalog.displayForColumn(key, recordType); return { key, ...display, label: display.fallback && display.label === key ? humanizeKey(key) : display.label } })
  const hasHashes = previewRows.some(item => item.row_hash || item.source_hash)
  if (hasHashes) columns.push({ key: '__source', label: 'Source row', description: 'Exact source row and hash' })
  // Keep the same locator that the analyst can inspect in the exported row.
  // A custom cell renderer alone would leave the CSV column empty.
  const rows = hasHashes ? previewRows.map(item => ({
    ...item,
    __source: `Row ${item.row_number ?? item.source_row ?? 'not reported'} · ${item.row_hash || item.source_hash || 'hash not reported'}`,
  })) : previewRows
  function cell(item, column, value) {
    if (column.key !== '__source') return <LanguageText>{value == null || value === '' ? '—' : String(value)}</LanguageText>
    const hash = item.row_hash || item.source_hash
    return hash ? <details className="row-hash"><summary>Show row hash</summary><LanguageText as="bdi" identifier>{hash}</LanguageText></details> : <span>Not reported</span>
  }
  return <section className="source-viewer structured-viewer" aria-labelledby="structured-viewer-heading"><h2 id="structured-viewer-heading">Structured source</h2>{rows.length ? <VirtualizedTable columns={columns} rows={rows} label="Structured evidence rows" initialRow={row} renderCell={cell} onRowSelect={onRowSelect ? item => onRowSelect(item, columns) : undefined} exportName="nexusai-structured-evidence.csv" /> : <p>The evidence-detail response did not include a record preview.</p>}</section>
}

export function ImageViewer(props) { return <ImageInspector {...props} /> }

export function AudioViewer({ detail, objectUrl, sourceTime, charSpan }) {
  const media = useRef(null)
  const cues = useMemo(() => transcriptCues(detail), [detail])
  const [current, setCurrent] = useState(number(sourceTime) || 0)
  const cited = number(sourceTime)
  function seek(time) { if (media.current) { media.current.currentTime = time; media.current.play().catch(() => {}) } }
  const plain = transcriptText(detail)
  return (
    <section className="source-viewer audio-viewer" aria-labelledby="audio-viewer-heading">
      <header><h2 id="audio-viewer-heading">Audio source</h2>{cited !== null ? <EvidenceStrengthBadge strength={strength(null, true)} ariaLabel="Evidence strength" /> : null}</header>
      <MediaPlayer kind="audio" src={objectUrl} label="Audio player" mediaRef={media} initialTime={cited} durationHint={number(detail.item?.metadata?.duration_seconds) || 0} onTime={setCurrent} markers={cited !== null ? [{ id: 'cited', time: cited, label: 'Cited moment', cited: true }] : []} />
      {cues.length ? <TranscriptPanel cues={cues} time={current} onSeek={seek} cited={cited} /> : plain ? <><p className="viewer-gap">A transcript was supplied without timestamp cues, so synchronized seeking and speaker turns are unavailable.</p><HighlightedText text={plain} span={charSpan} /></> : <p className="viewer-gap">No transcript was supplied for this audio.</p>}
    </section>
  )
}

export function VideoViewer({ detail, objectUrl, frame }) {
  const media = useRef(null)
  const canvas = useRef(null)
  const [capture, setCapture] = useState(null)
  const events = useMemo(() => videoEvents(detail, frame), [detail, frame])
  useEffect(() => () => { if (capture) URL.revokeObjectURL(capture) }, [capture])
  function seek(time) { if (media.current) media.current.currentTime = time }
  function extractFrame() {
    const video = media.current; const target = canvas.current
    if (!video || !target || !video.videoWidth || !video.videoHeight) return
    target.width = video.videoWidth; target.height = video.videoHeight
    target.getContext('2d')?.drawImage(video, 0, 0)
    target.toBlob(blob => {
      if (!blob) return
      if (capture) URL.revokeObjectURL(capture)
      setCapture(URL.createObjectURL(blob))
    }, 'image/png')
  }
  return (
    <section className="source-viewer video-viewer" aria-labelledby="video-viewer-heading">
      <header><h2 id="video-viewer-heading">Video source</h2><button type="button" onClick={extractFrame}>Extract current frame</button></header>
      <MediaPlayer kind="video" src={objectUrl} label="Video player" mediaRef={media} initialTime={number(frame)} durationHint={number(detail.item?.metadata?.duration_seconds) || 0} markers={events} />
      {events.length ? <ol className="video-events">{events.map(event => <li key={event.id} className={event.cited ? 'is-cited' : undefined}><button type="button" onClick={() => seek(event.time)}><time>{formatClock(event.time)}</time><span>{event.label}</span></button><EvidenceStrengthBadge strength={strength(event.confidence, event.cited)} /></li>)}</ol> : <p className="viewer-gap">No timestamped derived events were supplied for this video.</p>}
      <canvas ref={canvas} hidden />
      {capture ? <figure className="frame-capture"><img src={capture} alt={`Extracted video frame at ${media.current?.currentTime?.toFixed(1) || 'current'} seconds`} /><figcaption><a href={capture} download="nexusai-frame.png">Download extracted frame</a></figcaption></figure> : null}
    </section>
  )
}
