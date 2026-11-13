import { useEffect, useMemo, useRef, useState } from 'react'
import { EvidenceStrengthBadge } from './Citations.jsx'
import { LanguageText } from './AnalystComponents.jsx'
import { VirtualizedTable } from './VirtualizedTable.jsx'
import { humanizeKey } from '../lib/format.js'
import { semanticCatalog } from '../lib/semanticCatalog.js'
import { lineage, transcriptText, videoEvents } from '../lib/viewerPresentation.js'
import { audioSegments } from '../lib/audioSegments.js'
import { plateReadings, videoText } from '../lib/mediaReadings.js'
import { strength } from '../lib/viewerStrength.js'
import { formatClock } from '../lib/viewerTools.js'
import { DocumentReader } from './viewers/DocumentReader.jsx'
import { HighlightedText } from './viewers/HighlightedText.jsx'
import { ImageInspector } from './viewers/ImageInspector.jsx'
import { MediaPlayer } from './viewers/MediaPlayer.jsx'
import { SpeechBanner, languageName } from './viewers/SpeechBanner.jsx'
import { VideoBoxes } from './viewers/MediaMarks.jsx'
import { groupsFromReads, VideoPlates } from './viewers/VideoPlates.jsx'
import { TranscriptPanel } from './viewers/TranscriptPanel.jsx'

export { HighlightedText }

// no time asked for (null, an empty parameter) is no time, not second 0
function number(value) { if (value == null || value === '') return null; const result = Number(value); return Number.isFinite(result) ? result : null }

export function LineagePanel({ detail, version }) {
  const model = lineage(detail, version)
  return <aside className="lineage-panel" aria-labelledby="lineage-heading"><h2 id="lineage-heading">Evidence lineage</h2><ol><li><span>Source file</span><strong><LanguageText>{model.source}</LanguageText></strong></li><li><span>Version</span><strong><LanguageText as="bdi" identifier>{model.version}</LanguageText></strong></li>{model.artifacts.map(artifact => <li className="lineage-artifact" key={artifact.id}><div className="lineage-artifact__heading"><span>{artifact.label}</span><EvidenceStrengthBadge strength={strength(artifact.confidence)} /></div><dl><div><dt>Recorded producer</dt><dd><LanguageText as="bdi" identifier>{artifact.producer}</LanguageText></dd></div><div><dt>Producer version</dt><dd><LanguageText as="bdi" identifier>{artifact.producerVersion}</LanguageText></dd></div></dl></li>)}</ol>{!model.artifacts.length ? <p>No derived artifacts were reported for this source.</p> : null}</aside>
}

export function DocumentViewer(props) { return <DocumentReader {...props} /> }

// The system's own columns of a stored record: they identify the record for the system and say nothing about the evidence, so the grid keeps them behind a toggle.
export const TECHNICAL_COLUMNS = ['batch_id', 'collection_id', 'file_id', 'record_id', 'record_type', 'ingested_at', 'raw_payload', 'source_file', '__source']

export function StructuredViewer({ detail, row, recordType, onRowSelect }) {
  const [showTechnical, setShowTechnical] = useState(false)
  const previewRows = detail.records_preview || []
  const keys = [...new Set(previewRows.flatMap(item => Object.keys(item || {})))].filter(key => !['metadata', 'row_hash', 'source_hash'].includes(key))
  const allColumns = keys.map(key => { const display = semanticCatalog.displayForColumn(key, recordType); return { key, ...display, label: display.fallback && display.label === key ? humanizeKey(key) : display.label } })
  const hasHashes = previewRows.some(item => item.row_hash || item.source_hash)
  if (hasHashes) allColumns.push({ key: '__source', label: 'Row and hash', description: 'Exact source row and hash' })
  const columns = showTechnical ? allColumns : allColumns.filter(column => !TECHNICAL_COLUMNS.includes(column.key))
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
  const hiddenCount = allColumns.length - allColumns.filter(column => !TECHNICAL_COLUMNS.includes(column.key)).length
  return <section className="source-viewer structured-viewer" aria-labelledby="structured-viewer-heading"><h2 id="structured-viewer-heading">Stored records of the file</h2>{hiddenCount > 0 ? <p className="structured-viewer__toggle"><button type="button" aria-pressed={showTechnical} onClick={() => setShowTechnical(value => !value)}>{showTechnical ? 'Hide the system columns' : `Show the system columns (${hiddenCount})`}</button></p> : null}{rows.length ? <VirtualizedTable columns={columns} rows={rows} label="Structured evidence rows" initialRow={row} renderCell={cell} onRowSelect={onRowSelect ? item => onRowSelect(item, columns) : undefined} exportName="nexusai-structured-evidence.csv" /> : <p>The evidence-detail response did not include a record preview.</p>}</section>
}

export function ImageViewer(props) { return <ImageInspector {...props} /> }

export function AudioViewer({ detail, objectUrl, sourceTime, charSpan }) {
  const media = useRef(null)
  const speech = useMemo(() => audioSegments(detail), [detail])
  const cues = speech.segments
  const [current, setCurrent] = useState(number(sourceTime) || 0)
  const cited = number(sourceTime)
  function seek(time) { if (media.current) { media.current.currentTime = time; media.current.play().catch(() => {}) } }
  const plain = transcriptText(detail)
  const bands = useMemo(() => cues.map((cue, index) => ({ id: cue.id, start: cue.start, end: cue.end, tone: cue.status === 'SEARCH_LEAD' ? 'caution' : cue.status === 'LOW_RESOURCE_LANGUAGE' ? 'danger' : 'plain', active: cues[index].start <= current && (cue.end == null ? index === cues.length - 1 || current < cues[index + 1].start : current < cue.end), label: `Segment ${index + 1}, ${formatClock(cue.start)}${cue.end ? ` to ${formatClock(cue.end)}` : ''}${cue.language ? `, ${languageName(cue.language)}` : ''}` })), [cues, current])
  return (
    <section className="source-viewer audio-viewer mx" aria-labelledby="audio-viewer-heading">
      <header><h2 id="audio-viewer-heading">Audio source</h2>{cited !== null ? <EvidenceStrengthBadge strength={strength(null, true)} ariaLabel="Evidence strength" /> : null}</header>
      {cues.length ? <SpeechBanner model={speech} /> : null}
      <MediaPlayer kind="audio" src={objectUrl} label="Audio player" mediaRef={media} initialTime={cited} durationHint={number(detail.item?.metadata?.duration_seconds) || speech.audioSeconds || 0} onTime={setCurrent} markers={cited !== null ? [{ id: 'cited', time: cited, label: 'Cited moment', cited: true }] : []} bands={bands} />
      {cues.length ? <TranscriptPanel cues={cues} time={current} onSeek={seek} cited={cited} stopAt={speech.complete ? null : speech.readUntil} overall={speech.status} /> : plain ? <><p className="viewer-gap">A transcript was supplied without timestamp cues, so synchronized seeking and speaker turns are unavailable.</p><HighlightedText text={plain} span={charSpan} /></> : <p className="viewer-gap">No transcript was supplied for this audio.</p>}
    </section>
  )
}

export function VideoViewer({ detail, objectUrl, frame }) {
  const media = useRef(null)
  const canvas = useRef(null)
  const [capture, setCapture] = useState(null)
  const events = useMemo(() => videoEvents(detail, frame), [detail, frame])
  const readings = useMemo(() => plateReadings(detail), [detail])
  const plates = useMemo(() => groupsFromReads(readings.groups, readings.reads), [readings])
  const words = useMemo(() => videoText(detail), [detail])
  const [current, setCurrent] = useState(0)
  const [natural, setNatural] = useState(null)
  const bands = useMemo(() => plates.map(group => ({ id: group.id, start: group.first, end: group.last, tone: 'plate', active: current >= group.first - 0.05 && current <= group.last + 0.05, label: `Plate ${group.plate}, ${formatClock(group.first)} to ${formatClock(group.last)}` })), [plates, current])
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
    <section className="source-viewer video-viewer mx" aria-labelledby="video-viewer-heading">
      <header><h2 id="video-viewer-heading">Video source</h2><button type="button" onClick={extractFrame}>Extract current frame</button></header>
      <div className={`mx__grid${plates.length ? ' mx__grid--wide' : ''}`}>
      <div className="mx__stage"><MediaPlayer kind="video" src={objectUrl} label="Video player" mediaRef={media} initialTime={number(frame)} durationHint={number(detail.item?.metadata?.duration_seconds) || 0} markers={plates.length ? [] : events} bands={bands} onTime={setCurrent} onMetadata={node => setNatural({ width: node.videoWidth, height: node.videoHeight })} overlay={<VideoBoxes plates={plates} texts={words} time={current} media={media} natural={natural} />} /></div>
      {plates.length ? <VideoPlates groups={plates} texts={words} time={current} onSeek={seek} media={media} /> : events.length ? <ol className="video-events">{events.map(event => <li key={event.id} className={event.cited ? 'is-cited' : undefined}><button type="button" onClick={() => seek(event.time)}><time>{formatClock(event.time)}</time><span>{event.label}</span></button><EvidenceStrengthBadge strength={strength(event.confidence, event.cited)} /></li>)}</ol> : <p className="viewer-gap">No timestamped derived events were supplied for this video.</p>}
      </div>
      <canvas ref={canvas} hidden />
      {capture ? <figure className="frame-capture"><img src={capture} alt={`Extracted video frame at ${media.current?.currentTime?.toFixed(1) || 'current'} seconds`} /><figcaption><a href={capture} download="nexusai-frame.png">Download extracted frame</a></figcaption></figure> : null}
    </section>
  )
}
