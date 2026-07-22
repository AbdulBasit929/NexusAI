package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultEvidenceLimit = 50
	maxEvidenceLimit     = 250
)

type evidenceListRequest struct {
	TenantID         string
	CollectionID     string
	CaseID           string
	Modality         string
	DetectedType     string
	ProcessingStatus string
	Query            string
	Limit            int
	Offset           int
}

func evidenceListHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
			return
		}
		if db == nil {
			writeError(w, http.StatusServiceUnavailable, errors.New("FORENSIC_DATABASE_URL is not configured"))
			return
		}
		req := evidenceListRequest{
			TenantID:         defaultString(r.URL.Query().Get("tenant_id"), "default"),
			CollectionID:     strings.TrimSpace(r.URL.Query().Get("collection_id")),
			CaseID:           strings.TrimSpace(r.URL.Query().Get("case_id")),
			Modality:         normalize(r.URL.Query().Get("modality")),
			DetectedType:     normalize(r.URL.Query().Get("detected_type")),
			ProcessingStatus: normalize(r.URL.Query().Get("processing_status")),
			Query:            strings.TrimSpace(r.URL.Query().Get("q")),
			Limit:            clampEvidenceLimit(queryInt(r, "limit", defaultEvidenceLimit)),
			Offset:           clampOffset(queryInt(r, "offset", 0)),
		}
		if req.CollectionID == "" {
			writeError(w, http.StatusBadRequest, errors.New("collection_id is required"))
			return
		}
		resp, err := loadEvidenceCatalog(r.Context(), db, req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func evidenceDetailHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
			return
		}
		if db == nil {
			writeError(w, http.StatusServiceUnavailable, errors.New("FORENSIC_DATABASE_URL is not configured"))
			return
		}
		evidenceID := strings.TrimSpace(r.PathValue("evidence_id"))
		if evidenceID == "" {
			writeError(w, http.StatusBadRequest, errors.New("evidence_id is required"))
			return
		}
		tenantID := defaultString(r.URL.Query().Get("tenant_id"), "default")
		limit := clampEvidenceLimit(queryInt(r, "limit", 25))
		resp, err := loadEvidenceDetail(r.Context(), db, tenantID, evidenceID, limit)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusNotFound, errors.New("evidence item not found"))
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func loadEvidenceCatalog(ctx context.Context, db *pgxpool.Pool, req evidenceListRequest) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin evidence catalog query: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", req.TenantID); err != nil {
		return nil, fmt.Errorf("set tenant context: %w", err)
	}

	summary, err := rowsFromQuery(ctx, tx, evidenceSummarySQL, req.TenantID, req.CollectionID, req.CaseID, req.Modality, req.DetectedType, req.ProcessingStatus, req.Query)
	if err != nil {
		return nil, err
	}
	items, err := rowsFromQuery(ctx, tx, evidenceListSQL, req.TenantID, req.CollectionID, req.CaseID, req.Modality, req.DetectedType, req.ProcessingStatus, req.Query, req.Limit, req.Offset)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit evidence catalog query: %w", err)
	}
	return map[string]any{
		"tenant_id":     req.TenantID,
		"collection_id": req.CollectionID,
		"filters": map[string]any{
			"case_id":           req.CaseID,
			"modality":          req.Modality,
			"detected_type":     req.DetectedType,
			"processing_status": req.ProcessingStatus,
			"q":                 req.Query,
			"limit":             req.Limit,
			"offset":            req.Offset,
		},
		"summary":      firstRow(summary),
		"items":        items,
		"generated_at": time.Now().UTC(),
	}, nil
}

func loadEvidenceDetail(ctx context.Context, db *pgxpool.Pool, tenantID, evidenceID string, limit int) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin evidence detail query: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return nil, fmt.Errorf("set tenant context: %w", err)
	}
	itemRows, err := rowsFromQuery(ctx, tx, evidenceDetailSQL, tenantID, evidenceID)
	if err != nil {
		return nil, err
	}
	if len(itemRows) == 0 {
		return nil, pgx.ErrNoRows
	}
	item := itemRows[0]
	jobs, err := rowsFromQuery(ctx, tx, evidenceJobsSQL, tenantID, evidenceID)
	if err != nil {
		return nil, err
	}
	assets, err := rowsFromQuery(ctx, tx, evidenceAssetsSQL, tenantID, evidenceID)
	if err != nil {
		return nil, err
	}
	recordsPreview, err := rowsFromQuery(ctx, tx, evidenceRecordsPreviewSQL, tenantID, evidenceID, limit)
	if err != nil {
		return nil, err
	}
	entityRollup, err := rowsFromQuery(ctx, tx, evidenceEntityRollupSQL, tenantID, evidenceID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit evidence detail query: %w", err)
	}
	return map[string]any{
		"tenant_id":       tenantID,
		"evidence_id":     evidenceID,
		"item":            item,
		"ingest_jobs":     jobs,
		"kb_assets":       assets,
		"records_preview": recordsPreview,
		"entity_rollup":   entityRollup,
		"generated_at":    time.Now().UTC(),
	}, nil
}

const evidenceWhereSQL = `
WHERE evidence.tenant_id = $1
  AND evidence.collection_id = $2
  AND ($3 = '' OR coalesce(evidence.case_id, '') = $3)
  AND ($4 = '' OR evidence.modality = $4)
  AND ($5 = '' OR evidence.detected_type = $5)
  AND ($6 = '' OR evidence.processing_status = $6)
  AND (
    $7 = ''
    OR evidence.source_file ILIKE '%' || $7 || '%'
    OR evidence.original_filename ILIKE '%' || $7 || '%'
    OR evidence.sha256 ILIKE '%' || $7 || '%'
    OR coalesce(evidence.kb_entry_ref, '') ILIKE '%' || $7 || '%'
    OR evidence.metadata::text ILIKE '%' || $7 || '%'
  )`

const evidenceSummarySQL = `
WITH filtered AS (
  SELECT evidence.*
  FROM forensic.evidence_items evidence
` + evidenceWhereSQL + `
),
status_rollup AS (
  SELECT processing_status, count(*) AS status_count
  FROM filtered
  GROUP BY processing_status
),
modality_rollup AS (
  SELECT modality, count(*) AS modality_count
  FROM filtered
  GROUP BY modality
)
SELECT
  (SELECT count(*) FROM filtered) AS evidence_total,
  (SELECT count(*) FROM filtered WHERE processing_status = 'completed') AS completed,
  (SELECT count(*) FROM filtered WHERE processing_status = 'processing') AS processing,
  (SELECT count(*) FROM filtered WHERE processing_status IN ('queued', 'registered')) AS queued,
  (SELECT count(*) FROM filtered WHERE processing_status = 'failed') AS failed,
  (SELECT count(*) FROM filtered WHERE modality = 'structured_records') AS structured_records,
  (SELECT count(*) FROM filtered WHERE modality = 'document') AS documents,
  (SELECT count(*) FROM filtered WHERE modality = 'image') AS images,
  (SELECT count(*) FROM filtered WHERE modality = 'audio') AS audio,
  (SELECT count(*) FROM filtered WHERE modality = 'video') AS video,
  (SELECT coalesce(sum(size_bytes), 0) FROM filtered) AS total_size_bytes,
  coalesce((SELECT jsonb_object_agg(processing_status, status_count ORDER BY processing_status) FROM status_rollup), '{}'::jsonb) AS status_counts,
  coalesce((SELECT jsonb_object_agg(modality, modality_count ORDER BY modality) FROM modality_rollup), '{}'::jsonb) AS modality_counts`

const evidenceListSQL = `
SELECT
  evidence.evidence_id::text,
  evidence.tenant_id,
  evidence.collection_id,
  evidence.case_id,
  evidence.user_id,
  evidence.original_filename,
  evidence.source_file,
  evidence.content_type,
  evidence.extension,
  evidence.size_bytes,
  evidence.sha256,
  evidence.modality,
  evidence.detected_type,
  evidence.classifier_confidence,
  evidence.processing_route,
  evidence.processing_status,
  evidence.raw_storage_ref,
  evidence.kb_entry_ref,
  evidence.records_batch_id::text,
  evidence.extracted_text_ref,
  evidence.media_metadata,
  evidence.entities,
  evidence.metadata,
  evidence.warnings,
  evidence.errors,
  evidence.created_at,
  evidence.updated_at,
  jobs.id::text AS job_id,
  jobs.status::text AS ingest_status,
  jobs.total_rows,
  jobs.accepted_rows,
  assets.id::text AS kb_asset_id,
  assets.rag_status,
  assets.structured_status,
  assets.storage_mode
FROM forensic.evidence_items evidence
LEFT JOIN forensic.records_ingest_jobs jobs
  ON jobs.evidence_id = evidence.evidence_id
 AND jobs.tenant_id = evidence.tenant_id
LEFT JOIN forensic.kb_collection_assets assets
  ON assets.evidence_id = evidence.evidence_id
 AND assets.tenant_id = evidence.tenant_id
` + evidenceWhereSQL + `
ORDER BY evidence.created_at DESC, evidence.evidence_id
LIMIT $8
OFFSET $9`

const evidenceDetailSQL = `
SELECT
  evidence_id::text,
  tenant_id,
  collection_id,
  case_id,
  user_id,
  original_filename,
  source_file,
  content_type,
  extension,
  size_bytes,
  sha256,
  modality,
  detected_type,
  classifier_confidence,
  processing_route,
  processing_status,
  raw_storage_ref,
  kb_entry_ref,
  records_batch_id::text,
  extracted_text_ref,
  media_metadata,
  entities,
  metadata,
  warnings,
  errors,
  created_at,
  updated_at
FROM forensic.evidence_items
WHERE tenant_id = $1 AND evidence_id = $2::uuid`

const evidenceJobsSQL = `
SELECT id::text AS job_id, source_file, record_type::text AS record_type, status::text AS status,
       attempt_count, max_attempts, total_rows, accepted_rows, duplicate_rows,
       rejected_rows, error_message, queued_at, started_at, completed_at, metadata
FROM forensic.records_ingest_jobs
WHERE tenant_id = $1 AND evidence_id = $2::uuid
ORDER BY queued_at DESC`

const evidenceAssetsSQL = `
SELECT id::text AS kb_asset_id, collection_id, file_id, batch_id::text, source_file,
       source_entry, detected_record_type::text AS detected_record_type,
       requested_record_type::text AS requested_record_type, storage_mode,
       rag_status, structured_status, content_type, size_bytes, headers,
       routing_decision, quality_report, created_at, updated_at
FROM forensic.kb_collection_assets
WHERE tenant_id = $1 AND evidence_id = $2::uuid
ORDER BY updated_at DESC`

const evidenceRecordsPreviewSQL = `
SELECT record_id::text, collection_id, file_id, batch_id::text, record_type,
       "timestamp", primary_target, secondary_target, source_file, row_number,
       row_hash, raw_payload, metadata, ingested_at
FROM forensic.records
WHERE tenant_id = $1 AND evidence_id = $2::uuid
ORDER BY "timestamp" DESC, row_number
LIMIT $3`

const evidenceEntityRollupSQL = `
WITH batches AS (
  SELECT tenant_id, collection_id, file_id, id AS batch_id
  FROM forensic.records_ingest_jobs
  WHERE tenant_id = $1 AND evidence_id = $2::uuid
)
SELECT entities.entity_type,
       count(*) AS observation_count,
       count(DISTINCT entities.entity_value) AS unique_entities,
       min(entities.observed_at) AS first_seen,
       max(entities.observed_at) AS last_seen
FROM forensic.record_entities entities
JOIN batches
  ON batches.tenant_id = entities.tenant_id
 AND batches.collection_id = entities.collection_id
 AND batches.file_id = entities.file_id
 AND batches.batch_id = entities.batch_id
GROUP BY entities.entity_type
ORDER BY observation_count DESC, entities.entity_type`

func clampEvidenceLimit(limit int) int {
	if limit < 1 {
		return defaultEvidenceLimit
	}
	if limit > maxEvidenceLimit {
		return maxEvidenceLimit
	}
	return limit
}
