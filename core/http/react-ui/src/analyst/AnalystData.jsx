import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useLocation, useNavigate, useOutletContext } from 'react-router-dom'
import { recordsApi } from '../utils/api'
import AnalystAddData from './AnalystAddData'
import AnalystEvidenceRow from './AnalystEvidenceRow'
import { useAnalystPortal } from './AnalystPortalLayout'
import { detailDisclosureModel, evidenceFilterSummary, evidenceFindingPreview, evidenceSourceType as sourceType, evidenceTypePresentation, visibleStatusFilters } from './analystDataPresentation'
import { capabilityForSource, friendlySourceName, sourceDetailPresentation, sourceMaturityPresentation, sourceProductState } from './analystPresentation'
import { analystSourceNavigationContext } from './analystAskPresentation'
import {
  audioDetailPresentation,
  artifactBounds,
  artifactObservation,
  artifactTimestamp,
  faceSimilarityEligible,
  formatTimestamp,
  groupMediaArtifacts,
  mediaArtifactTypes,
  mediaQuickActions,
  sourceProcessingState,
  sourceMediaKind,
  romanUrduBelongsToTranscript,
} from './analystMediaPresentation'

function formatBytes(value) {
  const bytes = Number(value) || 0
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const unit = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / (1024 ** unit)).toLocaleString(undefined, { maximumFractionDigits: 1 })} ${units[unit]}`
}

const sourceCategories = [
  ['all', 'All'], ['structured', 'Structured'], ['documents', 'Documents'], ['images', 'Images'],
  ['anpr', 'ANPR'], ['audio', 'Audio'], ['video', 'Video'],
]

function countByProduct(items, state) {
  return items.filter(item => sourceProductState(item).id === state).length
}


function confidenceLabel(value) {
  const confidence = Number(value)
  return Number.isFinite(confidence) && confidence >= 0 ? `${Math.round(confidence * 100)}% confidence` : 'Confidence not reported'
}

function artifactPresentation(artifact) {
  const type = String(artifact?.artifact_type || '')
  const observation = artifact?.metadata?.observation || artifact?.metadata || {}
  const locator = artifact?.citation_locator || {}
  if (type === 'forensics.face-observation/v1') return {
    icon: 'fa-user-shield', label: 'Face candidate', text: 'Candidate visual similarity; not identity',
    detail: `${confidenceLabel(observation.detection_confidence ?? artifact.confidence)} · ${Number(observation.embedding_dimension || 0)}-dimension candidate embedding`,
  }
  if (type === 'forensics.image-ocr-observation/v1') return {
    icon: 'fa-font', label: 'General image OCR', text: observation.raw_text || 'No OCR text reported',
    detail: `${confidenceLabel(observation.confidence ?? artifact.confidence)} · ${String(observation.script_family || 'script undetermined').replaceAll('_', ' ')} · model candidate`,
  }
  if (type === 'forensics.image-embedding-observation/v1') return {
    icon: 'fa-images', label: 'Semantic image vector', text: 'Candidate semantic visual similarity; not evidence identity or fact',
    detail: `${observation.embedding_model || 'Model not reported'} · ${Number(observation.embedding_dimension || 0)} dimensions · explicit candidate scope only`,
  }
  if (type === 'forensics.audio-roman-urdu-segment/v1') return {
    icon: 'fa-language', label: 'Roman Urdu derivative', text: observation.roman_urdu_text || 'No Roman Urdu text reported',
    detail: `Raw Urdu remains authoritative · identifiers ${observation.identifier_preservation_pass === true ? 'preserved' : 'require review'} · ${formatTimestamp(locator.start_seconds ?? observation.start_seconds)}`,
  }
  if (type === 'forensics.audio-timestamp-segment/v1') return {
    icon: 'fa-waveform-lines', label: 'Machine transcript', text: observation.text || 'No transcript text reported',
    detail: `${observation.requested_language || observation.language || 'language not reported'} · ${formatTimestamp(locator.start_seconds ?? observation.start_seconds)} to ${formatTimestamp(locator.end_seconds ?? observation.end_seconds)} · model observation`,
  }
  if (type === 'forensics.document-native-text-passage/v1') return {
    icon: 'fa-file-lines', label: 'Native document passage', text: artifact?.metadata?.text || 'No passage text reported',
    detail: `${locator.page ? `page ${locator.page}` : locator.paragraph ? `paragraph ${locator.paragraph}` : `section ${locator.section || 1}`} · source-encoded text; no OCR`,
  }
  if (type === mediaArtifactTypes.anpr) return {
    icon: 'fa-car-side', label: 'Plate-region candidate', text: observation.normalized_plate_text || observation.raw_plate_text || 'Unread plate candidate',
    detail: `${confidenceLabel(observation.ocr_confidence ?? artifact.confidence)} · image/version/crop provenance · analyst review required`,
  }
  if (type === 'forensics.video-observation/v1') return {
    icon: 'fa-film', label: 'Video observation', text: 'Bounded source-linked video observation',
    detail: `${locator.timestamp_seconds ?? observation.frame_timestamp_seconds ?? 0}s · no event identity inferred`,
  }
  return { icon: 'fa-file-shield', label: type ? type.replace('forensics.', '').replace('/v1', '').replaceAll('-', ' ') : 'Derived artifact', text: 'Source-bound derived output', detail: 'Review its citation and recorded limitations before use' }
}

function EvidenceCrop({ source, bounds, alt }) {
  const canvasRef = useRef(null)
  const [state, setState] = useState('loading')
  useEffect(() => {
    if (!source || !bounds) return undefined
    let current = true
    const image = new Image()
    image.onload = () => {
      if (!current || !canvasRef.current) return
      const x = Math.max(0, Number(bounds.x) || 0)
      const y = Math.max(0, Number(bounds.y) || 0)
      const width = Math.max(1, Math.min(Number(bounds.width) || 1, image.naturalWidth - x))
      const height = Math.max(1, Math.min(Number(bounds.height) || 1, image.naturalHeight - y))
      const scale = Math.min(1, 720 / Math.max(width, height))
      const canvas = canvasRef.current
      canvas.width = Math.max(1, Math.round(width * scale))
      canvas.height = Math.max(1, Math.round(height * scale))
      canvas.getContext('2d')?.drawImage(image, x, y, width, height, 0, 0, canvas.width, canvas.height)
      setState('ready')
    }
    image.onerror = () => current && setState('error')
    image.src = source
    return () => { current = false }
  }, [bounds, source])
  return <figure className="analyst-evidence-crop" data-state={state}>
    <canvas ref={canvasRef} role="img" aria-label={alt} />
    {state === 'loading' && <span>Reconstructing source crop…</span>}
    {state === 'error' && <span>Crop preview unavailable; source bounds remain recorded.</span>}
    <figcaption>Dynamically reconstructed from authorized source pixels and the recorded bounding box; no new evidence artifact is retained.</figcaption>
  </figure>
}

function resultCitation(artifact) {
  const reference = artifact.citation_ref || (artifact.artifact_id ? `Artifact ${artifact.artifact_id}` : '')
  if (!reference) return null
  return <details className="analyst-finding-citation"><summary><i className="fas fa-link" aria-hidden="true" /> Source reference</summary><code>{reference}</code></details>
}

function CandidateSimilarityPanel({ item, groups, candidates, caseId }) {
  const [kind, setKind] = useState('face')
  const [state, setState] = useState('idle')
  const [result, setResult] = useState(null)
  const [error, setError] = useState('')
  const [compareID, setCompareID] = useState('')
  const imageCandidates = useMemo(() => candidates.filter(candidate => candidate.evidence_id !== item.evidence_id && String(candidate.modality || '').toLowerCase() === 'image'), [candidates, item.evidence_id])
  const names = useMemo(() => Object.fromEntries(candidates.map(candidate => [candidate.evidence_id, friendlySourceName(candidate)])), [candidates])
  const face = groups.faces.find(faceSimilarityEligible)
  const image = groups.imageEmbeddings.find(artifact => Number(artifactObservation(artifact).embedding_dimension || 0) > 0)
  if (!face && !image) return null
  const runRanking = async selectedKind => {
    const query = selectedKind === 'face' ? face : image
    if (!query) return
    setKind(selectedKind)
    setState('loading')
    setError('')
    setResult(null)
    try {
      const response = selectedKind === 'face'
        ? await recordsApi.forensicCaseFaceSimilarity(caseId, { query_face_observation_id: query.metadata?.observation_id, top_k: 10 })
        : await recordsApi.forensicCaseImageSimilarity(caseId, { query_image_observation_id: query.metadata?.observation_id, top_k: 10 })
      setResult(response)
      setState('ready')
    } catch (reason) {
      setError(reason.message)
      setState('error')
    }
  }
  const runComparison = async () => {
    if (!compareID) return
    setKind('comparison')
    setState('loading')
    setError('')
    setResult(null)
    try {
      setResult(await recordsApi.forensicCaseImageCompare(caseId, { evidence_id_a: item.evidence_id, evidence_id_b: compareID }))
      setState('ready')
    } catch (reason) {
      setError(reason.message)
      setState('error')
    }
  }
  const ranked = Array.isArray(result?.results) ? result.results : []
  return <section className="analyst-finding-section analyst-candidate-tools" aria-labelledby="candidate-tools-title">
    <h4 id="candidate-tools-title">Candidate comparison</h4>
    <p>The backend selects authorized current-version image candidates independently of the page currently loaded. Scores rank candidates; they do not establish identity or fact.</p>
    <div className="analyst-candidate-actions">
      {face && <button type="button" onClick={() => runRanking('face')} disabled={state === 'loading'}><i className="fas fa-user-group" /> Rank face candidates</button>}
      {image && <button type="button" onClick={() => runRanking('image')} disabled={state === 'loading'}><i className="fas fa-images" /> Rank similar images</button>}
      <label><span>Compare pixels and observations with</span><select value={compareID} onChange={event => setCompareID(event.target.value)}><option value="">Choose a case image</option>{imageCandidates.map(candidate => <option key={candidate.evidence_id} value={candidate.evidence_id}>{friendlySourceName(candidate)}</option>)}</select></label>
      <button type="button" onClick={runComparison} disabled={!compareID || state === 'loading'}><i className="fas fa-code-compare" /> Compare images</button>
    </div>
    {state === 'loading' && <p role="status"><i className="fas fa-spinner fa-spin" /> Running bounded read-only comparison…</p>}
    {state === 'error' && <div className="analyst-inline-error" role="alert">Comparison unavailable: {error}</div>}
    {state === 'ready' && kind !== 'comparison' && <div className="analyst-ranked-results"><header><strong>{kind === 'face' ? 'Face candidate ranking' : 'Semantic image ranking'}</strong><small>{result?.distance_metric || 'cosine similarity'} · {kind === 'face' ? 'not identity' : 'candidate semantic similarity; not fact'}</small></header>{ranked.length === 0 ? <div className="analyst-zero-results"><i className="fas fa-circle-check" /><span><strong>Complete — 0 eligible candidates</strong><small>No score was inferred.</small></span></div> : ranked.map((entry, index) => { const evidenceID = entry.candidate_evidence_id || entry.source_evidence_id; const score = Number(entry.similarity_score ?? entry.score); return <article key={`${evidenceID}:${entry.candidate_face_observation_id || entry.candidate_image_observation_id || index}`}><span>{entry.rank || index + 1}</span><div><strong>{names[evidenceID] || evidenceID}</strong><small>{Number.isFinite(score) ? score.toFixed(4) : 'Score unavailable'} · {entry.review_state || 'model candidate'}</small><code>{entry.citation_ref || 'Citation unavailable'}</code></div></article> })}</div>}
    {state === 'ready' && kind === 'comparison' && <div className="analyst-comparison-result"><strong>Image comparison result</strong><dl><div><dt>Exact duplicate</dt><dd>{result?.exact_duplicate === true ? 'Yes' : 'No'}</dd></div><div><dt>Perceptual similarity</dt><dd>{Number.isFinite(Number(result?.perceptual?.similarity)) ? Number(result.perceptual.similarity).toFixed(4) : 'Unavailable'}</dd></div><div><dt>Semantic similarity</dt><dd>{Number.isFinite(Number(result?.visual_similarity?.score)) ? Number(result.visual_similarity.score).toFixed(4) : 'Unavailable'}</dd></div><div><dt>Near-duplicate candidate</dt><dd>{result?.perceptual?.near_duplicate_candidate === true ? 'Yes' : 'No'}</dd></div></dl><p>{(result?.limitations || []).join(' ')}</p></div>}
  </section>
}

function MediaFindings({ item, artifacts, jobs, source, onSelect, onSeek, casePath, candidates, caseId }) {
  const [showAllOCR, setShowAllOCR] = useState(false)
  const groups = useMemo(() => groupMediaArtifacts(artifacts), [artifacts])
  const state = sourceProcessingState(item, jobs, artifacts)
  const modality = String(item?.modality || '').toLowerCase()
  const isMedia = ['image', 'audio', 'video'].includes(modality)
  if (!isMedia) return null
  const displayedPlates = modality === 'video' && groups.plateGroups.length > 0 ? groups.plateGroups : groups.plates
  const findingCount = displayedPlates.length + groups.faces.length + groups.ocr.length + groups.audio.length + groups.romanUrdu.length
  const quickActions = mediaQuickActions(item, groups)
  const shownOCR = showAllOCR ? groups.ocr : groups.ocr.slice(0, 6)
  const frameTimes = [...new Set([...groups.plates, ...groups.ocr].map(artifactTimestamp).filter(value => value !== null))].sort((a, b) => a - b)
  return <section id="source-intelligence" className="analyst-media-results" aria-labelledby="source-intelligence-title">
    <header>
      <div><span className="analyst-eyebrow">Findings</span><h3 id="source-intelligence-title">What was found</h3></div>
      <span className="analyst-result-state" data-tone={state.tone}>{state.label}</span>
    </header>
    {state.id === 'COMPLETE_RESULTS' && findingCount === 0 && <div className="analyst-zero-results"><i className="fas fa-circle-check" /><span><strong>Processing complete — zero media findings</strong><small>No plate, face, recognized-text, or transcript observation is recorded. No missing result was inferred.</small></span></div>}
    {state.id === 'COMPLETE_ZERO_RESULTS' && <div className="analyst-zero-results"><i className="fas fa-circle-check" /><span><strong>Processing complete — zero derived observations</strong><small>This is a completed negative result, not a processor failure.</small></span></div>}
    {displayedPlates.length > 0 && <section className="analyst-finding-section" aria-labelledby="detected-plates-title">
      <h4 id="detected-plates-title">Detected plates <span>{displayedPlates.length}</span></h4>
      <div className="analyst-finding-grid">{displayedPlates.map(artifact => {
        const grouped = artifact.artifact_type === mediaArtifactTypes.videoAnprGroup
        const groupObservation = artifactObservation(artifact)
        const bestArtifact = grouped ? groups.plates.find(item => item?.metadata?.observation_id === groupObservation.best_observation_id) : artifact
        const observation = grouped ? { ...artifactObservation(bestArtifact), ...groupObservation } : groupObservation
        const bounds = artifactBounds(bestArtifact)
        const plate = observation.normalized_plate_text || observation.raw_plate_text || 'Unread plate candidate'
        return <article key={artifact.artifact_id} className="analyst-plate-card">
          <span className="analyst-finding-kicker">{grouped ? 'Grouped plate observations' : 'Plate observation'}</span>
          <strong className="analyst-plate-text"><bdi dir="ltr">{plate}</bdi></strong>
          <button type="button" onClick={() => { onSelect(bestArtifact || artifact); const time = artifactTimestamp(artifact); if (time !== null) onSeek?.(time) }}><i className="fas fa-crosshairs" /> {artifactTimestamp(artifact) !== null ? `Open at ${formatTimestamp(artifactTimestamp(artifact))}` : 'Show in source'}</button>
          <dl><div><dt>Detector</dt><dd>{confidenceLabel(observation.detection_confidence)}</dd></div><div><dt>Plate OCR</dt><dd>{confidenceLabel(observation.ocr_confidence ?? artifact.confidence)}</dd></div>{artifactTimestamp(artifact) !== null && <div><dt>{grouped ? 'First seen' : 'Frame'}</dt><dd>{formatTimestamp(artifactTimestamp(artifact))}</dd></div>}{grouped && <div><dt>Sightings</dt><dd>{Number(observation.sightings_count || 0)} · through {formatTimestamp(observation.last_seen_seconds)}</dd></div>}<div><dt>Review</dt><dd>{observation.manual_review_required === false ? 'Not required' : 'Analyst review required'}</dd></div></dl>
          {bounds && modality === 'image' && <EvidenceCrop source={source} bounds={bounds} alt={`Recorded crop for plate candidate ${plate}`} />}
          {resultCitation(artifact)}
        </article>
      })}</div>
    </section>}
    {modality === 'video' && <section className="analyst-finding-section" aria-labelledby="video-plate-timeline-title">
      <h4 id="video-plate-timeline-title">Plate timeline</h4>
      {displayedPlates.length === 0 ? <div className="analyst-zero-results"><i className="fas fa-film" /><span><strong>Plate processing complete — 0 detections</strong><small>The retained video has no ANPR observation. This is not a positive plate sighting.</small></span></div> : <div className="analyst-timeline">{displayedPlates.map(artifact => <button type="button" key={artifact.artifact_id} onClick={() => onSeek?.(artifactTimestamp(artifact) || 0)}><time>{formatTimestamp(artifactTimestamp(artifact))}</time><strong>{artifactObservation(artifact).normalized_plate_text || 'Plate candidate'}</strong>{artifact.artifact_type === mediaArtifactTypes.videoAnprGroup && <small>{artifactObservation(artifact).sightings_count} sightings · grouped OCR, not tracking</small>}</button>)}</div>}
    </section>}
    {groups.faces.length > 0 && <section className="analyst-finding-section" aria-labelledby="detected-faces-title">
      <h4 id="detected-faces-title">Detected faces <span>{groups.faces.length}</span></h4>
      <p>Face observations and similarity are model candidates, not identity determinations.</p>
      <div className="analyst-finding-grid">{groups.faces.map((artifact, index) => {
        const observation = artifactObservation(artifact)
        const bounds = artifactBounds(artifact)
        const eligible = faceSimilarityEligible(artifact)
        return <article key={artifact.artifact_id}>
          <button type="button" onClick={() => onSelect(artifact)}><i className="fas fa-crosshairs" /> Focus face {index + 1}</button>
          <strong>Face candidate {index + 1}</strong>
          <small>{confidenceLabel(observation.detection_confidence ?? artifact.confidence)} · {bounds ? `${bounds.width} × ${bounds.height}px crop` : 'bounds unavailable'}</small>
          {bounds && modality === 'image' && <EvidenceCrop source={source} bounds={bounds} alt={`Recorded crop for face candidate ${index + 1}`} />}
          <span className="analyst-candidate-state" data-ready={eligible}>{eligible ? 'Candidate similarity available in explicit case scope' : 'Similarity unavailable — no valid embedding recorded'}</span>
          {resultCitation(artifact)}
        </article>
      })}</div>
    </section>}
    {groups.ocr.length > 0 && <section className="analyst-finding-section" aria-labelledby="detected-text-title">
      <h4 id="detected-text-title">Detected text <span>{groups.ocr.length}</span></h4>
      <p>Whitespace-only output is removed. Low-confidence and unusual-script outputs remain visible as review-required model observations.</p>
      <div className="analyst-ocr-list">{shownOCR.map(artifact => {
        const observation = artifactObservation(artifact)
        const timestamp = artifactTimestamp(artifact)
        return <button type="button" key={artifact.artifact_id} onClick={() => { onSelect(artifact); if (timestamp !== null) onSeek?.(timestamp) }}>
          <bdi dir="auto">{observation.raw_text}</bdi><small>{confidenceLabel(observation.confidence ?? artifact.confidence)} · {String(observation.script_family || 'script undetermined').replaceAll('_', ' ')}{timestamp === null ? '' : ` · ${formatTimestamp(timestamp)}`}</small>
        </button>
      })}</div>
      {groups.ocr.length > 6 && <button className="analyst-text-action" type="button" onClick={() => setShowAllOCR(value => !value)}>{showAllOCR ? 'Show first 6 observations' : `Show all ${groups.ocr.length} observations`}</button>}
    </section>}
    {modality === 'video' && frameTimes.length > 0 && <section className="analyst-finding-section" aria-labelledby="video-frame-observations-title"><h4 id="video-frame-observations-title">Frame observations</h4><div className="analyst-timeline">{frameTimes.map(time => <button type="button" key={time} onClick={() => onSeek?.(time)}><time>{formatTimestamp(time)}</time><strong>{groups.plates.filter(item => artifactTimestamp(item) === time).length} plate · {groups.ocr.filter(item => artifactTimestamp(item) === time).length} text</strong></button>)}</div></section>}
    {(groups.audio.length > 0 || groups.romanUrdu.length > 0) && <section className="analyst-finding-section" aria-labelledby="recorded-speech-title"><h4 id="recorded-speech-title">Recorded speech</h4><p>Machine transcript · review recommended. Urdu source text remains primary; Roman Urdu is a reading aid.</p><div className="analyst-transcript-list">{groups.audio.map(artifact => {
      const observation = artifactObservation(artifact)
      const timestamp = artifactTimestamp(artifact)
      const roman = groups.romanUrdu.find(candidate => romanUrduBelongsToTranscript(candidate, artifact))
      const romanObservation = roman ? artifactObservation(roman) : null
      return <article key={artifact.artifact_id}><header><strong>Transcript</strong><time>{formatTimestamp(timestamp)} · {observation.requested_language || observation.language || 'language not reported'}</time></header><bdi className="analyst-transcript-primary" dir="auto">{observation.text}</bdi>{romanObservation?.roman_urdu_text && <div className="analyst-roman-urdu"><span>Roman Urdu</span><bdi dir="ltr">{romanObservation.roman_urdu_text}</bdi></div>}{resultCitation(artifact)}</article>
    })}{groups.romanUrdu.filter(roman => !groups.audio.some(audio => romanUrduBelongsToTranscript(roman, audio))).map(artifact => { const observation = artifactObservation(artifact); return <article key={artifact.artifact_id}><header><strong>Roman Urdu</strong><time>{formatTimestamp(artifactTimestamp(artifact))}</time></header><bdi className="analyst-transcript-primary" dir="ltr">{observation.roman_urdu_text}</bdi><small>Secondary representation; source-language transcript was not included in this bounded view.</small>{resultCitation(artifact)}</article> })}</div></section>}
    {modality === 'image' && <CandidateSimilarityPanel item={item} groups={groups} candidates={candidates} caseId={caseId} />}
    {quickActions.length > 0 && <section className="analyst-source-actions analyst-media-quick-actions"><h4>Use these findings</h4>{quickActions.map(action => <Link key={action.id} to={`${casePath}${casePath.includes('?') ? '&' : '?'}evidence=${encodeURIComponent(item.evidence_id)}&prompt=${encodeURIComponent(action.prompt)}`}><i className={`fas ${action.icon}`} /> {action.label}</Link>)}</section>}
  </section>
}

function SourceMediaReview({ item, artifacts, jobs, caseId, casePath, candidates, sourceTime }) {
  const modality = sourceMediaKind(item)
  const source = item?.evidence_id ? recordsApi.forensicCaseEvidenceContentUrl(caseId, item.evidence_id) : ''
  const metadata = item?.media_metadata || {}
  const videoTrack = Array.isArray(metadata.tracks) ? metadata.tracks.find(track => track.handler_type === 'vide') : null
  const width = Number(metadata.width_pixels || videoTrack?.width_pixels || 0)
  const height = Number(metadata.height_pixels || videoTrack?.height_pixels || 0)
  const playerRef = useRef(null)
  const [selectedArtifact, setSelectedArtifact] = useState(null)
  const [visible, setVisible] = useState({ plate: true, face: true, ocr: true })
  const [imageZoom, setImageZoom] = useState(1)
  useEffect(() => {
    const player = playerRef.current
    const seconds = Number(sourceTime)
    if (!player || !Number.isFinite(seconds) || seconds < 0) return undefined
    const applySourceTime = () => { player.currentTime = seconds }
    if (player.readyState >= 1) applySourceTime()
    else player.addEventListener('loadedmetadata', applySourceTime, { once: true })
    return () => player.removeEventListener('loadedmetadata', applySourceTime)
  }, [sourceTime, item?.evidence_id])
  if (!['image', 'audio', 'video'].includes(modality) || !item?.evidence_id) return null
  const audioFacts = modality === 'audio' ? audioDetailPresentation(item, artifacts) : null
  const regions = artifacts.map(artifact => {
    const observation = artifactObservation(artifact)
    const bounds = artifactBounds(artifact)
    const type = String(artifact?.artifact_type || '')
    const kind = type === mediaArtifactTypes.face ? 'face' : type === mediaArtifactTypes.ocr ? 'ocr' : type === mediaArtifactTypes.anpr ? 'plate' : ''
    const label = kind === 'plate' ? observation.normalized_plate_text || 'Plate candidate' : kind === 'face' ? 'Face candidate' : observation.raw_text || 'OCR'
    return { id: artifact.artifact_id, artifact, bounds, kind, label, timestamp: artifactTimestamp(artifact) }
  }).filter(region => region.kind && region.bounds && width > 0 && height > 0)
  const seek = seconds => {
    if (!playerRef.current || seconds === null) return
    playerRef.current.currentTime = Math.max(0, Number(seconds) || 0)
    playerRef.current.play?.().catch(() => {})
  }
  const previewTitle = modality === 'image' ? 'Image preview' : modality === 'audio' ? 'Listen to recording' : 'Video preview'
  const resetImageView = () => {
    setImageZoom(1)
    setSelectedArtifact(null)
    setVisible({ plate: true, face: true, ocr: true })
  }
  return <section className="analyst-derived-intelligence" aria-labelledby="source-media-review-title">
    <h3 id="source-media-review-title">{previewTitle}</h3>
    {modality === 'image' && <div className="analyst-media-toolbar" role="toolbar" aria-label="Image view controls">
      <button type="button" aria-label="Zoom out" onClick={() => setImageZoom(value => Math.max(1, Number((value - .25).toFixed(2))))} disabled={imageZoom <= 1}><i className="fas fa-magnifying-glass-minus" /></button>
      <output aria-live="polite">{Math.round(imageZoom * 100)}%</output>
      <button type="button" aria-label="Zoom in" onClick={() => setImageZoom(value => Math.min(3, Number((value + .25).toFixed(2))))} disabled={imageZoom >= 3}><i className="fas fa-magnifying-glass-plus" /></button>
      <button type="button" onClick={() => setImageZoom(1)}>Fit</button>
      <button type="button" onClick={resetImageView}>Reset view</button>
    </div>}
    {regions.length > 0 && <div className="analyst-overlay-controls" aria-label="Visible findings">{[['plate', 'Plates'], ['face', 'Faces'], ['ocr', 'Text']].filter(([kind]) => regions.some(region => region.kind === kind)).map(([kind, label]) => <button key={kind} type="button" aria-pressed={visible[kind]} onClick={() => setVisible(current => ({ ...current, [kind]: !current[kind] }))}><i className={`fas ${visible[kind] ? 'fa-eye' : 'fa-eye-slash'}`} /> {label}</button>)}</div>}
    {modality === 'image' && <div className="analyst-media-stage" style={{ aspectRatio: width > 0 && height > 0 ? `${width} / ${height}` : '16 / 9' }}>
      <div className="analyst-media-canvas" style={{ transform: `scale(${imageZoom})` }}>
        <img src={source} alt={`Retained evidence preview for ${item.original_filename || item.source_file || 'image'}`} loading="lazy" />
        {regions.filter(region => visible[region.kind]).map(region => <button type="button" className={selectedArtifact?.artifact_id === region.id ? 'active' : ''} data-kind={region.kind} key={region.id} title={`${region.label} — focus result`} aria-label={`Focus ${region.label}`} onClick={() => setSelectedArtifact(region.artifact)} style={{ left: `${Number(region.bounds.x) / width * 100}%`, top: `${Number(region.bounds.y) / height * 100}%`, width: `${Number(region.bounds.width) / width * 100}%`, height: `${Number(region.bounds.height) / height * 100}%` }}><b>{region.label}</b></button>)}
      </div>
    </div>}
    {modality === 'audio' && <div className="analyst-audio-source-summary">
      <div className="analyst-audio-source-icon"><i className="fas fa-waveform-lines" aria-hidden="true" /><span>{audioFacts.transcriptSegmentCount > 0 ? 'Machine transcript available' : 'No transcript recorded'}</span></div>
      <dl>
        <div><dt>Duration</dt><dd>{audioFacts.durationSeconds === null ? 'Not reported' : formatTimestamp(audioFacts.durationSeconds)}</dd></div>
        <div><dt>Language</dt><dd>{audioFacts.language || 'Not reported'}</dd></div>
        <div><dt>Review</dt><dd>{audioFacts.asrStatus}</dd></div>
        <div><dt>Transcript</dt><dd>{audioFacts.transcriptSegmentCount} segments · {audioFacts.romanUrduSegmentCount} Roman Urdu</dd></div>
      </dl>
      <audio ref={playerRef} src={source} controls preload="metadata">Audio preview is not supported by this browser.</audio>
      {(audioFacts.format || audioFacts.model) && <details className="analyst-audio-details"><summary>Audio details</summary><dl>{audioFacts.format && <div><dt>Format</dt><dd>{audioFacts.format}</dd></div>}{audioFacts.model && <div><dt>Transcript processor</dt><dd>{audioFacts.model}</dd></div>}</dl></details>}
    </div>}
    {modality === 'video' && <div className="analyst-video-stage"><video ref={playerRef} src={source} controls preload="metadata">Video preview is not supported by this browser.</video></div>}
    <p>This preview comes from the retained source. Highlighted regions and machine-produced text should be reviewed.</p>
    <MediaFindings item={item} artifacts={artifacts} jobs={jobs} source={source} onSelect={setSelectedArtifact} onSeek={seek} casePath={casePath} candidates={candidates} caseId={caseId} />
  </section>
}

function SourceDocumentReview({ item, artifacts, caseId }) {
  const modality = String(item?.modality || '').toLowerCase()
  const detectedType = sourceType(item)
  if (!['document', 'text'].includes(modality) && !['document', 'text', 'pdf'].some(type => detectedType.includes(type))) return null
  if (!item?.evidence_id) return null
  const source = recordsApi.forensicCaseEvidenceContentUrl(caseId, item.evidence_id)
  const filename = String(item.original_filename || item.source_file || 'document').split(/[\\/]/).pop()
  const extension = filename.includes('.') ? filename.split('.').pop().toLowerCase() : ''
  const passages = artifacts.filter(artifact => String(artifact?.artifact_type || '').includes('document-native-text-passage'))
  const sourceAction = extension === 'docx' ? 'Download original DOCX' : 'Open original source'
  return <section className="analyst-derived-intelligence analyst-document-review" aria-labelledby="source-document-review-title">
    <h3 id="source-document-review-title">Document preview</h3>
    {['txt', 'md', 'log'].includes(extension) && <iframe className="analyst-document-frame" src={source} sandbox="" title={`Retained text source ${filename}`} />}
    {extension === 'pdf' && <object className="analyst-document-frame" data={source} type="application/pdf" aria-label={`Retained PDF source ${filename}`}><p>PDF preview is unavailable in this browser. Use the source action below.</p></object>}
    {extension === 'docx' && <p>DOCX has no reliable browser-native renderer. Retained extracted passages appear below when processing has completed, and the original remains available as a governed source action.</p>}
    {!['txt', 'md', 'log', 'pdf', 'docx'].includes(extension) && <p>This document type has no safe inline browser preview. It is not reinterpreted as another format.</p>}
    <a className="analyst-source-action" href={source} target="_blank" rel="noreferrer"><i className="fas fa-arrow-up-right-from-square" /> {sourceAction}</a>
    <p>{passages.length > 0 ? `${passages.length} searchable passage${passages.length === 1 ? '' : 's'} available from this source.` : 'No extracted text is available for this source yet.'}</p>
  </section>
}

function DerivedIntelligence({ artifacts }) {
  if (!artifacts.length) return null
  return <section id="source-intelligence" className="analyst-derived-intelligence" aria-labelledby="source-intelligence-title">
    <h3 id="source-intelligence-title">Derived intelligence</h3>
    <p>Every item below is source-bound, versioned, cited, and limited to its recorded model or deterministic contract.</p>
    <div className="analyst-derived-list">{artifacts.map(artifact => {
      const presentation = artifactPresentation(artifact)
      return <article key={artifact.artifact_id}>
        <i className={`fas ${presentation.icon}`} aria-hidden="true" />
        <span><strong>{presentation.label}</strong><bdi dir="auto">{presentation.text}</bdi><small>{presentation.detail}</small>{resultCitation(artifact)}</span>
      </article>
    })}</div>
  </section>
}

export default function AnalystData({ embedded = false, onRequestAdd }) {
  const portal = useAnalystPortal()
  const { addToast } = useOutletContext()
  const location = useLocation()
  const navigate = useNavigate()
  const closeButtonRef = useRef(null)
  const addButtonRef = useRef(null)
  const sourceDrawerRef = useRef(null)
  const sourceReturnFocusRef = useRef(null)
  const firstPage = useMemo(() => Array.isArray(portal.dataState.evidence?.items) ? portal.dataState.evidence.items : [], [portal.dataState.evidence])
  const summary = portal.dataState.evidence?.summary || {}
  const [items, setItems] = useState(firstPage)
  const [hasNext, setHasNext] = useState(Boolean(portal.dataState.evidence?.pagination?.has_next))
  const [loadingMore, setLoadingMore] = useState(false)
  const [loadError, setLoadError] = useState('')
  const [query, setQuery] = useState('')
  const [statusFilter, setStatusFilter] = useState('all')
  const [typeFilter, setTypeFilter] = useState('all')
  const [dateFilter, setDateFilter] = useState('all')
  const [sortOrder, setSortOrder] = useState('recent')
  const [selected, setSelected] = useState(null)
  const [detail, setDetail] = useState(null)
  const [detailState, setDetailState] = useState('idle')
  const [reprocessPlan, setReprocessPlan] = useState(null)
  const [addFiles, setAddFiles] = useState([])
  const params = new URLSearchParams(location.search)
  const requestedEvidence = params.get('evidence') || ''
  const citationContext = analystSourceNavigationContext(params)
  const addOpen = params.get('add') === 'true'

  useEffect(() => {
    setItems(firstPage)
    setHasNext(Boolean(portal.dataState.evidence?.pagination?.has_next))
  }, [firstPage, portal.activeWorkspace.caseId, portal.dataState.evidence?.pagination?.has_next])

  const filteredItems = useMemo(() => items.filter(item => {
    const status = sourceProductState(item)
    const haystack = `${friendlySourceName(item)} ${item.original_filename || ''} ${item.source_file || ''} ${sourceType(item)}`.toLowerCase()
    if (query.trim() && !haystack.includes(query.trim().toLowerCase())) return false
    if (statusFilter !== 'all' && status.id !== statusFilter) return false
    if (typeFilter !== 'all' && evidenceTypePresentation(item).category !== typeFilter) return false
    if (dateFilter !== 'all') {
      const created = new Date(item.created_at).getTime()
      const cutoff = Date.now() - Number(dateFilter) * 24 * 60 * 60 * 1000
      if (!Number.isFinite(created) || created < cutoff) return false
    }
    return true
  }).sort((a, b) => sortOrder === 'name'
    ? friendlySourceName(a).localeCompare(friendlySourceName(b))
    : sortOrder === 'oldest'
      ? (Date.parse(a.created_at) || 0) - (Date.parse(b.created_at) || 0)
      : (Date.parse(b.created_at) || 0) - (Date.parse(a.created_at) || 0)), [dateFilter, items, query, statusFilter, typeFilter, sortOrder])

  const openSource = async item => {
    if (!item?.evidence_id) return
    setSelected(item)
    const nextParams = new URLSearchParams(location.search)
    nextParams.set('evidence', item.evidence_id)
    nextParams.delete('add')
    navigate(`${location.pathname}?${nextParams.toString()}`, { replace: true })
    setDetail(null)
    setReprocessPlan(null)
    setDetailState('loading')
    try {
      // Media presentation is artifact-driven. Request the bounded endpoint
      // maximum so a late-created ANPR/face result is never hidden behind a
      // small generic preview limit; raw record values are not needed here.
      const response = await recordsApi.forensicCaseEvidenceDetail(portal.activeWorkspace.caseId, item.evidence_id, { limit: 250, include_records_preview: false })
      setDetail(response)
      setDetailState('ready')
      if (sourceProductState(response?.item).id === 'failed') {
        try {
          setReprocessPlan(await recordsApi.forensicCaseEvidenceReprocessPlan(portal.activeWorkspace.caseId, item.evidence_id))
        } catch (error) {
          setReprocessPlan({ error: error.message })
        }
      }
    } catch (error) {
      setDetail({ error: error.message })
      setDetailState('error')
    }
  }

  useEffect(() => {
    if (!requestedEvidence || selected?.evidence_id === requestedEvidence) return
    openSource(items.find(item => item.evidence_id === requestedEvidence) || { evidence_id: requestedEvidence, source_file: requestedEvidence })
  // The deep link is resolved only when its workspace or evidence identifier changes.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [portal.activeWorkspace.caseId, requestedEvidence])

  const closeSource = () => {
    setSelected(null)
    setDetail(null)
    setDetailState('idle')
    setReprocessPlan(null)
    const nextParams = new URLSearchParams(location.search)
    nextParams.delete('evidence')
    nextParams.delete('source_time')
    nextParams.delete('page')
    nextParams.delete('row')
    nextParams.delete('finding')
    navigate(`${location.pathname}${nextParams.size ? `?${nextParams}` : ''}`, { replace: true })
  }

  useEffect(() => {
    if (!selected) return undefined
    sourceReturnFocusRef.current = document.activeElement
    const drawer = sourceDrawerRef.current
    const focusableSelector = 'button:not(:disabled), [href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])'
    const handleDrawerKey = event => {
      if (event.key === 'Escape') { closeSource(); return }
      if (event.key !== 'Tab' || !drawer) return
      const focusable = Array.from(drawer.querySelectorAll(focusableSelector)).filter(element => element.offsetParent !== null)
      if (!focusable.length) return
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
      if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
    }
    window.addEventListener('keydown', handleDrawerKey)
    closeButtonRef.current?.focus({ preventScroll: true })
    return () => {
      window.removeEventListener('keydown', handleDrawerKey)
      sourceReturnFocusRef.current?.focus?.({ preventScroll: true })
    }
  // Closing is intentionally bound only to drawer visibility.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [Boolean(selected)])

  const openAddData = (files = []) => {
    if (embedded && onRequestAdd) {
      onRequestAdd(files)
      return
    }
    setAddFiles(Array.from(files || []))
    const nextParams = new URLSearchParams(location.search)
    nextParams.set('add', 'true')
    nextParams.delete('evidence')
    navigate(`${location.pathname}?${nextParams.toString()}`, { replace: true })
  }

  const closeAddData = () => {
    const nextParams = new URLSearchParams(location.search)
    nextParams.delete('add')
    navigate(`${location.pathname}${nextParams.size ? `?${nextParams}` : ''}`, { replace: true })
    setAddFiles([])
    window.setTimeout(() => addButtonRef.current?.focus(), 0)
  }

  const loadMore = async () => {
    setLoadingMore(true)
    setLoadError('')
    try {
      const response = await recordsApi.forensicCaseEvidence(portal.activeWorkspace.caseId, { limit: 12, offset: items.length })
      const incoming = Array.isArray(response?.items) ? response.items : []
      setItems(current => [...current, ...incoming.filter(item => !current.some(existing => existing.evidence_id === item.evidence_id))])
      setHasNext(Boolean(response?.pagination?.has_next))
    } catch (error) {
      setLoadError(error.message)
    } finally {
      setLoadingMore(false)
    }
  }

  const statusCounts = summary.status_counts || {}
  const registered = Number(statusCounts.registered || 0)
  const tabCounts = {
    all: Number(summary.evidence_total || items.length),
    ready: Number(summary.completed || countByProduct(items, 'ready')),
    processing: Math.max(0, Number(summary.processing || 0) + Number(summary.queued || 0) - registered),
    attention: registered + Number(statusCounts.manual_review || 0) + Number(statusCounts.needs_review || 0),
    failed: Number(summary.failed || countByProduct(items, 'failed')),
  }
  const statusFilters = visibleStatusFilters(tabCounts)
  const filtersActive = Boolean(query.trim() || statusFilter !== 'all' || typeFilter !== 'all' || dateFilter !== 'all')
  const clearFilters = () => {
    setQuery('')
    setStatusFilter('all')
    setTypeFilter('all')
    setDateFilter('all')
  }

  return (
    <div className={`analyst-page analyst-data-page${embedded ? ' analyst-data-page--embedded' : ''}`}>
      <section className="analyst-data-hero" onDragOver={event => event.preventDefault()} onDrop={event => { event.preventDefault(); openAddData(event.dataTransfer.files) }}>
        <div><span className="analyst-eyebrow">Evidence library</span><h1>Evidence</h1><p>Find a source, understand what it contains, and open its verified findings.</p></div>
        <div className="analyst-data-hero__actions">
          <span><strong>{tabCounts.all.toLocaleString()}</strong> sources <i aria-hidden="true">·</i> <strong>{tabCounts.ready.toLocaleString()}</strong> ready</span>
          <button ref={addButtonRef} className="analyst-primary-action" type="button" onClick={() => openAddData()}><i className="fas fa-plus" /> Add data</button>
        </div>
      </section>

      <section className="analyst-data-controls" aria-label="Find and filter evidence">
        <label className="analyst-search-field"><i className="fas fa-magnifying-glass" aria-hidden="true" /><span className="sr-only">Search sources</span><input value={query} onChange={event => setQuery(event.target.value)} placeholder="Search sources" /></label>
        <label className="analyst-filter-field"><span>Type</span><select value={typeFilter} onChange={event => setTypeFilter(event.target.value)}>{sourceCategories.map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
        <label className="analyst-filter-field"><span>Sort loaded sources</span><select value={sortOrder} onChange={event => setSortOrder(event.target.value)}><option value="recent">Newest first</option><option value="oldest">Oldest first</option><option value="name">Source name</option></select></label>
        <details className="analyst-more-filters">
          <summary><i className="fas fa-sliders" aria-hidden="true" /> More filters</summary>
          <div><label><span>Added</span><select value={dateFilter} onChange={event => setDateFilter(event.target.value)}><option value="all">Any date</option><option value="7">Last 7 days</option><option value="30">Last 30 days</option><option value="90">Last 90 days</option></select></label>{filtersActive && <button type="button" onClick={clearFilters}>Clear filters</button>}</div>
        </details>
        <button className="analyst-refresh-button" type="button" onClick={portal.refresh} disabled={portal.dataState.status === 'loading'} aria-label="Refresh workspace sources"><i className={`fas ${portal.dataState.status === 'loading' ? 'fa-spinner fa-spin' : 'fa-rotate'}`} /></button>
      </section>

      <div className="analyst-status-filters" role="group" aria-label="Filter sources by status">
        {statusFilters.map(([value, label]) => <button key={value} type="button" className={statusFilter === value ? 'active' : ''} aria-pressed={statusFilter === value} onClick={() => setStatusFilter(value)}>{label}<span>{tabCounts[value]}</span></button>)}
      </div>

      <section className="analyst-evidence-library" aria-label="Workspace sources" aria-busy={portal.dataState.status === 'loading' || loadingMore}>
        <header><div><h2>Source inventory</h2><p>{filtersActive ? evidenceFilterSummary(filteredItems.length, hasNext) : `${items.length} loaded sources · ${sortOrder === 'name' ? 'Source name' : sortOrder === 'oldest' ? 'Oldest first' : 'Most recently added first'}`}</p></div>{filtersActive && <button type="button" onClick={clearFilters}>Clear filters</button>}</header>
        <div className="analyst-evidence-list">
        {filteredItems.map(item => {
          const status = sourceProductState(item)
          const capability = capabilityForSource(item, portal.dataState.capabilities)
          return <AnalystEvidenceRow key={item.evidence_id} item={item} status={status} capability={capability} onOpen={() => openSource(item)} />
        })}
        {!filteredItems.length && portal.dataState.status === 'ready' && <div className="analyst-empty"><i className={`fas ${items.length || hasNext ? 'fa-magnifying-glass' : 'fa-folder-plus'}`} /><h2>{hasNext ? 'No matches in loaded sources' : items.length ? 'No sources match these filters' : 'No data sources yet'}</h2><p>{hasNext ? 'Load more sources below to check the rest of this workspace.' : items.length ? 'Try a different search, type, date, or status.' : 'Add one or several files. Each source will be preserved, inspected, classified, and prepared.'}</p>{!items.length && !hasNext && <button className="analyst-primary-action" type="button" onClick={() => openAddData()}><i className="fas fa-plus" /> Add Data</button>}</div>}
        </div>
      </section>
      {loadError && <div className="analyst-inline-error" role="alert">More sources could not be loaded: {loadError}</div>}
      {hasNext && <div className="analyst-load-more"><button className="analyst-secondary-action" type="button" onClick={loadMore} disabled={loadingMore}>{loadingMore ? <><i className="fas fa-spinner fa-spin" /> Loading sources</> : 'Load more sources'}</button></div>}

      {!embedded && addOpen && <AnalystAddData
        key={`${portal.activeWorkspace.caseId}:${addFiles.map(file => file.name).join('|')}`}
        caseId={portal.activeWorkspace.caseId}
        collectionId={portal.activeWorkspace.collectionId}
        capabilities={portal.dataState.capabilities}
        initialFiles={addFiles}
        onClose={closeAddData}
        onComplete={async () => { await portal.refresh(); addToast?.('Source catalog refreshed.', 'success') }}
        onOpenSource={(evidenceId, sourceFile) => { closeAddData(); openSource({ evidence_id: evidenceId, source_file: sourceFile }) }}
      />}

      {selected && (
        <div className="analyst-source-drawer-backdrop" role="presentation" onMouseDown={event => { if (event.target === event.currentTarget) closeSource() }}>
          <aside ref={sourceDrawerRef} className="analyst-source-drawer" role="dialog" aria-modal="true" aria-labelledby="analyst-source-title">
            <header className="analyst-evidence-detail-header">
              <span className="analyst-evidence-detail-header__icon" aria-hidden="true"><i className={`fas ${evidenceTypePresentation(detail?.item || selected).icon}`} /></span>
              <div><span>{evidenceTypePresentation(detail?.item || selected).label}</span><h2 id="analyst-source-title">{friendlySourceName(detail?.item || selected)}</h2></div>
              <button ref={closeButtonRef} className="analyst-drawer-close" type="button" onClick={closeSource} aria-label="Close source details"><i className="fas fa-xmark" /></button>
            </header>
            {citationContext.length > 0 && <section className="analyst-source-citation-context" aria-label="Citation context"><i className="fas fa-link" aria-hidden="true" /><span><strong>Opened from a citation</strong><small>The recorded locator is preserved below. Playback, seeking, and page movement remain analyst-controlled.</small><dl>{citationContext.map(context => <div key={context.kind}><dt>{context.label}</dt><dd><bdi dir={context.kind === 'time' ? 'ltr' : 'auto'}>{context.value}</bdi></dd></div>)}</dl></span></section>}
            {detailState === 'loading' && <div className="analyst-detail-loading" role="status"><i className="fas fa-circle-notch fa-spin" /><span><strong>Opening evidence</strong><small>Loading retained details and published findings…</small></span></div>}
            {detailState === 'error' && <div className="analyst-inline-error" role="alert">{detail?.error}</div>}
            {detailState === 'ready' && (() => {
              const { item, accounting } = sourceDetailPresentation(detail?.item, selected, items)
              const status = sourceProductState(item)
              const maturity = sourceMaturityPresentation(item)
              const capability = capabilityForSource(item, portal.dataState.capabilities)
              const actionableCapability = ['queryable', 'semantic_only', 'limited'].includes(capability?.availability)
              const problems = [...(detail?.warnings || item.warnings || []), ...(detail?.errors || item.errors || []).map(error => error.message || String(error))]
              const artifacts = Array.isArray(detail?.derived_artifacts) ? detail.derived_artifacts : []
              const mediaKind = sourceMediaKind(item)
              const mediaSource = ['image', 'audio', 'video'].includes(mediaKind)
              const operations = status.id === 'ready' ? [...(capability?.deterministic_operations || []), ...(capability?.semantic_operations || [])] : []
              const suggestions = !mediaSource && status.id === 'ready' && actionableCapability ? (capability.suggested_queries || []).slice(0, 4) : []
              const originalName = String(item.original_filename || item.source_file || 'Not reported').split(/[\\/]/).pop()
              const disclosure = detailDisclosureModel({ maturity, accounting, operations, problems })
              const typePresentation = evidenceTypePresentation(item)
              const findingPreview = evidenceFindingPreview(item, status, capability, artifacts)
              const reported = value => value === null || value === undefined ? 'Not reported' : Number(value).toLocaleString()
              return <>
                <section id="source-overview" className="analyst-evidence-overview">
                  <div className="analyst-evidence-overview__status"><span className="analyst-simple-status" data-tone={status.tone}>{status.label}</span><p>{status.description}</p></div>
                  <div className="analyst-evidence-overview__lead"><span>What is available</span><strong>{findingPreview}</strong></div>
                  <dl className="analyst-evidence-overview__facts">
                    <div><dt>Evidence type</dt><dd>{typePresentation.label}</dd></div>
                    <div><dt>Added</dt><dd>{item.created_at ? new Date(item.created_at).toLocaleDateString(undefined, { dateStyle: 'medium' }) : 'Not reported'}</dd></div>
                    <div><dt>Size</dt><dd>{formatBytes(item.size_bytes)}</dd></div>
                  </dl>
                </section>
                <SourceMediaReview item={item} artifacts={artifacts} jobs={detail?.ingest_jobs || []} caseId={portal.activeWorkspace.caseId} casePath={portal.path('/analyst')} candidates={items} sourceTime={params.get('source_time')} />
                <SourceDocumentReview item={item} artifacts={artifacts} caseId={portal.activeWorkspace.caseId} />
                {!mediaSource && <DerivedIntelligence artifacts={artifacts} />}
                {suggestions.length > 0 && <section className="analyst-source-actions"><h3>Productive next actions</h3>{suggestions.map(suggestion => {
                  const prompt = `Use ${originalName} as the focal source where the governed workspace evidence permits: ${suggestion}`
                  return <Link key={suggestion} to={portal.path(`/analyst?evidence=${encodeURIComponent(item.evidence_id)}&prompt=${encodeURIComponent(prompt)}`)}><i className="fas fa-sparkles" /> {suggestion}</Link>
                })}</section>}
                {status.id === 'failed' && <section className="analyst-recovery-plan"><h3>Recovery review</h3><p>The original source remains preserved. Failed processing is not silently retried or overwritten.</p>{!reprocessPlan && <p role="status">Reviewing the governed recovery plan…</p>}{reprocessPlan?.error && <p>Recovery planning is unavailable: {reprocessPlan.error}</p>}{reprocessPlan?.eligibility && <><p>{reprocessPlan.eligibility.reason}</p><strong>{reprocessPlan.approval?.required ? 'Operator approval is required before a new immutable processing generation can be requested.' : 'No execution is available from this screen.'}</strong></>}</section>}
                <details className="analyst-evidence-disclosure">
                  <summary><span><strong>Details</strong><small>Processing, quality, fields and available analysis</small></span><i className="fas fa-chevron-down" aria-hidden="true" /></summary>
                  <div className="analyst-evidence-disclosure__content">
                    <section className="analyst-detail-section" aria-labelledby="source-processing-title">
                      <h3 id="source-processing-title">Processing summary</h3>
                      <dl className="analyst-maturity-summary">
                        <div><dt>Original filename</dt><dd>{originalName}</dd></div>
                        <div><dt>Input rows</dt><dd>{reported(accounting.inputRows)}</dd></div>
                        <div><dt>Accepted</dt><dd>{reported(accounting.acceptedRows)}</dd></div>
                        <div><dt>Duplicates</dt><dd>{reported(accounting.duplicateRows)}</dd></div>
                        <div><dt>Rejected</dt><dd>{reported(accounting.rejectedRows)}</dd></div>
                      </dl>
                    </section>
                    {disclosure.fields && <section id="source-fields" className="analyst-detail-section" aria-labelledby="source-mapping-title">
                      <h3 id="source-mapping-title">Field mapping</h3>
                      <p>Source names and governed meanings. Record values stay out of this overview.</p>
                      <div className="analyst-field-mapping" role="table" aria-label="Source field mapping">
                        <div role="row"><strong role="columnheader">Source field</strong><strong role="columnheader">Mapped as</strong><strong role="columnheader">Type</strong><strong role="columnheader">Status</strong><strong role="columnheader">Pattern / example</strong></div>
                        {maturity.fields.map(field => <div role="row" key={field.source}><span role="cell">{field.source}</span><span role="cell">{field.canonical}</span><span role="cell">{field.type.replaceAll('_', ' ')}</span><span role="cell" data-state={field.state}>{field.status}</span><span role="cell">{field.pattern}</span></div>)}
                      </div>
                    </section>}
                    {disclosure.additionalFields && <section className="analyst-detail-section"><h3>Additional source fields</h3><p>Preserved without inventing canonical meaning.</p><ul>{[...(maturity.mapping.recognized_extension_fields || []), ...(maturity.mapping.unmapped_fields || [])].map(field => <li key={field}>{field}</li>)}</ul></section>}
                    {disclosure.quality && <section id="source-quality" className="analyst-detail-section" aria-labelledby="source-quality-title">
                      <h3 id="source-quality-title">Quality</h3>
                      <dl className="analyst-maturity-summary">
                        <div><dt>State</dt><dd>{maturity.quality.state ? String(maturity.quality.state).replaceAll('_', ' ') : status.label}</dd></div>
                        <div><dt>Accepted</dt><dd>{reported(maturity.quality.counts?.accepted_records ?? accounting.acceptedRows)}</dd></div>
                        <div><dt>Rejected</dt><dd>{reported(maturity.quality.counts?.rejected_records ?? accounting.rejectedRows)}</dd></div>
                        <div><dt>Duplicates</dt><dd>{reported(maturity.quality.counts?.duplicate_records ?? accounting.duplicateRows)}</dd></div>
                        <div><dt>Unmapped</dt><dd>{reported(maturity.quality.counts?.unmapped_fields)}</dd></div>
                      </dl>
                    </section>}
                    {disclosure.capabilities && <section id="source-capabilities" className="analyst-detail-section"><h3>Available analysis</h3>{maturity.time?.contract_version && <p>Time interpretation: {maturity.time.source_timezone || 'unknown timezone'} · {maturity.time.date_order || 'unresolved date order'}</p>}{maturity.capabilities.length > 0 && <ul className="analyst-capability-readiness">{maturity.capabilities.map(entry => <li key={entry.id}><span>{entry.label}</span><strong data-status={entry.status}>{entry.status}</strong><small>{entry.reason}</small></li>)}</ul>}{operations.length > 0 && <ul className="analyst-operation-list">{operations.map(operation => <li key={operation}>{operation}</li>)}</ul>}</section>}
                    {disclosure.notes && <section className="analyst-source-notes"><h3>{status.id === 'ready' ? 'Processing notes' : 'What needs attention'}</h3>{problems.map((message, index) => <p key={index}>{message}</p>)}</section>}
                  </div>
                </details>
                <details id="source-technical" className="analyst-technical-details"><summary><span><strong>Technical details</strong><small>Identifiers, integrity and processing route</small></span><i className="fas fa-chevron-down" aria-hidden="true" /></summary><dl><div><dt>Evidence ID</dt><dd><bdi dir="ltr"><code>{item.evidence_id}</code></bdi></dd></div><div><dt>SHA-256</dt><dd><bdi dir="ltr"><code>{item.sha256 || 'Not reported'}</code></bdi></dd></div><div><dt>Processing route</dt><dd><code>{item.processing_route || 'Not reported'}</code></dd></div></dl></details>
              </>
            })()}
          </aside>
        </div>
      )}
    </div>
  )
}
