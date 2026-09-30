import { useEffect, useMemo, useRef, useState } from 'react'
import { EvidenceStrengthBadge } from './Citations.jsx'
import { LanguageText } from './AnalystComponents.jsx'
import { VirtualizedTable } from './VirtualizedTable.jsx'
import { humanizeKey } from '../lib/format.js'
import { semanticCatalog } from '../lib/semanticCatalog.js'
import { documentPages, imageRegions, lineage, transcriptCues, transcriptText, videoEvents } from '../lib/viewerPresentation.js'

function number(value) { const result = Number(value); return Number.isFinite(result) ? result : null }
function parsed(value) { if (!value) return null; try { return JSON.parse(value) } catch { return null } }

function strength(confidence, source = false) {
  if (source) return { id: 'strong', label: 'Source evidence' }
  if (confidence == null) return { id: 'medium', label: 'Candidate observation' }
  if (confidence >= 0.8) return { id: 'strong', label: `High-confidence observation · ${Math.round(confidence * 100)}%` }
  if (confidence >= 0.5) return { id: 'medium', label: `Review observation · ${Math.round(confidence * 100)}%` }
  return { id: 'weak', label: `Low-confidence observation · ${Math.round(confidence * 100)}%` }
}

function spanBounds(value) {
  const span = parsed(value) || value
  if (Array.isArray(span)) return [number(span[0]), number(span[1])]
  if (span && typeof span === 'object') return [number(span.start ?? span.char_start), number(span.end ?? span.char_end)]
  return [null, null]
}

export function HighlightedText({ text, span }) {
  const [start, end] = spanBounds(span)
  if (start === null || end === null || start < 0 || end <= start || start >= text.length) return <LanguageText as="pre" className="source-text">{text}</LanguageText>
  return <LanguageText as="pre" className="source-text" isolate={false}>{text.slice(0, start)}<mark id="exact-source" className="arrival-highlight">{text.slice(start, Math.min(end, text.length))}</mark>{text.slice(end)}</LanguageText>
}

export function LineagePanel({ detail, version }) {
  const model = lineage(detail, version)
  return <aside className="lineage-panel" aria-labelledby="lineage-heading"><h2 id="lineage-heading">Evidence lineage</h2><ol><li><span>Source file</span><strong><LanguageText>{model.source}</LanguageText></strong></li><li><span>Version</span><strong><LanguageText as="bdi" identifier>{model.version}</LanguageText></strong></li>{model.artifacts.map(artifact => <li className="lineage-artifact" key={artifact.id}><div className="lineage-artifact__heading"><span>{artifact.label}</span><EvidenceStrengthBadge strength={strength(artifact.confidence)} /></div><dl><div><dt>Recorded producer</dt><dd><LanguageText as="bdi" identifier>{artifact.producer}</LanguageText></dd></div><div><dt>Producer version</dt><dd><LanguageText as="bdi" identifier>{artifact.producerVersion}</LanguageText></dd></div></dl></li>)}</ol>{!model.artifacts.length ? <p>No derived artifacts were reported for this source.</p> : null}</aside>
}

export function DocumentViewer({ detail, objectUrl, page, charSpan, onPageChange }) {
  const pages = documentPages(detail)
  const requested = number(page) || pages[0]?.number || 1
  const selectedIndex = Math.max(0, pages.findIndex(item => item.number === requested))
  const selected = pages[selectedIndex]
  return <section className="source-viewer document-viewer" aria-labelledby="document-viewer-heading"><header><h2 id="document-viewer-heading">Document source · page {requested}</h2>{pages.length > 1 ? <nav aria-label="Document pages"><button type="button" disabled={selectedIndex === 0} onClick={() => onPageChange(pages[selectedIndex - 1].number)}>Previous page</button><span>Page {selectedIndex + 1} of {pages.length}</span><button type="button" disabled={selectedIndex === pages.length - 1} onClick={() => onPageChange(pages[selectedIndex + 1].number)}>Next page</button></nav> : null}</header>{charSpan ? <EvidenceStrengthBadge strength={strength(null, true)} ariaLabel="Evidence strength" /> : null}{selected?.text ? <HighlightedText text={selected.text} span={charSpan} /> : objectUrl ? <iframe title="Document source" src={`${objectUrl}#page=${requested}`} /> : <p>The retained document cannot be displayed inline.</p>}</section>
}

export function StructuredViewer({ detail, row, recordType }) {
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
  return <section className="source-viewer structured-viewer" aria-labelledby="structured-viewer-heading"><h2 id="structured-viewer-heading">Structured source</h2>{rows.length ? <VirtualizedTable columns={columns} rows={rows} label="Structured evidence rows" initialRow={row} renderCell={cell} exportName="nexusai-structured-evidence.csv" /> : <p>The evidence-detail response did not include a record preview.</p>}</section>
}

function overlayStyle(box, size) {
  const [x, y, width, height] = box
  const normalized = Math.max(...box) <= 1
  return normalized
    ? { insetInlineStart: `${x * 100}%`, insetBlockStart: `${y * 100}%`, inlineSize: `${width * 100}%`, blockSize: `${height * 100}%` }
    : { insetInlineStart: `${(x / size.width) * 100}%`, insetBlockStart: `${(y / size.height) * 100}%`, inlineSize: `${(width / size.width) * 100}%`, blockSize: `${(height / size.height) * 100}%` }
}

export function ImageViewer({ detail, objectUrl, citedBBox }) {
  const regions = useMemo(() => imageRegions(detail, parsed(citedBBox)), [detail, citedBBox])
  const [show, setShow] = useState(regions.length > 0)
  const [size, setSize] = useState({ width: 1, height: 1 })
  return <section className="source-viewer image-viewer" aria-labelledby="image-viewer-heading"><header><h2 id="image-viewer-heading">Image source</h2>{regions.length ? <button type="button" aria-pressed={show} onClick={() => setShow(value => !value)}>{show ? 'Hide observation overlays' : 'Show observation overlays'}</button> : null}</header><div className="image-source"><img src={objectUrl} alt={`Source ${detail.item?.original_filename || 'image'}`} onLoad={event => setSize({ width: event.currentTarget.naturalWidth || 1, height: event.currentTarget.naturalHeight || 1 })} />{show ? regions.map(region => <span key={region.id} className={`evidence-overlay${region.cited ? ' evidence-overlay--cited arrival-highlight' : ''}`} style={overlayStyle(region.bbox, size)}><span>{region.label}</span>{region.confidence == null ? null : <small>{Math.round(region.confidence * 100)}%</small>}</span>) : null}</div>{regions.length ? <ul className="overlay-key">{regions.map(region => <li key={region.id}><span>{region.label}</span><EvidenceStrengthBadge strength={strength(region.confidence, region.cited)} /></li>)}</ul> : <p>No OCR or plate regions were supplied with this evidence.</p>}</section>
}

export function AudioViewer({ detail, objectUrl, sourceTime, charSpan }) {
  const media = useRef(null)
  const cues = useMemo(() => transcriptCues(detail), [detail])
  const [current, setCurrent] = useState(number(sourceTime) || 0)
  useEffect(() => {
    const node = media.current; const target = number(sourceTime)
    if (!node || target === null) return undefined
    const seek = () => { node.currentTime = target; setCurrent(target) }
    node.addEventListener('loadedmetadata', seek, { once: true })
    return () => node.removeEventListener('loadedmetadata', seek)
  }, [objectUrl, sourceTime])
  function seek(time) { if (media.current) { media.current.currentTime = time; media.current.play().catch(() => {}) } }
  const active = cues.findIndex(cue => current >= cue.start && (cue.end == null || current < cue.end))
  const plain = transcriptText(detail)
  return <section className="source-viewer audio-viewer" aria-labelledby="audio-viewer-heading"><h2 id="audio-viewer-heading">Audio source</h2>{sourceTime != null && sourceTime !== '' ? <EvidenceStrengthBadge strength={strength(null, true)} ariaLabel="Evidence strength" /> : null}<audio ref={media} className="evidence-media" controls src={objectUrl} onTimeUpdate={event => setCurrent(event.currentTarget.currentTime)} />{cues.length ? <ol className="transcript-cues">{cues.map((cue, index) => <li key={cue.id} aria-current={active === index ? 'true' : undefined}><button type="button" onClick={() => seek(cue.start)}><time>{cue.start.toFixed(1)}s</time>{cue.speaker ? <strong>{cue.speaker}</strong> : null}<LanguageText>{cue.text}</LanguageText></button></li>)}</ol> : plain ? <><p className="viewer-gap">A transcript was supplied without timestamp cues, so synchronized seeking and speaker turns are unavailable.</p><HighlightedText text={plain} span={charSpan} /></> : <p>No transcript was supplied for this audio.</p>}</section>
}

export function VideoViewer({ detail, objectUrl, frame }) {
  const media = useRef(null)
  const canvas = useRef(null)
  const [duration, setDuration] = useState(number(detail.item?.metadata?.duration_seconds) || 0)
  const [capture, setCapture] = useState(null)
  const events = useMemo(() => videoEvents(detail, frame), [detail, frame])
  useEffect(() => {
    const node = media.current; const target = number(frame)
    if (!node || target === null) return undefined
    const seek = () => { node.currentTime = target }
    node.addEventListener('loadedmetadata', seek, { once: true })
    return () => node.removeEventListener('loadedmetadata', seek)
  }, [objectUrl, frame])
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
  return <section className="source-viewer video-viewer" aria-labelledby="video-viewer-heading"><header><h2 id="video-viewer-heading">Video source</h2><button type="button" onClick={extractFrame}>Extract current frame</button></header><video ref={media} className="evidence-media" controls src={objectUrl} onLoadedMetadata={event => setDuration(event.currentTarget.duration || duration)} />{duration > 0 && events.length ? <div className="event-scrubber" aria-label="Derived events on video timeline">{events.map(event => <button key={event.id} type="button" className={event.cited ? 'arrival-highlight' : undefined} style={{ insetInlineStart: `${Math.min(100, Math.max(0, (event.time / duration) * 100))}%` }} onClick={() => seek(event.time)} aria-label={`${event.label} at ${event.time} seconds`} />)}</div> : null}{events.length ? <ol className="video-events">{events.map(event => <li key={event.id}><button type="button" onClick={() => seek(event.time)}><time>{event.time.toFixed(1)}s</time><span>{event.label}</span></button><EvidenceStrengthBadge strength={strength(event.confidence, event.cited)} /></li>)}</ol> : <p>No timestamped derived events were supplied for this video.</p>}<canvas ref={canvas} hidden />{capture ? <figure className="frame-capture"><img src={capture} alt={`Extracted video frame at ${media.current?.currentTime?.toFixed(1) || 'current'} seconds`} /><figcaption><a href={capture} download="nexusai-frame.png">Download extracted frame</a></figcaption></figure> : null}</section>
}
