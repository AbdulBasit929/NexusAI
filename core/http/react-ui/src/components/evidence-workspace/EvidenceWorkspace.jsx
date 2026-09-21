import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { agentCollectionsApi, recordsApi } from '../../utils/api'
import { resolveEvidenceModality } from '../../utils/evidenceModality'
import './EvidenceWorkspace.css'

function numberValue(value) {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function formatBytes(value) {
  const bytes = numberValue(value)
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const unit = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / (1024 ** unit)).toLocaleString(undefined, { maximumFractionDigits: unit ? 1 : 0 })} ${units[unit]}`
}

function formatDate(value) {
  if (!value) return 'Not reported'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? String(value) : date.toLocaleString()
}

function humanize(value, fallback = 'Not reported') {
  if (value == null || value === '') return fallback
  const words = String(value).replaceAll('_', ' ').split(' ')
  const acronyms = new Map([['ycbcr', 'YCbCr'], ['rgb', 'RGB'], ['cmyk', 'CMYK'], ['exif', 'EXIF'], ['ocr', 'OCR'], ['gps', 'GPS']])
  return words.map(word => acronyms.get(word.toLowerCase()) || word.replace(/^\w/, letter => letter.toUpperCase())).join(' ')
}

function uniqueValues(items, key) {
  return Array.from(new Set(items.map(item => item?.[key]).filter(Boolean))).sort()
}

function fileKey(file, index = 0) {
  return `${file?.name || 'file'}-${file?.size || 0}-${file?.lastModified || 0}-${index}`
}

const imageExtensions = new Set(['png', 'jpg', 'jpeg', 'gif', 'bmp', 'tif', 'tiff', 'webp', 'heic', 'heif', 'dng', 'raw', 'svg'])
const maxImageIntakeBytes = 64 * 1024 * 1024

function imageStagingState(file) {
  const extension = String(file?.name || '').split('.').pop().toLowerCase()
  const isImage = String(file?.type || '').toLowerCase().startsWith('image/') || imageExtensions.has(extension)
  if (!isImage) return { isImage: false, blocked: false, message: 'Evidence source' }
  if (numberValue(file?.size) > maxImageIntakeBytes) return { isImage: true, blocked: false, message: 'Large image; the server will apply its current admission policy' }
  if (!['png', 'jpg', 'jpeg', 'gif', 'bmp', 'tif', 'tiff', 'webp'].includes(extension)) return { isImage: true, blocked: false, message: 'Image will be preserved for manual format review' }
  return { isImage: true, blocked: false, message: 'Bounded image-header inspection enabled' }
}

function queueCount(summary, ...keys) {
  return keys.reduce((total, key) => total + numberValue(summary?.[key]), 0)
}

function SummaryCard({ icon, label, value, detail, tone = 'neutral' }) {
  return (
    <article className="evidence-summary-card" data-tone={tone}>
      <span className="evidence-summary-card__icon"><i className={`fas ${icon}`} aria-hidden="true" /></span>
      <div><span>{label}</span><strong>{value}</strong><small>{detail}</small></div>
    </article>
  )
}

function StatusPill({ status }) {
  return <span className="evidence-status" data-status={status || 'unknown'}>{humanize(status, 'Unknown')}</span>
}

function IntakeResultPill({ status }) {
  const icon = status === 'succeeded' ? 'fa-circle-check' : status === 'failed' ? 'fa-triangle-exclamation' : status === 'uploading' ? 'fa-spinner fa-spin' : 'fa-clock'
  return <span className="evidence-intake-result" data-status={status || 'staged'}><i className={`fas ${icon}`} /> {humanize(status, 'Staged')}</span>
}

function QueueSnapshot({ status }) {
  const summary = status?.summary || {}
  const recentJobs = Array.isArray(status?.recent_jobs) ? status.recent_jobs : []
  const activeJobs = recentJobs.filter(job => ['queued', 'running', 'processing', 'registered'].includes(String(job.status || '').toLowerCase()))
  const failedJobs = recentJobs.filter(job => ['failed', 'dead_letter'].includes(String(job.status || '').toLowerCase()))
  const visibleJobs = (activeJobs.length ? activeJobs : failedJobs.length ? failedJobs : recentJobs).slice(0, 4)

  return (
    <section className="evidence-queue" aria-label="Evidence queue observability">
      <div className="evidence-queue__heading">
        <div><span className="evidence-eyebrow">Queue observability</span><h2>Processing and publication state</h2></div>
        <span>{formatDate(status?.generated_at)}</span>
      </div>
      <div className="evidence-queue-grid">
        <div><span>In flight</span><strong>{queueCount(summary, 'evidence_in_flight').toLocaleString()}</strong><small>Queued, registered, or processing</small></div>
        <div><span>Accepted rows</span><strong>{numberValue(summary.accepted_rows).toLocaleString()}</strong><small>{numberValue(summary.duplicate_rows).toLocaleString()} duplicate rows</small></div>
        <div data-tone={numberValue(summary.rejected_rows) ? 'warning' : 'neutral'}><span>Rejected rows</span><strong>{numberValue(summary.rejected_rows).toLocaleString()}</strong><small>Backend-accounted rejects</small></div>
        <div data-tone={numberValue(summary.failed_jobs) ? 'danger' : 'neutral'}><span>Failed jobs</span><strong>{numberValue(summary.failed_jobs).toLocaleString()}</strong><small>{numberValue(summary.completed_jobs_missing_kb_asset).toLocaleString()} missing KB asset links</small></div>
      </div>
      {visibleJobs.length > 0 && (
        <div className="evidence-queue-list" aria-label="Recent queue jobs">
          {visibleJobs.map(job => (
            <div key={job.job_id}>
              <StatusPill status={job.status} />
              <span><strong>{job.source_file || job.job_id}</strong><small>{humanize(job.record_type, 'General')} ; attempt {numberValue(job.attempt_count)} of {numberValue(job.max_attempts) || '-'}</small></span>
              <time>{formatDate(job.completed_at || job.started_at || job.queued_at)}</time>
            </div>
          ))}
        </div>
      )}
    </section>
  )
}

function EmptyInspection() {
  return (
    <div className="evidence-inspection-empty">
      <span><i className="fas fa-fingerprint" aria-hidden="true" /></span>
      <h2>Choose evidence to inspect</h2>
      <p>Open a registered source to review its immutable identity, observed processing state, exact row accounting, linked artifacts, and recorded limitations.</p>
    </div>
  )
}

function ArtifactLedger({ artifacts }) {
  const [selectedArtifactId, setSelectedArtifactId] = useState(artifacts[0]?.artifact_id || '')
  const selected = artifacts.find(artifact => artifact.artifact_id === selectedArtifactId) || artifacts[0]

  useEffect(() => {
    setSelectedArtifactId(artifacts[0]?.artifact_id || '')
  }, [artifacts])

  if (!artifacts.length) return <p className="evidence-muted">No derived artifact or citation locator is recorded for this source.</p>

  return (
    <div className="evidence-artifact-ledger">
      <div className="evidence-artifact-list" aria-label="Derived artifact citations">
        {artifacts.map(artifact => (
          <button
            type="button"
            key={artifact.artifact_id}
            data-selected={selected?.artifact_id === artifact.artifact_id}
            onClick={() => setSelectedArtifactId(artifact.artifact_id)}
            aria-label={`Inspect citation ${humanize(artifact.artifact_type, artifact.artifact_id)}`}
          >
            <span><i className="fas fa-file-shield" /><strong>{humanize(artifact.artifact_type, 'Derived artifact')}</strong></span>
            <StatusPill status={artifact.processing_status} />
            <small>{artifact.media_type || 'Media type not reported'}</small>
          </button>
        ))}
      </div>
      {selected && (
        <article className="evidence-citation" id={`evidence-artifact-${selected.artifact_id}`} aria-label="Selected artifact citation">
          <div className="evidence-citation__header"><div><span className="evidence-eyebrow">Stable citation</span><h4>{humanize(selected.artifact_type, 'Derived artifact')}</h4></div><a href={`#evidence-artifact-${selected.artifact_id}`} className="btn btn-secondary btn-sm"><i className="fas fa-location-crosshairs" /> Open locator</a></div>
          <dl>
            <div><dt>Citation ref</dt><dd><code>{selected.citation_ref || 'Not exposed'}</code></dd></div>
            <div><dt>Artifact ID</dt><dd><code>{selected.artifact_id}</code></dd></div>
            <div><dt>Run ID</dt><dd><code>{selected.run_id || 'Not exposed'}</code></dd></div>
            <div><dt>Content hash</dt><dd><code>{selected.content_sha256 || 'Not exposed'}</code></dd></div>
          </dl>
          {selected?.metadata?.observation && (
            <div className="evidence-locator">
              <span>Derived observation (not source truth)</span>
              <code>{JSON.stringify(selected.metadata.observation)}</code>
            </div>
          )}
          <div className="evidence-locator"><span>Exact locator metadata</span><code>{JSON.stringify(selected.citation_locator || {})}</code></div>
        </article>
      )}
    </div>
  )
}

function CustodyTimeline({ events, integrity }) {
  if (!events.length) {
    return <div className="evidence-custody-empty"><i className="fas fa-shield-halved" /><div><strong>No custody events published for this source</strong><span>NexusAI preserves the boundary and does not infer missing custody history.</span></div></div>
  }
  return (
    <div className="evidence-custody">
      <div className="evidence-custody__assurance" data-valid={integrity?.chain_valid === true}>
        <i className={`fas ${integrity?.chain_valid === true ? 'fa-shield-circle-check' : 'fa-triangle-exclamation'}`} />
        <div><strong>{integrity?.chain_valid === true ? 'Hash chain verified' : 'Custody integrity requires review'}</strong><span>{numberValue(integrity?.event_count).toLocaleString()} append-only event{numberValue(integrity?.event_count) === 1 ? '' : 's'} ; {numberValue(integrity?.broken_links)} broken link{numberValue(integrity?.broken_links) === 1 ? '' : 's'}</span></div>
      </div>
      <ol className="evidence-custody-list">
        {events.map(event => <li key={event.custody_event_id}><span className="evidence-custody-list__marker"><i className="fas fa-fingerprint" /></span><div><div><strong>{humanize(event.event_type, 'Recorded custody event')}</strong><time>{formatDate(event.occurred_at)}</time></div><p>{humanize(event.actor_type, 'Actor')} {event.actor_id ? `; ${event.actor_id}` : ''}{event.custody_location ? `; ${event.custody_location}` : ''}</p>{event.reason && <span>{event.reason}</span>}<code title={event.event_sha256}>Event hash {String(event.event_sha256 || '').slice(0, 18)}...</code></div></li>)}
      </ol>
    </div>
  )
}

function MediaEvidencePreview({ item }) {
  if (!['image', 'audio', 'video'].includes(item.modality)) return null
  const caseId = item.case_id || item.collection_id
  const source = recordsApi.forensicCaseEvidenceContentUrl(caseId, item.evidence_id)
  return (
    <section className="evidence-detail-section evidence-media-preview" aria-labelledby="evidence-media-preview-title">
      <div className="evidence-section-heading"><div><span className="evidence-eyebrow">Authorized source preview</span><h3 id="evidence-media-preview-title">Retained {item.modality}</h3></div><span className="evidence-confidence">Verified source bytes</span></div>
      {item.modality === 'image' && <img src={source} alt={`Retained evidence preview for ${item.source_file || 'image'}`} loading="lazy" />}
      {item.modality === 'audio' && <audio src={source} controls preload="metadata">Audio preview is not supported by this browser.</audio>}
      {item.modality === 'video' && <video src={source} controls preload="metadata">Video preview is not supported by this browser.</video>}
      <p><i className="fas fa-shield-halved" /> Preview is case-authorized and streamed from the immutable retained source. Model observations and annotations do not replace it.</p>
    </section>
  )
}

function PlateRegionReview({ artifacts, mediaMetadata }) {
  const candidates = useMemo(() => artifacts
    .map(artifact => ({
      artifact,
      candidate: artifact?.metadata?.candidate || artifact?.metadata?.plate_region_candidate || (artifact?.artifact_type === 'plate_region_candidate' ? artifact.metadata : null),
      observation: artifact?.metadata?.observation || {},
    }))
    .filter(entry => entry.candidate?.contract_version === 'forensics.plate-region-candidate/v1'), [artifacts])
  const [selectedId, setSelectedId] = useState(candidates[0]?.candidate?.candidate_id || '')

  useEffect(() => {
    setSelectedId(candidates[0]?.candidate?.candidate_id || '')
  }, [candidates])

  const width = numberValue(mediaMetadata.width_pixels)
  const height = numberValue(mediaMetadata.height_pixels)
  const selected = candidates.find(entry => entry.candidate.candidate_id === selectedId) || candidates[0]
  if (!candidates.length) {
    return (
      <div className="evidence-region-empty">
        <i className="fas fa-vector-square" />
        <div><strong>No plate-region candidates recorded</strong><span>The source passed image admission, but no approved local detector output or human-defined region is present. NexusAI does not convert image metadata into a plate claim.</span></div>
      </div>
    )
  }

  return (
    <div className="evidence-region-review">
      <div className="evidence-region-stage" style={{ aspectRatio: width > 0 && height > 0 ? `${width} / ${height}` : '16 / 9' }} aria-label="Original-image coordinate overlay; source pixels are unchanged">
        <div className="evidence-region-stage__notice"><i className="fas fa-image" /><span>Coordinate overlay</span><small>Original pixels remain immutable</small></div>
        {candidates.map(({ candidate }) => {
          const bounds = candidate.bounds || {}
          const invalid = width <= 0 || height <= 0 || numberValue(bounds.width) <= 0 || numberValue(bounds.height) <= 0
          if (invalid) return null
          return (
            <button
              type="button"
              key={candidate.candidate_id}
              className="evidence-region-box"
              data-selected={selected?.candidate?.candidate_id === candidate.candidate_id}
              style={{ left: `${numberValue(bounds.x) / width * 100}%`, top: `${numberValue(bounds.y) / height * 100}%`, width: `${numberValue(bounds.width) / width * 100}%`, height: `${numberValue(bounds.height) / height * 100}%` }}
              onClick={() => setSelectedId(candidate.candidate_id)}
              aria-label={`Inspect plate-region candidate ${candidate.candidate_id}`}
            ><span>{Math.round(numberValue(candidate.confidence) * 100)}%</span></button>
          )
        })}
      </div>
      <div className="evidence-region-list" aria-label="Plate-region candidates">
        {candidates.map(({ artifact, candidate, observation }) => (
          <button type="button" key={candidate.candidate_id} data-selected={selected?.candidate?.candidate_id === candidate.candidate_id} onClick={() => setSelectedId(candidate.candidate_id)}>
            <span><strong>{observation.normalized_plate_text || 'Unread plate candidate'}</strong><small>{candidate.candidate_id} · {observation.raw_plate_text ? `Raw OCR: ${observation.raw_plate_text}` : humanize(candidate.review_state, 'Model candidate')}</small></span>
            <span><b>{Math.round(numberValue(candidate.confidence) * 100)}%</b><small>{candidate.detector_id} {candidate.detector_version}</small></span>
            <code>{candidate.bounds ? `${candidate.bounds.x},${candidate.bounds.y} · ${candidate.bounds.width}×${candidate.bounds.height}` : 'Bounds unavailable'}</code>
            <code>{artifact.citation_ref || 'Citation pending'}</code>
          </button>
        ))}
      </div>
      {selected && <p className="evidence-region-boundary"><i className="fas fa-shield-halved" /> Candidate geometry is displayed in original-image pixels. Selection here changes only the inspection focus; accept, reject, correction, and manual bounding require an append-only authorized review event.</p>}
    </div>
  )
}

function ReprocessGovernance({ plan, loading, error }) {
  if (loading) return <div className="evidence-reprocess-state" role="status"><i className="fas fa-spinner fa-spin" /> Loading governed reprocess review...</div>
  if (error) return <div className="evidence-reprocess-state" data-tone="warning"><i className="fas fa-triangle-exclamation" /><span>Reprocess review unavailable: {error}</span></div>
  if (!plan) return <div className="evidence-reprocess-state"><i className="fas fa-lock" /><span>Reprocess review has not been published for this source.</span></div>

  const eligible = plan?.eligibility?.eligible === true
  const approval = plan?.approval || {}
  const current = plan?.current_state || {}
  return (
    <section className="evidence-reprocess" aria-label="Governed reprocess review">
      <div className="evidence-reprocess__assurance" data-eligible={eligible}>
        <i className={`fas ${eligible ? 'fa-circle-check' : 'fa-clock'}`} />
        <div><strong>{eligible ? 'Eligible for approval review' : 'Not eligible for reprocess'}</strong><span>{plan?.eligibility?.reason || 'Eligibility was not reported.'}</span></div>
        <StatusPill status={current.latest_job_status || current.evidence_status} />
      </div>
      <div className="evidence-reprocess__grid">
        <div><span>Approval</span><strong>{humanize(approval.state, 'Not granted')}</strong><small>{approval.notice || 'No execution authority is published.'}</small></div>
        <div><span>Latest generation</span><strong>{numberValue(current.reprocess_generation).toLocaleString()}</strong><small>Prior processing remains immutable</small></div>
        <div><span>Execution</span><strong>Disabled here</strong><small>Evidence Operations never submits a reprocess request.</small></div>
      </div>
      <p className="evidence-reprocess__boundary"><i className="fas fa-shield-halved" /> This is a read-only governance review. A separately authorized operator workflow, reason, and idempotency key are required before any queue action.</p>
    </section>
  )
}

function EvidenceInspection({ detail, loading, error, reprocessPlan, reprocessLoading, reprocessError, onClose }) {
  if (loading) return <div className="evidence-inspection-state" role="status"><i className="fas fa-spinner fa-spin" /> Loading evidence truth...</div>
  if (error) return <div className="alert alert-error evidence-inspection-state" role="alert"><strong>Evidence detail unavailable.</strong><span>{error}</span></div>
  if (!detail?.item) return <EmptyInspection />

  const item = detail.item
  const jobs = Array.isArray(detail.ingest_jobs) ? detail.ingest_jobs : []
  const assets = Array.isArray(detail.kb_assets) ? detail.kb_assets : []
  const preview = Array.isArray(detail.records_preview) ? detail.records_preview : []
  const entities = Array.isArray(detail.entity_rollup) ? detail.entity_rollup : []
  const processingRuns = Array.isArray(detail.processing_runs) ? detail.processing_runs : []
  const processingEvents = Array.isArray(detail.processing_events) ? detail.processing_events : []
  const derivedArtifacts = Array.isArray(detail.derived_artifacts) ? detail.derived_artifacts : []
  const custodyEvents = Array.isArray(detail.custody_events) ? detail.custody_events : []
  const custodyIntegrity = detail.custody_integrity || {}
  const mediaMetadata = item.media_metadata && typeof item.media_metadata === 'object' ? item.media_metadata : {}
  const warnings = Array.isArray(item.warnings) ? item.warnings : []
  const errors = Array.isArray(item.errors) ? item.errors : []
  const latestJob = jobs[0]
  const lineage = [
    ['Evidence ID', item.evidence_id],
    ['Collection', item.collection_id],
    ['Case', item.case_id || item.collection_id],
    ['Raw storage', item.raw_storage_ref],
    ['Records batch', item.records_batch_id],
    ['Knowledge asset', item.kb_entry_ref],
    ['Extracted text', item.extracted_text_ref],
  ]

  return (
    <article className="evidence-inspection" aria-labelledby="evidence-inspection-title">
      <header className="evidence-inspection__header">
        <div>
          <span className="evidence-eyebrow">Evidence inspection</span>
          <h2 id="evidence-inspection-title">{item.source_file || item.original_filename || 'Registered source'}</h2>
          <p>{humanize(item.modality, 'Unknown modality')} ; {humanize(item.detected_type, 'Unclassified')}</p>
        </div>
        <button type="button" className="btn btn-secondary btn-sm" onClick={onClose} aria-label="Close evidence inspection"><i className="fas fa-xmark" /></button>
      </header>

      <div className="evidence-inspection__assurance">
        <StatusPill status={item.processing_status} />
        <span><i className="fas fa-lock" /> Read-only source truth</span>
        <span>{formatBytes(item.size_bytes)}</span>
      </div>

      <MediaEvidencePreview item={item} />

      {(warnings.length > 0 || errors.length > 0) && (
        <section className="evidence-findings" data-tone={errors.length ? 'danger' : 'warning'} aria-labelledby="evidence-findings-title">
          <h3 id="evidence-findings-title"><i className="fas fa-triangle-exclamation" /> Actionable processing notes</h3>
          <ul>{[...errors, ...warnings].map((message, index) => <li key={`${index}-${message}`}>{typeof message === 'string' ? message : JSON.stringify(message)}</li>)}</ul>
        </section>
      )}

      <section className="evidence-detail-section" aria-labelledby="evidence-integrity-title">
        <div className="evidence-section-heading"><div><span className="evidence-eyebrow">Identity and integrity</span><h3 id="evidence-integrity-title">Registered source</h3></div><span className="evidence-confidence">{item.classifier_confidence == null ? 'Confidence not reported' : `${(numberValue(item.classifier_confidence) * 100).toFixed(1)}% classifier confidence`}</span></div>
        <dl className="evidence-key-grid">
          <div><dt>SHA-256</dt><dd><code>{item.sha256 || 'Hash unavailable'}</code></dd></div>
          <div><dt>Content type</dt><dd>{item.content_type || 'Not reported'}</dd></div>
          <div><dt>Processing route</dt><dd>{humanize(item.processing_route)}</dd></div>
          <div><dt>Registered</dt><dd>{formatDate(item.created_at)}</dd></div>
        </dl>
      </section>

      {item.modality === 'image' && (
        <>
        <section className="evidence-detail-section" role="region" aria-labelledby="evidence-image-intake-title">
          <div className="evidence-section-heading"><div><span className="evidence-eyebrow">Visual evidence admission</span><h3 id="evidence-image-intake-title">Immutable image metadata</h3></div><span className="evidence-confidence">No OCR or detection claimed</span></div>
          <div className="evidence-image-admission" data-state={mediaMetadata.validation_state || 'not_reported'}>
            <i className={`fas ${mediaMetadata.downstream_decode_allowed === true ? 'fa-shield-circle-check' : 'fa-triangle-exclamation'}`} />
            <div><strong>{humanize(mediaMetadata.validation_state, 'Admission state not reported')}</strong><span>{mediaMetadata.downstream_decode_allowed === true ? 'Header checks passed for a separately approved downstream decoder.' : 'Automatic pixel decoding remains blocked pending review.'}</span></div>
          </div>
          <dl className="evidence-key-grid">
            <div><dt>Detected format</dt><dd>{humanize(mediaMetadata.format)}</dd></div>
            <div><dt>Dimensions</dt><dd>{mediaMetadata.width_pixels && mediaMetadata.height_pixels ? `${numberValue(mediaMetadata.width_pixels).toLocaleString()} × ${numberValue(mediaMetadata.height_pixels).toLocaleString()} px` : 'Not safely decoded'}</dd></div>
            <div><dt>Display orientation</dt><dd>{humanize(mediaMetadata.exif_orientation_label, 'Not supplied')}</dd></div>
            <div><dt>Color model</dt><dd>{humanize(mediaMetadata.color_model)}</dd></div>
            <div><dt>Pixel count</dt><dd>{mediaMetadata.pixel_count == null ? 'Not reported' : numberValue(mediaMetadata.pixel_count).toLocaleString()}</dd></div>
            <div><dt>Inspection</dt><dd>{humanize(mediaMetadata.inspection_mode, 'Bounded header only')}</dd></div>
          </dl>
          <p className="evidence-image-privacy"><i className="fas fa-user-shield" /> EXIF free text and GPS are not surfaced by this admission contract; only bounded technical fields and orientation are retained here.</p>
        </section>
        <section className="evidence-detail-section" role="region" aria-labelledby="evidence-plate-regions-title">
          <div className="evidence-section-heading"><div><span className="evidence-eyebrow">Visual provenance</span><h3 id="evidence-plate-regions-title">Plate-region candidates</h3></div><span className="evidence-confidence">Candidates are not proof</span></div>
          <PlateRegionReview artifacts={derivedArtifacts} mediaMetadata={mediaMetadata} />
        </section>
        </>
      )}

      <section className="evidence-detail-section" aria-labelledby="evidence-accounting-title">
        <div className="evidence-section-heading"><div><span className="evidence-eyebrow">Processing pipeline</span><h3 id="evidence-accounting-title">Exact row accounting</h3></div><span className="evidence-confidence">{jobs.length} immutable run{jobs.length === 1 ? '' : 's'}</span></div>
        {latestJob ? (
          <>
            <div className="evidence-accounting-grid">
              <div><span>Total</span><strong>{numberValue(latestJob.total_rows).toLocaleString()}</strong></div>
              <div data-tone="good"><span>Accepted</span><strong>{numberValue(latestJob.accepted_rows).toLocaleString()}</strong></div>
              <div data-tone="warning"><span>Duplicate</span><strong>{numberValue(latestJob.duplicate_rows).toLocaleString()}</strong></div>
              <div data-tone="danger"><span>Rejected</span><strong>{numberValue(latestJob.rejected_rows).toLocaleString()}</strong></div>
            </div>
            <div className="evidence-run-list">
              {jobs.map(job => (
                <div className="evidence-run" key={job.job_id || `${job.created_at}-${job.reprocess_generation}`}>
                  <StatusPill status={job.status} />
                  <div><strong>{humanize(job.record_type, 'General evidence processing')}</strong><small>Attempt {numberValue(job.attempt_count)} of {numberValue(job.max_attempts) || '-'} ; generation {numberValue(job.reprocess_generation)}</small></div>
                  <time>{formatDate(job.completed_at || job.updated_at || job.created_at)}</time>
                </div>
              ))}
            </div>
          </>
        ) : <p className="evidence-muted">No ingest or reprocessing job is linked to this evidence item.</p>}
      </section>

      <section className="evidence-detail-section" aria-labelledby="evidence-lineage-title">
        <div className="evidence-section-heading"><div><span className="evidence-eyebrow">Provenance</span><h3 id="evidence-lineage-title">Lineage and linked outputs</h3></div><span className="evidence-confidence">{assets.length} linked asset{assets.length === 1 ? '' : 's'}</span></div>
        <dl className="evidence-lineage-list">{lineage.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value ? <code>{value}</code> : <span>Not exposed</span>}</dd></div>)}</dl>
        <div className="evidence-artifact-grid">
          <div><span><i className="fas fa-book" /> Knowledge assets</span><strong>{assets.length}</strong><small>{assets.length ? assets.map(asset => humanize(asset.rag_status || asset.structured_status)).join('; ') : 'No linked KB artifact'}</small></div>
          <div><span><i className="fas fa-table-list" /> Row preview</span><strong>{preview.length}</strong><small>{detail.records_preview_included === false ? 'Preview intentionally excluded' : 'Bounded, privacy-projected rows'}</small></div>
          <div><span><i className="fas fa-diagram-project" /> Entity groups</span><strong>{entities.length}</strong><small>{entities.length ? `${entities.reduce((sum, entity) => sum + numberValue(entity.observation_count), 0).toLocaleString()} observations` : 'No entity rollup'}</small></div>
        </div>
      </section>

      <section className="evidence-detail-section" aria-labelledby="evidence-artifacts-title">
        <div className="evidence-section-heading"><div><span className="evidence-eyebrow">Derived outputs</span><h3 id="evidence-artifacts-title">Artifacts and citations</h3></div><span className="evidence-confidence">{derivedArtifacts.length} cited artifact{derivedArtifacts.length === 1 ? '' : 's'}</span></div>
        <ArtifactLedger artifacts={derivedArtifacts} />
      </section>

      <section className="evidence-detail-section" aria-labelledby="evidence-processing-events-title">
        <div className="evidence-section-heading"><div><span className="evidence-eyebrow">Pipeline audit</span><h3 id="evidence-processing-events-title">Canonical runs and events</h3></div><span className="evidence-confidence">{processingRuns.length} run{processingRuns.length === 1 ? '' : 's'} ; {processingEvents.length} event{processingEvents.length === 1 ? '' : 's'}</span></div>
        {processingRuns.length || processingEvents.length ? <div className="evidence-processing-ledger">{processingRuns.map(run => <div key={run.run_id}><StatusPill status={run.status} /><span><strong>{humanize(run.run_kind, 'Processing run')}</strong><small>{run.pipeline_id || 'Pipeline not reported'} ; attempt {numberValue(run.attempt_count)}</small></span><time>{formatDate(run.completed_at || run.started_at || run.requested_at)}</time></div>)}{processingEvents.slice(0, 6).map(event => <div key={event.event_id} data-event><i className="fas fa-wave-square" /><span><strong>{humanize(event.event_type, 'Processing event')}</strong><small>{event.status_from || 'none'} to {event.status_to || 'none'} ; sequence {event.event_sequence}</small></span><time>{formatDate(event.occurred_at)}</time></div>)}</div> : <p className="evidence-muted">No canonical processing run or event is recorded for this source.</p>}
      </section>

      <section className="evidence-detail-section" role="region" aria-labelledby="evidence-custody-title">
        <div className="evidence-section-heading"><div><span className="evidence-eyebrow">Chain of custody</span><h3 id="evidence-custody-title">Append-only custody record</h3></div><span className="evidence-confidence">{custodyEvents.length ? 'Backend-recorded history' : 'Truthful unavailable state'}</span></div>
        <CustodyTimeline events={custodyEvents} integrity={custodyIntegrity} />
      </section>

      <section className="evidence-detail-section" role="region" aria-labelledby="evidence-reprocess-title">
        <div className="evidence-section-heading"><div><span className="evidence-eyebrow">Governed recovery</span><h3 id="evidence-reprocess-title">Reprocess approval review</h3></div><span className="evidence-confidence">Read-only control</span></div>
        <ReprocessGovernance plan={reprocessPlan} loading={reprocessLoading} error={reprocessError} />
      </section>
    </article>
  )
}

export default function EvidenceWorkspace({ caseId, evidence, status, loading, onRefresh, onRefreshStatus, addToast, initialEvidenceId = '' }) {
  const pageSize = 25
  const [files, setFiles] = useState([])
  const [uploading, setUploading] = useState(false)
  const [intakeResults, setIntakeResults] = useState([])
  const [query, setQuery] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [modalityFilter, setModalityFilter] = useState('')
  const [page, setPage] = useState(0)
  const [catalog, setCatalog] = useState(evidence || null)
  const [catalogLoading, setCatalogLoading] = useState(false)
  const [catalogError, setCatalogError] = useState('')
  const [selectedId, setSelectedId] = useState('')
  const [detail, setDetail] = useState(null)
  const [detailError, setDetailError] = useState('')
  const [detailLoading, setDetailLoading] = useState(false)
  const [reprocessPlan, setReprocessPlan] = useState(null)
  const [reprocessPlanError, setReprocessPlanError] = useState('')
  const [reprocessPlanLoading, setReprocessPlanLoading] = useState(false)
  const requestSequence = useRef(0)
  const catalogSequence = useRef(0)
  const catalogFiltersHydrated = useRef(false)
  const items = useMemo(() => Array.isArray(catalog?.items) ? catalog.items : [], [catalog])
  const summary = catalog?.summary || {}
  const pagination = catalog?.pagination || {}
  const total = numberValue(pagination.evidence_total ?? summary.evidence_total ?? items.length)
  const offset = numberValue(pagination.offset ?? page * pageSize)
  const returned = numberValue(pagination.returned ?? items.length)
  const hasNext = pagination.has_next ?? (offset + returned < total)
  const hasPrevious = pagination.has_previous ?? offset > 0
  const statuses = useMemo(() => uniqueValues(items, 'processing_status'), [items])
  const modalities = useMemo(() => uniqueValues(items, 'modality'), [items])
  const currentLoading = loading || catalogLoading
  const blockedStagedFiles = useMemo(() => files.filter(file => imageStagingState(file).blocked), [files])

  const loadCatalog = useCallback(async (next = {}) => {
    const nextPage = Math.max(0, Number.isFinite(next.page) ? next.page : 0)
    const sequence = ++catalogSequence.current
    setCatalogLoading(true)
    setCatalogError('')
    try {
      const data = await onRefresh({
        limit: pageSize,
        offset: nextPage * pageSize,
        q: query.trim(),
        processing_status: statusFilter,
        modality: modalityFilter,
      })
      if (catalogSequence.current === sequence && data) {
        setCatalog(data)
        setPage(nextPage)
      }
    } catch (error) {
      if (catalogSequence.current === sequence) setCatalogError(error.message)
    } finally {
      if (catalogSequence.current === sequence) setCatalogLoading(false)
    }
  }, [modalityFilter, onRefresh, pageSize, query, statusFilter])

  useEffect(() => {
    requestSequence.current += 1
    catalogSequence.current += 1
    setSelectedId('')
    setDetail(null)
    setDetailError('')
    setDetailLoading(false)
    setReprocessPlan(null)
    setReprocessPlanError('')
    setReprocessPlanLoading(false)
    setCatalog(null)
    setCatalogError('')
    setFiles([])
    setIntakeResults([])
    setPage(0)
    catalogFiltersHydrated.current = false
  }, [caseId])

  useEffect(() => {
    if (evidence) setCatalog(evidence)
  }, [evidence])

  useEffect(() => {
    if (!caseId || !onRefresh) return
    if (!catalogFiltersHydrated.current) {
      catalogFiltersHydrated.current = true
      return
    }
    const timeout = setTimeout(() => { loadCatalog({ page: 0 }) }, 250)
    return () => clearTimeout(timeout)
  }, [caseId, loadCatalog, onRefresh])

  const inspect = async item => {
    const evidenceId = item?.evidence_id
    if (!evidenceId) return
    const sequence = ++requestSequence.current
    setSelectedId(evidenceId)
    setDetail(null)
    setDetailError('')
    setDetailLoading(true)
    setReprocessPlan(null)
    setReprocessPlanError('')
    setReprocessPlanLoading(true)
    try {
      const result = await recordsApi.forensicCaseEvidenceDetail(caseId, evidenceId, { limit: 12, include_records_preview: true })
      if (requestSequence.current === sequence) setDetail(result)
    } catch (error) {
      if (requestSequence.current === sequence) setDetailError(error.message)
    } finally {
      if (requestSequence.current === sequence) setDetailLoading(false)
    }
    try {
      const plan = await recordsApi.forensicCaseEvidenceReprocessPlan(caseId, evidenceId)
      if (requestSequence.current === sequence) setReprocessPlan(plan)
    } catch (planError) {
      if (requestSequence.current === sequence) setReprocessPlanError(planError.message)
    } finally {
      if (requestSequence.current === sequence) setReprocessPlanLoading(false)
    }
  }

  useEffect(() => {
    if (!caseId || !initialEvidenceId || selectedId === initialEvidenceId) return
    inspect({ evidence_id: initialEvidenceId })
  // inspect intentionally uses current case scope and request sequencing.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [caseId, initialEvidenceId])

  const upload = async event => {
    event.preventDefault()
    if (!files.length || blockedStagedFiles.length) return
    setUploading(true)
    const initialResults = files.map((file, index) => ({ key: fileKey(file, index), name: file.name, size: file.size, status: 'queued', message: 'Waiting for an intake slot' }))
    setIntakeResults(initialResults)
    let cursor = 0
    let succeeded = 0
    let failed = 0
    const updateResult = (key, patch) => setIntakeResults(current => current.map(item => item.key === key ? { ...item, ...patch } : item))
    try {
      const workers = Array.from({ length: Math.min(2, files.length) }, async () => {
        while (cursor < files.length) {
          const index = cursor
          cursor += 1
          const file = files[index]
          const key = fileKey(file, index)
          updateResult(key, { status: 'uploading', message: 'Registering with the active case' })
          try {
            const form = new FormData()
            form.append('file', file)
            form.append('case_id', caseId)
            form.append('evidence_role', 'source')
            form.append('jurisdiction', 'PK')
            form.append('source_timezone_state', 'unknown')
            form.append('source_date_order_state', 'unresolved')
            const result = await agentCollectionsApi.upload(caseId, form)
            const evidenceStatus = String(result?.evidence_status || result?.records_status || '').toLowerCase()
            if (['failed', 'skipped'].includes(evidenceStatus)) {
              throw new Error(result?.records_warning || `Evidence registration ${evidenceStatus}`)
            }
            succeeded += 1
            updateResult(key, { status: 'succeeded', message: result?.status || 'Registered', job_id: result?.job_id || result?.evidence_id || '' })
          } catch (error) {
            failed += 1
            updateResult(key, { status: 'failed', message: error.message })
          }
        }
      })
      await Promise.all(workers)
      addToast(`Evidence intake finished: ${succeeded} registered, ${failed} failed`, failed ? 'warning' : 'success')
      if (succeeded > 0) setFiles([])
      await loadCatalog({ page: 0 })
      await onRefreshStatus?.()
    } finally {
      setUploading(false)
    }
  }

  const closeInspection = () => {
    requestSequence.current += 1
    setSelectedId('')
    setDetail(null)
    setDetailError('')
    setDetailLoading(false)
    setReprocessPlan(null)
    setReprocessPlanError('')
    setReprocessPlanLoading(false)
  }

  return (
    <div className="evidence-workspace">
      <section className="evidence-command evidence-command--sticky" aria-labelledby="evidence-catalog-title">
        <div className="evidence-command__heading"><div><span className="evidence-eyebrow">Evidence operations</span><h2 id="evidence-catalog-title">Source registry and processing truth</h2><p>Search the authorized case, inspect one immutable source, and reconcile exact backend state without leaving the investigation desk.</p></div><button className="btn btn-secondary btn-sm" onClick={() => loadCatalog({ page })} disabled={currentLoading}><i className={`fas ${currentLoading ? 'fa-spinner fa-spin' : 'fa-rotate'}`} /> Refresh truth</button></div>
        <div className="evidence-filters">
          <label className="evidence-search"><span>Search registered evidence</span><div><i className="fas fa-magnifying-glass" /><input value={query} onChange={event => { setQuery(event.target.value); setPage(0) }} placeholder="Filename, evidence ID, hash, or type" /></div></label>
          <label><span>Processing state</span><select value={statusFilter} onChange={event => { setStatusFilter(event.target.value); setPage(0) }}><option value="">All states</option>{statuses.map(status => <option key={status} value={status}>{humanize(status)}</option>)}</select></label>
          <label><span>Modality</span><select value={modalityFilter} onChange={event => { setModalityFilter(event.target.value); setPage(0) }}><option value="">All modalities</option>{modalities.map(modality => <option key={modality} value={modality}>{humanize(modality)}</option>)}</select></label>
        </div>
        {(query || statusFilter || modalityFilter) && <div className="evidence-active-filters" aria-label="Active evidence filters">
          {query && <button type="button" onClick={() => { setQuery(''); setPage(0) }}>Search: <bdi dir="auto">{query}</bdi> <i className="fas fa-xmark" aria-hidden="true" /></button>}
          {statusFilter && <button type="button" onClick={() => { setStatusFilter(''); setPage(0) }}>State: {humanize(statusFilter)} <i className="fas fa-xmark" aria-hidden="true" /></button>}
          {modalityFilter && <button type="button" onClick={() => { setModalityFilter(''); setPage(0) }}>Modality: {humanize(modalityFilter)} <i className="fas fa-xmark" aria-hidden="true" /></button>}
          <button type="button" data-clear onClick={() => { setQuery(''); setStatusFilter(''); setModalityFilter(''); setPage(0) }}>Clear all</button>
        </div>}
        {catalogError && <div className="alert alert-error evidence-catalog-alert" role="alert">{catalogError}</div>}
        <div className="evidence-result-line" role="status" aria-live="polite"><span>{currentLoading ? 'Refreshing the registered source catalog…' : items.length ? <><strong>{items.length}</strong> returned; {offset + 1}-{Math.min(offset + returned, total)} of {total.toLocaleString()}</> : query || statusFilter || modalityFilter ? 'No registered sources match the active filters.' : 'No registered sources.'}</span></div>
        <div className="evidence-pagination" aria-label="Evidence catalog pagination">
          <button className="btn btn-secondary btn-sm" type="button" onClick={() => loadCatalog({ page: Math.max(0, page - 1) })} disabled={currentLoading || !hasPrevious}><i className="fas fa-chevron-left" /> Previous</button>
          <span>Page {page + 1}; {pageSize} per page</span>
          <button className="btn btn-secondary btn-sm" type="button" onClick={() => loadCatalog({ page: page + 1 })} disabled={currentLoading || !hasNext}>Next <i className="fas fa-chevron-right" /></button>
        </div>
      </section>

      <section className="evidence-summary" aria-label="Evidence processing summary" aria-busy={currentLoading}>
        <SummaryCard icon="fa-box-archive" label="Registered sources" value={currentLoading ? '-' : total.toLocaleString()} detail={formatBytes(summary.total_size_bytes)} />
        <SummaryCard icon="fa-circle-check" label="Ready" value={currentLoading ? '-' : numberValue(summary.completed).toLocaleString()} detail="Backend-completed evidence" tone="good" />
        <SummaryCard icon="fa-arrows-rotate" label="In pipeline" value={currentLoading ? '-' : (numberValue(summary.processing) + numberValue(summary.queued)).toLocaleString()} detail="Queued or processing" tone="active" />
        <SummaryCard icon="fa-triangle-exclamation" label="Requires review" value={currentLoading ? '-' : numberValue(summary.failed).toLocaleString()} detail="Backend-reported failures" tone={numberValue(summary.failed) ? 'danger' : 'neutral'} />
      </section>

      <QueueSnapshot status={status} />

      <details className="evidence-intake">
        <summary><span><i className="fas fa-cloud-arrow-up" /><strong>Evidence intake</strong><small>Stage and register source files in the active case</small></span><span className="evidence-intake__boundary"><i className="fas fa-lock" /> Scope locked to {caseId}</span></summary>
        <form onSubmit={upload} className="evidence-intake__body">
          <label className="evidence-dropzone">
            <i className="fas fa-file-circle-plus" />
            <span><strong>Choose one or more source files</strong><small>Registration preserves case scope and source bytes. Raster images receive bounded signature, dimensions, orientation, pixel-budget, and metadata-privacy checks before later analysis.</small></span>
            <input type="file" multiple onChange={event => setFiles(Array.from(event.target.files || []))} />
          </label>
          {files.length > 0 && <div className="evidence-staging" aria-label="Staged evidence files">{files.map((file, index) => { const state = imageStagingState(file); return <div key={fileKey(file, index)} data-blocked={state.blocked}><span><strong>{file.name}</strong><small>{formatBytes(file.size)} · {state.message}</small></span><button type="button" onClick={() => setFiles(current => current.filter((_, itemIndex) => itemIndex !== index))} aria-label={`Remove ${file.name}`}><i className="fas fa-xmark" /></button></div> })}</div>}
          {blockedStagedFiles.length > 0 && <div className="alert alert-error" role="alert">A staged file cannot be submitted. Remove it before registration.</div>}
          <div className="evidence-intake__actions"><span>{files.length ? `${files.length} source file${files.length === 1 ? '' : 's'} staged; max 2 concurrent registrations` : 'Nothing is uploaded until registration is confirmed.'}</span><button className="btn btn-primary" disabled={!files.length || uploading || blockedStagedFiles.length > 0}>{uploading ? <><i className="fas fa-spinner fa-spin" /> Registering</> : <><i className="fas fa-shield-halved" /> Confirm registration</>}</button></div>
          {intakeResults.length > 0 && <div className="evidence-intake-ledger" aria-label="Evidence intake results">{intakeResults.map(result => <div key={result.key}><IntakeResultPill status={result.status} /><span><strong>{result.name}</strong><small>{result.message}{result.job_id ? `; ${result.job_id}` : ''}</small></span></div>)}</div>}
        </form>
      </details>

      <div className="evidence-browser" data-inspecting={Boolean(selectedId)}>
        <section className="evidence-catalog" aria-label="Registered evidence catalog">
          {items.length ? items.map(item => (
            <button type="button" className="evidence-source" data-selected={selectedId === item.evidence_id} key={item.evidence_id} onClick={() => inspect(item)} aria-label={`Inspect ${item.source_file || item.original_filename || item.evidence_id}`}>
              <span className="evidence-source__icon"><i className={`fas ${resolveEvidenceModality(item).icon}`} /></span>
              <span className="evidence-source__content"><strong>{item.source_file || item.original_filename || 'Unnamed evidence'}</strong><small>{resolveEvidenceModality(item).label} · {humanize(item.detected_type, 'Unclassified')}</small><code>{item.sha256 ? `SHA-256 ${String(item.sha256).slice(0, 20)}…` : 'Hash unavailable'}</code></span>
              <span className="evidence-source__state"><StatusPill status={item.processing_status} /><small>{formatBytes(item.size_bytes)}</small><i className="fas fa-chevron-right" /></span>
            </button>
          )) : <div className="evidence-catalog-empty"><i className="fas fa-box-open" /><h2>No registered evidence on this page</h2><p>{query || statusFilter || modalityFilter ? 'Clear or change the active filters.' : 'Use Evidence intake to establish source identity and processing provenance.'}</p></div>}
        </section>
        <aside className="evidence-inspection-shell" aria-live="polite"><EvidenceInspection detail={detail} loading={detailLoading} error={detailError} reprocessPlan={reprocessPlan} reprocessLoading={reprocessPlanLoading} reprocessError={reprocessPlanError} onClose={closeInspection} /></aside>
      </div>
    </div>
  )
}
